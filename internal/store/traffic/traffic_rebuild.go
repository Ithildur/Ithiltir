package traffic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dash/internal/model"

	"gorm.io/gorm"
)

type ServerTrafficSource struct {
	Start  time.Time
	End    time.Time
	Ifaces []string
}

func (s *Store) ServerTrafficSource(ctx context.Context, serverID int64, start time.Time) (ServerTrafficSource, error) {
	if s == nil || s.db == nil {
		return ServerTrafficSource{}, fmt.Errorf("store: db is nil")
	}
	if serverID <= 0 {
		return ServerTrafficSource{}, fmt.Errorf("invalid server id")
	}
	var rows []struct {
		Iface string    `gorm:"column:iface"`
		Start time.Time `gorm:"column:start_at"`
		End   time.Time `gorm:"column:end_at"`
	}
	q := s.db.WithContext(ctx).
		Table("nic_metrics").
		Select("iface, MIN(collected_at) AS start_at, MAX(collected_at) AS end_at").
		Where("server_id = ?", serverID)
	if !start.IsZero() {
		q = q.Where("collected_at >= ?", start)
	}
	if err := q.Group("iface").Order("iface").Scan(&rows).Error; err != nil {
		return ServerTrafficSource{}, err
	}
	if len(rows) == 0 {
		return ServerTrafficSource{}, nil
	}

	source := ServerTrafficSource{
		Start:  trafficBucketStart(rows[0].Start),
		End:    trafficMetricRangeEnd(rows[0].End),
		Ifaces: make([]string, 0, len(rows)),
	}
	for _, row := range rows {
		if row.Start.Before(source.Start) {
			source.Start = trafficBucketStart(row.Start)
		}
		rowEnd := trafficMetricRangeEnd(row.End)
		if rowEnd.After(source.End) {
			source.End = rowEnd
		}
		source.Ifaces = append(source.Ifaces, row.Iface)
	}
	return source, nil
}

func (s *Store) RebuildServerTraffic5mChunk(ctx context.Context, serverID int64, ifaces []string, start, end time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store: db is nil")
	}
	if serverID <= 0 {
		return fmt.Errorf("invalid server id")
	}
	if !end.After(start) {
		return nil
	}
	if len(ifaces) == 0 {
		return fmt.Errorf("traffic rebuild ifaces is empty")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rows, err := loadServerTrafficNICRows(tx, serverID, ifaces, start, end)
		if err != nil {
			return fmt.Errorf("load server traffic nic rows: %w", err)
		}
		items := buildTraffic5mRows(rows, start, end)

		if err := deleteServerTraffic5m(tx, serverID, ifaces, start, end); err != nil {
			return fmt.Errorf("delete server traffic 5m rows: %w", err)
		}
		if len(items) == 0 {
			return nil
		}
		if err := tx.CreateInBatches(items, 500).Error; err != nil {
			return fmt.Errorf("insert server traffic 5m rows: %w", err)
		}
		return nil
	})
}

func loadServerTrafficNICRows(tx *gorm.DB, serverID int64, ifaces []string, start, end time.Time) ([]trafficNICRow, error) {
	var rows []trafficNICRow
	if serverID <= 0 || len(ifaces) == 0 {
		return rows, nil
	}

	var b strings.Builder
	args := make([]any, 0, len(ifaces)*2+4)
	b.WriteString(`
WITH scoped_pairs(server_id, iface) AS (
	VALUES `)
	for i, iface := range ifaces {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("(?, ?)")
		args = append(args, serverID, iface)
	}
	b.WriteString(`
),
scoped AS (
	SELECT
		p.server_id,
		p.iface,
		COALESCE(NULLIF(s.traffic_cycle_mode, ''), 'default') AS traffic_cycle_mode,
		COALESCE(s.traffic_billing_start_day, 1) AS traffic_billing_start_day,
		COALESCE(s.traffic_billing_anchor_date, '') AS traffic_billing_anchor_date,
		COALESCE(s.traffic_billing_timezone, '') AS traffic_billing_timezone
	FROM scoped_pairs p
	LEFT JOIN servers s ON s.id = p.server_id
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
	WHERE n.collected_at >= ? AND n.collected_at < ?
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
	FROM scoped s
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
	FROM scoped s
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
SELECT server_id, iface, traffic_cycle_mode, traffic_billing_start_day, traffic_billing_anchor_date, traffic_billing_timezone, collected_at, bytes_recv, bytes_sent FROM window_rows
UNION ALL
SELECT server_id, iface, traffic_cycle_mode, traffic_billing_start_day, traffic_billing_anchor_date, traffic_billing_timezone, collected_at, bytes_recv, bytes_sent FROM next_rows
ORDER BY server_id, iface, collected_at
`)
	args = append(args, start, end, start, end)
	err := tx.Raw(b.String(), args...).Scan(&rows).Error
	return rows, err
}

func deleteServerTraffic5m(tx *gorm.DB, serverID int64, ifaces []string, start, end time.Time) error {
	return tx.
		Where("server_id = ? AND iface IN ? AND bucket >= ? AND bucket < ?", serverID, ifaces, start, end).
		Delete(&model.Traffic5m{}).Error
}

func (s *Store) DeleteServerTrafficMonthlySnapshots(ctx context.Context, serverID int64, start, end time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store: db is nil")
	}
	if serverID <= 0 {
		return fmt.Errorf("invalid server id")
	}
	return s.db.WithContext(ctx).
		Where("server_id = ? AND cycle_end > ? AND cycle_start < ?", serverID, start, end).
		Delete(&model.TrafficMonthly{}).Error
}

func trafficMetricRangeEnd(t time.Time) time.Time {
	bucket := trafficBucketStart(t)
	if t.UTC().Equal(bucket) {
		return bucket
	}
	return bucket.Add(trafficBucketSize)
}
