package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

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
		err = sendSMTP(ctx, typed, messageWithChannelID(channel.ID, msg))
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

func messageWithChannelID(channelID int64, msg Message) Message {
	if channelID <= 0 {
		return msg
	}
	metadata := make(map[string]string, len(msg.Metadata)+1)
	for k, v := range msg.Metadata {
		metadata[k] = v
	}
	metadata["channel_id"] = strconv.FormatInt(channelID, 10)
	msg.Metadata = metadata
	return msg
}
