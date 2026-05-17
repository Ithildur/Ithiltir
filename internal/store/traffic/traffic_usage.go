package traffic

import (
	"context"
	"fmt"
	"math"
	"time"

	"dash/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type trafficUsageKey struct {
	serverID        int64
	iface           string
	cycleMode       string
	billingStartDay int16
	cycleStart      time.Time
	cycleEnd        time.Time
}

type trafficUsageAccumulator struct {
	row model.TrafficMonthUsage
}

type trafficUsageFetch struct {
	IfaceCount          int       `gorm:"column:iface_count"`
	InBytes             int64     `gorm:"column:in_bytes"`
	OutBytes            int64     `gorm:"column:out_bytes"`
	InPeakBytesPerSec   float64   `gorm:"column:in_peak_bytes_per_sec"`
	OutPeakBytesPerSec  float64   `gorm:"column:out_peak_bytes_per_sec"`
	BothPeakBytesPerSec float64   `gorm:"column:both_peak_bytes_per_sec"`
	SampleCount         int       `gorm:"column:sample_count"`
	GapCount            int       `gorm:"column:gap_count"`
	ResetCount          int       `gorm:"column:reset_count"`
	CoveredUntil        time.Time `gorm:"column:covered_until"`
}

func (s *Store) BackfillTrafficMonthUsage(ctx context.Context, settings Settings, loc *time.Location, start, end time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store: db is nil")
	}
	var ok bool
	settings, ok = NormalizeSettings(settings)
	if !ok {
		return fmt.Errorf("invalid traffic settings")
	}
	if loc == nil {
		loc = time.Local
	}
	if end.IsZero() {
		end = time.Now().UTC()
	}
	end = trafficBucketStart(end)
	if start.IsZero() {
		start = end.Add(-trafficBackfillWindow)
	}
	start = trafficBucketStart(start)
	if !end.After(start) {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rows, err := loadTrafficNICRows(tx, start, end)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		progress, err := loadTrafficUsageProgress(tx, start, end)
		if err != nil {
			return err
		}
		items := buildTrafficMonthUsageRows(rows, settings, loc, start, end, progress)
		if len(items) == 0 {
			return nil
		}
		return upsertTrafficMonthUsage(tx, items)
	})
}

func loadTrafficUsageProgress(tx *gorm.DB, start, end time.Time) (map[trafficUsageKey]time.Time, error) {
	var rows []model.TrafficMonthUsage
	err := tx.
		Where("cycle_end > ? AND cycle_start < ?", start, end).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make(map[trafficUsageKey]time.Time, len(rows))
	for _, row := range rows {
		out[trafficUsageKeyFromUsage(row)] = row.LastCollectedAt
	}
	return out, nil
}

func buildTrafficMonthUsageRows(rows []trafficNICRow, settings Settings, loc *time.Location, start, end time.Time, progress map[trafficUsageKey]time.Time) []model.TrafficMonthUsage {
	usage := make(map[trafficUsageKey]*trafficUsageAccumulator)
	var prev trafficNICRow
	hasPrev := false

	for _, row := range rows {
		if !hasPrev || prev.ServerID != row.ServerID || prev.Iface != row.Iface {
			prev = row
			hasPrev = true
			continue
		}
		if row.CollectedAt.After(prev.CollectedAt) {
			mergeTrafficUsagePair(usage, progress, settings, loc, start, end, prev, row)
		}
		prev = row
	}

	out := make([]model.TrafficMonthUsage, 0, len(usage))
	for _, acc := range usage {
		out = append(out, acc.row)
	}
	return out
}

func mergeTrafficUsagePair(usage map[trafficUsageKey]*trafficUsageAccumulator, progress map[trafficUsageKey]time.Time, settings Settings, loc *time.Location, start, end time.Time, prev, current trafficNICRow) {
	inDelta := current.BytesRecv - prev.BytesRecv
	outDelta := current.BytesSent - prev.BytesSent
	cycleSettings := SettingsWithServerCycleSettings(settings, serverCycleSettingsFromRow(current))
	cycleLoc := SettingsLocation(cycleSettings, loc)
	if inDelta < 0 || outDelta < 0 {
		mergeTrafficUsageReset(usage, progress, cycleSettings, cycleLoc, start, end, current)
		return
	}
	if inDelta == 0 && outDelta == 0 {
		return
	}

	pairStart := prev.CollectedAt.UTC()
	pairEnd := current.CollectedAt.UTC()
	if !pairEnd.After(pairStart) || !pairEnd.After(start) || !pairStart.Before(end) {
		return
	}

	totalSec := pairEnd.Sub(pairStart).Seconds()
	if totalSec <= 0 {
		return
	}
	inRate := float64(inDelta) / totalSec
	outRate := float64(outDelta) / totalSec
	gap := totalSec > trafficMaxBillingGap.Seconds()

	for cursor := pairStart; cursor.Before(pairEnd); {
		cycle := currentTrafficCycleAnchored(cycleSettings.CycleMode, cycleSettings.BillingStartDay, cycleSettings.BillingAnchorDate, cycleLoc, cursor)
		segEnd := minTime(pairEnd, cycle.End)
		if !segEnd.After(cursor) {
			break
		}

		key := trafficUsageKeyFromCycle(current.ServerID, current.Iface, cycle)
		from := cursor
		if last := progress[key]; last.After(from) {
			from = last
		}
		if from.Before(start) {
			from = start
		}
		if segEnd.After(from) && segEnd.After(cycle.Start) && from.Before(cycle.End) {
			seconds := segEnd.Sub(from).Seconds()
			inBytes := int64(math.Round(float64(inDelta) * seconds / totalSec))
			outBytes := int64(math.Round(float64(outDelta) * seconds / totalSec))
			mergeTrafficUsageSample(trafficUsageAccumulatorFor(usage, key, cycle), inBytes, outBytes, inRate, outRate, gap, segEnd)
			progress[key] = maxTime(progress[key], segEnd)
		}
		cursor = segEnd
	}
}

func mergeTrafficUsageReset(usage map[trafficUsageKey]*trafficUsageAccumulator, progress map[trafficUsageKey]time.Time, settings Settings, loc *time.Location, start, end time.Time, row trafficNICRow) {
	at := row.CollectedAt.UTC()
	if at.Before(start) || !at.Before(end) {
		return
	}
	cycle := currentTrafficCycleAnchored(settings.CycleMode, settings.BillingStartDay, settings.BillingAnchorDate, loc, at)
	key := trafficUsageKeyFromCycle(row.ServerID, row.Iface, cycle)
	if !at.After(progress[key]) {
		return
	}
	acc := trafficUsageAccumulatorFor(usage, key, cycle)
	acc.row.ResetCount++
	acc.row.LastCollectedAt = maxTime(acc.row.LastCollectedAt, at)
	acc.row.CoveredUntil = maxTime(acc.row.CoveredUntil, at)
	progress[key] = at
}

func serverCycleSettingsFromRow(row trafficNICRow) ServerCycleSettings {
	return ServerCycleSettings{
		Mode:              ServerCycleMode(row.ServerCycleMode),
		BillingStartDay:   row.BillingStartDay,
		BillingAnchorDate: row.BillingAnchorDate,
		BillingTimezone:   row.BillingTimezone,
	}
}

func mergeTrafficUsageSample(acc *trafficUsageAccumulator, inBytes, outBytes int64, inRate, outRate float64, gap bool, coveredUntil time.Time) {
	acc.row.InBytes += inBytes
	acc.row.OutBytes += outBytes
	acc.row.CoveredUntil = maxTime(acc.row.CoveredUntil, coveredUntil)
	acc.row.LastCollectedAt = maxTime(acc.row.LastCollectedAt, coveredUntil)
	if gap {
		acc.row.GapCount++
		return
	}
	acc.row.SampleCount++
	inRate = nonNegative(inRate)
	outRate = nonNegative(outRate)
	acc.row.InPeakBytesPerSec = math.Max(acc.row.InPeakBytesPerSec, inRate)
	acc.row.OutPeakBytesPerSec = math.Max(acc.row.OutPeakBytesPerSec, outRate)
	acc.row.BothPeakBytesPerSec = math.Max(acc.row.BothPeakBytesPerSec, inRate+outRate)
}

func trafficUsageAccumulatorFor(usage map[trafficUsageKey]*trafficUsageAccumulator, key trafficUsageKey, cycle TrafficCycle) *trafficUsageAccumulator {
	acc := usage[key]
	if acc != nil {
		return acc
	}
	acc = &trafficUsageAccumulator{
		row: model.TrafficMonthUsage{
			ServerID:        key.serverID,
			Iface:           key.iface,
			CycleMode:       key.cycleMode,
			BillingStartDay: key.billingStartDay,
			Timezone:        cycle.Timezone,
			CycleStart:      cycle.Start,
			CycleEnd:        cycle.End,
		},
	}
	usage[key] = acc
	return acc
}

func upsertTrafficMonthUsage(tx *gorm.DB, items []model.TrafficMonthUsage) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "server_id"},
			{Name: "iface"},
			{Name: "cycle_mode"},
			{Name: "billing_start_day"},
			{Name: "cycle_start"},
			{Name: "cycle_end"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"covered_until":           gorm.Expr("GREATEST(traffic_month_usage.covered_until, EXCLUDED.covered_until)"),
			"last_collected_at":       gorm.Expr("GREATEST(traffic_month_usage.last_collected_at, EXCLUDED.last_collected_at)"),
			"in_bytes":                gorm.Expr("traffic_month_usage.in_bytes + EXCLUDED.in_bytes"),
			"out_bytes":               gorm.Expr("traffic_month_usage.out_bytes + EXCLUDED.out_bytes"),
			"in_peak_bytes_per_sec":   gorm.Expr("GREATEST(traffic_month_usage.in_peak_bytes_per_sec, EXCLUDED.in_peak_bytes_per_sec)"),
			"out_peak_bytes_per_sec":  gorm.Expr("GREATEST(traffic_month_usage.out_peak_bytes_per_sec, EXCLUDED.out_peak_bytes_per_sec)"),
			"both_peak_bytes_per_sec": gorm.Expr("GREATEST(traffic_month_usage.both_peak_bytes_per_sec, EXCLUDED.both_peak_bytes_per_sec)"),
			"sample_count":            gorm.Expr("traffic_month_usage.sample_count + EXCLUDED.sample_count"),
			"gap_count":               gorm.Expr("traffic_month_usage.gap_count + EXCLUDED.gap_count"),
			"reset_count":             gorm.Expr("traffic_month_usage.reset_count + EXCLUDED.reset_count"),
		}),
	}).CreateInBatches(items, 500).Error
}

func (s *Store) trafficUsageSummaryForCycle(ctx context.Context, q TrafficQuery, cycle TrafficCycle) (TrafficSummary, error) {
	statEnd, cycleComplete := trafficStatEnd(cycle, q.Ref)
	status := trafficSnapshotStatus(cycle, q.Ref)
	row, ok, err := s.fetchTrafficUsage(ctx, q, cycle)
	if err != nil {
		return TrafficSummary{}, err
	}
	if !ok {
		stat := emptyTrafficStat(cycle.Start, statEnd, cycleComplete, status)
		applyP95Status(&stat, q.UsageMode, false)
		return TrafficSummary{
			ServerID:  q.ServerID,
			Iface:     normalizeTrafficIface(q.Iface),
			UsageMode: q.UsageMode,
			Cycle:     cycle,
			Stat:      stat,
		}, ErrNoTrafficData
	}

	coveredUntil := row.CoveredUntil
	if coveredUntil.IsZero() {
		coveredUntil = statEnd
	}
	stat := TrafficStat{
		InBytes:             row.InBytes,
		OutBytes:            row.OutBytes,
		InPeakBytesPerSec:   row.InPeakBytesPerSec,
		OutPeakBytesPerSec:  row.OutPeakBytesPerSec,
		BothPeakBytesPerSec: row.BothPeakBytesPerSec,
		SampleCount:         row.SampleCount,
		ExpectedSampleCount: 0,
		EffectiveStart:      cycle.Start,
		EffectiveEnd:        statEnd,
		CoverageRatio:       trafficUsageCoverage(row.GapCount, row.ResetCount),
		CoveredUntil:        coveredUntil,
		GapCount:            row.GapCount,
		ResetCount:          row.ResetCount,
		CycleComplete:       cycleComplete,
		DataComplete:        trafficUsageDataComplete(row.GapCount, row.ResetCount),
		Status:              status,
		Partial:             !trafficUsageDataComplete(row.GapCount, row.ResetCount),
	}
	applyP95Status(&stat, q.UsageMode, false)
	applyTrafficSelection(&stat, q.DirectionMode)
	return TrafficSummary{
		ServerID:  q.ServerID,
		Iface:     normalizeTrafficIface(q.Iface),
		UsageMode: q.UsageMode,
		Cycle:     cycle,
		Stat:      stat,
	}, nil
}

func (s *Store) fetchTrafficUsage(ctx context.Context, q TrafficQuery, cycle TrafficCycle) (trafficUsageFetch, bool, error) {
	iface := normalizeTrafficIface(q.Iface)
	row, ok, err := s.fetchTrafficUsageIface(ctx, q, cycle, iface)
	return row, ok, err
}

func (s *Store) fetchTrafficUsageIface(ctx context.Context, q TrafficQuery, cycle TrafficCycle, iface string) (trafficUsageFetch, bool, error) {
	db := s.db.WithContext(ctx).
		Table("traffic_month_usage").
		Select(`COUNT(*) AS iface_count,
			COALESCE(SUM(in_bytes), 0) AS in_bytes,
			COALESCE(SUM(out_bytes), 0) AS out_bytes,
			COALESCE(SUM(in_peak_bytes_per_sec), 0) AS in_peak_bytes_per_sec,
			COALESCE(SUM(out_peak_bytes_per_sec), 0) AS out_peak_bytes_per_sec,
			COALESCE(SUM(both_peak_bytes_per_sec), 0) AS both_peak_bytes_per_sec,
			COALESCE(SUM(sample_count), 0) AS sample_count,
			COALESCE(SUM(gap_count), 0) AS gap_count,
			COALESCE(SUM(reset_count), 0) AS reset_count,
			COALESCE(MAX(covered_until), ?::timestamptz) AS covered_until`, cycle.Start).
		Where("server_id = ?", q.ServerID).
		Where("cycle_mode = ?", string(cycle.Mode)).
		Where("billing_start_day = ?", cycle.BillingStartDay).
		Where("cycle_start = ? AND cycle_end = ?", cycle.Start, cycle.End).
		Where("iface = ?", iface)

	var row trafficUsageFetch
	if err := db.Scan(&row).Error; err != nil {
		return trafficUsageFetch{}, false, err
	}
	return row, row.IfaceCount > 0, nil
}

func trafficUsageKeyFromCycle(serverID int64, iface string, cycle TrafficCycle) trafficUsageKey {
	return trafficUsageKey{
		serverID:        serverID,
		iface:           iface,
		cycleMode:       string(cycle.Mode),
		billingStartDay: int16(cycle.BillingStartDay),
		cycleStart:      cycle.Start,
		cycleEnd:        cycle.End,
	}
}

func trafficUsageKeyFromUsage(row model.TrafficMonthUsage) trafficUsageKey {
	return trafficUsageKey{
		serverID:        row.ServerID,
		iface:           row.Iface,
		cycleMode:       row.CycleMode,
		billingStartDay: row.BillingStartDay,
		cycleStart:      row.CycleStart,
		cycleEnd:        row.CycleEnd,
	}
}

func trafficUsageDataComplete(gaps, resets int) bool {
	return gaps == 0 && resets == 0
}

func trafficUsageCoverage(gaps, resets int) float64 {
	if trafficUsageDataComplete(gaps, resets) {
		return 1
	}
	return 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return b
	}
	return a
}
