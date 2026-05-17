package alert

import (
	"context"
	"errors"
	"time"

	"dash/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Store) EnqueueControlTask(ctx context.Context, taskType, dedupeKey string, payload any) error {
	return s.enqueueControlTask(ctx, taskType, dedupeKey, payload)
}

func (s *Store) EnqueueFullReconcileTask(ctx context.Context, dedupeKey string) error {
	return s.enqueueControlTask(ctx, ControlTaskFullReconcile, dedupeKey, map[string]any{})
}

func (s *Store) LeaseNextControlTask(ctx context.Context, now, leaseUntil time.Time) (*model.AlertControlTask, error) {
	var item model.AlertControlTask
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("(status = ? AND available_at <= ?) OR (status = ? AND leased_until <= ?)",
				model.TaskStatusPending, now,
				model.TaskStatusLeased, now).
			Order("available_at ASC, id ASC").
			Limit(1).
			Take(&item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := tx.Model(&model.AlertControlTask{}).
			Where("id = ?", item.ID).
			Updates(map[string]any{
				"status":        model.TaskStatusLeased,
				"leased_until":  leaseUntil,
				"attempt_count": gorm.Expr("attempt_count + 1"),
				"last_error":    nil,
			}).Error; err != nil {
			return err
		}
		item.Status = model.TaskStatusLeased
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

func (s *Store) CompleteControlTask(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&model.AlertControlTask{}, "id = ?", id).Error
}

func (s *Store) RetryControlTask(ctx context.Context, id int64, availableAt time.Time, lastError string) error {
	return s.db.WithContext(ctx).
		Model(&model.AlertControlTask{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":       model.TaskStatusPending,
			"available_at": availableAt,
			"leased_until": nil,
			"last_error":   emptyToNil(lastError),
		}).Error
}

func (s *Store) enqueueControlTask(ctx context.Context, taskType, dedupeKey string, payload any) error {
	raw, err := marshalJSON(payload)
	if err != nil {
		return err
	}
	item := model.AlertControlTask{
		TaskType:     taskType,
		DedupeKey:    dedupeKey,
		Payload:      raw,
		Status:       model.TaskStatusPending,
		AttemptCount: 0,
		AvailableAt:  time.Now().UTC(),
	}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "dedupe_key"}},
			DoNothing: true,
		}).
		Create(&item).Error
}
