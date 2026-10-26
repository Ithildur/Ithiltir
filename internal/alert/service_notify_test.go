package alert

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"dash/internal/infra"
	"dash/internal/model"
	alertstore "dash/internal/store/alert"
	pgtest "dash/internal/testutil/postgres"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestOpenNotificationParamsRenderPerChannelLanguage(t *testing.T) {
	store := alertstore.New(nil, pgtest.ConfigCipher(t))
	cache := newNotifyCache(store, time.Hour)
	cache.current = notifyTargets{
		Enabled: true,
		Channels: []model.NotifyChannel{
			{
				ID:      1,
				Type:    model.NotifyTypeWebhook,
				Config:  datatypes.JSON(`{"language":"en","url":"https://example.com/en"}`),
				Enabled: true,
			},
			{
				ID:      2,
				Type:    model.NotifyTypeWebhook,
				Config:  datatypes.JSON(`{"url":"https://example.com/system"}`),
				Enabled: true,
			},
		},
		RefreshedAt: time.Now(),
		Ready:       true,
	}
	service := &Service{
		notify:  cache,
		message: MessageConfig{Language: messageLanguageZH, Location: time.UTC},
	}
	transition := OpenTransition{
		Rule: CompiledRule{
			RuleID:      1,
			Name:        "CPU high",
			Metric:      "cpu.usage_ratio",
			Operator:    ">",
			Threshold:   0.9,
			DurationSec: 60,
		},
		ObjectID:           42,
		TriggeredAt:        time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC),
		CurrentValue:       0.95,
		EffectiveThreshold: 0.9,
	}

	params, err := service.openNotificationParams(context.Background(), transition)
	if err != nil {
		t.Fatalf("openNotificationParams() error = %v", err)
	}
	if len(params) != 2 {
		t.Fatalf("openNotificationParams() count = %d, want 2", len(params))
	}
	if !strings.HasPrefix(params[0].Payload.Title, "Alert triggered:") {
		t.Fatalf("English channel title = %q", params[0].Payload.Title)
	}
	if !strings.HasPrefix(params[1].Payload.Title, "告警触发:") {
		t.Fatalf("system-language channel title = %q", params[1].Payload.Title)
	}
}

func TestIntegrationAlertNotificationBatch(t *testing.T) {
	db := pgtest.NewDB(t)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	store := alertstore.New(db, pgtest.ConfigCipher(t))
	service := &Service{store: store, logger: infra.WithModule("alert")}
	channel := model.NotifyChannel{
		Name: "batch", Type: model.NotifyTypeTelegram, Enabled: true,
		Config: datatypes.JSON(`{"mode":"bot","bot_token":"123:test","chat_id":"42"}`),
	}
	if err := store.CreateChannel(ctx, &channel); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for i := range 6 {
		event := model.AlertEvent{
			RuleID: 1, RuleGeneration: 1, RuleSnapshot: datatypes.JSON(`{}`),
			ObjectType: model.ObjectTypeServer, ObjectID: int64(i + 1),
			Status: model.AlertStatusClosed, FirstTriggerAt: now, LastTriggerAt: now, ClosedAt: &now,
		}
		if err := db.Create(&event).Error; err != nil {
			t.Fatal(err)
		}
		for _, transition := range []string{"opened", "closed"} {
			payload, err := json.Marshal(alertstore.NotificationPayload{
				Title: fmt.Sprintf("%s node-%d", transition, i), Body: "触发时间: original-time",
				Metadata: map[string]string{"transition": transition, "event_id": fmt.Sprint(event.ID)},
			})
			if err != nil {
				t.Fatal(err)
			}
			row := model.AlertNotificationOutbox{
				EventID: &event.ID, Transition: transition, ChannelID: channel.ID, ChannelType: channel.Type,
				Payload: payload, DedupeKey: fmt.Sprintf("%d:%s", event.ID, transition),
				Status: model.OutboxStatusPending, NextAttemptAt: now,
			}
			if err := db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
		}
	}

	var received []string
	original := http.DefaultTransport
	http.DefaultTransport = notificationTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.telegram.org" {
			return nil, fmt.Errorf("unexpected notification host: %s", r.URL.Host)
		}
		var payload struct {
			Text string `json:"text"`
		}
		if err := json.UnmarshalRead(r.Body, &payload); err != nil {
			return nil, err
		}
		received = append(received, payload.Text)
		status, body := http.StatusOK, `{"ok":true}`
		if len(received) == 1 {
			status, body = http.StatusTooManyRequests, `{"ok":false,"error_code":429,"parameters":{"retry_after":1}}`
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = original })

	if err := service.processNotifications(ctx); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 {
		t.Fatalf("requests = %d, want one failed alarm digest and no recovery", len(received))
	}
	var rows []model.AlertNotificationOutbox
	if err := db.Order("id ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Transition == "opened" {
			if row.Status != model.OutboxStatusRetry || row.AttemptCount != 1 || !row.NextAttemptAt.After(now) {
				t.Fatalf("failed alarm was not retained for retry: %+v", row)
			}
		} else if row.Status != model.OutboxStatusPending || row.AttemptCount != 0 {
			t.Fatalf("recovery overtook its alarm: %+v", row)
		}
	}
	if err := db.Model(&model.AlertNotificationOutbox{}).Where("status = ?", model.OutboxStatusRetry).
		Update("next_attempt_at", time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}

	// Fail midway through confirming a digest. The entire completion must retry
	// locally without sending the accepted digest again.
	completionWrites := 0
	const callback = "test:batch_completion_failure"
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		updates, ok := tx.Statement.Dest.(map[string]any)
		if !ok || tx.Statement.Table != "alert_notification_outbox" || updates["status"] != model.OutboxStatusSent {
			return
		}
		completionWrites++
		if completionWrites == 2 {
			tx.AddError(errors.New("temporary batch completion failure"))
		}
		if completionWrites == 3 {
			var sent int64
			if err := db.Model(&model.AlertNotificationOutbox{}).Where("status = ?", model.OutboxStatusSent).Count(&sent).Error; err != nil {
				t.Error(err)
			} else if sent != 0 {
				t.Errorf("failed batch committed %d rows before retry", sent)
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Callback().Update().Remove(callback) })
	if err := service.processNotifications(ctx); err != nil {
		t.Fatal(err)
	}
	if completionWrites < 3 {
		t.Fatal("batch completion rollback and retry were not exercised")
	}
	if len(received) != 3 {
		t.Fatalf("requests = %d, want failed alarm, alarm retry, recovery", len(received))
	}
	for i, transition := range []string{"opened", "opened", "closed"} {
		text := received[i]
		if strings.Count(text, "\n\n---\n\n") != 5 {
			t.Fatalf("digest does not contain six separated alerts: %q", text)
		}
		for i := range 6 {
			if !strings.Contains(text, fmt.Sprintf("%s node-%d", transition, i)) {
				t.Fatalf("digest lost %s node-%d: %q", transition, i, text)
			}
		}
		if strings.Count(text, "original-time") != 6 {
			t.Fatalf("digest lost original timestamps: %q", text)
		}
	}
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		wantAttempts := int32(1)
		if row.Transition == "opened" {
			wantAttempts = 2
		}
		if row.Status != model.OutboxStatusSent || row.AttemptCount != wantAttempts || row.SentAt == nil {
			t.Fatalf("incorrect final delivery state: %+v", row)
		}
	}
}

type notificationTransport func(*http.Request) (*http.Response, error)

func (transport notificationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}
