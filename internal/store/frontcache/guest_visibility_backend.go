package frontcache

import (
	"context"
	"errors"
	"strconv"

	"dash/internal/infra/cachekeys"

	"github.com/redis/go-redis/v9"
)

var errCorruptGuestVisibility = errors.New("corrupt guest visibility")

func (b *memCacheBackend) loadGuestVisibleIDs(_ context.Context, ids []int64) (map[int64]struct{}, bool, error) {
	b.mem.mu.RLock()
	defer b.mem.mu.RUnlock()
	if !b.mem.guestVisibleMeta {
		return nil, false, nil
	}
	out := make(map[int64]struct{}, len(ids))
	if len(ids) == 0 {
		return out, true, nil
	}
	for _, id := range ids {
		if _, ok := b.mem.frontGuestVisible[strconv.FormatInt(id, 10)]; ok {
			out[id] = struct{}{}
		}
	}
	return out, true, nil
}

func (b *redisCacheBackend) loadGuestVisibleIDs(ctx context.Context, ids []int64) (map[int64]struct{}, bool, error) {
	wantCount, ok, err := b.loadMeta(ctx, cachekeys.RedisKeyGuestVisibilityMeta)
	if errors.Is(err, errCorruptCacheMeta) {
		return nil, false, errCorruptGuestVisibility
	}
	if err != nil || !ok {
		return nil, false, err
	}
	gotCount, err := b.redis.SCard(ctx, cachekeys.RedisKeyGuestVisibleIDs).Result()
	if err != nil {
		return nil, false, err
	}
	if int(gotCount) != wantCount {
		return nil, false, errCorruptGuestVisibility
	}
	out := make(map[int64]struct{}, len(ids))
	if len(ids) == 0 {
		return out, true, nil
	}
	members := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		members = append(members, strconv.FormatInt(id, 10))
	}
	hits, err := b.redis.SMIsMember(ctx, cachekeys.RedisKeyGuestVisibleIDs, members...).Result()
	if err != nil {
		return nil, false, err
	}
	if len(hits) != len(ids) {
		return nil, false, errors.New("redis smismember length mismatch")
	}
	for i, ok := range hits {
		if ok {
			out[ids[i]] = struct{}{}
		}
	}
	return out, true, nil
}

func (b *memCacheBackend) replaceGuestVisibleIDs(_ context.Context, allowed map[int64]struct{}) error {
	guest := make(map[string]struct{}, len(allowed))
	for id := range allowed {
		guest[strconv.FormatInt(id, 10)] = struct{}{}
	}
	b.mem.mu.Lock()
	b.mem.guestVisibleMeta = false
	b.mem.frontGuestVisible = guest
	b.mem.guestVisibleMeta = true
	b.mem.mu.Unlock()
	return nil
}

func (b *redisCacheBackend) replaceGuestVisibleIDs(ctx context.Context, allowed map[int64]struct{}) error {
	meta := cacheMetaValue(len(allowed))
	_, err := b.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, cachekeys.RedisKeyGuestVisibilityMeta)
		pipe.Del(ctx, cachekeys.RedisKeyGuestVisibleIDs)
		if len(allowed) > 0 {
			members := make([]interface{}, 0, len(allowed))
			for id := range allowed {
				members = append(members, strconv.FormatInt(id, 10))
			}
			pipe.SAdd(ctx, cachekeys.RedisKeyGuestVisibleIDs, members...)
		}
		pipe.Set(ctx, cachekeys.RedisKeyGuestVisibilityMeta, meta, 0)
		return nil
	})
	return err
}

func (b *memCacheBackend) clearGuestVisibilityMeta(_ context.Context) error {
	b.mem.mu.Lock()
	b.mem.guestVisibleMeta = false
	b.mem.mu.Unlock()
	return nil
}

func (b *redisCacheBackend) clearGuestVisibilityMeta(ctx context.Context) error {
	return b.redis.Del(ctx, cachekeys.RedisKeyGuestVisibilityMeta).Err()
}
