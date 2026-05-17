package metrics

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"dash/internal/model"
	"dash/internal/nodetags"
)

// NodeView is the sanitized metrics report for frontend consumption.
// It intentionally omits "alive" and "hostname"; the frontend derives online status from received_at + stale_after_sec.
type NodeView struct {
	Node        NodeMeta    `json:"node"`
	Observation Observation `json:"observation"`
	System      System      `json:"system"`
	CPU         CPU         `json:"cpu"`
	Memory      Memory      `json:"memory"`
	Disk        Disk        `json:"disk"`
	Network     Network     `json:"network"`
	Processes   Processes   `json:"processes"`
	Connections Connections `json:"connections"`
	Raid        *RAID       `json:"raid,omitempty"`
	Thermal     *Thermal    `json:"thermal,omitempty"`
}

type NodeMeta struct {
	ID         string   `json:"id"`
	Order      int      `json:"order,omitempty"`
	Title      string   `json:"title"`
	SearchText []string `json:"search_text,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

type Observation struct {
	ReceivedAt    string `json:"received_at,omitempty"`
	ObservedAt    string `json:"observed_at"`
	SentAt        string `json:"sent_at,omitempty"`
	StaleAfterSec int    `json:"stale_after_sec"`
}

type System struct {
	OSFamily        string `json:"os_family"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	KernelVersion   string `json:"kernel_version"`
	Arch            string `json:"arch,omitempty"`
	UptimeText      string `json:"uptime_text"`
}

type CPU struct {
	UsageRatio    float64 `json:"usage_ratio"`
	Load          CPULoad `json:"load"`
	ModelName     string  `json:"model_name,omitempty"`
	CoresPhysical int     `json:"cores_physical,omitempty"`
	CoresLogical  int     `json:"cores_logical,omitempty"`
	Sockets       int     `json:"sockets,omitempty"`
}

type CPULoad struct {
	L1  float64 `json:"l1"`
	L5  float64 `json:"l5"`
	L15 float64 `json:"l15"`
}

type Memory struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	BuffersBytes   uint64  `json:"buffers_bytes"`
	CachedBytes    uint64  `json:"cached_bytes"`
	UsedRatio      float64 `json:"used_ratio"`
	SwapTotalBytes uint64  `json:"swap_total_bytes"`
	SwapUsedBytes  uint64  `json:"swap_used_bytes"`
}

type Disk struct {
	Mounts             []DiskMount `json:"mounts"`
	IO                 DiskIO      `json:"io"`
	TemperatureDevices []string    `json:"temperature_devices,omitempty"`
	Smart              *DiskSmart  `json:"smart,omitempty"`
}

type DiskMount struct {
	Mountpoint string  `json:"mountpoint"`
	FSType     string  `json:"fs_type"`
	TotalBytes uint64  `json:"total_bytes"`
	UsedBytes  uint64  `json:"used_bytes"`
	UsedRatio  float64 `json:"used_ratio"`
}

type DiskIO struct {
	Total    IOTotal            `json:"total"`
	ByDevice map[string]IOTotal `json:"by_device,omitempty"`
}

type Network struct {
	Total       NetTotal       `json:"total"`
	ByInterface []NetInterface `json:"by_interface,omitempty"`
}

type NetTotal struct {
	BytesRecv uint64  `json:"bytes_recv"`
	BytesSent uint64  `json:"bytes_sent"`
	RecvBPS   float64 `json:"recv_bps"`
	SentBPS   float64 `json:"sent_bps"`
}

type NetInterface struct {
	Name      string  `json:"name"`
	BytesRecv uint64  `json:"bytes_recv"`
	BytesSent uint64  `json:"bytes_sent"`
	RecvBPS   float64 `json:"recv_bps"`
	SentBPS   float64 `json:"sent_bps"`
}

type Processes struct {
	Count int `json:"count"`
}

type Connections struct {
	TCP int `json:"tcp"`
	UDP int `json:"udp"`
}

type RAID struct {
	Supported bool        `json:"supported"`
	Available bool        `json:"available"`
	Arrays    []RAIDArray `json:"arrays"`
}

type RAIDArray struct {
	Name         string       `json:"name"`
	Status       string       `json:"status"`
	Active       int          `json:"active"`
	Working      int          `json:"working"`
	Failed       int          `json:"failed"`
	Health       string       `json:"health"`
	Members      []RaidMember `json:"members"`
	SyncStatus   string       `json:"sync_status,omitempty"`
	SyncProgress string       `json:"sync_progress,omitempty"`
}

func ParseNodeID(id string) (int64, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func DurationSecondsCeil(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int(math.Ceil(d.Seconds()))
}

func BuildNodeView(server model.Server, report NodeReport, staleAfterSec int) (NodeView, error) {
	sys := report.Metrics.System
	tags, err := nodetags.Parse(server.Tags)
	if err != nil {
		return NodeView{}, fmt.Errorf("server %d tags: %w", server.ID, err)
	}

	node := NodeMeta{
		ID:    strconv.FormatInt(server.ID, 10),
		Order: server.DisplayOrder,
		Title: strings.TrimSpace(server.Name),
		Tags:  tags,
	}
	searchCandidates := append([]string{
		node.Title,
		stringOrEmpty(server.OS),
		stringOrEmpty(server.Platform),
		stringOrEmpty(server.Arch),
	}, tags...)
	node.SearchText = buildSearchText(searchCandidates...)

	obs := Observation{
		ReceivedAt:    FormatTimestamp(report.Timestamp),
		ObservedAt:    FormatTimestamp(report.Timestamp),
		SentAt:        report.SentAt,
		StaleAfterSec: staleAfterSec,
	}
	if parsedObserved := ParseReportedAt(report.SentAt); parsedObserved != nil {
		obs.ObservedAt = FormatTimestamp(parsedObserved.UTC())
	}

	system := System{
		OSFamily:        normalizeOSFamily(stringOrEmpty(server.OS)),
		Platform:        stringOrEmpty(server.Platform),
		PlatformVersion: stringOrEmpty(server.PlatformVersion),
		KernelVersion:   stringOrEmpty(server.KernelVersion),
		Arch:            stringOrEmpty(server.Arch),
		UptimeText:      sys.Uptime,
	}

	cpu := CPU{
		UsageRatio: report.Metrics.CPU.UsageRatio,
		Load: CPULoad{
			L1:  report.Metrics.CPU.Load1,
			L5:  report.Metrics.CPU.Load5,
			L15: report.Metrics.CPU.Load15,
		},
	}
	if modelName := strings.TrimSpace(stringOrEmpty(server.CPUModel)); modelName != "" {
		cpu.ModelName = modelName
	}
	if cores := int(int16OrZero(server.CPUCoresPhys)); cores > 0 {
		cpu.CoresPhysical = cores
	}
	if cores := int(int16OrZero(server.CPUCoresLog)); cores > 0 {
		cpu.CoresLogical = cores
	}
	sockets := int(int16OrZero(server.CPUSockets))
	if sockets <= 0 {
		sockets = 1
	}
	cpu.Sockets = sockets

	mem := report.Metrics.Memory
	memTotal := mem.Total
	if memTotal == 0 && server.MemTotal != nil && *server.MemTotal > 0 {
		memTotal = uint64(*server.MemTotal)
	}
	swapTotal := mem.SwapTotal
	if swapTotal == 0 && server.SwapTotal != nil && *server.SwapTotal > 0 {
		swapTotal = uint64(*server.SwapTotal)
	}
	memory := Memory{
		TotalBytes:     memTotal,
		UsedBytes:      mem.Used,
		AvailableBytes: mem.Available,
		BuffersBytes:   mem.Buffers,
		CachedBytes:    mem.Cached,
		UsedRatio:      mem.UsedRatio,
		SwapTotalBytes: swapTotal,
		SwapUsedBytes:  mem.SwapUsed,
	}

	processes := Processes{Count: report.Metrics.Processes.ProcessCount}
	connections := Connections{TCP: report.Metrics.Connections.TCPCount, UDP: report.Metrics.Connections.UDPCount}

	out := NodeView{
		Node:        node,
		Observation: obs,
		System:      system,
		CPU:         cpu,
		Memory:      memory,
		Disk:        buildDisk(report.Metrics),
		Network:     buildNetwork(report.Metrics.Network),
		Processes:   processes,
		Connections: connections,
		Thermal:     normalizeThermal(report.Metrics.Thermal),
	}

	if raid := report.Metrics.Raid; raid.Supported || raid.Available || len(raid.Arrays) > 0 {
		arrays := make([]RAIDArray, 0, len(raid.Arrays))
		for _, array := range raid.Arrays {
			arrays = append(arrays, RAIDArray{
				Name:         array.Name,
				Status:       array.Status,
				Active:       array.Active,
				Working:      array.Working,
				Failed:       array.Failed,
				Health:       array.Health,
				Members:      array.Members,
				SyncStatus:   array.SyncStatus,
				SyncProgress: array.SyncProgress,
			})
		}
		out.Raid = &RAID{
			Supported: raid.Supported,
			Available: raid.Available,
			Arrays:    arrays,
		}
	}

	return out, nil
}

func buildSearchText(candidates ...string) []string {
	terms := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		terms = append(terms, candidate)
	}
	if len(terms) == 0 {
		return nil
	}
	return terms
}

func normalizeOSFamily(rawOS string) string {
	normalized := strings.ToLower(strings.TrimSpace(rawOS))
	switch {
	case strings.Contains(normalized, "windows"):
		return "windows"
	case strings.Contains(normalized, "linux"):
		return "linux"
	case strings.Contains(normalized, "darwin"), strings.Contains(normalized, "mac"):
		return "darwin"
	default:
		return "unknown"
	}
}

func buildDisk(metric Metrics) Disk {
	mounts := make([]DiskMount, 0, len(metric.Disk.Logical))
	for _, logical := range metric.Disk.Logical {
		selectedMount := strings.TrimSpace(logical.Mountpoint)
		label := resolveMountpoint(selectedMount, logical.Ref)
		if label == "" {
			label = strings.TrimSpace(logical.Name)
		}
		if label == "" {
			continue
		}
		fsType := mountpointFSType(logical, selectedMount)
		total := logical.Total
		if total == 0 {
			total = logical.Used + logical.Free
		}
		mounts = append(mounts, DiskMount{
			Mountpoint: label,
			FSType:     fsType,
			TotalBytes: total,
			UsedBytes:  logical.Used,
			UsedRatio:  logical.UsedRatio,
		})
	}
	sort.SliceStable(mounts, func(i, j int) bool {
		if mounts[i].TotalBytes == mounts[j].TotalBytes {
			return mounts[i].Mountpoint < mounts[j].Mountpoint
		}
		return mounts[i].TotalBytes > mounts[j].TotalBytes
	})

	var total IOTotal
	var byDevice map[string]IOTotal
	for _, ioMetric := range metric.Disk.BaseIO {
		ioTotal := IOTotal{
			ReadBPS:   ioMetric.ReadRateBytesPerSec,
			WriteBPS:  ioMetric.WriteRateBytesPerSec,
			ReadIOPS:  ioMetric.ReadIOPS,
			WriteIOPS: ioMetric.WriteIOPS,
			IOPS:      ioMetric.IOPS,
		}
		if ioTotal.IOPS == 0 {
			ioTotal.IOPS = ioTotal.ReadIOPS + ioTotal.WriteIOPS
		}
		device := strings.TrimSpace(ioMetric.Ref)
		if device == "" {
			device = strings.TrimSpace(ioMetric.Name)
		}
		if device != "" {
			if byDevice == nil {
				byDevice = make(map[string]IOTotal, len(metric.Disk.BaseIO))
			}
			byDevice[device] = ioTotal
		}
		if ioMetric.Role == "primary" {
			total.ReadBPS += ioTotal.ReadBPS
			total.WriteBPS += ioTotal.WriteBPS
			total.ReadIOPS += ioTotal.ReadIOPS
			total.WriteIOPS += ioTotal.WriteIOPS
			total.IOPS += ioTotal.IOPS
		}
	}

	return Disk{
		Mounts: mounts,
		IO: DiskIO{
			Total:    total,
			ByDevice: byDevice,
		},
		TemperatureDevices: SmartTemperatureDeviceNames(metric.Disk.Smart),
		Smart:              normalizeDiskSmart(metric.Disk.Smart),
	}
}

type IOTotal struct {
	ReadBPS   float64 `json:"read_bps"`
	WriteBPS  float64 `json:"write_bps"`
	ReadIOPS  float64 `json:"read_iops"`
	WriteIOPS float64 `json:"write_iops"`
	IOPS      float64 `json:"iops"`
}

func resolveMountpoint(mountpoint, path string) string {
	mountpoint = strings.TrimSpace(mountpoint)
	if mountpoint != "" {
		return mountpoint
	}
	path = strings.TrimSpace(path)
	if path != "" {
		return path
	}
	return ""
}

func mountpointFSType(item DiskLogicalMetrics, mountpoint string) string {
	if mountpoint == "" || len(item.Mountpoints) == 0 {
		return ""
	}
	mp, ok := item.Mountpoints[mountpoint]
	if !ok {
		return ""
	}
	return strings.TrimSpace(mp.FSType)
}

func buildNetwork(interfaces []NetIOMetrics) Network {
	var total NetTotal
	var byInterface []NetInterface
	for _, iface := range interfaces {
		total.BytesRecv += iface.BytesRecv
		total.BytesSent += iface.BytesSent
		total.RecvBPS += iface.RecvRateBytesPerSec
		total.SentBPS += iface.SentRateBytesPerSec
		name := strings.TrimSpace(iface.Name)
		if name != "" {
			byInterface = append(byInterface, NetInterface{
				Name:      name,
				BytesRecv: iface.BytesRecv,
				BytesSent: iface.BytesSent,
				RecvBPS:   iface.RecvRateBytesPerSec,
				SentBPS:   iface.SentRateBytesPerSec,
			})
		}
	}
	if len(byInterface) == 0 {
		byInterface = nil
	}
	return Network{Total: total, ByInterface: byInterface}
}
