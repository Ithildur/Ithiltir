package frontcache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"dash/internal/infra/cachekeys"
	"dash/internal/metrics"

	"github.com/redis/go-redis/v9"
)

var errCorruptFrontSnapshot = errors.New("corrupt front snapshot")
var errCorruptFrontRuntime = errors.New("corrupt front runtime")
var errDuplicateFrontSnapshotID = errors.New("duplicate front snapshot node id")
var errInvalidFrontSnapshotID = errors.New("invalid front snapshot node id")
var errFrontSnapshotMissingID = errors.New("front snapshot node missing id")

type redisCacheBackend struct {
	redis *redis.Client
}

type memCacheBackend struct {
	mem *memState
}

func newCacheBackend(redisClient *redis.Client, mem *memState) cacheBackend {
	if redisClient != nil {
		return &redisCacheBackend{redis: redisClient}
	}
	return &memCacheBackend{mem: mem}
}

func (s *Store) fetchSnapshotCache(ctx context.Context) ([]metrics.NodeView, bool, error) {
	nodes, ok, err := s.backend.fetchSnapshotCache(ctx)
	if errors.Is(err, errCorruptFrontSnapshot) {
		if clearErr := s.backend.clearFrontMeta(ctx); clearErr != nil {
			return nil, false, errors.Join(err, clearErr)
		}
		return nil, false, nil
	}
	return nodes, ok, err
}

func (s *Store) LoadFrontNodeSnapshot(ctx context.Context, id int64) (*metrics.NodeView, error) {
	return s.backend.loadFrontNodeSnapshot(ctx, id)
}

func (s *Store) ListFrontSnapshotIDs(ctx context.Context) ([]int64, error) {
	return s.backend.listFrontSnapshotIDs(ctx)
}

func (s *Store) replaceFrontSnapshot(ctx context.Context, nodes []metrics.NodeView) error {
	return s.backend.replaceSnapshot(ctx, nodes)
}

func (s *Store) PutNodeSnapshot(ctx context.Context, node metrics.NodeView) error {
	return s.backend.putNodeSnapshot(ctx, node)
}

func (s *Store) PatchNodeSnapshot(ctx context.Context, id int64, name *string, order *int) error {
	return s.backend.patchNodeSnapshot(ctx, id, name, order)
}

func (s *Store) RemoveNodeSnapshot(ctx context.Context, id int64) error {
	return s.backend.removeNodeSnapshot(ctx, id)
}

func (s *Store) ClearFrontMeta(ctx context.Context) error {
	return s.backend.clearFrontMeta(ctx)
}

func (b *memCacheBackend) loadFrontNodeSnapshot(_ context.Context, id int64) (*metrics.NodeView, error) {
	if id <= 0 {
		return nil, nil
	}
	idStr := strconv.FormatInt(id, 10)
	b.mem.mu.RLock()
	raw, ok := b.mem.frontNodes[idStr]
	smartRaw := b.mem.frontSmart[idStr]
	thermalRaw := b.mem.frontThermal[idStr]
	b.mem.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	node, err := decodeFrontNode(raw, idStr)
	if err != nil {
		return nil, err
	}
	if err := applyFrontRuntime(&node, smartRaw, thermalRaw); err != nil {
		return nil, err
	}
	return &node, nil
}

func (b *redisCacheBackend) loadFrontNodeSnapshot(ctx context.Context, id int64) (*metrics.NodeView, error) {
	if id <= 0 {
		return nil, nil
	}
	idStr := strconv.FormatInt(id, 10)
	raw, err := b.redis.Get(ctx, cachekeys.RedisKeyFrontNodeSnapshotPrefix+idStr).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	node, err := decodeFrontNode(raw, idStr)
	if err != nil {
		return nil, err
	}
	smartRuntime, err := b.loadSmartRuntime(ctx, id)
	if err != nil {
		return nil, err
	}
	thermalRuntime, err := b.loadThermalRuntime(ctx, id)
	if err != nil {
		return nil, err
	}
	applySmartRuntime(&node, smartRuntime)
	applyThermalRuntime(&node, thermalRuntime)
	return &node, nil
}

func (b *redisCacheBackend) loadSmartRuntime(ctx context.Context, id int64) (*frontSmartRuntime, error) {
	if id <= 0 {
		return nil, nil
	}
	key := cachekeys.RedisKeyFrontNodeSmartPrefix + strconv.FormatInt(id, 10)
	raw, err := b.redis.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	runtime, err := decodeSmartRuntime(raw)
	if err != nil {
		if delErr := b.redis.Del(ctx, key).Err(); delErr != nil {
			return nil, errors.Join(err, delErr)
		}
		return nil, nil
	}
	return runtime, nil
}

func (b *memCacheBackend) loadSmartRuntimes(_ context.Context, ids []int64) (map[int64]*frontSmartRuntime, error) {
	out := make(map[int64]*frontSmartRuntime, len(ids))
	b.mem.mu.RLock()
	defer b.mem.mu.RUnlock()
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		runtime, err := decodeSmartRuntime(b.mem.frontSmart[strconv.FormatInt(id, 10)])
		if err != nil {
			return nil, err
		}
		if runtime != nil {
			out[id] = runtime
		}
	}
	return out, nil
}

func (b *redisCacheBackend) loadThermalRuntime(ctx context.Context, id int64) (*frontThermalRuntime, error) {
	if id <= 0 {
		return nil, nil
	}
	key := cachekeys.RedisKeyFrontNodeThermalPrefix + strconv.FormatInt(id, 10)
	raw, err := b.redis.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	runtime, err := decodeThermalRuntime(raw)
	if err != nil {
		if delErr := b.redis.Del(ctx, key).Err(); delErr != nil {
			return nil, errors.Join(err, delErr)
		}
		return nil, nil
	}
	return runtime, nil
}

func (b *redisCacheBackend) loadSmartRuntimes(ctx context.Context, ids []int64) (map[int64]*frontSmartRuntime, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, cachekeys.RedisKeyFrontNodeSmartPrefix+strconv.FormatInt(id, 10))
	}
	vals, err := b.redis.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*frontSmartRuntime, len(ids))
	for i, v := range vals {
		raw, ok := redisBytes(v)
		if !ok {
			continue
		}
		runtime, err := decodeSmartRuntime(raw)
		if err != nil {
			if delErr := b.redis.Del(ctx, keys[i]).Err(); delErr != nil {
				return nil, errors.Join(err, delErr)
			}
			continue
		}
		if runtime != nil {
			out[ids[i]] = runtime
		}
	}
	return out, nil
}

func (b *memCacheBackend) listFrontSnapshotIDs(_ context.Context) ([]int64, error) {
	b.mem.mu.RLock()
	defer b.mem.mu.RUnlock()
	ids := make([]int64, 0, len(b.mem.frontNodes))
	for item := range b.mem.frontNodes {
		id, err := frontSnapshotID(item)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (b *redisCacheBackend) listFrontSnapshotIDs(ctx context.Context) ([]int64, error) {
	ids := make([]int64, 0)
	var cursor uint64
	for {
		items, next, err := b.redis.SScan(ctx, cachekeys.RedisKeyFrontNodeIDs, cursor, "*", 256).Result()
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			id, err := frontSnapshotID(item)
			if err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		cursor = next
		if cursor == 0 {
			return ids, nil
		}
	}
}

func (b *memCacheBackend) fetchSnapshotCache(_ context.Context) ([]metrics.NodeView, bool, error) {
	b.mem.mu.RLock()
	if !b.mem.frontMeta {
		b.mem.mu.RUnlock()
		return nil, false, nil
	}
	nodes := make([]metrics.NodeView, 0, len(b.mem.frontNodes))
	for id, raw := range b.mem.frontNodes {
		node, err := decodeFrontNode(raw, id)
		if err != nil {
			b.mem.mu.RUnlock()
			return nil, false, errCorruptFrontSnapshot
		}
		if err := applyFrontRuntime(&node, b.mem.frontSmart[id], b.mem.frontThermal[id]); err != nil {
			b.mem.mu.RUnlock()
			return nil, false, corruptFrontRuntime(err)
		}
		nodes = append(nodes, node)
	}
	b.mem.mu.RUnlock()
	return nodes, true, nil
}

func (b *redisCacheBackend) fetchSnapshotCache(ctx context.Context) ([]metrics.NodeView, bool, error) {
	count, ok, err := b.loadMeta(ctx, cachekeys.RedisKeyFrontMeta)
	if errors.Is(err, errCorruptCacheMeta) {
		return nil, false, errCorruptFrontSnapshot
	}
	if err != nil || !ok {
		return nil, false, err
	}

	ids, err := b.redis.SMembers(ctx, cachekeys.RedisKeyFrontNodeIDs).Result()
	if err != nil {
		return nil, false, err
	}
	if len(ids) != count {
		return nil, false, errCorruptFrontSnapshot
	}
	if len(ids) == 0 {
		return []metrics.NodeView{}, true, nil
	}

	keys := make([]string, 0, len(ids))
	smartKeys := make([]string, 0, len(ids))
	thermalKeys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, cachekeys.RedisKeyFrontNodeSnapshotPrefix+id)
		smartKeys = append(smartKeys, cachekeys.RedisKeyFrontNodeSmartPrefix+id)
		thermalKeys = append(thermalKeys, cachekeys.RedisKeyFrontNodeThermalPrefix+id)
	}
	vals, err := b.redis.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, false, err
	}
	thermalVals, err := b.redis.MGet(ctx, thermalKeys...).Result()
	if err != nil {
		return nil, false, err
	}
	smartVals, err := b.redis.MGet(ctx, smartKeys...).Result()
	if err != nil {
		return nil, false, err
	}

	nodes := make([]metrics.NodeView, 0, len(vals))
	for i, v := range vals {
		raw, ok := redisBytes(v)
		if !ok {
			return nil, false, errCorruptFrontSnapshot
		}
		n, err := decodeFrontNode(raw, ids[i])
		if err != nil {
			return nil, false, errCorruptFrontSnapshot
		}
		var smartRaw []byte
		if raw, ok := redisBytes(smartVals[i]); ok {
			smartRaw = raw
		}
		var thermalRaw []byte
		if raw, ok := redisBytes(thermalVals[i]); ok {
			thermalRaw = raw
		}
		if err := applyFrontRuntime(&n, smartRaw, thermalRaw); err != nil {
			if delErr := b.redis.Del(ctx, smartKeys[i], thermalKeys[i]).Err(); delErr != nil {
				return nil, false, errors.Join(corruptFrontRuntime(err), delErr)
			}
			return nil, false, corruptFrontRuntime(err)
		}
		nodes = append(nodes, n)
	}
	return nodes, true, nil
}

func (b *memCacheBackend) putNodeSnapshot(_ context.Context, node metrics.NodeView) error {
	id, raw, err := frontSnapshotPayload(node)
	if err != nil {
		return err
	}
	smartRaw, hasSmart, err := frontSmartPayload(node)
	if err != nil {
		return err
	}
	thermalRaw, hasThermal, err := frontThermalPayload(node)
	if err != nil {
		return err
	}
	b.mem.mu.Lock()
	_, exists := b.mem.frontNodes[id]
	if !exists {
		b.mem.frontMeta = false
	}
	b.mem.frontNodes[id] = raw
	if hasSmart {
		b.mem.frontSmart[id] = smartRaw
	} else {
		delete(b.mem.frontSmart, id)
	}
	if hasThermal {
		b.mem.frontThermal[id] = thermalRaw
	} else {
		delete(b.mem.frontThermal, id)
	}
	b.mem.mu.Unlock()
	return nil
}

func (b *redisCacheBackend) putNodeSnapshot(ctx context.Context, node metrics.NodeView) error {
	id, raw, err := frontSnapshotPayload(node)
	if err != nil {
		return err
	}
	smartRaw, hasSmart, err := frontSmartPayload(node)
	if err != nil {
		return err
	}
	thermalRaw, hasThermal, err := frontThermalPayload(node)
	if err != nil {
		return err
	}
	exists, err := b.redis.SIsMember(ctx, cachekeys.RedisKeyFrontNodeIDs, id).Result()
	if err != nil {
		return err
	}
	_, err = b.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, cachekeys.RedisKeyFrontNodeSnapshotPrefix+id, raw, 0)
		if hasSmart {
			pipe.Set(ctx, cachekeys.RedisKeyFrontNodeSmartPrefix+id, smartRaw, 0)
		} else {
			pipe.Del(ctx, cachekeys.RedisKeyFrontNodeSmartPrefix+id)
		}
		if hasThermal {
			pipe.Set(ctx, cachekeys.RedisKeyFrontNodeThermalPrefix+id, thermalRaw, 0)
		} else {
			pipe.Del(ctx, cachekeys.RedisKeyFrontNodeThermalPrefix+id)
		}
		pipe.SAdd(ctx, cachekeys.RedisKeyFrontNodeIDs, id)
		if !exists {
			pipe.Del(ctx, cachekeys.RedisKeyFrontMeta)
		}
		return nil
	})
	return err
}

func (b *memCacheBackend) patchNodeSnapshot(_ context.Context, id int64, name *string, order *int) error {
	if id <= 0 || (name == nil && order == nil) {
		return nil
	}
	idStr := strconv.FormatInt(id, 10)
	b.mem.mu.Lock()
	defer b.mem.mu.Unlock()

	raw, ok := b.mem.frontNodes[idStr]
	if !ok {
		return nil
	}
	payload, err := patchNodeSnapshotPayload(raw, idStr, name, order)
	if err != nil {
		return err
	}
	b.mem.frontNodes[idStr] = payload
	return nil
}

func (b *redisCacheBackend) patchNodeSnapshot(ctx context.Context, id int64, name *string, order *int) error {
	if id <= 0 || (name == nil && order == nil) {
		return nil
	}
	idStr := strconv.FormatInt(id, 10)
	key := cachekeys.RedisKeyFrontNodeSnapshotPrefix + idStr
	raw, err := b.redis.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return err
	}
	payload, err := patchNodeSnapshotPayload(raw, idStr, name, order)
	if err != nil {
		return err
	}
	return b.redis.Set(ctx, key, payload, 0).Err()
}

func (b *memCacheBackend) removeNodeSnapshot(_ context.Context, id int64) error {
	if id <= 0 {
		return nil
	}
	idStr := strconv.FormatInt(id, 10)
	b.mem.mu.Lock()
	delete(b.mem.frontNodes, idStr)
	delete(b.mem.frontSmart, idStr)
	delete(b.mem.frontThermal, idStr)
	delete(b.mem.frontGuestVisible, idStr)
	b.mem.frontMeta = false
	b.mem.guestVisibleMeta = false
	b.mem.mu.Unlock()
	return nil
}

func (b *redisCacheBackend) removeNodeSnapshot(ctx context.Context, id int64) error {
	if id <= 0 {
		return nil
	}
	idStr := strconv.FormatInt(id, 10)
	_, err := b.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx,
			cachekeys.RedisKeyFrontNodeSnapshotPrefix+idStr,
			cachekeys.RedisKeyFrontNodeSmartPrefix+idStr,
			cachekeys.RedisKeyFrontNodeThermalPrefix+idStr,
		)
		pipe.SRem(ctx, cachekeys.RedisKeyFrontNodeIDs, idStr)
		pipe.SRem(ctx, cachekeys.RedisKeyGuestVisibleIDs, idStr)
		pipe.Del(ctx, cachekeys.RedisKeyFrontMeta, cachekeys.RedisKeyGuestVisibilityMeta)
		return nil
	})
	return err
}

func (b *memCacheBackend) replaceSnapshot(_ context.Context, nodes []metrics.NodeView) error {
	newSet, thermalSet, err := frontSnapshotPayloads(nodes)
	if err != nil {
		return err
	}
	b.mem.mu.Lock()
	b.mem.frontMeta = false
	b.mem.frontNodes = newSet
	b.mem.frontThermal = thermalSet
	b.mem.frontSmart = keepFrontRuntime(b.mem.frontSmart, newSet)
	b.mem.frontMeta = true
	b.mem.mu.Unlock()
	return nil
}

func (b *redisCacheBackend) replaceSnapshot(ctx context.Context, nodes []metrics.NodeView) error {
	oldIDs, err := b.redis.SMembers(ctx, cachekeys.RedisKeyFrontNodeIDs).Result()
	if err != nil {
		return err
	}

	newSet, thermalSet, err := frontSnapshotPayloads(nodes)
	if err != nil {
		return err
	}

	removedKeys := make([]string, 0, len(oldIDs)*3)
	for _, id := range oldIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		removedKeys = append(removedKeys, cachekeys.RedisKeyFrontNodeThermalPrefix+id)
		if _, ok := newSet[id]; ok {
			continue
		}
		removedKeys = append(removedKeys, cachekeys.RedisKeyFrontNodeSnapshotPrefix+id)
		removedKeys = append(removedKeys, cachekeys.RedisKeyFrontNodeSmartPrefix+id)
	}

	meta := cacheMetaValue(len(newSet))
	_, err = b.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, cachekeys.RedisKeyFrontMeta)
		pipe.Del(ctx, cachekeys.RedisKeyFrontNodeIDs)
		if len(removedKeys) > 0 {
			pipe.Del(ctx, removedKeys...)
		}
		if len(newSet) > 0 {
			members := make([]interface{}, 0, len(newSet))
			for id, raw := range newSet {
				pipe.Set(ctx, cachekeys.RedisKeyFrontNodeSnapshotPrefix+id, raw, 0)
				members = append(members, id)
			}
			for id, raw := range thermalSet {
				pipe.Set(ctx, cachekeys.RedisKeyFrontNodeThermalPrefix+id, raw, 0)
			}
			pipe.SAdd(ctx, cachekeys.RedisKeyFrontNodeIDs, members...)
		}
		pipe.Set(ctx, cachekeys.RedisKeyFrontMeta, meta, 0)
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func frontSnapshotPayloads(nodes []metrics.NodeView) (map[string][]byte, map[string][]byte, error) {
	out := make(map[string][]byte, len(nodes))
	thermal := make(map[string][]byte, len(nodes))
	for _, node := range nodes {
		id, snapshotRaw, err := frontSnapshotPayload(node)
		if err != nil {
			return nil, nil, err
		}
		if _, exists := out[id]; exists {
			return nil, nil, fmt.Errorf("%w: %s", errDuplicateFrontSnapshotID, id)
		}
		out[id] = snapshotRaw
		thermalRaw, ok, err := frontThermalPayload(node)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			thermal[id] = thermalRaw
		}
	}
	return out, thermal, nil
}

func frontSnapshotPayload(node metrics.NodeView) (string, []byte, error) {
	id, err := normalizeFrontNodeID(node.Node.ID)
	if err != nil {
		return "", nil, err
	}
	node.Node.ID = id
	node.Disk.Smart = nil
	node.Disk.TemperatureDevices = nil
	node.Thermal = nil
	raw, err := json.Marshal(node)
	if err != nil {
		return "", nil, err
	}
	return id, raw, nil
}

func patchNodeSnapshotPayload(raw []byte, id string, name *string, order *int) ([]byte, error) {
	node, err := decodeFrontNode(raw, id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		patchNodeTitle(&node, *name)
	}
	if order != nil {
		node.Node.Order = *order
	}
	return json.Marshal(node)
}

func patchNodeTitle(node *metrics.NodeView, name string) {
	oldTitle := strings.TrimSpace(node.Node.Title)
	title := strings.TrimSpace(name)
	node.Node.Title = title

	items := append([]string{title}, node.Node.SearchText...)
	searchText := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if oldTitle != "" && oldTitle != title && item == oldTitle {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		searchText = append(searchText, item)
	}
	node.Node.SearchText = searchText
}

func decodeFrontNode(raw []byte, wantID string) (metrics.NodeView, error) {
	var node metrics.NodeView
	if err := json.Unmarshal(raw, &node); err != nil {
		return node, err
	}
	id, err := normalizeFrontNodeID(node.Node.ID)
	if err != nil || id != wantID {
		return node, errCorruptFrontSnapshot
	}
	node.Node.ID = id
	return node, nil
}

func normalizeFrontNodeID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return "", errFrontSnapshotMissingID
	}
	parsed, ok := metrics.ParseNodeID(id)
	if !ok || strconv.FormatInt(parsed, 10) != id {
		return "", errInvalidFrontSnapshotID
	}
	return id, nil
}

func frontSnapshotID(raw string) (int64, error) {
	id, err := normalizeFrontNodeID(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", err, raw)
	}
	parsed, _ := metrics.ParseNodeID(id)
	return parsed, nil
}

func (b *memCacheBackend) clearFrontMeta(_ context.Context) error {
	b.mem.mu.Lock()
	b.mem.frontMeta = false
	b.mem.mu.Unlock()
	return nil
}

func (b *redisCacheBackend) clearFrontMeta(ctx context.Context) error {
	return b.redis.Del(ctx, cachekeys.RedisKeyFrontMeta).Err()
}

func redisBytes(v any) ([]byte, bool) {
	switch val := v.(type) {
	case string:
		return []byte(val), true
	case []byte:
		return val, true
	default:
		return nil, false
	}
}
