package frontcache

import (
	"context"

	"dash/internal/metrics"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

type Store struct {
	db                *gorm.DB
	backend           cacheBackend
	snapshotSF        singleflight.Group
	guestVisibilitySF singleflight.Group
}

type cacheBackend interface {
	loadFrontNodeSnapshot(ctx context.Context, id int64) (*metrics.NodeView, error)
	loadSmartRuntimes(ctx context.Context, ids []int64) (map[int64]*frontSmartRuntime, error)
	listFrontSnapshotIDs(ctx context.Context) ([]int64, error)
	fetchSnapshotCache(ctx context.Context) ([]metrics.NodeView, bool, error)
	putNodeSnapshot(ctx context.Context, node metrics.NodeView) error
	patchNodeSnapshot(ctx context.Context, id int64, name *string, order *int) error
	removeNodeSnapshot(ctx context.Context, id int64) error
	replaceSnapshot(ctx context.Context, nodes []metrics.NodeView) error
	clearFrontMeta(ctx context.Context) error
	loadGuestVisibleIDs(ctx context.Context, ids []int64) (map[int64]struct{}, bool, error)
	replaceGuestVisibleIDs(ctx context.Context, allowed map[int64]struct{}) error
	clearGuestVisibilityMeta(ctx context.Context) error
}

func New(db *gorm.DB, redisClient *redis.Client) *Store {
	mem := newMemory()
	return &Store{
		db:      db,
		backend: newCacheBackend(redisClient, mem),
	}
}

func singleflightDo[T any](ctx context.Context, sf *singleflight.Group, key string, fn func(context.Context) (T, error)) (T, error) {
	ch := sf.DoChan(key, func() (any, error) {
		return fn(context.WithoutCancel(ctx))
	})
	var zero T
	select {
	case res := <-ch:
		if res.Err != nil {
			return zero, res.Err
		}
		return res.Val.(T), nil
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}
