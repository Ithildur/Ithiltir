package alert

import (
	"context"
	"testing"
	"time"

	"dash/internal/model"
	"dash/internal/notify"
	alertstore "dash/internal/store/alert"
	pgtest "dash/internal/testutil/postgres"

	"gorm.io/datatypes"
)

func TestEnqueueDefaultSkipsWithoutTargets(t *testing.T) {
	store := alertstore.New(nil, testNotifyConfigCipher(t))
	cache := newNotifyCache(store, time.Hour)
	cache.current = notifyTargets{
		Enabled:     true,
		RefreshedAt: time.Now(),
		Ready:       true,
	}
	cache.ready = true
	service := &Service{store: store, notify: cache}

	status, err := service.EnqueueDefault(context.Background(), "system:test", notify.Message{
		Title:    "test",
		Metadata: map[string]string{"event": "test"},
	})
	if err != nil {
		t.Fatalf("EnqueueDefault() error = %v", err)
	}
	if status != notify.EnqueueSkippedNoTargets {
		t.Fatalf("EnqueueDefault() status = %q, want %q", status, notify.EnqueueSkippedNoTargets)
	}
}

func TestIntegrationEnqueueDefaultResolvesTargetsAndPersistsOnce(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)
	store := alertstore.New(db, testNotifyConfigCipher(t))
	service := &Service{
		store:  store,
		notify: newNotifyCache(store, 0),
	}
	message := notify.Message{
		Title: "Dash update available",
		Body:  "1.1.0",
		Metadata: map[string]string{
			"kind":  "dash_update",
			"event": "available",
		},
	}
	key := "dash-update:available:release:1.1.0"

	status, err := service.EnqueueDefault(ctx, key, message)
	if err != nil {
		t.Fatalf("EnqueueDefault(no targets) error = %v", err)
	}
	if status != notify.EnqueueSkippedNoTargets {
		t.Fatalf("EnqueueDefault(no targets) status = %q, want %q", status, notify.EnqueueSkippedNoTargets)
	}

	channel := model.NotifyChannel{
		Name:     "updates",
		Type:     model.NotifyTypeWebhook,
		Config:   datatypes.JSON(`{"url":"https://example.com/hook"}`),
		Enabled:  true,
		Revision: 1,
	}
	if err := store.CreateChannel(ctx, &channel); err != nil {
		t.Fatalf("CreateChannel() error = %v", err)
	}
	if err := store.ReplaceSettings(ctx, true, []int64{channel.ID}); err != nil {
		t.Fatalf("ReplaceSettings() error = %v", err)
	}
	for range 2 {
		status, err = service.EnqueueDefault(ctx, key, message)
		if err != nil {
			t.Fatalf("EnqueueDefault() error = %v", err)
		}
		if status != notify.EnqueueQueued {
			t.Fatalf("EnqueueDefault() status = %q, want %q", status, notify.EnqueueQueued)
		}
	}

	var rows []model.AlertNotificationOutbox
	if err := db.Where("channel_id = ?", channel.ID).Find(&rows).Error; err != nil {
		t.Fatalf("load notification outbox: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("notification outbox rows = %d, want 1", len(rows))
	}
	if rows[0].EventID != nil || rows[0].Transition != "available" || rows[0].Status != model.OutboxStatusPending {
		t.Fatalf("notification outbox row = %+v", rows[0])
	}
}

func testNotifyConfigCipher(t *testing.T) *notify.ConfigCipher {
	t.Helper()
	configCipher, err := notify.NewConfigCipher(make([]byte, notify.ConfigKeySize))
	if err != nil {
		t.Fatalf("NewConfigCipher() error = %v", err)
	}
	return configCipher
}
