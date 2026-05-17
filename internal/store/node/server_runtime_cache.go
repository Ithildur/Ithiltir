package node

import (
	"context"
	"errors"
	"strconv"
	"time"

	"dash/internal/infra/cachekeys"

	"github.com/redis/go-redis/v9"
)

const (
	serverRuntimeFieldIP           = "ip"
	serverRuntimeFieldLastOnlineAt = "last_online_at"
)

type redisNodeRuntimeBackend struct {
	redis *redis.Client
}

type memNodeRuntimeBackend struct {
	mem *memState
}

func newNodeRuntimeBackend(redisClient *redis.Client, mem *memState) nodeRuntimeBackend {
	if redisClient != nil {
		return &redisNodeRuntimeBackend{redis: redisClient}
	}
	return &memNodeRuntimeBackend{mem: mem}
}

func (s *Store) GetServerRuntimeIP(ctx context.Context, serverID int64) (string, bool, error) {
	if s == nil || s.runtime == nil {
		return "", false, nil
	}
	return s.runtime.getServerRuntimeIP(ctx, serverID)
}

func (s *Store) SetServerRuntime(ctx context.Context, serverID int64, ip string, lastOnlineAt time.Time) error {
	if s == nil || s.runtime == nil {
		return nil
	}
	return s.runtime.setServerRuntime(ctx, serverID, ip, lastOnlineAt)
}

func (b *memNodeRuntimeBackend) getServerRuntimeIP(_ context.Context, serverID int64) (string, bool, error) {
	if serverID <= 0 || b == nil || b.mem == nil {
		return "", false, nil
	}
	b.mem.runtimeMu.RLock()
	state, ok := b.mem.runtime[serverID]
	b.mem.runtimeMu.RUnlock()
	if !ok || state.ip == "" {
		return "", false, nil
	}
	return state.ip, true, nil
}

func (b *redisNodeRuntimeBackend) getServerRuntimeIP(ctx context.Context, serverID int64) (string, bool, error) {
	if serverID <= 0 || b == nil || b.redis == nil {
		return "", false, nil
	}
	key := cachekeys.RedisKeyServerRuntimePrefix + strconv.FormatInt(serverID, 10)
	val, err := b.redis.HGet(ctx, key, serverRuntimeFieldIP).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", false, nil
		}
		return "", false, err
	}
	if val == "" {
		return "", false, nil
	}
	return val, true, nil
}

func (b *memNodeRuntimeBackend) setServerRuntime(_ context.Context, serverID int64, ip string, lastOnlineAt time.Time) error {
	if serverID <= 0 || b == nil || b.mem == nil {
		return nil
	}
	b.mem.runtimeMu.Lock()
	state := b.mem.runtime[serverID]
	if ip != "" {
		state.ip = ip
	}
	if !lastOnlineAt.IsZero() {
		state.lastOnlineAt = lastOnlineAt.UTC()
	}
	b.mem.runtime[serverID] = state
	b.mem.runtimeMu.Unlock()
	return nil
}

func (b *redisNodeRuntimeBackend) setServerRuntime(ctx context.Context, serverID int64, ip string, lastOnlineAt time.Time) error {
	if serverID <= 0 || b == nil || b.redis == nil {
		return nil
	}
	key := cachekeys.RedisKeyServerRuntimePrefix + strconv.FormatInt(serverID, 10)
	fields := make(map[string]any, 2)
	if ip != "" {
		fields[serverRuntimeFieldIP] = ip
	}
	if !lastOnlineAt.IsZero() {
		fields[serverRuntimeFieldLastOnlineAt] = lastOnlineAt.UTC().Format(time.RFC3339)
	}
	if len(fields) == 0 {
		return nil
	}
	return b.redis.HSet(ctx, key, fields).Err()
}
