package frontcache

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"dash/internal/metrics"
	"dash/internal/store/frontprojection"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

const projectionBuildAttempts = 3

var errProjectionChanged = errors.New("front cache projection changed while rebuilding")

type Store struct {
	db                *gorm.DB
	backend           cacheBackend
	projection        *frontprojection.Gate
	unknownMu         sync.Mutex
	unknownRuntime    map[int64]struct{}
	snapshotSF        singleflight.Group
	guestVisibilitySF singleflight.Group
}

func (s *Store) currentProjectionVersion() uint64 {
	return s.projection.Version()
}

func (s *Store) publishProjectionIfCurrent(version uint64, fn func() error) (bool, error) {
	return s.projection.Publish(version, fn)
}

func (s *Store) Validate() error {
	if s == nil || s.db == nil || s.backend == nil || s.projection == nil {
		return fmt.Errorf("store: front cache is not initialized")
	}
	return nil
}

type cacheBackend interface {
	loadSmartRuntimes(ctx context.Context, ids []int64) (map[int64]*frontSmartRuntime, error)
	fetchSnapshotCache(ctx context.Context) ([]metrics.NodeView, bool, error)
	hasNodeRuntime(ctx context.Context, id int64) (bool, error)
	putNodeRuntime(ctx context.Context, projection frontNodeProjection, invalidateCatalog bool) error
	removeNodeMetadata(ctx context.Context, id int64) error
	removeNodeSnapshot(ctx context.Context, id int64) error
	replaceSnapshot(ctx context.Context, nodes []frontNodeProjection) error
	clearFrontMeta(ctx context.Context) error
	loadGuestVisibleIDs(ctx context.Context, ids []int64) (map[int64]struct{}, bool, error)
	replaceGuestVisibleIDs(ctx context.Context, allowed map[int64]struct{}) error
	clearGuestVisibilityMeta(ctx context.Context) error
}

func New(db *gorm.DB, redisClient *redis.Client, projection *frontprojection.Gate) *Store {
	mem := newMemory()
	return &Store{
		db:             db,
		backend:        newCacheBackend(redisClient, mem),
		projection:     projection,
		unknownRuntime: make(map[int64]struct{}),
	}
}

func singleflightDetached[T any](ctx context.Context, sf *singleflight.Group, key string, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	// Cache rebuilds are shared work: once one caller starts the rebuild, do not
	// let that caller's cancellation fail every waiter. Each waiter still returns
	// on its own ctx cancellation below, while rebuild I/O is bounded by the
	// rebuild function's own timeouts.
	rebuildCtx := context.WithoutCancel(ctx)
	ch := sf.DoChan(key, func() (any, error) {
		return fn(rebuildCtx)
	})
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
