package notify

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"html"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"dash/internal/lang"
)

const (
	alertThreadDomain     = "ithiltir.local"
	alertOpenTransition   = "opened"
	alertClosedTransition = "closed"
)

func sendSMTP(ctx context.Context, cfg EmailConfig, msg Message) error {
	fromAddr, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return fmt.Errorf("from is invalid")
	}
	toHeader, toAddrs, err := parseMailList(cfg.To)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(cfg.SMTPHost, fmt.Sprintf("%d", cfg.SMTPPort))
	dialer := net.Dialer{Timeout: 10 * time.Second}

	var conn net.Conn
	if cfg.UseTLS && cfg.SMTPPort == 465 {
		tlsConn, err := tls.DialWithDialer(&dialer, "tcp", addr, &tls.Config{
			ServerName: cfg.SMTPHost,
		})
		if err != nil {
			return err
		}
		conn = tlsConn
	} else {
		c, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return err
		}
		conn = c
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return err
		}
	}

	client, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Quit()

	if cfg.UseTLS && cfg.SMTPPort != 465 {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("smtp server does not support starttls")
		}
		if err := client.StartTLS(&tls.Config{ServerName: cfg.SMTPHost}); err != nil {
			return err
		}
	}

	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	if err := client.Mail(fromAddr.Address); err != nil {
		return err
	}
	for _, rcpt := range toAddrs {
		if err := client.Rcpt(rcpt); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(composeEmail(cfg.From, toHeader, msg)); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return nil
}

func composeEmail(from string, to []string, msg Message) []byte {
	subject := strings.TrimSpace(msg.Title)
	if subject == "" {
		subject = emailFallbackSubject(msg.Metadata)
	}
	subject = cleanHeaderValue(subject)
	body := strings.TrimSpace(msg.Body)
	if body == "" {
		body = msg.Text()
	}
	boundary := emailBoundary(msg)
	headers := []string{
		"From: " + from,
		"To: " + strings.Join(to, ", "),
		"Subject: " + encodeSubject(subject),
	}
	headers = append(headers, emailThreadHeaders(msg)...)
	headers = append(headers,
		"MIME-Version: 1.0",
		fmt.Sprintf(`Content-Type: multipart/alternative; boundary="%s"`, boundary),
		"",
		"--"+boundary,
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		normalizeEmailBody(body),
		"--"+boundary,
		"Content-Type: text/html; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		renderHTML(subject, body, msg.Metadata),
		"--"+boundary+"--",
		"",
	)
	return []byte(strings.Join(headers, "\r\n"))
}

func encodeSubject(subject string) string {
	for _, r := range subject {
		if r > 127 {
			return mime.QEncoding.Encode("UTF-8", subject)
		}
	}
	return subject
}

func emailBoundary(msg Message) string {
	sum := sha1.Sum([]byte(msg.Title + "\n" + msg.Body + "\n" + msg.Metadata["dedupe_key"]))
	return "ithiltir-" + hex.EncodeToString(sum[:12])
}

func emailThreadHeaders(msg Message) []string {
	if len(msg.Metadata) == 0 {
		return nil
	}
	eventID := cleanIDToken(msg.Metadata["event_id"])
	channelID := cleanIDToken(msg.Metadata["channel_id"])
	transition := cleanIDToken(msg.Metadata["transition"])
	if eventID == "" || channelID == "" || transition == "" {
		return nil
	}

	headers := []string{
		"Message-ID: " + alertMessageID(eventID, channelID, transition),
	}
	if transition == alertClosedTransition {
		opened := alertMessageID(eventID, channelID, alertOpenTransition)
		headers = append(headers,
			"In-Reply-To: "+opened,
			"References: "+opened,
		)
	}
	return headers
}

func alertMessageID(eventID, channelID, transition string) string {
	return fmt.Sprintf("<ithiltir-alert-%s-%s-%s@%s>", eventID, channelID, transition, alertThreadDomain)
}

func renderHTML(title, body string, metadata map[string]string) string {
	accent := "#2563eb"
	tint := "#eff6ff"
	border := "#bfdbfe"
	transition := ""
	if metadata != nil {
		transition = metadata["transition"]
	}
	switch transition {
	case alertOpenTransition:
		accent = "#dc2626"
		tint = "#fff1f2"
		border = "#fecdd3"
	case alertClosedTransition:
		accent = "#16a34a"
		tint = "#f0fdf4"
		border = "#bbf7d0"
	}

	safeTitle := html.EscapeString(title)
	safeBody := html.EscapeString(normalizeEmailBody(body))
	badge := ""
	if transition != "" {
		badge = `<div style="font-size:12px;line-height:16px;font-weight:700;letter-spacing:0;text-transform:uppercase;color:` + accent + `;">` + html.EscapeString(emailTransitionLabel(transition, metadata)) + `</div>`
	}
	return `<!doctype html>
<html>
<body style="margin:0;background:#f4f6f8;padding:24px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;color:#111827;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="border-collapse:collapse;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="border-collapse:collapse;max-width:640px;background:#ffffff;border:1px solid #e5e7eb;border-top:4px solid ` + accent + `;border-radius:12px;">
          <tr>
            <td style="padding:24px;">
              ` + badge + `
              <h1 style="margin:8px 0 0;font-size:22px;line-height:30px;font-weight:700;color:#111827;">` + safeTitle + `</h1>
              <div style="margin-top:18px;padding:16px;background:` + tint + `;border:1px solid ` + border + `;border-radius:10px;font-size:15px;line-height:24px;color:#111827;white-space:pre-wrap;">` + safeBody + `</div>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`
}

func emailFallbackSubject(metadata map[string]string) string {
	if lang.Normalize(metadata["language"]) == lang.English {
		return "Alert notification"
	}
	return "告警通知"
}

func emailTransitionLabel(transition string, metadata map[string]string) string {
	language := lang.Normalize(metadata["language"])
	switch transition {
	case alertOpenTransition:
		if language == lang.English {
			return "Triggered"
		}
		return "告警触发"
	case alertClosedTransition:
		if language == lang.English {
			return "Recovered"
		}
		return "告警恢复"
	default:
		return transition
	}
}

func cleanHeaderValue(raw string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(raw)
}

func cleanIDToken(raw string) string {
	raw = strings.TrimSpace(raw)
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeEmailBody(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	return strings.ReplaceAll(raw, "\n", "\r\n")
}

func parseMailList(raw []string) ([]string, []string, error) {
	if len(raw) == 0 {
		return nil, nil, fmt.Errorf("to cannot be empty")
	}
	header := make([]string, 0, len(raw))
	rcpt := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, nil, fmt.Errorf("to cannot contain empty values")
		}
		addr, err := mail.ParseAddress(item)
		if err != nil {
			return nil, nil, fmt.Errorf("to is invalid")
		}
		header = append(header, addr.String())
		rcpt = append(rcpt, addr.Address)
	}
	return header, rcpt, nil
}
