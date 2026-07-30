package notify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantClass DeliveryClass
		wantCode  string
		wantRetry time.Duration
	}{
		{
			name:      "invalid config blocks",
			err:       fmt.Errorf("decode: %w", ErrInvalidConfig),
			wantClass: DeliveryBlocked,
			wantCode:  "invalid_config",
		},
		{
			name:      "http rate limit retries",
			err:       deliveryStatusError("webhook_http", 429, 45*time.Second, "rate limited"),
			wantClass: DeliveryRetry,
			wantCode:  "webhook_http_429",
			wantRetry: 45 * time.Second,
		},
		{
			name:      "http authentication blocks",
			err:       deliveryStatusError("webhook_http", 401, 0, "unauthorized"),
			wantClass: DeliveryBlocked,
			wantCode:  "webhook_http_401",
		},
		{
			name:      "smtp temporary failure retries",
			err:       &textproto.Error{Code: 451, Msg: "try later"},
			wantClass: DeliveryRetry,
			wantCode:  "smtp_451",
		},
		{
			name:      "smtp permanent response blocks",
			err:       &textproto.Error{Code: 550, Msg: "mailbox unavailable"},
			wantClass: DeliveryBlocked,
			wantCode:  "smtp_550",
		},
		{
			name:      "timeout retries",
			err:       context.DeadlineExceeded,
			wantClass: DeliveryRetry,
			wantCode:  "timeout",
		},
		{
			name:      "unknown error retries",
			err:       errors.New("unknown failure"),
			wantClass: DeliveryRetry,
			wantCode:  "send_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.err)
			if got.Class != tt.wantClass || got.Code != tt.wantCode || got.RetryAfter != tt.wantRetry {
				t.Fatalf("Classify() = %+v, want class=%s code=%s retry_after=%s", got, tt.wantClass, tt.wantCode, tt.wantRetry)
			}
		})
	}
}

func TestParseRetryAfterCapsUntrustedValues(t *testing.T) {
	if got := parseRetryAfter("999999999999999", time.Now()); got != maxDeliveryRetryAfter {
		t.Fatalf("parseRetryAfter() = %s, want %s", got, maxDeliveryRetryAfter)
	}
}

func TestTelegramHTTPErrorUsesJSONRetryAfter(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": []string{"30"}},
	}
	var parsed telegramBotResponse
	parsed.Description = "retry later"
	parsed.Parameters.RetryAfter = 90

	got := Classify(telegramBotStatusError(resp, parsed))
	if got.Class != DeliveryRetry || got.Code != "telegram_http_429" || got.RetryAfter != 90*time.Second {
		t.Fatalf("telegramBotStatusError() = %+v", got)
	}
}

func TestHTTPStatusClassification(t *testing.T) {
	tests := []struct {
		status int
		class  DeliveryClass
	}{
		{status: 300, class: DeliveryBlocked},
		{status: 400, class: DeliveryBlocked},
		{status: 401, class: DeliveryBlocked},
		{status: 403, class: DeliveryBlocked},
		{status: 404, class: DeliveryBlocked},
		{status: 405, class: DeliveryBlocked},
		{status: 408, class: DeliveryRetry},
		{status: 409, class: DeliveryRetry},
		{status: 410, class: DeliveryBlocked},
		{status: 413, class: DeliveryBlocked},
		{status: 415, class: DeliveryBlocked},
		{status: 422, class: DeliveryBlocked},
		{status: 423, class: DeliveryRetry},
		{status: 425, class: DeliveryRetry},
		{status: 429, class: DeliveryRetry},
		{status: 500, class: DeliveryRetry},
		{status: 599, class: DeliveryRetry},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.status), func(t *testing.T) {
			got := Classify(deliveryStatusError("webhook_http", tt.status, 0, ""))
			if got.Class != tt.class {
				t.Fatalf("status %d class = %q, want %q", tt.status, got.Class, tt.class)
			}
		})
	}
}

func TestErrorSummaryBoundsPersistedText(t *testing.T) {
	raw := strings.Repeat("界", 600) + "\nsecret detail"
	got := ErrorSummary(errors.New(raw))
	if strings.ContainsAny(got, "\r\n\t") {
		t.Fatalf("ErrorSummary() retained control whitespace: %q", got)
	}
	if utf8.RuneCountInString(got) != 512 {
		t.Fatalf("ErrorSummary() rune count = %d, want 512", utf8.RuneCountInString(got))
	}
}
