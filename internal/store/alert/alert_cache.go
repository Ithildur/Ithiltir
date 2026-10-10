package alert

import (
	"context"

	"dash/internal/metrics"
)

func (s *Store) MarkServerDirty(id int64) {
	s.dirty.mark(id, nil)
}

func (s *Store) MarkServerMetrics(id int64, snapshot metrics.NodeView) {
	s.dirty.mark(id, &snapshot)
}

func (s *Store) NextDirtyServer(ctx context.Context) (int64, *metrics.NodeView, bool) {
	return s.dirty.next(ctx)
}

func (s *Store) FinishDirtyServer(id int64, retry bool) {
	s.dirty.finish(id, retry)
}
