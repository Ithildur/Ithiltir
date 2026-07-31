package alert

import (
	"context"
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

func TestSaveRuntimeStateReplacesCorruptMemoryField(t *testing.T) {
	ctx := context.Background()
	st := alertstore.New(nil, testNotifyConfigCipher(t))

	const serverID = int64(42)
	key := ruleStateKey(7, 3)
	if err := st.SaveAlertRuntime(ctx, serverID, nil, map[string][]byte{key: []byte("{")}, false); err != nil {
		t.Fatalf("SaveAlertRuntime(corrupt) error = %v", err)
	}
	now := time.Date(2026, time.July, 12, 10, 0, 0, 0, time.UTC)
	restored := RuntimeState{
		Phase:       RuntimePhaseFiring,
		RuleID:      7,
		Generation:  3,
		FiringSince: formatRuntimeTime(now),
		EventID:     99,
	}
	if err := saveRuntimeState(
		ctx,
		st,
		serverID,
		map[string]RuntimeState{},
		map[string]RuntimeState{key: restored},
		key,
	); err != nil {
		t.Fatalf("saveRuntimeState() error = %v", err)
	}

	raw, err := st.LoadAlertRuntime(ctx, serverID)
	if err != nil {
		t.Fatalf("LoadAlertRuntime() error = %v", err)
	}
	loaded := decodeRuntimeState(raw)
	if len(loaded.Corrupt) != 0 {
		t.Fatalf("corrupt fields after repair = %v", loaded.Corrupt)
	}
	if got := loaded.States[key]; !runtimeStateEqual(got, restored) {
		t.Fatalf("repaired runtime = %+v, want %+v", got, restored)
	}
}

func TestIntegrationLoadRuntimeStateRepairsCorruptFieldsFromOpenEvents(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)
	st := alertstore.New(db, testNotifyConfigCipher(t))
	now := time.Date(2026, time.July, 12, 10, 0, 0, 0, time.UTC)
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
	if state := loaded.States[openKey]; state.EventID != event.ID {
		t.Fatalf("persisted firing event_id = %d, want %d", state.EventID, event.ID)
	}
}
