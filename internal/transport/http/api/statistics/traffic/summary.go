package traffic

import (
	"context"
	"errors"
	"net/http"

	"dash/internal/infra"
	trafficstore "dash/internal/store/traffic"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func (h *handler) summaryRoute(r *routes.Blueprint) {
	r.Get(
		"/summary",
		"Get traffic summary",
		routes.Func(h.summaryHandler),
		routes.Auth(routes.AuthOptional),
	)
}

func (h *handler) summaryHandler(w http.ResponseWriter, r *http.Request) {
	settings, err := loadSettings(r.Context(), h.traffic, h.location)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch traffic settings")
		return
	}
	in, err := parseTrafficQuery(r.URL.Query(), settings)
	if err != nil {
		httperr.TryWrite(w, httperr.InvalidRequest(err))
		return
	}
	allowed, err := h.canReadTraffic(r.Context(), r, in.ServerID)
	if err != nil {
		httperr.TryWrite(w, httperr.ServiceUnavailable(err))
		return
	}
	if !allowed {
		httperr.TryWrite(w, httperr.Forbidden(errTrafficGuestForbidden))
		return
	}
	effectiveSettings, err := loadEffectiveCycleSettings(r.Context(), h.traffic, in.ServerID, settings)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch traffic cycle settings")
		return
	}
	in.applySettings(effectiveSettings)

	serverName, err := infra.WithPGReadTimeout(r.Context(), func(c context.Context) (string, error) {
		return h.traffic.TrafficServerName(c, in.ServerID)
	})
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch traffic server")
		return
	}

	q := queryFromInput(in, h.location)
	if in.UsageMode == trafficstore.UsageBilling {
		q.P95Enabled, err = loadP95Enabled(r.Context(), h.traffic, in.ServerID)
		if err != nil {
			httperr.TryWrite(w, httperr.ServiceUnavailable(err))
			return
		}
	}
	summary, err := infra.WithPGReadTimeout(r.Context(), func(c context.Context) (trafficstore.TrafficSummary, error) {
		return h.traffic.TrafficSummary(c, q)
	})
	if err != nil && !errors.Is(err, trafficstore.ErrNoTrafficData) {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch traffic summary")
		return
	}
	response.WriteJSON(w, http.StatusOK, summaryViewWithServerName(summary, q.DirectionMode, serverName))
}
