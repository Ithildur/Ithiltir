package traffic

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type cycleRule struct {
	mode   BillingCycleMode
	day    int
	anchor string
	loc    *time.Location
}

func newCycleRule(mode BillingCycleMode, day int, anchorDate string, loc *time.Location) (cycleRule, error) {
	if loc == nil {
		return cycleRule{}, fmt.Errorf("traffic cycle location is nil")
	}
	normalized, ok := NormalizeCycleMode(mode)
	if !ok {
		return cycleRule{}, fmt.Errorf("invalid traffic cycle mode %q", mode)
	}
	if day < 1 || day > 31 {
		return cycleRule{}, fmt.Errorf("invalid traffic billing start day %d", day)
	}

	anchor := strings.TrimSpace(anchorDate)
	switch normalized {
	case CycleCalendarMonth:
		if day != 1 || anchor != "" {
			return cycleRule{}, fmt.Errorf("calendar month requires day 1 and no anchor")
		}
	case CycleClampMonthEnd:
		if anchor != "" {
			return cycleRule{}, fmt.Errorf("clamped billing cycle cannot have an anchor")
		}
	case CycleWHMCS:
		anchorTime, valid := parseTrafficAnchorDate(anchor, loc)
		if !valid {
			return cycleRule{}, fmt.Errorf("invalid traffic billing anchor date %q", anchorDate)
		}
		if day != anchorTime.Day() {
			return cycleRule{}, fmt.Errorf("traffic billing start day does not match anchor date")
		}
		anchor = formatTrafficAnchorDate(anchorTime)
	default:
		return cycleRule{}, fmt.Errorf("invalid traffic cycle mode %q", normalized)
	}

	return cycleRule{mode: normalized, day: day, anchor: anchor, loc: loc}, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func trafficBucketStart(t time.Time) time.Time {
	return t.UTC().Truncate(trafficBucketSize)
}

func (r cycleRule) at(ref time.Time) (TrafficCycle, error) {
	ref = ref.In(r.loc)
	start, next, err := trafficCycleBounds(r.mode, r.day, r.anchor, r.loc, ref)
	if err != nil {
		return TrafficCycle{}, err
	}

	return TrafficCycle{
		Mode:              r.mode,
		BillingStartDay:   r.day,
		BillingAnchorDate: r.anchor,
		Timezone:          r.loc.String(),
		Start:             start.UTC(),
		End:               next.UTC(),
		location:          r.loc,
	}, nil
}

func (r cycleRule) previous(cycle TrafficCycle) (TrafficCycle, error) {
	currentStart := cycle.Start.In(r.loc)
	prev, err := prevTrafficBoundary(r.mode, r.day, r.anchor, r.loc, currentStart)
	if err != nil {
		return TrafficCycle{}, err
	}
	return TrafficCycle{
		Mode:              r.mode,
		BillingStartDay:   r.day,
		BillingAnchorDate: r.anchor,
		Timezone:          r.loc.String(),
		Start:             prev.UTC(),
		End:               currentStart.UTC(),
		location:          r.loc,
	}, nil
}

func (r cycleRule) closed(ref, since time.Time) ([]TrafficCycle, error) {
	current, err := r.at(ref)
	if err != nil {
		return nil, err
	}
	cycle, err := r.previous(current)
	if err != nil {
		return nil, err
	}
	out := make([]TrafficCycle, 0, 2)
	since = since.UTC()
	for range trafficMaxMonthlyMonths {
		if !cycle.End.After(since) {
			break
		}
		out = append(out, cycle)
		cycle, err = r.previous(cycle)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func trafficCycleBounds(mode BillingCycleMode, day int, anchorDate string, loc *time.Location, ref time.Time) (time.Time, time.Time, error) {
	bounds, err := cycleBoundariesAround(mode, day, anchorDate, loc, ref)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return trafficBoundsFrom(bounds, ref)
}

func trafficBoundsFrom(bounds []time.Time, ref time.Time) (time.Time, time.Time, error) {
	for i := 0; i+1 < len(bounds); i++ {
		if !ref.Before(bounds[i]) && ref.Before(bounds[i+1]) {
			return bounds[i], bounds[i+1], nil
		}
	}
	return time.Time{}, time.Time{}, fmt.Errorf("traffic cycle bounds do not contain %s", ref.Format(time.RFC3339))
}

func prevTrafficBoundary(mode BillingCycleMode, day int, anchorDate string, loc *time.Location, current time.Time) (time.Time, error) {
	bounds, err := cycleBoundariesAround(mode, day, anchorDate, loc, current)
	if err != nil {
		return time.Time{}, err
	}
	return prevBoundaryFrom(bounds, current)
}

func cycleBoundariesAround(mode BillingCycleMode, day int, anchorDate string, loc *time.Location, ref time.Time) ([]time.Time, error) {
	switch mode {
	case CycleCalendarMonth, CycleClampMonthEnd:
		return trafficBoundariesAround(mode, day, loc, ref)
	case CycleWHMCS:
		return whmcsBoundariesAround(day, anchorDate, loc, ref)
	default:
		return nil, fmt.Errorf("invalid traffic cycle mode %q", mode)
	}
}

func prevBoundaryFrom(bounds []time.Time, current time.Time) (time.Time, error) {
	var prev time.Time
	for _, bound := range bounds {
		if !bound.Before(current) {
			if !prev.IsZero() {
				return prev, nil
			}
			break
		}
		prev = bound
	}
	return time.Time{}, fmt.Errorf("traffic cycle has no boundary before %s", current.Format(time.RFC3339))
}

func whmcsBoundariesAround(day int, anchorDate string, loc *time.Location, ref time.Time) ([]time.Time, error) {
	ref = ref.In(loc)
	anchor, ok := parseTrafficAnchorDate(anchorDate, loc)
	if !ok {
		return nil, fmt.Errorf("invalid traffic billing anchor date %q", anchorDate)
	}
	if !anchor.After(ref) {
		return whmcsBoundariesFromAnchor(anchor, ref), nil
	}
	anchor = time.Date(ref.Year()-1, time.December, day, 0, 0, 0, 0, loc)
	bounds := make([]time.Time, 0, 30)
	for i := 0; i < 30; i++ {
		bounds = append(bounds, anchor)
		anchor = anchor.AddDate(0, 1, 0)
	}
	return bounds, nil
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
	if raw == "" || loc == nil {
		return time.Time{}, false
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

func trafficBoundariesAround(mode BillingCycleMode, day int, loc *time.Location, ref time.Time) ([]time.Time, error) {
	startYear := ref.In(loc).Year() - 2
	bounds := make([]time.Time, 0, 60)
	for i := 0; i < 60; i++ {
		year := startYear + i/12
		month := time.Month(i%12 + 1)
		bound, err := trafficBoundaryInMonth(mode, day, year, month, loc)
		if err != nil {
			return nil, err
		}
		bounds = append(bounds, bound)
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
	return out, nil
}

func trafficBoundaryInMonth(mode BillingCycleMode, day, year int, month time.Month, loc *time.Location) (time.Time, error) {
	switch mode {
	case CycleCalendarMonth:
		return time.Date(year, month, 1, 0, 0, 0, 0, loc), nil
	case CycleClampMonthEnd:
		return time.Date(year, month, minInt(day, daysInMonth(year, month)), 0, 0, 0, 0, loc), nil
	case CycleWHMCS:
		return time.Date(year, month, day, 0, 0, 0, 0, loc), nil
	default:
		return time.Time{}, fmt.Errorf("invalid traffic cycle mode %q", mode)
	}
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
