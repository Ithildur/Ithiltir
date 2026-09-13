package model

import (
	"time"

	"gorm.io/datatypes"
)

type ServiceType string

const (
	ServiceTypeHTTP ServiceType = "http"
	ServiceTypeTCP  ServiceType = "tcp"
	ServiceTypePing ServiceType = "ping"
	ServiceTypeDNS  ServiceType = "dns"
	ServiceTypeTLS  ServiceType = "tls"
)

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
