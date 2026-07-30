package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"dash/internal/model"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChannelDeliveryStatus string

const (
	ChannelDeliveryUnknown  ChannelDeliveryStatus = "unknown"
	ChannelDeliveryHealthy  ChannelDeliveryStatus = "healthy"
	ChannelDeliveryDegraded ChannelDeliveryStatus = "degraded"
	ChannelDeliveryDisabled ChannelDeliveryStatus = "disabled"
)

type ChannelDelivery struct {
	Channel      model.NotifyChannel
	PendingCount int64
	BlockedCount int64
	NextRetryAt  *time.Time
	NextProbeAt  *time.Time
}

func (d ChannelDelivery) Status() ChannelDeliveryStatus {
	if !d.Channel.Enabled {
		return ChannelDeliveryDisabled
	}
	if d.Channel.ConsecutiveFailures > 0 || d.BlockedCount > 0 {
		return ChannelDeliveryDegraded
	}
	if d.Channel.LastSuccessAt != nil {
		return ChannelDeliveryHealthy
	}
	return ChannelDeliveryUnknown
}

func (s *Store) ListChannels(ctx context.Context) ([]model.NotifyChannel, error) {
	var items []model.NotifyChannel
	err := s.db.WithContext(ctx).
		Where("is_deleted = ?", false).
		Order("id DESC").
		Find(&items).Error
	return items, err
}

func (s *Store) ListChannelDeliveries(ctx context.Context) ([]ChannelDelivery, error) {
	channels, err := s.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	stats, err := s.channelQueueStats(ctx, channelIDs(channels))
	if err != nil {
		return nil, err
	}

	out := make([]ChannelDelivery, 0, len(channels))
	for _, channel := range channels {
		state := ChannelDelivery{Channel: channel}
		if queued, ok := stats[channel.ID]; ok {
			state.PendingCount = queued.PendingCount
			state.BlockedCount = queued.BlockedCount
			state.NextRetryAt = queued.NextRetryAt
			state.NextProbeAt = queued.NextProbeAt
		}
		out = append(out, state)
	}
	return out, nil
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

func (s *Store) GetChannelDelivery(ctx context.Context, id int64) (ChannelDelivery, error) {
	channel, err := s.GetChannel(ctx, id)
	if err != nil {
		return ChannelDelivery{}, err
	}
	stats, err := s.channelQueueStats(ctx, []int64{id})
	if err != nil {
		return ChannelDelivery{}, err
	}
	state := ChannelDelivery{Channel: *channel}
	if queued, ok := stats[id]; ok {
		state.PendingCount = queued.PendingCount
		state.BlockedCount = queued.BlockedCount
		state.NextRetryAt = queued.NextRetryAt
		state.NextProbeAt = queued.NextProbeAt
	}
	return state, nil
}

func (s *Store) CreateChannel(ctx context.Context, item *model.NotifyChannel) error {
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *Store) ReplaceChannel(ctx context.Context, id, revision int64, next model.NotifyChannel) error {
	if revision <= 0 {
		return fmt.Errorf("replace notification channel: invalid revision %d", revision)
	}
	return s.WithTx(ctx, func(tx *Store) error {
		current, err := tx.lockChannel(ctx, id)
		if err != nil {
			return err
		}
		if current.Revision != revision {
			return ErrChannelVersionStale
		}

		now := time.Now().UTC()
		if err := tx.db.WithContext(ctx).
			Model(&model.NotifyChannel{}).
			Where("id = ? AND is_deleted = ?", id, false).
			Updates(map[string]any{
				"name":                 next.Name,
				"type":                 next.Type,
				"config":               next.Config,
				"enabled":              next.Enabled,
				"revision":             gorm.Expr("revision + 1"),
				"last_success_at":      nil,
				"last_failure_at":      nil,
				"consecutive_failures": 0,
				"last_error_code":      nil,
				"last_error":           nil,
				"config_updated_at":    now,
			}).Error; err != nil {
			return fmt.Errorf("replace notification channel: %w", err)
		}

		switch {
		case current.Type != next.Type:
			return tx.discardChannelNotifications(ctx, id, "channel_type_changed", "notification channel type changed")
		case !next.Enabled:
			return tx.pauseChannelNotifications(ctx, id)
		default:
			return tx.wakeChannelNotifications(ctx, id, now, []model.OutboxStatus{
				model.OutboxStatusRetry,
				model.OutboxStatusBlocked,
				model.OutboxStatusPaused,
			})
		}
	})
}

func (s *Store) UpdateChannelConfig(ctx context.Context, id, revision int64, config datatypes.JSON) error {
	if revision <= 0 {
		return fmt.Errorf("update notification channel config: invalid revision %d", revision)
	}
	return s.WithTx(ctx, func(tx *Store) error {
		current, err := tx.lockChannel(ctx, id)
		if err != nil {
			return err
		}
		if current.Revision != revision {
			return ErrChannelVersionStale
		}

		now := time.Now().UTC()
		if err := tx.db.WithContext(ctx).
			Model(&model.NotifyChannel{}).
			Where("id = ? AND is_deleted = ?", id, false).
			Updates(map[string]any{
				"config":               config,
				"revision":             gorm.Expr("revision + 1"),
				"last_success_at":      nil,
				"last_failure_at":      nil,
				"consecutive_failures": 0,
				"last_error_code":      nil,
				"last_error":           nil,
				"config_updated_at":    now,
			}).Error; err != nil {
			return fmt.Errorf("update notification channel config: %w", err)
		}
		if !current.Enabled {
			return tx.pauseChannelNotifications(ctx, id)
		}
		return tx.wakeChannelNotifications(ctx, id, now, []model.OutboxStatus{
			model.OutboxStatusRetry,
			model.OutboxStatusBlocked,
			model.OutboxStatusPaused,
		})
	})
}

func (s *Store) SetChannelEnabled(ctx context.Context, id int64, enabled bool) error {
	return s.WithTx(ctx, func(tx *Store) error {
		channel, err := tx.lockChannel(ctx, id)
		if err != nil {
			return err
		}
		if channel.Enabled == enabled {
			return nil
		}
		updates := map[string]any{
			"enabled":           enabled,
			"revision":          gorm.Expr("revision + 1"),
			"config_updated_at": time.Now().UTC(),
		}
		if err := tx.db.WithContext(ctx).
			Model(&model.NotifyChannel{}).
			Where("id = ? AND is_deleted = ?", id, false).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("update notification channel status: %w", err)
		}
		if !enabled {
			return tx.pauseChannelNotifications(ctx, id)
		}
		return tx.wakeChannelNotifications(
			ctx,
			id,
			time.Now().UTC(),
			[]model.OutboxStatus{
				model.OutboxStatusPending,
				model.OutboxStatusRetry,
				model.OutboxStatusBlocked,
				model.OutboxStatusPaused,
			},
		)
	})
}

func (s *Store) DeleteChannel(ctx context.Context, id int64) error {
	return s.WithTx(ctx, func(tx *Store) error {
		settings, err := tx.lockSettings(ctx)
		if err != nil {
			return err
		}

		var channel model.NotifyChannel
		if err := tx.db.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND is_deleted = ?", id, false).
			Take(&channel).Error; err != nil {
			return err
		}

		ids, err := DecodeChannelIDs(settings.ChannelIDs)
		if err != nil {
			return fmt.Errorf("decode alert settings: %w", err)
		}
		next := ids[:0]
		for _, channelID := range ids {
			if channelID != id {
				next = append(next, channelID)
			}
		}
		if len(next) != len(ids) {
			payload, err := json.Marshal(next)
			if err != nil {
				return err
			}
			if err := tx.db.WithContext(ctx).
				Model(&model.AlertSetting{}).
				Where("scope = ?", defaultAlertSettingsScope).
				Update("channel_ids", datatypes.JSON(payload)).Error; err != nil {
				return err
			}
		}
		if err := tx.discardChannelNotifications(
			ctx,
			id,
			"channel_deleted",
			"notification channel deleted",
		); err != nil {
			return err
		}

		return tx.db.WithContext(ctx).
			Model(&model.NotifyChannel{}).
			Where("id = ? AND is_deleted = ?", id, false).
			Updates(map[string]any{
				"is_deleted":        true,
				"revision":          gorm.Expr("revision + 1"),
				"config_updated_at": time.Now().UTC(),
			}).Error
	})
}

func (s *Store) RecoverChannelNotifications(
	ctx context.Context,
	id, revision int64,
	recoveredAt time.Time,
) error {
	return s.WithTx(ctx, func(tx *Store) error {
		channel, err := tx.lockChannel(ctx, id)
		if err != nil {
			return err
		}
		if channelVersionChanged(channel, revision) {
			return nil
		}
		if err := recordChannelSuccess(tx.db.WithContext(ctx), id, recoveredAt); err != nil {
			return err
		}
		if !channel.Enabled {
			return nil
		}
		return tx.wakeChannelNotifications(
			ctx,
			id,
			recoveredAt,
			[]model.OutboxStatus{model.OutboxStatusBlocked},
		)
	})
}

func (s *Store) RecordChannelFailure(ctx context.Context, failure ChannelFailure) error {
	if failure.FailedAt.IsZero() {
		failure.FailedAt = time.Now().UTC()
	}
	return s.WithTx(ctx, func(tx *Store) error {
		channel, err := tx.lockChannel(ctx, failure.ID)
		if err != nil {
			return err
		}
		if channelVersionChanged(channel, failure.Revision) {
			return nil
		}
		return recordChannelFailure(
			tx.db.WithContext(ctx),
			failure.ID,
			failure.Code,
			failure.LastError,
			failure.FailedAt,
		)
	})
}

type channelQueueStat struct {
	ChannelID    int64      `gorm:"column:channel_id"`
	PendingCount int64      `gorm:"column:pending_count"`
	BlockedCount int64      `gorm:"column:blocked_count"`
	NextRetryAt  *time.Time `gorm:"column:next_retry_at"`
	NextProbeAt  *time.Time `gorm:"column:next_probe_at"`
}

func (s *Store) channelQueueStats(ctx context.Context, ids []int64) (map[int64]channelQueueStat, error) {
	out := make(map[int64]channelQueueStat, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	active := []model.OutboxStatus{
		model.OutboxStatusPending,
		model.OutboxStatusSending,
		model.OutboxStatusRetry,
		model.OutboxStatusBlocked,
		model.OutboxStatusPaused,
	}
	var rows []channelQueueStat
	if err := s.db.WithContext(ctx).
		Model(&model.AlertNotificationOutbox{}).
		Select(
			"channel_id, "+
				"COUNT(*) AS pending_count, "+
				"SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS blocked_count, "+
				"MIN(CASE WHEN status = ? THEN next_attempt_at END) AS next_retry_at, "+
				"MIN(CASE WHEN status = ? THEN next_attempt_at END) AS next_probe_at",
			model.OutboxStatusBlocked,
			model.OutboxStatusRetry,
			model.OutboxStatusBlocked,
		).
		Where("channel_id IN ? AND status IN ?", ids, active).
		Group("channel_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load notification channel queue state: %w", err)
	}
	for _, row := range rows {
		out[row.ChannelID] = row
	}
	return out, nil
}

func channelIDs(channels []model.NotifyChannel) []int64 {
	ids := make([]int64, 0, len(channels))
	for _, channel := range channels {
		ids = append(ids, channel.ID)
	}
	return ids
}

func (s *Store) lockChannel(ctx context.Context, id int64) (model.NotifyChannel, error) {
	var channel model.NotifyChannel
	if err := s.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND is_deleted = ?", id, false).
		Take(&channel).Error; err != nil {
		return model.NotifyChannel{}, err
	}
	return channel, nil
}

func (s *Store) pauseChannelNotifications(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).
		Model(&model.AlertNotificationOutbox{}).
		Where("channel_id = ? AND status IN ?", id, []model.OutboxStatus{
			model.OutboxStatusPending,
			model.OutboxStatusRetry,
			model.OutboxStatusBlocked,
		}).
		UpdateColumns(map[string]any{
			"status":       model.OutboxStatusPaused,
			"is_probe":     false,
			"leased_until": nil,
			"failure_code": "channel_disabled",
			"last_error":   nil,
			"probe_count":  0,
		}).Error
}

func (s *Store) wakeChannelNotifications(
	ctx context.Context,
	id int64,
	now time.Time,
	statuses []model.OutboxStatus,
) error {
	return wakeChannelNotifications(s.db.WithContext(ctx), id, now, statuses)
}

func wakeChannelNotifications(
	db *gorm.DB,
	id int64,
	now time.Time,
	statuses []model.OutboxStatus,
) error {
	if len(statuses) == 0 {
		return nil
	}
	return db.
		Model(&model.AlertNotificationOutbox{}).
		Where("channel_id = ? AND status IN ?", id, statuses).
		UpdateColumns(map[string]any{
			"status":          model.OutboxStatusRetry,
			"is_probe":        false,
			"attempt_count":   0,
			"next_attempt_at": now,
			"leased_until":    nil,
			"failure_code":    nil,
			"last_error":      nil,
			"probe_count":     0,
		}).Error
}

func (s *Store) discardChannelNotifications(ctx context.Context, id int64, code, message string) error {
	return s.db.WithContext(ctx).
		Model(&model.AlertNotificationOutbox{}).
		Where("channel_id = ? AND status IN ?", id, []model.OutboxStatus{
			model.OutboxStatusPending,
			model.OutboxStatusRetry,
			model.OutboxStatusBlocked,
			model.OutboxStatusPaused,
		}).
		UpdateColumns(map[string]any{
			"status":       model.OutboxStatusDiscarded,
			"is_probe":     false,
			"leased_until": nil,
			"failure_code": code,
			"last_error":   emptyToNil(message),
			"probe_count":  0,
		}).Error
}

func recordChannelSuccess(tx *gorm.DB, id int64, at time.Time) error {
	if err := tx.Model(&model.NotifyChannel{}).
		Where("id = ? AND is_deleted = ?", id, false).
		UpdateColumns(map[string]any{
			"last_success_at":      at,
			"consecutive_failures": 0,
			"last_error_code":      nil,
			"last_error":           nil,
		}).Error; err != nil {
		return fmt.Errorf("record channel delivery success: %w", err)
	}
	return nil
}

func recordChannelFailure(tx *gorm.DB, id int64, code, message string, at time.Time) error {
	if err := tx.Model(&model.NotifyChannel{}).
		Where("id = ? AND is_deleted = ?", id, false).
		UpdateColumns(map[string]any{
			"last_failure_at":      at,
			"consecutive_failures": gorm.Expr("consecutive_failures + 1"),
			"last_error_code":      emptyToNil(code),
			"last_error":           emptyToNil(message),
		}).Error; err != nil {
		return fmt.Errorf("record channel delivery failure: %w", err)
	}
	return nil
}
