package node

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"dash/internal/config"
	"dash/internal/infra"
	"dash/internal/metrics"
	"dash/internal/model"
	"dash/internal/serverid"
	alertstore "dash/internal/store/alert"
	"dash/internal/store/frontcache"
	"dash/internal/store/metricdata"
	nodestore "dash/internal/store/node"
	"dash/internal/transport/http/httperr"
	"dash/internal/version"
	"github.com/Ithildur/EiluneKit/contextutil"
	"github.com/Ithildur/EiluneKit/http/decoder"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
	kitlog "github.com/Ithildur/EiluneKit/logging"

	"gorm.io/gorm"
)

type handler struct {
	node          *nodestore.Store
	metric        *metricdata.Store
	front         *frontcache.Store
	alert         *alertstore.Store
	serverID      *serverid.Store
	staleAfterSec int
}

func newHandler(node *nodestore.Store, metric *metricdata.Store, front *frontcache.Store, alert *alertstore.Store, serverID *serverid.Store, staleAfterSec int) *handler {
	return &handler{
		node:          node,
		metric:        metric,
		front:         front,
		alert:         alert,
		serverID:      serverID,
		staleAfterSec: staleAfterSec,
	}
}

func (h *handler) metricsRoute(r *routes.Blueprint) {
	r.Post(
		"/metrics",
		"Push node metrics",
		routes.Func(h.metricsHandler),
		routes.Tags("node"),
	)
}

// Hot path for node ingest; keep behavior stable to avoid silent failures.
func (h *handler) metricsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer r.Body.Close()
	receivedAt := time.Now().UTC()
	logger := infra.WithModule("node")

	validated, err := h.validateMetrics(ctx, r, receivedAt, logger)
	if err != nil {
		httperr.WriteOrInternal(w, logger, err)
		return
	}

	if err := h.persistMetrics(ctx, validated, r, logger); err != nil {
		httperr.WriteOrInternal(w, logger, err)
		return
	}

	h.writeMetricsResponse(ctx, w, validated, logger)
}

type validatedMetrics struct {
	server     model.Server
	report     metrics.NodeReport
	metric     model.ServerMetric
	receivedAt time.Time
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

func readSecret(r *http.Request) (string, bool) {
	secret := r.Header.Get("X-Node-Secret")
	if secret == "" {
		return "", false
	}
	return secret, true
}

func (h *handler) validateMetrics(ctx context.Context, r *http.Request, receivedAt time.Time, logger *kitlog.Helper) (*validatedMetrics, error) {
	secret, ok := readSecret(r)
	if !ok {
		return nil, httperr.Unauthorized(nil)
	}

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

	server, err := h.loadServer(ctx, secret)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httperr.Unauthorized(err)
		}
		logger.Error("redis load server failed", err)
		return nil, httperr.ServiceUnavailable(err)
	}
	report, reportedAtRaw := metrics.NormalizeReport(server.ID, server.DisplayOrder, report, receivedAt)
	metric, err := metrics.BuildMetric(server.ID, report.Metrics, receivedAt, reportedAtRaw)
	if err != nil {
		return nil, httperr.InvalidMetrics(err)
	}

	return &validatedMetrics{
		server:     server,
		report:     report,
		metric:     metric,
		receivedAt: receivedAt,
	}, nil
}

func decodeReport(r *http.Request) (metrics.NodeReport, error) {
	var report metrics.NodeReport
	if err := decoder.DecodeJSONBody(r, &report); err != nil {
		return report, err
	}
	return report, nil
}

func (h *handler) persistMetrics(ctx context.Context, validated *validatedMetrics, r *http.Request, logger *kitlog.Helper) error {
	updates := h.buildRuntimeUpdates(ctx, validated.server.ID, validated.receivedAt, r)

	// Agent disk.physical is validated but not persisted; base_io feeds disk IO history.
	if err := h.saveMetrics(ctx, metricdata.MetricsSample{
		ServerID:  validated.server.ID,
		Metric:    validated.metric,
		Updates:   updates,
		DiskIO:    validated.report.Metrics.Disk.BaseIO,
		DiskSmart: validated.report.Metrics.Disk.Smart,
		DiskUsage: validated.report.Metrics.Disk.Logical,
		Network:   validated.report.Metrics.Network,
	}); err != nil {
		logger.Error("save metrics failed", err, kitlog.String("node", validated.report.Hostname))
		return httperr.ServiceUnavailable(err)
	}

	if err := h.refreshFrontSnapshot(ctx, validated.server, validated.report); err != nil {
		logger.Warn("refresh front snapshot failed", err)
		if clearErr := h.front.ClearFrontMeta(ctx); clearErr != nil {
			logger.Warn("clear front snapshot meta failed", clearErr)
		}
	}
	if err := h.markAlertDirty(ctx, validated.server.ID); err != nil {
		logger.Error("mark alert dirty failed", err, kitlog.Int64("server_id", validated.server.ID))
		return httperr.ServiceUnavailable(err)
	}

	return nil
}

func (h *handler) writeMetricsResponse(ctx context.Context, w http.ResponseWriter, validated *validatedMetrics, logger *kitlog.Helper) {
	resp := metricsResponse{OK: true}

	manifest, err := h.updateManifest(ctx, validated)
	if err != nil {
		logger.Warn("node update manifest unavailable", err, kitlog.Int64("server_id", validated.server.ID))
	} else {
		resp.Update = manifest
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *handler) updateManifest(ctx context.Context, validated *validatedMetrics) (*updateManifest, error) {
	type resolved struct {
		target nodestore.AgentUpdateTarget
		ok     bool
	}
	got, err := infra.WithPGWriteTimeout(ctx, func(c context.Context) (resolved, error) {
		target, ok, err := h.node.ResolveAgentUpdate(c, validated.server.ID, validated.report.Version)
		return resolved{target: target, ok: ok}, err
	})
	if err != nil || !got.ok {
		return nil, err
	}

	return &updateManifest{
		ID:      got.target.Version,
		Version: got.target.Version,
		URL:     got.target.URL,
		SHA256:  got.target.SHA256,
		Size:    got.target.Size,
	}, nil
}

func (h *handler) loadServer(ctx context.Context, secret string) (model.Server, error) {
	return infra.WithPGReadTimeout(ctx, func(ctx context.Context) (model.Server, error) {
		return h.node.GetServerBySecret(ctx, secret)
	})
}

func (h *handler) saveMetrics(ctx context.Context, sample metricdata.MetricsSample) error {
	_, err := infra.WithPGWriteTimeout(ctx, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, h.metric.SaveMetrics(ctx, sample)
	})
	return err
}

func (h *handler) refreshFrontSnapshot(ctx context.Context, server model.Server, report metrics.NodeReport) error {
	_, err := contextutil.WithTimeout(ctx, config.RedisWriteTimeout, func(ctx context.Context) (struct{}, error) {
		frontNode, err := metrics.BuildNodeView(server, report, h.staleAfterSec)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, h.front.PutNodeSnapshot(ctx, frontNode)
	})
	return err
}

func (h *handler) markAlertDirty(ctx context.Context, serverID int64) error {
	_, err := contextutil.WithTimeout(ctx, config.RedisWriteTimeout, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, h.alert.MarkServerDirty(ctx, serverID)
	})
	return err
}

func (h *handler) buildRuntimeUpdates(ctx context.Context, serverID int64, receivedAt time.Time, r *http.Request) map[string]any {
	updates := map[string]any{}

	ip, ok := nodeClientIP(r)
	if !ok {
		return updates
	}

	ipStr := ip.String()

	cached := h.getIP(ctx, serverID)
	if cached == "" || cached != ipStr {
		updates["ip"] = ipStr
	}

	if err := h.node.SetServerRuntime(ctx, serverID, ipStr, receivedAt); err != nil {
		infra.WithModule("node").Warn("cache runtime write failed", err)
	}

	return updates
}

func (h *handler) getIP(ctx context.Context, serverID int64) string {
	logger := infra.WithModule("node")

	cached, hit, err := h.node.GetServerRuntimeIP(ctx, serverID)
	if err != nil {
		logger.Error("cache runtime ip read failed", err)
	}
	if hit {
		return cached
	}
	return ""
}
