package traffic

import (
	"sort"
	"strings"
	"time"
)

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func trafficBucketStart(t time.Time) time.Time {
	return t.UTC().Truncate(trafficBucketSize)
}

func currentTrafficCycle(mode BillingCycleMode, day int, loc *time.Location, ref time.Time) TrafficCycle {
	return currentTrafficCycleAnchored(mode, day, "", loc, ref)
}

func currentTrafficCycleAnchored(mode BillingCycleMode, day int, anchorDate string, loc *time.Location, ref time.Time) TrafficCycle {
	if loc == nil {
		loc = time.Local
	}
	ref = ref.In(loc)
	mode, _ = NormalizeCycleMode(mode)
	if day < 1 || day > 31 {
		day = 1
	}
	if mode == CycleCalendarMonth {
		day = 1
	}
	anchor := ""
	if mode == CycleWHMCS && strings.TrimSpace(anchorDate) != "" {
		if anchorTime, ok := parseTrafficAnchorDate(anchorDate, loc); ok {
			day = anchorTime.Day()
			anchor = formatTrafficAnchorDate(anchorTime)
		}
	}

	start, next := trafficCycleBounds(mode, day, anchor, loc, ref)

	return TrafficCycle{
		Mode:              mode,
		BillingStartDay:   day,
		BillingAnchorDate: anchor,
		Timezone:          loc.String(),
		Start:             start.UTC(),
		End:               next.UTC(),
	}
}

func prevTrafficCycle(cycle TrafficCycle) TrafficCycle {
	loc, err := time.LoadLocation(cycle.Timezone)
	if err != nil {
		loc = time.Local
	}
	currentStart := cycle.Start.In(loc)
	prev := prevTrafficBoundary(cycle.Mode, cycle.BillingStartDay, cycle.BillingAnchorDate, loc, currentStart)
	return TrafficCycle{
		Mode:              cycle.Mode,
		BillingStartDay:   cycle.BillingStartDay,
		BillingAnchorDate: cycle.BillingAnchorDate,
		Timezone:          cycle.Timezone,
		Start:             prev.UTC(),
		End:               currentStart.UTC(),
	}
}

func closedTrafficCycles(mode BillingCycleMode, day int, anchorDate string, loc *time.Location, ref, since time.Time) []TrafficCycle {
	current := currentTrafficCycleAnchored(mode, day, anchorDate, loc, ref)
	cycle := prevTrafficCycle(current)
	out := make([]TrafficCycle, 0, 2)
	since = since.UTC()
	for i := 0; i < trafficMaxMonthlyMonths; i++ {
		if !cycle.End.After(since) {
			break
		}
		out = append(out, cycle)
		cycle = prevTrafficCycle(cycle)
	}
	return out
}

func trafficCycleBounds(mode BillingCycleMode, day int, anchorDate string, loc *time.Location, ref time.Time) (time.Time, time.Time) {
	if mode == CycleWHMCS {
		return trafficBoundsFrom(whmcsBoundariesAround(day, anchorDate, loc, ref), ref)
	}
	bounds := trafficBoundariesAround(mode, day, loc, ref)
	return trafficBoundsFrom(bounds, ref)
}

func trafficBoundsFrom(bounds []time.Time, ref time.Time) (time.Time, time.Time) {
	for i := 0; i+1 < len(bounds); i++ {
		if !ref.Before(bounds[i]) && ref.Before(bounds[i+1]) {
			return bounds[i], bounds[i+1]
		}
	}
	start := time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, ref.Location())
	return start, start.AddDate(0, 1, 0)
}

func prevTrafficBoundary(mode BillingCycleMode, day int, anchorDate string, loc *time.Location, current time.Time) time.Time {
	if mode == CycleWHMCS {
		return prevBoundaryFrom(whmcsBoundariesAround(day, anchorDate, loc, current), current)
	}
	bounds := trafficBoundariesAround(mode, day, loc, current)
	return prevBoundaryFrom(bounds, current)
}

func prevBoundaryFrom(bounds []time.Time, current time.Time) time.Time {
	var prev time.Time
	for _, bound := range bounds {
		if !bound.Before(current) {
			if !prev.IsZero() {
				return prev
			}
			break
		}
		prev = bound
	}
	return current.AddDate(0, -1, 0)
}

func whmcsBoundariesAround(day int, anchorDate string, loc *time.Location, ref time.Time) []time.Time {
	ref = ref.In(loc)
	if anchor, ok := parseTrafficAnchorDate(anchorDate, loc); ok && !anchor.After(ref) {
		return whmcsBoundariesFromAnchor(anchor, ref)
	}
	anchor := time.Date(ref.Year()-1, time.December, day, 0, 0, 0, 0, loc)
	bounds := make([]time.Time, 0, 30)
	for i := 0; i < 30; i++ {
		bounds = append(bounds, anchor)
		anchor = anchor.AddDate(0, 1, 0)
	}
	return bounds
}

func whmcsBoundariesFromAnchor(anchor, ref time.Time) []time.Time {
	ref = ref.In(anchor.Location())
	bounds := make([]time.Time, 0, 72)
	for len(bounds) < 72 && anchor.Before(ref.AddDate(0, -2, 0)) {
		anchor = anchor.AddDate(0, 1, 0)
	}
	for len(bounds) < 72 {
		bounds = append(bounds, anchor)
		if anchor.After(ref.AddDate(0, 2, 0)) {
			break
		}
		anchor = anchor.AddDate(0, 1, 0)
	}
	return bounds
}

func parseTrafficAnchorDate(raw string, loc *time.Location) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if loc == nil {
		loc = time.Local
	}
	if t, err := time.ParseInLocation(time.DateOnly, raw, loc); err == nil {
		return time.Date(t.In(loc).Year(), t.In(loc).Month(), t.In(loc).Day(), 0, 0, 0, 0, loc), true
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		t = t.In(loc)
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc), true
	}
	return time.Time{}, false
}

func formatTrafficAnchorDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.DateOnly)
}

func trafficBoundariesAround(mode BillingCycleMode, day int, loc *time.Location, ref time.Time) []time.Time {
	startYear := ref.In(loc).Year() - 2
	bounds := make([]time.Time, 0, 60)
	for i := 0; i < 60; i++ {
		year := startYear + i/12
		month := time.Month(i%12 + 1)
		bounds = append(bounds, trafficBoundaryInMonth(mode, day, year, month, loc))
	}
	sort.Slice(bounds, func(i, j int) bool {
		return bounds[i].Before(bounds[j])
	})

	out := bounds[:0]
	for _, bound := range bounds {
		if len(out) == 0 || !bound.Equal(out[len(out)-1]) {
			out = append(out, bound)
		}
	}
	return out
}

func trafficBoundaryInMonth(mode BillingCycleMode, day, year int, month time.Month, loc *time.Location) time.Time {
	switch mode {
	case CycleClampMonthEnd:
		return time.Date(year, month, minInt(day, daysInMonth(year, month)), 0, 0, 0, 0, loc)
	case CycleWHMCS:
		return time.Date(year, month, day, 0, 0, 0, 0, loc)
	default:
		return time.Date(year, month, 1, 0, 0, 0, 0, loc)
	}
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
