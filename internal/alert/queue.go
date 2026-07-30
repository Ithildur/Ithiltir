package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	alertstore "dash/internal/store/alert"
	kitlog "github.com/Ithildur/EiluneKit/logging"
)

type runtimeStateRead struct {
	States  map[string]RuntimeState
	Corrupt []string
}

func readRuntimeState(ctx context.Context, st *alertstore.Store, serverID int64) (runtimeStateRead, error) {
	values, err := st.LoadAlertRuntime(ctx, serverID)
	if err != nil {
		return runtimeStateRead{}, fmt.Errorf("load alert runtime for server %d: %w", serverID, err)
	}
	return decodeRuntimeState(values), nil
}

func decodeRuntimeState(values map[string]string) runtimeStateRead {
	out := make(map[string]RuntimeState, len(values))
	corrupt := make([]string, 0)
	for key, raw := range values {
		var state RuntimeState
		if err := json.Unmarshal([]byte(raw), &state); err != nil {
			corrupt = append(corrupt, key)
			continue
		}
		if !validRuntimeState(key, state) {
			corrupt = append(corrupt, key)
			continue
		}
		out[key] = state
	}
	sort.Strings(corrupt)
	return runtimeStateRead{States: out, Corrupt: corrupt}
}

func validRuntimeState(key string, state RuntimeState) bool {
	if state.RuleID == 0 || state.Generation <= 0 || key != ruleStateKey(state.RuleID, state.Generation) {
		return false
	}
	for _, raw := range []string{
		state.PendingSince,
		state.FiringSince,
		state.CooldownUntil,
		state.LastDBHeartbeatAt,
		state.LastObservedAt,
		state.LastEvalAt,
	} {
		if strings.TrimSpace(raw) != "" && parseRuntimeTime(raw).IsZero() {
			return false
		}
	}
	switch state.Phase {
	case RuntimePhasePending:
		return !parseRuntimeTime(state.PendingSince).IsZero()
	case RuntimePhaseFiring:
		return state.EventID > 0 && !parseRuntimeTime(state.FiringSince).IsZero()
	case RuntimePhaseCooldown:
		return !parseRuntimeTime(state.CooldownUntil).IsZero()
	default:
		return false
	}
}

func (s *Service) loadRuntimeState(ctx context.Context, serverID int64) (map[string]RuntimeState, error) {
	loaded, err := readRuntimeState(ctx, s.store, serverID)
	if err != nil {
		return nil, err
	}
	if len(loaded.Corrupt) == 0 {
		return loaded.States, nil
	}

	events, err := s.store.ListOpenEventsByServer(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("load open events to repair alert runtime for server %d: %w", serverID, err)
	}
	restored := runtimeStatesFromOpenEvents(events, time.Now().UTC())[serverID]
	next := maps.Clone(loaded.States)
	for key, state := range restored {
		next[key] = state
	}
	if err := saveRuntimeState(ctx, s.store, serverID, loaded.States, next, loaded.Corrupt...); err != nil {
		return nil, fmt.Errorf("repair alert runtime for server %d: %w", serverID, err)
	}
	if s.logger != nil {
		s.logger.Info(
			"repaired corrupt alert runtime",
			nil,
			kitlog.Int64("server_id", serverID),
			kitlog.Int("corrupt_fields", len(loaded.Corrupt)),
			kitlog.Int("restored_firing_states", len(restored)),
		)
	}
	return next, nil
}

func saveRuntimeState(ctx context.Context, st *alertstore.Store, serverID int64, current, next map[string]RuntimeState, extraDeletes ...string) error {
	deleteSet := make(map[string]struct{}, len(current)+len(extraDeletes))
	for field := range current {
		if _, ok := next[field]; !ok {
			deleteSet[field] = struct{}{}
		}
	}
	for _, field := range extraDeletes {
		// A restored firing state overwrites the corrupt value directly; do not
		// delete the field that the same update is repairing.
		if _, restored := next[field]; !restored {
			deleteSet[field] = struct{}{}
		}
	}
	deletes := make([]string, 0, len(deleteSet))
	for field := range deleteSet {
		deletes = append(deletes, field)
	}
	sort.Strings(deletes)

	updates := make(map[string][]byte)
	for field, state := range next {
		if currentState, ok := current[field]; ok && runtimeStateEqual(currentState, state) {
			continue
		}
		raw, err := json.Marshal(state)
		if err != nil {
			return err
		}
		updates[field] = raw
	}
	if err := st.SaveAlertRuntime(ctx, serverID, deletes, updates, len(next) == 0); err != nil {
		return fmt.Errorf("save alert runtime for server %d: %w", serverID, err)
	}
	return nil
}

func runtimeStateEqual(a, b RuntimeState) bool {
	return a.Phase == b.Phase &&
		a.RuleID == b.RuleID &&
		a.Generation == b.Generation &&
		a.PendingSince == b.PendingSince &&
		a.FiringSince == b.FiringSince &&
		a.CooldownUntil == b.CooldownUntil &&
		a.LastDBHeartbeatAt == b.LastDBHeartbeatAt &&
		a.LastObservedAt == b.LastObservedAt &&
		a.LastEvalAt == b.LastEvalAt &&
		a.CurrentValue == b.CurrentValue &&
		a.EffectiveThreshold == b.EffectiveThreshold &&
		a.EventID == b.EventID
}
