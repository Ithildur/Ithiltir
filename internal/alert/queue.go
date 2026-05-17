package alert

import (
	"context"
	"encoding/json"

	alertstore "dash/internal/store/alert"
)

func loadRuntimeState(ctx context.Context, st *alertstore.Store, serverID int64) (map[string]RuntimeState, error) {
	values, err := st.LoadAlertRuntime(ctx, serverID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]RuntimeState, len(values))
	for key, raw := range values {
		var state RuntimeState
		if err := json.Unmarshal([]byte(raw), &state); err != nil {
			continue
		}
		out[key] = state
	}
	return out, nil
}

func saveRuntimeState(ctx context.Context, st *alertstore.Store, serverID int64, current, next map[string]RuntimeState) error {
	deletes := make([]string, 0)
	for field := range current {
		if _, ok := next[field]; !ok {
			deletes = append(deletes, field)
		}
	}
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
	return st.SaveAlertRuntime(ctx, serverID, deletes, updates, len(next) == 0)
}

func runtimeStateEqual(a, b RuntimeState) bool {
	return a.Phase == b.Phase &&
		a.RuleID == b.RuleID &&
		a.Generation == b.Generation &&
		a.PendingSince == b.PendingSince &&
		a.FiringSince == b.FiringSince &&
		a.LastDBHeartbeatAt == b.LastDBHeartbeatAt &&
		a.LastObservedAt == b.LastObservedAt &&
		a.LastEvalAt == b.LastEvalAt &&
		a.CurrentValue == b.CurrentValue &&
		a.EffectiveThreshold == b.EffectiveThreshold &&
		a.EventID == b.EventID
}
