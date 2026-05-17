package mtlogin

import (
	"context"
	"errors"
	"time"

	"dash/internal/infra/cachekeys"

	"github.com/redis/go-redis/v9"
)

type redisMTProtoLoginBackend struct {
	redis *redis.Client
}

type memMTProtoLoginBackend struct {
	mem *memState
}

func newMTProtoLoginBackend(redisClient *redis.Client, mem *memState) mtprotoLoginBackend {
	if redisClient != nil {
		return &redisMTProtoLoginBackend{redis: redisClient}
	}
	return &memMTProtoLoginBackend{mem: mem}
}

func (s *Store) SetMTProtoLogin(ctx context.Context, id string, raw []byte, ttl time.Duration) error {
	if s == nil || s.backend == nil {
		return nil
	}
	return s.backend.setMTProtoLogin(ctx, id, raw, ttl)
}

func (s *Store) GetMTProtoLogin(ctx context.Context, id string) ([]byte, error) {
	if s == nil || s.backend == nil {
		return nil, nil
	}
	return s.backend.getMTProtoLogin(ctx, id)
}

func (s *Store) DeleteMTProtoLogin(ctx context.Context, id string) error {
	if s == nil || s.backend == nil {
		return nil
	}
	return s.backend.deleteMTProtoLogin(ctx, id)
}

func (b *memMTProtoLoginBackend) setMTProtoLogin(_ context.Context, id string, raw []byte, ttl time.Duration) error {
	if b == nil || b.mem == nil || ttl <= 0 {
		return nil
	}
	b.mem.mtprotoMu.Lock()
	b.mem.mtproto[id] = mtprotoEntry{
		raw:       append([]byte(nil), raw...),
		expiresAt: time.Now().UTC().Add(ttl),
	}
	b.mem.mtprotoMu.Unlock()
	return nil
}

func (b *redisMTProtoLoginBackend) setMTProtoLogin(ctx context.Context, id string, raw []byte, ttl time.Duration) error {
	if b == nil || b.redis == nil {
		return nil
	}
	return b.redis.Set(ctx, cachekeys.RedisKeyMTProtoLoginPrefix+id, raw, ttl).Err()
}

func (b *memMTProtoLoginBackend) getMTProtoLogin(_ context.Context, id string) ([]byte, error) {
	if b == nil || b.mem == nil {
		return nil, nil
	}
	now := time.Now().UTC()
	b.mem.mtprotoMu.Lock()
	defer b.mem.mtprotoMu.Unlock()
	item, ok := b.mem.mtproto[id]
	if !ok {
		return nil, nil
	}
	if !item.expiresAt.After(now) {
		delete(b.mem.mtproto, id)
		return nil, nil
	}
	return append([]byte(nil), item.raw...), nil
}

func (b *redisMTProtoLoginBackend) getMTProtoLogin(ctx context.Context, id string) ([]byte, error) {
	if b == nil || b.redis == nil {
		return nil, nil
	}
	raw, err := b.redis.Get(ctx, cachekeys.RedisKeyMTProtoLoginPrefix+id).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	return raw, nil
}

func (b *memMTProtoLoginBackend) deleteMTProtoLogin(_ context.Context, id string) error {
	if b == nil || b.mem == nil {
		return nil
	}
	b.mem.mtprotoMu.Lock()
	delete(b.mem.mtproto, id)
	b.mem.mtprotoMu.Unlock()
	return nil
}

func (b *redisMTProtoLoginBackend) deleteMTProtoLogin(ctx context.Context, id string) error {
	if b == nil || b.redis == nil {
		return nil
	}
	return b.redis.Del(ctx, cachekeys.RedisKeyMTProtoLoginPrefix+id).Err()
}
