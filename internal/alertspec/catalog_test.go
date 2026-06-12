package alertspec

import "testing"

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
