package alert

import (
	"context"
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
	grouped := runtimeStatesFromOpenEvents(events)
	s.runtimeMu.Lock()
	s.runtime = grouped
	s.runtimeMu.Unlock()
	return nil
}

func (s *Service) restoreOpenRuntime(ctx context.Context) error {
	events, err := s.store.ListOpenEvents(ctx)
	if err != nil {
		return err
	}
	grouped := runtimeStatesFromOpenEvents(events)
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	for serverID, states := range grouped {
		current := s.runtime[serverID]
		if current == nil {
			s.runtime[serverID] = states
			continue
		}
		for key, state := range states {
			existing, ok := current[key]
			if ok && existing.Phase == RuntimePhaseFiring && existing.EventID == state.EventID {
				continue
			}
			current[key] = state
		}
	}
	return nil
}

func runtimeStatesFromOpenEvents(events []model.AlertEvent) map[int64]map[string]RuntimeState {
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
			PendingSince:       event.FirstTriggerAt.UTC().Truncate(time.Second),
			FiringSince:        event.FirstTriggerAt.UTC().Truncate(time.Second),
			LastDBHeartbeatAt:  event.LastTriggerAt.UTC().Truncate(time.Second),
			LastObservedAt:     event.LastTriggerAt.UTC().Truncate(time.Second),
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
