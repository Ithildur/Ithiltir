package alert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"dash/internal/infra"
	"dash/internal/model"
	"dash/internal/notify"
	alertstore "dash/internal/store/alert"
	kitlog "github.com/Ithildur/EiluneKit/logging"

	"gorm.io/gorm"
)

const (
	alertNotifySendTimeout              = 15 * time.Second
	notificationStoreRetryMaxDelay      = 30 * time.Second
	notificationCompleteShutdownTimeout = 10 * time.Second
)

var errNotificationTargetsUnavailable = errors.New("notification targets are unavailable")

func (s *Service) openNotificationParams(ctx context.Context, transition OpenTransition) ([]alertstore.AlertNotificationParams, error) {
	return s.notificationParams(ctx, "opened", map[string]string{
		"rule_id":   fmt.Sprintf("%d", transition.Rule.RuleID),
		"server_id": fmt.Sprintf("%d", transition.ObjectID),
	}, func(cfg MessageConfig) alertMessage {
		return buildOpenMessage(transition, cfg)
	})
}

func (s *Service) closeNotificationParams(ctx context.Context, transition CloseTransition) ([]alertstore.AlertNotificationParams, error) {
	if transition.CloseReason != "condition_cleared" {
		return nil, nil
	}
	return s.notificationParams(ctx, "closed", map[string]string{
		"rule_id":      fmt.Sprintf("%d", transition.Rule.RuleID),
		"server_id":    fmt.Sprintf("%d", transition.ObjectID),
		"close_reason": transition.CloseReason,
	}, func(cfg MessageConfig) alertMessage {
		return buildCloseMessage(transition, cfg)
	})
}

func (s *Service) notificationParams(
	ctx context.Context,
	transition string,
	metadata map[string]string,
	messageFor func(MessageConfig) alertMessage,
) ([]alertstore.AlertNotificationParams, error) {
	targetCtx, cancel := context.WithTimeout(ctx, alertNotifySendTimeout)
	targets, err := s.notify.Targets(targetCtx)
	cancel()
	if err != nil && !targets.Ready {
		return nil, fmt.Errorf("%w: %w", errNotificationTargetsUnavailable, err)
	}
	if !targets.Enabled || len(targets.Channels) == 0 {
		return nil, err
	}

	out := make([]alertstore.AlertNotificationParams, 0, len(targets.Channels))
	for i := range targets.Channels {
		channel := targets.Channels[i]
		language := notify.EffectiveLanguage(channel.Type, json.RawMessage(channel.Config), s.message.Language)
		message := messageFor(MessageConfig{Language: language, Location: s.message.Location})
		payloadMetadata := metadata
		if channel.Type == model.NotifyTypeEmail {
			payloadMetadata = emailNotificationMetadata(metadata, language)
		}
		out = append(out, alertstore.AlertNotificationParams{
			Transition:  transition,
			ChannelID:   channel.ID,
			ChannelType: channel.Type,
			Payload: alertstore.NotificationPayload{
				Title:    message.Title,
				Body:     message.Body,
				Metadata: payloadMetadata,
			},
		})
	}
	return out, err
}

func emailNotificationMetadata(metadata map[string]string, language string) map[string]string {
	out := make(map[string]string, len(metadata)+1)
	for k, v := range metadata {
		out[k] = v
	}
	if language != "" {
		out["language"] = language
	}
	return out
}

func (s *Service) runNotificationLoop(ctx context.Context) error {
	ticker := time.NewTicker(notificationPollInterval)
	defer ticker.Stop()

	for {
		processed, err := s.processNotifications(ctx)
		if err != nil {
			s.logger.Warn("process alert notifications failed", err)
			ticker.Reset(notificationPollInterval)
		} else if processed {
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
		item, err := s.store.TakeNextNotification(ctx, now)
		if err != nil {
			return processed, err
		}
		if item == nil {
			return processed, nil
		}
		processed = true

		channelRevision, failure, err := s.sendNotification(ctx, item)
		if err != nil {
			nextAttemptAt := time.Now().UTC().Add(notificationPollInterval)
			requeueErr := s.store.RequeueNotification(ctx, item.ID, item.ChannelID, nextAttemptAt)
			if requeueErr != nil {
				err = errors.Join(err, fmt.Errorf("requeue alert notification %d: %w", item.ID, requeueErr))
			}
			return processed, err
		}
		if failure == nil {
			if err := s.completeNotification(ctx, item, channelRevision, time.Now().UTC()); err != nil {
				return processed, err
			}
			continue
		}
		if ctx.Err() != nil {
			return processed, ctx.Err()
		}

		failedAt := time.Now().UTC()
		stored := alertstore.NotificationFailure{
			ID:              item.ID,
			ChannelID:       item.ChannelID,
			ChannelRevision: failure.channelRevision,
			Code:            failure.code,
			LastError:       notify.ErrorSummary(failure.err),
			FailedAt:        failedAt,
		}
		failure.status = notificationFailureStatus(item.IsProbe, failure.status)
		fields := append(
			notificationLogFields(item),
			kitlog.String("failure_code", failure.code),
			kitlog.String("next_status", string(failure.status)),
		)

		switch failure.status {
		case model.OutboxStatusRetry:
			delay := max(notificationRetryDelay(item.AttemptCount+1), failure.retryAfter)
			stored.NextAttemptAt = failedAt.Add(delay)
			if err := s.store.RetryNotification(ctx, stored); err != nil {
				return processed, fmt.Errorf("schedule alert notification %d retry: %w", item.ID, err)
			}
			s.logger.Warn("alert notification retry scheduled", failure.err, fields...)
		case model.OutboxStatusBlocked:
			delay := max(notificationBlockedDelay(item.ProbeCount), failure.retryAfter)
			stored.NextAttemptAt = failedAt.Add(delay)
			if err := s.store.BlockNotification(ctx, stored); err != nil {
				return processed, fmt.Errorf("block alert notification %d: %w", item.ID, err)
			}
			s.logger.Warn("alert notification blocked", failure.err, fields...)
		case model.OutboxStatusPaused:
			if err := s.store.PauseNotification(ctx, stored); err != nil {
				return processed, fmt.Errorf("pause alert notification %d: %w", item.ID, err)
			}
			s.logger.Warn("alert notification paused", failure.err, fields...)
		case model.OutboxStatusDiscarded:
			if err := s.store.DiscardNotification(ctx, stored); err != nil {
				return processed, fmt.Errorf("discard alert notification %d: %w", item.ID, err)
			}
			s.logger.Warn("alert notification discarded", failure.err, fields...)
		default:
			return processed, fmt.Errorf("unsupported notification failure status %q", failure.status)
		}
	}
}

func (s *Service) completeNotification(
	ctx context.Context,
	item *model.AlertNotificationOutbox,
	channelRevision int64,
	sentAt time.Time,
) error {
	err := s.completeNotificationUntil(ctx, item, channelRevision, sentAt)
	if err == nil || ctx.Err() == nil {
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), notificationCompleteShutdownTimeout)
	defer cancel()
	shutdownErr := s.completeNotificationUntil(shutdownCtx, item, channelRevision, sentAt)
	if shutdownErr == nil {
		return nil
	}
	return errors.Join(err, shutdownErr)
}

func (s *Service) completeNotificationUntil(
	ctx context.Context,
	item *model.AlertNotificationOutbox,
	channelRevision int64,
	sentAt time.Time,
) error {
	delay := notificationPollInterval
	for {
		_, err := infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
			return struct{}{}, s.store.CompleteNotification(
				c,
				item.ID,
				item.ChannelID,
				channelRevision,
				sentAt,
			)
		})
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return fmt.Errorf("complete alert notification %d: %w", item.ID, errors.Join(err, ctx.Err()))
		}

		s.logger.Warn(
			"persist sent alert notification failed; retrying without resending",
			err,
			notificationLogFields(item)...,
		)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("complete alert notification %d: %w", item.ID, errors.Join(err, ctx.Err()))
		case <-timer.C:
		}
		delay = min(delay*2, notificationStoreRetryMaxDelay)
	}
}

func notificationFailureStatus(isProbe bool, status model.OutboxStatus) model.OutboxStatus {
	if isProbe && status == model.OutboxStatusRetry {
		return model.OutboxStatusBlocked
	}
	return status
}

type notificationFailure struct {
	status          model.OutboxStatus
	code            string
	retryAfter      time.Duration
	channelRevision int64
	err             error
}

func (s *Service) sendNotification(
	ctx context.Context,
	item *model.AlertNotificationOutbox,
) (int64, *notificationFailure, error) {
	if item == nil {
		return 0, discardNotification("outbox_item_missing", errors.New("notification outbox item is nil")), nil
	}
	var payload alertstore.NotificationPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return 0, discardNotification("payload_invalid", fmt.Errorf("decode notification payload: %w", err)), nil
	}
	channel, err := s.store.GetChannel(ctx, item.ChannelID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, discardNotification(
			"channel_deleted",
			fmt.Errorf("notification channel %d not found", item.ChannelID),
		), nil
	}
	if err != nil {
		return 0, nil, fmt.Errorf("load notification channel %d: %w", item.ChannelID, err)
	}
	if !channel.Enabled {
		failure := pauseNotification(
			"channel_disabled",
			fmt.Errorf("notification channel %d disabled", item.ChannelID),
		)
		failure.channelRevision = channel.Revision
		return 0, failure, nil
	}
	if channel.Type != item.ChannelType {
		failure := discardNotification(
			"channel_type_changed",
			fmt.Errorf(
				"notification channel %d type changed from %s to %s",
				item.ChannelID,
				item.ChannelType,
				channel.Type,
			),
		)
		failure.channelRevision = channel.Revision
		return 0, failure, nil
	}

	msg := notify.Message{
		Title:    payload.Title,
		Body:     payload.Body,
		Metadata: payload.Metadata,
	}
	sendCtx, cancel := context.WithTimeout(ctx, alertNotifySendTimeout)
	err = notify.Send(sendCtx, channel, msg)
	cancel()
	if err == nil {
		return channel.Revision, nil, nil
	}
	delivery := notify.Classify(err)
	if delivery.Class == notify.DeliveryBlocked || item.AttemptCount+1 > notificationRetryLimit {
		return 0, &notificationFailure{
			status:          model.OutboxStatusBlocked,
			code:            delivery.Code,
			retryAfter:      delivery.RetryAfter,
			channelRevision: channel.Revision,
			err:             err,
		}, nil
	}
	failure := retryNotification(delivery.Code, err, delivery.RetryAfter)
	failure.channelRevision = channel.Revision
	return 0, failure, nil
}

func retryNotification(code string, err error, retryAfter time.Duration) *notificationFailure {
	return &notificationFailure{
		status:     model.OutboxStatusRetry,
		code:       code,
		retryAfter: retryAfter,
		err:        err,
	}
}

func pauseNotification(code string, err error) *notificationFailure {
	return &notificationFailure{status: model.OutboxStatusPaused, code: code, err: err}
}

func discardNotification(code string, err error) *notificationFailure {
	return &notificationFailure{status: model.OutboxStatusDiscarded, code: code, err: err}
}

func notificationLogFields(item *model.AlertNotificationOutbox) []slog.Attr {
	if item == nil {
		return nil
	}
	fields := []slog.Attr{
		kitlog.Int64("notification_id", item.ID),
		kitlog.Int64("channel_id", item.ChannelID),
		kitlog.String("channel_type", string(item.ChannelType)),
	}
	if item.EventID != nil {
		fields = append(fields, kitlog.Int64("event_id", *item.EventID))
	}
	return fields
}
