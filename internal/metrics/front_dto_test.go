package metrics

import (
	"testing"
	"time"

	"dash/internal/model"
)

func TestBuildNodeViewUsesReportedTimeAsObservedAt(t *testing.T) {
	receivedAt := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	reportedAt := receivedAt.Add(-5 * time.Second)

	view := BuildNodeView(model.Server{ID: 1}, NodeReport{
		Timestamp: receivedAt,
		SentAt:    reportedAt.Format(time.RFC3339),
	}, 14)
	if view.Observation.ReceivedAt != receivedAt.Format(time.RFC3339) {
		t.Fatalf("received_at = %q, want %q", view.Observation.ReceivedAt, receivedAt.Format(time.RFC3339))
	}
	if view.Observation.ObservedAt != reportedAt.Format(time.RFC3339) {
		t.Fatalf("observed_at = %q, want %q", view.Observation.ObservedAt, reportedAt.Format(time.RFC3339))
	}
}

func TestBuildNodeViewDropsInvalidStoredTags(t *testing.T) {
	view := BuildNodeView(model.Server{
		ID:   1,
		Tags: []byte(`["valid","bad\u0000tag"]`),
	}, NodeReport{}, 14)

	if len(view.Node.Tags) != 1 || view.Node.Tags[0] != "valid" {
		t.Fatalf("tags = %q, want valid stored tags preserved", view.Node.Tags)
	}
}
