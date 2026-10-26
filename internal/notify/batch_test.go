package notify

import (
	"testing"

	"dash/internal/model"
)

func TestJoinEmailAlerts(t *testing.T) {
	messages := []Message{
		{Title: "Offline: node-1", Body: "Triggered at: first", Metadata: map[string]string{
			"transition": "opened", "language": "en", "event_id": "1", "dedupe_key": "first",
		}},
		{Title: "Offline: node-2", Body: "Triggered at: second", Metadata: map[string]string{
			"transition": "opened", "language": "en", "event_id": "2", "dedupe_key": "second",
		}},
	}
	for i, body := range []string{
		"Offline: node-1\nTriggered at: first",
		"Offline: node-1\nTriggered at: first\n\n---\n\nOffline: node-2\nTriggered at: second",
	} {
		joined := JoinMessages(model.NotifyTypeEmail, messages[:i+1])
		if joined.Title != "Triggered" || joined.Body != body {
			t.Fatalf("%d-entry email digest = %+v", i+1, joined)
		}
	}
}
