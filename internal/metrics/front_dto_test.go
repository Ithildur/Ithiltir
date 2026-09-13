package metrics

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"dash/internal/model"
	"github.com/Ithildur/EiluneKit/http/response"
)

func TestNodeViewJSONPreservesOptionalFields(t *testing.T) {
	for _, tt := range []struct {
		name string
		view any
		want string
	}{
		{
			name: "omit unspecified order",
			view: NodeMeta{ID: "1", Title: "node"},
			want: `{"id":"1","title":"node"}`,
		},
		{
			name: "retain observed empty and zero SMART values",
			view: DiskSmartDevice{Health: new(""), ExitStatus: new(0), TempC: new(0.0)},
			want: `{"name":"","source":"","status":"","health":"","exit_status":0,"temp_c":0}`,
		},
		{
			name: "distinguish absent pressure from an empty resource",
			view: Pressure{CPU: &PressureResource{}},
			want: `{"cpu":{}}`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			response.WriteJSON(w, http.StatusOK, tt.view)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", w.Code, w.Body)
			}
			var want, got any
			if err := json.Unmarshal([]byte(tt.want), &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("metrics JSON = %s, want %s", w.Body, tt.want)
			}
		})
	}
}

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
