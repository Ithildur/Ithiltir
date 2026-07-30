package system

import (
	"context"
	"testing"

	pgtest "dash/internal/testutil/postgres"
)

func TestIntegrationSystemSettings(t *testing.T) {
	st := New(pgtest.NewDB(t))
	ctx := context.Background()

	t.Run("updates preserve other fields", func(t *testing.T) {
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
		if err := st.PatchSiteBrand(ctx, fullSiteBrandPatch(wantBrand)); err != nil {
			t.Fatalf("PatchSiteBrand() error = %v", err)
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
		policy, err := st.GetDashUpdatePolicy(ctx)
		if err != nil {
			t.Fatalf("GetDashUpdatePolicy() error = %v", err)
		}
		if policy.Channel != DashUpdateChannelPrerelease || policy.Mode != DashUpdateModeNotify {
			t.Fatalf("GetDashUpdatePolicy() = %#v", policy)
		}
	})

	t.Run("brand patch preserves unspecified fields", func(t *testing.T) {
		want := SiteBrand{
			LogoURL:    "http://legacy.example.test/logo.png",
			PageTitle:  "Old title",
			TopbarText: "Old topbar",
		}
		if err := st.PatchSiteBrand(ctx, fullSiteBrandPatch(want)); err != nil {
			t.Fatalf("PatchSiteBrand() error = %v", err)
		}

		title := "New title"
		if err := st.PatchSiteBrand(ctx, SiteBrandPatch{PageTitle: &title}); err != nil {
			t.Fatalf("PatchSiteBrand() error = %v", err)
		}
		want.PageTitle = title
		got, err := st.GetSiteBrand(ctx)
		if err != nil {
			t.Fatalf("GetSiteBrand() error = %v", err)
		}
		if got != want {
			t.Fatalf("GetSiteBrand() = %#v, want %#v", got, want)
		}
	})
}

func fullSiteBrandPatch(brand SiteBrand) SiteBrandPatch {
	return SiteBrandPatch{
		LogoURL:    &brand.LogoURL,
		PageTitle:  &brand.PageTitle,
		TopbarText: &brand.TopbarText,
	}
}
