package alert

import (
	"fmt"
	"strings"
	"time"

	"dash/internal/alertspec"
	"dash/internal/metrics"
)

const (
	RuntimePhasePending  = "pending"
	RuntimePhaseFiring   = "firing"
	RuntimePhaseCooldown = "cooldown"
)

type RuntimeState struct {
	Phase              string  `json:"phase"`
	RuleID             int64   `json:"rule_id"`
	Generation         int64   `json:"generation"`
	PendingSince       string  `json:"pending_since,omitempty"`
	FiringSince        string  `json:"firing_since,omitempty"`
	CooldownUntil      string  `json:"cooldown_until,omitempty"`
	LastDBHeartbeatAt  string  `json:"last_db_heartbeat_at,omitempty"`
	LastObservedAt     string  `json:"last_observed_at,omitempty"`
	LastEvalAt         string  `json:"last_eval_at,omitempty"`
	CurrentValue       float64 `json:"current_value"`
	EffectiveThreshold float64 `json:"effective_threshold"`
	EventID            int64   `json:"event_id,omitempty"`
}

type OpenTransition struct {
	StateKey           string
	Rule               CompiledRule
	ObjectID           int64
	TriggeredAt        time.Time
	PendingSince       time.Time
	ObservedAt         time.Time
	CurrentValue       float64
	EffectiveThreshold float64
	Snapshot           *metrics.NodeView
}

type CloseTransition struct {
	StateKey     string
	EventID      int64
	Rule         CompiledRule
	ObjectID     int64
	OpenedAt     time.Time
	ClosedAt     time.Time
	CloseReason  string
	CurrentValue *float64
	Snapshot     *metrics.NodeView
}

type EvalResult struct {
	Next             map[string]RuntimeState
	OpenTransitions  []OpenTransition
	CloseTransitions []CloseTransition
}

func EvaluateServer(serverID int64, snapshot *metrics.NodeView, compiled *CompiledRules, current map[string]RuntimeState, now time.Time) EvalResult {
	now = now.UTC()
	if current == nil {
		current = make(map[string]RuntimeState)
	}
	result := EvalResult{
		Next:             make(map[string]RuntimeState, len(current)),
		OpenTransitions:  make([]OpenTransition, 0),
		CloseTransitions: make([]CloseTransition, 0),
	}

	observedAt, hasObservedAt := snapshotObservedAt(snapshot)
	online, hasSnapshotTime := snapshotOnline(snapshot, now)
	if snapshot == nil || !hasObservedAt || !hasSnapshotTime {
		for key, state := range current {
			if state.Phase != RuntimePhaseFiring {
				continue
			}
			rule := ruleForState(compiled, state)
			if isOfflineRule(rule) {
				result.Next[key] = keepFiringState(state, state.CurrentValue, state.EffectiveThreshold, state.LastObservedAtTime(), now)
				continue
			}
			result.Next[key] = state
			result.CloseTransitions = append(result.CloseTransitions, CloseTransition{
				StateKey:     key,
				EventID:      state.EventID,
				Rule:         rule,
				ObjectID:     serverID,
				OpenedAt:     state.FiringSinceTime(),
				ClosedAt:     now,
				CloseReason:  "snapshot_stale",
				CurrentValue: nil,
			})
		}
		return result
	}

	seen := make(map[string]struct{}, len(compiled.Rules))
	for _, rule := range compiled.Rules {
		key := rule.StateKey()
		seen[key] = struct{}{}
		existing, exists := current[key]
		if exists && existing.Phase == RuntimePhaseCooldown {
			if cooldownActive(existing, now) {
				result.Next[key] = keepCooldownState(existing, now)
				continue
			}
			exists = false
		}

		value, ok, evalAt := metricValue(rule, snapshot, online, observedAt, now)
		threshold, err := effectiveThreshold(rule, *snapshot)
		conditionTrue := err == nil && ok && alertspec.Compare(rule.Operator, value, threshold)

		switch {
		case !conditionTrue:
			if exists && existing.Phase == RuntimePhaseFiring {
				closeReason := "condition_cleared"
				closedAt := evalAt
				currentValue := floatPtr(value)
				if !online && rule.Metric != "node.offline" {
					closeReason = "snapshot_stale"
					closedAt = now
					currentValue = nil
				}
				result.Next[key] = keepFiringState(existing, value, threshold, observedAt, now)
				result.CloseTransitions = append(result.CloseTransitions, CloseTransition{
					StateKey:     key,
					EventID:      existing.EventID,
					Rule:         rule,
					ObjectID:     serverID,
					OpenedAt:     existing.FiringSinceTime(),
					ClosedAt:     closedAt,
					CloseReason:  closeReason,
					CurrentValue: currentValue,
					Snapshot:     snapshot,
				})
			}
			continue
		case !exists:
			pendingSince := maxTime(rule.GenerationUpdatedAt, evalAt)
			result.Next[key] = newPendingState(rule, pendingSince, evalAt, now, value, threshold)
			if durationSatisfied(pendingSince, rule.DurationSec, evalAt) {
				result.OpenTransitions = append(result.OpenTransitions, OpenTransition{
					StateKey:           key,
					Rule:               rule,
					ObjectID:           serverID,
					TriggeredAt:        evalAt,
					PendingSince:       pendingSince,
					ObservedAt:         evalAt,
					CurrentValue:       value,
					EffectiveThreshold: threshold,
					Snapshot:           snapshot,
				})
			}
		case existing.Phase == RuntimePhaseFiring:
			result.Next[key] = keepFiringState(existing, value, threshold, evalAt, now)
		default:
			pendingSince := existing.PendingSinceTime()
			if pendingSince.IsZero() || pendingSince.Before(rule.GenerationUpdatedAt) {
				pendingSince = maxTime(rule.GenerationUpdatedAt, evalAt)
			}
			result.Next[key] = newPendingState(rule, pendingSince, evalAt, now, value, threshold)
			if durationSatisfied(pendingSince, rule.DurationSec, evalAt) {
				result.OpenTransitions = append(result.OpenTransitions, OpenTransition{
					StateKey:           key,
					Rule:               rule,
					ObjectID:           serverID,
					TriggeredAt:        evalAt,
					PendingSince:       pendingSince,
					ObservedAt:         evalAt,
					CurrentValue:       value,
					EffectiveThreshold: threshold,
					Snapshot:           snapshot,
				})
			}
		}
	}

	for key, state := range current {
		if _, ok := seen[key]; ok {
			continue
		}
		if state.Phase != RuntimePhaseFiring {
			continue
		}
		result.Next[key] = state
		closeReason := "rule_unmounted"
		if compiled != nil {
			if _, ok := compiled.ByStateKey[key]; ok {
				closeReason = "rule_unmounted"
			}
		}
		if !online {
			closeReason = "snapshot_stale"
		}
		result.CloseTransitions = append(result.CloseTransitions, CloseTransition{
			StateKey:     key,
			EventID:      state.EventID,
			Rule:         ruleForState(compiled, state),
			ObjectID:     serverID,
			OpenedAt:     state.FiringSinceTime(),
			ClosedAt:     now,
			CloseReason:  closeReason,
			CurrentValue: nil,
			Snapshot:     snapshot,
		})
	}

	return result
}

func (s RuntimeState) PendingSinceTime() time.Time {
	return parseRuntimeTime(s.PendingSince)
}

func (s RuntimeState) FiringSinceTime() time.Time {
	return parseRuntimeTime(s.FiringSince)
}

func (s RuntimeState) CooldownUntilTime() time.Time {
	return parseRuntimeTime(s.CooldownUntil)
}

func (s RuntimeState) LastDBHeartbeatAtTime() time.Time {
	return parseRuntimeTime(s.LastDBHeartbeatAt)
}

func (s RuntimeState) LastObservedAtTime() time.Time {
	return parseRuntimeTime(s.LastObservedAt)
}

func keepFiringState(state RuntimeState, currentValue, threshold float64, observedAt, now time.Time) RuntimeState {
	state.Phase = RuntimePhaseFiring
	state.LastObservedAt = formatRuntimeTime(observedAt)
	state.LastEvalAt = formatRuntimeTime(now)
	state.CurrentValue = currentValue
	state.EffectiveThreshold = threshold
	return state
}

func keepCooldownState(state RuntimeState, now time.Time) RuntimeState {
	state.Phase = RuntimePhaseCooldown
	state.LastEvalAt = formatRuntimeTime(now)
	return state
}

func newPendingState(rule CompiledRule, pendingSince, observedAt, now time.Time, currentValue, threshold float64) RuntimeState {
	return RuntimeState{
		Phase:              RuntimePhasePending,
		RuleID:             rule.RuleID,
		Generation:         rule.Generation,
		PendingSince:       formatRuntimeTime(pendingSince),
		LastObservedAt:     formatRuntimeTime(observedAt),
		LastEvalAt:         formatRuntimeTime(now),
		CurrentValue:       currentValue,
		EffectiveThreshold: threshold,
	}
}

func newCooldownState(rule CompiledRule, closedAt, now time.Time) RuntimeState {
	return RuntimeState{
		Phase:         RuntimePhaseCooldown,
		RuleID:        rule.RuleID,
		Generation:    rule.Generation,
		CooldownUntil: formatRuntimeTime(closedAt.Add(time.Duration(rule.CooldownMin) * time.Minute)),
		LastEvalAt:    formatRuntimeTime(now),
	}
}

func firingFromPending(rule CompiledRule, pending RuntimeState, eventID int64, observedAt, now time.Time, currentValue, threshold float64) RuntimeState {
	return RuntimeState{
		Phase:              RuntimePhaseFiring,
		RuleID:             rule.RuleID,
		Generation:         rule.Generation,
		PendingSince:       pending.PendingSince,
		FiringSince:        formatRuntimeTime(now),
		LastDBHeartbeatAt:  formatRuntimeTime(now),
		LastObservedAt:     formatRuntimeTime(observedAt),
		LastEvalAt:         formatRuntimeTime(now),
		CurrentValue:       currentValue,
		EffectiveThreshold: threshold,
		EventID:            eventID,
	}
}

func applyOpenTransition(next map[string]RuntimeState, transition OpenTransition, eventID int64) {
	pending := next[transition.StateKey]
	next[transition.StateKey] = firingFromPending(
		transition.Rule,
		pending,
		eventID,
		transition.ObservedAt,
		transition.TriggeredAt,
		transition.CurrentValue,
		transition.EffectiveThreshold,
	)
}

func snapshotObservedAt(snapshot *metrics.NodeView) (time.Time, bool) {
	if snapshot == nil {
		return time.Time{}, false
	}
	raw := strings.TrimSpace(snapshot.Observation.ObservedAt)
	if raw == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func snapshotReceivedAt(snapshot *metrics.NodeView) (time.Time, bool) {
	if snapshot == nil {
		return time.Time{}, false
	}
	raw := strings.TrimSpace(snapshot.Observation.ReceivedAt)
	if raw == "" {
		raw = strings.TrimSpace(snapshot.Observation.ObservedAt)
	}
	if raw == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func snapshotOnline(snapshot *metrics.NodeView, now time.Time) (bool, bool) {
	receivedAt, ok := snapshotReceivedAt(snapshot)
	if !ok || receivedAt.IsZero() {
		return false, false
	}
	staleAfter := time.Duration(maxInt(snapshot.Observation.StaleAfterSec, 0)) * time.Second
	return now.Sub(receivedAt) <= staleAfter, true
}

func metricValue(rule CompiledRule, snapshot *metrics.NodeView, online bool, observedAt, now time.Time) (float64, bool, time.Time) {
	if rule.Metric == "node.offline" {
		if snapshot == nil {
			return 0, false, now
		}
		if online {
			return 0, true, now
		}
		return 1, true, now
	}
	if !online || snapshot == nil {
		return 0, false, observedAt
	}
	value, ok := alertspec.ExtractMetricValue(rule.Metric, *snapshot)
	return value, ok, observedAt
}

func cooldownActive(state RuntimeState, now time.Time) bool {
	until := state.CooldownUntilTime()
	return !until.IsZero() && now.Before(until)
}

func cooldownAfterClose(transition CloseTransition) bool {
	return transition.Rule.CooldownMin > 0 && transition.CloseReason == "condition_cleared"
}

func durationSatisfied(pendingSince time.Time, durationSec int32, now time.Time) bool {
	if pendingSince.IsZero() {
		return false
	}
	return now.Sub(pendingSince) >= time.Duration(durationSec)*time.Second
}

func parseRuntimeTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func formatRuntimeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func floatPtr(v float64) *float64 {
	return &v
}

func shouldHeartbeatFiring(previous, next RuntimeState, interval time.Duration, closing bool) bool {
	if closing {
		return false
	}
	if previous.Phase != RuntimePhaseFiring || next.Phase != RuntimePhaseFiring || next.EventID <= 0 {
		return false
	}
	observedAt := next.LastObservedAtTime()
	if observedAt.IsZero() {
		return false
	}
	lastHeartbeat := previous.LastDBHeartbeatAtTime()
	if lastHeartbeat.IsZero() {
		lastHeartbeat = previous.FiringSinceTime()
	}
	if lastHeartbeat.IsZero() {
		return true
	}
	if !observedAt.After(lastHeartbeat) {
		return false
	}
	return observedAt.Sub(lastHeartbeat) >= interval
}

func effectiveThreshold(rule CompiledRule, node metrics.NodeView) (float64, error) {
	switch rule.ThresholdMode {
	case "", "static":
		return rule.Threshold, nil
	case "core_plus":
		if !alertspec.SupportsCorePlus(rule.Metric) {
			return 0, fmt.Errorf("metric %s does not support core_plus", rule.Metric)
		}
		return float64(alertspec.ResolveCPUCores(node)) + rule.Threshold + rule.ThresholdOffset, nil
	default:
		return 0, fmt.Errorf("unsupported threshold_mode %s", rule.ThresholdMode)
	}
}

func ruleForState(compiled *CompiledRules, state RuntimeState) CompiledRule {
	if compiled != nil {
		if rule, ok := compiled.ByStateKey[ruleStateKey(state.RuleID, state.Generation)]; ok {
			return rule
		}
	}
	return CompiledRule{
		RuleID:     state.RuleID,
		Generation: state.Generation,
		Snapshot: RuleSnapshot{
			RuleID:     state.RuleID,
			Generation: state.Generation,
		},
	}
}
