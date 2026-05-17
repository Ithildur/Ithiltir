package traffic

import (
	"net/http"

	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func (h *handler) settingsRoute(r *routes.Blueprint) {
	r.Get(
		"/settings",
		"Get traffic settings",
		routes.Func(h.settingsHandler),
		routes.Auth(routes.AuthOptional),
	)
	r.Patch(
		"/settings",
		"Patch traffic settings",
		routes.Func(h.patchSettingsHandler),
		routes.Auth(routes.AuthRequired),
		routes.Use(h.bearer),
		routes.Use(middleware.RequireJSONBody),
	)
}

func (h *handler) settingsHandler(w http.ResponseWriter, r *http.Request) {
	settings, err := loadSettings(r.Context(), h.traffic, h.location)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch traffic settings")
		return
	}
	response.WriteJSON(w, http.StatusOK, settingsViewFrom(settings))
}

func (h *handler) patchSettingsHandler(w http.ResponseWriter, r *http.Request) {
	var in trafficSettingsInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}
	if !in.hasFields() {
		httperr.Write(w, http.StatusBadRequest, "no_fields", "no fields to update")
		return
	}

	current, err := loadSettings(r.Context(), h.traffic, h.location)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch traffic settings")
		return
	}
	next, ok := in.apply(current)
	if !ok {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid traffic settings")
		return
	}
	if err := saveSettings(r.Context(), h.traffic, next); err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to update traffic settings")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
