package traffic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"dash/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	trafficBucketSize       = 5 * time.Minute
	trafficBackfillWindow   = 15 * time.Minute
	trafficCatchupWindow    = time.Hour
	trafficSnapshotGrace    = 48 * time.Hour
	trafficMaxMonthlyMonths = 24
	trafficMinCoveredSec    = 270.0
	trafficMaxBillingGap    = trafficBucketSize
	trafficMinP95Samples    = 20
)

var ErrNoTrafficData = errors.New("no traffic data")

type TrafficPeriod string
type TrafficSnapshotStatus string
type TrafficP95Status string
type TrafficDirection string

const (
	TrafficPeriodCurrent TrafficPeriod = "current"
	TrafficPeriodPrev    TrafficPeriod = "previous"

	TrafficSnapshotProvisional TrafficSnapshotStatus = "provisional"
	TrafficSnapshotGrace       TrafficSnapshotStatus = "grace"
	TrafficSnapshotSealed      TrafficSnapshotStatus = "sealed"
	TrafficSnapshotStale       TrafficSnapshotStatus = "stale"

	TrafficP95Available           TrafficP95Status = "available"
	TrafficP95Disabled            TrafficP95Status = "disabled"
	TrafficP95LiteMode            TrafficP95Status = "lite_mode"
	TrafficP95InsufficientSamples TrafficP95Status = "insufficient_samples"
	TrafficP95SnapshotMissing     TrafficP95Status = "snapshot_without_p95"

	TrafficDirectionNone   TrafficDirection = ""
	TrafficDirectionInKey  TrafficDirection = "in"
	TrafficDirectionOutKey TrafficDirection = "out"
	TrafficDirectionTotal  TrafficDirection = "total"
)

type TrafficQuery struct {
	ServerID          int64
	Iface             string
	UsageMode         UsageMode
	CycleMode         BillingCycleMode
	BillingStartDay   int
	BillingAnchorDate string
	DirectionMode     DirectionMode
	P95Enabled        bool
	Location          *time.Location
	Ref               time.Time
	Period            TrafficPeriod
}

type TrafficMonthlyQuery struct {
	ServerID          int64
	Iface             string
	UsageMode         UsageMode
	CycleMode         BillingCycleMode
	BillingStartDay   int
	BillingAnchorDate string
	DirectionMode     DirectionMode
	P95Enabled        bool
	Location          *time.Location
	Ref               time.Time
	Months            int
	Period            TrafficPeriod
}

type TrafficIface struct {
	Name string `json:"name"`
}

func (s *Store) TrafficServerName(ctx context.Context, serverID int64) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("store: db is nil")
	}
	if serverID <= 0 {
		return "", fmt.Errorf("invalid server id")
	}
	var row struct {
		Name string `gorm:"column:name"`
	}
	err := s.db.WithContext(ctx).
		Model(&model.Server{}).
		Select("name").
		Where("id = ? AND is_deleted = ?", serverID, false).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(row.Name), nil
}

type TrafficCycle struct {
	Mode              BillingCycleMode `json:"mode"`
	BillingStartDay   int              `json:"billing_start_day"`
	BillingAnchorDate string           `json:"billing_anchor_date,omitempty"`
	Timezone          string           `json:"timezone"`
	Start             time.Time        `json:"start"`
	End               time.Time        `json:"end"`
}

type TrafficStat struct {
	InBytes                 int64
	OutBytes                int64
	P95Enabled              bool
	P95Status               TrafficP95Status
	P95UnavailableReason    string
	InP95BytesPerSec        float64
	OutP95BytesPerSec       float64
	BothP95BytesPerSec      float64
	InPeakBytesPerSec       float64
	OutPeakBytesPerSec      float64
	BothPeakBytesPerSec     float64
	SelectedP95BytesPerSec  float64
	SelectedPeakBytesPerSec float64
	SelectedBytesDirection  TrafficDirection
	SelectedP95Direction    TrafficDirection
	SelectedPeakDirection   TrafficDirection
	SelectedBytes           int64
	SampleCount             int
	ExpectedSampleCount     int
	EffectiveStart          time.Time
	EffectiveEnd            time.Time
	CoverageRatio           float64
	CoveredUntil            time.Time
	GapCount                int
	ResetCount              int
	CycleComplete           bool
	DataComplete            bool
	Status                  TrafficSnapshotStatus
	Partial                 bool
}

type TrafficSummary struct {
	ServerID  int64
	Iface     string
	UsageMode UsageMode
	Cycle     TrafficCycle
	Stat      TrafficStat
}

type trafficBucket struct {
	Bucket             time.Time `gorm:"column:bucket"`
	InBytes            int64     `gorm:"column:in_bytes"`
	OutBytes           int64     `gorm:"column:out_bytes"`
	InRateBytesPerSec  float64   `gorm:"column:in_rate_bytes_per_sec"`
	OutRateBytesPerSec float64   `gorm:"column:out_rate_bytes_per_sec"`
	InPeakBytesPerSec  float64   `gorm:"column:in_peak_bytes_per_sec"`
	OutPeakBytesPerSec float64   `gorm:"column:out_peak_bytes_per_sec"`
	SampleCount        int       `gorm:"column:sample_count"`
	GapCount           int       `gorm:"column:gap_count"`
	ResetCount         int       `gorm:"column:reset_count"`
}

func (s *Store) ListTrafficIfaces(ctx context.Context, serverID int64) ([]TrafficIface, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store: db is nil")
	}
	if serverID <= 0 {
		return nil, fmt.Errorf("invalid server id")
	}
	var rows []TrafficIface
	if err := s.db.WithContext(ctx).
		Table("nic_metrics").
		Distinct("iface AS name").
		Where("server_id = ?", serverID).
		Order("name ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) TrafficP95Enabled(ctx context.Context, serverID int64) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("store: db is nil")
	}
	if serverID <= 0 {
		return false, fmt.Errorf("invalid server id")
	}
	var row struct {
		Enabled bool `gorm:"column:traffic_p95_enabled"`
	}
	err := s.db.WithContext(ctx).
		Model(&model.Server{}).
		Select("traffic_p95_enabled").
		Where("id = ? AND is_deleted = ?", serverID, false).
		Take(&row).Error
	return row.Enabled, err
}

func (s *Store) ServerCycleSettings(ctx context.Context, serverID int64) (ServerCycleSettings, error) {
	if s == nil || s.db == nil {
		return ServerCycleSettings{Mode: ServerCycleDefault}, fmt.Errorf("store: db is nil")
	}
	if serverID <= 0 {
		return ServerCycleSettings{Mode: ServerCycleDefault}, fmt.Errorf("invalid server id")
	}
	var row struct {
		Mode              string `gorm:"column:traffic_cycle_mode"`
		BillingStartDay   int16  `gorm:"column:traffic_billing_start_day"`
		BillingAnchorDate string `gorm:"column:traffic_billing_anchor_date"`
		BillingTimezone   string `gorm:"column:traffic_billing_timezone"`
	}
	err := s.db.WithContext(ctx).
		Model(&model.Server{}).
		Select("traffic_cycle_mode", "traffic_billing_start_day", "traffic_billing_anchor_date", "traffic_billing_timezone").
		Where("id = ? AND is_deleted = ?", serverID, false).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ServerCycleSettings{Mode: ServerCycleDefault}, nil
		}
		return ServerCycleSettings{Mode: ServerCycleDefault}, err
	}
	cycle, err := NormalizeServerCycleSettings(ServerCycleSettings{
		Mode:              ServerCycleMode(row.Mode),
		BillingStartDay:   int(row.BillingStartDay),
		BillingAnchorDate: strings.TrimSpace(row.BillingAnchorDate),
		BillingTimezone:   strings.TrimSpace(row.BillingTimezone),
	})
	if err != nil {
		return ServerCycleSettings{Mode: ServerCycleDefault}, nil
	}
	return cycle, nil
}

func (s *Store) TrafficSummary(ctx context.Context, q TrafficQuery) (TrafficSummary, error) {
	if s == nil || s.db == nil {
		return TrafficSummary{}, fmt.Errorf("store: db is nil")
	}
	q = normalizeTrafficQuery(q)
	if q.ServerID <= 0 {
		return TrafficSummary{}, fmt.Errorf("invalid server id")
	}
	if !validTrafficIface(q.Iface) {
		return TrafficSummary{}, fmt.Errorf("invalid iface")
	}

	cycle := currentTrafficCycleAnchored(q.CycleMode, q.BillingStartDay, q.BillingAnchorDate, q.Location, q.Ref)
	if q.UsageMode == UsageLite {
		return s.trafficUsageSummaryForCycle(ctx, q, cycle)
	}
	return s.trafficSummaryForCycle(ctx, q, cycle)
}

func (s *Store) TrafficMonthly(ctx context.Context, q TrafficMonthlyQuery) ([]TrafficSummary, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store: db is nil")
	}
	if q.Months <= 0 {
		q.Months = 6
	}
	if q.Months > trafficMaxMonthlyMonths {
		q.Months = trafficMaxMonthlyMonths
	}

	base := normalizeTrafficQuery(TrafficQuery{
		ServerID:          q.ServerID,
		Iface:             q.Iface,
		UsageMode:         q.UsageMode,
		CycleMode:         q.CycleMode,
		BillingStartDay:   q.BillingStartDay,
		BillingAnchorDate: q.BillingAnchorDate,
		DirectionMode:     q.DirectionMode,
		P95Enabled:        q.P95Enabled,
		Location:          q.Location,
		Ref:               q.Ref,
		Period:            q.Period,
	})
	if base.ServerID <= 0 {
		return nil, fmt.Errorf("invalid server id")
	}
	if !validTrafficIface(base.Iface) {
		return nil, fmt.Errorf("invalid iface")
	}

	cycle := trafficCycleForPeriod(base.CycleMode, base.BillingStartDay, base.BillingAnchorDate, base.Location, base.Ref, base.Period)
	out := make([]TrafficSummary, 0, q.Months)
	for i := 0; i < q.Months; i++ {
		if base.UsageMode == UsageLite {
			summary, err := s.trafficUsageSummaryForCycle(ctx, base, cycle)
			if err != nil && !errors.Is(err, ErrNoTrafficData) {
				return nil, err
			}
			if err == nil {
				out = append(out, summary)
			}
		} else if base.Period == TrafficPeriodCurrent && i == 0 {
			summary, err := s.trafficSummaryForCycle(ctx, base, cycle)
			if err != nil && !errors.Is(err, ErrNoTrafficData) {
				return nil, err
			}
			if err == nil {
				out = append(out, summary)
			}
		} else {
			summary, err := s.trafficMonthlyForCycle(ctx, base, cycle)
			if err != nil && !errors.Is(err, ErrNoTrafficData) {
				return nil, err
			}
			if err == nil {
				out = append(out, summary)
			}
		}
		cycle = prevTrafficCycle(cycle)
	}
	return out, nil
}

func (s *Store) trafficMonthlyForCycle(ctx context.Context, q TrafficQuery, cycle TrafficCycle) (TrafficSummary, error) {
	summary, ok, err := s.trafficMonthlySnapshot(ctx, q, cycle)
	if err != nil {
		return TrafficSummary{}, err
	}
	if ok {
		return summary, nil
	}

	return s.trafficSummaryForCycle(ctx, q, cycle)
}

func (s *Store) saveTrafficMonthly(ctx context.Context, summary TrafficSummary) error {
	if !summary.Stat.CycleComplete || summary.Stat.Status == TrafficSnapshotProvisional {
		return nil
	}
	now := time.Now().UTC()
	var sealedAt *time.Time
	if summary.Stat.Status == TrafficSnapshotSealed {
		sealedAt = &now
	}
	row := model.TrafficMonthly{
		ServerID:            summary.ServerID,
		Iface:               summary.Iface,
		CycleMode:           string(summary.Cycle.Mode),
		BillingStartDay:     int16(summary.Cycle.BillingStartDay),
		Timezone:            summary.Cycle.Timezone,
		CycleStart:          summary.Cycle.Start,
		CycleEnd:            summary.Cycle.End,
		Status:              string(summary.Stat.Status),
		EffectiveStart:      summary.Stat.EffectiveStart,
		EffectiveEnd:        summary.Stat.EffectiveEnd,
		CoveredUntil:        summary.Stat.CoveredUntil,
		GeneratedAt:         now,
		SealedAt:            sealedAt,
		InBytes:             summary.Stat.InBytes,
		OutBytes:            summary.Stat.OutBytes,
		P95Enabled:          summary.Stat.P95Status == TrafficP95Available,
		InP95BytesPerSec:    summary.Stat.InP95BytesPerSec,
		OutP95BytesPerSec:   summary.Stat.OutP95BytesPerSec,
		BothP95BytesPerSec:  summary.Stat.BothP95BytesPerSec,
		InPeakBytesPerSec:   summary.Stat.InPeakBytesPerSec,
		OutPeakBytesPerSec:  summary.Stat.OutPeakBytesPerSec,
		BothPeakBytesPerSec: summary.Stat.BothPeakBytesPerSec,
		SampleCount:         int32(summary.Stat.SampleCount),
		ExpectedSampleCount: int32(summary.Stat.ExpectedSampleCount),
		CoverageRatio:       summary.Stat.CoverageRatio,
		GapCount:            int32(summary.Stat.GapCount),
		ResetCount:          int32(summary.Stat.ResetCount),
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "server_id"},
			{Name: "iface"},
			{Name: "cycle_mode"},
			{Name: "billing_start_day"},
			{Name: "cycle_start"},
			{Name: "cycle_end"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"status",
			"effective_start",
			"effective_end",
			"covered_until",
			"generated_at",
			"sealed_at",
			"in_bytes",
			"out_bytes",
			"p95_enabled",
			"in_p95_bytes_per_sec",
			"out_p95_bytes_per_sec",
			"both_p95_bytes_per_sec",
			"in_peak_bytes_per_sec",
			"out_peak_bytes_per_sec",
			"both_peak_bytes_per_sec",
			"sample_count",
			"expected_sample_count",
			"coverage_ratio",
			"gap_count",
			"reset_count",
		}),
	}).Create(&row).Error
}

type trafficMonthlyCandidate struct {
	ServerID          int64  `gorm:"column:server_id"`
	Iface             string `gorm:"column:iface"`
	CycleMode         string `gorm:"column:traffic_cycle_mode"`
	BillingStartDay   int    `gorm:"column:traffic_billing_start_day"`
	BillingAnchorDate string `gorm:"column:traffic_billing_anchor_date"`
	BillingTimezone   string `gorm:"column:traffic_billing_timezone"`
}

type trafficMonthlySettingsKey struct {
	cycleMode         BillingCycleMode
	billingStartDay   int
	billingAnchorDate string
	billingTimezone   string
}

func (s *Store) RefreshTrafficMonthlySnapshots(ctx context.Context, settings Settings, loc *time.Location, ref time.Time, lookback time.Duration) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store: db is nil")
	}
	var ok bool
	settings, ok = NormalizeSettings(settings)
	if !ok {
		return fmt.Errorf("invalid traffic settings")
	}
	if settings.UsageMode != UsageBilling {
		return nil
	}
	if loc == nil {
		loc = time.Local
	}
	if ref.IsZero() {
		ref = time.Now().In(loc)
	}
	if lookback <= 0 {
		lookback = time.Duration(trafficMaxMonthlyMonths) * 31 * 24 * time.Hour
	}

	cycleSettings, err := s.trafficMonthlyCycleSettings(ctx, settings)
	if err != nil {
		return err
	}
	for _, cycleSetting := range cycleSettings {
		cycleLoc := SettingsLocation(cycleSetting, loc)
		since := ref.In(cycleLoc).Add(-lookback)
		for _, cycle := range closedTrafficCycles(cycleSetting.CycleMode, cycleSetting.BillingStartDay, cycleSetting.BillingAnchorDate, cycleLoc, ref, since) {
			candidates, err := s.trafficMonthlyCandidates(ctx, settings, cycleSetting, cycle)
			if err != nil {
				return err
			}
			for _, candidate := range candidates {
				sourceComplete := !cycle.Start.Before(since.UTC())
				sealed, err := s.trafficMonthlySealedExists(ctx, candidate, cycle, sourceComplete)
				if err != nil {
					return err
				}
				if sealed {
					continue
				}
				q := normalizeTrafficQuery(TrafficQuery{
					ServerID:          candidate.ServerID,
					Iface:             candidate.Iface,
					UsageMode:         UsageBilling,
					CycleMode:         cycleSetting.CycleMode,
					BillingStartDay:   cycleSetting.BillingStartDay,
					BillingAnchorDate: cycleSetting.BillingAnchorDate,
					DirectionMode:     cycleSetting.DirectionMode,
					P95Enabled:        true,
					Location:          cycleLoc,
					Ref:               ref,
				})
				summary, err := s.trafficSummaryForCycle(ctx, q, cycle)
				if errors.Is(err, ErrNoTrafficData) {
					continue
				}
				if err != nil {
					return err
				}
				summary = trafficMonthlySaveable(summary, sourceComplete)
				if err := s.saveTrafficMonthly(ctx, summary); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *Store) trafficMonthlyCycleSettings(ctx context.Context, global Settings) ([]Settings, error) {
	var rows []trafficMonthlyCandidate
	if err := s.db.WithContext(ctx).
		Model(&model.Server{}).
		Select("traffic_cycle_mode", "traffic_billing_start_day", "traffic_billing_anchor_date", "traffic_billing_timezone").
		Where("is_deleted = ?", false).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	seen := make(map[trafficMonthlySettingsKey]struct{}, len(rows)+1)
	out := make([]Settings, 0, len(rows)+1)
	for _, row := range rows {
		settings := SettingsWithServerCycleSettings(global, row.serverCycleSettings())
		key := trafficMonthlySettingsKeyFrom(settings)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, settings)
	}
	if len(out) == 0 {
		key := trafficMonthlySettingsKeyFrom(global)
		seen[key] = struct{}{}
		out = append(out, global)
	}
	return out, nil
}

func (s *Store) trafficMonthlyCandidates(ctx context.Context, global, target Settings, cycle TrafficCycle) ([]trafficMonthlyCandidate, error) {
	var rows []trafficMonthlyCandidate
	if err := s.db.WithContext(ctx).
		Table("traffic_5m AS t").
		Select(`DISTINCT
			t.server_id,
			t.iface,
			COALESCE(NULLIF(s.traffic_cycle_mode, ''), 'default') AS traffic_cycle_mode,
			COALESCE(s.traffic_billing_start_day, 1) AS traffic_billing_start_day,
			COALESCE(s.traffic_billing_anchor_date, '') AS traffic_billing_anchor_date,
			COALESCE(s.traffic_billing_timezone, '') AS traffic_billing_timezone`).
		Joins("JOIN servers AS s ON s.id = t.server_id AND s.is_deleted = FALSE").
		Where("t.bucket >= ? AND t.bucket < ?", cycle.Start, cycle.End).
		Order("t.server_id ASC, t.iface ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]trafficMonthlyCandidate, 0, len(rows))
	targetKey := trafficMonthlySettingsKeyFrom(target)
	for _, row := range rows {
		effective := SettingsWithServerCycleSettings(global, row.serverCycleSettings())
		if trafficMonthlySettingsKeyFrom(effective) != targetKey {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func (c trafficMonthlyCandidate) serverCycleSettings() ServerCycleSettings {
	return ServerCycleSettings{
		Mode:              ServerCycleMode(c.CycleMode),
		BillingStartDay:   c.BillingStartDay,
		BillingAnchorDate: strings.TrimSpace(c.BillingAnchorDate),
		BillingTimezone:   strings.TrimSpace(c.BillingTimezone),
	}
}

func trafficMonthlySettingsKeyFrom(settings Settings) trafficMonthlySettingsKey {
	return trafficMonthlySettingsKey{
		cycleMode:         settings.CycleMode,
		billingStartDay:   settings.BillingStartDay,
		billingAnchorDate: strings.TrimSpace(settings.BillingAnchorDate),
		billingTimezone:   strings.TrimSpace(settings.BillingTimezone),
	}
}

func trafficMonthlySaveable(summary TrafficSummary, sourceComplete bool) TrafficSummary {
	if summary.Stat.Status == TrafficSnapshotSealed && !summary.Stat.DataComplete {
		if sourceComplete {
			summary.Stat.Status = TrafficSnapshotGrace
		} else {
			summary.Stat.Status = TrafficSnapshotStale
		}
	}
	return summary
}

func (s *Store) trafficMonthlySealedExists(ctx context.Context, candidate trafficMonthlyCandidate, cycle TrafficCycle, sourceComplete bool) (bool, error) {
	var row model.TrafficMonthly
	err := s.db.WithContext(ctx).
		Select("status", "sample_count", "expected_sample_count", "gap_count", "reset_count").
		Where("server_id = ?", candidate.ServerID).
		Where("iface = ?", candidate.Iface).
		Where("cycle_mode = ?", string(cycle.Mode)).
		Where("billing_start_day = ?", cycle.BillingStartDay).
		Where("cycle_start = ? AND cycle_end = ?", cycle.Start, cycle.End).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return trafficMonthlySnapshotReusable(row, sourceComplete)
}

func trafficMonthlySnapshotReusable(row model.TrafficMonthly, sourceComplete bool) (bool, error) {
	status, err := parseTrafficMonthlyStatus(row.Status)
	if err != nil {
		return false, err
	}
	dataComplete := trafficDataComplete(int(row.SampleCount), int(row.ExpectedSampleCount), int(row.GapCount), int(row.ResetCount))
	if sourceComplete && !dataComplete {
		return false, nil
	}
	if status == TrafficSnapshotStale {
		return true, nil
	}
	return status == TrafficSnapshotSealed && dataComplete, nil
}

func (s *Store) trafficMonthlySnapshot(ctx context.Context, q TrafficQuery, cycle TrafficCycle) (TrafficSummary, bool, error) {
	var row model.TrafficMonthly
	err := s.db.WithContext(ctx).
		Where("server_id = ?", q.ServerID).
		Where("iface = ?", q.Iface).
		Where("cycle_mode = ?", string(cycle.Mode)).
		Where("billing_start_day = ?", cycle.BillingStartDay).
		Where("cycle_start = ? AND cycle_end = ?", cycle.Start, cycle.End).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return TrafficSummary{}, false, nil
		}
		return TrafficSummary{}, false, err
	}
	status, err := parseTrafficMonthlyStatus(row.Status)
	if err != nil {
		return TrafficSummary{}, false, err
	}
	if status != TrafficSnapshotSealed && status != TrafficSnapshotStale {
		return TrafficSummary{}, false, nil
	}
	dataComplete := trafficDataComplete(int(row.SampleCount), int(row.ExpectedSampleCount), int(row.GapCount), int(row.ResetCount))
	stat := TrafficStat{
		InBytes:             row.InBytes,
		OutBytes:            row.OutBytes,
		P95Enabled:          q.P95Enabled,
		InPeakBytesPerSec:   row.InPeakBytesPerSec,
		OutPeakBytesPerSec:  row.OutPeakBytesPerSec,
		BothPeakBytesPerSec: row.BothPeakBytesPerSec,
		SampleCount:         int(row.SampleCount),
		ExpectedSampleCount: int(row.ExpectedSampleCount),
		EffectiveStart:      row.EffectiveStart,
		EffectiveEnd:        row.EffectiveEnd,
		CoverageRatio:       row.CoverageRatio,
		CoveredUntil:        row.CoveredUntil,
		GapCount:            int(row.GapCount),
		ResetCount:          int(row.ResetCount),
		CycleComplete:       true,
		DataComplete:        dataComplete,
		Status:              status,
		Partial:             !dataComplete,
	}
	if q.P95Enabled && row.P95Enabled {
		stat.InP95BytesPerSec = row.InP95BytesPerSec
		stat.OutP95BytesPerSec = row.OutP95BytesPerSec
		stat.BothP95BytesPerSec = row.BothP95BytesPerSec
	}
	applyP95Status(&stat, q.UsageMode, row.P95Enabled)
	applyTrafficSelection(&stat, q.DirectionMode)
	return TrafficSummary{
		ServerID:  q.ServerID,
		Iface:     normalizeTrafficIface(q.Iface),
		UsageMode: q.UsageMode,
		Cycle:     cycle,
		Stat:      stat,
	}, true, nil
}

func (s *Store) trafficSummaryForCycle(ctx context.Context, q TrafficQuery, cycle TrafficCycle) (TrafficSummary, error) {
	statEnd, cycleComplete := trafficStatEnd(cycle, q.Ref)
	status := trafficSnapshotStatus(cycle, q.Ref)
	effectiveStart, effectiveEnd, err := s.trafficEffectiveWindow(ctx, q.ServerID, q.Iface, cycle.Start, statEnd)
	if err != nil {
		return TrafficSummary{}, err
	}
	rows, err := s.fetchTrafficBuckets(ctx, q, cycle, statEnd)
	if err != nil {
		return TrafficSummary{}, err
	}
	if len(rows) == 0 {
		return TrafficSummary{
			ServerID:  q.ServerID,
			Iface:     normalizeTrafficIface(q.Iface),
			UsageMode: q.UsageMode,
			Cycle:     cycle,
			Stat:      emptyTrafficStatWithP95(effectiveStart, effectiveEnd, cycleComplete, status, q.UsageMode, q.P95Enabled),
		}, ErrNoTrafficData
	}

	stat := buildTrafficStat(rows, effectiveStart, effectiveEnd, cycleComplete, status, q.DirectionMode, q.UsageMode, q.P95Enabled)
	summary := TrafficSummary{
		ServerID:  q.ServerID,
		Iface:     normalizeTrafficIface(q.Iface),
		UsageMode: q.UsageMode,
		Cycle:     cycle,
		Stat:      stat,
	}
	return summary, nil
}

func (s *Store) trafficEffectiveWindow(ctx context.Context, serverID int64, iface string, start, end time.Time) (time.Time, time.Time, error) {
	if !end.After(start) {
		return start, end, nil
	}
	var row struct {
		CreatedAt time.Time `gorm:"column:created_at"`
	}
	err := s.db.WithContext(ctx).
		Model(&model.Server{}).
		Select("created_at").
		Where("id = ?", serverID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return start, end, nil
		}
		return time.Time{}, time.Time{}, err
	}
	effectiveStart := start
	if row.CreatedAt.After(effectiveStart) && row.CreatedAt.Before(end) {
		effectiveStart = trafficBucketStart(row.CreatedAt)
		if effectiveStart.Before(start) {
			effectiveStart = start
		}
	}
	firstMetric, ok, err := s.firstTrafficMetricTime(ctx, serverID, iface, end)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if ok && firstMetric.After(effectiveStart) && firstMetric.Before(end) {
		effectiveStart = trafficBucketStart(firstMetric)
		if effectiveStart.Before(start) {
			effectiveStart = start
		}
	}
	if !end.After(effectiveStart) {
		return effectiveStart, effectiveStart, nil
	}
	return effectiveStart, end, nil
}

func (s *Store) firstTrafficMetricTime(ctx context.Context, serverID int64, iface string, before time.Time) (time.Time, bool, error) {
	iface = normalizeTrafficIface(iface)
	db := s.db.WithContext(ctx).
		Table("nic_metrics").
		Select("MIN(collected_at)").
		Where("server_id = ? AND iface = ? AND collected_at < ?", serverID, iface, before)
	var first sql.NullTime
	if err := db.Scan(&first).Error; err != nil {
		return time.Time{}, false, err
	}
	if !first.Valid || first.Time.IsZero() {
		return time.Time{}, false, nil
	}
	return first.Time.UTC(), true, nil
}

func emptyTrafficStatWithP95(start, end time.Time, cycleComplete bool, status TrafficSnapshotStatus, usage UsageMode, p95Enabled bool) TrafficStat {
	stat := emptyTrafficStat(start, end, cycleComplete, status)
	stat.P95Enabled = p95Enabled
	applyP95Status(&stat, usage, false)
	return stat
}

func (s *Store) fetchTrafficBuckets(ctx context.Context, q TrafficQuery, cycle TrafficCycle, statEnd time.Time) ([]trafficBucket, error) {
	var rows []trafficBucket
	err := s.db.WithContext(ctx).
		Table("traffic_5m").
		Select(`bucket,
			in_bytes,
			out_bytes,
			in_rate_bytes_per_sec,
			out_rate_bytes_per_sec,
			in_peak_bytes_per_sec,
			out_peak_bytes_per_sec,
			sample_count,
			gap_count,
			reset_count`).
		Where("server_id = ? AND iface = ?", q.ServerID, q.Iface).
		Where("bucket >= ? AND bucket < ?", cycle.Start, statEnd).
		Order("bucket ASC").
		Find(&rows).Error
	return rows, err
}

func normalizeTrafficQuery(q TrafficQuery) TrafficQuery {
	if q.Location == nil {
		q.Location = time.Local
	}
	if q.Ref.IsZero() {
		q.Ref = time.Now()
	}
	cycle, ok := NormalizeCycleMode(q.CycleMode)
	if !ok {
		cycle = CycleCalendarMonth
	}
	usage, ok := NormalizeUsageMode(q.UsageMode)
	if !ok {
		usage = UsageLite
	}
	direction, ok := NormalizeDirectionMode(q.DirectionMode)
	if !ok {
		direction = DirectionOut
	}
	day := q.BillingStartDay
	if day < 1 || day > 31 {
		day = 1
	}
	if cycle == CycleCalendarMonth {
		day = 1
	}
	anchor := ""
	if cycle == CycleWHMCS {
		anchor = strings.TrimSpace(q.BillingAnchorDate)
		if anchorTime, ok := parseTrafficAnchorDate(anchor, q.Location); ok {
			anchor = formatTrafficAnchorDate(anchorTime)
			day = anchorTime.Day()
		}
	}
	if usage == UsageLite {
		q.P95Enabled = false
	}
	if q.Period != TrafficPeriodPrev {
		q.Period = TrafficPeriodCurrent
	}
	q.UsageMode = usage
	q.CycleMode = cycle
	q.DirectionMode = direction
	q.BillingStartDay = day
	q.BillingAnchorDate = anchor
	q.Iface = normalizeTrafficIface(q.Iface)
	return q
}

func normalizeTrafficIface(iface string) string {
	return strings.TrimSpace(iface)
}

func validTrafficIface(iface string) bool {
	return iface != "" && !strings.EqualFold(iface, "all")
}

func buildTrafficStat(rows []trafficBucket, start, end time.Time, cycleComplete bool, status TrafficSnapshotStatus, direction DirectionMode, usage UsageMode, p95Enabled bool) TrafficStat {
	stat := emptyTrafficStat(start, end, cycleComplete, status)
	stat.P95Enabled = p95Enabled

	inValues := make([]float64, 0, len(rows))
	outValues := make([]float64, 0, len(rows))
	bothValues := make([]float64, 0, len(rows))
	for _, row := range rows {
		stat.InBytes += row.InBytes
		stat.OutBytes += row.OutBytes
		stat.GapCount += row.GapCount
		stat.ResetCount += row.ResetCount

		inRate := nonNegative(row.InRateBytesPerSec)
		outRate := nonNegative(row.OutRateBytesPerSec)
		if row.SampleCount > 0 {
			stat.SampleCount++
			if p95Enabled {
				inValues = append(inValues, inRate)
				outValues = append(outValues, outRate)
				bothValues = append(bothValues, inRate+outRate)
			}
			inPeak := nonNegative(row.InPeakBytesPerSec)
			outPeak := nonNegative(row.OutPeakBytesPerSec)
			stat.InPeakBytesPerSec = math.Max(stat.InPeakBytesPerSec, inPeak)
			stat.OutPeakBytesPerSec = math.Max(stat.OutPeakBytesPerSec, outPeak)
			stat.BothPeakBytesPerSec = math.Max(stat.BothPeakBytesPerSec, inPeak+outPeak)
		}
	}

	if p95Enabled && len(inValues) >= trafficMinP95Samples {
		stat.InP95BytesPerSec = p95DiscardTop(inValues)
		stat.OutP95BytesPerSec = p95DiscardTop(outValues)
		stat.BothP95BytesPerSec = p95DiscardTop(bothValues)
	}
	stat.CoverageRatio = coverageRatio(stat.SampleCount, stat.ExpectedSampleCount)
	stat.DataComplete = trafficDataComplete(stat.SampleCount, stat.ExpectedSampleCount, stat.GapCount, stat.ResetCount)
	stat.Partial = !stat.DataComplete
	applyP95Status(&stat, usage, p95Enabled && len(inValues) >= trafficMinP95Samples)
	applyTrafficSelection(&stat, direction)
	return stat
}

func emptyTrafficStat(start, end time.Time, cycleComplete bool, status TrafficSnapshotStatus) TrafficStat {
	expected := expectedTrafficSamples(start, end)
	dataComplete := trafficDataComplete(0, expected, 0, 0)
	return TrafficStat{
		ExpectedSampleCount: expected,
		EffectiveStart:      start,
		EffectiveEnd:        end,
		CoverageRatio:       coverageRatio(0, expected),
		CoveredUntil:        end,
		CycleComplete:       cycleComplete,
		DataComplete:        dataComplete,
		Status:              status,
		Partial:             !dataComplete,
	}
}

func applyTrafficSelection(stat *TrafficStat, direction DirectionMode) {
	switch direction {
	case DirectionOut:
		stat.SelectedBytes = stat.OutBytes
		stat.SelectedP95BytesPerSec = stat.OutP95BytesPerSec
		stat.SelectedPeakBytesPerSec = stat.OutPeakBytesPerSec
		stat.SelectedBytesDirection = TrafficDirectionOutKey
		stat.SelectedP95Direction = TrafficDirectionOutKey
		stat.SelectedPeakDirection = TrafficDirectionOutKey
	case DirectionBoth:
		stat.SelectedBytes = stat.InBytes + stat.OutBytes
		stat.SelectedP95BytesPerSec = stat.BothP95BytesPerSec
		stat.SelectedPeakBytesPerSec = stat.BothPeakBytesPerSec
		stat.SelectedBytesDirection = TrafficDirectionTotal
		stat.SelectedP95Direction = TrafficDirectionTotal
		stat.SelectedPeakDirection = TrafficDirectionTotal
	case DirectionMax:
		stat.SelectedBytes, stat.SelectedBytesDirection = maxTrafficBytes(
			stat.InBytes,
			stat.OutBytes,
		)
		stat.SelectedP95BytesPerSec, stat.SelectedP95Direction = maxTrafficRate(
			stat.InP95BytesPerSec,
			stat.OutP95BytesPerSec,
		)
		stat.SelectedPeakBytesPerSec, stat.SelectedPeakDirection = maxTrafficRate(
			stat.InPeakBytesPerSec,
			stat.OutPeakBytesPerSec,
		)
	default:
		stat.SelectedBytes = stat.OutBytes
		stat.SelectedP95BytesPerSec = stat.OutP95BytesPerSec
		stat.SelectedPeakBytesPerSec = stat.OutPeakBytesPerSec
		stat.SelectedBytesDirection = TrafficDirectionOutKey
		stat.SelectedP95Direction = TrafficDirectionOutKey
		stat.SelectedPeakDirection = TrafficDirectionOutKey
	}
}

func maxTrafficBytes(inBytes, outBytes int64) (int64, TrafficDirection) {
	if inBytes > outBytes {
		return inBytes, TrafficDirectionInKey
	}
	return outBytes, TrafficDirectionOutKey
}

func maxTrafficRate(inRate, outRate float64) (float64, TrafficDirection) {
	if inRate > outRate {
		return inRate, TrafficDirectionInKey
	}
	return outRate, TrafficDirectionOutKey
}

func applyP95Status(stat *TrafficStat, usage UsageMode, p95Available bool) {
	switch {
	case usage == UsageLite:
		stat.P95Status = TrafficP95LiteMode
	case !stat.P95Enabled:
		stat.P95Status = TrafficP95Disabled
	case p95Available:
		stat.P95Status = TrafficP95Available
	case stat.SampleCount < trafficMinP95Samples:
		stat.P95Status = TrafficP95InsufficientSamples
	default:
		stat.P95Status = TrafficP95SnapshotMissing
	}
	if stat.P95Status != TrafficP95Available {
		stat.P95UnavailableReason = string(stat.P95Status)
	}
}

func p95DiscardTop(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] > sorted[j]
	})
	drop := int(math.Floor(float64(len(sorted)) * 0.05))
	if drop >= len(sorted) {
		drop = len(sorted) - 1
	}
	return sorted[drop]
}

func expectedTrafficSamples(start, end time.Time) int {
	if end.Before(start) || end.Equal(start) {
		return 0
	}
	return int(math.Ceil(end.Sub(start).Seconds() / trafficBucketSize.Seconds()))
}

func trafficStatEnd(cycle TrafficCycle, ref time.Time) (time.Time, bool) {
	if ref.IsZero() {
		ref = time.Now()
	}
	ref = ref.UTC()
	if !ref.Before(cycle.End) {
		return cycle.End, true
	}
	end := trafficBucketStart(ref)
	if end.Before(cycle.Start) {
		end = cycle.Start
	}
	return end, false
}

func trafficSnapshotStatus(cycle TrafficCycle, ref time.Time) TrafficSnapshotStatus {
	if ref.IsZero() {
		ref = time.Now()
	}
	ref = ref.UTC()
	if ref.Before(cycle.End) {
		return TrafficSnapshotProvisional
	}
	if ref.Before(cycle.End.Add(trafficSnapshotGrace)) {
		return TrafficSnapshotGrace
	}
	return TrafficSnapshotSealed
}

func parseTrafficMonthlyStatus(status string) (TrafficSnapshotStatus, error) {
	switch TrafficSnapshotStatus(status) {
	case TrafficSnapshotGrace:
		return TrafficSnapshotGrace, nil
	case TrafficSnapshotSealed:
		return TrafficSnapshotSealed, nil
	case TrafficSnapshotStale:
		return TrafficSnapshotStale, nil
	default:
		return "", fmt.Errorf("invalid traffic monthly snapshot status %q", status)
	}
}

func trafficDataComplete(count, expected, gaps, resets int) bool {
	if expected <= 0 {
		return true
	}
	return count >= expected && gaps == 0 && resets == 0
}

func coverageRatio(count, expected int) float64 {
	if expected <= 0 {
		return 1
	}
	ratio := float64(count) / float64(expected)
	if ratio > 1 {
		return 1
	}
	return ratio
}

func nonNegative(v float64) float64 {
	if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}
