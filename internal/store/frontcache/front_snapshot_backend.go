package frontcache

import (
	"context"
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
		if clearErr := s.ClearFrontMeta(ctx); clearErr != nil {
			return nil, false, errors.Join(err, clearErr)
		}
		return nil, false, nil
	}
	return nodes, ok, err
}

func (s *Store) replaceFrontSnapshotIfCurrent(ctx context.Context, nodes []frontNodeProjection, version uint64) (bool, error) {
	return s.publishProjectionIfCurrent(version, func() error {
		return s.backend.replaceSnapshot(ctx, nodes)
	})
}

func (s *Store) PutNodeRuntime(ctx context.Context, node metrics.NodeView, memoryTotal, swapTotal int64) error {
	projection := frontNodeProjectionFromView(node)
	projection.MemoryTotal = memoryTotal
	projection.SwapTotal = swapTotal
	return s.putNodeRuntime(ctx, projection)
}

func (s *Store) putNodeRuntime(ctx context.Context, projection frontNodeProjection) error {
	id, ok := metrics.ParseNodeID(projection.Node.Node.ID)
	if !ok {
		return fmt.Errorf("%w: %q", errInvalidFrontSnapshotID, projection.Node.Node.ID)
	}
	known, err := s.backend.hasNodeRuntime(ctx, id)
	if err != nil {
		return err
	}
	if known {
		s.forgetUnknownRuntime(id)
		return s.backend.putNodeRuntime(ctx, projection, false)
	}
	if !s.rememberUnknownRuntime(id) {
		return s.backend.putNodeRuntime(ctx, projection, false)
	}
	// Runtime samples never create catalog membership. An unknown ID only
	// invalidates the derived catalog so PostgreSQL can decide membership during
	// the next rebuild.
	err = s.projection.Mutate(func() error {
		return s.backend.putNodeRuntime(ctx, projection, true)
	})
	if err != nil {
		s.forgetUnknownRuntime(id)
	}
	return err
}

func (s *Store) rememberUnknownRuntime(id int64) bool {
	s.unknownMu.Lock()
	defer s.unknownMu.Unlock()
	if _, exists := s.unknownRuntime[id]; exists {
		return false
	}
	s.unknownRuntime[id] = struct{}{}
	return true
}

func (s *Store) forgetUnknownRuntime(id int64) {
	s.unknownMu.Lock()
	delete(s.unknownRuntime, id)
	s.unknownMu.Unlock()
}

func (s *Store) RemoveNodeMetadata(ctx context.Context, id int64) error {
	return s.backend.removeNodeMetadata(ctx, id)
}

func (s *Store) RemoveNodeSnapshot(ctx context.Context, id int64) error {
	return s.backend.removeNodeSnapshot(ctx, id)
}

func (s *Store) ClearFrontMeta(ctx context.Context) error {
	return s.backend.clearFrontMeta(ctx)
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

func (b *memCacheBackend) fetchSnapshotCache(_ context.Context) ([]metrics.NodeView, bool, error) {
	b.mem.mu.RLock()
	if !b.mem.frontCatalog {
		b.mem.mu.RUnlock()
		return nil, false, nil
	}
	nodes := make([]metrics.NodeView, 0, len(b.mem.frontIDs))
	for id := range b.mem.frontIDs {
		runtimeRaw, ok := b.mem.frontRuntime[id]
		if !ok {
			b.mem.mu.RUnlock()
			return nil, false, errCorruptFrontSnapshot
		}
		metaRaw, ok := b.mem.frontMetadata[id]
		if !ok {
			b.mem.mu.RUnlock()
			return nil, false, errCorruptFrontSnapshot
		}
		runtime, err := decodeFrontRuntime(runtimeRaw, id)
		if err != nil {
			b.mem.mu.RUnlock()
			return nil, false, errCorruptFrontSnapshot
		}
		meta, err := decodeFrontMetadata(metaRaw, id)
		if err != nil {
			b.mem.mu.RUnlock()
			return nil, false, errCorruptFrontSnapshot
		}
		node := composeFrontNode(runtime, meta)
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
	metaKeys := make([]string, 0, len(ids))
	smartKeys := make([]string, 0, len(ids))
	thermalKeys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, cachekeys.RedisKeyFrontNodeSnapshotPrefix+id)
		metaKeys = append(metaKeys, cachekeys.RedisKeyFrontNodeMetadataPrefix+id)
		smartKeys = append(smartKeys, cachekeys.RedisKeyFrontNodeSmartPrefix+id)
		thermalKeys = append(thermalKeys, cachekeys.RedisKeyFrontNodeThermalPrefix+id)
	}
	vals, err := b.redis.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, false, err
	}
	metaVals, err := b.redis.MGet(ctx, metaKeys...).Result()
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
		runtime, err := decodeFrontRuntime(raw, ids[i])
		if err != nil {
			if delErr := b.redis.Del(ctx, keys[i], smartKeys[i], thermalKeys[i]).Err(); delErr != nil {
				return nil, false, errors.Join(errCorruptFrontSnapshot, delErr)
			}
			return nil, false, errCorruptFrontSnapshot
		}
		metaRaw, ok := redisBytes(metaVals[i])
		if !ok {
			return nil, false, errCorruptFrontSnapshot
		}
		meta, err := decodeFrontMetadata(metaRaw, ids[i])
		if err != nil {
			if delErr := b.redis.Del(ctx, metaKeys[i]).Err(); delErr != nil {
				return nil, false, errors.Join(errCorruptFrontSnapshot, delErr)
			}
			return nil, false, errCorruptFrontSnapshot
		}
		n := composeFrontNode(runtime, meta)
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

func (b *memCacheBackend) hasNodeRuntime(_ context.Context, id int64) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	b.mem.mu.RLock()
	_, ok := b.mem.frontIDs[strconv.FormatInt(id, 10)]
	b.mem.mu.RUnlock()
	return ok, nil
}

func (b *redisCacheBackend) hasNodeRuntime(ctx context.Context, id int64) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	return b.redis.SIsMember(ctx, cachekeys.RedisKeyFrontNodeIDs, strconv.FormatInt(id, 10)).Result()
}

func (b *memCacheBackend) putNodeRuntime(_ context.Context, projection frontNodeProjection, invalidateCatalog bool) error {
	id, raw, err := frontRuntimePayload(projection)
	if err != nil {
		return err
	}
	smartRaw, hasSmart, err := frontSmartPayload(projection.Node)
	if err != nil {
		return err
	}
	thermalRaw, hasThermal, err := frontThermalPayload(projection.Node)
	if err != nil {
		return err
	}
	b.mem.mu.Lock()
	if invalidateCatalog {
		b.mem.frontCatalog = false
	}
	b.mem.frontRuntime[id] = raw
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

func (b *redisCacheBackend) putNodeRuntime(ctx context.Context, projection frontNodeProjection, invalidateCatalog bool) error {
	id, raw, err := frontRuntimePayload(projection)
	if err != nil {
		return err
	}
	smartRaw, hasSmart, err := frontSmartPayload(projection.Node)
	if err != nil {
		return err
	}
	thermalRaw, hasThermal, err := frontThermalPayload(projection.Node)
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
		if invalidateCatalog {
			pipe.Del(ctx, cachekeys.RedisKeyFrontMeta)
		}
		return nil
	})
	return err
}

func (b *memCacheBackend) removeNodeMetadata(_ context.Context, id int64) error {
	if id <= 0 {
		return nil
	}
	b.mem.mu.Lock()
	delete(b.mem.frontMetadata, strconv.FormatInt(id, 10))
	b.mem.mu.Unlock()
	return nil
}

func (b *redisCacheBackend) removeNodeMetadata(ctx context.Context, id int64) error {
	if id <= 0 {
		return nil
	}
	return b.redis.Del(ctx, cachekeys.RedisKeyFrontNodeMetadataPrefix+strconv.FormatInt(id, 10)).Err()
}

func (b *memCacheBackend) removeNodeSnapshot(_ context.Context, id int64) error {
	if id <= 0 {
		return nil
	}
	idStr := strconv.FormatInt(id, 10)
	b.mem.mu.Lock()
	delete(b.mem.frontRuntime, idStr)
	delete(b.mem.frontIDs, idStr)
	delete(b.mem.frontMetadata, idStr)
	delete(b.mem.frontSmart, idStr)
	delete(b.mem.frontThermal, idStr)
	delete(b.mem.frontGuestVisible, idStr)
	b.mem.frontCatalog = false
	b.mem.guestCatalog = false
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
			cachekeys.RedisKeyFrontNodeMetadataPrefix+idStr,
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

func (b *memCacheBackend) replaceSnapshot(_ context.Context, nodes []frontNodeProjection) error {
	payloads, err := frontSnapshotPayloads(nodes)
	if err != nil {
		return err
	}
	b.mem.mu.Lock()
	b.mem.frontCatalog = false
	runtime := make(map[string][]byte, len(payloads.runtime))
	thermal := make(map[string][]byte, len(payloads.runtime))
	for id, fallback := range payloads.runtime {
		existing, ok := b.mem.frontRuntime[id]
		if ok {
			if _, decodeErr := decodeFrontRuntime(existing, id); decodeErr == nil {
				runtime[id] = existing
				if raw, ok := b.mem.frontThermal[id]; ok {
					thermal[id] = raw
				}
				continue
			}
		}
		runtime[id] = fallback
		if raw, ok := payloads.thermal[id]; ok {
			thermal[id] = raw
		}
	}
	b.mem.frontRuntime = runtime
	b.mem.frontIDs = make(map[string]struct{}, len(payloads.runtime))
	for id := range payloads.runtime {
		b.mem.frontIDs[id] = struct{}{}
	}
	b.mem.frontMetadata = payloads.metadata
	b.mem.frontThermal = thermal
	b.mem.frontSmart = keepFrontRuntime(b.mem.frontSmart, payloads.runtime)
	b.mem.frontCatalog = true
	b.mem.mu.Unlock()
	return nil
}

func (b *redisCacheBackend) replaceSnapshot(ctx context.Context, nodes []frontNodeProjection) error {
	oldIDs, err := b.redis.SMembers(ctx, cachekeys.RedisKeyFrontNodeIDs).Result()
	if err != nil {
		return err
	}

	payloads, err := frontSnapshotPayloads(nodes)
	if err != nil {
		return err
	}

	removedKeys := make([]string, 0, len(oldIDs)*4)
	for _, id := range oldIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := payloads.runtime[id]; ok {
			continue
		}
		removedKeys = append(removedKeys, cachekeys.RedisKeyFrontNodeSnapshotPrefix+id)
		removedKeys = append(removedKeys, cachekeys.RedisKeyFrontNodeMetadataPrefix+id)
		removedKeys = append(removedKeys, cachekeys.RedisKeyFrontNodeSmartPrefix+id)
		removedKeys = append(removedKeys, cachekeys.RedisKeyFrontNodeThermalPrefix+id)
	}

	meta := cacheMetaValue(len(payloads.runtime))
	_, err = b.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, cachekeys.RedisKeyFrontMeta)
		pipe.Del(ctx, cachekeys.RedisKeyFrontNodeIDs)
		if len(removedKeys) > 0 {
			pipe.Del(ctx, removedKeys...)
		}
		if len(payloads.runtime) > 0 {
			members := make([]interface{}, 0, len(payloads.runtime))
			for id, raw := range payloads.runtime {
				pipe.SetNX(ctx, cachekeys.RedisKeyFrontNodeSnapshotPrefix+id, raw, 0)
				pipe.Set(ctx, cachekeys.RedisKeyFrontNodeMetadataPrefix+id, payloads.metadata[id], 0)
				members = append(members, id)
			}
			for id, raw := range payloads.thermal {
				pipe.SetNX(ctx, cachekeys.RedisKeyFrontNodeThermalPrefix+id, raw, 0)
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

type frontPayloads struct {
	runtime  map[string][]byte
	metadata map[string][]byte
	thermal  map[string][]byte
}

func frontSnapshotPayloads(nodes []frontNodeProjection) (frontPayloads, error) {
	out := frontPayloads{
		runtime:  make(map[string][]byte, len(nodes)),
		metadata: make(map[string][]byte, len(nodes)),
		thermal:  make(map[string][]byte, len(nodes)),
	}
	for _, node := range nodes {
		id, snapshotRaw, err := frontRuntimePayload(node)
		if err != nil {
			return frontPayloads{}, err
		}
		if _, exists := out.runtime[id]; exists {
			return frontPayloads{}, fmt.Errorf("%w: %s", errDuplicateFrontSnapshotID, id)
		}
		metaID, metaRaw, err := frontMetadataPayload(node.Meta)
		if err != nil {
			return frontPayloads{}, err
		}
		if metaID != id {
			return frontPayloads{}, errCorruptFrontSnapshot
		}
		out.runtime[id] = snapshotRaw
		out.metadata[id] = metaRaw
		thermalRaw, ok, err := frontThermalPayload(node.Node)
		if err != nil {
			return frontPayloads{}, err
		}
		if ok {
			out.thermal[id] = thermalRaw
		}
	}
	return out, nil
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

func (b *memCacheBackend) clearFrontMeta(_ context.Context) error {
	b.mem.mu.Lock()
	b.mem.frontCatalog = false
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
