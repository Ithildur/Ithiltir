package node

import (
	"context"
	"fmt"
	"sync"

	"dash/internal/store/frontprojection"
	"golang.org/x/sync/semaphore"
)

type nodeLock struct {
	mu   *semaphore.Weighted
	refs int
}

type nodeMutations struct {
	structural sync.RWMutex
	locksMu    sync.Mutex
	locks      map[int64]*nodeLock
}

func (m *nodeMutations) global(projection *frontprojection.Gate, fn func() error) error {
	if projection == nil {
		return fmt.Errorf("store: front projection gate is nil")
	}
	if fn == nil {
		return fmt.Errorf("store: projection mutation is nil")
	}
	m.structural.Lock()
	defer m.structural.Unlock()
	return projection.Mutate(fn)
}

func (m *nodeMutations) projected(ctx context.Context, id int64, projection *frontprojection.Gate, fn func() error) error {
	if projection == nil {
		return fmt.Errorf("store: front projection gate is nil")
	}
	m.structural.RLock()
	defer m.structural.RUnlock()
	return m.runtime(ctx, id, func() error {
		return projection.Mutate(fn)
	})
}

func (m *nodeMutations) runtime(ctx context.Context, id int64, fn func() error) error {
	if id <= 0 {
		return fmt.Errorf("store: invalid node id %d", id)
	}
	if fn == nil {
		return fmt.Errorf("store: node mutation is nil")
	}

	lock := m.retain(id)
	defer m.release(id, lock)
	if err := lock.mu.Acquire(ctx, 1); err != nil {
		return err
	}
	defer lock.mu.Release(1)
	return fn()
}

func (m *nodeMutations) retain(id int64) *nodeLock {
	m.locksMu.Lock()
	if m.locks == nil {
		m.locks = make(map[int64]*nodeLock)
	}
	lock := m.locks[id]
	if lock == nil {
		lock = &nodeLock{mu: semaphore.NewWeighted(1)}
		m.locks[id] = lock
	}
	lock.refs++
	m.locksMu.Unlock()

	return lock
}

func (m *nodeMutations) release(id int64, lock *nodeLock) {
	m.locksMu.Lock()
	lock.refs--
	if lock.refs == 0 {
		delete(m.locks, id)
	}
	m.locksMu.Unlock()
}
