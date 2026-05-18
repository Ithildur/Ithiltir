package notify

import (
	"strings"
	"testing"
)

func TestComposeEmailClosedThreadHeaders(t *testing.T) {
	raw := string(composeEmail("Monitor <alerts@example.com>", []string{"ops@example.com"}, Message{
		Title: "告警恢复: CPU 使用率过高 @ 9900x",
		Body:  "状态: closed\n当前值: <42%>",
		Metadata: map[string]string{
			"event_id":   "77",
			"channel_id": "9",
			"transition": "closed",
			"dedupe_key": "alert:77:closed:9",
		},
	}))

	for _, want := range []string{
		"Message-ID: <ithiltir-alert-77-9-closed@ithiltir.local>",
		"In-Reply-To: <ithiltir-alert-77-9-opened@ithiltir.local>",
		"References: <ithiltir-alert-77-9-opened@ithiltir.local>",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("expected email to contain %q, got:\n%s", want, raw)
		}
	}
}

func TestComposeEmailEscapesHTMLBody(t *testing.T) {
	raw := string(composeEmail("Monitor <alerts@example.com>", []string{"ops@example.com"}, Message{
		Title: "Alert",
		Body:  "Current value: <42%>",
	}))
	htmlAt := strings.Index(raw, "Content-Type: text/html; charset=UTF-8")
	if htmlAt < 0 {
		t.Fatalf("expected HTML part, got:\n%s", raw)
	}
	html := raw[htmlAt:]
	if !strings.Contains(html, "Current value: &lt;42%&gt;") {
		t.Fatalf("expected escaped HTML body, got:\n%s", html)
	}
	if strings.Contains(html, "Current value: <42%>") {
		t.Fatalf("expected raw body to stay out of HTML part, got:\n%s", html)
	}
}

func TestComposeEmailEnglishFallbacks(t *testing.T) {
	raw := string(composeEmail("Monitor <alerts@example.com>", []string{"ops@example.com"}, Message{
		Body: "Status: opened",
		Metadata: map[string]string{
			"language":   "en",
			"event_id":   "77",
			"channel_id": "9",
			"transition": "opened",
		},
	}))

	for _, want := range []string{
		"Subject: Alert notification",
		">Triggered<",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("expected email to contain %q, got:\n%s", want, raw)
		}
	}
	if strings.Contains(raw, "告警通知") || strings.Contains(raw, "告警触发") {
		t.Fatalf("expected English email shell, got:\n%s", raw)
	}
}
