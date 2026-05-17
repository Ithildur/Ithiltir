package notify

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"dash/internal/model"
)

const (
	TelegramModeBot     = "bot"
	TelegramModeMTProto = "mtproto"
)

func NormalizeType(v string) (model.NotifyType, error) {
	typed := model.NotifyType(strings.TrimSpace(v))
	switch typed {
	case model.NotifyTypeTelegram,
		model.NotifyTypeEmail,
		model.NotifyTypeWebhook:
		return typed, nil
	default:
		return "", errors.New("type is not supported")
	}
}

func IsAllowedType(v string) bool {
	_, err := NormalizeType(v)
	return err == nil
}

func SessionFromConfig(raw []byte) (string, bool, error) {
	cfg, err := DecodeConfig(model.NotifyTypeTelegram, raw)
	if err != nil {
		return "", false, ErrInvalidConfig
	}
	mtprotoCfg, ok := cfg.(TelegramMTProtoConfig)
	if !ok {
		return "", false, nil
	}
	return mtprotoCfg.Session, true, nil
}

func parseWebhookURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("url cannot be empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("url is invalid")
	}
	return parsed, nil
}
