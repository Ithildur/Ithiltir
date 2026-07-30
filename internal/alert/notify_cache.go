package alert

import (
	"context"
	"fmt"
	"sync"
	"time"

	"dash/internal/model"
	alertstore "dash/internal/store/alert"
)

type notifyTargets struct {
	Enabled     bool
	Channels    []model.NotifyChannel
	RefreshedAt time.Time
	Ready       bool
}

type notifyCache struct {
	store      *alertstore.Store
	minRefresh time.Duration
	mu         sync.Mutex
	current    notifyTargets
	ready      bool
}

func newNotifyCache(st *alertstore.Store, minRefresh time.Duration) *notifyCache {
	return &notifyCache{
		store:      st,
		minRefresh: minRefresh,
	}
}

func (c *notifyCache) Targets(ctx context.Context) (notifyTargets, error) {
	if c == nil || c.store == nil {
		return notifyTargets{}, fmt.Errorf("notification target store is not initialized")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ready && time.Since(c.current.RefreshedAt) < c.minRefresh {
		return c.current, nil
	}

	targets, err := c.load(ctx)
	if err != nil {
		if c.ready {
			return c.current, err
		}
		return notifyTargets{}, err
	}
	c.current = targets
	c.ready = true
	return targets, nil
}

func (c *notifyCache) load(ctx context.Context) (notifyTargets, error) {
	now := time.Now().UTC()
	settings, err := c.store.GetSettings(ctx)
	if err != nil {
		return notifyTargets{}, err
	}

	targets := notifyTargets{
		Enabled:     settings.Enabled,
		RefreshedAt: now,
	}
	if !settings.Enabled {
		targets.Ready = true
		return targets, nil
	}

	ids, err := alertstore.DecodeChannelIDs(settings.ChannelIDs)
	if err != nil || len(ids) == 0 {
		targets.Ready = err == nil
		return targets, err
	}
	channels, err := c.store.ListChannelsByIDs(ctx, ids)
	if err != nil {
		return notifyTargets{}, err
	}
	if len(channels) != len(ids) {
		return notifyTargets{}, alertstore.ErrUnknownChannel
	}
	targets.Channels = enabledChannelsInOrder(ids, channels)
	targets.Ready = true
	return targets, nil
}

func enabledChannelsInOrder(ids []int64, channels []model.NotifyChannel) []model.NotifyChannel {
	byID := make(map[int64]model.NotifyChannel, len(channels))
	for _, channel := range channels {
		if channel.Enabled && !channel.IsDeleted {
			byID[channel.ID] = channel
		}
	}
	out := make([]model.NotifyChannel, 0, len(byID))
	for _, id := range ids {
		if channel, ok := byID[id]; ok {
			out = append(out, channel)
		}
	}
	return out
}
