package alertspec

import (
	"testing"

	"dash/internal/model"
)

func TestBuiltinRulesDoNotReuseRetiredIDs(t *testing.T) {
	retired := map[int64]string{
		-5: "retired SMART temperature rule",
	}

	seen := make(map[int64]string, len(BuiltinRules()))
	for _, rule := range BuiltinRules() {
		if reason, ok := retired[rule.ID]; ok {
			t.Fatalf("builtin rule %s reuses retired id %d: %s", rule.Name, rule.ID, reason)
		}
		if existing, ok := seen[rule.ID]; ok {
			t.Fatalf("builtin rule id %d is used by both %s and %s", rule.ID, existing, rule.Name)
		}
		seen[rule.ID] = rule.Name
	}
}

func TestNormalizeRuleRejectsStaticThresholdOffset(t *testing.T) {
	_, err := NormalizeRuleModel(model.AlertRule{
		Name:            "cpu_high",
		Metric:          "cpu.usage_ratio",
		Operator:        ">=",
		Threshold:       0.9,
		DurationSec:     60,
		ThresholdMode:   "static",
		ThresholdOffset: 1,
		Generation:      1,
	})
	if err == nil {
		t.Fatal("NormalizeRuleModel() accepted a static threshold offset")
	}
}
