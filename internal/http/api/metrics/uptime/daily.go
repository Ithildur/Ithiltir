package uptime

import (
	"context"
	"net/http"
	"time"

	"dash/internal/http/httperr"
	"dash/internal/infra"
	"dash/internal/store/metricdata"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type dailyView struct {
	Enabled     bool                    `json:"enabled"`
	Timezone    string                  `json:"timezone"`
	GeneratedAt time.Time               `json:"generated_at"`
	WarningSLA  float64                 `json:"warning_sla"`
	ErrorSLA    float64                 `json:"error_sla"`
	Nodes       []metricdata.NodeUptime `json:"nodes"`
}

func dailyRoute(r *routes.Blueprint, h *handler) {
	r.Get("/", "Fetch 45 days of node uptime", h.dailyHandler)
	r.Get("", "Fetch 45 days of node uptime", h.dailyHandler)
}

func (h *handler) dailyHandler(w http.ResponseWriter, r *http.Request) {
	out, err := infra.WithPGReadTimeout(r.Context(), func(ctx context.Context) (dailyView, error) {
		settings, err := h.system.GetSettings(ctx)
		view := dailyView{Nodes: make([]metricdata.NodeUptime, 0), Timezone: h.loc.String(), GeneratedAt: time.Now()}
		if err != nil {
			return view, err
		}
		authorized := routes.Authenticated(r.Context())
		view.Enabled = authorized || settings.UptimeGuestVisible
		view.WarningSLA, view.ErrorSLA = settings.UptimeWarningSLA, settings.UptimeErrorSLA
		if view.Enabled {
			view.Nodes, err = h.metric.FetchUptime(ctx, view.GeneratedAt, h.loc, authorized)
		}
		return view, err
	})
	if err != nil {
		httperr.TryWrite(w, httperr.ServiceUnavailable(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, out)
}
