package alert

import (
	"context"
	"strings"
	"testing"
	"time"

	"dash/internal/model"
	alertstore "dash/internal/store/alert"

	"gorm.io/datatypes"
)

func TestOpenNotificationParamsRenderPerChannelLanguage(t *testing.T) {
	store := alertstore.New(nil, testNotifyConfigCipher(t))
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
	cache.ready = true
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
