// Package nodeingest owns transport-independent node report acceptance.
package nodeingest

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"time"

	"github.com/Ithildur/EiluneKit/contextutil"
	kitlog "github.com/Ithildur/EiluneKit/logging"

	"dash/internal/config"
	"dash/internal/infra"
	"dash/internal/metrics"
	"dash/internal/model"
	"dash/internal/serverid"
	alertstore "dash/internal/store/alert"
	"dash/internal/store/frontcache"
	"dash/internal/store/metricdata"
	nodestore "dash/internal/store/node"
	"dash/internal/version"
)

var errMetricsIdentityChanged = errors.New("node identity changed while acquiring the projection lock")

const metricsIdentityAttempts = 3

type Receiver struct {
	node          *nodestore.Store
	metric        *metricdata.Store
	front         *frontcache.Store
	alert         *alertstore.Store
	serverID      *serverid.Store
	staleAfterSec int
}

func New(node *nodestore.Store, metric *metricdata.Store, front *frontcache.Store, alert *alertstore.Store, serverID *serverid.Store, staleAfterSec int) *Receiver {
	return &Receiver{
		node:          node,
		metric:        metric,
		front:         front,
		alert:         alert,
		serverID:      serverID,
		staleAfterSec: staleAfterSec,
	}
}

func (h *Receiver) Metrics(ctx context.Context, secret string, server model.Server, report metrics.NodeReport, receivedAt time.Time, ip netip.Addr) (Response, error) {
	logger := infra.WithModule("node")
	in, err := metricsReport(report, receivedAt, secret)
	if err != nil {
		return Response{}, err
	}
	var validated *validatedMetrics
	for attempt := range metricsIdentityAttempts {
		if err != nil {
			break
		}
		lockedID := server.ID
		err = h.node.WithMetricsIngest(lockedID, func() error {
			current, authErr := h.Authenticate(ctx, in.secret)
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
			currentUpdated, err = h.persistMetrics(ctx, validated, ip, logger)
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
		err = unavailable(err)
	}
	if err != nil {
		return Response{}, err
	}

	return h.metricsResponse(ctx, validated, logger), nil
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

type Response struct {
	OK     bool            `json:"ok"`
	Update *UpdateManifest `json:"update"`
}

type UpdateManifest struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

func metricsReport(report metrics.NodeReport, receivedAt time.Time, secret string) (*metricsInput, error) {
	report.Version = strings.TrimSpace(report.Version)
	if report.Version == "" {
		return nil, invalidMetrics(nil)
	}
	if err := version.ValidateNodeVersion(report.Version); err != nil {
		return nil, invalidMetrics(err)
	}
	if strings.TrimSpace(report.Hostname) == "" {
		return nil, invalidMetrics(nil)
	}
	if report.Timestamp.IsZero() {
		return nil, invalidMetrics(nil)
	}
	if err := metrics.ValidateReport(report); err != nil {
		return nil, invalidMetrics(err)
	}
	return &metricsInput{secret: secret, report: report, receivedAt: receivedAt}, nil
}

func validateMetrics(in *metricsInput, server model.Server) (*validatedMetrics, error) {
	report, reportedAtRaw := metrics.NormalizeReport(server.ID, server.DisplayOrder, in.report, in.receivedAt)
	metric, runtime, err := metrics.BuildMetric(server.ID, report.Metrics, in.receivedAt, reportedAtRaw)
	if err != nil {
		return nil, invalidMetrics(err)
	}

	return &validatedMetrics{
		server:  server,
		report:  report,
		metric:  metric,
		runtime: runtime,
	}, nil
}

func (h *Receiver) persistMetrics(ctx context.Context, validated *validatedMetrics, ip netip.Addr, logger *kitlog.Helper) (bool, error) {
	updates, nextIP := buildServerUpdates(validated.server, ip)

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
		return false, unavailable(err)
	}
	if currentUpdated && nextIP != nil {
		validated.server.IP = nextIP
		if err := h.node.SyncServerCache(context.WithoutCancel(ctx), validated.server); err != nil {
			return false, unavailable(err)
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

func buildServerUpdates(server model.Server, ip netip.Addr) (map[string]any, *string) {
	updates := map[string]any{}
	if !ip.IsValid() {
		return updates, nil
	}

	ipStr := ip.String()
	if server.IP != nil && *server.IP == ipStr {
		return updates, nil
	}
	updates["ip"] = ipStr
	return updates, &ipStr
}

func (h *Receiver) metricsResponse(ctx context.Context, validated *validatedMetrics, logger *kitlog.Helper) Response {
	resp := Response{OK: true}

	manifest, err := h.updateManifest(validated)
	if err != nil {
		logger.Warn(ctx, "node update manifest unavailable", err, kitlog.Int64("server_id", validated.server.ID))
	} else {
		resp.Update = manifest
	}

	return resp
}
func (h *Receiver) updateManifest(validated *validatedMetrics) (*UpdateManifest, error) {
	target, ok, err := h.node.ResolveAgentUpdate(validated.server.ID, validated.report.Version)
	if err != nil || !ok {
		return nil, err
	}

	return &UpdateManifest{
		ID:      target.Version,
		Version: target.Version,
		URL:     target.URL,
		SHA256:  target.SHA256,
		Size:    target.Size,
	}, nil
}

func (h *Receiver) refreshFrontSnapshot(ctx context.Context, node metrics.NodeView, report metrics.NodeReport) error {
	_, err := contextutil.WithTimeout(context.WithoutCancel(ctx), config.RedisWriteTimeout, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, h.front.PutNodeRuntime(ctx, node, report.Metrics.Memory.Total, report.Metrics.Memory.SwapTotal)
	})
	return err
}

func (h *Receiver) clearFrontMeta(ctx context.Context) error {
	_, err := contextutil.WithTimeout(context.WithoutCancel(ctx), config.RedisWriteTimeout, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, h.front.ClearFrontMeta(ctx)
	})
	return err
}
