package alert

import (
	"testing"
	"time"

	"dash/internal/model"
	"gorm.io/datatypes"
)

func TestCompiledRuleFromEventBuildsRuleFromSnapshot(t *testing.T) {
	updatedAt := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	event := model.AlertEvent{
		ID:             10,
		RuleID:         7,
		RuleGeneration: 3,
		RuleSnapshot: datatypes.JSON([]byte(`{
			"rule_id":7,
			"generation":3,
			"name":"cpu-high",
			"metric":"cpu.usage_ratio",
			"operator":">=",
			"threshold":0.9,
			"duration_sec":60,
			"threshold_mode":"static",
			"threshold_offset":0,
			"updated_at":"` + updatedAt.Format(time.RFC3339) + `"
		}`)),
	}
	rule, err := ruleFromEvent(event)
	if err != nil {
		t.Fatalf("ruleFromEvent returned error: %v", err)
	}
	if rule.RuleID != 7 || rule.Generation != 3 {
		t.Fatalf("unexpected compiled rule identity: %+v", rule)
	}
	if rule.Metric != "cpu.usage_ratio" || rule.DurationSec != 60 || rule.ThresholdMode != "static" {
		t.Fatalf("unexpected compiled rule contents: %+v", rule)
	}
	if !rule.GenerationUpdatedAt.Equal(updatedAt) {
		t.Fatalf("expected updated_at %s, got %s", updatedAt, rule.GenerationUpdatedAt)
	}
}
