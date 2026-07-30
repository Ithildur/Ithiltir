package traffic

import (
	"context"
	"errors"
	"testing"

	pgtest "dash/internal/testutil/postgres"
)

func TestNormalizeSettingsRejectsInvalidDirection(t *testing.T) {
	_, err := NormalizeSettings(Settings{
		GuestAccessMode: GuestAccessMode(""),
		UsageMode:       UsageMode(""),
		CycleMode:       BillingCycleMode(""),
		BillingStartDay: 0,
		DirectionMode:   DirectionMode("dominant"),
	})
	if !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("NormalizeSettings(invalid) error = %v, want %v", err, ErrInvalidSettings)
	}
}

func TestIntegrationPatchSettingsMergesConcurrentFields(t *testing.T) {
	st := New(pgtest.NewDB(t))
	ctx := context.Background()

	guest := GuestAccessByNode
	usage := UsageBilling
	ref := recentTrafficTestRef()
	start := make(chan struct{})
	done := make(chan error, 2)
	go func() {
		<-start
		_, err := st.PatchSettingsAt(ctx, SettingsPatch{GuestAccessMode: &guest}, ref)
		done <- err
	}()
	go func() {
		<-start
		_, err := st.PatchSettingsAt(ctx, SettingsPatch{UsageMode: &usage}, ref)
		done <- err
	}()
	close(start)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatalf("PatchSettingsAt() error = %v", err)
		}
	}

	got, err := st.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	if got.GuestAccessMode != guest || got.UsageMode != usage {
		t.Fatalf("settings = %#v, want guest=%q usage=%q", got, guest, usage)
	}
}

func defaultSettings() Settings {
	return Settings{
		GuestAccessMode: GuestAccessDisabled,
		UsageMode:       UsageLite,
		CycleMode:       CycleCalendarMonth,
		BillingStartDay: 1,
		DirectionMode:   DirectionOut,
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
	if cycle.Mode != ServerCycleMode(CycleCalendarMonth) || cycle.BillingStartDay != 1 || cycle.BillingAnchorDate != "" || cycle.BillingTimezone != "" {
		t.Fatalf("default cycle = %#v, want explicit calendar month", cycle)
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

func TestLegacyDefaultCycleDoesNotInheritGlobalCycle(t *testing.T) {
	global := defaultSettings()
	global.CycleMode = CycleClampMonthEnd
	global.BillingStartDay = 20
	global.BillingTimezone = "UTC"

	got, err := SettingsWithServerCycle(global, ServerCycleSettings{Mode: ServerCycleDefault})
	if err != nil {
		t.Fatalf("SettingsWithServerCycle() error = %v", err)
	}
	if got.CycleMode != CycleCalendarMonth || got.BillingStartDay != 1 ||
		got.BillingAnchorDate != "" || got.BillingTimezone != "" {
		t.Fatalf("legacy default cycle = %#v, want explicit calendar month", got)
	}
}
