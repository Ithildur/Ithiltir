package model

import (
	"time"
)

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
	CoveredFrom     time.Time `gorm:"column:covered_from"`      // 本账期实际纳入统计的起点。
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

// TrafficMaterializationProgress is the durable scan high-water mark for one
// of the two fixed traffic materializers.
type TrafficMaterializationProgress struct {
	Kind         string    `gorm:"column:kind;size:16;primaryKey"`
	ScannedUntil time.Time `gorm:"column:scanned_until"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (TrafficMaterializationProgress) TableName() string {
	return "traffic_materialization_progress"
}

// TrafficUsageRepair is a short-lived per-node repair cursor created by an
// immediate billing-cycle change. Its presence excludes the node from the live
// Usage scan until retained raw samples have been replayed.
type TrafficUsageRepair struct {
	ServerID     int64     `gorm:"column:server_id;primaryKey"`
	ScannedUntil time.Time `gorm:"column:scanned_until"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (TrafficUsageRepair) TableName() string {
	return "traffic_usage_repairs"
}

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
