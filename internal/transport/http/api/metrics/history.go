package metrics

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dash/internal/config"
	"dash/internal/infra"
	metricspkg "dash/internal/metrics"
	"dash/internal/store/frontcache"
	"dash/internal/store/metricdata"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	authjwt "github.com/Ithildur/EiluneKit/auth/jwt"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	metric *metricdata.Store
	front  *frontcache.Store
	auth   *authjwt.Manager
}

func newHandler(metric *metricdata.Store, front *frontcache.Store, auth *authjwt.Manager) *handler {
	return &handler{metric: metric, front: front, auth: auth}
}

type historyInput struct {
	ServerID    int64
	Metric      string
	Range       string
	Aggregation metricdata.HistoryAggregation
	Device      string
	Spec        rangeSpec
}

type rangeSpec struct {
	Duration   time.Duration
	Step       time.Duration
	UseRollup  bool
	RollupBase time.Duration
}

type historyView struct {
	ServerID    int64          `json:"server_id"`
	Metric      string         `json:"metric"`
	Range       string         `json:"range"`
	Aggregation string         `json:"agg"`
	StepSec     int            `json:"step_sec"`
	Points      []historyPoint `json:"points"`
}

type historyPoint struct {
	TS    string   `json:"ts"`
	Value *float64 `json:"value"`
}

func (h *handler) historyRoute(r *routes.Blueprint) {
	r.Get(
		"/history",
		"Fetch metrics history",
		routes.Func(h.historyHandler),
		routes.Tags("metrics"),
		routes.Auth(routes.AuthOptional),
	)
}

func (h *handler) historyHandler(w http.ResponseWriter, r *http.Request) {
	in, err := parseHistory(r)
	if err != nil {
		httperr.TryWrite(w, httperr.InvalidRequest(err))
		return
	}
	allowed, err := h.canReadHistory(r.Context(), r, in.ServerID)
	if err != nil {
		httperr.TryWrite(w, httperr.ServiceUnavailable(err))
		return
	}
	if !allowed {
		httperr.TryWrite(w, httperr.Forbidden(errHistoryGuestForbidden))
		return
	}

	now := time.Now().UTC()
	since := now.Add(-in.Spec.Duration)
	points, err := infra.WithPGReadTimeout(r.Context(), func(ctx context.Context) ([]metricdata.HistoryPoint, error) {
		return h.metric.FetchHistory(ctx, metricdata.HistoryQuery{
			ServerID:    in.ServerID,
			Metric:      in.Metric,
			Aggregation: in.Aggregation,
			Device:      in.Device,
			Step:        in.Spec.Step,
			Since:       since,
			Until:       now,
			UseRollup:   in.Spec.UseRollup,
			RollupBase:  in.Spec.RollupBase,
		})
	})
	if err != nil {
		httperr.TryWrite(w, httperr.ServiceUnavailable(err))
		return
	}

	out := historyView{
		ServerID:    in.ServerID,
		Metric:      in.Metric,
		Range:       in.Range,
		Aggregation: string(in.Aggregation),
		StepSec:     int(in.Spec.Step.Seconds()),
		Points:      make([]historyPoint, 0, len(points)),
	}
	for _, p := range points {
		out.Points = append(out.Points, historyPoint{
			TS:    metricspkg.FormatTimestamp(p.TS),
			Value: p.Value,
		})
	}
	response.WriteJSON(w, http.StatusOK, out)
}

var errHistoryGuestForbidden = errors.New("history guest access denied")

func (h *handler) canReadHistory(ctx context.Context, r *http.Request, serverID int64) (bool, error) {
	if h.isAuthorized(r) {
		return true, nil
	}
	if h.metric == nil {
		return false, nil
	}

	mode, err := h.metric.GetHistoryGuestAccessMode(ctx)
	if err != nil {
		return false, err
	}
	if mode != metricdata.HistoryGuestAccessByNode {
		return false, nil
	}
	return h.isGuestVisible(ctx, serverID)
}

func (h *handler) isAuthorized(r *http.Request) bool {
	return request.HasValidBearer(r, h.auth)
}

func (h *handler) isGuestVisible(ctx context.Context, serverID int64) (bool, error) {
	if h.front == nil || serverID <= 0 {
		return false, nil
	}
	return h.front.EnsureGuestVisible(ctx, serverID, frontcache.GuestVisibilityOptions{
		CacheTimeout: config.RedisFetchTimeout,
		BuildTimeout: config.PGReadTimeout,
	})
}

func parseHistory(r *http.Request) (historyInput, error) {
	if r == nil {
		return historyInput{}, errors.New("nil request")
	}
	q := r.URL.Query()
	metric := strings.TrimSpace(q.Get("metric"))
	if metric == "" {
		return historyInput{}, errors.New("metric is required")
	}
	if !metricdata.HasHistory(metric) {
		return historyInput{}, errors.New("invalid metric")
	}
	device := strings.TrimSpace(q.Get("device"))
	if metricdata.HistoryNeedsDevice(metric) && device == "" {
		return historyInput{}, errors.New("device is required")
	}
	rangeKey := strings.TrimSpace(q.Get("range"))
	if rangeKey == "" {
		return historyInput{}, errors.New("range is required")
	}
	spec, ok := rangeSpecs[rangeKey]
	if !ok {
		return historyInput{}, errors.New("invalid range")
	}

	aggregation := metricdata.HistoryAggregation(strings.TrimSpace(q.Get("agg")))
	if aggregation == "" {
		aggregation = metricdata.HistoryAggregationAvg
	}
	switch aggregation {
	case metricdata.HistoryAggregationAvg, metricdata.HistoryAggregationMax, metricdata.HistoryAggregationMin, metricdata.HistoryAggregationLast:
	default:
		return historyInput{}, errors.New("invalid agg")
	}

	serverID, err := parseServerID(q)
	if err != nil {
		return historyInput{}, err
	}

	return historyInput{
		ServerID:    serverID,
		Metric:      metric,
		Range:       rangeKey,
		Aggregation: aggregation,
		Device:      device,
		Spec:        spec,
	}, nil
}

func parseServerID(q url.Values) (int64, error) {
	raw := strings.TrimSpace(q.Get("server_id"))
	if raw == "" {
		return 0, errors.New("server_id is required")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err == nil && id > 0 {
		return id, nil
	}
	return 0, errors.New("invalid server_id")
}

var rangeSpecs = map[string]rangeSpec{
	"30m": {Duration: 30 * time.Minute, Step: 3 * time.Second},
	"1h":  {Duration: time.Hour, Step: 6 * time.Second},
	"12h": {Duration: 12 * time.Hour, Step: 60 * time.Second},
	"24h": {Duration: 24 * time.Hour, Step: 120 * time.Second},
	"1w":  {Duration: 7 * 24 * time.Hour, Step: 15 * time.Minute, UseRollup: true, RollupBase: 15 * time.Minute},
	"15d": {Duration: 15 * 24 * time.Hour, Step: 30 * time.Minute, UseRollup: true, RollupBase: 15 * time.Minute},
	"30d": {Duration: 30 * 24 * time.Hour, Step: time.Hour, UseRollup: true, RollupBase: time.Hour},
}
