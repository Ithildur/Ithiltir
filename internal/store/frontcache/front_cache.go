package frontcache

import (
	"context"
	"fmt"
	"time"

	"dash/internal/metrics"
)

type FrontSnapshotOptions struct {
	CacheTimeout  time.Duration
	BuildTimeout  time.Duration
	StaleAfterSec int
}

// EnsureSnapshot coordinates fetch and rebuild for the full front metrics cache.
func (s *Store) EnsureSnapshot(ctx context.Context, opts FrontSnapshotOptions) ([]metrics.NodeView, error) {
	nodes, ok, err := s.loadSnapshot(ctx, opts.CacheTimeout)
	if err != nil {
		return nil, err
	}
	if ok {
		return nodes, nil
	}

	nodes, err = singleflightDo(ctx, &s.snapshotSF, "rebuild", func(rebuildCtx context.Context) ([]metrics.NodeView, error) {
		return s.rebuildSnapshot(rebuildCtx, opts.BuildTimeout, opts.CacheTimeout, opts.StaleAfterSec)
	})
	if err != nil {
		return nil, err
	}

	return nodes, nil
}

func (s *Store) loadSnapshot(ctx context.Context, timeout time.Duration) ([]metrics.NodeView, bool, error) {
	cacheCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return s.fetchSnapshotCache(cacheCtx)
}

func (s *Store) rebuildSnapshot(ctx context.Context, dbTimeout, cacheTimeout time.Duration, staleAfterSec int) ([]metrics.NodeView, error) {
	dbCtx, dbCancel := context.WithTimeout(ctx, dbTimeout)
	defer dbCancel()

	nodes, err := s.FetchFrontNodes(dbCtx, staleAfterSec, 0, 0, true)
	if err != nil {
		return nil, fmt.Errorf("fetch front nodes: %w", err)
	}
	if nodes == nil {
		nodes = make([]metrics.NodeView, 0)
	}

	cacheCtx, cacheCancel := context.WithTimeout(ctx, cacheTimeout)
	defer cacheCancel()

	if err := s.replaceFrontSnapshot(cacheCtx, nodes); err != nil {
		return nil, fmt.Errorf("publish front snapshot: %w", err)
	}

	return nodes, nil
}

func (s *Store) applySmartRuntimeFields(ctx context.Context, nodes []metrics.NodeView) error {
	ids := make([]int64, 0, len(nodes))
	for i := range nodes {
		id, ok := metrics.ParseNodeID(nodes[i].Node.ID)
		if !ok {
			return fmt.Errorf("%w: %q", errInvalidFrontSnapshotID, nodes[i].Node.ID)
		}
		ids = append(ids, id)
	}
	runtimes, err := s.backend.loadSmartRuntimes(ctx, ids)
	if err != nil {
		return err
	}
	for i, id := range ids {
		applySmartRuntime(&nodes[i], runtimes[id])
	}
	return nil
}
