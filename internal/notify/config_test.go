package notify

import (
	"encoding/json"
	"errors"
	"testing"

	"dash/internal/model"
)

func TestMTProtoSessionFromConfig(t *testing.T) {
	session, isMTProto, err := SessionFromConfig([]byte(`{"mode":"mtproto","api_id":123,"api_hash":"hash","phone":"+10000000000","chat_id":"-1001","session":"session-text"}`))
	if err != nil {
		t.Fatalf("SessionFromConfig returned error: %v", err)
	}
	if !isMTProto || session != "session-text" {
		t.Fatalf("expected mtproto session, got %q, %v", session, isMTProto)
	}
}

func TestMTProtoSessionFromConfigIgnoresBotConfig(t *testing.T) {
	session, isMTProto, err := SessionFromConfig([]byte(`{"mode":"bot","bot_token":"token","chat_id":"-1001"}`))
	if err != nil {
		t.Fatalf("SessionFromConfig returned error: %v", err)
	}
	if isMTProto || session != "" {
		t.Fatalf("expected bot config to be ignored, got %q, %v", session, isMTProto)
	}
}

func TestMTProtoSessionFromConfigRejectsMalformedConfig(t *testing.T) {
	_, _, err := SessionFromConfig([]byte(`{"mode":"mtproto"}`))
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestNormalizeConfigForUpdateKeepsBlankSecrets(t *testing.T) {
	bot := updatedConfig(t, model.NotifyTypeTelegram,
		`{"mode":"bot","bot_token":"old-token","chat_id":"old-chat"}`,
		`{"mode":"bot","bot_token":"","chat_id":"new-chat"}`,
	).(TelegramBotConfig)
	if bot.BotToken != "old-token" || bot.ChatID != "new-chat" {
		t.Fatalf("unexpected bot config: %+v", bot)
	}

	email := updatedConfig(t, model.NotifyTypeEmail,
		`{"smtp_host":"smtp.example.com","smtp_port":587,"username":"user","password":"old-pass","from":"a@example.com","to":["b@example.com"],"use_tls":true}`,
		`{"smtp_host":"smtp.example.com","smtp_port":587,"username":"user","password":"","from":"a@example.com","to":["c@example.com"],"use_tls":true}`,
	).(EmailConfig)
	if email.Password != "old-pass" || len(email.To) != 1 || email.To[0] != "c@example.com" {
		t.Fatalf("unexpected email config: %+v", email)
	}

	webhook := updatedConfig(t, model.NotifyTypeWebhook,
		`{"url":"https://example.com/old","secret":"old-secret"}`,
		`{"url":"https://example.com/new","secret":""}`,
	).(WebhookConfig)
	if webhook.Secret != "old-secret" || webhook.URL != "https://example.com/new" {
		t.Fatalf("unexpected webhook config: %+v", webhook)
	}
}

func TestNormalizeConfigForUpdateKeepsMTProtoSession(t *testing.T) {
	cfg := updatedConfig(t, model.NotifyTypeTelegram,
		`{"mode":"mtproto","api_id":123,"api_hash":"old-hash","phone":"+10000000000","chat_id":"-1001","session":"old-session","username":"old-user"}`,
		`{"mode":"mtproto","api_id":123,"api_hash":"","phone":"+10000000000","chat_id":"-1002"}`,
	).(TelegramMTProtoConfig)
	if cfg.APIHash != "old-hash" || cfg.Session != "old-session" || cfg.Username != "old-user" || cfg.ChatID != "-1002" {
		t.Fatalf("unexpected mtproto config: %+v", cfg)
	}
}

func TestNormalizeConfigForUpdateDropsMTProtoSession(t *testing.T) {
	cfg := updatedConfig(t, model.NotifyTypeTelegram,
		`{"mode":"mtproto","api_id":123,"api_hash":"old-hash","phone":"+10000000000","chat_id":"-1001","session":"old-session"}`,
		`{"mode":"mtproto","api_id":123,"api_hash":"new-hash","phone":"+10000000000","chat_id":"-1001"}`,
	).(TelegramMTProtoConfig)
	if cfg.Session != "" {
		t.Fatalf("expected session to be cleared, got %q", cfg.Session)
	}
}

func updatedConfig(t *testing.T, typ model.NotifyType, previous, next string) any {
	t.Helper()
	raw, err := NormalizeConfigForUpdate(typ, json.RawMessage(next), typ, json.RawMessage(previous))
	if err != nil {
		t.Fatalf("NormalizeConfigForUpdate returned error: %v", err)
	}
	cfg, err := DecodeConfig(typ, raw)
	if err != nil {
		t.Fatalf("DecodeConfig returned error: %v", err)
	}
	return cfg
}
