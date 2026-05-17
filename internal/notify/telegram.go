package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	telegramAlertOpenIcon  = "❌"
	telegramAlertCloseIcon = "✅"
	telegramAlertTimeIcon  = "🕒"
)

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
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram bot status: %s", resp.Status)
	}

	var parsed struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return err
	}
	if !parsed.OK {
		if parsed.Description == "" {
			return fmt.Errorf("telegram bot send failed")
		}
		return fmt.Errorf("telegram bot send failed: %s", parsed.Description)
	}
	return nil
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
