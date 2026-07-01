package events

import (
	"reflect"
	"testing"
	"time"

	alertstore "dash/internal/store/alert"
)

func TestSummaryViewsIncludesMetrics(t *testing.T) {
	at := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	got := summaryViews([]alertstore.OpenEventSummary{
		{
			ServerID:      7,
			OpenCount:     2,
			LastTriggerAt: at,
			Metric:        "node.offline",
			RuleName:      "offline",
			Metrics:       []string{"node.offline", "cpu.load1"},
		},
	})
	if len(got) != 1 {
		t.Fatalf("summaryViews len = %d, want 1", len(got))
	}
	wantMetrics := []string{"node.offline", "cpu.load1"}
	if !reflect.DeepEqual(got[0].Metrics, wantMetrics) {
		t.Fatalf("Metrics = %#v, want %#v", got[0].Metrics, wantMetrics)
	}
}

func TestSummaryViewsUsesEmptyMetrics(t *testing.T) {
	at := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	got := summaryViews([]alertstore.OpenEventSummary{
		{
			ServerID:      7,
			OpenCount:     1,
			LastTriggerAt: at,
			Metric:        "node.offline",
			RuleName:      "offline",
		},
	})
	if len(got) != 1 {
		t.Fatalf("summaryViews len = %d, want 1", len(got))
	}
	if got[0].Metrics == nil {
		t.Fatalf("Metrics = nil, want empty slice")
	}
	if len(got[0].Metrics) != 0 {
		t.Fatalf("Metrics len = %d, want 0", len(got[0].Metrics))
	}
}
