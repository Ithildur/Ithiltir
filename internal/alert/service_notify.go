package alert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"dash/internal/model"
	"dash/internal/notify"
	alertstore "dash/internal/store/alert"
	kitlog "github.com/Ithildur/EiluneKit/logging"

	"gorm.io/gorm"
)

const alertNotifySendTimeout = 15 * time.Second

func (s *Service) openNotificationParams(ctx context.Context, transition OpenTransition, message alertMessage) ([]alertstore.AlertNotificationParams, error) {
	return s.notificationParams(ctx, "opened", message, map[string]string{
		"rule_id":   fmt.Sprintf("%d", transition.Rule.RuleID),
		"server_id": fmt.Sprintf("%d", transition.ObjectID),
	})
}

func (s *Service) closeNotificationParams(ctx context.Context, transition CloseTransition, message alertMessage) ([]alertstore.AlertNotificationParams, error) {
	if transition.CloseReason != "condition_cleared" {
		return nil, nil
	}
	return s.notificationParams(ctx, "closed", message, map[string]string{
		"rule_id":      fmt.Sprintf("%d", transition.Rule.RuleID),
		"server_id":    fmt.Sprintf("%d", transition.ObjectID),
		"close_reason": transition.CloseReason,
	})
}

func (s *Service) notificationParams(ctx context.Context, transition string, message alertMessage, metadata map[string]string) ([]alertstore.AlertNotificationParams, error) {
	targetCtx, cancel := context.WithTimeout(ctx, alertNotifySendTimeout)
	targets, err := s.notify.Targets(targetCtx)
	cancel()
	if !targets.Enabled || len(targets.Channels) == 0 {
		return nil, err
	}

	out := make([]alertstore.AlertNotificationParams, 0, len(targets.Channels))
	for i := range targets.Channels {
		channel := targets.Channels[i]
		out = append(out, alertstore.AlertNotificationParams{
			Transition:  transition,
			ChannelID:   channel.ID,
			ChannelType: channel.Type,
			Payload: alertstore.AlertNotificationPayload{
				Title:    message.Title,
				Body:     message.Body,
				Metadata: metadata,
			},
		})
	}
	return out, err
}

func (s *Service) runNotificationLoop(ctx context.Context) error {
	ticker := time.NewTicker(notificationPollInterval)
	defer ticker.Stop()

	for {
		processed, err := s.processNotifications(ctx)
		if err != nil {
			s.logger.Warn("process alert notifications failed", err)
		}
		if processed {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (s *Service) processNotifications(ctx context.Context) (bool, error) {
	processed := false
	for {
		now := time.Now().UTC()
		item, err := s.store.LeaseNextNotification(ctx, now, now.Add(notificationLeaseTTL))
		if err != nil {
			return processed, err
		}
		if item == nil {
			return processed, nil
		}
		processed = true

		permanent, err := s.sendNotification(ctx, item)
		if err == nil {
			if completeErr := s.store.CompleteNotification(ctx, item.ID, time.Now().UTC()); completeErr != nil {
				s.logger.Warn("complete alert notification failed", completeErr, kitlog.Int64("notification_id", item.ID))
			}
			continue
		}
		if permanent {
			if failErr := s.store.FailNotification(ctx, item.ID, err.Error()); failErr != nil {
				s.logger.Warn("fail alert notification failed", failErr, kitlog.Int64("notification_id", item.ID))
			}
			s.logger.Warn("alert notification permanently failed", err, notificationLogFields(item)...)
			continue
		}
		next := now.Add(controlTaskRetryDelay(item.AttemptCount))
		if retryErr := s.store.RetryNotification(ctx, item.ID, next, err.Error()); retryErr != nil {
			s.logger.Warn("retry alert notification failed", retryErr, kitlog.Int64("notification_id", item.ID))
		}
		s.logger.Warn("alert notification failed", err, notificationLogFields(item)...)
	}
}

func (s *Service) sendNotification(ctx context.Context, item *model.AlertNotificationOutbox) (bool, error) {
	if item == nil {
		return true, nil
	}
	var payload alertstore.AlertNotificationPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return true, fmt.Errorf("decode notification payload: %w", err)
	}
	channel, err := s.store.GetChannel(ctx, item.ChannelID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, fmt.Errorf("notification channel %d not found", item.ChannelID)
	}
	if err != nil {
		return false, err
	}
	if !channel.Enabled {
		return true, fmt.Errorf("notification channel %d disabled", item.ChannelID)
	}
	if channel.Type != item.ChannelType {
		return true, fmt.Errorf("notification channel %d type changed from %s to %s", item.ChannelID, item.ChannelType, channel.Type)
	}

	msg := notify.Message{
		Title:    payload.Title,
		Body:     payload.Body,
		Metadata: payload.Metadata,
	}
	sendCtx, cancel := context.WithTimeout(ctx, alertNotifySendTimeout)
	err = notify.Send(sendCtx, channel, msg)
	cancel()
	if errors.Is(err, notify.ErrInvalidConfig) {
		return true, err
	}
	return false, err
}

func notificationLogFields(item *model.AlertNotificationOutbox) []slog.Attr {
	if item == nil {
		return nil
	}
	return []slog.Attr{
		kitlog.Int64("notification_id", item.ID),
		kitlog.Int64("event_id", item.EventID),
		kitlog.Int64("channel_id", item.ChannelID),
		kitlog.String("channel_type", string(item.ChannelType)),
	}
}
