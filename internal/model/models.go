package model

import (
	"time"

	"github.com/Ithildur/EiluneKit/postgres/dbtypes"

	"gorm.io/datatypes"
)

// Enum types reflecting PostgreSQL enums.
type (
	ServiceType  string
	TaskType     string
	TargetType   string
	ObjectType   string
	AlertStatus  string
	NotifyType   string
	OutboxStatus string
	TaskStatus   string
)

const (
	ServiceTypeHTTP ServiceType = "http"
	ServiceTypeTCP  ServiceType = "tcp"
	ServiceTypePing ServiceType = "ping"
	ServiceTypeDNS  ServiceType = "dns"
	ServiceTypeTLS  ServiceType = "tls"

	TaskTypeShell  TaskType = "shell"
	TaskTypeHTTP   TaskType = "http"
	TaskTypeScript TaskType = "script"

	TargetTypeAll    TargetType = "all"
	TargetTypeGroup  TargetType = "group"
	TargetTypeServer TargetType = "server"
	TargetTypeCustom TargetType = "custom"

	ObjectTypeServer  ObjectType = "server"
	ObjectTypeService ObjectType = "service"

	AlertStatusOpen   AlertStatus = "open"
	AlertStatusClosed AlertStatus = "closed"

	NotifyTypeTelegram NotifyType = "telegram"
	NotifyTypeEmail    NotifyType = "email"
	NotifyTypeWebhook  NotifyType = "webhook"
	NotifyTypeWeChat   NotifyType = "wechat"
	NotifyTypeSlack    NotifyType = "slack"
	NotifyTypeDiscord  NotifyType = "discord"

	OutboxStatusPending         OutboxStatus = "pending"
	OutboxStatusSending         OutboxStatus = "sending"
	OutboxStatusSent            OutboxStatus = "sent"
	OutboxStatusRetry           OutboxStatus = "retry"
	OutboxStatusFailedPermanent OutboxStatus = "failed_permanent"

	TaskStatusPending TaskStatus = "pending"
	TaskStatusLeased  TaskStatus = "leased"
)

// Group represents table groups.
type Group struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Name      string    `gorm:"column:name;size:64;not null"`
	Remark    string    `gorm:"column:remark"`
	IsDeleted bool      `gorm:"column:is_deleted;not null;default:false"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (Group) TableName() string { return "groups" }

// Server represents table servers.
type Server struct {
	ID                       int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Name                     string         `gorm:"column:name;size:64;not null"`
	Hostname                 string         `gorm:"column:hostname;size:128;not null"`
	Secret                   string         `gorm:"column:secret;size:128;not null;unique"` // 节点 agent 鉴权密钥，访客接口不能暴露。
	Tags                     datatypes.JSON `gorm:"column:tags"`
	IP                       *string        `gorm:"column:ip"`
	OS                       *string        `gorm:"column:os"`
	Platform                 *string        `gorm:"column:platform"`
	PlatformVersion          *string        `gorm:"column:platform_version"`
	KernelVersion            *string        `gorm:"column:kernel_version"`
	Arch                     *string        `gorm:"column:arch"`
	Location                 *string        `gorm:"column:location"`
	CPUModel                 *string        `gorm:"column:cpu_model"`
	CPUVendor                *string        `gorm:"column:cpu_vendor"`
	CPUCoresPhys             *int16         `gorm:"column:cpu_cores_physical"`
	CPUCoresLog              *int16         `gorm:"column:cpu_cores_logical"`
	CPUSockets               *int16         `gorm:"column:cpu_sockets"`
	CPUMhz                   *float64       `gorm:"column:cpu_mhz"`
	MemTotal                 *int64         `gorm:"column:mem_total"`
	SwapTotal                *int64         `gorm:"column:swap_total"`
	DiskTotal                *int64         `gorm:"column:disk_total"`
	RootPath                 *string        `gorm:"column:root_path;size:256"` // 前端根磁盘展示入口，通常来自最大逻辑盘挂载点。
	RootFSType               *string        `gorm:"column:root_fs_type"`
	RaidSupported            *bool          `gorm:"column:raid_supported"`
	RaidAvailable            *bool          `gorm:"column:raid_available"`
	IntervalSec              *int32         `gorm:"column:interval_sec"`
	IsGuestVisible           bool           `gorm:"column:is_guest_visible;not null;default:false"`    // 访客读取历史指标和流量时按它过滤。
	TrafficP95Enabled        bool           `gorm:"column:traffic_p95_enabled;not null;default:false"` // 高级计费模式下是否为该节点计算 95 带宽。
	TrafficCycleMode         string         `gorm:"column:traffic_cycle_mode;size:32;not null;default:'default'"`
	TrafficBillingStartDay   int16          `gorm:"column:traffic_billing_start_day;not null;default:1"`
	TrafficBillingAnchorDate string         `gorm:"column:traffic_billing_anchor_date;size:10;not null;default:''"`
	TrafficBillingTimezone   string         `gorm:"column:traffic_billing_timezone;size:64;not null;default:''"`
	IsDeleted                bool           `gorm:"column:is_deleted;not null;default:false"`
	AgentVersion             *string        `gorm:"column:agent_version"`
	Remark                   *string        `gorm:"column:remark"`
	DisplayOrder             int            `gorm:"column:display_order;not null;default:0"`
	CreatedAt                time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt                time.Time      `gorm:"column:updated_at;autoUpdateTime"`
}

func (Server) TableName() string { return "servers" }

// ServerGroup represents many-to-many between servers and groups.
type ServerGroup struct {
	ServerID  int64     `gorm:"column:server_id;primaryKey"`
	GroupID   int64     `gorm:"column:group_id;primaryKey"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (ServerGroup) TableName() string { return "server_groups" }

// MetricsSnapshot contains all metrics fields shared by ServerMetric.
// This eliminates field duplication and centralizes the metrics schema.
// Adding a field also requires ingest, history rebuild, frontend snapshot conversion, and migration updates.
type MetricsSnapshot struct {
	CPUUsageRatio     float64        `gorm:"column:cpu_usage_ratio;default:0"`
	Load1             float64        `gorm:"column:load1;default:0"`
	Load5             float64        `gorm:"column:load5;default:0"`
	Load15            float64        `gorm:"column:load15;default:0"`
	CPUUser           float64        `gorm:"column:cpu_user;default:0"`
	CPUSystem         float64        `gorm:"column:cpu_system;default:0"`
	CPUIdle           float64        `gorm:"column:cpu_idle;default:0"`
	CPUIowait         float64        `gorm:"column:cpu_iowait;default:0"`
	CPUSteal          float64        `gorm:"column:cpu_steal;default:0"`
	CPUTempC          *float64       `gorm:"column:cpu_temp_c"`
	MemTotal          int64          `gorm:"column:mem_total;default:0"`
	MemUsed           int64          `gorm:"column:mem_used;default:0"`
	MemAvailable      int64          `gorm:"column:mem_available;default:0"`
	MemBuffers        int64          `gorm:"column:mem_buffers;default:0"`
	MemCached         int64          `gorm:"column:mem_cached;default:0"`
	MemUsedRatio      float64        `gorm:"column:mem_used_ratio;default:0"`
	SwapTotal         int64          `gorm:"column:swap_total;default:0"`
	SwapUsed          int64          `gorm:"column:swap_used;default:0"`
	SwapFree          int64          `gorm:"column:swap_free;default:0"`
	SwapUsedRatio     float64        `gorm:"column:swap_used_ratio;default:0"`
	NetInBytes        int64          `gorm:"column:net_in_bytes;default:0"`
	NetOutBytes       int64          `gorm:"column:net_out_bytes;default:0"`
	NetInBps          float64        `gorm:"column:net_in_bps;default:0"`
	NetOutBps         float64        `gorm:"column:net_out_bps;default:0"`
	ProcessCount      int32          `gorm:"column:process_count;default:0"`
	TCPConn           int32          `gorm:"column:tcp_conn;default:0"`
	UDPConn           int32          `gorm:"column:udp_conn;default:0"`
	UptimeSeconds     int64          `gorm:"column:uptime_seconds;default:0"`
	RaidSupported     bool           `gorm:"column:raid_supported;default:false"`
	RaidAvailable     bool           `gorm:"column:raid_available;default:false"`
	RaidOverallHealth string         `gorm:"column:raid_overall_health;default:''"`
	Raid              datatypes.JSON `gorm:"column:raid"`
	Thermal           datatypes.JSON `gorm:"column:thermal"`
}

var metricsSnapshotColumns = []string{
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
	"raid",
	"thermal",
}

// ServerMetric represents table server_metrics (time-series history).
type ServerMetric struct {
	ServerID    int64      `gorm:"column:server_id;not null;primaryKey"`
	CollectedAt time.Time  `gorm:"column:collected_at;not null;primaryKey"` // 服务端接收并归档的时间，历史主时间轴。
	ReportedAt  *time.Time `gorm:"column:reported_at"`                      // agent 原始观测时间；告警 duration 用它，不能替代主键时间。
	MetricsSnapshot
}

func (ServerMetric) TableName() string { return "server_metrics" }

// ServerCurrentMetric represents table server_current_metrics (latest server metrics).
type ServerCurrentMetric struct {
	ServerID    int64      `gorm:"column:server_id;not null;primaryKey"`
	CollectedAt time.Time  `gorm:"column:collected_at;not null"` // 当前态对应的历史采集时间，用于拒绝旧上报覆盖。
	ReportedAt  *time.Time `gorm:"column:reported_at"`           // agent 原始观测时间；告警 duration 用它，不能替代 CollectedAt。
	MetricsSnapshot
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (ServerCurrentMetric) TableName() string { return "server_current_metrics" }

func ServerCurrentMetricUpdateColumns() []string {
	columns := append([]string{"collected_at", "reported_at"}, metricsSnapshotColumns...)
	return append(columns, "updated_at")
}

func (m ServerCurrentMetric) ToServerMetric() ServerMetric {
	return ServerMetric{
		ServerID:        m.ServerID,
		CollectedAt:     m.CollectedAt,
		ReportedAt:      m.ReportedAt,
		MetricsSnapshot: m.MetricsSnapshot,
	}
}

// DiskMetric represents table disk_metrics (per base_io time series).
type DiskMetric struct {
	ServerID    int64     `gorm:"column:server_id;primaryKey"`
	Name        string    `gorm:"column:name;size:128;primaryKey"`
	Ref         string    `gorm:"column:ref;size:128"`
	Kind        string    `gorm:"column:kind;size:16"`
	Role        string    `gorm:"column:role;size:16"` // primary 设备进入前端磁盘 IO 汇总，其他设备只保留明细。
	Path        string    `gorm:"column:path;size:256"`
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
	Name        string    `gorm:"column:name;size:128;primaryKey"`
	Ref         string    `gorm:"column:ref;size:128"`
	Path        string    `gorm:"column:path;size:256"`
	CollectedAt time.Time `gorm:"column:collected_at;primaryKey"`
	TempC       float64   `gorm:"column:temp_c"`
}

func (DiskPhysicalMetric) TableName() string { return "disk_physical_metrics" }

// ServerCurrentDiskMetric represents table server_current_disk_metrics (latest per base_io device).
type ServerCurrentDiskMetric struct {
	ServerID    int64     `gorm:"column:server_id;primaryKey"`
	Name        string    `gorm:"column:name;size:128;primaryKey"`
	Ref         string    `gorm:"column:ref;size:128"`
	Kind        string    `gorm:"column:kind;size:16"`
	Role        string    `gorm:"column:role;size:16"` // primary 设备进入前端磁盘 IO 汇总，其他设备只保留明细。
	Path        string    `gorm:"column:path;size:256"`
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
	Name        string    `gorm:"column:name;size:128;primaryKey"`
	Ref         string    `gorm:"column:ref;size:128"`
	Kind        string    `gorm:"column:kind;size:16"`
	Mountpoint  string    `gorm:"column:mountpoint;size:256"`
	Path        string    `gorm:"column:path;size:256"`
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
	Name        string    `gorm:"column:name;size:128;primaryKey"`
	Ref         string    `gorm:"column:ref;size:128"`
	Kind        string    `gorm:"column:kind;size:16"`
	Mountpoint  string    `gorm:"column:mountpoint;size:256"`
	Path        string    `gorm:"column:path;size:256"`
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

// Traffic5m stores 5-minute billing samples. Bucket uses the half-open window [bucket, bucket+5m).
// Gaps and counter resets are recorded explicitly and must not be hidden during aggregation.
type Traffic5m struct {
	ServerID   int64     `gorm:"column:server_id;primaryKey"`
	Iface      string    `gorm:"column:iface;size:64;primaryKey"`
	Bucket     time.Time `gorm:"column:bucket;primaryKey"`
	InBytes    int64     `gorm:"column:in_bytes"`
	OutBytes   int64     `gorm:"column:out_bytes"`
	CoveredSec float64   `gorm:"column:covered_seconds"` // 真实覆盖秒数；低覆盖桶不参与 95 带宽样本。

	InRateBytesPerSec  float64   `gorm:"column:in_rate_bytes_per_sec"`
	OutRateBytesPerSec float64   `gorm:"column:out_rate_bytes_per_sec"`
	InPeakBytesPerSec  float64   `gorm:"column:in_peak_bytes_per_sec"`
	OutPeakBytesPerSec float64   `gorm:"column:out_peak_bytes_per_sec"`
	SampleCount        int32     `gorm:"column:sample_count"`
	GapCount           int32     `gorm:"column:gap_count"`
	ResetCount         int32     `gorm:"column:reset_count"`
	CreatedAt          time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (Traffic5m) TableName() string { return "traffic_5m" }

// TrafficMonthUsage is the lightweight monthly accumulator per interface.
type TrafficMonthUsage struct {
	ServerID        int64     `gorm:"column:server_id;primaryKey"`
	Iface           string    `gorm:"column:iface;size:64;primaryKey"`
	CycleMode       string    `gorm:"column:cycle_mode;size:32;primaryKey"`
	BillingStartDay int16     `gorm:"column:billing_start_day;primaryKey"`
	Timezone        string    `gorm:"column:timezone;size:64"`
	CycleStart      time.Time `gorm:"column:cycle_start;primaryKey"`
	CycleEnd        time.Time `gorm:"column:cycle_end;primaryKey"`
	CoveredUntil    time.Time `gorm:"column:covered_until"`     // 本账期已累计到的业务时间。
	LastCollectedAt time.Time `gorm:"column:last_collected_at"` // 增量回填进度，防止重复累计同一对采样点。

	InBytes             int64     `gorm:"column:in_bytes"`
	OutBytes            int64     `gorm:"column:out_bytes"`
	InPeakBytesPerSec   float64   `gorm:"column:in_peak_bytes_per_sec"`
	OutPeakBytesPerSec  float64   `gorm:"column:out_peak_bytes_per_sec"`
	BothPeakBytesPerSec float64   `gorm:"column:both_peak_bytes_per_sec"`
	SampleCount         int32     `gorm:"column:sample_count"`
	GapCount            int32     `gorm:"column:gap_count"`
	ResetCount          int32     `gorm:"column:reset_count"`
	CreatedAt           time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt           time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (TrafficMonthUsage) TableName() string { return "traffic_month_usage" }

// TrafficMonthly is the persisted monthly snapshot used by historical billing reads.
// The current cycle is calculated live; old cycles prefer this table to avoid rescanning 5-minute samples.
type TrafficMonthly struct {
	ServerID        int64      `gorm:"column:server_id;primaryKey"`
	Iface           string     `gorm:"column:iface;size:64;primaryKey"`
	CycleMode       string     `gorm:"column:cycle_mode;size:32;primaryKey"`
	BillingStartDay int16      `gorm:"column:billing_start_day;primaryKey"`
	Timezone        string     `gorm:"column:timezone;size:64"`
	CycleStart      time.Time  `gorm:"column:cycle_start;primaryKey"`
	CycleEnd        time.Time  `gorm:"column:cycle_end;primaryKey"`
	Status          string     `gorm:"column:status;size:16"` // provisional/grace/sealed/stale，决定快照是否可复用。
	EffectiveStart  time.Time  `gorm:"column:effective_start"`
	EffectiveEnd    time.Time  `gorm:"column:effective_end"`
	CoveredUntil    time.Time  `gorm:"column:covered_until"` // 本快照实际覆盖到的源数据时间。
	GeneratedAt     time.Time  `gorm:"column:generated_at"`
	SealedAt        *time.Time `gorm:"column:sealed_at"` // sealed 快照的封存时间；非完整快照保持为空。

	InBytes             int64     `gorm:"column:in_bytes"`
	OutBytes            int64     `gorm:"column:out_bytes"`
	P95Enabled          bool      `gorm:"column:p95_enabled"`
	InP95BytesPerSec    float64   `gorm:"column:in_p95_bytes_per_sec"`
	OutP95BytesPerSec   float64   `gorm:"column:out_p95_bytes_per_sec"`
	BothP95BytesPerSec  float64   `gorm:"column:both_p95_bytes_per_sec"`
	InPeakBytesPerSec   float64   `gorm:"column:in_peak_bytes_per_sec"`
	OutPeakBytesPerSec  float64   `gorm:"column:out_peak_bytes_per_sec"`
	BothPeakBytesPerSec float64   `gorm:"column:both_peak_bytes_per_sec"`
	SampleCount         int32     `gorm:"column:sample_count"`
	ExpectedSampleCount int32     `gorm:"column:expected_sample_count"`
	CoverageRatio       float64   `gorm:"column:coverage_ratio"`
	GapCount            int32     `gorm:"column:gap_count"`
	ResetCount          int32     `gorm:"column:reset_count"`
	CreatedAt           time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt           time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (TrafficMonthly) TableName() string { return "traffic_monthly" }

// Service represents table services.
type Service struct {
	ID            int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Name          string         `gorm:"column:name;size:128;not null"`
	GroupID       *int64         `gorm:"column:group_id"`
	Type          ServiceType    `gorm:"column:type;type:service_type;not null"`
	Target        string         `gorm:"column:target;size:255;not null"`
	Port          *int32         `gorm:"column:port"`
	Region        *string        `gorm:"column:region"`
	IntervalSec   int32          `gorm:"column:interval_sec;not null"`
	TimeoutSec    int32          `gorm:"column:timeout_sec;not null"`
	Retry         int16          `gorm:"column:retry;not null"`
	HTTPMethod    *string        `gorm:"column:http_method"`
	HTTPHeaders   datatypes.JSON `gorm:"column:http_headers"`
	HTTPBody      *string        `gorm:"column:http_body"`
	ExpectStatus  *string        `gorm:"column:expect_status"`
	ExpectKeyword *string        `gorm:"column:expect_keyword"`
	Enabled       bool           `gorm:"column:enabled;not null"`
	IsDeleted     bool           `gorm:"column:is_deleted;not null;default:false"`
	Remark        *string        `gorm:"column:remark"`
	CreatedAt     time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time      `gorm:"column:updated_at;autoUpdateTime"`
}

func (Service) TableName() string { return "services" }

// ServiceCheck represents table service_checks.
type ServiceCheck struct {
	ServiceID     int64     `gorm:"column:service_id;not null;primaryKey"`
	ProbeServerID *int64    `gorm:"column:probe_server_id"`
	TS            time.Time `gorm:"column:ts;not null;primaryKey"`
	Status        int16     `gorm:"column:status;not null"`
	LatencyMs     *int32    `gorm:"column:latency_ms"`
	HTTPCode      *int32    `gorm:"column:http_code"`
	Result        *string   `gorm:"column:result"`
}

func (ServiceCheck) TableName() string { return "service_checks" }

// Task represents table tasks.
type Task struct {
	ID         int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Name       string         `gorm:"column:name;size:128;not null"`
	Type       TaskType       `gorm:"column:type;type:task_type;not null"`
	CronExpr   string         `gorm:"column:cron_expr;size:64;not null"`
	TimeoutSec int32          `gorm:"column:timeout_sec;not null"`
	Retries    int16          `gorm:"column:retries;not null"`
	TargetType TargetType     `gorm:"column:target_type;type:target_type;not null"`
	GroupID    *int64         `gorm:"column:group_id"`
	ServerIDs  datatypes.JSON `gorm:"column:server_ids"`
	Payload    string         `gorm:"column:payload;not null"`
	Enabled    bool           `gorm:"column:enabled;not null"`
	IsDeleted  bool           `gorm:"column:is_deleted;not null;default:false"`
	CreatedAt  time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time      `gorm:"column:updated_at;autoUpdateTime"`
}

func (Task) TableName() string { return "tasks" }

// TaskLog represents table task_logs.
type TaskLog struct {
	ID       int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TaskID   int64      `gorm:"column:task_id;not null"`
	ServerID int64      `gorm:"column:server_id;not null"`
	StartAt  time.Time  `gorm:"column:start_at;not null"`
	EndAt    *time.Time `gorm:"column:end_at"`
	Status   string     `gorm:"column:status;size:16;not null"`
	ExitCode *int32     `gorm:"column:exit_code"`
	Output   *string    `gorm:"column:output"`
}

func (TaskLog) TableName() string { return "task_logs" }

// NotifyChannel represents table notify_channels.
type NotifyChannel struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Name      string         `gorm:"column:name;size:64;not null"`
	Type      NotifyType     `gorm:"column:type;type:notify_type;not null"`
	Config    datatypes.JSON `gorm:"column:config;not null"` // 按 Type 存放不同渠道配置；更新时空 secret 表示保留旧 secret。
	Enabled   bool           `gorm:"column:enabled;not null"`
	IsDeleted bool           `gorm:"column:is_deleted;not null;default:false"`
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime"`
}

func (NotifyChannel) TableName() string { return "notify_channels" }

// AlertSetting represents table alert_settings.
type AlertSetting struct {
	ID         int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Scope      string         `gorm:"column:scope;size:32;not null"` // 当前只有 global；预留作用域不等于多实例。
	Enabled    bool           `gorm:"column:enabled;not null"`
	ChannelIDs datatypes.JSON `gorm:"column:channel_ids;not null"` // 全局默认通知渠道 ID 列表。
	CreatedAt  time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time      `gorm:"column:updated_at;autoUpdateTime"`
}

func (AlertSetting) TableName() string { return "alert_settings" }

type SystemSetting struct {
	ID            int16     `gorm:"column:id;primaryKey;autoIncrement:false"`
	ActiveThemeID string    `gorm:"column:active_theme_id;size:64;not null;default:''"` // 空值表示使用内置默认主题。
	LogoURL       string    `gorm:"column:logo_url;type:text;not null"`
	PageTitle     string    `gorm:"column:page_title;type:text;not null"`
	TopbarText    string    `gorm:"column:topbar_text;type:text;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (SystemSetting) TableName() string { return "system_settings" }

type MetricSetting struct {
	ID                     int16     `gorm:"column:id;primaryKey;autoIncrement:false"`
	HistoryGuestAccessMode string    `gorm:"column:history_guest_access_mode;size:16;not null;default:'disabled'"` // by_node 时仍受 Server.IsGuestVisible 限制。
	CreatedAt              time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt              time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (MetricSetting) TableName() string { return "metric_settings" }

type TrafficSetting struct {
	ID                int16     `gorm:"column:id;primaryKey;autoIncrement:false"`
	GuestAccessMode   string    `gorm:"column:guest_access_mode;size:16;not null;default:'disabled'"` // by_node 时仍受 Server.IsGuestVisible 限制。
	UsageMode         string    `gorm:"column:usage_mode;size:16;not null;default:'lite'"`
	CycleMode         string    `gorm:"column:cycle_mode;size:32;not null;default:'calendar_month'"`
	BillingStartDay   int16     `gorm:"column:billing_start_day;not null;default:1"`
	BillingAnchorDate string    `gorm:"column:billing_anchor_date;size:10;not null;default:''"` // WHMCS 兼容模式下固定账期锚点，优先于 billing_start_day。
	BillingTimezone   string    `gorm:"column:billing_timezone;size:64;not null;default:''"`    // 空值表示使用应用时区。
	DirectionMode     string    `gorm:"column:direction_mode;size:16;not null;default:'out'"`   // 计费展示方向，不改变原始入/出流量存储。
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (TrafficSetting) TableName() string { return "traffic_settings" }

// AlertRule represents table alert_rules.
type AlertRule struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Name            string    `gorm:"column:name;size:128;not null"`
	Enabled         bool      `gorm:"column:enabled;not null"`
	Generation      int64     `gorm:"column:generation;not null"` // 规则语义版本；变更后用它关闭旧事件，避免新旧规则串线。
	Metric          string    `gorm:"column:metric;size:64;not null"`
	Operator        string    `gorm:"column:operator;size:8;not null"`
	Threshold       float64   `gorm:"column:threshold;not null"`
	DurationSec     int32     `gorm:"column:duration_sec;not null"`
	CooldownMin     int32     `gorm:"column:cooldown_min;not null"`
	ThresholdMode   string    `gorm:"column:threshold_mode;size:16;not null"`
	ThresholdOffset float64   `gorm:"column:threshold_offset;not null"`
	IsDeleted       bool      `gorm:"column:is_deleted;not null;default:false"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (AlertRule) TableName() string { return "alert_rules" }

// AlertRuleMount represents per-server rule mount overrides.
type AlertRuleMount struct {
	RuleID    int64     `gorm:"column:rule_id;primaryKey"`
	ServerID  int64     `gorm:"column:server_id;primaryKey"`
	Enabled   bool      `gorm:"column:enabled;not null"` // 覆盖规则默认挂载状态；没有记录时按规则默认值处理。
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (AlertRuleMount) TableName() string { return "alert_rule_mounts" }

// AlertEvent represents table alert_events.
type AlertEvent struct {
	ID                 int64          `gorm:"column:id;primaryKey;autoIncrement"`
	RuleID             int64          `gorm:"column:rule_id;not null"`
	RuleGeneration     int64          `gorm:"column:rule_generation;not null"` // 事件绑定触发时的规则版本，关闭时不能跨版本误配。
	RuleSnapshot       datatypes.JSON `gorm:"column:rule_snapshot;not null"`   // 触发时的规则快照；历史事件不依赖后来规则改名或改阈值。
	ObjectType         ObjectType     `gorm:"column:object_type;type:object_type;not null"`
	ObjectID           int64          `gorm:"column:object_id;not null"`
	Status             AlertStatus    `gorm:"column:status;type:alert_status;not null"`
	FirstTriggerAt     time.Time      `gorm:"column:first_trigger_at;not null"`
	LastTriggerAt      time.Time      `gorm:"column:last_trigger_at;not null"`
	ClosedAt           *time.Time     `gorm:"column:closed_at"`
	CurrentValue       *float64       `gorm:"column:current_value"`
	EffectiveThreshold *float64       `gorm:"column:effective_threshold"`
	CloseReason        *string        `gorm:"column:close_reason"`
	Title              *string        `gorm:"column:title"`
	Message            *string        `gorm:"column:message"`
}

func (AlertEvent) TableName() string { return "alert_events" }

// AlertNotificationOutbox represents table alert_notification_outbox.
type AlertNotificationOutbox struct {
	ID            int64          `gorm:"column:id;primaryKey;autoIncrement"`
	EventID       int64          `gorm:"column:event_id;not null"`
	Transition    string         `gorm:"column:transition;size:16;not null"` // open/close；同一事件不同 transition 要分别投递。
	ChannelID     int64          `gorm:"column:channel_id;not null"`
	ChannelType   NotifyType     `gorm:"column:channel_type;type:notify_type;not null"`
	Payload       datatypes.JSON `gorm:"column:payload;not null"`             // 已渲染通知内容；发送器不再读当前规则重建文案。
	DedupeKey     string         `gorm:"column:dedupe_key;size:255;not null"` // 幂等键，防止控制流重试造成重复通知。
	Status        OutboxStatus   `gorm:"column:status;size:32;not null"`
	AttemptCount  int32          `gorm:"column:attempt_count;not null"`
	NextAttemptAt time.Time      `gorm:"column:next_attempt_at;not null"`
	LastError     *string        `gorm:"column:last_error"`
	LeasedUntil   *time.Time     `gorm:"column:leased_until"`
	CreatedAt     time.Time      `gorm:"column:created_at;autoCreateTime"`
	SentAt        *time.Time     `gorm:"column:sent_at"`
}

func (AlertNotificationOutbox) TableName() string { return "alert_notification_outbox" }

// AlertControlTask represents table alert_control_tasks.
type AlertControlTask struct {
	ID           int64          `gorm:"column:id;primaryKey;autoIncrement"`
	TaskType     string         `gorm:"column:task_type;size:32;not null"`
	DedupeKey    string         `gorm:"column:dedupe_key;size:255;not null"` // 控制任务去重键；规则同一代只需要一个重算任务。
	Payload      datatypes.JSON `gorm:"column:payload;not null"`             // worker 只按 payload 执行，不回读请求上下文。
	Status       TaskStatus     `gorm:"column:status;size:16;not null"`
	AttemptCount int32          `gorm:"column:attempt_count;not null"`
	AvailableAt  time.Time      `gorm:"column:available_at;not null"`
	LeasedUntil  *time.Time     `gorm:"column:leased_until"`
	LastError    *string        `gorm:"column:last_error"`
	CreatedAt    time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;autoUpdateTime"`
}

func (AlertControlTask) TableName() string { return "alert_control_tasks" }
