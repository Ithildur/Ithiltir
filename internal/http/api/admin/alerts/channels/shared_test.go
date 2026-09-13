package channels

import (
	"encoding/json"
	"strings"
	"testing"

	"dash/internal/model"
	alertstore "dash/internal/store/alert"

	"gorm.io/datatypes"
)

func TestViewFromDeliveryKeepsInvalidStoredConfigReadable(t *testing.T) {
	view := viewFromDelivery(alertstore.ChannelDelivery{
		Channel: model.NotifyChannel{
			ID:     42,
			Name:   "legacy webhook",
			Type:   model.NotifyTypeWebhook,
			Config: datatypes.JSON(`{"url":"ftp://example.com/hook"}`),
		},
	})
	if view.Config != nil {
		t.Fatalf("view config = %#v, want nil for invalid stored config", view.Config)
	}

	payload, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal channel view: %v", err)
	}
	if !strings.Contains(string(payload), `"config":null`) {
		t.Fatalf("channel view = %s, want config=null", payload)
	}
}
