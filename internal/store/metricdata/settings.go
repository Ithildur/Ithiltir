package metricdata

import (
	"context"
	"errors"
	"fmt"

	"dash/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const metricSettingsID int16 = 1

type HistoryGuestAccessMode string

const (
	HistoryGuestAccessDisabled HistoryGuestAccessMode = "disabled"
	HistoryGuestAccessByNode   HistoryGuestAccessMode = "by_node"
)

func NormalizeHistoryGuestAccessMode(mode HistoryGuestAccessMode) (HistoryGuestAccessMode, bool) {
	switch mode {
	case HistoryGuestAccessDisabled:
		return HistoryGuestAccessDisabled, true
	case HistoryGuestAccessByNode:
		return HistoryGuestAccessByNode, true
	default:
		return HistoryGuestAccessDisabled, false
	}
}

func defaultMetricSetting() model.MetricSetting {
	return model.MetricSetting{
		ID:                     metricSettingsID,
		HistoryGuestAccessMode: string(HistoryGuestAccessDisabled),
	}
}

func (s *Store) loadSettings(ctx context.Context) (model.MetricSetting, error) {
	var item model.MetricSetting
	err := s.db.WithContext(ctx).
		Where("id = ?", metricSettingsID).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultMetricSetting(), nil
		}
		return model.MetricSetting{}, fmt.Errorf("load metric settings: %w", err)
	}
	return item, nil
}

func (s *Store) saveSettings(ctx context.Context, item model.MetricSetting) error {
	item.ID = metricSettingsID
	err := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"history_guest_access_mode"}),
		}).
		Create(&item).Error
	if err != nil {
		return fmt.Errorf("save metric settings: %w", err)
	}
	return nil
}

func (s *Store) GetHistoryGuestAccessMode(ctx context.Context) (HistoryGuestAccessMode, error) {
	item, err := s.loadSettings(ctx)
	if err != nil {
		return HistoryGuestAccessDisabled, err
	}

	mode := HistoryGuestAccessMode(item.HistoryGuestAccessMode)
	normalized, _ := NormalizeHistoryGuestAccessMode(mode)
	if normalized != mode {
		return HistoryGuestAccessDisabled, fmt.Errorf("invalid history guest access mode: %s", mode)
	}
	return normalized, nil
}

func (s *Store) SetHistoryGuestAccessMode(ctx context.Context, mode HistoryGuestAccessMode) error {
	normalized, ok := NormalizeHistoryGuestAccessMode(mode)
	if !ok {
		return fmt.Errorf("invalid history guest access mode: %s", mode)
	}
	return s.saveSettings(ctx, model.MetricSetting{
		HistoryGuestAccessMode: string(normalized),
	})
}
