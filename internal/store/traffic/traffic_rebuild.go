package traffic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"dash/internal/model"

	"gorm.io/gorm"
)

var ErrTrafficFactsDisabled = errors.New("traffic facts are disabled")

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
	// Keep source bounds out of iface grouping on compressed nic_metrics chunks.
	// TimescaleDB can assert on varchar GROUP BY when MIN/MAX is grouped by
	// iface; keep the time aggregate separate and let the DB return only the
	// small iface set.
	var bounds struct {
		Start sql.NullTime `gorm:"column:start_at"`
		End   sql.NullTime `gorm:"column:end_at"`
	}
	boundsQ := s.db.WithContext(ctx).
		Table("nic_metrics").
		Select("MIN(collected_at) AS start_at, MAX(collected_at) AS end_at").
		Where("server_id = ?", serverID)
	if !start.IsZero() {
		boundsQ = boundsQ.Where("collected_at >= ?", start)
	}
	if err := boundsQ.Scan(&bounds).Error; err != nil {
		return ServerTrafficSource{}, err
	}
	if !bounds.Start.Valid || !bounds.End.Valid || bounds.Start.Time.IsZero() || bounds.End.Time.IsZero() {
		return ServerTrafficSource{}, nil
	}
	startAt := bounds.Start.Time.UTC()
	endAt := bounds.End.Time.UTC()

	var ifaces []string
	ifacesQ := s.db.WithContext(ctx).
		Table("nic_metrics").
		Distinct("iface").
		Where("server_id = ?", serverID).
		Order("iface")
	if !start.IsZero() {
		ifacesQ = ifacesQ.Where("collected_at >= ?", start)
	}
	if err := ifacesQ.Pluck("iface", &ifaces).Error; err != nil {
		return ServerTrafficSource{}, err
	}
	if len(ifaces) == 0 {
		return ServerTrafficSource{}, nil
	}

	return ServerTrafficSource{
		Start:  trafficBucketStart(startAt),
		End:    trafficMetricRangeEnd(endAt),
		Ifaces: ifaces,
	}, nil
}

func (s *Store) RebuildTraffic5mChunk(ctx context.Context, serverID int64, ifaces []string, start, end time.Time) error {
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
		enabled, err := lockFactsEnabled(tx)
		if err != nil {
			return err
		}
		if !enabled {
			return ErrTrafficFactsDisabled
		}
		rows, err := loadRebuildNICRows(tx, serverID, ifaces, start, end)
		if err != nil {
			return fmt.Errorf("load server traffic nic rows: %w", err)
		}
		items := buildTraffic5mRows(rows, start, end)

		if err := deleteTraffic5mRange(tx, serverID, ifaces, start, end); err != nil {
			return fmt.Errorf("delete server traffic 5m rows: %w", err)
		}
		if len(items) > 0 {
			if err := tx.CreateInBatches(items, 500).Error; err != nil {
				return fmt.Errorf("insert server traffic 5m rows: %w", err)
			}
		}
		if err := deleteTrafficMonthlySnapshots(tx, serverID, start, end); err != nil {
			return fmt.Errorf("delete server traffic monthly snapshots: %w", err)
		}
		return nil
	})
}

func loadRebuildNICRows(tx *gorm.DB, serverID int64, ifaces []string, start, end time.Time) ([]trafficNICRow, error) {
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
		b.WriteString("(?::bigint, ?::text)")
		args = append(args, serverID, iface)
	}
	b.WriteString(`
	),
	window_rows AS (
		SELECT
			n.server_id,
			n.iface,
			n.collected_at,
			n.bytes_recv,
			n.bytes_sent
		FROM nic_metrics n
		JOIN scoped_pairs s ON s.server_id = n.server_id AND s.iface = n.iface
		WHERE n.collected_at >= ? AND n.collected_at < ?
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
				AND n.collected_at >= ?
			ORDER BY n.collected_at ASC
			LIMIT 1
		) p ON true
	)
	SELECT server_id, iface, collected_at, bytes_recv, bytes_sent FROM prev_rows
	UNION ALL
	SELECT server_id, iface, collected_at, bytes_recv, bytes_sent FROM window_rows
	UNION ALL
	SELECT server_id, iface, collected_at, bytes_recv, bytes_sent FROM next_rows
	ORDER BY server_id, iface, collected_at
	`)
	args = append(args, start, end, start, end)
	err := tx.Raw(b.String(), args...).Scan(&rows).Error
	return rows, err
}

func deleteTraffic5mRange(tx *gorm.DB, serverID int64, ifaces []string, start, end time.Time) error {
	return tx.
		Where("server_id = ? AND iface IN ? AND bucket >= ? AND bucket < ?", serverID, ifaces, start, end).
		Delete(&model.Traffic5m{}).Error
}

func deleteTrafficMonthlySnapshots(tx *gorm.DB, serverID int64, start, end time.Time) error {
	return tx.
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
