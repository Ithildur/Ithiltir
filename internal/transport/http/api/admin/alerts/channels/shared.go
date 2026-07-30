package channels

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"dash/internal/model"
	alertstore "dash/internal/store/alert"
)

type channelView struct {
	ID                  int64                            `json:"id"`
	Name                string                           `json:"name"`
	Type                model.NotifyType                 `json:"type"`
	Config              any                              `json:"config"`
	Enabled             bool                             `json:"enabled"`
	DeliveryStatus      alertstore.ChannelDeliveryStatus `json:"delivery_status"`
	LastSuccessAt       *string                          `json:"last_success_at"`
	LastFailureAt       *string                          `json:"last_failure_at"`
	ConsecutiveFailures int32                            `json:"consecutive_failures"`
	LastErrorCode       *string                          `json:"last_error_code"`
	LastError           *string                          `json:"last_error"`
	NextRetryAt         *string                          `json:"next_retry_at"`
	NextProbeAt         *string                          `json:"next_probe_at"`
	PendingCount        int64                            `json:"pending_count"`
	BlockedCount        int64                            `json:"blocked_count"`
	CreatedAt           string                           `json:"created_at"`
	UpdatedAt           string                           `json:"updated_at"`
}

func viewFromDelivery(delivery alertstore.ChannelDelivery, config any) channelView {
	channel := delivery.Channel
	return channelView{
		ID:                  channel.ID,
		Name:                channel.Name,
		Type:                channel.Type,
		Config:              config,
		Enabled:             channel.Enabled,
		DeliveryStatus:      delivery.Status(),
		LastSuccessAt:       formatOptionalTime(channel.LastSuccessAt),
		LastFailureAt:       formatOptionalTime(channel.LastFailureAt),
		ConsecutiveFailures: channel.ConsecutiveFailures,
		LastErrorCode:       channel.LastErrorCode,
		LastError:           channel.LastError,
		NextRetryAt:         formatOptionalTime(delivery.NextRetryAt),
		NextProbeAt:         formatOptionalTime(delivery.NextProbeAt),
		PendingCount:        delivery.PendingCount,
		BlockedCount:        delivery.BlockedCount,
		CreatedAt:           channel.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           channel.ConfigUpdatedAt.Format(time.RFC3339),
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}

func normalizeChannelName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", errors.New("name is required")
	}
	if utf8.RuneCountInString(name) > 64 {
		return "", errors.New("name must contain at most 64 characters")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", errors.New("name must not contain control characters")
		}
	}
	return name, nil
}
