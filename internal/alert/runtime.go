package alert

import (
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

// RuntimeState uses UTC timestamps with second precision.
type RuntimeState struct {
	Phase              string
	RuleID             int64
	Generation         int64
	PendingSince       time.Time
	FiringSince        time.Time
	CooldownUntil      time.Time
	LastDBHeartbeatAt  time.Time
	LastObservedAt     time.Time
	CurrentValue       float64
	EffectiveThreshold float64
	EventID            int64
}

type OpenTransition struct {
	StateKey           string
	Rule               CompiledRule
	ObjectID           int64
	TriggeredAt        time.Time
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
	result := EvalResult{
		Next:             make(map[string]RuntimeState, len(current)),
		OpenTransitions:  make([]OpenTransition, 0),
		CloseTransitions: make([]CloseTransition, 0),
	}
	seen := make(map[string]struct{})
	if compiled != nil {
		seen = make(map[string]struct{}, len(compiled.Rules))
		for _, rule := range compiled.Rules {
			seen[rule.StateKey()] = struct{}{}
		}
	}

	observedAt, hasObservedAt := snapshotObservedAt(snapshot)
	online, hasSnapshotTime := snapshotOnline(snapshot, now)
	if snapshot == nil || !hasObservedAt || !hasSnapshotTime {
		for key, state := range current {
			if state.Phase != RuntimePhaseFiring {
				continue
			}
			result.Next[key] = state
			if _, ok := seen[key]; ok {
				continue
			}
			result.CloseTransitions = append(result.CloseTransitions, CloseTransition{
				StateKey:    key,
				EventID:     state.EventID,
				Rule:        ruleForState(compiled, state),
				ObjectID:    serverID,
				OpenedAt:    state.FiringSince,
				ClosedAt:    now,
				CloseReason: "rule_unmounted",
			})
		}
		return result
	}

	for _, rule := range compiled.Rules {
		key := rule.StateKey()
		existing, exists := current[key]
		if exists && existing.Phase == RuntimePhaseCooldown {
			if cooldownActive(existing, now) {
				result.Next[key] = existing
				continue
			}
			exists = false
		}
		if !online && !isOfflineRule(rule) {
			if exists && existing.Phase == RuntimePhaseFiring {
				result.Next[key] = existing
			}
			continue
		}

		value, ok, evalAt := metricValue(rule, snapshot, online, observedAt, now)
		threshold, err := alertspec.EffectiveThreshold(alertspec.Threshold{
			Metric: rule.Metric,
			Mode:   rule.ThresholdMode,
			Value:  rule.Threshold,
			Offset: rule.ThresholdOffset,
		}, *snapshot)
		if err != nil || !ok {
			if exists && existing.Phase == RuntimePhaseFiring {
				result.Next[key] = existing
			}
			continue
		}
		if !alertspec.Compare(rule.Operator, value, threshold) {
			if exists && existing.Phase == RuntimePhaseFiring {
				result.Next[key] = keepFiringState(existing, value, threshold, observedAt)
				result.CloseTransitions = append(result.CloseTransitions, CloseTransition{
					StateKey:     key,
					EventID:      existing.EventID,
					Rule:         rule,
					ObjectID:     serverID,
					OpenedAt:     existing.FiringSince,
					ClosedAt:     evalAt,
					CloseReason:  "condition_cleared",
					CurrentValue: new(value),
					Snapshot:     snapshot,
				})
			}
			continue
		}
		if exists && existing.Phase == RuntimePhaseFiring {
			result.Next[key] = keepFiringState(existing, value, threshold, evalAt)
			continue
		}
		pendingSince := existing.PendingSince
		if !exists || pendingSince.IsZero() || pendingSince.Before(rule.GenerationUpdatedAt) {
			pendingSince = maxTime(rule.GenerationUpdatedAt, evalAt)
		}
		result.Next[key] = newPendingState(rule, pendingSince, evalAt, value, threshold)
		if durationSatisfied(pendingSince, rule.DurationSec, evalAt) {
			result.OpenTransitions = append(result.OpenTransitions, OpenTransition{
				StateKey:           key,
				Rule:               rule,
				ObjectID:           serverID,
				TriggeredAt:        evalAt,
				CurrentValue:       value,
				EffectiveThreshold: threshold,
				Snapshot:           snapshot,
			})
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
		result.CloseTransitions = append(result.CloseTransitions, CloseTransition{
			StateKey:     key,
			EventID:      state.EventID,
			Rule:         ruleForState(compiled, state),
			ObjectID:     serverID,
			OpenedAt:     state.FiringSince,
			ClosedAt:     now,
			CloseReason:  "rule_unmounted",
			CurrentValue: nil,
			Snapshot:     snapshot,
		})
	}

	return result
}

func keepFiringState(state RuntimeState, currentValue, threshold float64, observedAt time.Time) RuntimeState {
	state.LastObservedAt = observedAt.UTC().Truncate(time.Second)
	state.CurrentValue = currentValue
	state.EffectiveThreshold = threshold
	return state
}

func newPendingState(rule CompiledRule, pendingSince, observedAt time.Time, currentValue, threshold float64) RuntimeState {
	return RuntimeState{
		Phase:              RuntimePhasePending,
		RuleID:             rule.RuleID,
		Generation:         rule.Generation,
		PendingSince:       pendingSince.UTC().Truncate(time.Second),
		LastObservedAt:     observedAt.UTC().Truncate(time.Second),
		CurrentValue:       currentValue,
		EffectiveThreshold: threshold,
	}
}

func newCooldownState(rule CompiledRule, closedAt time.Time) RuntimeState {
	return RuntimeState{
		Phase:         RuntimePhaseCooldown,
		RuleID:        rule.RuleID,
		Generation:    rule.Generation,
		CooldownUntil: closedAt.Add(time.Duration(rule.CooldownMin) * time.Minute).UTC().Truncate(time.Second),
	}
}

func applyOpenTransition(next map[string]RuntimeState, transition OpenTransition, eventID int64) {
	state := next[transition.StateKey]
	state.Phase = RuntimePhaseFiring
	state.FiringSince = transition.TriggeredAt.UTC().Truncate(time.Second)
	state.LastDBHeartbeatAt = state.FiringSince
	state.EventID = eventID
	next[transition.StateKey] = state
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
	staleAfter := time.Duration(max(snapshot.Observation.StaleAfterSec, 0)) * time.Second
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
	until := state.CooldownUntil
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

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func shouldHeartbeatFiring(previous, next RuntimeState, interval time.Duration, closing bool) bool {
	if closing {
		return false
	}
	if previous.Phase != RuntimePhaseFiring || next.Phase != RuntimePhaseFiring || next.EventID <= 0 {
		return false
	}
	observedAt := next.LastObservedAt
	if observedAt.IsZero() {
		return false
	}
	lastHeartbeat := previous.LastDBHeartbeatAt
	if lastHeartbeat.IsZero() {
		lastHeartbeat = previous.FiringSince
	}
	if lastHeartbeat.IsZero() {
		return true
	}
	if !observedAt.After(lastHeartbeat) {
		return false
	}
	return observedAt.Sub(lastHeartbeat) >= interval
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
	}
}
