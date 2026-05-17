package alert

import (
	"context"

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
