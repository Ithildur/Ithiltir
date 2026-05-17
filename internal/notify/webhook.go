package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func sendWebhook(ctx context.Context, cfg WebhookConfig, msg Message) error {
	parsedURL, err := parseWebhookURL(cfg.URL)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"title":   msg.Title,
		"message": msg.Body,
		"sent_at": time.Now().UTC().Format(time.RFC3339),
	}
	if len(msg.Metadata) > 0 {
		payload["meta"] = msg.Metadata
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, parsedURL.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if dedupeKey := msg.Metadata["dedupe_key"]; dedupeKey != "" {
		req.Header.Set("X-Alert-Dedupe-Key", dedupeKey)
	}
	if eventID := msg.Metadata["event_id"]; eventID != "" {
		req.Header.Set("X-Alert-Event-ID", eventID)
	}
	if transition := msg.Metadata["transition"]; transition != "" {
		req.Header.Set("X-Alert-Transition", transition)
	}
	if cfg.Secret != "" {
		req.Header.Set("X-Webhook-Signature", "sha256="+signWebhook(cfg.Secret, body))
	}

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status: %s", resp.Status)
	}
	return nil
}

func signWebhook(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
