package model

import (
	"time"

	"gorm.io/datatypes"
)

type ObjectType string

type AlertStatus string

type OutboxStatus string

type TaskStatus string

const (
	ObjectTypeServer      ObjectType   = "server"
	ObjectTypeService     ObjectType   = "service"
	AlertStatusOpen       AlertStatus  = "open"
	AlertStatusClosed     AlertStatus  = "closed"
	OutboxStatusPending   OutboxStatus = "pending"
	OutboxStatusSending   OutboxStatus = "sending"
	OutboxStatusSent      OutboxStatus = "sent"
	OutboxStatusRetry     OutboxStatus = "retry"
	OutboxStatusBlocked   OutboxStatus = "blocked"
	OutboxStatusPaused    OutboxStatus = "paused"
	OutboxStatusDiscarded OutboxStatus = "discarded"
	TaskStatusPending     TaskStatus   = "pending"
	TaskStatusFailed      TaskStatus   = "failed"
)

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
	EventID       *int64         `gorm:"column:event_id"`
	Transition    string         `gorm:"column:transition;size:32;not null"` // 通知事件；告警使用 opened/closed，系统通知使用自己的稳定事件名。
	ChannelID     int64          `gorm:"column:channel_id;not null"`
	ChannelType   NotifyType     `gorm:"column:channel_type;type:notify_type;not null"`
	Payload       datatypes.JSON `gorm:"column:payload;not null"`             // 已渲染通知内容；发送器不再读当前规则重建文案。
	DedupeKey     string         `gorm:"column:dedupe_key;size:255;not null"` // 幂等键，防止控制流重试造成重复通知。
	Status        OutboxStatus   `gorm:"column:status;size:32;not null"`
	AttemptCount  int32          `gorm:"column:attempt_count;not null"`
	ProbeCount    int32          `gorm:"column:probe_count;not null"`
	IsProbe       bool           `gorm:"column:is_probe;not null"`
	NextAttemptAt time.Time      `gorm:"column:next_attempt_at;not null"`
	FailureCode   *string        `gorm:"column:failure_code;size:64"`
	LastError     *string        `gorm:"column:last_error"`
	LeasedUntil   *time.Time     `gorm:"column:leased_until"` // 历史 schema 兼容字段；当前单实例 worker 不使用租约。
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
	LeasedUntil  *time.Time     `gorm:"column:leased_until"` // 历史 schema 兼容字段；当前单实例 worker 不按租约截止时间取件。
	LastError    *string        `gorm:"column:last_error"`
	CreatedAt    time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;autoUpdateTime"`
}

func (AlertControlTask) TableName() string { return "alert_control_tasks" }
