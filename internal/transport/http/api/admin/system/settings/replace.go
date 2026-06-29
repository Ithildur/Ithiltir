package settings

import (
	"net/http"

	"dash/internal/store/metricdata"
	systemstore "dash/internal/store/system"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func replaceRoute(r *routes.Blueprint, h *handler) {
	r.Put(
		"/",
		"Replace system settings",
		routes.Func(h.replaceHandler),
		routes.Use(middleware.RequireJSONBody),
	)
}

func (h *handler) replaceHandler(w http.ResponseWriter, r *http.Request) {
	var in settingsInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}

	if in.HistoryGuestAccessMode == nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "history_guest_access_mode is required")
		return
	}
	mode, ok := metricdata.NormalizeHistoryGuestAccessMode(*in.HistoryGuestAccessMode)
	if !ok {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid history_guest_access_mode")
		return
	}

	if in.DashUpdateChannel == nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "dash_update_channel is required")
		return
	}
	channel, ok := systemstore.ParseDashUpdateChannel(*in.DashUpdateChannel)
	if !ok {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid dash_update_channel")
		return
	}

	if in.DashUpdateMode == nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "dash_update_mode is required")
		return
	}
	updateMode, ok := systemstore.ParseDashUpdateMode(*in.DashUpdateMode)
	if !ok {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid dash_update_mode")
		return
	}

	brand, err := in.siteBrand()
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid site brand fields")
		return
	}

	if err := saveSettings(r.Context(), h.metric, h.system, &mode, &channel, &updateMode, &brand); err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to update settings")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
