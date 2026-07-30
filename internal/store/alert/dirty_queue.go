package alert

import (
	"context"
	"sync"

	"dash/internal/metrics"
)

type dirtyServerState struct {
	queued     bool
	running    bool
	dirtyAgain bool
	snapshot   *metrics.NodeView
}

type dirtyQueue struct {
	mu     sync.Mutex
	states map[int64]dirtyServerState
	ready  []int64
	head   int
	wakeup chan struct{}
}

func newDirtyQueue() *dirtyQueue {
	return &dirtyQueue{
		states: make(map[int64]dirtyServerState),
		wakeup: make(chan struct{}, 1),
	}
}

func (q *dirtyQueue) mark(id int64, snapshot *metrics.NodeView) {
	if q == nil || id <= 0 {
		return
	}

	q.mu.Lock()
	state, ok := q.states[id]
	if snapshot != nil {
		copy := *snapshot
		state.snapshot = &copy
	}
	switch {
	case !ok:
		state.queued = true
		q.states[id] = state
		q.ready = append(q.ready, id)
		q.signal()
	case state.running:
		state.dirtyAgain = true
		q.states[id] = state
	default:
		q.states[id] = state
	}
	q.mu.Unlock()
}

func (q *dirtyQueue) next(ctx context.Context) (int64, *metrics.NodeView, bool) {
	if q == nil {
		return 0, nil, false
	}
	for {
		q.mu.Lock()
		for q.head < len(q.ready) {
			id := q.ready[q.head]
			q.ready[q.head] = 0
			q.head++

			state, ok := q.states[id]
			if !ok || !state.queued {
				continue
			}
			state.queued = false
			state.running = true
			state.dirtyAgain = false
			q.states[id] = state
			q.compact()
			if q.head < len(q.ready) {
				q.signal()
			}
			q.mu.Unlock()
			return id, state.snapshot, true
		}
		q.ready = q.ready[:0]
		q.head = 0
		q.mu.Unlock()

		select {
		case <-ctx.Done():
			return 0, nil, false
		case <-q.wakeup:
		}
	}
}

func (q *dirtyQueue) finish(id int64, retry bool) {
	if q == nil || id <= 0 {
		return
	}

	q.mu.Lock()
	state, ok := q.states[id]
	if !ok || !state.running {
		q.mu.Unlock()
		return
	}
	state.running = false
	if retry || state.dirtyAgain {
		state.queued = true
		state.dirtyAgain = false
		q.states[id] = state
		q.ready = append(q.ready, id)
		q.signal()
	} else {
		delete(q.states, id)
	}
	q.mu.Unlock()
}

func (q *dirtyQueue) signal() {
	select {
	case q.wakeup <- struct{}{}:
	default:
	}
}

func (q *dirtyQueue) compact() {
	if q.head < 256 || q.head*2 < len(q.ready) {
		return
	}
	q.ready = append(q.ready[:0], q.ready[q.head:]...)
	q.head = 0
}
