package alert

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"dash/internal/model"
	alertstore "dash/internal/store/alert"

	"gorm.io/gorm"
)

type notifyTargets struct {
	Enabled     bool
	Channels    []model.NotifyChannel
	RefreshedAt time.Time
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
		return defaultNotifyTargets(time.Now().UTC()), nil
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
		return defaultNotifyTargets(time.Now().UTC()), err
	}
	c.current = targets
	c.ready = true
	return targets, nil
}

func (c *notifyCache) load(ctx context.Context) (notifyTargets, error) {
	now := time.Now().UTC()
	settings, err := c.store.GetSettings(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultNotifyTargets(now), nil
	}
	if err != nil {
		return notifyTargets{}, err
	}

	targets := notifyTargets{
		Enabled:     settings.Enabled,
		RefreshedAt: now,
	}
	if !settings.Enabled {
		return targets, nil
	}

	ids, err := decodeNotifyChannelIDs(settings.ChannelIDs)
	if err != nil || len(ids) == 0 {
		return targets, err
	}
	channels, err := c.store.ListChannelsByIDs(ctx, ids)
	if err != nil {
		return notifyTargets{}, err
	}
	targets.Channels = enabledChannelsInOrder(ids, channels)
	return targets, nil
}

func defaultNotifyTargets(refreshedAt time.Time) notifyTargets {
	return notifyTargets{
		Enabled:     true,
		Channels:    []model.NotifyChannel{},
		RefreshedAt: refreshedAt,
	}
}

func decodeNotifyChannelIDs(raw []byte) ([]int64, error) {
	if len(raw) == 0 {
		return []int64{}, nil
	}
	var ids []int64
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, err
	}
	if ids == nil {
		return []int64{}, nil
	}
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
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
