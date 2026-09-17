package metricdata

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const UptimeDays = 45

type UptimeDay struct {
	Date       string   `json:"date"`
	Percent    *float64 `json:"percent"`
	ObservedMS int64    `json:"observed_ms"`
}

type NodeUptime struct {
	ServerID string      `json:"server_id"`
	Days     []UptimeDay `json:"days"`
}

type UptimeHours struct {
	Date       string       `json:"date"`
	Hours      [24]*float64 `json:"hours"`
	ObservedMS [24]int64    `json:"observed_ms"`
}

func UptimeStart(now time.Time, loc *time.Location) time.Time {
	return UptimeDayStart(uptimeNoon(now.In(loc)).AddDate(0, 0, 1-UptimeDays))
}

func uptimeNoon(day time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, day.Location())
}

// UptimeDayStart finds the first observable minute of a local date, including
// timezones whose DST transition skips or repeats midnight.
func UptimeDayStart(day time.Time) time.Time {
	date := day.Format(time.DateOnly)
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	for start.Format(time.DateOnly) < date {
		start = start.Add(time.Minute)
	}
	for start.Add(-time.Minute).Format(time.DateOnly) == date {
		start = start.Add(-time.Minute)
	}
	return start
}

func (s *Store) FetchUptime(ctx context.Context, now time.Time, loc *time.Location, authorized bool) ([]NodeUptime, error) {
	start := UptimeStart(now, loc)
	firstDate := uptimeNoon(now.In(loc)).AddDate(0, 0, 1-UptimeDays)
	// Calendar boundaries come from Go's configured location, including DST and
	// the system-local timezone, which need not have a PostgreSQL timezone name.
	bounds := make([]string, UptimeDays)
	args := []any{authorized}
	for day := range UptimeDays {
		bounds[day] = "?::timestamptz"
		args = append(args, UptimeDayStart(firstDate.AddDate(0, 0, day)))
	}
	args = append(args, start, now, UptimeDays-1)
	// Aggregate the retained buckets once before filling the small node/date grid.
	// Joining that grid to the continuous aggregate first repeats its work.
	query := `WITH eligible AS MATERIALIZED (
		SELECT id FROM servers WHERE NOT is_deleted AND (? OR is_guest_visible)
	), totals AS MATERIALIZED (
		SELECT o.server_id, width_bucket(o.bucket, ARRAY[` + strings.Join(bounds, ",") + `]) - 1 AS day,
		       sum(o.observed_ms)::bigint AS observed_ms, sum(o.online_ms)::bigint AS online_ms
		FROM node_online_1h o JOIN eligible s ON s.id = o.server_id
		WHERE o.bucket >= ? AND o.bucket <= ?
		GROUP BY o.server_id, day
	)
	SELECT s.id AS server_id, d.day, COALESCE(o.observed_ms, 0) AS observed_ms, COALESCE(o.online_ms, 0) AS online_ms
	FROM eligible s CROSS JOIN generate_series(0, ?) AS d(day)
	LEFT JOIN totals o ON o.server_id = s.id AND o.day = d.day
	ORDER BY s.id, d.day`
	var rows []struct {
		ServerID, ObservedMS, OnlineMS int64
		Day                            int
	}
	if err := s.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	nodes := make([]NodeUptime, 0)
	var previous int64
	for _, row := range rows {
		if row.ServerID != previous {
			nodes = append(nodes, NodeUptime{ServerID: fmt.Sprint(row.ServerID), Days: make([]UptimeDay, 0, UptimeDays)})
			previous = row.ServerID
		}
		days := &nodes[len(nodes)-1].Days
		*days = append(*days, UptimeDay{
			Date:    firstDate.AddDate(0, 0, row.Day).Format(time.DateOnly),
			Percent: uptimePercent(row.OnlineMS, row.ObservedMS), ObservedMS: row.ObservedMS,
		})
	}
	return nodes, nil
}

func (s *Store) FetchUptimeDay(ctx context.Context, serverID int64, day, now time.Time, authorized bool) (UptimeHours, error) {
	out := UptimeHours{Date: day.Format(time.DateOnly)}
	var rows []struct {
		Bucket               *time.Time
		OnlineMS, ObservedMS int64
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT o.bucket, COALESCE(o.online_ms, 0) AS online_ms, COALESCE(o.observed_ms, 0) AS observed_ms
		FROM servers s LEFT JOIN node_online_1h o ON o.server_id = s.id
		 AND o.bucket >= ? AND o.bucket < ? AND o.bucket <= ?
		WHERE s.id = ? AND NOT s.is_deleted AND (? OR s.is_guest_visible)
	`, day, UptimeDayStart(uptimeNoon(day).AddDate(0, 0, 1)), now, serverID, authorized).Scan(&rows).Error; err != nil {
		return out, err
	}
	if len(rows) == 0 {
		return out, ErrServerNotFound
	}
	var online [24]int64
	for _, row := range rows {
		if row.Bucket == nil {
			continue
		}
		hour := row.Bucket.In(day.Location()).Hour()
		out.ObservedMS[hour] += row.ObservedMS
		online[hour] += row.OnlineMS
	}
	for hour := range out.Hours {
		out.Hours[hour] = uptimePercent(online[hour], out.ObservedMS[hour])
	}
	return out, nil
}

func uptimePercent(onlineMS, observedMS int64) *float64 {
	if observedMS == 0 {
		return nil
	}
	return new(100 * float64(onlineMS) / float64(observedMS))
}
