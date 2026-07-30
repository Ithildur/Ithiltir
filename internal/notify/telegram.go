package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	telegramAlertOpenIcon  = "❌"
	telegramAlertCloseIcon = "✅"
	telegramAlertTimeIcon  = "🕒"
)

type telegramBotResponse struct {
	OK          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

func sendTelegramBot(ctx context.Context, cfg TelegramBotConfig, msg Message) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	payload := struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}{
		ChatID: cfg.ChatID,
		Text:   telegramBotText(msg),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return requestError(err)
	}
	defer resp.Body.Close()

	var parsed telegramBotResponse
	decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&parsed)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return telegramBotStatusError(resp, parsed)
	}
	if decodeErr != nil {
		return fmt.Errorf("decode telegram bot response: %w", decodeErr)
	}
	if !parsed.OK {
		status := parsed.ErrorCode
		if status <= 0 {
			status = http.StatusBadRequest
		}
		return deliveryStatusError(
			"telegram_api",
			status,
			retryAfterSeconds(int64(parsed.Parameters.RetryAfter)),
			parsed.Description,
		)
	}
	return nil
}

func telegramBotStatusError(resp *http.Response, parsed telegramBotResponse) error {
	if resp == nil {
		return retryError("telegram_http_no_response", 0, errors.New("telegram endpoint returned no response"))
	}
	retryAfter := max(
		parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()),
		retryAfterSeconds(int64(parsed.Parameters.RetryAfter)),
	)
	return deliveryStatusError("telegram_http", resp.StatusCode, retryAfter, parsed.Description)
}

func sendMTProto(ctx context.Context, cfg TelegramMTProtoConfig, msg Message) error {
	return sendMTProtoMessage(ctx, cfg, nil, msg.Text())
}

func telegramBotText(msg Message) string {
	if msg.Metadata["transition"] == "" {
		return msg.Text()
	}

	icon := telegramAlertOpenIcon
	if msg.Metadata["transition"] == "closed" {
		icon = telegramAlertCloseIcon
	}
	title := strings.TrimSpace(msg.Title)
	body := telegramAlertTimeText(strings.TrimSpace(msg.Body))
	if title != "" && !strings.HasPrefix(title, icon+" ") {
		title = icon + " " + title
	}

	switch {
	case title == "":
		return body
	case body == "":
		return title
	default:
		return title + "\n" + body
	}
}

func telegramAlertTimeText(body string) string {
	if body == "" {
		return ""
	}

	lines := strings.Split(body, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if !telegramAlertTimeLine(trimmed) || strings.HasPrefix(trimmed, telegramAlertTimeIcon+" ") {
			continue
		}
		indent := line[:len(line)-len(trimmed)]
		lines[i] = indent + telegramAlertTimeIcon + " " + trimmed
	}
	return strings.Join(lines, "\n")
}

func telegramAlertTimeLine(line string) bool {
	return strings.HasPrefix(line, "触发时间:") ||
		strings.HasPrefix(line, "恢复时间:") ||
		strings.HasPrefix(line, "Triggered at:") ||
		strings.HasPrefix(line, "Recovered at:")
}
