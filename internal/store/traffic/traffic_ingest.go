package traffic

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"dash/internal/model"

	"gorm.io/gorm"
)

type trafficNICRow struct {
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

func (s *Store) BackfillTraffic5m(ctx context.Context, start, end time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store: db is nil")
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
		items := buildTraffic5mRows(rows, start, end)

		if err := tx.
			Where("bucket >= ? AND bucket < ?", start, end).
			Delete(&model.Traffic5m{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return tx.CreateInBatches(items, 500).Error
	})
}

func (s *Store) BackfillTraffic5mMissing(ctx context.Context, lookback time.Duration, end time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store: db is nil")
	}
	if lookback <= 0 {
		return nil
	}
	if end.IsZero() {
		end = time.Now().UTC()
	}
	end = trafficBucketStart(end)
	start := trafficBucketStart(end.Add(-lookback))
	if !end.After(start) {
		return nil
	}

	gap, ok, err := s.nextTraffic5mGap(ctx, start, end)
	if err != nil || !ok {
		return err
	}
	gapEnd := minTime(gap.Add(trafficCatchupWindow), end)
	if !gapEnd.After(gap) {
		gapEnd = minTime(gap.Add(trafficBucketSize), end)
	}
	if !gapEnd.After(gap) {
		return nil
	}
	return s.BackfillTraffic5m(ctx, gap, gapEnd)
}

func (s *Store) nextTraffic5mGap(ctx context.Context, start, end time.Time) (time.Time, bool, error) {
	var rows []struct {
		Bucket time.Time `gorm:"column:bucket"`
	}
	err := s.db.WithContext(ctx).Raw(`
WITH raw_rows AS (
	SELECT
		server_id,
		iface,
		collected_at,
		lag(collected_at) OVER (
			PARTITION BY server_id, iface
			ORDER BY collected_at
		) AS prev_at
	FROM nic_metrics
	WHERE collected_at >= ? AND collected_at <= ?
),
raw_buckets AS (
	SELECT DISTINCT
		server_id,
		iface,
		gs.bucket
	FROM raw_rows
	CROSS JOIN LATERAL generate_series(
		time_bucket('5 minutes', prev_at),
		time_bucket('5 minutes', collected_at - INTERVAL '1 microsecond'),
		INTERVAL '5 minutes'
	) AS gs(bucket)
	WHERE collected_at >= ? AND prev_at IS NOT NULL AND collected_at > prev_at
)
SELECT rb.bucket
FROM raw_buckets rb
WHERE NOT EXISTS (
	SELECT 1
	FROM traffic_5m t
	WHERE t.server_id = rb.server_id
		AND t.iface = rb.iface
		AND t.bucket = rb.bucket
)
	AND rb.bucket >= ?
ORDER BY rb.bucket ASC
LIMIT 1
`, start.Add(-trafficBucketSize), end, start, start).Scan(&rows).Error
	if err != nil {
		return time.Time{}, false, err
	}
	if len(rows) == 0 {
		return time.Time{}, false, nil
	}
	return rows[0].Bucket, true, nil
}

func loadTrafficNICRows(tx *gorm.DB, start, end time.Time) ([]trafficNICRow, error) {
	var rows []trafficNICRow
	err := tx.Raw(`
WITH scoped AS (
	SELECT
		n.server_id,
		n.iface,
		COALESCE(NULLIF(s.traffic_cycle_mode, ''), 'default') AS traffic_cycle_mode,
		COALESCE(s.traffic_billing_start_day, 1) AS traffic_billing_start_day,
		COALESCE(s.traffic_billing_anchor_date, '') AS traffic_billing_anchor_date,
		COALESCE(s.traffic_billing_timezone, '') AS traffic_billing_timezone
	FROM nic_metrics n
	LEFT JOIN servers s ON s.id = n.server_id
	WHERE n.collected_at >= ? AND n.collected_at <= ?
	GROUP BY
		n.server_id,
		n.iface,
		COALESCE(NULLIF(s.traffic_cycle_mode, ''), 'default'),
		COALESCE(s.traffic_billing_start_day, 1),
		COALESCE(s.traffic_billing_anchor_date, ''),
		COALESCE(s.traffic_billing_timezone, '')
),
window_rows AS (
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
	JOIN scoped s ON s.server_id = n.server_id AND s.iface = n.iface
	WHERE n.collected_at >= ? AND n.collected_at <= ?
),
prev_rows AS (
	SELECT DISTINCT ON (n.server_id, n.iface)
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
	JOIN scoped s ON s.server_id = n.server_id AND s.iface = n.iface
	WHERE n.collected_at < ?
	ORDER BY n.server_id, n.iface, n.collected_at DESC
)
SELECT server_id, iface, traffic_cycle_mode, traffic_billing_start_day, traffic_billing_anchor_date, traffic_billing_timezone, collected_at, bytes_recv, bytes_sent FROM prev_rows
UNION ALL
SELECT server_id, iface, traffic_cycle_mode, traffic_billing_start_day, traffic_billing_anchor_date, traffic_billing_timezone, collected_at, bytes_recv, bytes_sent FROM window_rows
ORDER BY server_id, iface, collected_at
`, start, end, start, end, start).Scan(&rows).Error
	return rows, err
}

type traffic5mAccumulator struct {
	row      model.Traffic5m
	validSec float64
	validIn  int64
	validOut int64
	invalid  bool
}

type traffic5mKey struct {
	serverID int64
	iface    string
	bucket   time.Time
}

func buildTraffic5mRows(rows []trafficNICRow, start, end time.Time) []model.Traffic5m {
	buckets := make(map[traffic5mKey]*traffic5mAccumulator)
	var prev trafficNICRow
	hasPrev := false

	for _, row := range rows {
		if !hasPrev || prev.ServerID != row.ServerID || prev.Iface != row.Iface {
			prev = row
			hasPrev = true
			continue
		}

		if row.CollectedAt.After(prev.CollectedAt) {
			mergeTrafficPair(buckets, prev, row, start, end)
		}
		prev = row
	}

	out := make([]model.Traffic5m, 0, len(buckets))
	for _, bucket := range buckets {
		if !bucket.invalid && bucket.validSec >= trafficMinCoveredSec {
			bucket.row.SampleCount = 1
			bucket.row.InRateBytesPerSec = float64(bucket.validIn) / bucket.validSec
			bucket.row.OutRateBytesPerSec = float64(bucket.validOut) / bucket.validSec
			bucket.row.InPeakBytesPerSec = bucket.row.InRateBytesPerSec
			bucket.row.OutPeakBytesPerSec = bucket.row.OutRateBytesPerSec
		} else {
			bucket.row.SampleCount = 0
			bucket.row.InRateBytesPerSec = 0
			bucket.row.OutRateBytesPerSec = 0
			bucket.row.InPeakBytesPerSec = 0
			bucket.row.OutPeakBytesPerSec = 0
		}
		out = append(out, bucket.row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ServerID != out[j].ServerID {
			return out[i].ServerID < out[j].ServerID
		}
		if out[i].Iface != out[j].Iface {
			return out[i].Iface < out[j].Iface
		}
		return out[i].Bucket.Before(out[j].Bucket)
	})
	return out
}

func mergeTrafficPair(buckets map[traffic5mKey]*traffic5mAccumulator, prev, current trafficNICRow, start, end time.Time) {
	inDelta := current.BytesRecv - prev.BytesRecv
	outDelta := current.BytesSent - prev.BytesSent
	if inDelta < 0 || outDelta < 0 {
		bucket := trafficBucketStart(current.CollectedAt)
		if !bucket.Before(start) && bucket.Before(end) {
			acc := traffic5mAccumulatorFor(buckets, current.ServerID, current.Iface, bucket)
			acc.row.ResetCount++
			acc.invalid = true
		}
		return
	}

	for _, sample := range splitTrafficSamples(current.ServerID, current.Iface, prev.CollectedAt.UTC(), current.CollectedAt.UTC(), inDelta, outDelta) {
		if sample.Bucket.Before(start) || !sample.Bucket.Before(end) {
			continue
		}
		mergeTrafficSample(traffic5mAccumulatorFor(buckets, sample.ServerID, sample.Iface, sample.Bucket), sample)
	}
}

func traffic5mAccumulatorFor(buckets map[traffic5mKey]*traffic5mAccumulator, serverID int64, iface string, bucket time.Time) *traffic5mAccumulator {
	key := traffic5mKey{serverID: serverID, iface: iface, bucket: bucket}
	acc := buckets[key]
	if acc != nil {
		return acc
	}
	acc = &traffic5mAccumulator{
		row: model.Traffic5m{
			ServerID: serverID,
			Iface:    iface,
			Bucket:   bucket,
		},
	}
	buckets[key] = acc
	return acc
}

func mergeTrafficSample(acc *traffic5mAccumulator, sample trafficSample) {
	acc.row.InBytes += sample.InBytes
	acc.row.OutBytes += sample.OutBytes
	acc.row.CoveredSec += sample.Seconds
	acc.row.GapCount += int32(sample.Gap)
	if sample.Gap > 0 || !sample.Valid {
		acc.invalid = true
	}
	if !sample.Valid {
		return
	}
	acc.validSec += sample.Seconds
	acc.validIn += sample.InBytes
	acc.validOut += sample.OutBytes
	acc.row.InPeakBytesPerSec = math.Max(acc.row.InPeakBytesPerSec, sample.InRate)
	acc.row.OutPeakBytesPerSec = math.Max(acc.row.OutPeakBytesPerSec, sample.OutRate)
}

type trafficSample struct {
	ServerID int64
	Iface    string
	Bucket   time.Time
	InBytes  int64
	OutBytes int64
	Seconds  float64
	InRate   float64
	OutRate  float64
	Gap      int
	Valid    bool
}

func splitTrafficSamples(serverID int64, iface string, start, end time.Time, inDelta, outDelta int64) []trafficSample {
	if !end.After(start) {
		return nil
	}
	seconds := end.Sub(start).Seconds()
	if seconds <= 0 {
		return nil
	}

	inRate := float64(inDelta) / seconds
	outRate := float64(outDelta) / seconds
	type segment struct {
		bucket  time.Time
		seconds float64
	}
	segments := make([]segment, 0, int(math.Ceil(seconds/trafficBucketSize.Seconds()))+1)
	for cursor := start; cursor.Before(end); {
		bucket := trafficBucketStart(cursor)
		segEnd := minTime(bucket.Add(trafficBucketSize), end)
		covered := segEnd.Sub(cursor).Seconds()
		if covered > 0 {
			segments = append(segments, segment{
				bucket:  bucket,
				seconds: covered,
			})
		}
		cursor = segEnd
	}
	if len(segments) == 0 {
		return nil
	}

	gap := seconds > trafficMaxBillingGap.Seconds()

	out := make([]trafficSample, 0, len(segments))
	var elapsed float64
	var assignedIn int64
	var assignedOut int64
	for i, seg := range segments {
		elapsed += seg.seconds
		inBytes := int64(math.Round(float64(inDelta)*elapsed/seconds)) - assignedIn
		outBytes := int64(math.Round(float64(outDelta)*elapsed/seconds)) - assignedOut
		if i == len(segments)-1 {
			inBytes = inDelta - assignedIn
			outBytes = outDelta - assignedOut
		}
		assignedIn += inBytes
		assignedOut += outBytes

		segGap := 0
		if i == 0 && gap {
			segGap = 1
		}
		sampleInRate := inRate
		sampleOutRate := outRate
		if gap {
			sampleInRate = 0
			sampleOutRate = 0
		}
		out = append(out, trafficSample{
			ServerID: serverID,
			Iface:    iface,
			Bucket:   seg.bucket,
			InBytes:  inBytes,
			OutBytes: outBytes,
			Seconds:  seg.seconds,
			InRate:   sampleInRate,
			OutRate:  sampleOutRate,
			Gap:      segGap,
			Valid:    !gap,
		})
	}
	return out
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
