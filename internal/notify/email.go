package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
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
		subject = "告警通知"
	}
	body := strings.TrimSpace(msg.Body)
	if body == "" {
		body = msg.Text()
	}
	headers := []string{
		"From: " + from,
		"To: " + strings.Join(to, ", "),
		"Subject: " + encodeSubject(subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		body,
	}
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
