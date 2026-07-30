package metrics

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// ValidateReport checks the semantic shape of a node metrics push body.
func ValidateReport(report NodeReport) error {
	m := report.Metrics

	if err := validateTextLength("version", report.Version, maxNodeVersionChars); err != nil {
		return err
	}
	if err := validateTextLength("hostname", report.Hostname, maxNodeHostnameChars); err != nil {
		return err
	}
	if m.Disk.Physical == nil {
		return errors.New("missing disk physical")
	}
	if m.Disk.Logical == nil {
		return errors.New("missing disk logical")
	}
	if m.Disk.Filesystems == nil {
		return errors.New("missing disk filesystems")
	}
	if m.Disk.BaseIO == nil {
		return errors.New("missing disk base_io")
	}
	if m.Network == nil {
		return errors.New("missing network")
	}
	if m.Raid.Arrays == nil {
		return errors.New("missing raid arrays")
	}

	if strings.TrimSpace(m.System.Uptime) == "" {
		return errors.New("missing system uptime")
	}
	if err := validateCPU(m.CPU); err != nil {
		return err
	}
	if err := validateMemory(m.Memory); err != nil {
		return err
	}
	if m.System.UptimeSeconds < 0 {
		return errors.New("invalid system uptime_seconds")
	}
	if m.Processes.ProcessCount < 0 {
		return errors.New("invalid process count")
	}
	if m.Connections.TCPCount < 0 || m.Connections.UDPCount < 0 {
		return errors.New("invalid connection counts")
	}

	if err := validateDiskPhysical(m.Disk.Physical); err != nil {
		return err
	}
	if err := validateDiskLogical(m.Disk.Logical); err != nil {
		return err
	}
	if err := validateDiskFilesystems(m.Disk.Filesystems); err != nil {
		return err
	}
	if err := validateDiskBaseIO(m.Disk.BaseIO); err != nil {
		return err
	}
	if err := validateNetwork(m.Network); err != nil {
		return err
	}
	if err := validateRaidArrays(m.Raid.Arrays); err != nil {
		return err
	}
	if m.Disk.Smart != nil {
		if err := validateDiskSmart(*m.Disk.Smart); err != nil {
			return err
		}
	}
	if m.Thermal != nil {
		if err := validateThermal(*m.Thermal); err != nil {
			return err
		}
	}
	if m.Pressure != nil {
		if err := validatePressure(*m.Pressure); err != nil {
			return err
		}
	}

	return nil
}

func validateCPU(item CPUMetrics) error {
	if !validRatio(item.UsageRatio) {
		return errors.New("invalid cpu usage_ratio")
	}
	if !validNonNegativeFloat(item.Load1, item.Load5, item.Load15) {
		return errors.New("invalid cpu load")
	}
	if !validNonNegativeFloat(
		item.Times.User,
		item.Times.System,
		item.Times.Idle,
		item.Times.Iowait,
		item.Times.Steal,
	) {
		return errors.New("invalid cpu times")
	}
	return nil
}

func validateMemory(item MemoryMetrics) error {
	if !validNonNegativeInt64(
		item.Total,
		item.Used,
		item.Available,
		item.Buffers,
		item.Cached,
		item.SwapTotal,
		item.SwapUsed,
		item.SwapFree,
	) {
		return errors.New("invalid memory counters")
	}
	if !validRatio(item.UsedRatio) || !validRatio(item.SwapUsedRatio) {
		return errors.New("invalid memory ratio")
	}
	return nil
}

func validateDiskPhysical(items []DiskPhysicalMetrics) error {
	seenPhysical := make(map[string]struct{}, len(items))
	for i, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("missing disk physical[%d].name", i)
		}
		name := strings.TrimSpace(item.Name)
		if err := validateTextLength(fmt.Sprintf("disk physical[%d].name", i), name, maxDiskNameChars); err != nil {
			return err
		}
		if err := validateTextLength(fmt.Sprintf("disk physical[%d].ref", i), item.Ref, maxDiskRefChars); err != nil {
			return err
		}
		if _, ok := seenPhysical[name]; ok {
			return fmt.Errorf("duplicate disk physical name: %s", name)
		}
		seenPhysical[name] = struct{}{}
		if !validNonNegativeInt64(item.ReadBytes, item.WriteBytes) {
			return fmt.Errorf("invalid disk physical[%d] counters", i)
		}
		if !validDiskIO(
			item.ReadRateBytesPerSec,
			item.WriteRateBytesPerSec,
			item.IOPS,
			item.ReadIOPS,
			item.WriteIOPS,
			item.UtilRatio,
			item.QueueLength,
			item.WaitMs,
			item.ServiceMs,
		) {
			return fmt.Errorf("invalid disk physical[%d] io", i)
		}
	}
	return nil
}

func validateDiskLogical(items []DiskLogicalMetrics) error {
	seenLogical := make(map[string]struct{}, len(items))
	for i, item := range items {
		if strings.TrimSpace(item.Kind) == "" {
			return fmt.Errorf("missing disk logical[%d].kind", i)
		}
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("missing disk logical[%d].name", i)
		}
		name := strings.TrimSpace(item.Name)
		if err := validateTextLength(fmt.Sprintf("disk logical[%d].kind", i), item.Kind, maxDiskKindChars); err != nil {
			return err
		}
		if err := validateTextLength(fmt.Sprintf("disk logical[%d].name", i), name, maxDiskNameChars); err != nil {
			return err
		}
		if err := validateTextLength(fmt.Sprintf("disk logical[%d].ref", i), item.Ref, maxDiskRefChars); err != nil {
			return err
		}
		if err := validateTextLength(fmt.Sprintf("disk logical[%d].health", i), item.Health, maxDiskStateChars); err != nil {
			return err
		}
		if err := validateTextLength(fmt.Sprintf("disk logical[%d].level", i), item.Level, maxDiskStateChars); err != nil {
			return err
		}
		if _, ok := seenLogical[name]; ok {
			return fmt.Errorf("duplicate disk logical name: %s", name)
		}
		seenLogical[name] = struct{}{}
		if !validNonNegativeInt64(item.Total, item.Used, item.Free) {
			return fmt.Errorf("invalid disk logical[%d] counters", i)
		}
		if item.Total == 0 && item.Used > math.MaxInt64-item.Free {
			return fmt.Errorf("disk logical[%d] total overflow", i)
		}
		if !validRatio(item.UsedRatio) {
			return fmt.Errorf("invalid disk logical[%d] used_ratio", i)
		}
		for mountpoint, stats := range item.Mountpoints {
			path := fmt.Sprintf("disk logical[%d] mountpoint %q fs_type", i, mountpoint)
			if err := validateTextLength(path, stats.FSType, maxFSTypeChars); err != nil {
				return err
			}
			if !validNonNegativeInt64(stats.InodesTotal, stats.InodesUsed, stats.InodesFree) ||
				!validRatio(stats.InodesUsedRatio) {
				return fmt.Errorf("invalid disk logical[%d] mountpoint %q", i, mountpoint)
			}
		}
	}
	return nil
}

func validateDiskFilesystems(items []DiskFilesystemMetrics) error {
	for i, item := range items {
		if strings.TrimSpace(item.Path) == "" {
			return fmt.Errorf("missing disk filesystems[%d].path", i)
		}
		if err := validateTextLength(fmt.Sprintf("disk filesystems[%d].fs_type", i), item.FSType, maxFSTypeChars); err != nil {
			return err
		}
		if !validNonNegativeInt64(
			item.Total,
			item.Used,
			item.Free,
			item.InodesTotal,
			item.InodesUsed,
			item.InodesFree,
		) {
			return fmt.Errorf("invalid disk filesystems[%d] counters", i)
		}
		if !validRatio(item.UsedRatio) || !validRatio(item.InodesUsedRatio) {
			return fmt.Errorf("invalid disk filesystems[%d] ratio", i)
		}
	}
	return nil
}

func validateDiskBaseIO(items []DiskBaseIOMetrics) error {
	seenBaseIO := make(map[string]struct{}, len(items))
	for i, item := range items {
		if strings.TrimSpace(item.Kind) == "" {
			return fmt.Errorf("missing disk base_io[%d].kind", i)
		}
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("missing disk base_io[%d].name", i)
		}
		name := strings.TrimSpace(item.Name)
		if err := validateTextLength(fmt.Sprintf("disk base_io[%d].kind", i), item.Kind, maxDiskKindChars); err != nil {
			return err
		}
		if err := validateTextLength(fmt.Sprintf("disk base_io[%d].name", i), name, maxDiskNameChars); err != nil {
			return err
		}
		if err := validateTextLength(fmt.Sprintf("disk base_io[%d].ref", i), item.Ref, maxDiskRefChars); err != nil {
			return err
		}
		if err := validateTextLength(fmt.Sprintf("disk base_io[%d].role", i), item.Role, maxDiskRoleChars); err != nil {
			return err
		}
		if _, ok := seenBaseIO[name]; ok {
			return fmt.Errorf("duplicate disk base_io name: %s", name)
		}
		seenBaseIO[name] = struct{}{}
		if !validNonNegativeInt64(item.ReadBytes, item.WriteBytes) {
			return fmt.Errorf("invalid disk base_io[%d] counters", i)
		}
		if !validDiskIO(
			item.ReadRateBytesPerSec,
			item.WriteRateBytesPerSec,
			item.IOPS,
			item.ReadIOPS,
			item.WriteIOPS,
			item.UtilRatio,
			item.QueueLength,
			item.WaitMs,
			item.ServiceMs,
		) {
			return fmt.Errorf("invalid disk base_io[%d] io", i)
		}
	}
	return nil
}

func validateDiskSmart(item DiskSmart) error {
	if strings.TrimSpace(item.Status) == "" {
		return errors.New("missing disk smart status")
	}
	if item.Devices == nil {
		return errors.New("missing disk smart devices")
	}
	for i, device := range item.Devices {
		if strings.TrimSpace(device.Name) == "" {
			return fmt.Errorf("missing disk smart devices[%d].name", i)
		}
		if err := validateTextLength(fmt.Sprintf("disk smart devices[%d].name", i), device.Name, maxDiskNameChars); err != nil {
			return err
		}
		if err := validateTextLength(fmt.Sprintf("disk smart devices[%d].ref", i), device.Ref, maxDiskRefChars); err != nil {
			return err
		}
		if strings.TrimSpace(device.Source) == "" {
			return fmt.Errorf("missing disk smart devices[%d].source", i)
		}
		if strings.TrimSpace(device.Status) == "" {
			return fmt.Errorf("missing disk smart devices[%d].status", i)
		}
	}
	return nil
}

func validateNetwork(items []NetIOMetrics) error {
	seenNetwork := make(map[string]struct{}, len(items))
	for i, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("missing network[%d].name", i)
		}
		name := strings.TrimSpace(item.Name)
		if err := validateTextLength(fmt.Sprintf("network[%d].name", i), name, maxInterfaceChars); err != nil {
			return err
		}
		if _, ok := seenNetwork[name]; ok {
			return fmt.Errorf("duplicate network name: %s", name)
		}
		seenNetwork[name] = struct{}{}
		if !validNonNegativeInt64(
			item.BytesRecv,
			item.BytesSent,
			item.PacketsRecv,
			item.PacketsSent,
			item.ErrIn,
			item.ErrOut,
			item.DropIn,
			item.DropOut,
		) {
			return fmt.Errorf("invalid network[%d] counters", i)
		}
		if !validNonNegativeFloat(
			item.RecvRateBytesPerSec,
			item.SentRateBytesPerSec,
			item.RecvRatePacketsPerSec,
			item.SentRatePacketsPerSec,
		) {
			return fmt.Errorf("invalid network[%d] rates", i)
		}
	}
	return nil
}

func validateThermal(item Thermal) error {
	if strings.TrimSpace(item.Status) == "" {
		return errors.New("missing thermal status")
	}
	if item.Sensors == nil {
		return errors.New("missing thermal sensors")
	}
	for i, sensor := range item.Sensors {
		if strings.TrimSpace(sensor.Kind) == "" {
			return fmt.Errorf("missing thermal sensors[%d].kind", i)
		}
		if strings.TrimSpace(sensor.Name) == "" {
			return fmt.Errorf("missing thermal sensors[%d].name", i)
		}
		if strings.TrimSpace(sensor.SensorKey) == "" {
			return fmt.Errorf("missing thermal sensors[%d].sensor_key", i)
		}
		if strings.TrimSpace(sensor.Source) == "" {
			return fmt.Errorf("missing thermal sensors[%d].source", i)
		}
		if strings.TrimSpace(sensor.Status) == "" {
			return fmt.Errorf("missing thermal sensors[%d].status", i)
		}
	}
	return nil
}

func validatePressure(item Pressure) error {
	if item.CPU != nil {
		if err := validatePressureResource("pressure.cpu", item.CPU); err != nil {
			return err
		}
	}
	if item.Memory != nil {
		if err := validatePressureResource("pressure.memory", item.Memory); err != nil {
			return err
		}
	}
	if item.IO != nil {
		if err := validatePressureResource("pressure.io", item.IO); err != nil {
			return err
		}
	}
	return nil
}

func validatePressureResource(path string, item *PressureResource) error {
	if item == nil {
		return nil
	}
	if !validPressureStats(item.Some) {
		return fmt.Errorf("invalid %s.some", path)
	}
	if !validPressureStats(item.Full) {
		return fmt.Errorf("invalid %s.full", path)
	}
	return nil
}

func validateRaidArrays(items []RaidArray) error {
	for i, arr := range items {
		if strings.TrimSpace(arr.Name) == "" {
			return fmt.Errorf("missing raid arrays[%d].name", i)
		}
		if strings.TrimSpace(arr.Status) == "" {
			return fmt.Errorf("missing raid arrays[%d].status", i)
		}
		if strings.TrimSpace(arr.Health) == "" {
			return fmt.Errorf("missing raid arrays[%d].health", i)
		}
		if err := validateTextLength(fmt.Sprintf("raid arrays[%d].health", i), arr.Health, maxRaidHealthChars); err != nil {
			return err
		}
		if arr.Members == nil {
			return fmt.Errorf("missing raid arrays[%d].members", i)
		}
		if arr.Active < 0 || arr.Working < 0 || arr.Failed < 0 {
			return fmt.Errorf("invalid raid arrays[%d] counts", i)
		}
		for j, member := range arr.Members {
			if strings.TrimSpace(member.Name) == "" {
				return fmt.Errorf("missing raid arrays[%d].members[%d].name", i, j)
			}
			if strings.TrimSpace(member.State) == "" {
				return fmt.Errorf("missing raid arrays[%d].members[%d].state", i, j)
			}
		}
	}
	return nil
}

func validNonNegativeInt64(values ...int64) bool {
	for _, value := range values {
		if value < 0 {
			return false
		}
	}
	return true
}

func validNonNegativeFloat(values ...float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return false
		}
	}
	return true
}

func validRatio(value float64) bool {
	return validNonNegativeFloat(value) && value <= 1
}

func validDiskIO(readRate, writeRate, iops, readIOPS, writeIOPS, utilRatio, queueLength, waitMS, serviceMS float64) bool {
	if !validNonNegativeFloat(readRate, writeRate, iops, readIOPS, writeIOPS, queueLength, waitMS, serviceMS) {
		return false
	}
	if !validRatio(utilRatio) {
		return false
	}
	return !math.IsInf(readIOPS+writeIOPS, 0)
}
