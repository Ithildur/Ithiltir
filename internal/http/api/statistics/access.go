package statistics

import (
	"context"
	"net/http"

	"dash/internal/config"
	"dash/internal/http/httperr"
	"dash/internal/store"
	"dash/internal/store/metricdata"
	trafficstore "dash/internal/store/traffic"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type accessView struct {
	HistoryGuestAccessMode metricdata.HistoryGuestAccessMode `json:"history_guest_access_mode"`
	GuestAccessMode        trafficstore.GuestAccessMode      `json:"traffic_guest_access_mode"`
}

func (h *handler) accessRoute(r *routes.Blueprint) {
	r.Get(
		"/access",
		"Get statistics access settings",
		h.accessHandler,
		routes.Tags("statistics"),
		routes.Auth(routes.AuthOptional),
		routes.Use(h.optionalBearer),
	)
}

func (h *handler) accessHandler(w http.ResponseWriter, r *http.Request) {
	out, err := loadAccess(r.Context(), h.store)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch access settings")
		return
	}
	response.WriteJSON(w, http.StatusOK, out)
}

func loadAccess(ctx context.Context, st *store.Stores) (accessView, error) {
	dbCtx, cancel := context.WithTimeout(ctx, config.PGReadTimeout)
	defer cancel()
	historyMode, err := st.Metric.GetHistoryGuestAccessMode(dbCtx)
	if err != nil {
		return accessView{}, err
	}
	trafficSettings, err := st.Traffic.GetSettings(dbCtx)
	if err != nil {
		return accessView{}, err
	}
	return accessView{
		HistoryGuestAccessMode: historyMode,
		GuestAccessMode:        trafficSettings.GuestAccessMode,
	}, nil
}
