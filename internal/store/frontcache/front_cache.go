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

	nodes, err = singleflightDetached(ctx, &s.snapshotSF, "rebuild", func(rebuildCtx context.Context) ([]metrics.NodeView, error) {
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
	for range projectionBuildAttempts {
		version := s.currentProjectionVersion()
		dbCtx, dbCancel := context.WithTimeout(ctx, dbTimeout)
		projections, err := s.fetchFrontProjections(dbCtx, staleAfterSec, 0, 0, true, 0, true)
		dbCancel()
		if err != nil {
			return nil, fmt.Errorf("fetch front nodes: %w", err)
		}
		if projections == nil {
			projections = make([]frontNodeProjection, 0)
		}

		cacheCtx, cacheCancel := context.WithTimeout(ctx, cacheTimeout)
		published, err := s.replaceFrontSnapshotIfCurrent(cacheCtx, projections, version)
		cacheCancel()
		if err != nil {
			return nil, fmt.Errorf("publish front snapshot: %w", err)
		}
		if published {
			cacheCtx, cacheCancel := context.WithTimeout(ctx, cacheTimeout)
			cached, ok, cacheErr := s.fetchSnapshotCache(cacheCtx)
			cacheCancel()
			if cacheErr == nil && ok {
				return cached, nil
			}
			nodes := make([]metrics.NodeView, 0, len(projections))
			for _, projection := range projections {
				nodes = append(nodes, projection.Node)
			}
			return nodes, nil
		}
	}
	return nil, errProjectionChanged
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
