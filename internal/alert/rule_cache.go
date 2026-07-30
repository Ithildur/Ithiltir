package alert

import (
	"context"
	"fmt"
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

func (c *RuleCache) Refresh(ctx context.Context, force bool) (*CompiledRules, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("alert rule store is not initialized")
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
		return nil, err
	}

	compiled, err := CompileRules(items, time.Now().UTC())
	if err != nil {
		if current := c.current.Load(); current != nil {
			return current, err
		}
		return nil, err
	}
	c.current.Store(compiled)
	return compiled, nil
}
