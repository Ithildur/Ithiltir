package alert

import (
	"errors"
	"testing"
	"time"

	"dash/internal/model"
	pgtest "dash/internal/testutil/postgres"
)

func TestIntegrationOpenEventReplay(t *testing.T) {
	db := pgtest.NewDB(t)
	st := newTestStore(t, db)
	channel, fixture := createNotificationFixture(t, db, "open-replay")
	params := AlertOpenEventParams{
		RuleGeneration: 1, RuleSnapshot: []byte(`{}`),
		ObjectType: model.ObjectTypeServer, ObjectID: fixture.ObjectID,
		TriggeredAt: time.Now().UTC(),
		Notifications: []AlertNotificationParams{{
			ChannelID: channel.ID, ChannelType: channel.Type, Transition: "opened",
			Payload: NotificationPayload{Title: "open", Body: "original"},
		}},
	}
	for _, ruleID := range []int64{1, -1} {
		params.RuleID, params.Builtin = ruleID, ruleID < 0
		first, err := st.WriteOpenTransition(t.Context(), params)
		if err != nil || !first.Created || first.EventID == 0 {
			t.Fatalf("open rule %d: %+v, %v", ruleID, first, err)
		}
		replayed, err := st.WriteOpenTransition(t.Context(), params)
		if err != nil || replayed.Created || replayed.EventID != first.EventID {
			t.Fatalf("replay rule %d: %+v, %v", ruleID, replayed, err)
		}
		var count int64
		if err := db.Model(&model.AlertNotificationOutbox{}).Where("event_id = ?", first.EventID).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("replayed outbox count = %d, error %v", count, err)
		}
		if !params.Builtin {
			if err := db.Model(&model.AlertRule{}).Where("id = ?", ruleID).Update("enabled", false).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := st.WriteOpenTransition(t.Context(), params); !errors.Is(err, ErrAlertRuleVersionStale) {
				t.Fatalf("disabled rule replay error = %v, want stale rule", err)
			}
		}
	}

	params.RuleID = -2
	params.Notifications[0].ChannelType = ""
	if _, err := st.WriteOpenTransition(t.Context(), params); err == nil {
		t.Fatal("invalid notification was silently accepted")
	}
	var count int64
	if err := db.Model(&model.AlertEvent{}).Where("rule_id = ? AND object_id = ?", params.RuleID, params.ObjectID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("failed notification left %d events, error %v", count, err)
	}
}
