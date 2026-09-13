package model

import (
	"time"

	"gorm.io/datatypes"
)

type TaskType string

type TargetType string

const (
	TaskTypeShell    TaskType   = "shell"
	TaskTypeHTTP     TaskType   = "http"
	TaskTypeScript   TaskType   = "script"
	TargetTypeAll    TargetType = "all"
	TargetTypeGroup  TargetType = "group"
	TargetTypeServer TargetType = "server"
	TargetTypeCustom TargetType = "custom"
)

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
