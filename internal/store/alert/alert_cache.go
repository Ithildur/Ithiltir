package alert

import (
	"context"

	"dash/internal/metrics"
)

func (s *Store) MarkServerDirty(id int64) {
	if s == nil {
		return
	}
	s.dirty.mark(id, nil)
}

func (s *Store) MarkServerMetrics(id int64, snapshot metrics.NodeView) {
	if s == nil {
		return
	}
	s.dirty.mark(id, &snapshot)
}

func (s *Store) NextDirtyServer(ctx context.Context) (int64, *metrics.NodeView, bool) {
	if s == nil {
		return 0, nil, false
	}
	return s.dirty.next(ctx)
}

func (s *Store) FinishDirtyServer(id int64, retry bool) {
	if s == nil {
		return
	}
	s.dirty.finish(id, retry)
}

func (s *Store) LoadAlertRuntime(_ context.Context, id int64) (map[string]string, error) {
	if id <= 0 || s == nil || s.runtime == nil {
		return map[string]string{}, nil
	}
	s.runtime.alertMu.Lock()
	defer s.runtime.alertMu.Unlock()
	current := s.runtime.alertRuntime[id]
	if len(current) == 0 {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(current))
	for key, value := range current {
		out[key] = value
	}
	return out, nil
}

func (s *Store) SaveAlertRuntime(_ context.Context, id int64, deletes []string, updates map[string][]byte, clear bool) error {
	if id <= 0 || s == nil || s.runtime == nil {
		return nil
	}
	s.runtime.alertMu.Lock()
	defer s.runtime.alertMu.Unlock()
	if clear {
		delete(s.runtime.alertRuntime, id)
		return nil
	}
	current := s.runtime.alertRuntime[id]
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
		delete(s.runtime.alertRuntime, id)
		return nil
	}
	s.runtime.alertRuntime[id] = current
	return nil
}

func (s *Store) ListAlertRuntimeServerIDs(context.Context) ([]int64, error) {
	if s == nil || s.runtime == nil {
		return nil, nil
	}
	s.runtime.alertMu.Lock()
	defer s.runtime.alertMu.Unlock()
	ids := make([]int64, 0, len(s.runtime.alertRuntime))
	for id := range s.runtime.alertRuntime {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
