package nodeingest

import (
	"context"
	"errors"
	"net/netip"
	"strings"

	"dash/internal/infra"
	"dash/internal/metrics"
	nodestore "dash/internal/store/node"
	"dash/internal/version"
)

func (h *Receiver) Static(ctx context.Context, secret string, serverID int64, snapshot metrics.StaticMetrics, ip netip.Addr) error {
	if err := normalizeStatic(&snapshot); err != nil {
		return err
	}
	disk, hasDisk := selectLargestDisk(snapshot.Disk.Logical)
	patch := staticPatch(snapshot, disk, hasDisk, ip)
	if err := h.saveStatic(ctx, secret, serverID, patch); err != nil {
		infra.WithModule("node").Error(ctx, "save static metrics failed", err)
		return unavailable(err)
	}
	return nil
}
func normalizeStatic(snapshot *metrics.StaticMetrics) error {
	snapshot.Version = strings.TrimSpace(snapshot.Version)
	if snapshot.Version == "" {
		return invalidStatic(nil)
	}
	if err := version.ValidateNodeVersion(snapshot.Version); err != nil {
		return invalidStatic(err)
	}
	if snapshot.Timestamp.IsZero() || snapshot.ReportIntervalSeconds <= 0 {
		return invalidStatic(nil)
	}
	if err := validateStaticNumbers(*snapshot); err != nil {
		return invalidStatic(err)
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
		return invalidStatic(err)
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
		return invalidStatic(nil)
	}
	return nil
}

func (h *Receiver) saveStatic(ctx context.Context, secret string, serverID int64, patch nodestore.ServerStaticPatch) error {
	_, err := infra.WithPGWriteTimeout(ctx, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, h.node.UpdateStatic(ctx, secret, serverID, patch)
	})
	return err
}

func staticPatch(snapshot metrics.StaticMetrics, disk metrics.StaticDiskLogical, hasDisk bool, ip netip.Addr) nodestore.ServerStaticPatch {
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

	if ip.IsValid() {
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
