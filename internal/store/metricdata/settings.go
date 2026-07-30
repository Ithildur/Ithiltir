package metricdata

import (
	"context"
	"fmt"

	"dash/internal/model"
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

func (s *Store) loadSettings(ctx context.Context) (model.MetricSetting, error) {
	var item model.MetricSetting
	err := s.db.WithContext(ctx).
		Where("id = ?", metricSettingsID).
		First(&item).Error
	if err != nil {
		return model.MetricSetting{}, fmt.Errorf("load metric settings: %w", err)
	}
	return item, nil
}

func (s *Store) saveSettings(ctx context.Context, mode HistoryGuestAccessMode) error {
	result := s.db.WithContext(ctx).
		Model(&model.MetricSetting{}).
		Where("id = ?", metricSettingsID).
		Update("history_guest_access_mode", string(mode))
	if result.Error != nil {
		return fmt.Errorf("save metric settings: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("save metric settings: singleton row is missing")
	}
	return nil
}

func (s *Store) GetHistoryGuestAccessMode(ctx context.Context) (HistoryGuestAccessMode, error) {
	item, err := s.loadSettings(ctx)
	if err != nil {
		return HistoryGuestAccessDisabled, err
	}

	mode := HistoryGuestAccessMode(item.HistoryGuestAccessMode)
	normalized, ok := NormalizeHistoryGuestAccessMode(mode)
	if !ok {
		return HistoryGuestAccessDisabled, fmt.Errorf("invalid history guest access mode: %s", mode)
	}
	return normalized, nil
}

func (s *Store) SetHistoryGuestAccessMode(ctx context.Context, mode HistoryGuestAccessMode) error {
	normalized, ok := NormalizeHistoryGuestAccessMode(mode)
	if !ok {
		return fmt.Errorf("invalid history guest access mode: %s", mode)
	}
	return s.saveSettings(ctx, normalized)
}
