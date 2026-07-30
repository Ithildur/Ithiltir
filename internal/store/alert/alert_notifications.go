package alert

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"dash/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func insertAlertNotifications(tx *gorm.DB, eventID int64, params []AlertNotificationParams, now time.Time) error {
	if eventID <= 0 || len(params) == 0 {
		return nil
	}
	items := make([]notificationParams, 0, len(params))
	for _, param := range params {
		if param.ChannelID <= 0 || param.Transition == "" || param.ChannelType == "" {
			continue
		}
		dedupeKey := alertNotificationDedupeKey(eventID, param.Transition, param.ChannelID)
		payload := param.Payload
		payload.Metadata = cloneStringMap(payload.Metadata)
		if payload.Metadata == nil {
			payload.Metadata = make(map[string]string, 3)
		}
		payload.Metadata["event_id"] = strconv.FormatInt(eventID, 10)
		payload.Metadata["transition"] = param.Transition
		payload.Metadata["dedupe_key"] = dedupeKey

		items = append(items, notificationParams{
			EventID:     &eventID,
			Transition:  param.Transition,
			ChannelID:   param.ChannelID,
			ChannelType: param.ChannelType,
			Payload:     payload,
			DedupeKey:   dedupeKey,
		})
	}
	return insertNotifications(tx, items, now)
}

type notificationParams struct {
	EventID     *int64
	Transition  string
	ChannelID   int64
	ChannelType model.NotifyType
	Payload     NotificationPayload
	DedupeKey   string
}

func insertNotifications(tx *gorm.DB, params []notificationParams, now time.Time) error {
	if len(params) == 0 {
		return nil
	}
	schedules, err := blockedNotificationSchedules(tx, params)
	if err != nil {
		return err
	}
	rows := make([]model.AlertNotificationOutbox, 0, len(params))
	for _, param := range params {
		if param.ChannelID <= 0 || param.ChannelType == "" || param.Transition == "" || param.DedupeKey == "" {
			continue
		}
		raw, err := marshalJSON(param.Payload)
		if err != nil {
			return fmt.Errorf("encode notification payload: %w", err)
		}
		status := model.OutboxStatusPending
		nextAttemptAt := now
		if nextProbeAt, ok := schedules[param.ChannelID]; ok {
			status = model.OutboxStatusBlocked
			nextAttemptAt = nextProbeAt
		}
		rows = append(rows, model.AlertNotificationOutbox{
			EventID:       param.EventID,
			Transition:    param.Transition,
			ChannelID:     param.ChannelID,
			ChannelType:   param.ChannelType,
			Payload:       raw,
			DedupeKey:     param.DedupeKey,
			Status:        status,
			NextAttemptAt: nextAttemptAt,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "dedupe_key"}},
			DoNothing: true,
		}).
		CreateInBatches(rows, 100).Error
}

func blockedNotificationSchedules(tx *gorm.DB, params []notificationParams) (map[int64]time.Time, error) {
	idSet := make(map[int64]struct{}, len(params))
	for _, param := range params {
		if param.ChannelID > 0 {
			idSet[param.ChannelID] = struct{}{}
		}
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	if len(ids) == 0 {
		return map[int64]time.Time{}, nil
	}
	var locked []int64
	if err := tx.
		Model(&model.NotifyChannel{}).
		Clauses(clause.Locking{Strength: "SHARE"}).
		Where("id IN ?", ids).
		Order("id ASC").
		Pluck("id", &locked).Error; err != nil {
		return nil, fmt.Errorf("lock notification channels for enqueue: %w", err)
	}

	type schedule struct {
		ChannelID   int64     `gorm:"column:channel_id"`
		NextProbeAt time.Time `gorm:"column:next_probe_at"`
	}
	var rows []schedule
	if err := tx.
		Model(&model.AlertNotificationOutbox{}).
		Select("channel_id, MIN(next_attempt_at) AS next_probe_at").
		Where("channel_id IN ? AND status = ?", ids, model.OutboxStatusBlocked).
		Group("channel_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load blocked notification schedules: %w", err)
	}
	out := make(map[int64]time.Time, len(rows))
	for _, row := range rows {
		out[row.ChannelID] = row.NextProbeAt
	}
	return out, nil
}

// EnqueueNotifications hands delivery ownership to the durable outbox. The
// caller's key identifies one logical message; channel IDs make each target
// independently idempotent and independently retryable.
func (s *Store) EnqueueNotifications(
	ctx context.Context,
	key, event string,
	channels []model.NotifyChannel,
	payload NotificationPayload,
	now time.Time,
) error {
	key = strings.TrimSpace(key)
	event = strings.TrimSpace(event)
	if key == "" || event == "" {
		return fmt.Errorf("enqueue notifications: key and event are required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	items := make([]notificationParams, 0, len(channels))
	for _, channel := range channels {
		dedupeKey := notificationDedupeKey(key, channel.ID)
		if len(dedupeKey) > 255 {
			return fmt.Errorf("enqueue notifications: dedupe key exceeds 255 bytes")
		}
		items = append(items, notificationParams{
			Transition:  event,
			ChannelID:   channel.ID,
			ChannelType: channel.Type,
			Payload:     payload,
			DedupeKey:   dedupeKey,
		})
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return insertNotifications(tx, items, now)
	})
}

func alertNotificationDedupeKey(eventID int64, transition string, channelID int64) string {
	return fmt.Sprintf("alert:%d:%s:%d", eventID, transition, channelID)
}

func notificationDedupeKey(key string, channelID int64) string {
	return fmt.Sprintf("%s:%d", key, channelID)
}

func (s *Store) TakeNextNotification(ctx context.Context, now time.Time) (*model.AlertNotificationOutbox, error) {
	var item model.AlertNotificationOutbox
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("(status IN ? AND next_attempt_at <= ?) OR status = ?",
				[]model.OutboxStatus{
					model.OutboxStatusPending,
					model.OutboxStatusRetry,
					model.OutboxStatusBlocked,
				}, now,
				model.OutboxStatusSending).
			Order("CASE WHEN status = 'sending' THEN 0 ELSE 1 END, next_attempt_at ASC, id ASC").
			Limit(1).
			Take(&item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		isProbe := item.IsProbe || item.Status == model.OutboxStatusBlocked
		if err := tx.Model(&model.AlertNotificationOutbox{}).
			Where("id = ?", item.ID).
			UpdateColumns(map[string]any{
				"status":          model.OutboxStatusSending,
				"is_probe":        isProbe,
				"leased_until":    now,
				"next_attempt_at": now,
			}).Error; err != nil {
			return err
		}
		item.Status = model.OutboxStatusSending
		item.IsProbe = isProbe
		item.LeasedUntil = &now
		return nil
	})
	if err != nil {
		return nil, err
	}
	if item.ID == 0 {
		return nil, nil
	}
	return &item, nil
}

func (s *Store) CompleteNotification(
	ctx context.Context,
	id, channelID int64,
	channelRevision int64,
	sentAt time.Time,
) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		channel, err := lockDeliveryChannel(tx, channelID)
		if err != nil {
			return err
		}
		marked, err := markNotificationSent(tx, id, channelID, sentAt)
		if err != nil {
			return err
		}
		if !marked {
			return nil
		}
		if channel.IsDeleted || channelVersionChanged(channel, channelRevision) {
			return nil
		}
		if err := recordChannelSuccess(tx, channelID, sentAt); err != nil {
			return err
		}
		if err := wakeChannelNotifications(
			tx,
			channelID,
			sentAt,
			[]model.OutboxStatus{model.OutboxStatusBlocked},
		); err != nil {
			return fmt.Errorf("wake blocked channel notifications: %w", err)
		}
		return nil
	})
}

func (s *Store) RetryNotification(ctx context.Context, failure NotificationFailure) error {
	return s.recordNotificationFailure(ctx, failure, model.OutboxStatusRetry, false)
}

func (s *Store) BlockNotification(ctx context.Context, failure NotificationFailure) error {
	return s.recordNotificationFailure(ctx, failure, model.OutboxStatusBlocked, true)
}

// RequeueNotification releases an in-flight item after a local worker or storage
// failure. It deliberately does not consume delivery budget or change channel
// health because no channel delivery result was observed.
func (s *Store) RequeueNotification(
	ctx context.Context,
	id, channelID int64,
	nextAttemptAt time.Time,
) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return releaseSendingNotification(tx, id, channelID, nextAttemptAt)
	})
}

func (s *Store) PauseNotification(ctx context.Context, failure NotificationFailure) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		channel, err := lockDeliveryChannel(tx, failure.ChannelID)
		if err != nil {
			return err
		}
		switch {
		case channel.IsDeleted:
			return updateSendingNotification(tx, failure.ID, failure.ChannelID, map[string]any{
				"status":       model.OutboxStatusDiscarded,
				"leased_until": nil,
				"failure_code": "channel_deleted",
				"last_error":   "notification channel deleted",
				"probe_count":  0,
			})
		case channel.Enabled:
			return retrySendingNotification(tx, failure.ID, failure.ChannelID, failure.FailedAt)
		default:
			return updateSendingNotification(
				tx,
				failure.ID,
				failure.ChannelID,
				notificationFailureUpdates(failure, model.OutboxStatusPaused),
			)
		}
	})
}

func (s *Store) DiscardNotification(ctx context.Context, failure NotificationFailure) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		channel, err := lockDeliveryChannel(tx, failure.ChannelID)
		if err != nil {
			return err
		}
		if !channel.IsDeleted && channelVersionChanged(channel, failure.ChannelRevision) {
			return retrySendingNotification(tx, failure.ID, failure.ChannelID, failure.FailedAt)
		}
		return updateSendingNotification(
			tx,
			failure.ID,
			failure.ChannelID,
			notificationFailureUpdates(failure, model.OutboxStatusDiscarded),
		)
	})
}

func (s *Store) recordNotificationFailure(
	ctx context.Context,
	failure NotificationFailure,
	status model.OutboxStatus,
	coalesce bool,
) error {
	if failure.ChannelRevision <= 0 {
		return fmt.Errorf("record notification failure: channel revision is required")
	}
	if failure.FailedAt.IsZero() {
		failure.FailedAt = time.Now().UTC()
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		channel, err := lockDeliveryChannel(tx, failure.ChannelID)
		if err != nil {
			return err
		}
		switch {
		case channel.IsDeleted:
			return updateSendingNotification(tx, failure.ID, failure.ChannelID, map[string]any{
				"status":       model.OutboxStatusDiscarded,
				"leased_until": nil,
				"failure_code": "channel_deleted",
				"last_error":   "notification channel deleted",
				"probe_count":  0,
			})
		case !channel.Enabled:
			return updateSendingNotification(tx, failure.ID, failure.ChannelID, map[string]any{
				"status":       model.OutboxStatusPaused,
				"leased_until": nil,
				"failure_code": "channel_disabled",
				"last_error":   nil,
				"probe_count":  0,
			})
		case channelVersionChanged(channel, failure.ChannelRevision):
			return retrySendingNotification(tx, failure.ID, failure.ChannelID, failure.FailedAt)
		}

		updates := notificationFailureUpdates(failure, status)
		updates["attempt_count"] = gorm.Expr("attempt_count + 1")
		if status == model.OutboxStatusBlocked {
			updates["probe_count"] = gorm.Expr("probe_count + 1")
		}
		if err := updateSendingNotification(tx, failure.ID, failure.ChannelID, updates); err != nil {
			return err
		}
		if coalesce {
			if err := tx.Model(&model.AlertNotificationOutbox{}).
				Where("channel_id = ? AND status IN ?", failure.ChannelID, []model.OutboxStatus{
					model.OutboxStatusPending,
					model.OutboxStatusRetry,
				}).
				UpdateColumns(map[string]any{
					"status":          model.OutboxStatusBlocked,
					"is_probe":        false,
					"next_attempt_at": failure.NextAttemptAt,
					"leased_until":    nil,
					"failure_code":    emptyToNil(failure.Code),
					"last_error":      emptyToNil(failure.LastError),
					"probe_count":     0,
				}).Error; err != nil {
				return fmt.Errorf("block queued channel notifications: %w", err)
			}
			if err := tx.Model(&model.AlertNotificationOutbox{}).
				Where("channel_id = ? AND status = ?", failure.ChannelID, model.OutboxStatusBlocked).
				UpdateColumn("next_attempt_at", failure.NextAttemptAt).Error; err != nil {
				return fmt.Errorf("postpone blocked channel notifications: %w", err)
			}
		}
		if err := recordChannelFailure(
			tx,
			failure.ChannelID,
			failure.Code,
			failure.LastError,
			failure.FailedAt,
		); err != nil {
			return err
		}
		return nil
	})
}

func notificationFailureUpdates(failure NotificationFailure, status model.OutboxStatus) map[string]any {
	updates := map[string]any{
		"status":       status,
		"leased_until": nil,
		"failure_code": emptyToNil(failure.Code),
		"last_error":   emptyToNil(failure.LastError),
	}
	if status != model.OutboxStatusBlocked {
		updates["probe_count"] = 0
	}
	if !failure.NextAttemptAt.IsZero() {
		updates["next_attempt_at"] = failure.NextAttemptAt
	}
	return updates
}

func updateSendingNotification(tx *gorm.DB, id, channelID int64, updates map[string]any) error {
	// is_probe is meaningful only while an item is in flight. PostgreSQL evaluates
	// the other update expressions against the old row, so callers may still use
	// it to choose the next state before this assignment clears the marker.
	updates["is_probe"] = false
	res := tx.Model(&model.AlertNotificationOutbox{}).
		Where("id = ? AND channel_id = ? AND status = ?", id, channelID, model.OutboxStatusSending).
		UpdateColumns(updates)
	if res.Error != nil {
		return fmt.Errorf("update sending notification: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotificationStateChanged
	}
	return nil
}

func markNotificationSent(tx *gorm.DB, id, channelID int64, sentAt time.Time) (bool, error) {
	err := updateSendingNotification(tx, id, channelID, map[string]any{
		"status":        model.OutboxStatusSent,
		"sent_at":       sentAt,
		"leased_until":  nil,
		"attempt_count": gorm.Expr("attempt_count + 1"),
		"probe_count":   0,
		"failure_code":  nil,
		"last_error":    nil,
	})
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, ErrNotificationStateChanged) {
		return false, err
	}

	var row struct {
		Status model.OutboxStatus `gorm:"column:status"`
	}
	if err := tx.Model(&model.AlertNotificationOutbox{}).
		Select("status").
		Where("id = ? AND channel_id = ?", id, channelID).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrNotificationStateChanged
		}
		return false, fmt.Errorf("load notification completion state: %w", err)
	}
	if row.Status == model.OutboxStatusSent {
		return false, nil
	}
	return false, ErrNotificationStateChanged
}

func releaseSendingNotification(tx *gorm.DB, id, channelID int64, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return updateSendingNotification(tx, id, channelID, map[string]any{
		"status":          gorm.Expr("CASE WHEN is_probe THEN ? ELSE ? END", model.OutboxStatusBlocked, model.OutboxStatusRetry),
		"next_attempt_at": now,
		"leased_until":    nil,
		"failure_code":    gorm.Expr("CASE WHEN is_probe THEN failure_code ELSE NULL END"),
		"last_error":      gorm.Expr("CASE WHEN is_probe THEN last_error ELSE NULL END"),
		"probe_count":     gorm.Expr("CASE WHEN is_probe THEN probe_count ELSE 0 END"),
	})
}

func retrySendingNotification(tx *gorm.DB, id, channelID int64, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return updateSendingNotification(tx, id, channelID, map[string]any{
		"status":          model.OutboxStatusRetry,
		"next_attempt_at": now,
		"leased_until":    nil,
		"failure_code":    nil,
		"last_error":      nil,
		"probe_count":     0,
	})
}

func lockDeliveryChannel(tx *gorm.DB, id int64) (model.NotifyChannel, error) {
	var channel model.NotifyChannel
	if err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).
		Take(&channel).Error; err != nil {
		return model.NotifyChannel{}, fmt.Errorf("lock notification channel: %w", err)
	}
	return channel, nil
}

func channelVersionChanged(channel model.NotifyChannel, observed int64) bool {
	return observed > 0 && channel.Revision != observed
}
