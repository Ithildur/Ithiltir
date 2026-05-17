package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"dash/internal/model"
)

func Send(ctx context.Context, channel *model.NotifyChannel, msg Message) error {
	if channel == nil {
		return errors.New("channel is nil")
	}

	cfg, err := DecodeConfig(channel.Type, json.RawMessage(channel.Config))
	if err != nil {
		return fmt.Errorf("notify send channel=%d type=%s action=decode_config: %w: %w", channel.ID, channel.Type, ErrInvalidConfig, err)
	}

	var action string
	switch typed := cfg.(type) {
	case TelegramBotConfig:
		action = "telegram_bot"
		err = sendTelegramBot(ctx, typed, msg)
	case TelegramMTProtoConfig:
		action = "telegram_mtproto"
		err = sendMTProto(ctx, typed, msg)
	case EmailConfig:
		action = "email"
		err = sendSMTP(ctx, typed, msg)
	case WebhookConfig:
		action = "webhook"
		err = sendWebhook(ctx, typed, msg)
	default:
		return fmt.Errorf("unsupported notify type: %s", channel.Type)
	}
	if err != nil {
		return fmt.Errorf("notify send channel=%d type=%s action=%s: %w", channel.ID, channel.Type, action, err)
	}
	return nil
}
