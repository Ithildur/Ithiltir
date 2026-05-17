package alert

import (
	"context"
	"encoding/json"

	"dash/internal/model"

	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
)

const defaultAlertSettingsScope = "global"

func (s *Store) GetSettings(ctx context.Context) (*model.AlertSetting, error) {
	var item model.AlertSetting
	err := s.db.WithContext(ctx).
		Where("scope = ?", defaultAlertSettingsScope).
		First(&item).Error
	return &item, err
}

func (s *Store) UpsertSettings(ctx context.Context, enabled bool, ids []int64) error {
	payload, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	item := model.AlertSetting{
		Scope:      defaultAlertSettingsScope,
		Enabled:    enabled,
		ChannelIDs: datatypes.JSON(payload),
	}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "scope"}},
			DoUpdates: clause.AssignmentColumns([]string{"enabled", "channel_ids"}),
		}).
		Create(&item).Error
}
