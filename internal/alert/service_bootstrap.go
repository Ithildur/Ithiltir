package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"dash/internal/model"
)

func (s *Service) rebuildRuntimeFromOpenEvents(ctx context.Context) error {
	if _, err := s.closeDeletedServers(ctx); err != nil {
		return err
	}

	events, err := s.store.ListOpenEvents(ctx)
	if err != nil {
		return err
	}
	grouped := runtimeStatesFromOpenEvents(events, time.Now().UTC())

	targets := make(map[int64]struct{}, len(grouped))
	for serverID := range grouped {
		targets[serverID] = struct{}{}
	}
	existingIDs, err := s.store.ListAlertRuntimeServerIDs(ctx)
	if err != nil {
		return err
	}
	for _, serverID := range existingIDs {
		targets[serverID] = struct{}{}
	}

	for serverID := range targets {
		current, err := loadRuntimeState(ctx, s.store, serverID)
		if err != nil {
			return err
		}
		// Runtime is a cache: startup rebuild replaces it with DB-open firing truth.
		next := make(map[string]RuntimeState, len(grouped[serverID]))
		for key, state := range grouped[serverID] {
			next[key] = state
		}
		if err := saveRuntimeState(ctx, s.store, serverID, current, next); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) restoreOpenRuntime(ctx context.Context) error {
	events, err := s.store.ListOpenEvents(ctx)
	if err != nil {
		return err
	}
	grouped := runtimeStatesFromOpenEvents(events, time.Now().UTC())
	for serverID, states := range grouped {
		current, err := loadRuntimeState(ctx, s.store, serverID)
		if err != nil {
			return err
		}
		next := current
		changed := false
		for key, state := range states {
			existing, ok := current[key]
			if ok && existing.Phase == RuntimePhaseFiring && existing.EventID > 0 && existing.EventID == state.EventID {
				continue
			}
			if !changed {
				next = make(map[string]RuntimeState, len(current)+len(states))
				for key, state := range current {
					next[key] = state
				}
				changed = true
			}
			next[key] = state
		}
		if !changed {
			continue
		}
		if err := saveRuntimeState(ctx, s.store, serverID, current, next); err != nil {
			return err
		}
	}
	return nil
}

func runtimeStatesFromOpenEvents(events []model.AlertEvent, now time.Time) map[int64]map[string]RuntimeState {
	grouped := make(map[int64]map[string]RuntimeState)
	for _, event := range events {
		if event.ObjectType != model.ObjectTypeServer {
			continue
		}
		serverID := event.ObjectID
		if serverID <= 0 {
			continue
		}
		states := grouped[serverID]
		if states == nil {
			states = make(map[string]RuntimeState)
			grouped[serverID] = states
		}
		key := ruleStateKey(event.RuleID, event.RuleGeneration)
		states[key] = RuntimeState{
			Phase:              RuntimePhaseFiring,
			RuleID:             event.RuleID,
			Generation:         event.RuleGeneration,
			PendingSince:       formatRuntimeTime(event.FirstTriggerAt),
			FiringSince:        formatRuntimeTime(event.FirstTriggerAt),
			LastDBHeartbeatAt:  formatRuntimeTime(event.LastTriggerAt),
			LastObservedAt:     formatRuntimeTime(event.LastTriggerAt),
			LastEvalAt:         formatRuntimeTime(now),
			CurrentValue:       float64OrZero(event.CurrentValue),
			EffectiveThreshold: float64OrZero(event.EffectiveThreshold),
			EventID:            event.ID,
		}
	}
	return grouped
}

func float64OrZero(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func ruleFromEvent(event model.AlertEvent) (CompiledRule, error) {
	rule := CompiledRule{
		RuleID:     event.RuleID,
		Generation: event.RuleGeneration,
		Snapshot:   RuleSnapshot{RuleID: event.RuleID, Generation: event.RuleGeneration},
	}
	if err := json.Unmarshal(event.RuleSnapshot, &rule.Snapshot); err != nil {
		return rule, fmt.Errorf("decode rule_snapshot for event %d: %w", event.ID, err)
	}
	if err := validateSnapshot(event, rule.Snapshot); err != nil {
		return rule, err
	}
	rule.Name = rule.Snapshot.Name
	rule.Builtin = rule.Snapshot.Builtin
	rule.Metric = rule.Snapshot.Metric
	rule.Operator = rule.Snapshot.Operator
	rule.Threshold = rule.Snapshot.Threshold
	rule.DurationSec = rule.Snapshot.DurationSec
	rule.CooldownMin = rule.Snapshot.CooldownMin
	rule.ThresholdMode = rule.Snapshot.ThresholdMode
	rule.ThresholdOffset = rule.Snapshot.ThresholdOffset
	if updatedAt, err := time.Parse(time.RFC3339, rule.Snapshot.UpdatedAt); err == nil {
		rule.GenerationUpdatedAt = updatedAt.UTC()
	}
	return rule, nil
}

func validateSnapshot(event model.AlertEvent, snapshot RuleSnapshot) error {
	if snapshot.RuleID != event.RuleID {
		return fmt.Errorf("rule_snapshot rule_id mismatch for event %d", event.ID)
	}
	if snapshot.Generation != event.RuleGeneration {
		return fmt.Errorf("rule_snapshot generation mismatch for event %d", event.ID)
	}
	if snapshot.Metric == "" || snapshot.Operator == "" || snapshot.ThresholdMode == "" {
		return fmt.Errorf("rule_snapshot is incomplete for event %d", event.ID)
	}
	if snapshot.DurationSec < 0 {
		return fmt.Errorf("rule_snapshot duration_sec is invalid for event %d", event.ID)
	}
	if snapshot.CooldownMin < 0 {
		return fmt.Errorf("rule_snapshot cooldown_min is invalid for event %d", event.ID)
	}
	return nil
}
