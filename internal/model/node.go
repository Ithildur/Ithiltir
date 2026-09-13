package model

import (
	"time"

	"gorm.io/datatypes"
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
	Name                     string         `gorm:"column:name;size:255;not null"`
	Hostname                 string         `gorm:"column:hostname;size:255;not null"`
	Secret                   string         `gorm:"column:secret;size:128;not null;unique"` // 节点 agent 鉴权密钥，访客接口不能暴露。
	Tags                     datatypes.JSON `gorm:"column:tags"`
	IP                       *string        `gorm:"column:ip"`
	OS                       *string        `gorm:"column:os;size:32"`
	Platform                 *string        `gorm:"column:platform;size:32"`
	PlatformVersion          *string        `gorm:"column:platform_version;size:255"`
	KernelVersion            *string        `gorm:"column:kernel_version;size:255"`
	Arch                     *string        `gorm:"column:arch;size:32"`
	Location                 *string        `gorm:"column:location"`
	CPUModel                 *string        `gorm:"column:cpu_model;type:text"`
	CPUVendor                *string        `gorm:"column:cpu_vendor;type:text"`
	CPUCoresPhys             *int16         `gorm:"column:cpu_cores_physical"`
	CPUCoresLog              *int16         `gorm:"column:cpu_cores_logical"`
	CPUSockets               *int16         `gorm:"column:cpu_sockets"`
	CPUMhz                   *float64       `gorm:"column:cpu_mhz"`
	MemTotal                 *int64         `gorm:"column:mem_total"`
	SwapTotal                *int64         `gorm:"column:swap_total"`
	DiskTotal                *int64         `gorm:"column:disk_total"`
	RootPath                 *string        `gorm:"column:root_path;type:text"` // 前端根磁盘展示入口，通常来自最大逻辑盘挂载点。
	RootFSType               *string        `gorm:"column:root_fs_type;size:32"`
	RaidSupported            *bool          `gorm:"column:raid_supported"`
	RaidAvailable            *bool          `gorm:"column:raid_available"`
	IntervalSec              *int32         `gorm:"column:interval_sec"`
	IsGuestVisible           bool           `gorm:"column:is_guest_visible;not null;default:false"`    // 访客读取历史指标和流量时按它过滤。
	TrafficP95Enabled        bool           `gorm:"column:traffic_p95_enabled;not null;default:false"` // 高级计费模式下是否为该节点计算 95 带宽。
	TrafficCycleMode         string         `gorm:"column:traffic_cycle_mode;size:32;not null;default:'calendar_month'"`
	TrafficBillingStartDay   int16          `gorm:"column:traffic_billing_start_day;not null;default:1"`
	TrafficBillingAnchorDate string         `gorm:"column:traffic_billing_anchor_date;size:10;not null;default:''"`
	TrafficBillingTimezone   string         `gorm:"column:traffic_billing_timezone;size:64;not null;default:''"`
	TrafficDirectionMode     string         `gorm:"column:traffic_direction_mode;size:16;not null;default:'default'"`
	IsDeleted                bool           `gorm:"column:is_deleted;not null;default:false"`
	AgentVersion             *string        `gorm:"column:agent_version;size:64"`
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
