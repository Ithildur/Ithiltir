package themes

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"dash/internal/infra"
	themefs "dash/internal/theme"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/routes"
	"github.com/go-chi/chi/v5"
)

func deleteRoute(r *routes.Blueprint, h *handler) {
	r.Delete(
		"/{id}",
		"Delete theme package",
		routes.Func(h.deleteHandler),
	)
}

func (h *handler) deleteHandler(w http.ResponseWriter, r *http.Request) {
	id, err := themefs.NormalizeID(chi.URLParam(r, "id"))
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_theme_id", err.Error())
		return
	}
	if themefs.IsDefault(id) || themefs.IsBuiltin(id) {
		httperr.Write(w, http.StatusConflict, "builtin_theme_not_deletable", "builtin theme cannot be deleted")
		return
	}

	exists, err := h.themes.CustomExists(id)
	if err != nil {
		httperr.Write(w, http.StatusInternalServerError, "theme_storage_unavailable", "failed to inspect theme storage")
		return
	}
	if !exists {
		httperr.Write(w, http.StatusNotFound, "not_found", "theme not found")
		return
	}

	activeID, err := infra.WithPGReadTimeout(r.Context(), func(ctx context.Context) (string, error) {
		return themefs.ReadActiveID(ctx, h.store)
	})
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to load active theme")
		return
	}
	if activeID == id {
		httperr.Write(w, http.StatusConflict, "active_theme_not_deletable", "active theme cannot be deleted")
		return
	}

	if err := h.themes.RemoveCustom(id); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			httperr.Write(w, http.StatusNotFound, "not_found", "theme not found")
			return
		}
		h.logger.Warn("failed to remove theme files", err, slog.String("theme_id", id))
		httperr.Write(w, http.StatusInternalServerError, "theme_storage_unavailable", "failed to delete theme files")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
