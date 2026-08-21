package notify

import (
	"encoding/json"
	"errors"
	"testing"

	"dash/internal/lang"
	"dash/internal/model"
)

func TestSessionFromConfig(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantSession string
		wantMTProto bool
		wantErr     error
	}{
		{
			name:        "MTProto",
			raw:         `{"mode":"mtproto","api_id":123,"api_hash":"hash","phone":"+10000000000","chat_id":"-1001","session":"session-text"}`,
			wantSession: "session-text",
			wantMTProto: true,
		},
		{name: "bot", raw: `{"mode":"bot","bot_token":"token","chat_id":"-1001"}`},
		{name: "malformed MTProto", raw: `{"mode":"mtproto"}`, wantErr: ErrInvalidConfig},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, isMTProto, err := SessionFromConfig([]byte(tt.raw))
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("SessionFromConfig() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || session != tt.wantSession || isMTProto != tt.wantMTProto {
				t.Fatalf("SessionFromConfig() = %q, %v, %v", session, isMTProto, err)
			}
		})
	}
}

func TestNormalizeConfigForUpdate(t *testing.T) {
	tests := []struct {
		name     string
		typ      model.NotifyType
		previous string
		next     string
		want     string
		wantErr  bool
	}{
		{
			name:     "blank bot token",
			typ:      model.NotifyTypeTelegram,
			previous: `{"mode":"bot","bot_token":"old-token","chat_id":"old-chat"}`,
			next:     `{"mode":"bot","bot_token":"","chat_id":"new-chat"}`,
			want:     `{"mode":"bot","bot_token":"old-token","chat_id":"new-chat"}`,
		},
		{
			name:     "blank email password",
			typ:      model.NotifyTypeEmail,
			previous: `{"smtp_host":"smtp.example.com","smtp_port":587,"username":"user","password":"old-pass","from":"a@example.com","to":["b@example.com"],"use_tls":true}`,
			next:     `{"smtp_host":"smtp.example.com","smtp_port":587,"username":"user","password":"","from":"a@example.com","to":["c@example.com"],"use_tls":true}`,
			want:     `{"smtp_host":"smtp.example.com","smtp_port":587,"username":"user","password":"old-pass","from":"a@example.com","to":["c@example.com"],"use_tls":true}`,
		},
		{
			name:     "blank webhook secret",
			typ:      model.NotifyTypeWebhook,
			previous: `{"url":"https://example.com/old","secret":"old-secret"}`,
			next:     `{"url":"https://example.com/new","secret":""}`,
			want:     `{"url":"https://example.com/new","secret":"old-secret"}`,
		},
		{
			name:     "unchanged MTProto login",
			typ:      model.NotifyTypeTelegram,
			previous: `{"mode":"mtproto","api_id":123,"api_hash":"old-hash","phone":"+10000000000","chat_id":"-1001","session":"old-session","username":"old-user"}`,
			next:     `{"mode":"mtproto","api_id":123,"api_hash":"","phone":"+10000000000","chat_id":"-1002"}`,
			want:     `{"mode":"mtproto","api_id":123,"api_hash":"old-hash","phone":"+10000000000","chat_id":"-1002","session":"old-session","username":"old-user"}`,
		},
		{
			name:     "changed MTProto login",
			typ:      model.NotifyTypeTelegram,
			previous: `{"mode":"mtproto","api_id":123,"api_hash":"old-hash","phone":"+10000000000","chat_id":"-1001","session":"old-session"}`,
			next:     `{"mode":"mtproto","api_id":123,"api_hash":"new-hash","phone":"+10000000000","chat_id":"-1001"}`,
			want:     `{"mode":"mtproto","api_id":123,"api_hash":"new-hash","phone":"+10000000000","chat_id":"-1001"}`,
		},
		{
			name:     "null bot token",
			typ:      model.NotifyTypeTelegram,
			previous: `{"mode":"bot","bot_token":"old-token","chat_id":"old-chat"}`,
			next:     `{"mode":"bot","bot_token":null,"chat_id":"new-chat"}`,
			wantErr:  true,
		},
		{
			name:     "null MTProto hash",
			typ:      model.NotifyTypeTelegram,
			previous: `{"mode":"mtproto","api_id":123,"api_hash":"old-hash","phone":"+10000000000","chat_id":"-1001"}`,
			next:     `{"mode":"mtproto","api_id":123,"api_hash":null,"phone":"+10000000000","chat_id":"-1002"}`,
			wantErr:  true,
		},
		{
			name:     "null email password",
			typ:      model.NotifyTypeEmail,
			previous: `{"smtp_host":"smtp.example.com","smtp_port":587,"username":"user","password":"old-pass","from":"a@example.com","to":["b@example.com"],"use_tls":true}`,
			next:     `{"smtp_host":"smtp.example.com","smtp_port":587,"username":"user","password":null,"from":"a@example.com","to":["c@example.com"],"use_tls":true}`,
			wantErr:  true,
		},
		{
			name:     "null webhook secret",
			typ:      model.NotifyTypeWebhook,
			previous: `{"url":"https://example.com/old","secret":"old-secret"}`,
			next:     `{"url":"https://example.com/new","secret":null}`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeConfigForUpdate(
				tt.typ,
				json.RawMessage(tt.next),
				tt.typ,
				json.RawMessage(tt.previous),
			)
			if tt.wantErr {
				if err == nil {
					t.Fatal("NormalizeConfigForUpdate() error = nil")
				}
				return
			}
			if err != nil || string(got) != tt.want {
				t.Fatalf("NormalizeConfigForUpdate() = %s, %v; want %s", got, err, tt.want)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	if _, err := DecodeConfig(model.NotifyTypeEmail, json.RawMessage(
		`{"smtp_host":"smtp.example.com","smtp_port":587,"username":"user","password":"pass","from":"a@example.com","to":["b@example.com"],"use_tls":null}`,
	)); err == nil {
		t.Fatal("DecodeConfig() accepted null email TLS")
	}
	if _, err := parseWebhookURL("http://:8080"); err == nil {
		t.Fatal("parseWebhookURL() accepted a missing hostname")
	}
}

func TestNotificationLanguageConfig(t *testing.T) {
	legacy := json.RawMessage(`{"url":"https://example.com/hook"}`)
	viewAny, err := SanitizeConfig(model.NotifyTypeWebhook, legacy)
	if err != nil {
		t.Fatalf("SanitizeConfig() error = %v", err)
	}
	view, ok := viewAny.(WebhookView)
	if !ok || view.Language != lang.System {
		t.Fatalf("SanitizeConfig() = %#v, want language %q", viewAny, lang.System)
	}
	if got := EffectiveLanguage(model.NotifyTypeWebhook, legacy, lang.English); got != lang.English {
		t.Fatalf("EffectiveLanguage(legacy) = %q, want %q", got, lang.English)
	}

	explicit := json.RawMessage(`{"language":"zh","url":"https://example.com/hook"}`)
	if got := EffectiveLanguage(model.NotifyTypeWebhook, explicit, lang.English); got != lang.Chinese {
		t.Fatalf("EffectiveLanguage(explicit) = %q, want %q", got, lang.Chinese)
	}
	if _, err := NormalizeConfig(model.NotifyTypeWebhook, json.RawMessage(
		`{"language":"fr","url":"https://example.com/hook"}`,
	)); err == nil {
		t.Fatal("NormalizeConfig() accepted unsupported language")
	}

	updated, err := NormalizeConfigForUpdate(
		model.NotifyTypeWebhook,
		json.RawMessage(`{"url":"https://example.com/new"}`),
		model.NotifyTypeWebhook,
		json.RawMessage(`{"language":"en","url":"https://example.com/old"}`),
	)
	if err != nil {
		t.Fatalf("NormalizeConfigForUpdate() error = %v", err)
	}
	if want := `{"language":"en","url":"https://example.com/new"}`; string(updated) != want {
		t.Fatalf("NormalizeConfigForUpdate() = %s, want %s", updated, want)
	}
}

func TestNormalizeConfigPreservesCredentialWhitespace(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "bot token",
			raw:  `{"mode":"bot","bot_token":" token ","chat_id":"-1001"}`,
			want: `{"mode":"bot","bot_token":" token ","chat_id":"-1001"}`,
		},
		{
			name: "MTProto credentials",
			raw:  `{"mode":"mtproto","api_id":123,"api_hash":" hash ","phone":"+10000000000","chat_id":"-1001","session":" session "}`,
			want: `{"mode":"mtproto","api_id":123,"api_hash":" hash ","phone":"+10000000000","chat_id":"-1001","session":" session "}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeConfig(model.NotifyTypeTelegram, json.RawMessage(tt.raw))
			if err != nil || string(got) != tt.want {
				t.Fatalf("NormalizeConfig() = %s, %v; want %s", got, err, tt.want)
			}
		})
	}
}
