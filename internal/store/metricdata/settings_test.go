package metricdata

import (
	"context"
	"net/url"
	"testing"

	"dash/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSQLiteStore(t *testing.T) *Store {
	t.Helper()

	dsn := "file:" + url.QueryEscape(t.Name()) + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&model.MetricSetting{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return New(db)
}

func TestHistoryGuestAccessModeDefaultsAndRejectsInvalid(t *testing.T) {
	st := newSQLiteStore(t)
	ctx := context.Background()

	mode, err := st.GetHistoryGuestAccessMode(ctx)
	if err != nil {
		t.Fatalf("GetHistoryGuestAccessMode() error = %v", err)
	}
	if mode != HistoryGuestAccessDisabled {
		t.Fatalf("GetHistoryGuestAccessMode() = %q, want %q", mode, HistoryGuestAccessDisabled)
	}

	if err := st.SetHistoryGuestAccessMode(ctx, HistoryGuestAccessByNode); err != nil {
		t.Fatalf("SetHistoryGuestAccessMode(by_node) error = %v", err)
	}
	mode, err = st.GetHistoryGuestAccessMode(ctx)
	if err != nil {
		t.Fatalf("GetHistoryGuestAccessMode() error = %v", err)
	}
	if mode != HistoryGuestAccessByNode {
		t.Fatalf("GetHistoryGuestAccessMode() = %q, want %q", mode, HistoryGuestAccessByNode)
	}

	if err := st.SetHistoryGuestAccessMode(ctx, HistoryGuestAccessMode("public")); err == nil {
		t.Fatal("SetHistoryGuestAccessMode(invalid) error = nil, want error")
	}
}

func TestHistoryGuestAccessModeInvalidStoredValueReturnsError(t *testing.T) {
	st := newSQLiteStore(t)
	ctx := context.Background()

	if err := st.db.WithContext(ctx).Create(&model.MetricSetting{
		ID:                     metricSettingsID,
		HistoryGuestAccessMode: "public",
	}).Error; err != nil {
		t.Fatalf("Create(MetricSetting) error = %v", err)
	}

	if _, err := st.GetHistoryGuestAccessMode(ctx); err == nil {
		t.Fatal("GetHistoryGuestAccessMode() error = nil, want error")
	}
}
