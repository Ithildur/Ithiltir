package alert

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"dash/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func insertAlertNotifications(tx *gorm.DB, eventID int64, params []AlertNotificationParams, now time.Time) error {
	if eventID <= 0 || len(params) == 0 {
		return nil
	}
	rows := make([]model.AlertNotificationOutbox, 0, len(params))
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

		raw, err := marshalJSON(payload)
		if err != nil {
			return err
		}
		rows = append(rows, model.AlertNotificationOutbox{
			EventID:       eventID,
			Transition:    param.Transition,
			ChannelID:     param.ChannelID,
			ChannelType:   param.ChannelType,
			Payload:       raw,
			DedupeKey:     dedupeKey,
			Status:        model.OutboxStatusPending,
			AttemptCount:  0,
			NextAttemptAt: now,
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

func alertNotificationDedupeKey(eventID int64, transition string, channelID int64) string {
	return fmt.Sprintf("alert:%d:%s:%d", eventID, transition, channelID)
}

func (s *Store) LeaseNextNotification(ctx context.Context, now, leaseUntil time.Time) (*model.AlertNotificationOutbox, error) {
	var item model.AlertNotificationOutbox
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("(status IN ? AND next_attempt_at <= ?) OR (status = ? AND leased_until <= ?)",
				[]model.OutboxStatus{model.OutboxStatusPending, model.OutboxStatusRetry}, now,
				model.OutboxStatusSending, now).
			Order("next_attempt_at ASC, id ASC").
			Limit(1).
			Take(&item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := tx.Model(&model.AlertNotificationOutbox{}).
			Where("id = ?", item.ID).
			Updates(map[string]any{
				"status":          model.OutboxStatusSending,
				"leased_until":    leaseUntil,
				"attempt_count":   gorm.Expr("attempt_count + 1"),
				"last_error":      nil,
				"next_attempt_at": now,
			}).Error; err != nil {
			return err
		}
		item.Status = model.OutboxStatusSending
		item.AttemptCount++
		item.LeasedUntil = &leaseUntil
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

func (s *Store) CompleteNotification(ctx context.Context, id int64, sentAt time.Time) error {
	return s.db.WithContext(ctx).
		Model(&model.AlertNotificationOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":       model.OutboxStatusSent,
			"sent_at":      sentAt,
			"leased_until": nil,
			"last_error":   nil,
		}).Error
}

func (s *Store) RetryNotification(ctx context.Context, id int64, nextAttemptAt time.Time, lastError string) error {
	return s.db.WithContext(ctx).
		Model(&model.AlertNotificationOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":          model.OutboxStatusRetry,
			"next_attempt_at": nextAttemptAt,
			"leased_until":    nil,
			"last_error":      emptyToNil(lastError),
		}).Error
}

func (s *Store) FailNotification(ctx context.Context, id int64, lastError string) error {
	return s.db.WithContext(ctx).
		Model(&model.AlertNotificationOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":       model.OutboxStatusFailedPermanent,
			"leased_until": nil,
			"last_error":   emptyToNil(lastError),
		}).Error
}
