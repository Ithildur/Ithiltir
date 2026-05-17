package traffic

import trafficstore "dash/internal/store/traffic"

type cycleView struct {
	Mode              trafficstore.BillingCycleMode `json:"mode"`
	BillingStartDay   int                           `json:"billing_start_day"`
	BillingAnchorDate string                        `json:"billing_anchor_date,omitempty"`
	Timezone          string                        `json:"timezone"`
	Start             string                        `json:"start"`
	End               string                        `json:"end"`
}

type statView struct {
	InBytes                 int64    `json:"in_bytes"`
	OutBytes                int64    `json:"out_bytes"`
	P95Enabled              bool     `json:"p95_enabled"`
	P95Status               string   `json:"p95_status"`
	P95UnavailableReason    string   `json:"p95_unavailable_reason,omitempty"`
	InP95BytesPerSec        *float64 `json:"in_p95_bytes_per_sec"`
	OutP95BytesPerSec       *float64 `json:"out_p95_bytes_per_sec"`
	InPeakBytesPerSec       float64  `json:"in_peak_bytes_per_sec"`
	OutPeakBytesPerSec      float64  `json:"out_peak_bytes_per_sec"`
	SelectedBytes           int64    `json:"selected_bytes"`
	SelectedP95BytesPerSec  *float64 `json:"selected_p95_bytes_per_sec"`
	SelectedPeakBytesPerSec float64  `json:"selected_peak_bytes_per_sec"`
	SelectedBytesDirection  string   `json:"selected_bytes_direction"`
	SelectedP95Direction    string   `json:"selected_p95_direction,omitempty"`
	SelectedPeakDirection   string   `json:"selected_peak_direction"`
	SampleCount             int      `json:"sample_count"`
	ExpectedSampleCount     int      `json:"expected_sample_count"`
	EffectiveStart          string   `json:"effective_start"`
	EffectiveEnd            string   `json:"effective_end"`
	CoverageRatio           float64  `json:"coverage_ratio"`
	CoveredUntil            string   `json:"covered_until"`
	GapCount                int      `json:"gap_count"`
	ResetCount              int      `json:"reset_count"`
	CycleComplete           bool     `json:"cycle_complete"`
	DataComplete            bool     `json:"data_complete"`
	Status                  string   `json:"status"`
	Partial                 bool     `json:"partial"`
}

type summaryView struct {
	ServerID      int64                      `json:"server_id"`
	ServerName    string                     `json:"server_name,omitempty"`
	Iface         string                     `json:"iface"`
	UsageMode     trafficstore.UsageMode     `json:"usage_mode"`
	DirectionMode trafficstore.DirectionMode `json:"direction_mode"`
	Cycle         cycleView                  `json:"cycle"`
	Stats         statView                   `json:"stats"`
}

func summaryViewFrom(summary trafficstore.TrafficSummary, direction trafficstore.DirectionMode) summaryView {
	return summaryView{
		ServerID:      summary.ServerID,
		Iface:         summary.Iface,
		UsageMode:     summary.UsageMode,
		DirectionMode: direction,
		Cycle: cycleView{
			Mode:              summary.Cycle.Mode,
			BillingStartDay:   summary.Cycle.BillingStartDay,
			BillingAnchorDate: summary.Cycle.BillingAnchorDate,
			Timezone:          summary.Cycle.Timezone,
			Start:             formatTime(summary.Cycle.Start),
			End:               formatTime(summary.Cycle.End),
		},
		Stats: statViewFrom(summary.Stat),
	}
}

func summaryViewWithServerName(summary trafficstore.TrafficSummary, direction trafficstore.DirectionMode, serverName string) summaryView {
	out := summaryViewFrom(summary, direction)
	out.ServerName = serverName
	return out
}

func statViewFrom(stat trafficstore.TrafficStat) statView {
	return statView{
		InBytes:                 stat.InBytes,
		OutBytes:                stat.OutBytes,
		P95Enabled:              stat.P95Enabled,
		P95Status:               string(stat.P95Status),
		P95UnavailableReason:    stat.P95UnavailableReason,
		InP95BytesPerSec:        p95Value(stat, stat.InP95BytesPerSec),
		OutP95BytesPerSec:       p95Value(stat, stat.OutP95BytesPerSec),
		InPeakBytesPerSec:       stat.InPeakBytesPerSec,
		OutPeakBytesPerSec:      stat.OutPeakBytesPerSec,
		SelectedBytes:           stat.SelectedBytes,
		SelectedP95BytesPerSec:  p95Value(stat, stat.SelectedP95BytesPerSec),
		SelectedPeakBytesPerSec: stat.SelectedPeakBytesPerSec,
		SelectedBytesDirection:  string(stat.SelectedBytesDirection),
		SelectedP95Direction:    string(stat.SelectedP95Direction),
		SelectedPeakDirection:   string(stat.SelectedPeakDirection),
		SampleCount:             stat.SampleCount,
		ExpectedSampleCount:     stat.ExpectedSampleCount,
		EffectiveStart:          formatTime(stat.EffectiveStart),
		EffectiveEnd:            formatTime(stat.EffectiveEnd),
		CoverageRatio:           stat.CoverageRatio,
		CoveredUntil:            formatTime(stat.CoveredUntil),
		GapCount:                stat.GapCount,
		ResetCount:              stat.ResetCount,
		CycleComplete:           stat.CycleComplete,
		DataComplete:            stat.DataComplete,
		Status:                  string(stat.Status),
		Partial:                 stat.Partial,
	}
}

func p95Value(stat trafficstore.TrafficStat, value float64) *float64 {
	if stat.P95Status != trafficstore.TrafficP95Available {
		return nil
	}
	return &value
}
