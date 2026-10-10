package alert

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"dash/internal/infra"
	"dash/internal/model"
	"dash/internal/notify"
	alertstore "dash/internal/store/alert"
	pgtest "dash/internal/testutil/postgres"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestIntegrationDefaultNotificationDelivery(t *testing.T) {
	db := pgtest.NewDB(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	store := alertstore.New(db, pgtest.ConfigCipher(t))
	service := &Service{
		store:  store,
		notify: newNotifyCache(store, 0),
		logger: infra.WithModule("alert"),
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
	var received atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	channels := []model.NotifyChannel{
		{
			Name:     "system-updates",
			Type:     model.NotifyTypeWebhook,
			Config:   datatypes.JSON(`{"language":"system","url":"` + server.URL + `/system"}`),
			Enabled:  true,
			Revision: 1,
		},
		{
			Name:     "english-updates",
			Type:     model.NotifyTypeWebhook,
			Config:   datatypes.JSON(`{"language":"en","url":"` + server.URL + `/en"}`),
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

	// Fail only the first outbox write after the remote endpoint has accepted it.
	var failedCtx context.Context
	const callback = "test:fail_notification_completion"
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if received.Load() == 0 || tx.Statement.Table != (model.AlertNotificationOutbox{}).TableName() {
			return
		}
		if failedCtx == nil {
			failedCtx = tx.Statement.Context
			tx.AddError(errors.New("temporary completion failure"))
			return
		}
		if !errors.Is(failedCtx.Err(), context.Canceled) {
			t.Errorf("failed attempt context = %v, want canceled before retry", failedCtx.Err())
		}
		before, hadDeadline := failedCtx.Deadline()
		after, hasDeadline := tx.Statement.Context.Deadline()
		if !hadDeadline || !hasDeadline || !after.After(before) {
			t.Error("completion retry did not receive a fresh timeout budget")
		}
	}); err != nil {
		t.Fatalf("register completion failure: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Callback().Update().Remove(callback); err != nil {
			t.Errorf("remove completion failure: %v", err)
		}
	})
	processed, err := service.processNotifications(ctx)
	if err != nil || !processed {
		t.Fatalf("processNotifications() = %v, %v", processed, err)
	}
	if failedCtx == nil {
		t.Fatal("completion write failure was not exercised")
	}
	if got := received.Load(); got != 2 {
		t.Fatalf("webhook received %d requests, want 2", got)
	}
	if err := db.Where("channel_id IN ?", channelIDs).Find(&rows).Error; err != nil {
		t.Fatalf("load delivered notifications: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("delivered notifications = %d, want 2", len(rows))
	}
	for _, row := range rows {
		if row.Status != model.OutboxStatusSent || row.AttemptCount != 1 || row.SentAt == nil {
			t.Fatalf("notification = %+v, want one completed delivery", row)
		}
	}
}
