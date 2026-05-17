package alert

import (
	"testing"
	"time"

	"dash/internal/metrics"
	"dash/internal/model"
)

func TestEvaluateServerLifecycle(t *testing.T) {
	base := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	compiled := CompileRules([]model.AlertRule{{
		ID:              1,
		Name:            "cpu_load1_high",
		Enabled:         true,
		Generation:      2,
		Metric:          "cpu.load1",
		Operator:        ">=",
		Threshold:       0,
		DurationSec:     60,
		ThresholdMode:   "core_plus",
		ThresholdOffset: 1,
		UpdatedAt:       base,
	}}, base)

	snapshot1 := testSnapshot(base.Add(30*time.Second), base.Add(30*time.Second), 10, 5.0)
	step1 := EvaluateServer(42, snapshot1, compiled, nil, base.Add(30*time.Second))
	if _, ok := openForRule(step1.OpenTransitions, 1); ok {
		t.Fatalf("expected no open transition on first observation")
	}
	state1, ok := step1.Next[ruleStateKey(1, 2)]
	if !ok || state1.Phase != RuntimePhasePending {
		t.Fatalf("expected pending runtime state, got %+v", state1)
	}

	snapshot2 := testSnapshot(base.Add(90*time.Second), base.Add(119*time.Second), 10, 5.5)
	step2 := EvaluateServer(42, snapshot2, compiled, step1.Next, base.Add(120*time.Second))
	opened, ok := openForRule(step2.OpenTransitions, 1)
	if !ok {
		t.Fatalf("expected cpu rule to open after duration")
	}
	if !opened.TriggeredAt.Equal(base.Add(90 * time.Second)) {
		t.Fatalf("expected trigger at observed_at, got %s", opened.TriggeredAt)
	}
	applyOpenTransition(step2.Next, opened, 99)

	snapshot3 := testSnapshot(base.Add(100*time.Second), base.Add(139*time.Second), 10, 1.0)
	step3 := EvaluateServer(42, snapshot3, compiled, step2.Next, base.Add(140*time.Second))
	if len(step3.CloseTransitions) != 1 {
		t.Fatalf("expected one close transition, got %d", len(step3.CloseTransitions))
	}
	if step3.CloseTransitions[0].CloseReason != "condition_cleared" {
		t.Fatalf("unexpected close reason %s", step3.CloseTransitions[0].CloseReason)
	}
	if !step3.CloseTransitions[0].ClosedAt.Equal(base.Add(100 * time.Second)) {
		t.Fatalf("expected close at observed_at, got %s", step3.CloseTransitions[0].ClosedAt)
	}
	if _, ok := step3.Next[ruleStateKey(1, 2)]; !ok {
		t.Fatalf("expected runtime to keep firing state until close is persisted")
	}
}

func TestEvaluateServerClosesOnStaleSnapshot(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 5, 0, 0, time.UTC)
	current := map[string]RuntimeState{
		ruleStateKey(1, 1): {
			Phase:      RuntimePhaseFiring,
			RuleID:     1,
			Generation: 1,
			EventID:    77,
		},
	}
	snapshot := testSnapshot(now.Add(-60*time.Second), now.Add(-20*time.Second), 10, 8)
	result := EvaluateServer(42, snapshot, emptyRules(now), current, now)
	if len(result.CloseTransitions) != 1 {
		t.Fatalf("expected one close transition, got %d", len(result.CloseTransitions))
	}
	if result.CloseTransitions[0].CloseReason != "snapshot_stale" {
		t.Fatalf("expected snapshot_stale, got %s", result.CloseTransitions[0].CloseReason)
	}
	if _, ok := result.Next[ruleStateKey(1, 1)]; !ok {
		t.Fatalf("expected firing runtime state retained until close succeeds")
	}
}

func TestEvaluateServerOpensOfflineBuiltinOnStaleSnapshot(t *testing.T) {
	base := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	snapshot := testSnapshot(base, base, 10, 0)
	result := EvaluateServer(42, snapshot, CompileRules(nil, base), nil, base.Add(11*time.Second))
	if len(result.OpenTransitions) != 1 || result.OpenTransitions[0].Rule.Metric != "node.offline" {
		t.Fatalf("expected node.offline transition, got %+v", result.OpenTransitions)
	}
}

func TestEvaluateServerDurationUsesObservedAtNotWorkerDelay(t *testing.T) {
	base := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	compiled := CompileRules([]model.AlertRule{{
		ID:            11,
		Name:          "cpu_load1_high",
		Enabled:       true,
		Generation:    1,
		Metric:        "cpu.load1",
		Operator:      ">=",
		Threshold:     4,
		DurationSec:   60,
		ThresholdMode: "static",
		UpdatedAt:     base,
	}}, base)

	snapshot1 := testSnapshot(base.Add(10*time.Second), base.Add(10*time.Second), 10, 6)
	step1 := EvaluateServer(42, snapshot1, compiled, nil, base.Add(10*time.Second))
	snapshot2 := testSnapshot(base.Add(50*time.Second), base.Add(5*time.Minute), 10, 6)
	step2 := EvaluateServer(42, snapshot2, compiled, step1.Next, base.Add(5*time.Minute))
	if _, ok := openForRule(step2.OpenTransitions, 11); ok {
		t.Fatalf("expected no cpu open transition before observed duration reaches threshold")
	}
}

func TestEvaluateServerImmediateDurationOpensOnFirstObservation(t *testing.T) {
	base := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	compiled := CompileRules([]model.AlertRule{{
		ID:            12,
		Name:          "cpu_high",
		Enabled:       true,
		Generation:    1,
		Metric:        "cpu.load1",
		Operator:      ">=",
		Threshold:     1,
		DurationSec:   0,
		ThresholdMode: "static",
		UpdatedAt:     base,
	}}, base)
	snapshot := testSnapshot(base.Add(time.Second), base.Add(time.Second), 10, 2)

	result := EvaluateServer(42, snapshot, compiled, nil, base.Add(time.Second))
	if _, ok := openForRule(result.OpenTransitions, 12); !ok {
		t.Fatalf("expected immediate cpu open transition")
	}
}

func TestEvaluateServerSuppressesOpenDuringCooldown(t *testing.T) {
	base := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	compiled := CompileRules([]model.AlertRule{{
		ID:            13,
		Name:          "cpu_high",
		Enabled:       true,
		Generation:    1,
		Metric:        "cpu.load1",
		Operator:      ">=",
		Threshold:     1,
		DurationSec:   0,
		CooldownMin:   5,
		ThresholdMode: "static",
		UpdatedAt:     base,
	}}, base)
	key := ruleStateKey(13, 1)
	current := map[string]RuntimeState{
		key: newCooldownState(compiled.ByStateKey[key], base, base),
	}
	snapshot := testSnapshot(base.Add(time.Minute), base.Add(time.Minute), 10, 2)

	result := EvaluateServer(42, snapshot, compiled, current, base.Add(time.Minute))
	if _, ok := openForRule(result.OpenTransitions, 13); ok {
		t.Fatalf("expected cooldown to suppress cpu open transition")
	}
	if result.Next[key].Phase != RuntimePhaseCooldown {
		t.Fatalf("expected cooldown state, got %+v", result.Next[key])
	}

	snapshot = testSnapshot(base.Add(6*time.Minute), base.Add(6*time.Minute), 10, 2)
	result = EvaluateServer(42, snapshot, compiled, result.Next, base.Add(6*time.Minute))
	if _, ok := openForRule(result.OpenTransitions, 13); !ok {
		t.Fatalf("expected cpu open after cooldown, got %+v", result.OpenTransitions)
	}
}

func openForRule(transitions []OpenTransition, id int64) (OpenTransition, bool) {
	for _, transition := range transitions {
		if transition.Rule.RuleID == id {
			return transition, true
		}
	}
	return OpenTransition{}, false
}

func testSnapshot(observedAt, receivedAt time.Time, staleAfterSec int, load1 float64) *metrics.NodeView {
	return &metrics.NodeView{
		Node: metrics.NodeMeta{
			ID:    "42",
			Title: "node-42",
		},
		Observation: metrics.Observation{
			ReceivedAt:    receivedAt.UTC().Format(time.RFC3339),
			ObservedAt:    observedAt.UTC().Format(time.RFC3339),
			StaleAfterSec: staleAfterSec,
		},
		CPU: metrics.CPU{
			Load: metrics.CPULoad{
				L1: load1,
			},
			CoresLogical: 4,
		},
	}
}
