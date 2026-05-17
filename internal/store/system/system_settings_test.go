package system

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
	if err := db.AutoMigrate(&model.SystemSetting{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return New(db)
}

func TestSiteBrandDefaultsAndStoredValues(t *testing.T) {
	st := newSQLiteStore(t)
	ctx := context.Background()

	brand, err := st.GetSiteBrand(ctx)
	if err != nil {
		t.Fatalf("GetSiteBrand() error = %v", err)
	}
	if brand != DefaultSiteBrand() {
		t.Fatalf("GetSiteBrand() = %#v, want %#v", brand, DefaultSiteBrand())
	}

	want := SiteBrand{
		LogoURL:    "data:image/svg+xml;base64,PHN2Zy8+",
		PageTitle:  "Status",
		TopbarText: "Ops",
	}
	if err := st.SetSiteBrand(ctx, want); err != nil {
		t.Fatalf("SetSiteBrand() error = %v", err)
	}
	brand, err = st.GetSiteBrand(ctx)
	if err != nil {
		t.Fatalf("GetSiteBrand() error = %v", err)
	}
	if brand != want {
		t.Fatalf("GetSiteBrand() = %#v, want %#v", brand, want)
	}
}

func TestSystemSettingsPreserveOtherFields(t *testing.T) {
	st := newSQLiteStore(t)
	ctx := context.Background()

	if err := st.SetActiveThemeID(ctx, "operator"); err != nil {
		t.Fatalf("SetActiveThemeID() error = %v", err)
	}
	wantBrand := SiteBrand{
		LogoURL:    "/logo.svg",
		PageTitle:  "Status",
		TopbarText: "Ops",
	}
	if err := st.SetSiteBrand(ctx, wantBrand); err != nil {
		t.Fatalf("SetSiteBrand() error = %v", err)
	}
	themeID, err := st.GetActiveThemeID(ctx)
	if err != nil {
		t.Fatalf("GetActiveThemeID() error = %v", err)
	}
	if themeID != "operator" {
		t.Fatalf("GetActiveThemeID() = %q, want operator", themeID)
	}

	if err := st.SetActiveThemeID(ctx, "default"); err != nil {
		t.Fatalf("SetActiveThemeID(default) error = %v", err)
	}
	brand, err := st.GetSiteBrand(ctx)
	if err != nil {
		t.Fatalf("GetSiteBrand() error = %v", err)
	}
	if brand != wantBrand {
		t.Fatalf("GetSiteBrand() = %#v, want %#v", brand, wantBrand)
	}
}
