package themes

import (
	"net/http"

	themefs "dash/internal/theme"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/routes"
	"github.com/go-chi/chi/v5"
)

func applyRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/{id}/apply",
		"Apply active theme package",
		routes.Func(h.applyHandler),
	)
}

func (h *handler) applyHandler(w http.ResponseWriter, r *http.Request) {
	id, err := themefs.NormalizeID(chi.URLParam(r, "id"))
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_theme_id", err.Error())
		return
	}

	exists, err := h.themes.ThemeExists(id)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "theme_unavailable", "failed to validate theme")
		return
	}
	if !exists {
		httperr.Write(w, http.StatusNotFound, "not_found", "theme not found")
		return
	}

	if err := h.saveActiveThemeID(r.Context(), id); err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to apply theme")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
