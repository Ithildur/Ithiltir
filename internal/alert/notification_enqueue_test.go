package alert

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"dash/internal/model"
	"dash/internal/notify"
	alertstore "dash/internal/store/alert"
	pgtest "dash/internal/testutil/postgres"

	"gorm.io/datatypes"
)

func TestIntegrationEnqueueDefaultResolvesTargetsAndPersistsOnce(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)
	store := alertstore.New(db, testNotifyConfigCipher(t))
	service := &Service{
		store:  store,
		notify: newNotifyCache(store, 0),
		message: MessageConfig{
			Language: messageLanguageZH,
			Location: time.UTC,
		},
	}
	messages := notify.Messages{
		Chinese: notify.Message{
			Title: "Dash 有可用更新",
			Body:  "1.1.0",
			Metadata: map[string]string{
				"kind":  "dash_update",
				"event": "available",
			},
		},
		English: notify.Message{
			Title: "Dash update available",
			Body:  "1.1.0",
			Metadata: map[string]string{
				"kind":  "dash_update",
				"event": "available",
			},
		},
	}
	key := "dash-update:available:release:1.1.0"

	status, err := service.EnqueueDefault(ctx, key, messages)
	if err != nil {
		t.Fatalf("EnqueueDefault(no targets) error = %v", err)
	}
	if status != notify.EnqueueSkippedNoTargets {
		t.Fatalf("EnqueueDefault(no targets) status = %q, want %q", status, notify.EnqueueSkippedNoTargets)
	}

	channels := []model.NotifyChannel{
		{
			Name:     "system-updates",
			Type:     model.NotifyTypeWebhook,
			Config:   datatypes.JSON(`{"language":"system","url":"https://example.com/system"}`),
			Enabled:  true,
			Revision: 1,
		},
		{
			Name:     "english-updates",
			Type:     model.NotifyTypeWebhook,
			Config:   datatypes.JSON(`{"language":"en","url":"https://example.com/en"}`),
			Enabled:  true,
			Revision: 1,
		},
	}
	for i := range channels {
		if err := store.CreateChannel(ctx, &channels[i]); err != nil {
			t.Fatalf("CreateChannel() error = %v", err)
		}
	}
	if err := store.ReplaceSettings(ctx, true, []int64{channels[0].ID, channels[1].ID}); err != nil {
		t.Fatalf("ReplaceSettings() error = %v", err)
	}
	for range 2 {
		status, err = service.EnqueueDefault(ctx, key, messages)
		if err != nil {
			t.Fatalf("EnqueueDefault() error = %v", err)
		}
		if status != notify.EnqueueQueued {
			t.Fatalf("EnqueueDefault() status = %q, want %q", status, notify.EnqueueQueued)
		}
	}

	var rows []model.AlertNotificationOutbox
	channelIDs := []int64{channels[0].ID, channels[1].ID}
	if err := db.Where("channel_id IN ?", channelIDs).Order("channel_id ASC").Find(&rows).Error; err != nil {
		t.Fatalf("load notification outbox: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("notification outbox rows = %d, want 2", len(rows))
	}
	wantTitles := map[int64]string{
		channels[0].ID: "Dash 有可用更新",
		channels[1].ID: "Dash update available",
	}
	for _, row := range rows {
		if row.EventID != nil || row.Transition != "available" || row.Status != model.OutboxStatusPending {
			t.Fatalf("notification outbox row = %+v", row)
		}
		var payload alertstore.NotificationPayload
		if err := json.Unmarshal(row.Payload, &payload); err != nil {
			t.Fatalf("decode notification payload: %v", err)
		}
		if payload.Title != wantTitles[row.ChannelID] {
			t.Fatalf("channel %d title = %q, want %q", row.ChannelID, payload.Title, wantTitles[row.ChannelID])
		}
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
