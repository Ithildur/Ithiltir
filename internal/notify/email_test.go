package notify

import (
	"strings"
	"testing"
)

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
