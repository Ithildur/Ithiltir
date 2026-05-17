package themes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"dash/internal/infra"
	themefs "dash/internal/theme"
	"dash/internal/transport/http/httperr"
	"log/slog"
)

type packageView struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	Author      string       `json:"author"`
	Description string       `json:"description"`
	Skin        themefs.Skin `json:"skin"`
	BuiltIn     bool         `json:"built_in"`
	Active      bool         `json:"active"`
	Deletable   bool         `json:"deletable"`
	Missing     bool         `json:"missing"`
	Broken      bool         `json:"broken"`
	HasPreview  bool         `json:"has_preview"`
	CreatedAt   *string      `json:"created_at"`
	UpdatedAt   *string      `json:"updated_at"`
}

func (h *handler) loadActiveThemeState(ctx context.Context) (themefs.Active, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (themefs.Active, error) {
		return themefs.ResolveActive(c, h.store, h.themes)
	})
}

func (h *handler) saveActiveThemeID(ctx context.Context, id string) error {
	_, err := infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
		return struct{}{}, themefs.SaveActiveID(c, h.store, id)
	})
	return err
}

func writeInvalidPackage(w http.ResponseWriter, err error) {
	httperr.Write(w, http.StatusBadRequest, "invalid_theme_package", err.Error())
}

func (h *handler) builtinViews(activeID string) ([]packageView, error) {
	manifests, err := themefs.ListBuiltinManifests()
	if err != nil {
		return nil, fmt.Errorf("load builtin themes: %w", err)
	}

	views := make([]packageView, 0, len(manifests))
	for _, manifest := range manifests {
		if manifest.ID == themefs.DefaultID {
			continue
		}
		views = append(
			views,
			themeView(
				manifest,
				true,
				manifest.ID == activeID,
				false,
				nil,
				nil,
				h.hasBuiltinPreview(manifest.ID),
			),
		)
	}
	return views, nil
}

func (h *handler) hasBuiltinPreview(id string) bool {
	_, err := themefs.BuiltinPreview(id)
	if err == nil {
		return true
	}
	if !errors.Is(err, os.ErrNotExist) {
		h.logger.Warn("failed to inspect builtin theme preview", err, slog.String("theme_id", id))
	}
	return false
}

func customView(item themefs.CustomTheme, active bool) packageView {
	return themeView(item.Manifest, false, active, !active, nil, item.UpdatedAt, item.HasPreview)
}

func missingView(id string) packageView {
	view := unavailableView(id)
	view.Missing = true
	return view
}

func brokenView(id string) packageView {
	view := unavailableView(id)
	view.Broken = true
	return view
}

func unavailableView(id string) packageView {
	manifest := themefs.Manifest{
		ID:   id,
		Name: id,
		Skin: themefs.Skin{
			Admin: themefs.AdminSkin{
				Shell: themefs.AdminShellSidebar,
				Frame: themefs.AdminFrameLayered,
			},
			Dashboard: themefs.DashboardSkin{
				Summary: themefs.DashboardSummaryCards,
				Density: themefs.DashboardDensityComfortable,
			},
		},
	}

	return themeView(manifest, false, false, false, nil, nil, false)
}

func themeView(
	manifest themefs.Manifest,
	builtIn bool,
	active bool,
	deletable bool,
	createdAt *time.Time,
	updatedAt *time.Time,
	hasPreview bool,
) packageView {
	return packageView{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Version:     manifest.Version,
		Author:      manifest.Author,
		Description: manifest.Description,
		Skin:        manifest.Skin,
		BuiltIn:     builtIn,
		Active:      active,
		Deletable:   deletable,
		HasPreview:  hasPreview,
		CreatedAt:   formatTimePtr(createdAt),
		UpdatedAt:   formatTimePtr(updatedAt),
	}
}

func formatTimePtr(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
