package node

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"dash/internal/config"
	"dash/internal/http/httperr"
	"dash/internal/infra"
	"dash/internal/metrics"
	"dash/internal/model"
	"dash/internal/serverid"
	alertstore "dash/internal/store/alert"
	"dash/internal/store/frontcache"
	"dash/internal/store/metricdata"
	nodestore "dash/internal/store/node"
	"dash/internal/version"
	"github.com/Ithildur/EiluneKit/contextutil"
	"github.com/Ithildur/EiluneKit/http/decoder"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
	kitlog "github.com/Ithildur/EiluneKit/logging"
)

var errMetricsIdentityChanged = errors.New("node identity changed while acquiring the projection lock")

const metricsIdentityAttempts = 3

type handler struct {
	node          *nodestore.Store
	metric        *metricdata.Store
	front         *frontcache.Store
	alert         *alertstore.Store
	serverID      *serverid.Store
	staleAfterSec int
	failedAuth    http.Handler
}

func newHandler(node *nodestore.Store, metric *metricdata.Store, front *frontcache.Store, alert *alertstore.Store, serverID *serverid.Store, staleAfterSec int, failedAuth http.Handler) *handler {
	return &handler{
		node:          node,
		metric:        metric,
		front:         front,
		alert:         alert,
		serverID:      serverID,
		staleAfterSec: staleAfterSec,
		failedAuth:    failedAuth,
	}
}

func (h *handler) metricsRoute(r *routes.Blueprint) {
	r.Post(
		"/metrics",
		"Push node metrics",
		h.metricsHandler,
		routes.Tags("node"),
	)
}

func (h *handler) metricsHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	receivedAt := time.Now().UTC()
	// Receipt, authentication and node-lock waits share the persistence budget;
	// no report may first enter storage after uptime finalizes its minute.
	ctx, cancel := context.WithDeadline(r.Context(), receivedAt.Add(config.PGWriteTimeout))
	defer cancel()
	logger := infra.WithModule("node")

	secret, server, err := h.authenticate(ctx, r, logger)
	if err != nil {
		h.writeError(w, r, logger, err)
		return
	}
	in, err := h.validateMetricsInput(r, receivedAt, secret)
	if err != nil {
		h.writeError(w, r, logger, err)
		return
	}

	var validated *validatedMetrics
	for attempt := range metricsIdentityAttempts {
		if err != nil {
			break
		}
		lockedID := server.ID
		err = h.node.WithMetricsIngest(lockedID, func() error {
			current, authErr := h.serverBySecret(ctx, in.secret, logger)
			if authErr != nil {
				return authErr
			}
			if current.ID != lockedID {
				server = current
				return errMetricsIdentityChanged
			}

			validated, err = validateMetrics(in, current)
			if err != nil {
				return err
			}
			var currentUpdated bool
			currentUpdated, err = h.persistMetrics(ctx, validated, r, logger)
			if err == nil && currentUpdated && validated.snapshot != nil {
				h.alert.MarkServerMetrics(validated.server.ID, *validated.snapshot)
			}
			return err
		})
		if !errors.Is(err, errMetricsIdentityChanged) {
			break
		}
		if attempt+1 < metricsIdentityAttempts {
			err = nil
		}
	}
	if errors.Is(err, errMetricsIdentityChanged) {
		logger.Warn(ctx, "node identity kept changing during metrics ingest", err)
		err = httperr.ServiceUnavailable(err)
	}
	if err != nil {
		h.writeError(w, r, logger, err)
		return
	}
	h.writeMetricsResponse(ctx, w, validated, logger)
}

type metricsInput struct {
	secret     string
	report     metrics.NodeReport
	receivedAt time.Time
}

type validatedMetrics struct {
	server   model.Server
	report   metrics.NodeReport
	metric   model.ServerMetric
	runtime  model.MetricRuntime
	snapshot *metrics.NodeView
}

type metricsResponse struct {
	OK     bool            `json:"ok"`
	Update *updateManifest `json:"update"`
}

type updateManifest struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

func (h *handler) validateMetricsInput(r *http.Request, receivedAt time.Time, secret string) (*metricsInput, error) {
	report, err := decodeReport(r)
	if err != nil {
		if errors.Is(err, decoder.ErrBodyTooLarge) {
			return nil, httperr.BodyTooLarge(err)
		}
		return nil, httperr.InvalidRequest(err)
	}

	report.Version = strings.TrimSpace(report.Version)
	if report.Version == "" {
		return nil, httperr.InvalidMetrics(nil)
	}
	if err := version.ValidateNodeVersion(report.Version); err != nil {
		return nil, httperr.InvalidMetrics(err)
	}
	if strings.TrimSpace(report.Hostname) == "" {
		return nil, httperr.InvalidMetrics(nil)
	}
	if report.Timestamp.IsZero() {
		return nil, httperr.InvalidMetrics(nil)
	}
	if err := metrics.ValidateReport(report); err != nil {
		return nil, httperr.InvalidMetrics(err)
	}
	return &metricsInput{secret: secret, report: report, receivedAt: receivedAt}, nil
}

func validateMetrics(in *metricsInput, server model.Server) (*validatedMetrics, error) {
	report, reportedAtRaw := metrics.NormalizeReport(server.ID, server.DisplayOrder, in.report, in.receivedAt)
	metric, runtime, err := metrics.BuildMetric(server.ID, report.Metrics, in.receivedAt, reportedAtRaw)
	if err != nil {
		return nil, httperr.InvalidMetrics(err)
	}

	return &validatedMetrics{
		server:  server,
		report:  report,
		metric:  metric,
		runtime: runtime,
	}, nil
}

func decodeReport(r *http.Request) (metrics.NodeReport, error) {
	var report metrics.NodeReport
	if err := decoder.DecodeJSONBody(r, &report); err != nil {
		return report, err
	}
	return report, nil
}

func (h *handler) persistMetrics(ctx context.Context, validated *validatedMetrics, r *http.Request, logger *kitlog.Helper) (bool, error) {
	updates, nextIP := buildServerUpdates(validated.server, r)

	// Disk IO history is sourced from base_io; disk.physical participates only
	// in report validation.
	currentUpdated, err := h.metric.SaveMetrics(ctx, metricdata.MetricsSample{
		ServerID:  validated.server.ID,
		Metric:    validated.metric,
		Runtime:   validated.runtime,
		Updates:   updates,
		DiskIO:    validated.report.Metrics.Disk.BaseIO,
		DiskSmart: validated.report.Metrics.Disk.Smart,
		DiskUsage: validated.report.Metrics.Disk.Logical,
		Network:   validated.report.Metrics.Network,
	})
	if err != nil {
		logger.Error(ctx, "save metrics failed", err, kitlog.String("node", validated.report.Hostname))
		return false, httperr.ServiceUnavailable(err)
	}
	if currentUpdated && nextIP != nil {
		validated.server.IP = nextIP
		if err := h.node.SyncServerCache(context.WithoutCancel(ctx), validated.server); err != nil {
			return false, httperr.ServiceUnavailable(err)
		}
	}

	if currentUpdated {
		frontNode := metrics.BuildNodeView(validated.server, validated.report, h.staleAfterSec)
		validated.snapshot = &frontNode
		if err := h.refreshFrontSnapshot(ctx, frontNode, validated.report); err != nil {
			logger.Warn(ctx, "refresh front snapshot failed", err)
			if clearErr := h.clearFrontMeta(ctx); clearErr != nil {
				logger.Warn(ctx, "clear front snapshot meta failed", clearErr)
			}
		}
	}

	return currentUpdated, nil
}

func buildServerUpdates(server model.Server, r *http.Request) (map[string]any, *string) {
	updates := map[string]any{}
	ip, ok := nodeClientIP(r)
	if !ok {
		return updates, nil
	}

	ipStr := ip.String()
	if server.IP != nil && *server.IP == ipStr {
		return updates, nil
	}
	updates["ip"] = ipStr
	return updates, &ipStr
}

func (h *handler) writeMetricsResponse(ctx context.Context, w http.ResponseWriter, validated *validatedMetrics, logger *kitlog.Helper) {
	resp := metricsResponse{OK: true}

	manifest, err := h.updateManifest(validated)
	if err != nil {
		logger.Warn(ctx, "node update manifest unavailable", err, kitlog.Int64("server_id", validated.server.ID))
	} else {
		resp.Update = manifest
	}

	response.WriteJSON(w, http.StatusOK, resp)
}
func (h *handler) updateManifest(validated *validatedMetrics) (*updateManifest, error) {
	target, ok, err := h.node.ResolveAgentUpdate(validated.server.ID, validated.report.Version)
	if err != nil || !ok {
		return nil, err
	}

	return &updateManifest{
		ID:      target.Version,
		Version: target.Version,
		URL:     target.URL,
		SHA256:  target.SHA256,
		Size:    target.Size,
	}, nil
}

func (h *handler) refreshFrontSnapshot(ctx context.Context, node metrics.NodeView, report metrics.NodeReport) error {
	_, err := contextutil.WithTimeout(context.WithoutCancel(ctx), config.RedisWriteTimeout, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, h.front.PutNodeRuntime(ctx, node, report.Metrics.Memory.Total, report.Metrics.Memory.SwapTotal)
	})
	return err
}

func (h *handler) clearFrontMeta(ctx context.Context) error {
	_, err := contextutil.WithTimeout(context.WithoutCancel(ctx), config.RedisWriteTimeout, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, h.front.ClearFrontMeta(ctx)
	})
	return err
}
