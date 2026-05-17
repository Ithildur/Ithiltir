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

func patchRoute(r *routes.Blueprint, h *handler) {
	r.Patch(
		"/",
		"Patch system settings",
		routes.Func(h.patchHandler),
		routes.Use(middleware.RequireJSONBody),
	)
}

func (h *handler) patchHandler(w http.ResponseWriter, r *http.Request) {
	var in settingsInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}

	if in.HistoryGuestAccessMode == nil && !in.hasSiteBrandFields() {
		httperr.Write(w, http.StatusBadRequest, "no_fields", "no fields to update")
		return
	}

	var mode *metricdata.HistoryGuestAccessMode
	if in.HistoryGuestAccessMode != nil {
		normalized, ok := metricdata.NormalizeHistoryGuestAccessMode(*in.HistoryGuestAccessMode)
		if !ok {
			httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid history_guest_access_mode")
			return
		}
		mode = &normalized
	}

	var brand *systemstore.SiteBrand
	if in.hasSiteBrandFields() {
		current, err := loadSiteBrand(r.Context(), h.system)
		if err != nil {
			httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch settings")
			return
		}
		next, err := in.applySiteBrand(current)
		if err != nil {
			httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid site brand fields")
			return
		}
		brand = &next
	}

	if err := saveSettings(r.Context(), h.metric, h.system, mode, brand); err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to update settings")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
