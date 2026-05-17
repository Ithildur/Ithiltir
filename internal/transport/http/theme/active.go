package theme

import (
	"context"
	"log/slog"
	"net/http"

	"dash/internal/infra"
	themefs "dash/internal/theme"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func activeCSSRoute(r *routes.Blueprint, h *handler, method string) {
	r.Handle(
		method,
		"/active.css",
		"Read active theme CSS",
		routes.Func(h.activeCSSHandler),
	)
}

func activeManifestRoute(r *routes.Blueprint, h *handler, method string) {
	r.Handle(
		method,
		"/active.json",
		"Read active theme manifest",
		routes.Func(h.activeManifestHandler),
	)
}

func (h *handler) activeCSSHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/css; charset=utf-8")

	active, err := h.active(r.Context())
	if err != nil {
		h.logger.Warn("failed to load active theme, fallback to frontend default", slog.Any("err", err))
	}

	_, _ = w.Write(active.CSS)
}

func (h *handler) activeManifestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	active, err := h.active(r.Context())
	if err != nil {
		h.logger.Warn("failed to load active theme, fallback to frontend default", slog.Any("err", err))
	}
	if themefs.IsDefault(active.ID) {
		http.NotFound(w, r)
		return
	}

	response.WriteJSON(w, http.StatusOK, active.Manifest)
}

func (h *handler) active(ctx context.Context) (themefs.Active, error) {
	if id, ok := themefs.RuntimeActiveID(); ok {
		active, err := h.themes.LoadActive(id)
		if err == nil {
			h.warnBrokenActive(active)
		}
		return active, err
	}

	active, err := infra.WithPGReadTimeout(ctx, func(c context.Context) (themefs.Active, error) {
		return themefs.ResolveActive(c, h.activeStore, h.themes)
	})
	if err != nil {
		return themefs.DefaultActive(), err
	}

	themefs.SetRuntimeActiveID(active.ConfiguredID)
	h.warnBrokenActive(active)
	return active, nil
}

func (h *handler) warnBrokenActive(active themefs.Active) {
	if active.BrokenErr == nil {
		return
	}
	h.logger.Warn(
		"active theme package is broken, fallback to frontend default",
		slog.String("theme_id", active.BrokenID),
		slog.Any("err", active.BrokenErr),
	)
}
