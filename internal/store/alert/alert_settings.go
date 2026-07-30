package alert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"dash/internal/model"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const defaultAlertSettingsScope = "global"

var ErrUnknownChannel = errors.New("alert settings contains an unknown channel")

func (s *Store) GetSettings(ctx context.Context) (*model.AlertSetting, error) {
	if err := s.ensureSettings(ctx); err != nil {
		return nil, err
	}
	var item model.AlertSetting
	err := s.db.WithContext(ctx).
		Where("scope = ?", defaultAlertSettingsScope).
		First(&item).Error
	return &item, err
}

func (s *Store) ReplaceSettings(ctx context.Context, enabled bool, ids []int64) error {
	payload, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	if _, err := DecodeChannelIDs(payload); err != nil {
		return err
	}
	return s.WithTx(ctx, func(tx *Store) error {
		if _, err := tx.lockSettings(ctx); err != nil {
			return err
		}
		if len(ids) > 0 {
			var channels []model.NotifyChannel
			if err := tx.db.WithContext(ctx).
				Model(&model.NotifyChannel{}).
				Select("id").
				Clauses(clause.Locking{Strength: "SHARE"}).
				Where("id IN ? AND is_deleted = ?", ids, false).
				Find(&channels).Error; err != nil {
				return err
			}
			if len(channels) != len(ids) {
				return ErrUnknownChannel
			}
		}

		res := tx.db.WithContext(ctx).
			Model(&model.AlertSetting{}).
			Where("scope = ?", defaultAlertSettingsScope).
			Updates(map[string]any{
				"enabled":     enabled,
				"channel_ids": datatypes.JSON(payload),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (s *Store) lockSettings(ctx context.Context) (*model.AlertSetting, error) {
	if err := s.ensureSettings(ctx); err != nil {
		return nil, err
	}
	var item model.AlertSetting
	err := s.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("scope = ?", defaultAlertSettingsScope).
		Take(&item).Error
	if err != nil {
		return nil, fmt.Errorf("lock alert settings: %w", err)
	}
	return &item, nil
}

func (s *Store) ensureSettings(ctx context.Context) error {
	var existing model.AlertSetting
	err := s.db.WithContext(ctx).
		Select("id").
		Where("scope = ?", defaultAlertSettingsScope).
		Take(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	item := model.AlertSetting{
		Scope:      defaultAlertSettingsScope,
		Enabled:    true,
		ChannelIDs: datatypes.JSON("[]"),
	}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "scope"}},
			DoNothing: true,
		}).
		Create(&item).Error
}
