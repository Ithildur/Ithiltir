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

	secret, server, err := h.authenticate(ctx, r, logger)
	if err != nil {
		h.writeError(w, r, logger, err)
		return
	}
	validated, err := validateStatic(r, secret, server)
	if err != nil {
		h.writeError(w, r, logger, err)
		return
	}

	if err := h.saveStatic(ctx, validated.secret, validated.server.ID, validated.patch); err != nil {
		logger.Error("save static metrics failed", err)
		h.writeError(w, r, logger, httperr.ServiceUnavailable(err))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

type validatedStatic struct {
	secret string
	server model.Server
	patch  nodestore.ServerStaticPatch
}

func validateStatic(r *http.Request, secret string, server model.Server) (*validatedStatic, error) {
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

	disk, hasDisk := selectLargestDisk(snapshot.Disk.Logical)
	patch := staticPatch(snapshot, disk, hasDisk, r)

	return &validatedStatic{
		secret: secret,
		server: server,
		patch:  patch,
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
	if err := validateStaticNumbers(*snapshot); err != nil {
		return httperr.InvalidStaticPayload(err)
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
	if err := metrics.ValidateStaticText(*snapshot); err != nil {
		return httperr.InvalidStaticPayload(err)
	}
	return nil
}

func validateStaticNumbers(snapshot metrics.StaticMetrics) error {
	info := snapshot.CPU.Info
	if info.Sockets < 0 || info.CoresPhysical < 0 || info.CoresLogical < 0 {
		return errors.New("negative cpu topology")
	}
	if snapshot.Memory.Total < 0 || (snapshot.Memory.SwapTotal != nil && *snapshot.Memory.SwapTotal < 0) {
		return errors.New("negative memory capacity")
	}
	for _, disk := range snapshot.Disk.Logical {
		if disk.Total < 0 || disk.Used < 0 || disk.Free < 0 {
			return errors.New("negative logical disk capacity")
		}
		for _, mountpoint := range disk.Mountpoints {
			if mountpoint.InodesTotal < 0 || mountpoint.InodesUsed < 0 || mountpoint.InodesFree < 0 {
				return errors.New("negative logical disk inode count")
			}
		}
	}
	for _, filesystem := range snapshot.Disk.Filesystems {
		if filesystem.Total < 0 || filesystem.InodesTotal < 0 {
			return errors.New("negative filesystem capacity")
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

func (h *handler) saveStatic(ctx context.Context, secret string, serverID int64, patch nodestore.ServerStaticPatch) error {
	_, err := infra.WithPGWriteTimeout(ctx, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, h.node.UpdateStatic(ctx, secret, serverID, patch)
	})
	return err
}

func staticPatch(snapshot metrics.StaticMetrics, disk metrics.StaticDiskLogical, hasDisk bool, r *http.Request) nodestore.ServerStaticPatch {
	sys := snapshot.System
	info := snapshot.CPU.Info
	patch := nodestore.ServerStaticPatch{
		Name:            &sys.Hostname,
		Hostname:        &sys.Hostname,
		OS:              &sys.OS,
		Platform:        &sys.Platform,
		PlatformVersion: &sys.PlatformVersion,
		KernelVersion:   &sys.KernelVersion,
		Arch:            &sys.Arch,
		AgentVersion:    &snapshot.Version,
		RaidSupported:   &snapshot.Raid.Supported,
		RaidAvailable:   &snapshot.Raid.Available,
	}
	patch.IntervalSec = &snapshot.ReportIntervalSeconds

	// CPU identity/topology and physical memory are last-known observations.
	// Zero or empty means the collector could not resolve the field, so it must
	// not erase a valid stored value.
	if v := strings.TrimSpace(info.ModelName); v != "" {
		patch.CPUModel = &v
	}
	if v := strings.TrimSpace(info.VendorID); v != "" {
		patch.CPUVendor = &v
	}
	if info.CoresPhysical > 0 {
		patch.CPUCoresPhys = &info.CoresPhysical
	}
	if info.CoresLogical > 0 {
		patch.CPUCoresLog = &info.CoresLogical
	}
	if info.Sockets > 0 {
		patch.CPUSockets = &info.Sockets
	}
	if info.FrequencyMhz > 0 {
		patch.CPUMhz = &info.FrequencyMhz
	}
	if snapshot.Memory.Total > 0 {
		patch.MemTotal = &snapshot.Memory.Total
	}
	// Unlike unavailable hardware data, zero swap is an observed state: it means
	// swap was disabled and must clear any previously stored positive capacity.
	if snapshot.Memory.SwapTotal != nil {
		patch.SwapTotal = snapshot.Memory.SwapTotal
	}

	if hasDisk {
		label := strings.TrimSpace(disk.Mountpoint)
		if label == "" {
			label = strings.TrimSpace(disk.Ref)
		}
		if label != "" {
			patch.Disk = &nodestore.DiskObservation{
				Path:   label,
				FSType: mountpointFSType(disk),
				Total:  disk.Total,
			}
		}
	}

	if ip, ok := nodeClientIP(r); ok {
		ipStr := ip.String()
		patch.IP = &ipStr
	}

	return patch
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
