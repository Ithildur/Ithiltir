package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"dash/internal/lang"
	"dash/internal/model"
)

type TelegramBotConfig struct {
	Language string `json:"language,omitempty"`
	Mode     string `json:"mode,omitempty"`
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type TelegramMTProtoConfig struct {
	Language string `json:"language,omitempty"`
	Mode     string `json:"mode"`
	APIID    int    `json:"api_id"`
	APIHash  string `json:"api_hash"`
	Phone    string `json:"phone"`
	ChatID   string `json:"chat_id"`
	Session  string `json:"session,omitempty"`
	Username string `json:"username,omitempty"`
}

type EmailConfig struct {
	Language string   `json:"language,omitempty"`
	SMTPHost string   `json:"smtp_host"`
	SMTPPort int      `json:"smtp_port"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
	UseTLS   bool     `json:"use_tls"`
}

type WebhookConfig struct {
	Language string `json:"language,omitempty"`
	URL      string `json:"url"`
	Secret   string `json:"secret,omitempty"`
}

type TelegramBotView struct {
	Language string `json:"language"`
	Mode     string `json:"mode,omitempty"`
	ChatID   string `json:"chat_id"`
}

type TelegramMTProtoView struct {
	Language string `json:"language"`
	Mode     string `json:"mode"`
	APIID    int    `json:"api_id"`
	Phone    string `json:"phone"`
	ChatID   string `json:"chat_id"`
	Username string `json:"username,omitempty"`
}

type EmailView struct {
	Language string   `json:"language"`
	SMTPHost string   `json:"smtp_host"`
	SMTPPort int      `json:"smtp_port"`
	Username string   `json:"username"`
	From     string   `json:"from"`
	To       []string `json:"to"`
	UseTLS   bool     `json:"use_tls"`
}

type WebhookView struct {
	Language string `json:"language"`
	URL      string `json:"url"`
}

type rawConfig map[string]json.RawMessage

const (
	maxNotifyConfigBytes  = 2 << 20
	maxBotTokenBytes      = 256
	maxChatIDBytes        = 256
	maxAPIHashBytes       = 256
	maxPhoneBytes         = 64
	maxSessionBytes       = 1 << 20
	maxUsernameBytes      = 1024
	maxPasswordBytes      = 8192
	maxMailAddressBytes   = 1024
	maxMailRecipients     = 100
	maxWebhookURLBytes    = 4096
	maxWebhookSecretBytes = 8192
)

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
			Language: viewLanguage(typed.Language),
			Mode:     typed.Mode,
			ChatID:   typed.ChatID,
		}, nil
	case TelegramMTProtoConfig:
		return TelegramMTProtoView{
			Language: viewLanguage(typed.Language),
			Mode:     typed.Mode,
			APIID:    typed.APIID,
			Phone:    typed.Phone,
			ChatID:   typed.ChatID,
			Username: typed.Username,
		}, nil
	case EmailConfig:
		return EmailView{
			Language: viewLanguage(typed.Language),
			SMTPHost: typed.SMTPHost,
			SMTPPort: typed.SMTPPort,
			Username: typed.Username,
			From:     typed.From,
			To:       append([]string(nil), typed.To...),
			UseTLS:   typed.UseTLS,
		}, nil
	case WebhookConfig:
		return WebhookView{
			Language: viewLanguage(typed.Language),
			URL:      typed.URL,
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
		return nil, fmt.Errorf("%w: decode previous config: %w", ErrStoredConfig, err)
	}

	fields, err := decodeObject(raw)
	if err != nil {
		return nil, err
	}
	inheritLanguageIfMissing(fields, previous)

	switch typ {
	case model.NotifyTypeTelegram:
		return normalizeTelegramForUpdate(fields, previous)
	case model.NotifyTypeEmail:
		if cfg, ok := previous.(EmailConfig); ok {
			inheritStringIfEmpty(fields, "password", cfg.Password)
		}
	case model.NotifyTypeWebhook:
		if cfg, ok := previous.(WebhookConfig); ok {
			inheritStringIfEmpty(fields, "secret", cfg.Secret)
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
			inheritStringIfEmpty(fields, "bot_token", cfg.BotToken)
		}
		return normalizeFields(model.NotifyTypeTelegram, fields)
	case TelegramModeMTProto:
		previousCfg, ok := previous.(TelegramMTProtoConfig)
		if !ok {
			return normalizeFields(model.NotifyTypeTelegram, fields)
		}

		inheritStringIfEmpty(fields, "api_hash", previousCfg.APIHash)
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

func inheritStringIfEmpty(fields rawConfig, key, value string) {
	if value == "" || !emptyStringField(fields, key) {
		return
	}
	raw, err := json.Marshal(value)
	if err == nil {
		fields[key] = raw
	}
}

func inheritLanguageIfMissing(fields rawConfig, previous any) {
	if _, ok := fields["language"]; ok {
		return
	}
	previousLanguage := typedLanguage(previous)
	if previousLanguage == "" {
		return
	}
	fields["language"] = json.RawMessage(strconv.Quote(previousLanguage))
}

func emptyStringField(fields rawConfig, key string) bool {
	raw, ok := fields[key]
	if !ok {
		return true
	}
	value, ok, err := readRawString(raw, key)
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
	language, err := readLanguage(fields)
	if err != nil {
		return nil, err
	}

	switch mode {
	case TelegramModeBot:
		if err := rejectUnknownFields(fields, "language", "mode", "bot_token", "chat_id"); err != nil {
			return nil, err
		}
		botToken, err := readRawStringNonEmpty(fields, "bot_token")
		if err != nil {
			return nil, err
		}
		if err := validateText("bot_token", botToken, maxBotTokenBytes); err != nil {
			return nil, err
		}
		chatID, err := readStringOrInt(fields, "chat_id")
		if err != nil {
			return nil, err
		}
		if err := validateText("chat_id", chatID, maxChatIDBytes); err != nil {
			return nil, err
		}
		return TelegramBotConfig{
			Language: language,
			Mode:     TelegramModeBot,
			BotToken: botToken,
			ChatID:   chatID,
		}, nil
	case TelegramModeMTProto:
		if err := rejectUnknownFields(fields, "language", "mode", "api_id", "api_hash", "phone", "chat_id", "session", "username"); err != nil {
			return nil, err
		}
		apiID, err := readPositiveInt(fields, "api_id")
		if err != nil {
			return nil, err
		}
		apiHash, err := readRawStringNonEmpty(fields, "api_hash")
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
		session, err := readOptionalRawString(fields, "session")
		if err != nil {
			return nil, err
		}
		username, err := readOptionalString(fields, "username")
		if err != nil {
			return nil, err
		}
		for _, field := range []struct {
			name  string
			value string
			limit int
		}{
			{"api_hash", apiHash, maxAPIHashBytes},
			{"phone", phone, maxPhoneBytes},
			{"chat_id", chatID, maxChatIDBytes},
			{"session", session, maxSessionBytes},
			{"username", username, maxUsernameBytes},
		} {
			if err := validateText(field.name, field.value, field.limit); err != nil {
				return nil, err
			}
		}
		return TelegramMTProtoConfig{
			Language: language,
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
	if err := rejectUnknownFields(fields, "language", "smtp_host", "smtp_port", "username", "password", "from", "to", "use_tls"); err != nil {
		return EmailConfig{}, err
	}
	language, err := readLanguage(fields)
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
	password, err := readRawStringRequired(fields, "password")
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
	for _, field := range []struct {
		name         string
		value        string
		limit        int
		allowControl bool
	}{
		{"smtp_host", smtpHost, 253, false},
		{"username", username, maxUsernameBytes, false},
		{"password", password, maxPasswordBytes, true},
		{"from", from, maxMailAddressBytes, false},
	} {
		if err := validateString(field.name, field.value, field.limit, field.allowControl); err != nil {
			return EmailConfig{}, err
		}
	}
	if len(to) > maxMailRecipients {
		return EmailConfig{}, fmt.Errorf("to must contain at most %d values", maxMailRecipients)
	}
	for _, address := range to {
		if err := validateText("to", address, maxMailAddressBytes); err != nil {
			return EmailConfig{}, err
		}
	}
	if _, _, err := parseMailList(to); err != nil {
		return EmailConfig{}, err
	}
	if _, err := mail.ParseAddress(from); err != nil {
		return EmailConfig{}, fmt.Errorf("from is invalid")
	}
	return EmailConfig{
		Language: language,
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
	if err := rejectUnknownFields(fields, "language", "url", "secret"); err != nil {
		return WebhookConfig{}, err
	}
	language, err := readLanguage(fields)
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
	secret, err := readOptionalRawString(fields, "secret")
	if err != nil {
		return WebhookConfig{}, err
	}
	if err := validateText("url", url, maxWebhookURLBytes); err != nil {
		return WebhookConfig{}, err
	}
	if err := validateString("secret", secret, maxWebhookSecretBytes, true); err != nil {
		return WebhookConfig{}, err
	}
	return WebhookConfig{
		Language: language,
		URL:      url,
		Secret:   secret,
	}, nil
}

func decodeObject(raw json.RawMessage) (rawConfig, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("config is required")
	}
	if len(raw) > maxNotifyConfigBytes {
		return nil, fmt.Errorf("config is too large")
	}
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("config must be valid UTF-8")
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

func readLanguage(fields rawConfig) (string, error) {
	raw, ok := fields["language"]
	if !ok {
		return "", nil
	}
	value, err := unmarshalString(raw, "language")
	if err != nil {
		return "", err
	}
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case lang.System, lang.Chinese, lang.English:
		return value, nil
	case "":
		return "", fmt.Errorf("language cannot be empty")
	default:
		return "", fmt.Errorf("language is not supported")
	}
}

func typedLanguage(cfg any) string {
	switch typed := cfg.(type) {
	case TelegramBotConfig:
		return typed.Language
	case TelegramMTProtoConfig:
		return typed.Language
	case EmailConfig:
		return typed.Language
	case WebhookConfig:
		return typed.Language
	default:
		return ""
	}
}

func viewLanguage(raw string) string {
	if raw == "" {
		return lang.System
	}
	return raw
}

func EffectiveLanguage(typ model.NotifyType, raw []byte, system string) string {
	cfg, err := DecodeConfig(typ, json.RawMessage(raw))
	if err != nil {
		return lang.Normalize(system)
	}
	return lang.Resolve(typedLanguage(cfg), system)
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

func readRawStringRequired(fields rawConfig, key string) (string, error) {
	raw, ok := fields[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	value, ok, err := readRawString(raw, key)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%s must be string", key)
	}
	return value, nil
}

func readRawStringNonEmpty(fields rawConfig, key string) (string, error) {
	value, err := readRawStringRequired(fields, key)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("%s cannot be empty", key)
	}
	return value, nil
}

func readOptionalString(fields rawConfig, key string) (string, error) {
	value, _, err := readStringField(fields, key)
	return value, err
}

func readOptionalRawString(fields rawConfig, key string) (string, error) {
	raw, ok := fields[key]
	if !ok {
		return "", nil
	}
	value, _, err := readRawString(raw, key)
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
	if value > int64(^uint(0)>>1) {
		return 0, fmt.Errorf("%s is out of range", key)
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
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false, fmt.Errorf("%s must be boolean", key)
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
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be string", key)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be string", key)
	}
	return value, nil
}

func readInteger(raw json.RawMessage, key string) (int64, bool, error) {
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err != nil {
		return 0, false, nil
	}
	n, err := number.Int64()
	if err != nil {
		return 0, true, fmt.Errorf("%s must be integer", key)
	}
	return n, true, nil
}

func readRawString(raw json.RawMessage, key string) (string, bool, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", false, fmt.Errorf("%s must be string", key)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false, fmt.Errorf("%s must be string", key)
	}
	return value, true, nil
}

func rejectUnknownFields(fields rawConfig, allowed ...string) error {
	known := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		known[key] = struct{}{}
	}
	for key := range fields {
		if _, ok := known[key]; !ok {
			return fmt.Errorf("config contains unknown field %q", key)
		}
	}
	return nil
}

func validateText(name, value string, limit int) error {
	return validateString(name, value, limit, false)
}

func validateString(name, value string, limit int, allowControl bool) error {
	if len(value) > limit {
		return fmt.Errorf("%s is too long", name)
	}
	if allowControl {
		return nil
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s contains control characters", name)
		}
	}
	return nil
}

func unsupportedTypeError(typ model.NotifyType) error {
	return fmt.Errorf("unsupported notify type: %s", typ)
}
