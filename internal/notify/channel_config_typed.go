package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"dash/internal/model"
)

type TelegramBotConfig struct {
	Mode     string `json:"mode,omitempty"`
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type TelegramMTProtoConfig struct {
	Mode     string `json:"mode"`
	APIID    int    `json:"api_id"`
	APIHash  string `json:"api_hash"`
	Phone    string `json:"phone"`
	ChatID   string `json:"chat_id"`
	Session  string `json:"session,omitempty"`
	Username string `json:"username,omitempty"`
}

type EmailConfig struct {
	SMTPHost string   `json:"smtp_host"`
	SMTPPort int      `json:"smtp_port"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
	UseTLS   bool     `json:"use_tls"`
}

type WebhookConfig struct {
	URL    string `json:"url"`
	Secret string `json:"secret,omitempty"`
}

type TelegramBotView struct {
	Mode   string `json:"mode,omitempty"`
	ChatID string `json:"chat_id"`
}

type TelegramMTProtoView struct {
	Mode     string `json:"mode"`
	APIID    int    `json:"api_id"`
	Phone    string `json:"phone"`
	ChatID   string `json:"chat_id"`
	Username string `json:"username,omitempty"`
}

type EmailView struct {
	SMTPHost string   `json:"smtp_host"`
	SMTPPort int      `json:"smtp_port"`
	Username string   `json:"username"`
	From     string   `json:"from"`
	To       []string `json:"to"`
	UseTLS   bool     `json:"use_tls"`
}

type WebhookView struct {
	URL string `json:"url"`
}

type rawConfig map[string]json.RawMessage

func DecodeConfig(typ model.NotifyType, raw json.RawMessage) (any, error) {
	switch typ {
	case model.NotifyTypeTelegram:
		return decodeTelegram(raw)
	case model.NotifyTypeEmail:
		return decodeEmail(raw)
	case model.NotifyTypeWebhook:
		return decodeWebhook(raw)
	default:
		return nil, unsupportedTypeError(typ)
	}
}

func SanitizeConfig(typ model.NotifyType, raw json.RawMessage) (any, error) {
	cfg, err := DecodeConfig(typ, raw)
	if err != nil {
		return nil, err
	}

	switch typed := cfg.(type) {
	case TelegramBotConfig:
		return TelegramBotView{
			Mode:   typed.Mode,
			ChatID: typed.ChatID,
		}, nil
	case TelegramMTProtoConfig:
		return TelegramMTProtoView{
			Mode:     typed.Mode,
			APIID:    typed.APIID,
			Phone:    typed.Phone,
			ChatID:   typed.ChatID,
			Username: typed.Username,
		}, nil
	case EmailConfig:
		return EmailView{
			SMTPHost: typed.SMTPHost,
			SMTPPort: typed.SMTPPort,
			Username: typed.Username,
			From:     typed.From,
			To:       append([]string(nil), typed.To...),
			UseTLS:   typed.UseTLS,
		}, nil
	case WebhookConfig:
		return WebhookView{
			URL: typed.URL,
		}, nil
	default:
		return nil, ErrInvalidConfig
	}
}

func NormalizeConfig(typ model.NotifyType, raw json.RawMessage) (json.RawMessage, error) {
	cfg, err := DecodeConfig(typ, raw)
	if err != nil {
		return nil, err
	}
	return marshalConfig(cfg)
}

func NormalizeConfigForUpdate(typ model.NotifyType, raw json.RawMessage, prevType model.NotifyType, prevRaw json.RawMessage) (json.RawMessage, error) {
	if typ != prevType {
		return NormalizeConfig(typ, raw)
	}

	previous, err := DecodeConfig(prevType, prevRaw)
	if err != nil {
		return NormalizeConfig(typ, raw)
	}

	fields, err := decodeObject(raw)
	if err != nil {
		return nil, err
	}

	switch typ {
	case model.NotifyTypeTelegram:
		return normalizeTelegramForUpdate(fields, previous)
	case model.NotifyTypeEmail:
		if cfg, ok := previous.(EmailConfig); ok {
			inheritStringIfBlank(fields, "password", cfg.Password)
		}
	case model.NotifyTypeWebhook:
		if cfg, ok := previous.(WebhookConfig); ok {
			inheritStringIfBlank(fields, "secret", cfg.Secret)
		}
	default:
		return NormalizeConfig(typ, raw)
	}

	return normalizeFields(typ, fields)
}

func marshalConfig(cfg any) (json.RawMessage, error) {
	payload, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func normalizeTelegramForUpdate(fields rawConfig, previous any) (json.RawMessage, error) {
	mode, err := readTelegramMode(fields)
	if err != nil {
		return nil, err
	}

	switch mode {
	case TelegramModeBot:
		if cfg, ok := previous.(TelegramBotConfig); ok {
			inheritStringIfBlank(fields, "bot_token", cfg.BotToken)
		}
		return normalizeFields(model.NotifyTypeTelegram, fields)
	case TelegramModeMTProto:
		previousCfg, ok := previous.(TelegramMTProtoConfig)
		if !ok {
			return normalizeFields(model.NotifyTypeTelegram, fields)
		}

		inheritStringIfBlank(fields, "api_hash", previousCfg.APIHash)
		nextRaw, err := marshalRawConfig(fields)
		if err != nil {
			return nil, err
		}
		nextAny, err := DecodeConfig(model.NotifyTypeTelegram, nextRaw)
		if err != nil {
			return nil, err
		}
		nextCfg, ok := nextAny.(TelegramMTProtoConfig)
		if !ok {
			return marshalConfig(nextAny)
		}
		if sameMTProtoLogin(previousCfg, nextCfg) {
			if nextCfg.Session == "" {
				nextCfg.Session = previousCfg.Session
			}
			if nextCfg.Username == "" {
				nextCfg.Username = previousCfg.Username
			}
		}
		return marshalConfig(nextCfg)
	default:
		return nil, ErrInvalidConfig
	}
}

func normalizeFields(typ model.NotifyType, fields rawConfig) (json.RawMessage, error) {
	raw, err := marshalRawConfig(fields)
	if err != nil {
		return nil, err
	}
	return NormalizeConfig(typ, raw)
}

func marshalRawConfig(fields rawConfig) (json.RawMessage, error) {
	payload, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func sameMTProtoLogin(a, b TelegramMTProtoConfig) bool {
	return a.APIID == b.APIID && a.APIHash == b.APIHash && a.Phone == b.Phone
}

func inheritStringIfBlank(fields rawConfig, key, value string) {
	if value == "" || !blankStringField(fields, key) {
		return
	}
	raw, err := json.Marshal(value)
	if err == nil {
		fields[key] = raw
	}
}

func blankStringField(fields rawConfig, key string) bool {
	raw, ok := fields[key]
	if !ok {
		return true
	}
	value, ok, err := readTrimmedString(raw, key)
	return err == nil && ok && value == ""
}

func decodeTelegram(raw json.RawMessage) (any, error) {
	fields, err := decodeObject(raw)
	if err != nil {
		return nil, err
	}
	mode, err := readTelegramMode(fields)
	if err != nil {
		return nil, err
	}

	switch mode {
	case TelegramModeBot:
		botToken, err := readString(fields, "bot_token")
		if err != nil {
			return nil, err
		}
		chatID, err := readStringOrInt(fields, "chat_id")
		if err != nil {
			return nil, err
		}
		return TelegramBotConfig{
			Mode:     TelegramModeBot,
			BotToken: botToken,
			ChatID:   chatID,
		}, nil
	case TelegramModeMTProto:
		apiID, err := readPositiveInt(fields, "api_id")
		if err != nil {
			return nil, err
		}
		apiHash, err := readString(fields, "api_hash")
		if err != nil {
			return nil, err
		}
		phone, err := readString(fields, "phone")
		if err != nil {
			return nil, err
		}
		chatID, err := readStringOrInt(fields, "chat_id")
		if err != nil {
			return nil, err
		}
		session, err := readOptionalString(fields, "session")
		if err != nil {
			return nil, err
		}
		username, err := readOptionalString(fields, "username")
		if err != nil {
			return nil, err
		}
		return TelegramMTProtoConfig{
			Mode:     TelegramModeMTProto,
			APIID:    apiID,
			APIHash:  apiHash,
			Phone:    phone,
			ChatID:   chatID,
			Session:  session,
			Username: username,
		}, nil
	default:
		return nil, ErrInvalidConfig
	}
}

func decodeEmail(raw json.RawMessage) (EmailConfig, error) {
	fields, err := decodeObject(raw)
	if err != nil {
		return EmailConfig{}, err
	}
	smtpHost, err := readString(fields, "smtp_host")
	if err != nil {
		return EmailConfig{}, err
	}
	smtpPort, err := readPort(fields, "smtp_port")
	if err != nil {
		return EmailConfig{}, err
	}
	username, err := readStringAllowEmpty(fields, "username")
	if err != nil {
		return EmailConfig{}, err
	}
	password, err := readStringAllowEmpty(fields, "password")
	if err != nil {
		return EmailConfig{}, err
	}
	from, err := readString(fields, "from")
	if err != nil {
		return EmailConfig{}, err
	}
	to, err := readStringList(fields, "to")
	if err != nil {
		return EmailConfig{}, err
	}
	useTLS, err := readBool(fields, "use_tls")
	if err != nil {
		return EmailConfig{}, err
	}
	return EmailConfig{
		SMTPHost: smtpHost,
		SMTPPort: smtpPort,
		Username: username,
		Password: password,
		From:     from,
		To:       to,
		UseTLS:   useTLS,
	}, nil
}

func decodeWebhook(raw json.RawMessage) (WebhookConfig, error) {
	fields, err := decodeObject(raw)
	if err != nil {
		return WebhookConfig{}, err
	}
	url, err := readString(fields, "url")
	if err != nil {
		return WebhookConfig{}, err
	}
	if _, err := parseWebhookURL(url); err != nil {
		return WebhookConfig{}, err
	}
	secret, err := readOptionalString(fields, "secret")
	if err != nil {
		return WebhookConfig{}, err
	}
	return WebhookConfig{
		URL:    url,
		Secret: secret,
	}, nil
}

func decodeObject(raw json.RawMessage) (rawConfig, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("config is required")
	}
	var fields rawConfig
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("config must be object")
	}
	return fields, nil
}

func readTelegramMode(fields rawConfig) (string, error) {
	if raw, ok := fields["mode"]; ok {
		mode, err := unmarshalString(raw, "mode")
		if err != nil {
			return "", err
		}
		mode = strings.ToLower(strings.TrimSpace(mode))
		if mode == "" {
			return "", fmt.Errorf("mode cannot be empty")
		}
		if mode != TelegramModeBot && mode != TelegramModeMTProto {
			return "", fmt.Errorf("mode is not supported")
		}
		return mode, nil
	}
	return TelegramModeBot, nil
}

func readString(fields rawConfig, key string) (string, error) {
	value, present, err := readStringField(fields, key)
	if err != nil {
		return "", err
	}
	if !present {
		return "", fmt.Errorf("%s is required", key)
	}
	if value == "" {
		return "", fmt.Errorf("%s cannot be empty", key)
	}
	return value, nil
}

func readStringAllowEmpty(fields rawConfig, key string) (string, error) {
	value, present, err := readStringField(fields, key)
	if err != nil {
		return "", err
	}
	if !present {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func readOptionalString(fields rawConfig, key string) (string, error) {
	value, _, err := readStringField(fields, key)
	return value, err
}

func readStringField(fields rawConfig, key string) (string, bool, error) {
	raw, ok := fields[key]
	if !ok {
		return "", false, nil
	}
	value, err := unmarshalString(raw, key)
	if err != nil {
		return "", false, err
	}
	return strings.TrimSpace(value), true, nil
}

func readStringOrInt(fields rawConfig, key string) (string, error) {
	raw, ok := fields[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	if value, ok, err := readTrimmedString(raw, key); err != nil {
		return "", err
	} else if ok {
		if value == "" {
			return "", fmt.Errorf("%s cannot be empty", key)
		}
		return value, nil
	}

	value, ok, err := readInteger(raw, key)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%s must be string or integer", key)
	}
	return strconv.FormatInt(value, 10), nil
}

func readPositiveInt(fields rawConfig, key string) (int, error) {
	raw, ok := fields[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}
	value, ok, err := readInteger(raw, key)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("%s must be integer", key)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return int(value), nil
}

func readPort(fields rawConfig, key string) (int, error) {
	port, err := readPositiveInt(fields, key)
	if err != nil {
		return 0, err
	}
	if port > 65535 {
		return 0, fmt.Errorf("%s is out of range", key)
	}
	return port, nil
}

func readBool(fields rawConfig, key string) (bool, error) {
	raw, ok := fields[key]
	if !ok {
		return false, fmt.Errorf("%s is required", key)
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, fmt.Errorf("%s must be boolean", key)
	}
	return value, nil
}

func readStringList(fields rawConfig, key string) ([]string, error) {
	raw, ok := fields[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	if value, ok, err := readTrimmedString(raw, key); err != nil {
		return nil, err
	} else if ok {
		if value == "" {
			return nil, fmt.Errorf("%s cannot be empty", key)
		}
		return []string{value}, nil
	}

	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("%s must be string list", key)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("%s cannot be empty", key)
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, fmt.Errorf("%s cannot contain empty values", key)
		}
		out = append(out, item)
	}
	return out, nil
}

func readTrimmedString(raw json.RawMessage, _ string) (string, bool, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false, nil
	}
	return strings.TrimSpace(value), true, nil
}

func unmarshalString(raw json.RawMessage, key string) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be string", key)
	}
	return value, nil
}

func readInteger(raw json.RawMessage, key string) (int64, bool, error) {
	if value, ok, err := readTrimmedString(raw, key); err != nil {
		return 0, false, err
	} else if ok {
		if value == "" {
			return 0, true, fmt.Errorf("%s is required", key)
		}
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, true, fmt.Errorf("%s must be integer", key)
		}
		return n, true, nil
	}

	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err != nil {
		return 0, false, nil
	}
	if n, err := number.Int64(); err == nil {
		return n, true, nil
	}
	floatValue, err := number.Float64()
	if err != nil || math.Trunc(floatValue) != floatValue {
		return 0, true, fmt.Errorf("%s must be integer", key)
	}
	return int64(floatValue), true, nil
}

func unsupportedTypeError(typ model.NotifyType) error {
	return fmt.Errorf("unsupported notify type: %s", typ)
}
