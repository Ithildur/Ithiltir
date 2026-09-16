package metricdata

import (
	"testing"
	"time"
)

func TestUptimeCalendar(t *testing.T) {
	for _, tc := range []struct {
		zone, date string
		hours      int
	}{
		{"Asia/Kathmandu", "2026-09-16", 24},
		{"America/New_York", "2026-03-08", 23},
		{"America/New_York", "2026-11-01", 25},
		{"America/Santiago", "2026-09-06", 23},
		{"America/Havana", "2026-11-01", 25},
	} {
		t.Run(tc.zone+"/"+tc.date, func(t *testing.T) {
			loc, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			noon, err := time.ParseInLocation(time.DateOnly+" 15", tc.date+" 12", loc)
			if err != nil {
				t.Fatal(err)
			}
			start := UptimeDayStart(noon)
			end := UptimeDayStart(noon.AddDate(0, 0, 1))
			if start.Format(time.DateOnly) != tc.date || start.Add(-time.Minute).Format(time.DateOnly) == tc.date || end.Sub(start) != time.Duration(tc.hours)*time.Hour {
				t.Fatalf("calendar day = %v to %v, want %s with %d hours", start, end, tc.date, tc.hours)
			}
		})
	}
}
