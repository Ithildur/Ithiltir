package traffic

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"dash/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSQLiteStore(t *testing.T) (*Store, *gorm.DB) {
	t.Helper()

	dsn := "file:" + url.QueryEscape(t.Name()) + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&model.TrafficSetting{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return New(db), db
}

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

func TestServerCycleSettingsMissingServerUsesDefault(t *testing.T) {
	st, db := newSQLiteStore(t)
	ctx := context.Background()
	if err := db.AutoMigrate(&model.Server{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	cycle, err := st.ServerCycleSettings(ctx, 404)
	if err != nil {
		t.Fatalf("ServerCycleSettings(missing) error = %v", err)
	}
	if cycle.Mode != ServerCycleDefault {
		t.Fatalf("mode = %q, want %q", cycle.Mode, ServerCycleDefault)
	}
}

func TestSettingsPersistAndRejectInvalidStoredValue(t *testing.T) {
	st, db := newSQLiteStore(t)
	ctx := context.Background()

	settings, err := st.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings(default) error = %v", err)
	}
	settings.GuestAccessMode = GuestAccessByNode
	settings.UsageMode = UsageBilling
	settings.CycleMode = CycleClampMonthEnd
	settings.BillingStartDay = 15
	settings.DirectionMode = DirectionBoth
	if err := st.SetSettings(ctx, settings); err != nil {
		t.Fatalf("SetSettings() error = %v", err)
	}
	got, err := st.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	if got != settings {
		t.Fatalf("GetSettings() = %#v, want %#v", got, settings)
	}

	if err := db.WithContext(ctx).
		Model(&model.TrafficSetting{}).
		Where("id = ?", trafficSettingsID).
		Update("direction_mode", "dominant").
		Error; err != nil {
		t.Fatalf("Update(invalid direction) error = %v", err)
	}
	if _, err := st.GetSettings(ctx); err == nil {
		t.Fatal("GetSettings(invalid) error = nil, want error")
	}
}
