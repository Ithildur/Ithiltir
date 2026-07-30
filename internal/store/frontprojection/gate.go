package frontprojection

import (
	"fmt"
	"sync"
)

// Gate coordinates PostgreSQL-derived projection changes with cache rebuild
// publication. It never holds its mutex while the caller performs database or
// cache I/O.
type Gate struct {
	mu      sync.Mutex
	cond    *sync.Cond
	version uint64
	active  int
	builds  int
}

func New() *Gate {
	g := &Gate{}
	g.cond = sync.NewCond(&g.mu)
	return g
}

// Mutate marks a projection-affecting operation. Concurrent mutations may run
// together; cache publication is excluded until every active mutation exits.
func (g *Gate) Mutate(fn func() error) error {
	if g == nil {
		return fmt.Errorf("front projection: gate is nil")
	}
	if fn == nil {
		return fmt.Errorf("front projection: mutation is nil")
	}

	g.mu.Lock()
	for g.builds != 0 {
		g.cond.Wait()
	}
	g.active++
	g.mu.Unlock()

	defer func() {
		g.mu.Lock()
		g.active--
		g.version++
		if g.active == 0 {
			g.cond.Broadcast()
		}
		g.mu.Unlock()
	}()
	return fn()
}

// Version returns a stable generation after all mutations that started before
// the call have completed.
func (g *Gate) Version() uint64 {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for g.active != 0 {
		g.cond.Wait()
	}
	return g.version
}

// Publish runs fn only when version is still current. Mutations wait for the
// publication to finish, but the gate mutex is released while fn performs I/O.
func (g *Gate) Publish(version uint64, fn func() error) (bool, error) {
	if g == nil {
		return false, fmt.Errorf("front projection: gate is nil")
	}
	if fn == nil {
		return false, fmt.Errorf("front projection: publication is nil")
	}

	g.mu.Lock()
	if g.active != 0 || g.version != version {
		g.mu.Unlock()
		return false, nil
	}
	g.builds++
	g.mu.Unlock()

	defer func() {
		g.mu.Lock()
		g.builds--
		if g.builds == 0 {
			g.cond.Broadcast()
		}
		g.mu.Unlock()
	}()
	if err := fn(); err != nil {
		return false, err
	}
	return true, nil
}
