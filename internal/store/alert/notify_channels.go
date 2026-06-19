package alert

import (
	"context"
	"errors"

	"dash/internal/model"

	"gorm.io/gorm"
)

func (s *Store) ListChannels(ctx context.Context) ([]model.NotifyChannel, error) {
	var items []model.NotifyChannel
	err := s.db.WithContext(ctx).
		Where("is_deleted = ?", false).
		Order("id DESC").
		Find(&items).Error
	return items, err
}

func (s *Store) ListChannelsByIDs(ctx context.Context, ids []int64) ([]model.NotifyChannel, error) {
	if len(ids) == 0 {
		return []model.NotifyChannel{}, nil
	}
	var items []model.NotifyChannel
	err := s.db.WithContext(ctx).
		Where("id IN ? AND is_deleted = ?", ids, false).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

func (s *Store) ListDefaultNotifyChannels(ctx context.Context) ([]model.NotifyChannel, error) {
	settings, err := s.GetSettings(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return []model.NotifyChannel{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return []model.NotifyChannel{}, nil
	}
	ids, err := decodeChannelIDs(settings.ChannelIDs)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []model.NotifyChannel{}, nil
	}
	channels, err := s.ListChannelsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return enabledChannelsInOrder(ids, channels), nil
}

func (s *Store) GetChannel(ctx context.Context, id int64) (*model.NotifyChannel, error) {
	var item model.NotifyChannel
	err := s.db.WithContext(ctx).
		Where("id = ? AND is_deleted = ?", id, false).
		First(&item).Error
	return &item, err
}

func (s *Store) CreateChannel(ctx context.Context, item *model.NotifyChannel) error {
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *Store) ReplaceChannel(ctx context.Context, id int64, updates map[string]any) error {
	res := s.db.WithContext(ctx).
		Model(&model.NotifyChannel{}).
		Where("id = ? AND is_deleted = ?", id, false).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) DeleteChannel(ctx context.Context, id int64) error {
	res := s.db.WithContext(ctx).
		Model(&model.NotifyChannel{}).
		Where("id = ? AND is_deleted = ?", id, false).
		Update("is_deleted", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
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
