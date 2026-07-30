package alert

import (
	"context"
	"maps"
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
		loaded, err := readRuntimeState(ctx, s.store, serverID)
		if err != nil {
			return err
		}
		// Runtime is a cache: startup rebuild replaces it with DB-open firing truth.
		next := make(map[string]RuntimeState, len(grouped[serverID]))
		for key, state := range grouped[serverID] {
			next[key] = state
		}
		if err := saveRuntimeState(ctx, s.store, serverID, loaded.States, next, loaded.Corrupt...); err != nil {
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
		loaded, err := readRuntimeState(ctx, s.store, serverID)
		if err != nil {
			return err
		}
		current := loaded.States
		next := current
		changed := len(loaded.Corrupt) > 0
		copied := false
		for key, state := range states {
			existing, ok := current[key]
			if ok && existing.Phase == RuntimePhaseFiring && existing.EventID > 0 && existing.EventID == state.EventID {
				continue
			}
			if !copied {
				next = maps.Clone(current)
				copied = true
			}
			next[key] = state
			changed = true
		}
		if !changed {
			continue
		}
		if err := saveRuntimeState(ctx, s.store, serverID, current, next, loaded.Corrupt...); err != nil {
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
