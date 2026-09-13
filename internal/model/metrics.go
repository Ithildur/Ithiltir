package model

import (
	"time"

	"github.com/Ithildur/EiluneKit/postgres/dbtypes"
	"gorm.io/datatypes"
)

// MetricValues contains scalar metric fields stored in both history and current rows.
// Adding a field also requires ingest, history rebuild, frontend snapshot conversion, and migration updates.
type MetricValues struct {
	CPUUsageRatio     float64  `gorm:"column:cpu_usage_ratio;default:0"`
	Load1             float64  `gorm:"column:load1;default:0"`
	Load5             float64  `gorm:"column:load5;default:0"`
	Load15            float64  `gorm:"column:load15;default:0"`
	CPUUser           float64  `gorm:"column:cpu_user;default:0"`
	CPUSystem         float64  `gorm:"column:cpu_system;default:0"`
	CPUIdle           float64  `gorm:"column:cpu_idle;default:0"`
	CPUIowait         float64  `gorm:"column:cpu_iowait;default:0"`
	CPUSteal          float64  `gorm:"column:cpu_steal;default:0"`
	CPUTempC          *float64 `gorm:"column:cpu_temp_c"`
	MemTotal          int64    `gorm:"column:mem_total;default:0"`
	MemUsed           int64    `gorm:"column:mem_used;default:0"`
	MemAvailable      int64    `gorm:"column:mem_available;default:0"`
	MemBuffers        int64    `gorm:"column:mem_buffers;default:0"`
	MemCached         int64    `gorm:"column:mem_cached;default:0"`
	MemUsedRatio      float64  `gorm:"column:mem_used_ratio;default:0"`
	SwapTotal         int64    `gorm:"column:swap_total;default:0"`
	SwapUsed          int64    `gorm:"column:swap_used;default:0"`
	SwapFree          int64    `gorm:"column:swap_free;default:0"`
	SwapUsedRatio     float64  `gorm:"column:swap_used_ratio;default:0"`
	NetInBytes        int64    `gorm:"column:net_in_bytes;default:0"`
	NetOutBytes       int64    `gorm:"column:net_out_bytes;default:0"`
	NetInBps          float64  `gorm:"column:net_in_bps;default:0"`
	NetOutBps         float64  `gorm:"column:net_out_bps;default:0"`
	ProcessCount      int32    `gorm:"column:process_count;default:0"`
	TCPConn           int32    `gorm:"column:tcp_conn;default:0"`
	UDPConn           int32    `gorm:"column:udp_conn;default:0"`
	UptimeSeconds     int64    `gorm:"column:uptime_seconds;default:0"`
	RaidSupported     bool     `gorm:"column:raid_supported;default:false"`
	RaidAvailable     bool     `gorm:"column:raid_available;default:false"`
	RaidOverallHealth string   `gorm:"column:raid_overall_health;size:16;default:''"`
	PSICPUSomeAvg10   *float64 `gorm:"column:psi_cpu_some_avg10"`
	PSICPUSomeAvg60   *float64 `gorm:"column:psi_cpu_some_avg60"`
	PSICPUSomeAvg300  *float64 `gorm:"column:psi_cpu_some_avg300"`
	PSICPUSomeTotal   *int64   `gorm:"column:psi_cpu_some_total"`
	PSICPUFullAvg10   *float64 `gorm:"column:psi_cpu_full_avg10"`
	PSICPUFullAvg60   *float64 `gorm:"column:psi_cpu_full_avg60"`
	PSICPUFullAvg300  *float64 `gorm:"column:psi_cpu_full_avg300"`
	PSICPUFullTotal   *int64   `gorm:"column:psi_cpu_full_total"`
	PSIMemSomeAvg10   *float64 `gorm:"column:psi_memory_some_avg10"`
	PSIMemSomeAvg60   *float64 `gorm:"column:psi_memory_some_avg60"`
	PSIMemSomeAvg300  *float64 `gorm:"column:psi_memory_some_avg300"`
	PSIMemSomeTotal   *int64   `gorm:"column:psi_memory_some_total"`
	PSIMemFullAvg10   *float64 `gorm:"column:psi_memory_full_avg10"`
	PSIMemFullAvg60   *float64 `gorm:"column:psi_memory_full_avg60"`
	PSIMemFullAvg300  *float64 `gorm:"column:psi_memory_full_avg300"`
	PSIMemFullTotal   *int64   `gorm:"column:psi_memory_full_total"`
	PSIIOSomeAvg10    *float64 `gorm:"column:psi_io_some_avg10"`
	PSIIOSomeAvg60    *float64 `gorm:"column:psi_io_some_avg60"`
	PSIIOSomeAvg300   *float64 `gorm:"column:psi_io_some_avg300"`
	PSIIOSomeTotal    *int64   `gorm:"column:psi_io_some_total"`
	PSIIOFullAvg10    *float64 `gorm:"column:psi_io_full_avg10"`
	PSIIOFullAvg60    *float64 `gorm:"column:psi_io_full_avg60"`
	PSIIOFullAvg300   *float64 `gorm:"column:psi_io_full_avg300"`
	PSIIOFullTotal    *int64   `gorm:"column:psi_io_full_total"`
}

// MetricRuntime contains current runtime payloads that should not be written to history.
type MetricRuntime struct {
	Raid    datatypes.JSON `gorm:"column:raid"`
	Thermal datatypes.JSON `gorm:"column:thermal"`
}

// MetricsSnapshot contains the full current metric snapshot.
type MetricsSnapshot struct {
	MetricValues  `gorm:"embedded"`
	MetricRuntime `gorm:"embedded"`
}

var metricValueColumns = []string{
	"cpu_usage_ratio",
	"load1",
	"load5",
	"load15",
	"cpu_user",
	"cpu_system",
	"cpu_idle",
	"cpu_iowait",
	"cpu_steal",
	"cpu_temp_c",
	"mem_total",
	"mem_used",
	"mem_available",
	"mem_buffers",
	"mem_cached",
	"mem_used_ratio",
	"swap_total",
	"swap_used",
	"swap_free",
	"swap_used_ratio",
	"net_in_bytes",
	"net_out_bytes",
	"net_in_bps",
	"net_out_bps",
	"process_count",
	"tcp_conn",
	"udp_conn",
	"uptime_seconds",
	"raid_supported",
	"raid_available",
	"raid_overall_health",
	"psi_cpu_some_avg10",
	"psi_cpu_some_avg60",
	"psi_cpu_some_avg300",
	"psi_cpu_some_total",
	"psi_cpu_full_avg10",
	"psi_cpu_full_avg60",
	"psi_cpu_full_avg300",
	"psi_cpu_full_total",
	"psi_memory_some_avg10",
	"psi_memory_some_avg60",
	"psi_memory_some_avg300",
	"psi_memory_some_total",
	"psi_memory_full_avg10",
	"psi_memory_full_avg60",
	"psi_memory_full_avg300",
	"psi_memory_full_total",
	"psi_io_some_avg10",
	"psi_io_some_avg60",
	"psi_io_some_avg300",
	"psi_io_some_total",
	"psi_io_full_avg10",
	"psi_io_full_avg60",
	"psi_io_full_avg300",
	"psi_io_full_total",
}

var metricRuntimeColumns = []string{
	"raid",
	"thermal",
}

var metricsSnapshotColumns = append(append([]string{}, metricValueColumns...), metricRuntimeColumns...)

// ServerMetric represents table server_metrics (time-series history).
type ServerMetric struct {
	ServerID     int64      `gorm:"column:server_id;not null;primaryKey"`
	CollectedAt  time.Time  `gorm:"column:collected_at;not null;primaryKey"` // 服务端接收并归档的时间，历史主时间轴。
	ReportedAt   *time.Time `gorm:"column:reported_at"`                      // agent 原始观测时间；告警 duration 用它，不能替代主键时间。
	MetricValues `gorm:"embedded"`
}

func (ServerMetric) TableName() string { return "server_metrics" }

// ServerCurrentMetric represents table server_current_metrics (latest server metrics).
type ServerCurrentMetric struct {
	ServerID        int64      `gorm:"column:server_id;not null;primaryKey"`
	CollectedAt     time.Time  `gorm:"column:collected_at;not null"` // 当前态对应的历史采集时间，用于拒绝旧上报覆盖。
	ReportedAt      *time.Time `gorm:"column:reported_at"`           // agent 原始观测时间；告警 duration 用它，不能替代 CollectedAt。
	MetricsSnapshot `gorm:"embedded"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (ServerCurrentMetric) TableName() string { return "server_current_metrics" }

func ServerCurrentMetricUpdateColumns() []string {
	columns := append([]string{"collected_at", "reported_at"}, metricsSnapshotColumns...)
	return append(columns, "updated_at")
}

// DiskMetric represents table disk_metrics (per base_io time series).
type DiskMetric struct {
	ServerID    int64     `gorm:"column:server_id;primaryKey"`
	Name        string    `gorm:"column:name;size:255;primaryKey"`
	Ref         string    `gorm:"column:ref;size:320"`
	Kind        string    `gorm:"column:kind;size:16"`
	Role        string    `gorm:"column:role;size:16"` // primary 设备进入前端磁盘 IO 汇总，其他设备只保留明细。
	Path        string    `gorm:"column:path;type:text"`
	CollectedAt time.Time `gorm:"column:collected_at;primaryKey"`

	ReadBytes            int64   `gorm:"column:read_bytes"`
	WriteBytes           int64   `gorm:"column:write_bytes"`
	ReadRateBytesPerSec  float64 `gorm:"column:read_rate_bytes_per_sec"`
	WriteRateBytesPerSec float64 `gorm:"column:write_rate_bytes_per_sec"`
	IOPS                 float64 `gorm:"column:iops"`
	ReadIOPS             float64 `gorm:"column:read_iops"`
	WriteIOPS            float64 `gorm:"column:write_iops"`
	UtilRatio            float64 `gorm:"column:util_ratio"`
	QueueLength          float64 `gorm:"column:queue_length"`
	WaitMs               float64 `gorm:"column:wait_ms"`
	ServiceMs            float64 `gorm:"column:service_ms"`
}

func (DiskMetric) TableName() string { return "disk_metrics" }

// DiskPhysicalMetric is one disk_physical_metrics temperature sample.
type DiskPhysicalMetric struct {
	ServerID    int64     `gorm:"column:server_id;primaryKey"`
	Name        string    `gorm:"column:name;size:255;primaryKey"`
	Ref         string    `gorm:"column:ref;size:320"`
	Path        string    `gorm:"column:path;type:text"`
	CollectedAt time.Time `gorm:"column:collected_at;primaryKey"`
	TempC       float64   `gorm:"column:temp_c"`
}

func (DiskPhysicalMetric) TableName() string { return "disk_physical_metrics" }

// ServerCurrentDiskMetric represents table server_current_disk_metrics (latest per base_io device).
type ServerCurrentDiskMetric struct {
	ServerID    int64     `gorm:"column:server_id;primaryKey"`
	Name        string    `gorm:"column:name;size:255;primaryKey"`
	Ref         string    `gorm:"column:ref;size:320"`
	Kind        string    `gorm:"column:kind;size:16"`
	Role        string    `gorm:"column:role;size:16"` // primary 设备进入前端磁盘 IO 汇总，其他设备只保留明细。
	Path        string    `gorm:"column:path;type:text"`
	CollectedAt time.Time `gorm:"column:collected_at;not null"` // 当前态对应的历史采集时间。

	ReadBytes            int64     `gorm:"column:read_bytes"`
	WriteBytes           int64     `gorm:"column:write_bytes"`
	ReadRateBytesPerSec  float64   `gorm:"column:read_rate_bytes_per_sec"`
	WriteRateBytesPerSec float64   `gorm:"column:write_rate_bytes_per_sec"`
	IOPS                 float64   `gorm:"column:iops"`
	ReadIOPS             float64   `gorm:"column:read_iops"`
	WriteIOPS            float64   `gorm:"column:write_iops"`
	UtilRatio            float64   `gorm:"column:util_ratio"`
	QueueLength          float64   `gorm:"column:queue_length"`
	WaitMs               float64   `gorm:"column:wait_ms"`
	ServiceMs            float64   `gorm:"column:service_ms"`
	CreatedAt            time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt            time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (ServerCurrentDiskMetric) TableName() string { return "server_current_disk_metrics" }

// DiskUsageMetric represents table disk_usage_metrics (per logical disk usage time series).
type DiskUsageMetric struct {
	ServerID    int64     `gorm:"column:server_id;primaryKey"`
	Name        string    `gorm:"column:name;size:255;primaryKey"`
	Ref         string    `gorm:"column:ref;size:320"`
	Kind        string    `gorm:"column:kind;size:16"`
	Mountpoint  string    `gorm:"column:mountpoint;type:text"`
	Path        string    `gorm:"column:path;type:text"`
	CollectedAt time.Time `gorm:"column:collected_at;primaryKey"`

	Total     int64   `gorm:"column:total"`
	Used      int64   `gorm:"column:used"`
	Free      int64   `gorm:"column:free"`
	UsedRatio float64 `gorm:"column:used_ratio"`

	FSType  string            `gorm:"column:fs_type;size:32"`
	Devices dbtypes.TextArray `gorm:"column:devices;type:text[]"`

	Health string `gorm:"column:health;size:32"`
	Level  string `gorm:"column:level;size:32"`
}

func (DiskUsageMetric) TableName() string { return "disk_usage_metrics" }

// ServerCurrentDiskUsageMetric represents table server_current_disk_usage_metrics (latest per logical disk).
type ServerCurrentDiskUsageMetric struct {
	ServerID    int64     `gorm:"column:server_id;primaryKey"`
	Name        string    `gorm:"column:name;size:255;primaryKey"`
	Ref         string    `gorm:"column:ref;size:320"`
	Kind        string    `gorm:"column:kind;size:16"`
	Mountpoint  string    `gorm:"column:mountpoint;type:text"`
	Path        string    `gorm:"column:path;type:text"`
	CollectedAt time.Time `gorm:"column:collected_at;not null"` // 当前态对应的历史采集时间。

	Total     int64   `gorm:"column:total"`
	Used      int64   `gorm:"column:used"`
	Free      int64   `gorm:"column:free"`
	UsedRatio float64 `gorm:"column:used_ratio"`

	FSType  string            `gorm:"column:fs_type;size:32"`
	Devices dbtypes.TextArray `gorm:"column:devices;type:text[]"`

	Health    string    `gorm:"column:health;size:32"`
	Level     string    `gorm:"column:level;size:32"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (ServerCurrentDiskUsageMetric) TableName() string {
	return "server_current_disk_usage_metrics"
}

// NICMetric represents table nic_metrics (per interface time series).
type NICMetric struct {
	ServerID    int64     `gorm:"column:server_id;primaryKey"`
	Iface       string    `gorm:"column:iface;size:64;primaryKey"`
	CollectedAt time.Time `gorm:"column:collected_at;primaryKey"`

	BytesRecv             int64   `gorm:"column:bytes_recv"`
	BytesSent             int64   `gorm:"column:bytes_sent"`
	RecvRateBytesPerSec   float64 `gorm:"column:recv_rate_bytes_per_sec"`
	SentRateBytesPerSec   float64 `gorm:"column:sent_rate_bytes_per_sec"`
	PacketsRecv           int64   `gorm:"column:packets_recv"`
	PacketsSent           int64   `gorm:"column:packets_sent"`
	RecvRatePacketsPerSec float64 `gorm:"column:recv_rate_packets_per_sec"`
	SentRatePacketsPerSec float64 `gorm:"column:sent_rate_packets_per_sec"`
	ErrIn                 int64   `gorm:"column:err_in"`
	ErrOut                int64   `gorm:"column:err_out"`
	DropIn                int64   `gorm:"column:drop_in"`
	DropOut               int64   `gorm:"column:drop_out"`

	Extra datatypes.JSON `gorm:"column:extra"`
}

func (NICMetric) TableName() string { return "nic_metrics" }

// ServerCurrentNICMetric represents table server_current_nic_metrics (latest per network interface).
type ServerCurrentNICMetric struct {
	ServerID    int64     `gorm:"column:server_id;primaryKey"`
	Iface       string    `gorm:"column:iface;size:64;primaryKey"`
	CollectedAt time.Time `gorm:"column:collected_at;not null"` // 当前态对应的历史采集时间。

	BytesRecv             int64   `gorm:"column:bytes_recv"`
	BytesSent             int64   `gorm:"column:bytes_sent"`
	RecvRateBytesPerSec   float64 `gorm:"column:recv_rate_bytes_per_sec"`
	SentRateBytesPerSec   float64 `gorm:"column:sent_rate_bytes_per_sec"`
	PacketsRecv           int64   `gorm:"column:packets_recv"`
	PacketsSent           int64   `gorm:"column:packets_sent"`
	RecvRatePacketsPerSec float64 `gorm:"column:recv_rate_packets_per_sec"`
	SentRatePacketsPerSec float64 `gorm:"column:sent_rate_packets_per_sec"`
	ErrIn                 int64   `gorm:"column:err_in"`
	ErrOut                int64   `gorm:"column:err_out"`
	DropIn                int64   `gorm:"column:drop_in"`
	DropOut               int64   `gorm:"column:drop_out"`

	Extra     datatypes.JSON `gorm:"column:extra"`
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime"`
}

func (ServerCurrentNICMetric) TableName() string { return "server_current_nic_metrics" }
