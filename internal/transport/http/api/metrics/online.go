package metrics

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dash/internal/infra"
	metricspkg "dash/internal/metrics"
	"dash/internal/store/metricdata"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type onlineInput struct {
	ServerID int64
	Range    metricdata.OnlineRange
}

type onlineView struct {
	ServerID int64         `json:"server_id"`
	Range    string        `json:"range"`
	StepSec  int           `json:"step_sec"`
	Points   []onlinePoint `json:"points"`
}

type onlinePoint struct {
	TS     string  `json:"ts"`
	Status int     `json:"status"`
	Rate   float64 `json:"rate"`
}

func (h *handler) onlineRoute(r *routes.Blueprint) {
	r.Get(
		"/online",
		"Fetch metrics online status",
		routes.Func(h.onlineHandler),
		routes.Tags("metrics"),
		routes.Auth(routes.AuthOptional),
	)
}

func (h *handler) onlineHandler(w http.ResponseWriter, r *http.Request) {
	in, err := parseOnline(r)
	if err != nil {
		httperr.TryWrite(w, httperr.InvalidRequest(err))
		return
	}

	if !h.isAuthorized(r) {
		ok, err := h.isGuestVisible(r.Context(), in.ServerID)
		if err != nil {
			infra.WithModule("metrics").Warn("guest visibility check failed", err)
			httperr.TryWrite(w, httperr.ServiceUnavailable(err))
			return
		}
		if !ok {
			httperr.TryWrite(w, httperr.Forbidden(nil))
			return
		}
	}

	type series struct {
		Points []metricdata.OnlinePoint
		Step   time.Duration
	}
	res, err := infra.WithPGReadTimeout(r.Context(), func(ctx context.Context) (series, error) {
		points, step, err := h.metric.FetchOnlinePoints(ctx, in.ServerID, in.Range)
		return series{Points: points, Step: step}, err
	})
	if err != nil {
		if errors.Is(err, metricdata.ErrServerNotFound) {
			httperr.TryWrite(w, httperr.NotFound(err))
			return
		}
		httperr.TryWrite(w, httperr.ServiceUnavailable(err))
		return
	}

	out := onlineView{
		ServerID: in.ServerID,
		Range:    string(in.Range),
		StepSec:  int(res.Step.Seconds()),
		Points:   make([]onlinePoint, 0, len(res.Points)),
	}
	for _, p := range res.Points {
		out.Points = append(out.Points, onlinePoint{
			TS:     metricspkg.FormatTimestamp(p.TS),
			Status: p.Status,
			Rate:   p.Rate,
		})
	}
	response.WriteJSON(w, http.StatusOK, out)
}

func parseOnline(r *http.Request) (onlineInput, error) {
	if r == nil {
		return onlineInput{}, errors.New("nil request")
	}
	q := r.URL.Query()
	rawID := strings.TrimSpace(q.Get("server_id"))
	if rawID == "" {
		return onlineInput{}, errors.New("server_id is required")
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return onlineInput{}, errors.New("invalid server_id")
	}
	rawRange := strings.TrimSpace(q.Get("range"))
	if rawRange == "" {
		return onlineInput{}, errors.New("range is required")
	}
	var rng metricdata.OnlineRange
	switch rawRange {
	case string(metricdata.OnlineRange24h):
		rng = metricdata.OnlineRange24h
	case string(metricdata.OnlineRange7d), "7days":
		rng = metricdata.OnlineRange7d
	default:
		return onlineInput{}, errors.New("invalid range")
	}
	return onlineInput{ServerID: id, Range: rng}, nil
}
