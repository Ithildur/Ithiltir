// Package metrics provides shared metrics types and conversion utilities.
package metrics

import "time"

// NodeReport represents the metrics report sent by node agents.
type NodeReport struct {
	Version      string    `json:"version"`
	Hostname     string    `json:"hostname"`
	Timestamp    time.Time `json:"timestamp"`
	Metrics      Metrics   `json:"metrics"`
	SentAt       string    `json:"sent_at,omitempty"`
	ServerID     int64     `json:"server_id,omitempty"`
	DisplayOrder int       `json:"display_order,omitempty"`
}

// Metrics is the full metrics snapshot for a node.
type Metrics struct {
	CPU         CPUMetrics        `json:"cpu"`
	Memory      MemoryMetrics     `json:"memory"`
	Disk        DiskMetrics       `json:"disk"`
	Network     []NetIOMetrics    `json:"network"`
	System      SystemMetrics     `json:"system"`
	Processes   ProcessMetrics    `json:"processes"`
	Connections ConnectionMetrics `json:"connections"`
	Raid        RaidMetrics       `json:"raid"`
	Thermal     *Thermal          `json:"thermal,omitempty"`
}

// CPUMetrics describes CPU usage and load.
type CPUMetrics struct {
	UsageRatio float64  `json:"usage_ratio"`
	Load1      float64  `json:"load1"`
	Load5      float64  `json:"load5"`
	Load15     float64  `json:"load15"`
	Times      CPUTimes `json:"times"`
}

// CPUTimes is CPU time breakdown.
type CPUTimes struct {
	User   float64 `json:"user"`
	System float64 `json:"system"`
	Idle   float64 `json:"idle"`
	Iowait float64 `json:"iowait"`
	Steal  float64 `json:"steal"`
}

// MemoryMetrics describes memory and swap usage.
type MemoryMetrics struct {
	Total         uint64  `json:"total,omitempty"`
	Used          uint64  `json:"used"`
	Available     uint64  `json:"available"`
	Buffers       uint64  `json:"buffers"`
	Cached        uint64  `json:"cached"`
	UsedRatio     float64 `json:"used_ratio"`
	SwapTotal     uint64  `json:"swap_total,omitempty"`
	SwapUsed      uint64  `json:"swap_used"`
	SwapFree      uint64  `json:"swap_free"`
	SwapUsedRatio float64 `json:"swap_used_ratio"`
}

// DiskMetrics wraps usage and IO.
type DiskMetrics struct {
	Physical    []DiskPhysicalMetrics   `json:"physical"`
	Logical     []DiskLogicalMetrics    `json:"logical"`
	Filesystems []DiskFilesystemMetrics `json:"filesystems"`
	BaseIO      []DiskBaseIOMetrics     `json:"base_io"`
	Smart       *DiskSmart              `json:"smart,omitempty"`
}

type DiskSmart struct {
	Status     string            `json:"status"`
	UpdatedAt  *time.Time        `json:"updated_at,omitempty"`
	TTLSeconds int               `json:"ttl_seconds,omitempty"`
	Devices    []DiskSmartDevice `json:"devices"`
}

type DiskSmartDevice struct {
	Ref                 string          `json:"ref,omitempty"`
	Name                string          `json:"name"`
	DevicePath          string          `json:"device_path,omitempty"`
	DeviceType          string          `json:"device_type,omitempty"`
	Protocol            string          `json:"protocol,omitempty"`
	Model               string          `json:"model,omitempty"`
	Serial              string          `json:"serial,omitempty"`
	WWN                 string          `json:"wwn,omitempty"`
	Source              string          `json:"source"`
	Status              string          `json:"status"`
	ExitStatus          *int            `json:"exit_status,omitempty"`
	Health              *string         `json:"health,omitempty"`
	TempC               *float64        `json:"temp_c,omitempty"`
	PowerOnHours        *uint64         `json:"power_on_hours,omitempty"`
	LifetimeUsedPercent *float64        `json:"lifetime_used_percent,omitempty"`
	CriticalWarning     *uint64         `json:"critical_warning,omitempty"`
	FailingAttrs        []DiskSmartAttr `json:"failing_attrs,omitempty"`
}

type DiskSmartAttr struct {
	ID         int    `json:"id,omitempty"`
	Name       string `json:"name"`
	WhenFailed string `json:"when_failed"`
}

// DiskPhysicalMetrics is per block-device IO.
type DiskPhysicalMetrics struct {
	Name                 string  `json:"name"`
	DevicePath           string  `json:"device_path,omitempty"`
	Ref                  string  `json:"ref,omitempty"`
	ReadBytes            uint64  `json:"read_bytes"`
	WriteBytes           uint64  `json:"write_bytes"`
	ReadRateBytesPerSec  float64 `json:"read_rate_bytes_per_sec"`
	WriteRateBytesPerSec float64 `json:"write_rate_bytes_per_sec"`
	IOPS                 float64 `json:"iops"`
	ReadIOPS             float64 `json:"read_iops"`
	WriteIOPS            float64 `json:"write_iops"`
	UtilRatio            float64 `json:"util_ratio"`
	QueueLength          float64 `json:"queue_length"`
	WaitMs               float64 `json:"wait_ms"`
	ServiceMs            float64 `json:"service_ms"`
}

// DiskLogicalMetrics is per logical storage (zfs/raid/lvm).
type DiskLogicalMetrics struct {
	Kind        string                           `json:"kind"`
	Name        string                           `json:"name"`
	DevicePath  string                           `json:"device_path,omitempty"`
	Ref         string                           `json:"ref,omitempty"`
	Total       uint64                           `json:"total,omitempty"`
	Used        uint64                           `json:"used"`
	Free        uint64                           `json:"free"`
	UsedRatio   float64                          `json:"used_ratio"`
	Health      string                           `json:"health,omitempty"`
	Level       string                           `json:"level,omitempty"`
	Mountpoint  string                           `json:"mountpoint,omitempty"`
	Mountpoints map[string]DiskMountpointMetrics `json:"mountpoints,omitempty"`
	Devices     []string                         `json:"devices,omitempty"`
}

// DiskFilesystemMetrics is per mountpoint usage.
type DiskFilesystemMetrics struct {
	Path            string  `json:"path"`
	Device          string  `json:"device,omitempty"`
	Mountpoint      string  `json:"mountpoint,omitempty"`
	Total           uint64  `json:"total,omitempty"`
	Used            uint64  `json:"used"`
	Free            uint64  `json:"free"`
	UsedRatio       float64 `json:"used_ratio"`
	FSType          string  `json:"fs_type,omitempty"`
	InodesTotal     uint64  `json:"inodes_total,omitempty"`
	InodesUsed      uint64  `json:"inodes_used"`
	InodesFree      uint64  `json:"inodes_free"`
	InodesUsedRatio float64 `json:"inodes_used_ratio"`
}

// DiskBaseIOMetrics is base IO unit for display.
type DiskBaseIOMetrics struct {
	Kind                 string  `json:"kind"`
	Name                 string  `json:"name"`
	DevicePath           string  `json:"device_path,omitempty"`
	Ref                  string  `json:"ref,omitempty"`
	Role                 string  `json:"role,omitempty"`
	ReadBytes            uint64  `json:"read_bytes,omitempty"`
	WriteBytes           uint64  `json:"write_bytes,omitempty"`
	ReadRateBytesPerSec  float64 `json:"read_rate_bytes_per_sec"`
	WriteRateBytesPerSec float64 `json:"write_rate_bytes_per_sec"`
	ReadIOPS             float64 `json:"read_iops"`
	WriteIOPS            float64 `json:"write_iops"`
	IOPS                 float64 `json:"iops"`
	UtilRatio            float64 `json:"util_ratio,omitempty"`
	QueueLength          float64 `json:"queue_length,omitempty"`
	WaitMs               float64 `json:"wait_ms,omitempty"`
	ServiceMs            float64 `json:"service_ms,omitempty"`
}

type DiskMountpointMetrics struct {
	FSType          string  `json:"fs_type,omitempty"`
	InodesTotal     uint64  `json:"inodes_total,omitempty"`
	InodesUsed      uint64  `json:"inodes_used,omitempty"`
	InodesFree      uint64  `json:"inodes_free,omitempty"`
	InodesUsedRatio float64 `json:"inodes_used_ratio,omitempty"`
}

// NetIOMetrics is per-interface IO.
type NetIOMetrics struct {
	Name                  string  `json:"name"`
	BytesRecv             uint64  `json:"bytes_recv"`
	BytesSent             uint64  `json:"bytes_sent"`
	RecvRateBytesPerSec   float64 `json:"recv_rate_bytes_per_sec"`
	SentRateBytesPerSec   float64 `json:"sent_rate_bytes_per_sec"`
	PacketsRecv           uint64  `json:"packets_recv"`
	PacketsSent           uint64  `json:"packets_sent"`
	RecvRatePacketsPerSec float64 `json:"recv_rate_packets_per_sec"`
	SentRatePacketsPerSec float64 `json:"sent_rate_packets_per_sec"`
	ErrIn                 uint64  `json:"err_in"`
	ErrOut                uint64  `json:"err_out"`
	DropIn                uint64  `json:"drop_in"`
	DropOut               uint64  `json:"drop_out"`
}

// SystemMetrics is uptime status info.
type SystemMetrics struct {
	Alive         bool   `json:"alive"`
	UptimeSeconds uint64 `json:"uptime_seconds"`
	Uptime        string `json:"uptime"`
}

type ProcessMetrics struct {
	ProcessCount int `json:"process_count"`
}

// ConnectionMetrics describes TCP/UDP counts.
type ConnectionMetrics struct {
	TCPCount int `json:"tcp_count"`
	UDPCount int `json:"udp_count"`
}

// RaidMetrics describes RAID status.
type RaidMetrics struct {
	Supported bool        `json:"supported"`
	Available bool        `json:"available"`
	Arrays    []RaidArray `json:"arrays"`
}

// RaidArray is one RAID array.
type RaidArray struct {
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

// RaidMember is per-disk state.
type RaidMember struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type Thermal struct {
	Status    string          `json:"status"`
	UpdatedAt *time.Time      `json:"updated_at,omitempty"`
	Sensors   []ThermalSensor `json:"sensors"`
}

type ThermalSensor struct {
	Kind      string   `json:"kind"`
	Name      string   `json:"name"`
	SensorKey string   `json:"sensor_key"`
	Source    string   `json:"source"`
	Status    string   `json:"status"`
	TempC     *float64 `json:"temp_c,omitempty"`
	HighC     *float64 `json:"high_c,omitempty"`
	CriticalC *float64 `json:"critical_c,omitempty"`
}
