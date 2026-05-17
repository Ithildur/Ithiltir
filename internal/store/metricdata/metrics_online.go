package metricdata

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

type OnlineRange string

const (
	OnlineRange24h OnlineRange = "24h"
	OnlineRange7d  OnlineRange = "7d"
)

type OnlinePoint struct {
	TS     time.Time `json:"ts"`
	Status int       `json:"status"`
	Rate   float64   `json:"rate"`
}

var ErrServerNotFound = errors.New("server not found")

type onlineRow struct {
	TS          time.Time `gorm:"column:ts"`
	SampleCount int64     `gorm:"column:sample_count"`
	IntervalSec int       `gorm:"column:interval_sec"`
}

func (s *Store) FetchOnlinePoints(ctx context.Context, serverID int64, rng OnlineRange) ([]OnlinePoint, time.Duration, error) {
	if s == nil || s.db == nil {
		return nil, 0, fmt.Errorf("store: db is nil")
	}
	if serverID <= 0 {
		return nil, 0, fmt.Errorf("invalid server id")
	}

	step, span, err := onlineRangeSpec(rng)
	if err != nil {
		return nil, 0, err
	}

	query := `
WITH params AS (
    SELECT ?::bigint AS server_id,
           ?::interval AS step,
           ?::interval AS span,
           COALESCE(NULLIF(s.interval_sec, 0), 3) AS interval_sec
    FROM servers s
    WHERE s.id = ?
),
series AS (
    SELECT generate_series(
        time_bucket(step, now()) - span,
        time_bucket(step, now()) - step,
        step
    ) AS ts,
    interval_sec
    FROM params
),
raw AS (
    SELECT time_bucket(step, bucket) AS ts,
           sum(sample_count)::bigint AS sample_count
    FROM server_online_30m, params
    WHERE server_id = params.server_id
      AND bucket >= time_bucket(step, now()) - span
      AND bucket < time_bucket(step, now())
    GROUP BY ts
)
SELECT series.ts,
       COALESCE(raw.sample_count, 0) AS sample_count,
       series.interval_sec
FROM series
LEFT JOIN raw ON raw.ts = series.ts
ORDER BY series.ts
`

	rows := make([]onlineRow, 0)
	if err := s.db.WithContext(ctx).Raw(query, serverID, formatInterval(step), formatInterval(span), serverID).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return nil, step, ErrServerNotFound
	}

	points := make([]OnlinePoint, 0, len(rows))
	stepSeconds := int(step.Seconds())
	for _, row := range rows {
		interval := row.IntervalSec
		if interval <= 0 {
			interval = 3
		}
		expected := int(math.Ceil(float64(stepSeconds) / float64(interval)))
		if expected <= 0 {
			expected = 1
		}
		rate := float64(row.SampleCount) / float64(expected)
		status := onlineStatus(rate)
		points = append(points, OnlinePoint{
			TS:     row.TS,
			Status: status,
			Rate:   rate,
		})
	}
	return points, step, nil
}

func onlineStatus(rate float64) int {
	switch {
	case rate >= 0.995:
		return 0
	case rate >= 0.99:
		return 1
	default:
		return 2
	}
}

func onlineRangeSpec(rng OnlineRange) (step, span time.Duration, err error) {
	switch rng {
	case OnlineRange24h:
		return 30 * time.Minute, 24 * time.Hour, nil
	case OnlineRange7d:
		return 210 * time.Minute, 7 * 24 * time.Hour, nil
	default:
		return 0, 0, fmt.Errorf("invalid range")
	}
}
