package model

import (
	"time"

	"gorm.io/datatypes"
)

type NotifyType string

const (
	NotifyTypeTelegram NotifyType = "telegram"
	NotifyTypeEmail    NotifyType = "email"
	NotifyTypeWebhook  NotifyType = "webhook"
	NotifyTypeWeChat   NotifyType = "wechat"
	NotifyTypeSlack    NotifyType = "slack"
	NotifyTypeDiscord  NotifyType = "discord"
)

// NotifyChannel represents table notify_channels.
type NotifyChannel struct {
	ID                  int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Name                string         `gorm:"column:name;size:64;not null"`
	Type                NotifyType     `gorm:"column:type;type:notify_type;not null"`
	Config              datatypes.JSON `gorm:"-"` // 仅存在于 Dash 内存；持久化由通知 store 在加密边界完成。
	Enabled             bool           `gorm:"column:enabled;not null"`
	IsDeleted           bool           `gorm:"column:is_deleted;not null;default:false"`
	Revision            int64          `gorm:"column:revision;not null;default:1"`
	LastSuccessAt       *time.Time     `gorm:"column:last_success_at"`
	LastFailureAt       *time.Time     `gorm:"column:last_failure_at"`
	ConsecutiveFailures int32          `gorm:"column:consecutive_failures;not null;default:0"`
	LastErrorCode       *string        `gorm:"column:last_error_code;size:64"`
	LastError           *string        `gorm:"column:last_error"`
	ConfigUpdatedAt     time.Time      `gorm:"column:config_updated_at;autoCreateTime"`
	CreatedAt           time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt           time.Time      `gorm:"column:updated_at;autoUpdateTime"`
}

func (NotifyChannel) TableName() string { return "notify_channels" }
