package settings

import (
	"net/http"

	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func detailRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/",
		"Get system settings",
		routes.Func(h.detailHandler),
	)
}

func (h *handler) detailHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	out, err := loadSettings(r.Context(), h.metric, h.system)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch settings")
		return
	}
	response.WriteJSON(w, http.StatusOK, out)
}
