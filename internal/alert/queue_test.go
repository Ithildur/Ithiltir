package alert

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"dash/internal/model"
	alertstore "dash/internal/store/alert"
	pgtest "dash/internal/testutil/postgres"

	"gorm.io/datatypes"
)

func TestDecodeRuntimeStateKeepsValidFieldsAndReportsCorruptFields(t *testing.T) {
	now := time.Date(2026, time.July, 12, 10, 0, 0, 0, time.UTC)
	valid := RuntimeState{
		Phase:        RuntimePhasePending,
		RuleID:       7,
		Generation:   3,
		PendingSince: formatRuntimeTime(now),
	}
	validRaw, err := json.Marshal(valid)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	loaded := decodeRuntimeState(map[string]string{
		ruleStateKey(7, 3): string(validRaw),
		"8:1":              "{",
		"9:1":              `{}`,
		"10:1":             `{"phase":"pending","rule_id":11,"generation":1,"pending_since":"2026-07-12T10:00:00Z"}`,
	})
	if got := loaded.States[ruleStateKey(7, 3)]; !runtimeStateEqual(got, valid) {
		t.Fatalf("valid runtime = %+v, want %+v", got, valid)
	}
	wantCorrupt := []string{"10:1", "8:1", "9:1"}
	if !reflect.DeepEqual(loaded.Corrupt, wantCorrupt) {
		t.Fatalf("corrupt fields = %v, want %v", loaded.Corrupt, wantCorrupt)
	}
}

func TestIntegrationLoadRuntimeStateRepairsCorruptFieldsFromOpenEvents(t *testing.T) {
	ctx := t.Context()
	db := pgtest.NewDB(t)
	st := alertstore.New(db, testNotifyConfigCipher(t))
	now := time.Now().UTC().Truncate(time.Second)
	currentValue := 91.5
	threshold := 90.0
	event := model.AlertEvent{
		RuleID:             7,
		RuleGeneration:     3,
		RuleSnapshot:       datatypes.JSON(`{"rule_id":7,"generation":3}`),
		ObjectType:         model.ObjectTypeServer,
		ObjectID:           42,
		Status:             model.AlertStatusOpen,
		FirstTriggerAt:     now.Add(-time.Minute),
		LastTriggerAt:      now,
		CurrentValue:       &currentValue,
		EffectiveThreshold: &threshold,
	}
	if err := db.WithContext(ctx).Create(&event).Error; err != nil {
		t.Fatalf("Create(alert event) error = %v", err)
	}

	pendingKey := ruleStateKey(8, 1)
	pending := RuntimeState{
		Phase:        RuntimePhasePending,
		RuleID:       8,
		Generation:   1,
		PendingSince: formatRuntimeTime(now.Add(-30 * time.Second)),
	}
	pendingRaw, err := json.Marshal(pending)
	if err != nil {
		t.Fatalf("json.Marshal(pending) error = %v", err)
	}
	openKey := ruleStateKey(event.RuleID, event.RuleGeneration)
	orphanKey := ruleStateKey(9, 1)
	if err := st.SaveAlertRuntime(ctx, event.ObjectID, nil, map[string][]byte{
		pendingKey: pendingRaw,
		openKey:    []byte("{"),
		orphanKey:  []byte(`{}`),
	}, false); err != nil {
		t.Fatalf("SaveAlertRuntime() error = %v", err)
	}

	svc := &Service{store: st}
	got, err := svc.loadRuntimeState(ctx, event.ObjectID)
	if err != nil {
		t.Fatalf("loadRuntimeState() error = %v", err)
	}
	if state := got[pendingKey]; !runtimeStateEqual(state, pending) {
		t.Fatalf("pending runtime = %+v, want %+v", state, pending)
	}
	restored, ok := got[openKey]
	if !ok || restored.Phase != RuntimePhaseFiring || restored.EventID != event.ID {
		t.Fatalf("restored runtime = %+v, want firing event %d", restored, event.ID)
	}
	if restored.RuleID != event.RuleID || restored.Generation != event.RuleGeneration ||
		!restored.FiringSinceTime().Equal(event.FirstTriggerAt) ||
		!restored.PendingSinceTime().Equal(event.FirstTriggerAt) ||
		!restored.LastDBHeartbeatAtTime().Equal(event.LastTriggerAt) ||
		!restored.LastObservedAtTime().Equal(event.LastTriggerAt) ||
		restored.CurrentValue != currentValue || restored.EffectiveThreshold != threshold {
		t.Fatalf("restored runtime does not match open event: %+v", restored)
	}
	if _, ok := got[orphanKey]; ok {
		t.Fatalf("orphan corrupt runtime %q was retained", orphanKey)
	}

	raw, err := st.LoadAlertRuntime(ctx, event.ObjectID)
	if err != nil {
		t.Fatalf("LoadAlertRuntime() error = %v", err)
	}
	if _, ok := raw[orphanKey]; ok {
		t.Fatalf("orphan corrupt field %q was not deleted", orphanKey)
	}
	loaded := decodeRuntimeState(raw)
	if len(loaded.Corrupt) != 0 {
		t.Fatalf("repaired cache still has corrupt fields: %v", loaded.Corrupt)
	}
	if state := loaded.States[openKey]; state != restored {
		t.Fatalf("persisted runtime = %+v, want %+v", state, restored)
	}
}
