package metrics

import (
	"time"
)

func stringOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func int16OrZero(p *int16) int16 {
	if p == nil {
		return 0
	}
	return *p
}

// FormatTimestamp formats a time.Time for JSON payloads, returning empty if zero.
func FormatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func ParseReportedAt(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &t
}
