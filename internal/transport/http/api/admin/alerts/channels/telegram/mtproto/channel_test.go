package mtproto

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"dash/internal/model"
	"dash/internal/notify"
	alertstore "dash/internal/store/alert"
	pgtest "dash/internal/testutil/postgres"

	"gorm.io/datatypes"
)

func TestIntegrationUpdateSessionRejectsLoginForReplacedChannel(t *testing.T) {
	ctx := context.Background()
	st := alertstore.New(pgtest.NewDB(t), pgtest.ConfigCipher(t))
	payload, err := json.Marshal(notify.TelegramMTProtoConfig{
		Mode:    "mtproto",
		APIID:   123,
		APIHash: "old-hash",
		Phone:   "+10000000000",
		ChatID:  "-1001",
		Session: "old-session",
	})
	if err != nil {
		t.Fatalf("marshal channel config: %v", err)
	}
	channel := model.NotifyChannel{
		Name:    "telegram",
		Type:    model.NotifyTypeTelegram,
		Config:  datatypes.JSON(payload),
		Enabled: true,
	}
	if err := st.CreateChannel(ctx, &channel); err != nil {
		t.Fatalf("CreateChannel() error = %v", err)
	}
	stored, err := st.GetChannel(ctx, channel.ID)
	if err != nil {
		t.Fatalf("GetChannel() error = %v", err)
	}
	originalRevision := stored.Revision

	next := *stored
	next.Name = "replacement"
	if err := st.ReplaceChannel(ctx, stored.ID, originalRevision, next); err != nil {
		t.Fatalf("ReplaceChannel() error = %v", err)
	}
	if err := updateSession(ctx, st, stored.ID, originalRevision, "stale-session"); !errors.Is(err, alertstore.ErrChannelVersionStale) {
		t.Fatalf("updateSession() error = %v, want ErrChannelVersionStale", err)
	}

	got, err := st.GetChannel(ctx, stored.ID)
	if err != nil {
		t.Fatalf("GetChannel(replaced) error = %v", err)
	}
	var config notify.TelegramMTProtoConfig
	if err := json.Unmarshal(got.Config, &config); err != nil {
		t.Fatalf("decode replaced config: %v", err)
	}
	if config.Session != "old-session" {
		t.Fatalf("stored session = %q, want old-session", config.Session)
	}
}
