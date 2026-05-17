package node

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"dash/internal/infra"
	"dash/internal/metrics"
	"dash/internal/model"
	nodestore "dash/internal/store/node"
	"dash/internal/transport/http/httperr"
	"dash/internal/version"
	"github.com/Ithildur/EiluneKit/http/decoder"
	"github.com/Ithildur/EiluneKit/http/routes"
	kitlog "github.com/Ithildur/EiluneKit/logging"

	"gorm.io/gorm"
)

func (h *handler) staticRoute(r *routes.Blueprint) {
	r.Post(
		"/static",
		"Push node static metrics",
		routes.Func(h.staticHandler),
		routes.Tags("node"),
	)
}

func (h *handler) staticHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer r.Body.Close()
	logger := infra.WithModule("node")

	validated, err := h.validateStatic(ctx, r, logger)
	if err != nil {
		httperr.WriteOrInternal(w, logger, err)
		return
	}

	if err := h.saveStatic(ctx, validated.secret, validated.server.ID, validated.updates); err != nil {
		logger.Error("save static metrics failed", err)
		httperr.WriteOrInternal(w, logger, httperr.ServiceUnavailable(err))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

type validatedStatic struct {
	secret  string
	server  model.Server
	updates nodestore.ServerStaticPatch
}

func (h *handler) validateStatic(ctx context.Context, r *http.Request, logger *kitlog.Helper) (*validatedStatic, error) {
	secret, ok := readSecret(r)
	if !ok {
		return nil, httperr.Unauthorized(nil)
	}

	snapshot, err := decodeStatic(r)
	if err != nil {
		if errors.Is(err, decoder.ErrBodyTooLarge) {
			return nil, httperr.BodyTooLarge(err)
		}
		return nil, httperr.InvalidRequest(err)
	}

	if err := normalizeStatic(&snapshot); err != nil {
		return nil, err
	}

	server, err := h.loadServer(ctx, secret)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httperr.Unauthorized(err)
		}
		logger.Error("redis load server failed", err)
		return nil, httperr.ServiceUnavailable(err)
	}

	disk, hasDisk := selectLargestDisk(snapshot.Disk.Logical)
	updates := staticPatch(snapshot, disk, hasDisk, r)
	applyDisplayName(&server, snapshot, &updates)

	return &validatedStatic{
		secret:  secret,
		server:  server,
		updates: updates,
	}, nil
}

func normalizeStatic(snapshot *metrics.StaticMetrics) error {
	snapshot.Version = strings.TrimSpace(snapshot.Version)
	if snapshot.Version == "" {
		return httperr.InvalidStaticPayload(nil)
	}
	if err := version.ValidateNodeVersion(snapshot.Version); err != nil {
		return httperr.InvalidStaticPayload(err)
	}
	if snapshot.Timestamp.IsZero() || snapshot.ReportIntervalSeconds <= 0 {
		return httperr.InvalidStaticPayload(nil)
	}
	sys := &snapshot.System
	for _, field := range []*string{
		&sys.Hostname,
		&sys.OS,
		&sys.Platform,
		&sys.PlatformVersion,
		&sys.KernelVersion,
		&sys.Arch,
	} {
		if err := requireTrimmed(field); err != nil {
			return err
		}
	}
	return nil
}

func requireTrimmed(v *string) error {
	*v = strings.TrimSpace(*v)
	if *v == "" {
		return httperr.InvalidStaticPayload(nil)
	}
	return nil
}

func decodeStatic(r *http.Request) (metrics.StaticMetrics, error) {
	var snapshot metrics.StaticMetrics
	if err := decoder.DecodeJSONBody(r, &snapshot); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func (h *handler) saveStatic(ctx context.Context, secret string, serverID int64, updates nodestore.ServerStaticPatch) error {
	_, err := infra.WithPGWriteTimeout(ctx, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, h.node.UpdateStatic(ctx, secret, serverID, updates)
	})
	return err
}

func applyDisplayName(server *model.Server, snapshot metrics.StaticMetrics, updates *nodestore.ServerStaticPatch) {
	hostname := snapshot.System.Hostname
	updates.Hostname = &hostname

	displayName := strings.TrimSpace(server.Name)
	if displayName == "" || displayName == "Untitled" {
		updates.Name = &hostname
	}
}

func staticPatch(snapshot metrics.StaticMetrics, disk metrics.StaticDiskLogical, hasDisk bool, r *http.Request) nodestore.ServerStaticPatch {
	var updates nodestore.ServerStaticPatch
	sys := snapshot.System
	// /api/node/static is fed by the official node agent only.
	// The agent validates these core static fields before sending, so treat them as required here.
	osVal := sys.OS
	updates.OS = &osVal
	platformVal := sys.Platform
	updates.Platform = &platformVal
	platformVersionVal := sys.PlatformVersion
	updates.PlatformVersion = &platformVersionVal
	kernelVersionVal := sys.KernelVersion
	updates.KernelVersion = &kernelVersionVal
	archVal := sys.Arch
	updates.Arch = &archVal
	agentVersion := snapshot.Version
	updates.AgentVersion = &agentVersion

	info := snapshot.CPU.Info
	if info.ModelName != "" {
		val := info.ModelName
		updates.CPUModel = &val
	}
	if info.VendorID != "" {
		val := info.VendorID
		updates.CPUVendor = &val
	}

	coresPhys := int16(info.CoresPhysical)
	updates.CPUCoresPhys = &coresPhys
	coresLog := int16(info.CoresLogical)
	updates.CPUCoresLog = &coresLog
	sockets := int16(info.Sockets)
	updates.CPUSockets = &sockets
	if info.FrequencyMhz > 0 {
		v := info.FrequencyMhz
		updates.CPUMhz = &v
	}

	memTotal := int64(snapshot.Memory.Total)
	updates.MemTotal = &memTotal
	if snapshot.Memory.SwapTotal > 0 {
		v := int64(snapshot.Memory.SwapTotal)
		updates.SwapTotal = &v
	}

	intervalSec := int32(snapshot.ReportIntervalSeconds)
	updates.IntervalSec = &intervalSec

	if hasDisk {
		v := int64(disk.Total)
		updates.DiskTotal = &v
		label := strings.TrimSpace(disk.Mountpoint)
		if label == "" {
			label = strings.TrimSpace(disk.Ref)
		}
		if label != "" {
			updates.RootPath = &label
		}
		fsType := mountpointFSType(disk)
		if fsType != "" {
			updates.RootFSType = &fsType
		} else {
			empty := ""
			updates.RootFSType = &empty
		}
	}

	raidSupported := snapshot.Raid.Supported
	raidAvailable := snapshot.Raid.Available
	updates.RaidSupported = &raidSupported
	updates.RaidAvailable = &raidAvailable

	if ip, ok := nodeClientIP(r); ok {
		ipStr := ip.String()
		updates.IP = &ipStr
	}

	return updates
}

func selectLargestDisk(items []metrics.StaticDiskLogical) (metrics.StaticDiskLogical, bool) {
	if len(items) == 0 {
		return metrics.StaticDiskLogical{}, false
	}
	best := items[0]
	for _, item := range items[1:] {
		if item.Total > best.Total {
			best = item
		}
	}
	return best, true
}

func mountpointFSType(item metrics.StaticDiskLogical) string {
	mountpoint := strings.TrimSpace(item.Mountpoint)
	if mountpoint == "" || len(item.Mountpoints) == 0 {
		return ""
	}
	if info, ok := item.Mountpoints[mountpoint]; ok {
		return strings.TrimSpace(info.FSType)
	}
	return ""
}
