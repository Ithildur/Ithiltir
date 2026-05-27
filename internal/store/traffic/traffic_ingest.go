package traffic

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"dash/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type trafficNICRow struct {
	ServerID    int64     `gorm:"column:server_id"`
	Iface       string    `gorm:"column:iface"`
	CollectedAt time.Time `gorm:"column:collected_at"`
	BytesRecv   int64     `gorm:"column:bytes_recv"`
	BytesSent   int64     `gorm:"column:bytes_sent"`
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
		rows, err := loadTrafficSampleRows(tx, start, end)
		if err != nil {
			return fmt.Errorf("load traffic nic rows: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		items := buildTraffic5mRows(rows, start, end)
		if len(items) == 0 {
			return nil
		}
		if err := upsertTraffic5mRows(tx, items); err != nil {
			return fmt.Errorf("upsert traffic 5m rows: %w", err)
		}
		return nil
	})
}

func loadTrafficSampleRows(tx *gorm.DB, start, end time.Time) ([]trafficNICRow, error) {
	var rows []trafficNICRow
	err := tx.Raw(`
	WITH current_rows AS (
		SELECT
			n.server_id,
			n.iface,
			n.collected_at,
			n.bytes_recv,
			n.bytes_sent
		FROM nic_metrics n
		JOIN servers s ON s.id = n.server_id AND s.is_deleted = FALSE
		WHERE n.collected_at >= ? AND n.collected_at <= ?
	),
	scoped_pairs AS (
		SELECT DISTINCT
			server_id,
			iface
		FROM current_rows
	),
	prev_rows AS (
		SELECT
			s.server_id,
			s.iface,
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
				AND n.collected_at > ?
			ORDER BY n.collected_at ASC
			LIMIT 1
		) p ON true
	)
	SELECT server_id, iface, collected_at, bytes_recv, bytes_sent FROM prev_rows
	UNION ALL
	SELECT server_id, iface, collected_at, bytes_recv, bytes_sent FROM current_rows
	UNION ALL
	SELECT server_id, iface, collected_at, bytes_recv, bytes_sent FROM next_rows
	ORDER BY server_id, iface, collected_at
	`, start, end, start, end).Scan(&rows).Error
	return rows, err
}

func upsertTraffic5mRows(tx *gorm.DB, items []model.Traffic5m) error {
	if len(items) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "server_id"},
			{Name: "iface"},
			{Name: "bucket"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"in_bytes",
			"out_bytes",
			"covered_seconds",
			"in_rate_bytes_per_sec",
			"out_rate_bytes_per_sec",
			"in_peak_bytes_per_sec",
			"out_peak_bytes_per_sec",
			"sample_count",
			"gap_count",
			"reset_count",
			"updated_at",
		}),
	}).CreateInBatches(items, 500).Error
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

	for _, sample := range splitTrafficSamplesWindow(current.ServerID, current.Iface, prev.CollectedAt.UTC(), current.CollectedAt.UTC(), start, end, inDelta, outDelta) {
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
	return splitTrafficSamplesWindow(serverID, iface, start, end, start, end, inDelta, outDelta)
}

func splitTrafficSamplesWindow(serverID int64, iface string, pairStart, pairEnd, windowStart, windowEnd time.Time, inDelta, outDelta int64) []trafficSample {
	if !pairEnd.After(pairStart) || !pairEnd.After(windowStart) || !pairStart.Before(windowEnd) {
		return nil
	}
	seconds := pairEnd.Sub(pairStart).Seconds()
	if seconds <= 0 {
		return nil
	}

	inRate := float64(inDelta) / seconds
	outRate := float64(outDelta) / seconds
	start := maxTime(pairStart, windowStart)
	end := minTime(pairEnd, windowEnd)
	if !end.After(start) {
		return nil
	}
	windowSec := end.Sub(start).Seconds()

	type segment struct {
		bucket  time.Time
		start   time.Time
		end     time.Time
		seconds float64
	}
	segments := make([]segment, 0, int(math.Ceil(windowSec/trafficBucketSize.Seconds()))+1)
	for cursor := start; cursor.Before(end); {
		bucket := trafficBucketStart(cursor)
		segEnd := minTime(bucket.Add(trafficBucketSize), end)
		covered := segEnd.Sub(cursor).Seconds()
		if covered > 0 {
			segments = append(segments, segment{
				bucket:  bucket,
				start:   cursor,
				end:     segEnd,
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
	for _, seg := range segments {
		segStart := seg.start.Sub(pairStart).Seconds()
		segEnd := seg.end.Sub(pairStart).Seconds()
		inBytes := int64(math.Round(float64(inDelta)*segEnd/seconds)) - int64(math.Round(float64(inDelta)*segStart/seconds))
		outBytes := int64(math.Round(float64(outDelta)*segEnd/seconds)) - int64(math.Round(float64(outDelta)*segStart/seconds))

		segGap := 0
		if gap && seg.start.Equal(pairStart) {
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
