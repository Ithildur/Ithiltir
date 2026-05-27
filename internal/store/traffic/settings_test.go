package traffic

import (
	"errors"
	"testing"
)

func TestSettingsDefaultDirectionIsOutbound(t *testing.T) {
	settings := DefaultSettings()
	if settings.DirectionMode != DirectionOut {
		t.Fatalf("direction = %q, want %q", settings.DirectionMode, DirectionOut)
	}

	normalized, ok := NormalizeSettings(Settings{
		GuestAccessMode: GuestAccessMode(""),
		UsageMode:       UsageMode(""),
		CycleMode:       BillingCycleMode(""),
		BillingStartDay: 0,
		DirectionMode:   DirectionMode("dominant"),
	})
	if ok {
		t.Fatalf("NormalizeSettings(invalid) ok = true, want false")
	}
	if normalized.DirectionMode != DirectionOut {
		t.Fatalf("normalized direction = %q, want %q", normalized.DirectionMode, DirectionOut)
	}
}

func TestNormalizeServerCycleSettings(t *testing.T) {
	cycle, err := NormalizeServerCycleSettings(ServerCycleSettings{
		Mode:              ServerCycleMode(CycleWHMCS),
		BillingStartDay:   31,
		BillingAnchorDate: " 2026-01-30 ",
		BillingTimezone:   " UTC ",
	})
	if err != nil {
		t.Fatalf("NormalizeServerCycleSettings() error = %v", err)
	}
	if cycle.Mode != ServerCycleMode(CycleWHMCS) {
		t.Fatalf("mode = %q, want %q", cycle.Mode, CycleWHMCS)
	}
	if cycle.BillingStartDay != 30 {
		t.Fatalf("billing day = %d, want 30", cycle.BillingStartDay)
	}
	if cycle.BillingAnchorDate != "2026-01-30" {
		t.Fatalf("anchor = %q, want 2026-01-30", cycle.BillingAnchorDate)
	}
	if cycle.BillingTimezone != "UTC" {
		t.Fatalf("timezone = %q, want UTC", cycle.BillingTimezone)
	}

	cycle, err = NormalizeServerCycleSettings(ServerCycleSettings{
		Mode:              ServerCycleMode(CycleCalendarMonth),
		BillingStartDay:   20,
		BillingAnchorDate: "2026-01-30",
	})
	if err != nil {
		t.Fatalf("NormalizeServerCycleSettings(calendar) error = %v", err)
	}
	if cycle.BillingStartDay != 1 || cycle.BillingAnchorDate != "" {
		t.Fatalf("calendar cycle = %#v, want day 1 and empty anchor", cycle)
	}

	cycle, err = NormalizeServerCycleSettings(ServerCycleSettings{
		Mode:              ServerCycleDefault,
		BillingStartDay:   31,
		BillingAnchorDate: "2026-01-30",
		BillingTimezone:   "UTC",
	})
	if err != nil {
		t.Fatalf("NormalizeServerCycleSettings(default) error = %v", err)
	}
	if cycle.Mode != ServerCycleDefault || cycle.BillingStartDay != 1 || cycle.BillingAnchorDate != "" || cycle.BillingTimezone != "" {
		t.Fatalf("default cycle = %#v, want cleared default", cycle)
	}

	if _, err := NormalizeServerCycleSettings(ServerCycleSettings{
		Mode:              ServerCycleMode(CycleWHMCS),
		BillingStartDay:   31,
		BillingAnchorDate: "not-a-date",
	}); !errors.Is(err, ErrInvalidServerCycleAnchorDate) {
		t.Fatalf("NormalizeServerCycleSettings(invalid anchor) error = %v, want %v", err, ErrInvalidServerCycleAnchorDate)
	}
	if _, err := NormalizeServerCycleSettings(ServerCycleSettings{
		Mode:            ServerCycleMode(CycleClampMonthEnd),
		BillingStartDay: 15,
		BillingTimezone: "No/Such_Zone",
	}); !errors.Is(err, ErrInvalidServerCycleTimezone) {
		t.Fatalf("NormalizeServerCycleSettings(invalid timezone) error = %v, want %v", err, ErrInvalidServerCycleTimezone)
	}
	if _, err := NormalizeServerCycleSettings(ServerCycleSettings{
		Mode:              ServerCycleDefault,
		BillingStartDay:   31,
		BillingAnchorDate: "not-a-date",
	}); !errors.Is(err, ErrInvalidServerCycleAnchorDate) {
		t.Fatalf("NormalizeServerCycleSettings(default invalid anchor) error = %v, want %v", err, ErrInvalidServerCycleAnchorDate)
	}
}
