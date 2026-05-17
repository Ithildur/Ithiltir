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

type dailyView struct {
	Items []dailyItemView `json:"items"`
}

type dailyItemView struct {
	ServerID      int64                      `json:"server_id"`
	Iface         string                     `json:"iface"`
	UsageMode     trafficstore.UsageMode     `json:"usage_mode"`
	DirectionMode trafficstore.DirectionMode `json:"direction_mode"`
	Start         string                     `json:"start"`
	End           string                     `json:"end"`
	Stats         statView                   `json:"stats"`
}

func (h *handler) dailyRoute(r *routes.Blueprint) {
	r.Get(
		"/daily",
		"List daily traffic summaries",
		routes.Func(h.dailyHandler),
		routes.Auth(routes.AuthOptional),
	)
}

func (h *handler) dailyHandler(w http.ResponseWriter, r *http.Request) {
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

	q := queryFromInput(in, h.location)
	if in.UsageMode == trafficstore.UsageBilling {
		q.P95Enabled, err = loadP95Enabled(r.Context(), h.traffic, in.ServerID)
		if err != nil {
			httperr.TryWrite(w, httperr.ServiceUnavailable(err))
			return
		}
	}
	items, err := infra.WithPGReadTimeout(r.Context(), func(c context.Context) ([]trafficstore.TrafficDaily, error) {
		return h.traffic.TrafficDaily(c, q)
	})
	if errors.Is(err, trafficstore.ErrTrafficDailyUnsupported) {
		httperr.Write(w, http.StatusConflict, "traffic_daily_requires_billing", "daily traffic requires billing mode")
		return
	}
	if err != nil && !errors.Is(err, trafficstore.ErrNoTrafficData) {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch daily traffic")
		return
	}

	out := dailyView{Items: make([]dailyItemView, 0, len(items))}
	for _, item := range items {
		out.Items = append(out.Items, dailyItemViewFrom(item, q.DirectionMode))
	}
	response.WriteJSON(w, http.StatusOK, out)
}

func dailyItemViewFrom(item trafficstore.TrafficDaily, direction trafficstore.DirectionMode) dailyItemView {
	return dailyItemView{
		ServerID:      item.ServerID,
		Iface:         item.Iface,
		UsageMode:     item.UsageMode,
		DirectionMode: direction,
		Start:         formatTime(item.Start),
		End:           formatTime(item.End),
		Stats:         statViewFrom(item.Stat),
	}
}
