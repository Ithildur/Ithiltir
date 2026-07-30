package notify

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultHTTPTimeout = 10 * time.Second
const maxDeliveryRetryAfter = 24 * time.Hour
const maxHTTPRedirects = 5

var defaultHTTPClient = &http.Client{
	Timeout:       defaultHTTPTimeout,
	CheckRedirect: controlledRedirect,
}

func controlledRedirect(req *http.Request, via []*http.Request) error {
	if req == nil || req.URL == nil || len(via) == 0 || via[0] == nil || via[0].URL == nil {
		return blockedError("redirect_invalid", errors.New("invalid notification redirect"))
	}
	if len(via) > maxHTTPRedirects {
		return blockedError(
			"redirect_limit",
			fmt.Errorf("notification redirect limit exceeded: %d", maxHTTPRedirects),
		)
	}

	original := via[0].URL
	previousRequest := via[len(via)-1]
	if previousRequest == nil || previousRequest.URL == nil {
		return blockedError("redirect_invalid", errors.New("invalid notification redirect source"))
	}
	if previousRequest.Method != req.Method {
		return blockedError(
			"redirect_method_changed",
			fmt.Errorf("notification redirect changed method from %s to %s", previousRequest.Method, req.Method),
		)
	}
	previous := previousRequest.URL
	target := req.URL
	if target.User != nil {
		return blockedError(
			"redirect_userinfo",
			errors.New("notification redirect must not contain user information"),
		)
	}
	if !strings.EqualFold(original.Hostname(), target.Hostname()) || target.Hostname() == "" {
		return blockedError(
			"redirect_host_changed",
			errors.New("notification redirect must keep the original host"),
		)
	}

	previousScheme := strings.ToLower(previous.Scheme)
	targetScheme := strings.ToLower(target.Scheme)
	switch {
	case previousScheme == targetScheme && (targetScheme == "http" || targetScheme == "https"):
		if redirectPort(previous) != redirectPort(target) {
			return blockedError(
				"redirect_port_changed",
				errors.New("notification redirect must keep the port"),
			)
		}
		return nil
	case previousScheme == "http" && targetScheme == "https":
		return nil
	default:
		return blockedError(
			"redirect_scheme_changed",
			errors.New("notification redirect must use HTTP or upgrade to HTTPS"),
		)
	}
}

func redirectPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if strings.EqualFold(u.Scheme, "https") {
		return "443"
	}
	return "80"
}

func requestError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("request failed: %w", urlErr.Err)
	}
	return fmt.Errorf("request failed: %w", err)
}

func deliveryStatusError(kind string, status int, retryAfter time.Duration, detail string) error {
	class := DeliveryBlocked
	if retryableHTTPStatus(status) {
		class = DeliveryRetry
	}

	message := fmt.Sprintf("%s returned status %d", kind, status)
	if detail = deliveryErrorDetail(detail); detail != "" {
		message += ": " + detail
	}
	return classifiedError(
		class,
		fmt.Sprintf("%s_%d", kind, status),
		retryAfter,
		errors.New(message),
	)
}

func responseStatusError(kind string, resp *http.Response, detail string) error {
	if resp == nil {
		return retryError(kind+"_no_response", 0, errors.New("notification endpoint returned no response"))
	}
	return deliveryStatusError(
		kind,
		resp.StatusCode,
		parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()),
		detail,
	)
}

func retryableHTTPStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout,
		http.StatusConflict,
		http.StatusLocked,
		http.StatusTooEarly,
		http.StatusTooManyRequests:
		return true
	default:
		return status >= http.StatusInternalServerError
	}
}

func parseRetryAfter(raw string, now time.Time) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return retryAfterSeconds(seconds)
	}
	at, err := http.ParseTime(raw)
	if err != nil || !at.After(now) {
		return 0
	}
	return min(at.Sub(now), maxDeliveryRetryAfter)
}

func deliveryErrorDetail(raw string) string {
	runes := []rune(strings.Join(strings.Fields(raw), " "))
	if len(runes) > 256 {
		runes = runes[:256]
	}
	return string(runes)
}

func retryAfterSeconds(seconds int64) time.Duration {
	if seconds <= 0 {
		return 0
	}
	if seconds >= int64(maxDeliveryRetryAfter/time.Second) {
		return maxDeliveryRetryAfter
	}
	return time.Duration(seconds) * time.Second
}
