package day

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"dash/internal/http/httperr"
	"dash/internal/infra"
	"dash/internal/store/metricdata"
	"dash/internal/store/system"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	metric *metricdata.Store
	system *system.Store
	loc    *time.Location
}

func Router(metric *metricdata.Store, system *system.Store, loc *time.Location) *routes.Blueprint {
	h := &handler{metric: metric, system: system, loc: loc}
	r := routes.NewBlueprint()
	dayRoute(r, h)
	return r
}

func dayRoute(r *routes.Blueprint, h *handler) {
	r.Get("/", "Fetch hourly uptime for one calendar day", h.dayHandler)
	r.Get("", "Fetch hourly uptime for one calendar day", h.dayHandler)
}

func (h *handler) dayHandler(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	id, err := strconv.ParseInt(r.URL.Query().Get("server_id"), 10, 64)
	date, dateErr := time.ParseInLocation(time.DateOnly+" 15", r.URL.Query().Get("date")+" 12", h.loc)
	if dateErr == nil {
		date = metricdata.UptimeDayStart(date)
	}
	if err != nil || id <= 0 || dateErr != nil || date.Before(metricdata.UptimeStart(now, h.loc)) || date.After(now) {
		httperr.TryWrite(w, httperr.InvalidRequest(errors.New("server_id and a date within the last 45 calendar days are required")))
		return
	}
	var denied bool
	out, err := infra.WithPGReadTimeout(r.Context(), func(ctx context.Context) (metricdata.UptimeHours, error) {
		authorized := routes.Authenticated(r.Context())
		if !authorized {
			settings, err := h.system.GetSettings(ctx)
			if err != nil {
				return metricdata.UptimeHours{}, err
			}
			if !settings.UptimeGuestVisible {
				denied = true
				return metricdata.UptimeHours{}, nil
			}
		}
		return h.metric.FetchUptimeDay(ctx, id, date, now, authorized)
	})
	if denied {
		httperr.TryWrite(w, httperr.Forbidden(nil))
		return
	}
	if errors.Is(err, metricdata.ErrServerNotFound) {
		httperr.TryWrite(w, httperr.NotFound(err))
		return
	}
	if err != nil {
		httperr.TryWrite(w, httperr.ServiceUnavailable(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, out)
}
