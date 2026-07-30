package alert

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dash/internal/infra"
	"dash/internal/notify"
	alertstore "dash/internal/store/alert"
)

// EnqueueDefault resolves the current default targets and hands delivery to the
// durable notification outbox.
func (s *Service) EnqueueDefault(
	ctx context.Context,
	key string,
	message notify.Message,
) (notify.EnqueueStatus, error) {
	if s == nil || s.store == nil || s.notify == nil {
		return "", fmt.Errorf("notification service is not initialized")
	}
	if ctx == nil {
		return "", fmt.Errorf("notification context is nil")
	}
	key = strings.TrimSpace(key)
	event := strings.TrimSpace(message.Metadata["event"])
	if key == "" || event == "" {
		return "", fmt.Errorf("notification key and event metadata are required")
	}

	targetCtx, cancel := context.WithTimeout(ctx, alertNotifySendTimeout)
	targets, targetErr := s.notify.Targets(targetCtx)
	cancel()
	if targetErr != nil && !targets.Ready {
		return "", fmt.Errorf("load default notification targets: %w", targetErr)
	}
	if !targets.Enabled || len(targets.Channels) == 0 {
		if targetErr != nil {
			return "", fmt.Errorf("refresh default notification targets: %w", targetErr)
		}
		return notify.EnqueueSkippedNoTargets, nil
	}

	payload := alertstore.NotificationPayload{
		Title:    message.Title,
		Body:     message.Body,
		Metadata: message.Metadata,
	}
	_, err := infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
		return struct{}{}, s.store.EnqueueNotifications(c, key, event, targets.Channels, payload, time.Now().UTC())
	})
	if err != nil {
		return "", fmt.Errorf("enqueue default notification: %w", err)
	}
	if targetErr != nil {
		return notify.EnqueueQueued, fmt.Errorf("refresh default notification targets: %w", targetErr)
	}
	return notify.EnqueueQueued, nil
}
