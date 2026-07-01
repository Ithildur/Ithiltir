package alert

import (
	"context"
	"errors"
	"strings"
	"time"

	"dash/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type EventStatus string

const (
	EventStatusAll    EventStatus = "all"
	EventStatusOpen   EventStatus = EventStatus(model.AlertStatusOpen)
	EventStatusClosed EventStatus = EventStatus(model.AlertStatusClosed)
)

type AlertEventQuery struct {
	ServerID int64
	Status   EventStatus
	Metric   string
	From     *time.Time
	To       *time.Time
	Cursor   *AlertEventCursor
	Limit    int
}

type AlertEventCursor struct {
	LastTriggerAt time.Time
	ID            int64
}

type AlertEventItem struct {
	ID                 int64
	RuleID             int64
	RuleGeneration     int64
	ServerID           int64
	ServerName         string
	ServerHostname     string
	ServerIP           *string
	Status             model.AlertStatus
	FirstTriggerAt     time.Time
	LastTriggerAt      time.Time
	ClosedAt           *time.Time
	CurrentValue       *float64
	EffectiveThreshold *float64
	CloseReason        *string
	Title              *string
	Message            *string
	Metric             string
	RuleName           string
}

type OpenEventSummary struct {
	ServerID      int64
	OpenCount     int64
	LastTriggerAt time.Time
	Metric        string
	RuleName      string
	Metrics       []string
}

type openEventSummaryRow struct {
	ServerID      int64
	OpenCount     int64
	LastTriggerAt time.Time
	Metric        string
	RuleName      string
	MetricsText   string
}

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

func (s *Store) ListEvents(ctx context.Context, q AlertEventQuery) ([]AlertEventItem, error) {
	if q.Status == "" {
		q.Status = EventStatusOpen
	}

	db := s.db.WithContext(ctx).
		Model(&model.AlertEvent{}).
		Select(`
			alert_events.id,
			alert_events.rule_id,
			alert_events.rule_generation,
			alert_events.object_id AS server_id,
			servers.name AS server_name,
			servers.hostname AS server_hostname,
			servers.ip AS server_ip,
			alert_events.status,
			alert_events.first_trigger_at,
			alert_events.last_trigger_at,
			alert_events.closed_at,
			alert_events.current_value,
			alert_events.effective_threshold,
			alert_events.close_reason,
			alert_events.title,
			alert_events.message,
			COALESCE(alert_events.rule_snapshot ->> 'metric', '') AS metric,
			COALESCE(alert_events.rule_snapshot ->> 'name', '') AS rule_name
		`).
		Joins("JOIN servers ON servers.id = alert_events.object_id").
		Where("alert_events.object_type = ? AND servers.is_deleted = ?", model.ObjectTypeServer, false)

	if q.ServerID > 0 {
		db = db.Where("alert_events.object_id = ?", q.ServerID)
	}
	if q.Status != EventStatusAll {
		db = db.Where("alert_events.status = ?", model.AlertStatus(q.Status))
	}
	if q.Metric != "" {
		db = db.Where("alert_events.rule_snapshot ->> 'metric' = ?", q.Metric)
	}
	if q.From != nil {
		db = db.Where("alert_events.last_trigger_at >= ?", *q.From)
	}
	if q.To != nil {
		db = db.Where("alert_events.last_trigger_at <= ?", *q.To)
	}
	if q.Cursor != nil {
		db = db.Where(
			"(alert_events.last_trigger_at < ? OR (alert_events.last_trigger_at = ? AND alert_events.id < ?))",
			q.Cursor.LastTriggerAt,
			q.Cursor.LastTriggerAt,
			q.Cursor.ID,
		)
	}

	var items []AlertEventItem
	db = db.
		Order("alert_events.last_trigger_at DESC").
		Order("alert_events.id DESC")
	if q.Limit > 0 {
		db = db.Limit(q.Limit)
	}
	err := db.Scan(&items).Error
	return items, err
}

func (s *Store) ListOpenEventSummaries(ctx context.Context) ([]OpenEventSummary, error) {
	ranked := s.db.WithContext(ctx).
		Model(&model.AlertEvent{}).
		Select(`
			alert_events.object_id AS server_id,
			COUNT(*) OVER (PARTITION BY alert_events.object_id) AS open_count,
			alert_events.last_trigger_at,
			COALESCE(alert_events.rule_snapshot ->> 'metric', '') AS metric,
			COALESCE(alert_events.rule_snapshot ->> 'name', '') AS rule_name,
			STRING_AGG(COALESCE(alert_events.rule_snapshot ->> 'metric', ''), E'\n') OVER (
				PARTITION BY alert_events.object_id
				ORDER BY alert_events.last_trigger_at DESC, alert_events.id DESC
				ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING
			) AS metrics_text,
			ROW_NUMBER() OVER (
				PARTITION BY alert_events.object_id
				ORDER BY alert_events.last_trigger_at DESC, alert_events.id DESC
			) AS row_rank
		`).
		Joins("JOIN servers ON servers.id = alert_events.object_id").
		Where(
			"alert_events.object_type = ? AND alert_events.status = ? AND servers.is_deleted = ?",
			model.ObjectTypeServer,
			model.AlertStatusOpen,
			false,
		)

	var rows []openEventSummaryRow
	err := s.db.WithContext(ctx).
		Table("(?) AS ranked", ranked).
		Select("server_id, open_count, last_trigger_at, metric, rule_name, metrics_text").
		Where("row_rank = 1").
		Order("last_trigger_at DESC").
		Order("server_id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]OpenEventSummary, 0, len(rows))
	for _, row := range rows {
		items = append(items, OpenEventSummary{
			ServerID:      row.ServerID,
			OpenCount:     row.OpenCount,
			LastTriggerAt: row.LastTriggerAt,
			Metric:        row.Metric,
			RuleName:      row.RuleName,
			Metrics:       splitSummaryMetrics(row.MetricsText, row.Metric),
		})
	}
	return items, nil
}

func splitSummaryMetrics(raw, fallback string) []string {
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		metric := strings.TrimSpace(part)
		if metric == "" {
			continue
		}
		out = append(out, metric)
	}
	if len(out) == 0 && fallback != "" {
		out = append(out, fallback)
	}
	return out
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
