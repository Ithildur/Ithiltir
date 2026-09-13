package events

import (
	"net/url"
	"testing"
	"time"

	alertstore "dash/internal/store/alert"
)

func TestParseListQuery(t *testing.T) {
	cursorAt := "2026-06-29T00:00:00.123456789Z"
	tests := []struct {
		name    string
		values  url.Values
		wantErr bool
		check   func(*testing.T, alertstore.AlertEventQuery)
	}{
		{
			name: "defaults",
			check: func(t *testing.T, got alertstore.AlertEventQuery) {
				if got.Status != alertstore.EventStatusOpen || got.Limit != defaultEventLimit {
					t.Fatalf("defaults = status %q limit %d", got.Status, got.Limit)
				}
				if got.From != nil || got.To != nil {
					t.Fatalf("default range = %v..%v, want nil", got.From, got.To)
				}
			},
		},
		{
			name: "filters",
			values: url.Values{
				"server_id": {"42"},
				"status":    {"all"},
				"metric":    {"cpu.load1"},
				"from":      {"2026-06-01T00:00:00Z"},
				"to":        {"2026-06-29T00:00:00Z"},
				"limit":     {"800"},
			},
			check: func(t *testing.T, got alertstore.AlertEventQuery) {
				if got.ServerID != 42 || got.Status != alertstore.EventStatusAll ||
					got.Metric != "cpu.load1" || got.Limit != maxEventLimit {
					t.Fatalf("filters = %#v", got)
				}
			},
		},
		{name: "invalid status", values: url.Values{"status": {"pending"}}, wantErr: true},
		{
			name:   "cursor",
			values: url.Values{"cursor": {cursorAt + ",99"}},
			check: func(t *testing.T, got alertstore.AlertEventQuery) {
				wantAt, err := time.Parse(time.RFC3339Nano, cursorAt)
				if err != nil {
					t.Fatalf("parse expected cursor: %v", err)
				}
				if got.Cursor == nil || !got.Cursor.LastTriggerAt.Equal(wantAt) || got.Cursor.ID != 99 {
					t.Fatalf("cursor = %#v, want %s,99", got.Cursor, wantAt)
				}
			},
		},
		{name: "invalid cursor", values: url.Values{"cursor": {"bad"}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseListQuery(tt.values)
			if tt.wantErr {
				if err == nil {
					t.Fatal("parseListQuery() error = nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseListQuery() error = %v", err)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}
