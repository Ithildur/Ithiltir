package traffic

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrTrafficDailyUnsupported = errors.New("daily traffic requires billing mode")

type TrafficDaily struct {
	ServerID  int64
	Iface     string
	UsageMode UsageMode
	Start     time.Time
	End       time.Time
	Stat      TrafficStat
}

func (s *Store) TrafficDaily(ctx context.Context, q TrafficQuery) ([]TrafficDaily, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store: db is nil")
	}
	q = normalizeTrafficQuery(q)
	if q.ServerID <= 0 {
		return nil, fmt.Errorf("invalid server id")
	}
	if !validTrafficIface(q.Iface) {
		return nil, fmt.Errorf("invalid iface")
	}
	if q.UsageMode != UsageBilling {
		return nil, ErrTrafficDailyUnsupported
	}

	cycle := trafficCycleForPeriod(q.CycleMode, q.BillingStartDay, q.BillingAnchorDate, q.Location, q.Ref, q.Period)
	statEnd, cycleComplete := trafficStatEnd(cycle, q.Ref)
	status := trafficSnapshotStatus(cycle, q.Ref)
	effectiveStart, effectiveEnd, err := s.trafficEffectiveWindow(ctx, q.ServerID, q.Iface, cycle.Start, statEnd)
	if err != nil {
		return nil, err
	}
	rows, err := s.fetchTrafficBuckets(ctx, q, cycle, statEnd)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoTrafficData
	}

	items := buildTrafficDaily(q, rows, effectiveStart, effectiveEnd, statEnd, cycleComplete, status)
	if len(items) == 0 {
		return nil, ErrNoTrafficData
	}
	return items, nil
}

func trafficCycleForPeriod(mode BillingCycleMode, day int, anchorDate string, loc *time.Location, ref time.Time, period TrafficPeriod) TrafficCycle {
	cycle := currentTrafficCycleAnchored(mode, day, anchorDate, loc, ref)
	if period == TrafficPeriodPrev {
		return prevTrafficCycle(cycle)
	}
	return cycle
}

func buildTrafficDaily(q TrafficQuery, rows []trafficBucket, effectiveStart, effectiveEnd, statEnd time.Time, cycleComplete bool, status TrafficSnapshotStatus) []TrafficDaily {
	if !effectiveEnd.After(effectiveStart) {
		return nil
	}

	loc := q.Location
	if loc == nil {
		loc = time.Local
	}
	rowsByDay := make(map[time.Time][]trafficBucket)
	for _, row := range rows {
		day := trafficDayStart(row.Bucket, loc)
		rowsByDay[day] = append(rowsByDay[day], row)
	}

	start := trafficDayStart(effectiveStart, loc)
	end := effectiveEnd.In(loc)
	out := make([]TrafficDaily, 0, 32)
	for cursor := start; cursor.Before(end); cursor = cursor.AddDate(0, 0, 1) {
		dayEnd := cursor.AddDate(0, 0, 1).UTC()
		windowStart := maxTime(cursor.UTC(), effectiveStart)
		windowEnd := minTime(dayEnd, effectiveEnd)
		if !windowEnd.After(windowStart) {
			continue
		}

		dayRows := trafficRowsInWindow(rowsByDay[cursor], windowStart, windowEnd)
		dayComplete := cycleComplete || !dayEnd.After(statEnd)
		stat := buildTrafficStat(dayRows, windowStart, windowEnd, dayComplete, status, q.DirectionMode, q.UsageMode, q.P95Enabled)
		out = append(out, TrafficDaily{
			ServerID:  q.ServerID,
			Iface:     normalizeTrafficIface(q.Iface),
			UsageMode: q.UsageMode,
			Start:     windowStart,
			End:       windowEnd,
			Stat:      stat,
		})
	}
	return out
}

func trafficDayStart(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	local := t.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}

func trafficRowsInWindow(rows []trafficBucket, start, end time.Time) []trafficBucket {
	if len(rows) == 0 {
		return nil
	}
	out := make([]trafficBucket, 0, len(rows))
	for _, row := range rows {
		if row.Bucket.Before(start) || !row.Bucket.Before(end) {
			continue
		}
		out = append(out, row)
	}
	return out
}
