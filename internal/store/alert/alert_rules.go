package alert

import (
	"context"
	"dash/internal/alertspec"
	"fmt"
	"math"
	"time"

	"dash/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ControlTaskRuleChange    = "rule_change"
	ControlTaskFullReconcile = "full_reconcile"
)

type AlertRuleItem struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Enabled         bool   `json:"enabled"`
	Metric          string
	Operator        string
	Threshold       float64
	DurationSec     int32
	CooldownMin     int32
	ThresholdMode   string
	ThresholdOffset float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type RuleChangePayload struct {
	RuleID        int64  `json:"rule_id"`
	OldGeneration int64  `json:"old_generation"`
	NewGeneration int64  `json:"new_generation"`
	CloseReason   string `json:"close_reason,omitempty"`
}

type AlertRulePatch struct {
	Name            *string
	Enabled         *bool
	Metric          *string
	Operator        *string
	Threshold       *float64
	DurationSec     *int32
	CooldownMin     *int32
	ThresholdMode   *string
	ThresholdOffset *float64
}

func (s *Store) ListRules(ctx context.Context) ([]AlertRuleItem, error) {
	var items []AlertRuleItem
	err := s.db.WithContext(ctx).
		Model(&model.AlertRule{}).
		Select("id", "name", "enabled",
			"metric", "operator", "threshold", "duration_sec", "cooldown_min", "threshold_mode", "threshold_offset",
			"created_at", "updated_at").
		Where("is_deleted = ?", false).
		Order("id DESC").
		Find(&items).Error
	return items, err
}

func (s *Store) RulesForCompile(ctx context.Context) ([]model.AlertRule, error) {
	var items []model.AlertRule
	err := s.db.WithContext(ctx).
		Where("is_deleted = ?", false).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

func (s *Store) CreateRule(ctx context.Context, rule *model.AlertRule) error {
	if err := prepareCreate(rule); err != nil {
		return err
	}
	return s.WithTx(ctx, func(tx *Store) error {
		if err := tx.db.WithContext(ctx).Create(rule).Error; err != nil {
			return err
		}
		payload := RuleChangePayload{
			RuleID:        rule.ID,
			OldGeneration: 0,
			NewGeneration: rule.Generation,
		}
		return tx.enqueueControlTask(ctx, ControlTaskRuleChange, changeKey(rule.ID, rule.Generation), payload)
	})
}

func (s *Store) PatchRule(ctx context.Context, id int64, patch AlertRulePatch) error {
	if !patch.HasUpdates() {
		return nil
	}
	return s.patchRule(ctx, id, patch)
}

func (s *Store) patchRule(ctx context.Context, id int64, patch AlertRulePatch) error {
	return s.WithTx(ctx, func(tx *Store) error {
		current, err := tx.lockRule(ctx, id)
		if err != nil {
			return err
		}

		merged := *current
		applyPatch(&merged, patch)
		normalized, err := alertspec.NormalizeRuleModel(merged)
		if err != nil {
			return err
		}
		updates := updatesFromPatch(normalized, patch)
		if len(updates) == 0 {
			return nil
		}

		nextGeneration := current.Generation
		reason := ""
		if needsGeneration(current, updates) {
			nextGeneration++
			updates["generation"] = nextGeneration
			reason = reasonFor(current, updates)
		}

		res := tx.db.WithContext(ctx).
			Model(&model.AlertRule{}).
			Where("id = ? AND is_deleted = ?", id, false).
			Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		if nextGeneration == current.Generation {
			return nil
		}

		payload := RuleChangePayload{
			RuleID:        current.ID,
			OldGeneration: current.Generation,
			NewGeneration: nextGeneration,
			CloseReason:   reason,
		}
		return tx.enqueueControlTask(ctx, ControlTaskRuleChange, changeKey(current.ID, nextGeneration), payload)
	})
}

func (s *Store) DeleteRule(ctx context.Context, id int64) error {
	return s.WithTx(ctx, func(tx *Store) error {
		current, err := tx.lockRule(ctx, id)
		if err != nil {
			return err
		}

		nextGeneration := current.Generation + 1
		res := tx.db.WithContext(ctx).
			Model(&model.AlertRule{}).
			Where("id = ? AND is_deleted = ?", id, false).
			Updates(map[string]any{
				"is_deleted": true,
				"generation": nextGeneration,
				"updated_at": time.Now().UTC(),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.db.WithContext(ctx).Delete(&model.AlertRuleMount{}, "rule_id = ?", id).Error; err != nil {
			return err
		}

		payload := RuleChangePayload{
			RuleID:        current.ID,
			OldGeneration: current.Generation,
			NewGeneration: nextGeneration,
			CloseReason:   "rule_deleted",
		}
		return tx.enqueueControlTask(ctx, ControlTaskRuleChange, changeKey(current.ID, nextGeneration), payload)
	})
}

func (s *Store) lockRule(ctx context.Context, id int64) (*model.AlertRule, error) {
	var item model.AlertRule
	err := s.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND is_deleted = ?", id, false).
		First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func changeKey(ruleID, generation int64) string {
	return fmt.Sprintf("rule_change:rule:%d:gen:%d", ruleID, generation)
}

func needsGeneration(current *model.AlertRule, updates map[string]any) bool {
	if current == nil {
		return false
	}
	if enabled, ok := updates["enabled"].(bool); ok && enabled != current.Enabled {
		return true
	}
	if metric, ok := updates["metric"].(string); ok && metric != current.Metric {
		return true
	}
	if operator, ok := updates["operator"].(string); ok && operator != current.Operator {
		return true
	}
	if threshold, ok := updates["threshold"].(float64); ok && !sameFloat(threshold, current.Threshold) {
		return true
	}
	if duration, ok := updates["duration_sec"].(int32); ok && duration != current.DurationSec {
		return true
	}
	if cooldown, ok := updates["cooldown_min"].(int32); ok && cooldown != current.CooldownMin {
		return true
	}
	if mode, ok := updates["threshold_mode"].(string); ok && mode != current.ThresholdMode {
		return true
	}
	if offset, ok := updates["threshold_offset"].(float64); ok && !sameFloat(offset, current.ThresholdOffset) {
		return true
	}
	return false
}

func reasonFor(current *model.AlertRule, updates map[string]any) string {
	if current == nil {
		return ""
	}
	if enabled, ok := updates["enabled"].(bool); ok && current.Enabled && !enabled {
		return "rule_disabled"
	}
	return "rule_updated"
}

func sameFloat(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func prepareCreate(rule *model.AlertRule) error {
	if rule == nil {
		return fmt.Errorf("alert rule is nil")
	}
	normalized, err := alertspec.PrepareCreateRule(*rule)
	if err != nil {
		return err
	}
	*rule = normalized
	return nil
}

func (p AlertRulePatch) HasUpdates() bool {
	return p.Name != nil ||
		p.Enabled != nil ||
		p.Metric != nil ||
		p.Operator != nil ||
		p.Threshold != nil ||
		p.DurationSec != nil ||
		p.CooldownMin != nil ||
		p.ThresholdMode != nil ||
		p.ThresholdOffset != nil
}

func applyPatch(rule *model.AlertRule, patch AlertRulePatch) {
	if rule == nil {
		return
	}
	if patch.Name != nil {
		rule.Name = *patch.Name
	}
	if patch.Enabled != nil {
		rule.Enabled = *patch.Enabled
	}
	if patch.Metric != nil {
		rule.Metric = *patch.Metric
	}
	if patch.Operator != nil {
		rule.Operator = *patch.Operator
	}
	if patch.Threshold != nil {
		rule.Threshold = *patch.Threshold
	}
	if patch.DurationSec != nil {
		rule.DurationSec = *patch.DurationSec
	}
	if patch.CooldownMin != nil {
		rule.CooldownMin = *patch.CooldownMin
	}
	if patch.ThresholdMode != nil {
		rule.ThresholdMode = *patch.ThresholdMode
	}
	if patch.ThresholdOffset != nil {
		rule.ThresholdOffset = *patch.ThresholdOffset
	}
}

func updatesFromPatch(normalized model.AlertRule, patch AlertRulePatch) map[string]any {
	updates := make(map[string]any, 8)
	if patch.Name != nil {
		updates["name"] = normalized.Name
	}
	if patch.Enabled != nil {
		updates["enabled"] = normalized.Enabled
	}
	if patch.Metric != nil {
		updates["metric"] = normalized.Metric
	}
	if patch.Operator != nil {
		updates["operator"] = normalized.Operator
	}
	if patch.Threshold != nil {
		updates["threshold"] = normalized.Threshold
	}
	if patch.DurationSec != nil {
		updates["duration_sec"] = normalized.DurationSec
	}
	if patch.CooldownMin != nil {
		updates["cooldown_min"] = normalized.CooldownMin
	}
	if patch.ThresholdMode != nil {
		updates["threshold_mode"] = normalized.ThresholdMode
	}
	if patch.ThresholdOffset != nil {
		updates["threshold_offset"] = normalized.ThresholdOffset
	}
	return updates
}
