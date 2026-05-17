package traffic

import (
	"context"
	"net/http"
	"time"

	"dash/internal/infra"
	trafficstore "dash/internal/store/traffic"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type monthlyView struct {
	IncludesCurrent bool          `json:"includes_current"`
	Items           []summaryView `json:"items"`
}

func (h *handler) monthlyRoute(r *routes.Blueprint) {
	r.Get(
		"/monthly",
		"List monthly traffic summaries",
		routes.Func(h.monthlyHandler),
		routes.Auth(routes.AuthOptional),
	)
}

func (h *handler) monthlyHandler(w http.ResponseWriter, r *http.Request) {
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
	period, err := parseTrafficPeriod(r.URL.Query().Get("period"))
	if err != nil {
		httperr.TryWrite(w, httperr.InvalidRequest(err))
		return
	}
	in.Period = period
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

	q := trafficstore.TrafficMonthlyQuery{
		ServerID:          in.ServerID,
		Iface:             in.Iface,
		UsageMode:         in.UsageMode,
		CycleMode:         in.CycleMode,
		BillingStartDay:   in.BillingStartDay,
		BillingAnchorDate: in.BillingAnchorDate,
		DirectionMode:     in.DirectionMode,
		Location:          trafficstore.SettingsLocation(effectiveSettings, h.location),
		Ref:               time.Now(),
		Months:            in.Months,
		Period:            in.Period,
	}
	if in.UsageMode == trafficstore.UsageBilling {
		q.P95Enabled, err = loadP95Enabled(r.Context(), h.traffic, in.ServerID)
		if err != nil {
			httperr.TryWrite(w, httperr.ServiceUnavailable(err))
			return
		}
	}
	items, err := infra.WithPGReadTimeout(r.Context(), func(c context.Context) ([]trafficstore.TrafficSummary, error) {
		return h.traffic.TrafficMonthly(c, q)
	})
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch monthly traffic")
		return
	}

	out := monthlyView{
		IncludesCurrent: in.Period == trafficstore.TrafficPeriodCurrent,
		Items:           make([]summaryView, 0, len(items)),
	}
	for _, item := range items {
		out.Items = append(out.Items, summaryViewFrom(item, q.DirectionMode))
	}
	response.WriteJSON(w, http.StatusOK, out)
}
