package model

import (
	"time"

	"gorm.io/datatypes"
)

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
	ID                 int16     `gorm:"column:id;primaryKey;autoIncrement:false"`
	ActiveThemeID      string    `gorm:"column:active_theme_id;size:64;not null;default:''"`            // 空值表示使用内置默认主题。
	DashUpdateChannel  string    `gorm:"column:dash_update_channel;size:16;not null;default:'release'"` // release 或 prerelease。
	DashUpdateMode     string    `gorm:"column:dash_update_mode;size:16;not null;default:'manual'"`     // manual、notify 或 auto。
	UptimeGuestVisible bool      `gorm:"column:uptime_guest_visible;not null;default:false"`
	UptimeWarningSLA   float64   `gorm:"column:uptime_warning_sla;not null;default:99"`
	UptimeErrorSLA     float64   `gorm:"column:uptime_error_sla;not null;default:95"`
	LogoURL            string    `gorm:"column:logo_url;type:text;not null"`
	PageTitle          string    `gorm:"column:page_title;type:text;not null"`
	TopbarText         string    `gorm:"column:topbar_text;type:text;not null"`
	CreatedAt          time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time `gorm:"column:updated_at;autoUpdateTime"`
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
