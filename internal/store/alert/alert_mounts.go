package alert

import (
	"context"
	"time"

	"dash/internal/model"

	"gorm.io/gorm/clause"
)

func (s *Store) ListRuleMounts(ctx context.Context) ([]model.AlertRuleMount, error) {
	var rows []model.AlertRuleMount
	err := s.db.WithContext(ctx).
		Model(&model.AlertRuleMount{}).
		Find(&rows).Error
	return rows, err
}

func (s *Store) RuleMountsForServer(ctx context.Context, id int64) (map[int64]bool, error) {
	var rows []model.AlertRuleMount
	err := s.db.WithContext(ctx).
		Where("server_id = ?", id).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[int64]bool, len(rows))
	for _, row := range rows {
		out[row.RuleID] = row.Enabled
	}
	return out, nil
}

func (s *Store) SetRuleMounts(ctx context.Context, ruleIDs, serverIDs []int64, enabled bool) error {
	if len(ruleIDs) == 0 || len(serverIDs) == 0 {
		return nil
	}
	now := time.Now().UTC()
	rows := make([]model.AlertRuleMount, 0, len(ruleIDs)*len(serverIDs))
	for _, ruleID := range ruleIDs {
		for _, serverID := range serverIDs {
			rows = append(rows, model.AlertRuleMount{
				RuleID:    ruleID,
				ServerID:  serverID,
				Enabled:   enabled,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
	}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "rule_id"}, {Name: "server_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"enabled", "updated_at"}),
		}).
		Create(&rows).Error
}
