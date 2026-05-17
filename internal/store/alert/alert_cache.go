package alert

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"dash/internal/infra/cachekeys"

	"github.com/redis/go-redis/v9"
)

var markDirtyServerScript = redis.NewScript(`
local added = redis.call('SADD', KEYS[1], ARGV[1])
if added == 1 then
  redis.call('LPUSH', KEYS[2], '1')
end
return added
`)

var claimDirtyServerScript = redis.NewScript(`
local member = redis.call('SPOP', KEYS[1])
if not member then
  return ''
end
redis.call('ZADD', KEYS[2], ARGV[1], member)
if redis.call('SCARD', KEYS[1]) > 0 then
  redis.call('LPUSH', KEYS[3], '1')
end
return member
`)

var requeueExpiredDirtyServersScript = redis.NewScript(`
local expired = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, ARGV[2])
local count = 0
for _, member in ipairs(expired) do
  if redis.call('ZREM', KEYS[1], member) == 1 then
    if redis.call('SADD', KEYS[2], member) == 1 then
      count = count + 1
    end
  end
end
if count > 0 then
  redis.call('LPUSH', KEYS[3], '1')
end
return count
`)

type redisAlertRuntimeBackend struct {
	redis *redis.Client
}

type memAlertRuntimeBackend struct {
	mem *memState
}

func newAlertRuntimeBackend(redisClient *redis.Client, mem *memState) alertRuntimeBackend {
	if redisClient != nil {
		return &redisAlertRuntimeBackend{redis: redisClient}
	}
	return &memAlertRuntimeBackend{mem: mem}
}

func (s *Store) MarkServerDirty(ctx context.Context, id int64) error {
	if s == nil || s.alert == nil {
		return nil
	}
	return s.alert.markServerDirty(ctx, id)
}

func (s *Store) ClaimDirtyServer(ctx context.Context, until time.Time) (int64, bool, error) {
	if s == nil || s.alert == nil {
		return 0, false, nil
	}
	return s.alert.claimDirtyServer(ctx, until)
}

func (s *Store) AckDirtyServer(ctx context.Context, id int64) error {
	if s == nil || s.alert == nil {
		return nil
	}
	return s.alert.ackDirtyServer(ctx, id)
}

func (s *Store) RequeueExpiredDirtyServers(ctx context.Context, now time.Time, limit int64) error {
	if s == nil || s.alert == nil {
		return nil
	}
	return s.alert.requeueExpiredDirtyServers(ctx, now, limit)
}

func (s *Store) WaitDirtyWakeup(ctx context.Context, timeout time.Duration) error {
	if s == nil || s.alert == nil {
		return nil
	}
	return s.alert.waitDirtyWakeup(ctx, timeout)
}

func (s *Store) LoadAlertRuntime(ctx context.Context, id int64) (map[string]string, error) {
	if s == nil || s.alert == nil {
		return map[string]string{}, nil
	}
	return s.alert.loadAlertRuntime(ctx, id)
}

func (s *Store) SaveAlertRuntime(ctx context.Context, id int64, deletes []string, updates map[string][]byte, clear bool) error {
	if s == nil || s.alert == nil {
		return nil
	}
	return s.alert.saveAlertRuntime(ctx, id, deletes, updates, clear)
}

func (s *Store) ListAlertRuntimeServerIDs(ctx context.Context) ([]int64, error) {
	if s == nil || s.alert == nil {
		return nil, nil
	}
	return s.alert.listAlertRuntimeServerIDs(ctx)
}

func (b *memAlertRuntimeBackend) markServerDirty(_ context.Context, id int64) error {
	if id <= 0 || b == nil || b.mem == nil {
		return nil
	}
	b.mem.alertMu.Lock()
	_, exists := b.mem.alertDirty[id]
	b.mem.alertDirty[id] = struct{}{}
	b.mem.alertMu.Unlock()
	if !exists {
		b.mem.wakeAlert()
	}
	return nil
}

func (b *redisAlertRuntimeBackend) markServerDirty(ctx context.Context, id int64) error {
	if id <= 0 || b == nil || b.redis == nil {
		return nil
	}
	_, err := markDirtyServerScript.Run(ctx, b.redis,
		[]string{cachekeys.RedisKeyAlertEvalDirtyServers, cachekeys.RedisKeyAlertEvalWakeup},
		strconv.FormatInt(id, 10),
	).Result()
	return err
}

func (b *memAlertRuntimeBackend) claimDirtyServer(_ context.Context, until time.Time) (int64, bool, error) {
	if b == nil || b.mem == nil {
		return 0, false, nil
	}
	b.mem.alertMu.Lock()
	defer b.mem.alertMu.Unlock()
	for id := range b.mem.alertDirty {
		delete(b.mem.alertDirty, id)
		b.mem.alertInflight[id] = until.UTC()
		if len(b.mem.alertDirty) > 0 {
			b.mem.wakeAlert()
		}
		return id, true, nil
	}
	return 0, false, nil
}

func (b *redisAlertRuntimeBackend) claimDirtyServer(ctx context.Context, until time.Time) (int64, bool, error) {
	if b == nil || b.redis == nil {
		return 0, false, nil
	}
	value, err := claimDirtyServerScript.Run(ctx, b.redis,
		[]string{cachekeys.RedisKeyAlertEvalDirtyServers, cachekeys.RedisKeyAlertEvalInflight, cachekeys.RedisKeyAlertEvalWakeup},
		strconv.FormatInt(until.UTC().UnixMilli(), 10),
	).Text()
	if err != nil {
		return 0, false, err
	}
	if value == "" {
		return 0, false, nil
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, false, nil
	}
	return id, true, nil
}

func (b *memAlertRuntimeBackend) ackDirtyServer(_ context.Context, id int64) error {
	if id <= 0 || b == nil || b.mem == nil {
		return nil
	}
	b.mem.alertMu.Lock()
	delete(b.mem.alertInflight, id)
	b.mem.alertMu.Unlock()
	return nil
}

func (b *redisAlertRuntimeBackend) ackDirtyServer(ctx context.Context, id int64) error {
	if id <= 0 || b == nil || b.redis == nil {
		return nil
	}
	return b.redis.ZRem(ctx, cachekeys.RedisKeyAlertEvalInflight, strconv.FormatInt(id, 10)).Err()
}

func (b *memAlertRuntimeBackend) requeueExpiredDirtyServers(_ context.Context, now time.Time, limit int64) error {
	if b == nil || b.mem == nil {
		return nil
	}
	if limit <= 0 {
		limit = 128
	}
	b.mem.alertMu.Lock()
	var count int64
	for id, until := range b.mem.alertInflight {
		if count >= limit || until.After(now.UTC()) {
			continue
		}
		delete(b.mem.alertInflight, id)
		if _, ok := b.mem.alertDirty[id]; ok {
			continue
		}
		b.mem.alertDirty[id] = struct{}{}
		count++
	}
	b.mem.alertMu.Unlock()
	if count > 0 {
		b.mem.wakeAlert()
	}
	return nil
}

func (b *redisAlertRuntimeBackend) requeueExpiredDirtyServers(ctx context.Context, now time.Time, limit int64) error {
	if b == nil || b.redis == nil {
		return nil
	}
	if limit <= 0 {
		limit = 128
	}
	_, err := requeueExpiredDirtyServersScript.Run(ctx, b.redis,
		[]string{cachekeys.RedisKeyAlertEvalInflight, cachekeys.RedisKeyAlertEvalDirtyServers, cachekeys.RedisKeyAlertEvalWakeup},
		strconv.FormatInt(now.UTC().UnixMilli(), 10),
		strconv.FormatInt(limit, 10),
	).Result()
	return err
}

func (b *memAlertRuntimeBackend) waitDirtyWakeup(ctx context.Context, timeout time.Duration) error {
	if b == nil || b.mem == nil {
		return nil
	}
	if timeout <= 0 {
		timeout = time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	case <-b.mem.alertWake:
		return nil
	}
}

func (b *redisAlertRuntimeBackend) waitDirtyWakeup(ctx context.Context, timeout time.Duration) error {
	if b == nil || b.redis == nil {
		return nil
	}
	_, err := b.redis.BRPop(ctx, timeout, cachekeys.RedisKeyAlertEvalWakeup).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	return err
}

func (b *memAlertRuntimeBackend) loadAlertRuntime(_ context.Context, id int64) (map[string]string, error) {
	if id <= 0 || b == nil || b.mem == nil {
		return map[string]string{}, nil
	}
	b.mem.alertMu.Lock()
	defer b.mem.alertMu.Unlock()
	current := b.mem.alertRuntime[id]
	if len(current) == 0 {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(current))
	for k, v := range current {
		out[k] = v
	}
	return out, nil
}

func (b *redisAlertRuntimeBackend) loadAlertRuntime(ctx context.Context, id int64) (map[string]string, error) {
	if id <= 0 || b == nil || b.redis == nil {
		return map[string]string{}, nil
	}
	return b.redis.HGetAll(ctx, alertRuntimeKey(id)).Result()
}

func (b *memAlertRuntimeBackend) saveAlertRuntime(_ context.Context, id int64, deletes []string, updates map[string][]byte, clear bool) error {
	if id <= 0 || b == nil || b.mem == nil {
		return nil
	}
	b.mem.alertMu.Lock()
	defer b.mem.alertMu.Unlock()
	if clear {
		delete(b.mem.alertRuntime, id)
		return nil
	}
	current := b.mem.alertRuntime[id]
	if current == nil {
		current = make(map[string]string)
	}
	for _, key := range deletes {
		delete(current, key)
	}
	for key, raw := range updates {
		current[key] = string(raw)
	}
	if len(current) == 0 {
		delete(b.mem.alertRuntime, id)
		return nil
	}
	b.mem.alertRuntime[id] = current
	return nil
}

func (b *redisAlertRuntimeBackend) saveAlertRuntime(ctx context.Context, id int64, deletes []string, updates map[string][]byte, clear bool) error {
	if id <= 0 || b == nil || b.redis == nil {
		return nil
	}
	key := alertRuntimeKey(id)
	pipe := b.redis.Pipeline()
	if len(deletes) > 0 {
		pipe.HDel(ctx, key, deletes...)
	}
	if len(updates) > 0 {
		values := make(map[string]any, len(updates))
		for k, v := range updates {
			values[k] = v
		}
		pipe.HSet(ctx, key, values)
	}
	if clear {
		pipe.Del(ctx, key)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (b *memAlertRuntimeBackend) listAlertRuntimeServerIDs(_ context.Context) ([]int64, error) {
	if b == nil || b.mem == nil {
		return nil, nil
	}
	b.mem.alertMu.Lock()
	defer b.mem.alertMu.Unlock()
	ids := make([]int64, 0, len(b.mem.alertRuntime))
	for id := range b.mem.alertRuntime {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (b *redisAlertRuntimeBackend) listAlertRuntimeServerIDs(ctx context.Context) ([]int64, error) {
	if b == nil || b.redis == nil {
		return nil, nil
	}
	ids := make([]int64, 0)
	seen := make(map[int64]struct{})
	var cursor uint64
	pattern := cachekeys.RedisKeyAlertStateServerPrefix + "*"
	for {
		keys, next, err := b.redis.Scan(ctx, cursor, pattern, 256).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			raw := strings.TrimPrefix(key, cachekeys.RedisKeyAlertStateServerPrefix)
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || id <= 0 {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
		cursor = next
		if cursor == 0 {
			return ids, nil
		}
	}
}

func alertRuntimeKey(id int64) string {
	return cachekeys.RedisKeyAlertStateServerPrefix + strconv.FormatInt(id, 10)
}
