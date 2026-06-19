package system

import (
	"context"
	"testing"

	pgtest "dash/internal/testutil/postgres"
)

func TestIntegrationSystemSettingsPreserveOtherFields(t *testing.T) {
	st := New(pgtest.NewDB(t))
	ctx := context.Background()

	brand, err := st.GetSiteBrand(ctx)
	if err != nil {
		t.Fatalf("GetSiteBrand() error = %v", err)
	}
	if brand != DefaultSiteBrand() {
		t.Fatalf("GetSiteBrand() = %#v, want %#v", brand, DefaultSiteBrand())
	}

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
	if err := st.SetDashUpdateChannel(ctx, DashUpdateChannelPrerelease); err != nil {
		t.Fatalf("SetDashUpdateChannel() error = %v", err)
	}
	if err := st.SetDashUpdateMode(ctx, DashUpdateModeNotify); err != nil {
		t.Fatalf("SetDashUpdateMode() error = %v", err)
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
	brand, err = st.GetSiteBrand(ctx)
	if err != nil {
		t.Fatalf("GetSiteBrand() error = %v", err)
	}
	if brand != wantBrand {
		t.Fatalf("GetSiteBrand() = %#v, want %#v", brand, wantBrand)
	}
	channel, err := st.GetDashUpdateChannel(ctx)
	if err != nil {
		t.Fatalf("GetDashUpdateChannel() error = %v", err)
	}
	if channel != DashUpdateChannelPrerelease {
		t.Fatalf("GetDashUpdateChannel() = %q, want %q", channel, DashUpdateChannelPrerelease)
	}
	mode, err := st.GetDashUpdateMode(ctx)
	if err != nil {
		t.Fatalf("GetDashUpdateMode() error = %v", err)
	}
	if mode != DashUpdateModeNotify {
		t.Fatalf("GetDashUpdateMode() = %q, want %q", mode, DashUpdateModeNotify)
	}
}
