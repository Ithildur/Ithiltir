package alert

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	alertstore "dash/internal/store/alert"
)

type RuleCache struct {
	store      *alertstore.Store
	minRefresh time.Duration
	mu         sync.Mutex
	current    atomic.Pointer[CompiledRules]
}

func NewRuleCache(st *alertstore.Store, minRefresh time.Duration) *RuleCache {
	return &RuleCache{
		store:      st,
		minRefresh: minRefresh,
	}
}

func (c *RuleCache) Get() *CompiledRules {
	if c == nil {
		return emptyRules(time.Now().UTC())
	}
	if current := c.current.Load(); current != nil {
		return current
	}
	return emptyRules(time.Now().UTC())
}

func (c *RuleCache) Refresh(ctx context.Context, force bool) (*CompiledRules, error) {
	if c == nil || c.store == nil {
		return emptyRules(time.Now().UTC()), nil
	}

	if !force {
		if current := c.current.Load(); current != nil && time.Since(current.RefreshedAt) < c.minRefresh {
			return current, nil
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !force {
		if current := c.current.Load(); current != nil && time.Since(current.RefreshedAt) < c.minRefresh {
			return current, nil
		}
	}

	items, err := c.store.RulesForCompile(ctx)
	if err != nil {
		if current := c.current.Load(); current != nil {
			return current, err
		}
		return emptyRules(time.Now().UTC()), err
	}

	compiled := CompileRules(items, time.Now().UTC())
	c.current.Store(compiled)
	return compiled, nil
}

func emptyRules(refreshedAt time.Time) *CompiledRules {
	return CompileRules(nil, refreshedAt)
}
