package traffic

import (
	"context"
	"errors"
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
	InBytes             int64     `gorm:"column:in_bytes"`
	OutBytes            int64     `gorm:"column:out_bytes"`
	InPeakBytesPerSec   float64   `gorm:"column:in_peak_bytes_per_sec"`
	OutPeakBytesPerSec  float64   `gorm:"column:out_peak_bytes_per_sec"`
	BothPeakBytesPerSec float64   `gorm:"column:both_peak_bytes_per_sec"`
	SampleCount         int       `gorm:"column:sample_count"`
	GapCount            int       `gorm:"column:gap_count"`
	ResetCount          int       `gorm:"column:reset_count"`
	CoveredFrom         time.Time `gorm:"column:covered_from"`
	CoveredUntil        time.Time `gorm:"column:covered_until"`
}

type trafficUsageNICRow struct {
	ServerID          int64     `gorm:"column:server_id"`
	Iface             string    `gorm:"column:iface"`
	ServerCycleMode   string    `gorm:"column:traffic_cycle_mode"`
	BillingStartDay   int       `gorm:"column:traffic_billing_start_day"`
	BillingAnchorDate string    `gorm:"column:traffic_billing_anchor_date"`
	BillingTimezone   string    `gorm:"column:traffic_billing_timezone"`
	CollectedAt       time.Time `gorm:"column:collected_at"`
	BytesRecv         int64     `gorm:"column:bytes_recv"`
	BytesSent         int64     `gorm:"column:bytes_sent"`
}

func (s *Store) MaterializeTrafficMonthUsage(ctx context.Context, loc *time.Location, target, sourceFloor time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("store: db is nil")
	}
	if loc == nil {
		return false, fmt.Errorf("traffic usage location is nil")
	}

	var hasMore bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item, err := loadTrafficSetting(tx.Clauses(clause.Locking{Strength: "SHARE"}))
		if err != nil {
			return err
		}
		settings, err := NormalizeSettings(settingsFromTrafficSetting(item))
		if err != nil {
			return fmt.Errorf("stored traffic settings: %w", err)
		}
		progress, err := lockMaterializationProgress(tx, materializationUsage)
		if err != nil {
			return err
		}
		start, end, advanced := nextMaterializationRange(progress, target, sourceFloor, trafficMaterializationOverlap)
		hasMore = advanced && end.Before(trafficBucketStart(target))
		if !advanced {
			if end.After(progress.ScannedUntil) {
				return setMaterializationProgress(tx, materializationUsage, end)
			}
			return nil
		}
		if end.After(start) {
			if err := materializeTrafficMonthUsageRange(tx, settings, loc, start, end, nil); err != nil {
				return err
			}
		}
		return setMaterializationProgress(tx, materializationUsage, end)
	})
	return hasMore, err
}

// MaterializeTrafficMonthUsageRepair advances one node-local Usage repair
// without moving the live Usage high-water mark.
func (s *Store) MaterializeTrafficMonthUsageRepair(ctx context.Context, loc *time.Location, target, sourceFloor time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("store: db is nil")
	}
	if loc == nil {
		return false, fmt.Errorf("traffic usage location is nil")
	}

	target = trafficBucketStart(target)
	sourceFloor = trafficBucketStart(sourceFloor)
	var hasMore bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item, err := loadTrafficSetting(tx.Clauses(clause.Locking{Strength: "SHARE"}))
		if err != nil {
			return err
		}
		settings, err := NormalizeSettings(settingsFromTrafficSetting(item))
		if err != nil {
			return fmt.Errorf("stored traffic settings: %w", err)
		}
		// The Usage progress row is also the database serialization point for
		// live scans, repairs, and cycle changes.
		if _, err := lockMaterializationProgress(tx, materializationUsage); err != nil {
			return err
		}
		repair, err := lockNextTrafficUsageRepair(tx)
		if err != nil {
			return err
		}
		if repair == nil {
			hasMore = false
			return nil
		}

		start := trafficBucketStart(repair.ScannedUntil)
		if start.Before(sourceFloor) {
			start = sourceFloor
		}
		serverIDs := []int64{repair.ServerID}
		if !target.After(start) {
			if err := deleteTrafficUsageRepair(tx, repair.ServerID); err != nil {
				return err
			}
			hasMore, err = trafficUsageRepairsRemain(tx)
			return err
		}

		end := start.Add(trafficUsageRepairChunk)
		if end.After(target) {
			end = target
		}
		if err := materializeTrafficMonthUsageRange(tx, settings, loc, start, end, serverIDs); err != nil {
			return err
		}
		if end.Equal(target) {
			if err := deleteTrafficUsageRepair(tx, repair.ServerID); err != nil {
				return err
			}
		} else if err := advanceTrafficUsageRepair(tx, repair.ServerID, end); err != nil {
			return err
		}
		hasMore, err = trafficUsageRepairsRemain(tx)
		return err
	})
	return hasMore, err
}

func materializeTrafficMonthUsageRange(tx *gorm.DB, settings Settings, loc *time.Location, start, end time.Time, serverIDs []int64) error {
	rows, err := loadTrafficUsageRows(tx, start, end, serverIDs)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	progress, err := loadTrafficUsageProgress(tx, start, end, rows)
	if err != nil {
		return err
	}
	items, err := buildTrafficMonthUsageRows(rows, settings, loc, start, end, progress)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	return upsertTrafficMonthUsage(tx, items)
}

func loadTrafficUsageRows(tx *gorm.DB, start, end time.Time, serverIDs []int64) ([]trafficUsageNICRow, error) {
	serverScope := `s.is_deleted = FALSE
			AND NOT EXISTS (
				SELECT 1
				FROM traffic_usage_repairs r
				WHERE r.server_id = s.id
			)`
	args := make([]any, 0, 7)
	if len(serverIDs) > 0 {
		serverScope = "s.is_deleted = FALSE AND s.id IN ?"
		args = append(args, serverIDs)
	}
	args = append(args, start, end, start, end, start, end)

	var rows []trafficUsageNICRow
	query := fmt.Sprintf(`
	WITH active_servers AS (
		SELECT
			s.id,
			COALESCE(NULLIF(s.traffic_cycle_mode, ''), 'calendar_month') AS traffic_cycle_mode,
			COALESCE(s.traffic_billing_start_day, 1) AS traffic_billing_start_day,
			COALESCE(s.traffic_billing_anchor_date, '') AS traffic_billing_anchor_date,
			COALESCE(s.traffic_billing_timezone, '') AS traffic_billing_timezone
		FROM servers s
		WHERE %s
	),
	current_rows AS (
		SELECT
			n.server_id,
			n.iface,
			s.traffic_cycle_mode,
			s.traffic_billing_start_day,
			s.traffic_billing_anchor_date,
			s.traffic_billing_timezone,
			n.collected_at,
			n.bytes_recv,
			n.bytes_sent
		FROM nic_metrics n
		JOIN active_servers s ON s.id = n.server_id
		WHERE n.collected_at >= ? AND n.collected_at < ?
	),
	scoped_pairs AS (
		SELECT
			server_id,
			iface,
			traffic_cycle_mode,
			traffic_billing_start_day,
			traffic_billing_anchor_date,
			traffic_billing_timezone
		FROM current_rows
		UNION
		SELECT
			n.server_id,
			n.iface,
			s.traffic_cycle_mode,
			s.traffic_billing_start_day,
			s.traffic_billing_anchor_date,
			s.traffic_billing_timezone
		FROM server_current_nic_metrics n
		JOIN active_servers s ON s.id = n.server_id
		UNION
		SELECT
			u.server_id,
			u.iface,
			s.traffic_cycle_mode,
			s.traffic_billing_start_day,
			s.traffic_billing_anchor_date,
			s.traffic_billing_timezone
		FROM traffic_month_usage u
		JOIN active_servers s ON s.id = u.server_id
		WHERE u.cycle_end > ? AND u.cycle_start < ?
	),
	prev_rows AS (
		SELECT
			s.server_id,
			s.iface,
			s.traffic_cycle_mode,
			s.traffic_billing_start_day,
			s.traffic_billing_anchor_date,
			s.traffic_billing_timezone,
			p.collected_at,
			p.bytes_recv,
			p.bytes_sent
		FROM scoped_pairs s
		JOIN LATERAL (
			SELECT
				n.collected_at,
				n.bytes_recv,
				n.bytes_sent
			FROM nic_metrics n
			WHERE n.server_id = s.server_id
				AND n.iface = s.iface
				AND n.collected_at < ?
			ORDER BY n.collected_at DESC
			LIMIT 1
		) p ON true
	),
	next_rows AS (
		SELECT
			s.server_id,
			s.iface,
			s.traffic_cycle_mode,
			s.traffic_billing_start_day,
			s.traffic_billing_anchor_date,
			s.traffic_billing_timezone,
			p.collected_at,
			p.bytes_recv,
			p.bytes_sent
		FROM scoped_pairs s
		JOIN LATERAL (
			SELECT
				n.collected_at,
				n.bytes_recv,
				n.bytes_sent
			FROM nic_metrics n
			WHERE n.server_id = s.server_id
				AND n.iface = s.iface
				AND n.collected_at >= ?
			ORDER BY n.collected_at ASC
			LIMIT 1
		) p ON true
	)
	SELECT server_id, iface, traffic_cycle_mode, traffic_billing_start_day, traffic_billing_anchor_date, traffic_billing_timezone, collected_at, bytes_recv, bytes_sent FROM prev_rows
	UNION ALL
	SELECT server_id, iface, traffic_cycle_mode, traffic_billing_start_day, traffic_billing_anchor_date, traffic_billing_timezone, collected_at, bytes_recv, bytes_sent FROM current_rows
	UNION ALL
	SELECT server_id, iface, traffic_cycle_mode, traffic_billing_start_day, traffic_billing_anchor_date, traffic_billing_timezone, collected_at, bytes_recv, bytes_sent FROM next_rows
	ORDER BY server_id, iface, collected_at
	`, serverScope)
	err := tx.Raw(query, args...).Scan(&rows).Error
	return rows, err
}

func lockNextTrafficUsageRepair(tx *gorm.DB) (*model.TrafficUsageRepair, error) {
	var repair model.TrafficUsageRepair
	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Order("scanned_until ASC, server_id ASC").
		Take(&repair).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lock traffic usage repair: %w", err)
	}
	return &repair, nil
}

func advanceTrafficUsageRepair(tx *gorm.DB, serverID int64, end time.Time) error {
	result := tx.
		Model(&model.TrafficUsageRepair{}).
		Where("server_id = ?", serverID).
		Update("scanned_until", end.UTC())
	if result.Error != nil {
		return fmt.Errorf("advance traffic usage repair: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("advance traffic usage repair: repair changed")
	}
	return nil
}

func deleteTrafficUsageRepair(tx *gorm.DB, serverID int64) error {
	result := tx.
		Where("server_id = ?", serverID).
		Delete(&model.TrafficUsageRepair{})
	if result.Error != nil {
		return fmt.Errorf("complete traffic usage repair: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("complete traffic usage repair: repair changed")
	}
	return nil
}

func trafficUsageRepairsRemain(tx *gorm.DB) (bool, error) {
	var repair model.TrafficUsageRepair
	err := tx.Select("server_id").Take(&repair).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check traffic usage repairs: %w", err)
	}
	return true, nil
}

func loadTrafficUsageProgress(tx *gorm.DB, start, end time.Time, samples []trafficUsageNICRow) (map[trafficUsageKey]time.Time, error) {
	serverIDs := trafficUsageServerIDs(samples)
	if len(serverIDs) == 0 {
		return map[trafficUsageKey]time.Time{}, nil
	}

	var rows []model.TrafficMonthUsage
	err := tx.
		Where("server_id IN ? AND cycle_end > ? AND cycle_start < ?", serverIDs, start, end).
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

func trafficUsageServerIDs(rows []trafficUsageNICRow) []int64 {
	seen := make(map[int64]struct{})
	out := make([]int64, 0)
	for _, row := range rows {
		if row.ServerID <= 0 {
			continue
		}
		if _, ok := seen[row.ServerID]; ok {
			continue
		}
		seen[row.ServerID] = struct{}{}
		out = append(out, row.ServerID)
	}
	return out
}

func buildTrafficMonthUsageRows(rows []trafficUsageNICRow, settings Settings, loc *time.Location, start, end time.Time, progress map[trafficUsageKey]time.Time) ([]model.TrafficMonthUsage, error) {
	usage := make(map[trafficUsageKey]*trafficUsageAccumulator)
	var prev trafficUsageNICRow
	var rule cycleRule
	var ruleServerID int64
	hasPrev := false
	hasRule := false

	for _, row := range rows {
		if !hasRule || row.ServerID != ruleServerID {
			effective, err := SettingsWithServerCycle(settings, serverCycleSettingsFromRow(row))
			if err != nil {
				return nil, fmt.Errorf("server %d traffic cycle settings: %w", row.ServerID, err)
			}
			cycleLoc, err := SettingsLocation(effective, loc)
			if err != nil {
				return nil, fmt.Errorf("server %d traffic cycle location: %w", row.ServerID, err)
			}
			rule, err = newCycleRule(
				effective.CycleMode,
				effective.BillingStartDay,
				effective.BillingAnchorDate,
				cycleLoc,
			)
			if err != nil {
				return nil, fmt.Errorf("server %d traffic cycle rule: %w", row.ServerID, err)
			}
			ruleServerID = row.ServerID
			hasRule = true
		}
		if !hasPrev || prev.ServerID != row.ServerID || prev.Iface != row.Iface {
			prev = row
			hasPrev = true
			continue
		}
		if row.CollectedAt.After(prev.CollectedAt) {
			if err := mergeTrafficUsagePair(usage, progress, rule, start, end, prev, row); err != nil {
				return nil, fmt.Errorf("server %d interface %q traffic usage: %w", row.ServerID, row.Iface, err)
			}
		}
		prev = row
	}

	out := make([]model.TrafficMonthUsage, 0, len(usage))
	for _, acc := range usage {
		out = append(out, acc.row)
	}
	return out, nil
}

func mergeTrafficUsagePair(usage map[trafficUsageKey]*trafficUsageAccumulator, progress map[trafficUsageKey]time.Time, rule cycleRule, start, end time.Time, prev, current trafficUsageNICRow) error {
	inDelta := current.BytesRecv - prev.BytesRecv
	outDelta := current.BytesSent - prev.BytesSent
	if inDelta < 0 || outDelta < 0 {
		return mergeTrafficUsageReset(usage, progress, rule, start, end, current)
	}
	if inDelta == 0 && outDelta == 0 {
		return nil
	}

	pairStart := prev.CollectedAt.UTC()
	pairEnd := current.CollectedAt.UTC()
	if !pairEnd.After(pairStart) || !pairEnd.After(start) || !pairStart.Before(end) {
		return nil
	}

	totalSec := pairEnd.Sub(pairStart).Seconds()
	if totalSec <= 0 {
		return nil
	}
	inRate := float64(inDelta) / totalSec
	outRate := float64(outDelta) / totalSec
	gap := totalSec > trafficMaxBillingGap.Seconds()

	for cursor := maxTime(pairStart, start); cursor.Before(pairEnd) && cursor.Before(end); {
		cycle, err := rule.at(cursor)
		if err != nil {
			return err
		}
		segEnd := minTime(minTime(pairEnd, cycle.End), end)
		if !segEnd.After(cursor) {
			return fmt.Errorf("traffic cycle does not advance after %s", cursor.Format(time.RFC3339))
		}

		key := trafficUsageKeyFromCycle(current.ServerID, current.Iface, cycle)
		from := cursor
		last := progress[key]
		if last.After(from) {
			from = last
		}
		if from.Before(start) {
			from = start
		}
		if segEnd.After(from) && segEnd.After(cycle.Start) && from.Before(cycle.End) {
			fromSec := from.Sub(pairStart).Seconds()
			toSec := segEnd.Sub(pairStart).Seconds()
			inBytes := trafficBytesAt(inDelta, toSec, totalSec) - trafficBytesAt(inDelta, fromSec, totalSec)
			outBytes := trafficBytesAt(outDelta, toSec, totalSec) - trafficBytesAt(outDelta, fromSec, totalSec)
			gapStart := maxTime(pairStart, cycle.Start)
			markGap := gap && !last.After(gapStart)
			mergeTrafficUsageSample(trafficUsageAccumulatorFor(usage, key, cycle), inBytes, outBytes, inRate, outRate, markGap, from, segEnd)
			progress[key] = maxTime(progress[key], segEnd)
		}
		cursor = segEnd
	}
	return nil
}

func mergeTrafficUsageReset(usage map[trafficUsageKey]*trafficUsageAccumulator, progress map[trafficUsageKey]time.Time, rule cycleRule, start, end time.Time, row trafficUsageNICRow) error {
	at := row.CollectedAt.UTC()
	if at.Before(start) || !at.Before(end) {
		return nil
	}
	cycle, err := rule.at(at)
	if err != nil {
		return err
	}
	key := trafficUsageKeyFromCycle(row.ServerID, row.Iface, cycle)
	if !at.After(progress[key]) {
		return nil
	}
	acc := trafficUsageAccumulatorFor(usage, key, cycle)
	acc.row.ResetCount++
	if acc.row.CoveredFrom.IsZero() || at.Before(acc.row.CoveredFrom) {
		acc.row.CoveredFrom = at
	}
	acc.row.LastCollectedAt = maxTime(acc.row.LastCollectedAt, at)
	acc.row.CoveredUntil = maxTime(acc.row.CoveredUntil, at)
	progress[key] = at
	return nil
}

func serverCycleSettingsFromRow(row trafficUsageNICRow) ServerCycleSettings {
	mode := ServerCycleMode(row.ServerCycleMode)
	day := row.BillingStartDay
	if mode == "" {
		mode = ServerCycleMode(CycleCalendarMonth)
	}
	if day == 0 {
		day = 1
	}
	return ServerCycleSettings{
		Mode:              mode,
		BillingStartDay:   day,
		BillingAnchorDate: row.BillingAnchorDate,
		BillingTimezone:   row.BillingTimezone,
	}
}

func mergeTrafficUsageSample(acc *trafficUsageAccumulator, inBytes, outBytes int64, inRate, outRate float64, gap bool, coveredFrom, coveredUntil time.Time) {
	nextIn, inOK := addTrafficBytes(acc.row.InBytes, inBytes)
	nextOut, outOK := addTrafficBytes(acc.row.OutBytes, outBytes)
	if inOK && outOK {
		acc.row.InBytes = nextIn
		acc.row.OutBytes = nextOut
	} else {
		gap = true
	}
	if acc.row.CoveredFrom.IsZero() || coveredFrom.Before(acc.row.CoveredFrom) {
		acc.row.CoveredFrom = coveredFrom
	}
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
	maxBytes := int64(math.MaxInt64)
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
			"covered_from":            gorm.Expr("LEAST(traffic_month_usage.covered_from, EXCLUDED.covered_from)"),
			"covered_until":           gorm.Expr("GREATEST(traffic_month_usage.covered_until, EXCLUDED.covered_until)"),
			"last_collected_at":       gorm.Expr("GREATEST(traffic_month_usage.last_collected_at, EXCLUDED.last_collected_at)"),
			"in_bytes":                gorm.Expr("CASE WHEN traffic_month_usage.in_bytes > ? - EXCLUDED.in_bytes THEN traffic_month_usage.in_bytes ELSE traffic_month_usage.in_bytes + EXCLUDED.in_bytes END", maxBytes),
			"out_bytes":               gorm.Expr("CASE WHEN traffic_month_usage.out_bytes > ? - EXCLUDED.out_bytes THEN traffic_month_usage.out_bytes ELSE traffic_month_usage.out_bytes + EXCLUDED.out_bytes END", maxBytes),
			"in_peak_bytes_per_sec":   gorm.Expr("GREATEST(traffic_month_usage.in_peak_bytes_per_sec, EXCLUDED.in_peak_bytes_per_sec)"),
			"out_peak_bytes_per_sec":  gorm.Expr("GREATEST(traffic_month_usage.out_peak_bytes_per_sec, EXCLUDED.out_peak_bytes_per_sec)"),
			"both_peak_bytes_per_sec": gorm.Expr("GREATEST(traffic_month_usage.both_peak_bytes_per_sec, EXCLUDED.both_peak_bytes_per_sec)"),
			"sample_count":            gorm.Expr("traffic_month_usage.sample_count + EXCLUDED.sample_count"),
			"gap_count":               gorm.Expr("traffic_month_usage.gap_count + EXCLUDED.gap_count + CASE WHEN traffic_month_usage.in_bytes > ? - EXCLUDED.in_bytes OR traffic_month_usage.out_bytes > ? - EXCLUDED.out_bytes THEN 1 ELSE 0 END", maxBytes, maxBytes),
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

	coveredFrom := row.CoveredFrom
	if coveredFrom.IsZero() {
		coveredFrom = cycle.Start
	}
	coveredUntil := row.CoveredUntil
	if coveredUntil.IsZero() {
		coveredUntil = statEnd
	}
	dataComplete := trafficUsageDataComplete(row.GapCount, row.ResetCount) &&
		!coveredFrom.After(cycle.Start) &&
		(!cycleComplete || !coveredUntil.Before(cycle.End))
	stat := TrafficStat{
		InBytes:             row.InBytes,
		OutBytes:            row.OutBytes,
		InPeakBytesPerSec:   row.InPeakBytesPerSec,
		OutPeakBytesPerSec:  row.OutPeakBytesPerSec,
		BothPeakBytesPerSec: row.BothPeakBytesPerSec,
		SampleCount:         row.SampleCount,
		ExpectedSampleCount: 0,
		EffectiveStart:      coveredFrom,
		EffectiveEnd:        statEnd,
		CoverageRatio:       trafficUsageCoverage(cycle.Start, statEnd, coveredFrom, coveredUntil, row.GapCount, row.ResetCount),
		CoveredUntil:        coveredUntil,
		GapCount:            row.GapCount,
		ResetCount:          row.ResetCount,
		CycleComplete:       cycleComplete,
		DataComplete:        dataComplete,
		Status:              status,
	}
	applyP95Status(&stat, q.UsageMode, false)
	if err := applyTrafficSelection(&stat, q.DirectionMode); err != nil {
		return TrafficSummary{}, err
	}
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
		Where("server_id = ?", q.ServerID).
		Where("cycle_mode = ?", string(cycle.Mode)).
		Where("billing_start_day = ?", cycle.BillingStartDay).
		Where("cycle_start = ? AND cycle_end = ?", cycle.Start, cycle.End).
		Where("iface = ?", iface)

	var row struct {
		InBytes             int64     `gorm:"column:in_bytes"`
		OutBytes            int64     `gorm:"column:out_bytes"`
		InPeakBytesPerSec   float64   `gorm:"column:in_peak_bytes_per_sec"`
		OutPeakBytesPerSec  float64   `gorm:"column:out_peak_bytes_per_sec"`
		BothPeakBytesPerSec float64   `gorm:"column:both_peak_bytes_per_sec"`
		SampleCount         int       `gorm:"column:sample_count"`
		GapCount            int       `gorm:"column:gap_count"`
		ResetCount          int       `gorm:"column:reset_count"`
		CoveredFrom         time.Time `gorm:"column:covered_from"`
		CoveredUntil        time.Time `gorm:"column:covered_until"`
	}
	err := db.
		Select([]string{
			"in_bytes",
			"out_bytes",
			"in_peak_bytes_per_sec",
			"out_peak_bytes_per_sec",
			"both_peak_bytes_per_sec",
			"sample_count",
			"gap_count",
			"reset_count",
			"covered_from",
			"covered_until",
		}).
		Take(&row).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return trafficUsageFetch{}, false, nil
		}
		return trafficUsageFetch{}, false, err
	}
	return trafficUsageFetch{
		InBytes:             row.InBytes,
		OutBytes:            row.OutBytes,
		InPeakBytesPerSec:   row.InPeakBytesPerSec,
		OutPeakBytesPerSec:  row.OutPeakBytesPerSec,
		BothPeakBytesPerSec: row.BothPeakBytesPerSec,
		SampleCount:         row.SampleCount,
		GapCount:            row.GapCount,
		ResetCount:          row.ResetCount,
		CoveredFrom:         row.CoveredFrom,
		CoveredUntil:        row.CoveredUntil,
	}, true, nil
}

func trafficUsageKeyFromCycle(serverID int64, iface string, cycle TrafficCycle) trafficUsageKey {
	return trafficUsageKey{
		serverID:        serverID,
		iface:           iface,
		cycleMode:       string(cycle.Mode),
		billingStartDay: int16(cycle.BillingStartDay),
		cycleStart:      cycle.Start.UTC(),
		cycleEnd:        cycle.End.UTC(),
	}
}

func trafficUsageKeyFromUsage(row model.TrafficMonthUsage) trafficUsageKey {
	return trafficUsageKey{
		serverID:        row.ServerID,
		iface:           row.Iface,
		cycleMode:       row.CycleMode,
		billingStartDay: row.BillingStartDay,
		cycleStart:      row.CycleStart.UTC(),
		cycleEnd:        row.CycleEnd.UTC(),
	}
}

func trafficUsageDataComplete(gaps, resets int) bool {
	return gaps == 0 && resets == 0
}

func trafficUsageCoverage(start, end, coveredFrom, coveredUntil time.Time, gaps, resets int) float64 {
	if !trafficUsageDataComplete(gaps, resets) {
		return 0
	}
	total := end.Sub(start)
	if total <= 0 {
		return 1
	}
	coveredStart := maxTime(start, coveredFrom)
	coveredEnd := minTime(end, coveredUntil)
	if !coveredEnd.After(coveredStart) {
		return 0
	}
	ratio := float64(coveredEnd.Sub(coveredStart)) / float64(total)
	if ratio > 1 {
		return 1
	}
	return ratio
}

func maxTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return b
	}
	return a
}
