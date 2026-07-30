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
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return nil, fmt.Errorf("url is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("url scheme must be http or https")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("url must not include user info")
	}
	if parsed.Fragment != "" {
		return nil, fmt.Errorf("url must not include a fragment")
	}
	return parsed, nil
}
