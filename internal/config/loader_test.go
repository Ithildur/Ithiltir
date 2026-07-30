package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadConfigFileRejectsUnknownField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("app:\n  listen: \":8080\"\n  unknown_field: true\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var cfg Config
	err := readConfigFile(path, &cfg)
	if err == nil || !strings.Contains(err.Error(), "field unknown_field not found") {
		t.Fatalf("readConfigFile() error = %v, want unknown field", err)
	}
}

func TestReadConfigFileRejectsMultipleDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "app:\n  listen: \":8080\"\n---\napp:\n  public_url: https://example.com\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var cfg Config
	err := readConfigFile(path, &cfg)
	if err == nil || !strings.Contains(err.Error(), "multiple YAML documents") {
		t.Fatalf("readConfigFile() error = %v, want multiple document error", err)
	}
}

func TestCompileDurationsRejectsExplicitNonPositiveValues(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		key  string
	}{
		{
			name: "node offline threshold",
			cfg:  Config{App: AppConfig{NodeOfflineThreshold: "0s"}},
			key:  "app.node_offline_threshold",
		},
		{
			name: "redis timeout",
			cfg: Config{Redis: RedisConfig{
				Addr:        "127.0.0.1:6379",
				DialTimeout: "0s",
			}},
			key: "redis.dial_timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := compileDurations(&tt.cfg, true)
			if err == nil || !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("compileDurations() error = %v, want %s", err, tt.key)
			}
		})
	}
}

func TestRuntimeConfigRequiresRedisAddrOnlyWhenEnabled(t *testing.T) {
	cfg := &Config{
		App: AppConfig{
			Listen:    ":8080",
			PublicURL: "http://127.0.0.1:8080",
		},
		Auth: AuthConfig{
			JWTSigningKey: "12345678901234567890123456789012",
		},
		Database: DatabaseConfig{
			Driver: "postgres",
			Host:   "127.0.0.1",
			Port:   5432,
			User:   "dash",
			Name:   "dash",
		},
	}

	if err := validateRuntime(cfg, true); err == nil || !strings.Contains(err.Error(), "redis.addr") {
		t.Fatalf("validateRuntime(redis enabled) error = %v, want missing redis.addr", err)
	}
	if err := validateRuntime(cfg, false); err != nil {
		t.Fatalf("validateRuntime(redis disabled) error = %v", err)
	}
}

func TestCompilePublicURL(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		want     string
		wantHost string
	}{
		{name: "http", raw: "http://127.0.0.1:8080/", want: "http"},
		{name: "ipv6", raw: "http://[::1]:8080/", want: "http"},
		{name: "bare ipv6", raw: "::1", want: "http", wantHost: "[::1]"},
		{name: "https", raw: "https://dash.example.com", want: "https"},
		{name: "uppercase scheme", raw: "HTTPS://dash.example.com", want: "https"},
		{name: "bare domain", raw: "dash.example.com", want: "https"},
		{name: "unsupported scheme", raw: "ftp://dash.example.com"},
		{name: "missing hostname", raw: "http://:8080/"},
		{name: "user info", raw: "https://user@dash.example.com"},
		{name: "path prefix", raw: "https://dash.example.com/app"},
		{name: "double slash path", raw: "https://dash.example.com//"},
		{name: "query", raw: "https://dash.example.com/?source=test"},
		{name: "empty query", raw: "https://dash.example.com/?"},
		{name: "fragment", raw: "https://dash.example.com/#top"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{App: AppConfig{PublicURL: tt.raw}}
			err := compilePublicURL(cfg)
			if tt.want == "" {
				if err == nil {
					t.Fatalf("compilePublicURL(%q) succeeded, want error", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("compilePublicURL(%q) error = %v", tt.raw, err)
			}
			if cfg.App.PublicURLScheme != tt.want {
				t.Fatalf("compilePublicURL(%q) scheme = %q, want %q", tt.raw, cfg.App.PublicURLScheme, tt.want)
			}
			if tt.wantHost != "" && cfg.App.PublicURLHost != tt.wantHost {
				t.Fatalf("compilePublicURL(%q) host = %q, want %q", tt.raw, cfg.App.PublicURLHost, tt.wantHost)
			}
		})
	}
}
