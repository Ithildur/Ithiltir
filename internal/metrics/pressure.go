package metrics

import (
	"math"

	"dash/internal/model"
)

func normalizePressure(in *Pressure) *Pressure {
	if in == nil {
		return nil
	}
	out := Pressure{
		CPU:    normalizePressureResource(in.CPU),
		Memory: normalizePressureResource(in.Memory),
		IO:     normalizePressureResource(in.IO),
	}
	if out.CPU == nil && out.Memory == nil && out.IO == nil {
		return nil
	}
	return &out
}

func normalizePressureResource(in *PressureResource) *PressureResource {
	if in == nil {
		return nil
	}
	out := PressureResource{
		Some: clonePressureStats(in.Some),
		Full: clonePressureStats(in.Full),
	}
	if out.Some == nil && out.Full == nil {
		return nil
	}
	return &out
}

func clonePressureStats(in *PressureStats) *PressureStats {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func applyPressureSnapshot(snap *model.MetricsSnapshot, pressure *Pressure) {
	pressure = normalizePressure(pressure)
	if snap == nil || pressure == nil {
		return
	}
	if pressure.CPU != nil {
		setPressureStats(&snap.PSICPUSomeAvg10, &snap.PSICPUSomeAvg60, &snap.PSICPUSomeAvg300, &snap.PSICPUSomeTotal, pressure.CPU.Some)
		setPressureStats(&snap.PSICPUFullAvg10, &snap.PSICPUFullAvg60, &snap.PSICPUFullAvg300, &snap.PSICPUFullTotal, pressure.CPU.Full)
	}
	if pressure.Memory != nil {
		setPressureStats(&snap.PSIMemSomeAvg10, &snap.PSIMemSomeAvg60, &snap.PSIMemSomeAvg300, &snap.PSIMemSomeTotal, pressure.Memory.Some)
		setPressureStats(&snap.PSIMemFullAvg10, &snap.PSIMemFullAvg60, &snap.PSIMemFullAvg300, &snap.PSIMemFullTotal, pressure.Memory.Full)
	}
	if pressure.IO != nil {
		setPressureStats(&snap.PSIIOSomeAvg10, &snap.PSIIOSomeAvg60, &snap.PSIIOSomeAvg300, &snap.PSIIOSomeTotal, pressure.IO.Some)
		setPressureStats(&snap.PSIIOFullAvg10, &snap.PSIIOFullAvg60, &snap.PSIIOFullAvg300, &snap.PSIIOFullTotal, pressure.IO.Full)
	}
}

func setPressureStats(avg10, avg60, avg300 **float64, total **int64, stats *PressureStats) {
	if stats == nil {
		return
	}
	*avg10 = float64Ptr(stats.Avg10)
	*avg60 = float64Ptr(stats.Avg60)
	*avg300 = float64Ptr(stats.Avg300)
	*total = int64Ptr(stats.Total)
}

func pressureFromSnapshot(snap model.MetricsSnapshot) *Pressure {
	pressure := Pressure{
		CPU: &PressureResource{
			Some: pressureStatsFromSnapshot(snap.PSICPUSomeAvg10, snap.PSICPUSomeAvg60, snap.PSICPUSomeAvg300, snap.PSICPUSomeTotal),
			Full: pressureStatsFromSnapshot(snap.PSICPUFullAvg10, snap.PSICPUFullAvg60, snap.PSICPUFullAvg300, snap.PSICPUFullTotal),
		},
		Memory: &PressureResource{
			Some: pressureStatsFromSnapshot(snap.PSIMemSomeAvg10, snap.PSIMemSomeAvg60, snap.PSIMemSomeAvg300, snap.PSIMemSomeTotal),
			Full: pressureStatsFromSnapshot(snap.PSIMemFullAvg10, snap.PSIMemFullAvg60, snap.PSIMemFullAvg300, snap.PSIMemFullTotal),
		},
		IO: &PressureResource{
			Some: pressureStatsFromSnapshot(snap.PSIIOSomeAvg10, snap.PSIIOSomeAvg60, snap.PSIIOSomeAvg300, snap.PSIIOSomeTotal),
			Full: pressureStatsFromSnapshot(snap.PSIIOFullAvg10, snap.PSIIOFullAvg60, snap.PSIIOFullAvg300, snap.PSIIOFullTotal),
		},
	}
	pressure.CPU = pressureResourceFromSnapshot(pressure.CPU)
	pressure.Memory = pressureResourceFromSnapshot(pressure.Memory)
	pressure.IO = pressureResourceFromSnapshot(pressure.IO)
	return normalizePressure(&pressure)
}

func pressureResourceFromSnapshot(resource *PressureResource) *PressureResource {
	if resource == nil || (resource.Some == nil && resource.Full == nil) {
		return nil
	}
	return resource
}

func pressureStatsFromSnapshot(avg10, avg60, avg300 *float64, total *int64) *PressureStats {
	if avg10 == nil && avg60 == nil && avg300 == nil && total == nil {
		return nil
	}
	stats := PressureStats{}
	if avg10 != nil {
		stats.Avg10 = *avg10
	}
	if avg60 != nil {
		stats.Avg60 = *avg60
	}
	if avg300 != nil {
		stats.Avg300 = *avg300
	}
	if total != nil && *total > 0 {
		stats.Total = *total
	}
	return &stats
}

func validPressureStats(stats *PressureStats) bool {
	if stats == nil {
		return true
	}
	return validPressureAvg(stats.Avg10) &&
		validPressureAvg(stats.Avg60) &&
		validPressureAvg(stats.Avg300) &&
		stats.Total >= 0
}

func validPressureAvg(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 100
}

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}
