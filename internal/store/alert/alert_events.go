package alert

import (
	"context"
	"errors"
	"time"

	"dash/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Store) ListOpenEvents(ctx context.Context) ([]model.AlertEvent, error) {
	var items []model.AlertEvent
	err := s.db.WithContext(ctx).
		Where("status = ?", model.AlertStatusOpen).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

func (s *Store) ListOpenEventsForDeletedServers(ctx context.Context) ([]model.AlertEvent, error) {
	var items []model.AlertEvent
	err := s.db.WithContext(ctx).
		Model(&model.AlertEvent{}).
		Joins("JOIN servers ON servers.id = alert_events.object_id AND servers.is_deleted = TRUE").
		Where("alert_events.object_type = ? AND alert_events.status = ?", model.ObjectTypeServer, model.AlertStatusOpen).
		Order("alert_events.id ASC").
		Find(&items).Error
	return items, err
}

func (s *Store) ListOpenEventsByRule(ctx context.Context, ruleID, generation int64) ([]model.AlertEvent, error) {
	var items []model.AlertEvent
	err := s.db.WithContext(ctx).
		Where("rule_id = ? AND rule_generation = ? AND status = ?", ruleID, generation, model.AlertStatusOpen).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

func (s *Store) ListOpenObjectIDs(ctx context.Context, objectType model.ObjectType) ([]int64, error) {
	var ids []int64
	err := s.db.WithContext(ctx).
		Model(&model.AlertEvent{}).
		Distinct("object_id").
		Where("object_type = ? AND status = ?", objectType, model.AlertStatusOpen).
		Order("object_id ASC").
		Pluck("object_id", &ids).Error
	return ids, err
}

func (s *Store) TouchOpenEvent(ctx context.Context, eventID int64, triggeredAt time.Time, currentValue, effectiveThreshold float64) (bool, error) {
	if eventID <= 0 {
		return false, nil
	}
	res := s.db.WithContext(ctx).
		Model(&model.AlertEvent{}).
		Where("id = ? AND status = ?", eventID, model.AlertStatusOpen).
		Updates(map[string]any{
			"last_trigger_at":     triggeredAt,
			"current_value":       currentValue,
			"effective_threshold": effectiveThreshold,
			"close_reason":        nil,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (s *Store) WriteOpenTransition(ctx context.Context, params AlertOpenEventParams) (AlertOpenEventResult, error) {
	var result AlertOpenEventResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		insertedID, err := s.insertOpenEvent(tx, params)
		if err != nil {
			if isUniqueConstraintError(err, "uniq_alert_event_open") {
				var event model.AlertEvent
				if err := tx.
					Where("rule_id = ? AND rule_generation = ? AND object_type = ? AND object_id = ? AND status = ?",
						params.RuleID, params.RuleGeneration, params.ObjectType, params.ObjectID, model.AlertStatusOpen).
					Take(&event).Error; err != nil {
					return err
				}
				result = AlertOpenEventResult{EventID: event.ID, Created: false}
				return nil
			}
			return err
		}
		if insertedID == 0 {
			return ErrAlertRuleVersionStale
		}
		if err := insertAlertNotifications(tx, insertedID, params.Notifications, time.Now().UTC()); err != nil {
			return err
		}

		result = AlertOpenEventResult{EventID: insertedID, Created: true}
		return nil
	})
	return result, err
}

func (s *Store) insertOpenEvent(tx *gorm.DB, params AlertOpenEventParams) (int64, error) {
	type insertedRow struct {
		ID int64
	}
	var inserted insertedRow
	if params.Builtin {
		err := tx.Raw(`
			INSERT INTO alert_events (
				rule_id,
				rule_generation,
				rule_snapshot,
				object_type,
				object_id,
				status,
				first_trigger_at,
				last_trigger_at,
				current_value,
				effective_threshold,
				title,
				message
			)
			VALUES (
				?,
				?,
				?::jsonb,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?
			)
			RETURNING id
			`,
			params.RuleID,
			params.RuleGeneration,
			string(params.RuleSnapshot),
			string(params.ObjectType),
			params.ObjectID,
			string(model.AlertStatusOpen),
			params.TriggeredAt,
			params.TriggeredAt,
			params.CurrentValue,
			params.EffectiveThreshold,
			emptyToNil(params.Title),
			emptyToNil(params.Message),
		).Scan(&inserted).Error
		return inserted.ID, err
	}

	err := tx.Raw(`
		INSERT INTO alert_events (
			rule_id,
			rule_generation,
			rule_snapshot,
			object_type,
			object_id,
			status,
			first_trigger_at,
			last_trigger_at,
			current_value,
			effective_threshold,
			title,
			message
		)
		SELECT
			ar.id,
			ar.generation,
			?::jsonb,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?
		FROM alert_rules AS ar
		WHERE ar.id = ?
			AND ar.generation = ?
			AND ar.enabled = TRUE
			AND ar.is_deleted = FALSE
		RETURNING id
		`,
		string(params.RuleSnapshot),
		string(params.ObjectType),
		params.ObjectID,
		string(model.AlertStatusOpen),
		params.TriggeredAt,
		params.TriggeredAt,
		params.CurrentValue,
		params.EffectiveThreshold,
		emptyToNil(params.Title),
		emptyToNil(params.Message),
		params.RuleID,
		params.RuleGeneration,
	).Scan(&inserted).Error
	return inserted.ID, err
}

func (s *Store) WriteCloseTransition(ctx context.Context, params AlertCloseEventParams) (AlertCloseEventResult, error) {
	var result AlertCloseEventResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var event model.AlertEvent
		query := tx.Where("status = ?", model.AlertStatusOpen)
		switch {
		case params.EventID > 0:
			query = query.Where("id = ?", params.EventID)
		default:
			query = query.Where("rule_id = ? AND rule_generation = ? AND object_type = ? AND object_id = ?",
				params.RuleID, params.RuleGeneration, params.ObjectType, params.ObjectID)
		}
		if err := query.Clauses(clause.Locking{Strength: "UPDATE"}).Take(&event).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				result = AlertCloseEventResult{Status: CloseStatusNotFound}
				return nil
			}
			return err
		}

		updates := map[string]any{
			"status":          model.AlertStatusClosed,
			"closed_at":       params.ClosedAt,
			"last_trigger_at": params.ClosedAt,
			"close_reason":    emptyToNil(params.CloseReason),
		}
		if params.CurrentValue != nil {
			updates["current_value"] = *params.CurrentValue
		}
		if err := tx.Model(&model.AlertEvent{}).Where("id = ?", event.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := insertAlertNotifications(tx, event.ID, params.Notifications, time.Now().UTC()); err != nil {
			return err
		}

		result = AlertCloseEventResult{EventID: event.ID, Status: CloseStatusClosed}
		return nil
	})
	return result, err
}
