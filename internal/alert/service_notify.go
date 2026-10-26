package alert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"dash/internal/config"
	"dash/internal/model"
	"dash/internal/notify"
	alertstore "dash/internal/store/alert"
	kitlog "github.com/Ithildur/EiluneKit/logging"

	"gorm.io/gorm"
)

const (
	notificationBatchInterval           = 3 * time.Second
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
	ticker := time.NewTicker(notificationBatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.processNotifications(ctx); err != nil {
				s.logger.Warn(ctx, "process alert notifications failed", err)
			}
		}
	}
}

func (s *Service) processNotifications(ctx context.Context) error {
	now := time.Now().UTC()
	for {
		items, err := s.store.TakeNotificationBatch(ctx, now)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}

		channelRevision, failure, err := s.sendNotification(ctx, items)
		if err != nil {
			nextAttemptAt := time.Now().UTC().Add(notificationPollInterval)
			requeueErr := s.store.WithTx(ctx, func(tx *alertstore.Store) error {
				for _, item := range items {
					if err := tx.RequeueNotification(ctx, item.ID, item.ChannelID, nextAttemptAt); err != nil {
						return err
					}
				}
				return nil
			})
			if requeueErr != nil {
				err = errors.Join(err, fmt.Errorf("requeue alert notification batch %d: %w", items[0].ID, requeueErr))
			}
			return err
		}
		if failure == nil {
			if err := s.completeNotification(ctx, items, channelRevision, time.Now().UTC()); err != nil {
				return err
			}
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		failedAt := time.Now().UTC()
		if err := s.store.WithTx(ctx, func(tx *alertstore.Store) error {
			for i := range items {
				if err := s.failNotification(ctx, tx, &items[i], *failure, failedAt); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
}

func (s *Service) failNotification(ctx context.Context, tx *alertstore.Store, item *model.AlertNotificationOutbox, failure notificationFailure, failedAt time.Time) error {
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
		if err := tx.RetryNotification(ctx, stored); err != nil {
			return fmt.Errorf("schedule alert notification %d retry: %w", item.ID, err)
		}
		s.logger.Warn(ctx, "alert notification retry scheduled", failure.err, fields...)
	case model.OutboxStatusBlocked:
		delay := max(notificationBlockedDelay(item.ProbeCount), failure.retryAfter)
		stored.NextAttemptAt = failedAt.Add(delay)
		if err := tx.BlockNotification(ctx, stored); err != nil {
			return fmt.Errorf("block alert notification %d: %w", item.ID, err)
		}
		s.logger.Warn(ctx, "alert notification blocked", failure.err, fields...)
	case model.OutboxStatusPaused:
		if err := tx.PauseNotification(ctx, stored); err != nil {
			return fmt.Errorf("pause alert notification %d: %w", item.ID, err)
		}
		s.logger.Warn(ctx, "alert notification paused", failure.err, fields...)
	case model.OutboxStatusDiscarded:
		if err := tx.DiscardNotification(ctx, stored); err != nil {
			return fmt.Errorf("discard alert notification %d: %w", item.ID, err)
		}
		s.logger.Warn(ctx, "alert notification discarded", failure.err, fields...)
	default:
		return fmt.Errorf("unsupported notification failure status %q", failure.status)
	}
	return nil
}

func (s *Service) completeNotification(
	ctx context.Context,
	items []model.AlertNotificationOutbox,
	channelRevision int64,
	sentAt time.Time,
) error {
	err := s.completeNotificationUntil(ctx, items, channelRevision, sentAt)
	if err == nil || ctx.Err() == nil {
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), notificationCompleteShutdownTimeout)
	defer cancel()
	shutdownErr := s.completeNotificationUntil(shutdownCtx, items, channelRevision, sentAt)
	if shutdownErr == nil {
		return nil
	}
	return errors.Join(err, shutdownErr)
}

func (s *Service) completeNotificationUntil(
	ctx context.Context,
	items []model.AlertNotificationOutbox,
	channelRevision int64,
	sentAt time.Time,
) error {
	delay := notificationPollInterval
	item := &items[0]
	for {
		err := s.completeNotificationOnce(ctx, items, channelRevision, sentAt)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return fmt.Errorf("complete alert notification %d: %w", item.ID, errors.Join(err, ctx.Err()))
		}

		s.logger.Warn(ctx,
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

func (s *Service) completeNotificationOnce(
	ctx context.Context,
	items []model.AlertNotificationOutbox,
	channelRevision int64,
	sentAt time.Time,
) error {
	dbCtx, cancel := context.WithTimeout(ctx, config.PGWriteTimeout)
	defer cancel()
	return s.store.WithTx(dbCtx, func(tx *alertstore.Store) error {
		for _, item := range items {
			if err := tx.CompleteNotification(dbCtx, item.ID, item.ChannelID, channelRevision, sentAt); err != nil {
				return err
			}
		}
		return nil
	})
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
	items []model.AlertNotificationOutbox,
) (int64, *notificationFailure, error) {
	item := &items[0]
	messages := make([]notify.Message, 0, len(items))
	for _, row := range items {
		var payload alertstore.NotificationPayload
		if err := json.Unmarshal(row.Payload, &payload); err != nil {
			return 0, discardNotification("payload_invalid", fmt.Errorf("decode notification payload: %w", err)), nil
		}
		messages = append(messages, notify.Message{Title: payload.Title, Body: payload.Body, Metadata: payload.Metadata})
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

	msg := messages[0]
	if item.EventID != nil && (item.ChannelType == model.NotifyTypeTelegram || item.ChannelType == model.NotifyTypeEmail) {
		msg = notify.JoinMessages(item.ChannelType, messages)
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
