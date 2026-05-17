package theme

import (
	"errors"
	"net/http"
	"os"

	themefs "dash/internal/theme"
	"github.com/Ithildur/EiluneKit/http/routes"
	"github.com/go-chi/chi/v5"
)

func previewRoute(r *routes.Blueprint, h *handler, method string) {
	r.Handle(
		method,
		"/preview/{id}.png",
		"Read theme preview image",
		routes.Func(h.previewHandler),
	)
}

func (h *handler) previewHandler(w http.ResponseWriter, r *http.Request) {
	id, err := themefs.NormalizeID(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	raw, err := h.themes.LoadPreview(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		h.logger.Warn("failed to load theme preview", "theme_id", id, "err", err)
		http.Error(w, "theme preview unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(raw)
}
