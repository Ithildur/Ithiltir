package frontcache

import (
	"context"
	"errors"
	"strconv"

	"github.com/redis/go-redis/v9"
)

var errCorruptCacheMeta = errors.New("corrupt cache meta")

func (b *redisCacheBackend) loadMeta(ctx context.Context, key string) (int, bool, error) {
	raw, err := b.redis.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	count, err := strconv.Atoi(string(raw))
	if err != nil || count < 0 {
		return 0, false, errCorruptCacheMeta
	}
	return count, true, nil
}

func cacheMetaValue(count int) string {
	return strconv.Itoa(count)
}
