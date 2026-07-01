package events

import (
	"net/url"
	"testing"
	"time"

	alertstore "dash/internal/store/alert"
)

func TestParseListQueryDefaults(t *testing.T) {
	got, err := parseListQuery(url.Values{})
	if err != nil {
		t.Fatalf("parseListQuery defaults: %v", err)
	}

	if got.Status != alertstore.EventStatusOpen {
		t.Fatalf("Status = %q, want open", got.Status)
	}
	if got.Limit != defaultEventLimit {
		t.Fatalf("Limit = %d, want %d", got.Limit, defaultEventLimit)
	}
	if got.From != nil {
		t.Fatalf("From = %v, want nil", got.From)
	}
	if got.To != nil {
		t.Fatalf("To = %v, want nil", got.To)
	}
}

func TestParseListQueryFilters(t *testing.T) {
	from := "2026-06-01T00:00:00Z"
	to := "2026-06-29T00:00:00Z"

	got, err := parseListQuery(url.Values{
		"server_id": {"42"},
		"status":    {"all"},
		"metric":    {"cpu.load1"},
		"from":      {from},
		"to":        {to},
		"limit":     {"800"},
	})
	if err != nil {
		t.Fatalf("parseListQuery filters: %v", err)
	}

	if got.ServerID != 42 {
		t.Fatalf("ServerID = %d, want 42", got.ServerID)
	}
	if got.Status != alertstore.EventStatusAll {
		t.Fatalf("Status = %q, want all", got.Status)
	}
	if got.Metric != "cpu.load1" {
		t.Fatalf("Metric = %q, want cpu.load1", got.Metric)
	}
	if got.Limit != maxEventLimit {
		t.Fatalf("Limit = %d, want capped %d", got.Limit, maxEventLimit)
	}
}

func TestParseListQueryRejectsInvalidStatus(t *testing.T) {
	_, err := parseListQuery(url.Values{"status": {"pending"}})
	if err == nil {
		t.Fatalf("parseListQuery accepted invalid status")
	}
}

func TestParseListQueryCursor(t *testing.T) {
	rawAt := "2026-06-29T00:00:00.123456789Z"
	got, err := parseListQuery(url.Values{"cursor": {rawAt + ",99"}})
	if err != nil {
		t.Fatalf("parseListQuery cursor: %v", err)
	}
	if got.Cursor == nil {
		t.Fatalf("Cursor = nil, want value")
	}
	wantAt, err := time.Parse(time.RFC3339Nano, rawAt)
	if err != nil {
		t.Fatalf("parse want cursor time: %v", err)
	}
	if !got.Cursor.LastTriggerAt.Equal(wantAt) {
		t.Fatalf("Cursor.LastTriggerAt = %v, want %v", got.Cursor.LastTriggerAt, wantAt)
	}
	if got.Cursor.ID != 99 {
		t.Fatalf("Cursor.ID = %d, want 99", got.Cursor.ID)
	}
}

func TestParseListQueryRejectsInvalidCursor(t *testing.T) {
	_, err := parseListQuery(url.Values{"cursor": {"bad"}})
	if err == nil {
		t.Fatalf("parseListQuery accepted invalid cursor")
	}
}
