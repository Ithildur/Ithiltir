package themes

import (
	"log/slog"
	"net/http"

	themefs "dash/internal/theme"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func listRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/",
		"List installed theme packages",
		routes.Func(h.listHandler),
	)
}

func (h *handler) listHandler(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	state, err := h.loadActiveThemeState(r.Context())
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to load active theme")
		return
	}
	switch state.State {
	case themefs.ActiveMissing:
		httperr.WriteWarningHeader(w, "theme_active_missing")
	case themefs.ActiveBroken:
		httperr.WriteWarningHeader(w, "theme_active_broken")
		h.logger.Warn(
			"active theme package is broken",
			state.Err,
			slog.String("theme_id", state.ConfiguredID),
		)
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
		h.logger.Warn(
			"theme package warning",
			warning.Err,
			slog.String("theme_id", warning.ID),
		)
	}

	builtin, err := h.builtinViews(state.ResolvedID)
	if err != nil {
		httperr.Write(w, http.StatusInternalServerError, "theme_unavailable", "failed to load builtin themes")
		return
	}

	items := make([]packageView, 0, len(custom)+len(builtin)+1)
	items = append(items, builtin...)

	for _, item := range custom {
		items = append(items, customView(item, item.Manifest.ID == state.ResolvedID))
	}
	items = markActiveIssue(items, state)

	response.WriteJSON(w, http.StatusOK, items)
}

func markActiveIssue(items []packageView, state themefs.Active) []packageView {
	if state.State != themefs.ActiveMissing && state.State != themefs.ActiveBroken {
		return items
	}
	for i := range items {
		if items[i].ID != state.ConfiguredID {
			continue
		}
		items[i].Active = false
		items[i].Deletable = false
		items[i].Missing = state.State == themefs.ActiveMissing
		items[i].Broken = state.State == themefs.ActiveBroken
		return items
	}

	if state.State == themefs.ActiveMissing {
		return append(items, missingView(state.ConfiguredID))
	}
	return append(items, brokenView(state.ConfiguredID))
}
