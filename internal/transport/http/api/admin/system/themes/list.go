package themes

import (
	"net/http"

	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
	"log/slog"
)

func listRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/",
		"List installed theme packages",
		routes.Func(h.listHandler),
	)
}

func (h *handler) listHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	state, err := h.loadActiveThemeState(r.Context())
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to load active theme")
		return
	}
	if state.MissingID != "" {
		httperr.WriteWarningHeader(w, "theme_active_missing")
	}
	if state.BrokenID != "" {
		httperr.WriteWarningHeader(w, "theme_active_broken")
		h.logger.Warn("active theme package is broken", state.BrokenErr, slog.String("theme_id", state.BrokenID))
	}

	custom, warnings, err := h.themes.ListCustomWithWarnings()
	if err != nil {
		httperr.Write(
			w,
			http.StatusInternalServerError,
			"theme_storage_unavailable",
			"failed to list themes",
		)
		return
	}
	for _, warning := range warnings {
		h.logger.Warn("skipped invalid theme package", warning.Err, slog.String("theme_id", warning.ID))
	}

	builtin, err := h.builtinViews(state.ID)
	if err != nil {
		httperr.Write(w, http.StatusInternalServerError, "theme_unavailable", "failed to load builtin themes")
		return
	}

	items := make([]packageView, 0, len(custom)+len(builtin)+1)
	items = append(items, builtin...)

	for _, item := range custom {
		items = append(items, customView(item, item.Manifest.ID == state.ID))
	}

	if state.MissingID != "" && !containsThemeID(items, state.MissingID) {
		items = append(items, missingView(state.MissingID))
	}
	if state.BrokenID != "" && !containsThemeID(items, state.BrokenID) {
		items = append(items, brokenView(state.BrokenID))
	}

	response.WriteJSON(w, http.StatusOK, items)
}

func containsThemeID(items []packageView, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
