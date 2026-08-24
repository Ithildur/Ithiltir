package metricdata

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type HistoryAggregation string

type HistoryResolution uint8

const (
	HistoryAggregationAvg  HistoryAggregation = "avg"
	HistoryAggregationMax  HistoryAggregation = "max"
	HistoryAggregationMin  HistoryAggregation = "min"
	HistoryAggregationLast HistoryAggregation = "last"
)

const (
	HistoryResolutionRaw HistoryResolution = iota
	HistoryResolution15m
	HistoryResolution1h
)

type HistoryQuery struct {
	ServerID    int64
	Metric      string
	Aggregation HistoryAggregation
	Device      string
	Step        time.Duration
	Since       time.Time
	Until       time.Time
	Resolution  HistoryResolution
}

type HistoryPoint struct {
	TS    time.Time `json:"ts"`
	Value *float64  `json:"value"`
}

type metricSource int

const (
	metricSourceServer metricSource = iota
	metricSourceDiskIO
	metricSourceDiskUsage
	metricSourceDiskPhysical
)

type metricDef struct {
	Source        metricSource
	Column        string
	RollupPrefix  string
	RequireDevice bool
	DeviceColumns []string
}

var historyDefs = map[string]metricDef{
	"cpu.usage_ratio":             {Source: metricSourceServer, Column: "cpu_usage_ratio", RollupPrefix: "cpu_usage_ratio"},
	"cpu.load1":                   {Source: metricSourceServer, Column: "load1", RollupPrefix: "load1"},
	"cpu.load5":                   {Source: metricSourceServer, Column: "load5", RollupPrefix: "load5"},
	"cpu.load15":                  {Source: metricSourceServer, Column: "load15", RollupPrefix: "load15"},
	"cpu.temp_c":                  {Source: metricSourceServer, Column: "cpu_temp_c", RollupPrefix: "cpu_temp_c"},
	"mem.used":                    {Source: metricSourceServer, Column: "mem_used", RollupPrefix: "mem_used"},
	"mem.used_ratio":              {Source: metricSourceServer, Column: "mem_used_ratio", RollupPrefix: "mem_used_ratio"},
	"proc.count":                  {Source: metricSourceServer, Column: "process_count", RollupPrefix: "process_count"},
	"net.recv_bps":                {Source: metricSourceServer, Column: "net_in_bps", RollupPrefix: "net_in_bps"},
	"net.sent_bps":                {Source: metricSourceServer, Column: "net_out_bps", RollupPrefix: "net_out_bps"},
	"conn.tcp":                    {Source: metricSourceServer, Column: "tcp_conn", RollupPrefix: "tcp_conn"},
	"conn.udp":                    {Source: metricSourceServer, Column: "udp_conn", RollupPrefix: "udp_conn"},
	"pressure.cpu.some_avg10":     {Source: metricSourceServer, Column: "psi_cpu_some_avg10", RollupPrefix: "psi_cpu_some_avg10"},
	"pressure.cpu.some_avg60":     {Source: metricSourceServer, Column: "psi_cpu_some_avg60", RollupPrefix: "psi_cpu_some_avg60"},
	"pressure.cpu.some_avg300":    {Source: metricSourceServer, Column: "psi_cpu_some_avg300", RollupPrefix: "psi_cpu_some_avg300"},
	"pressure.memory.some_avg10":  {Source: metricSourceServer, Column: "psi_memory_some_avg10", RollupPrefix: "psi_memory_some_avg10"},
	"pressure.memory.some_avg60":  {Source: metricSourceServer, Column: "psi_memory_some_avg60", RollupPrefix: "psi_memory_some_avg60"},
	"pressure.memory.some_avg300": {Source: metricSourceServer, Column: "psi_memory_some_avg300", RollupPrefix: "psi_memory_some_avg300"},
	"pressure.memory.full_avg10":  {Source: metricSourceServer, Column: "psi_memory_full_avg10", RollupPrefix: "psi_memory_full_avg10"},
	"pressure.memory.full_avg60":  {Source: metricSourceServer, Column: "psi_memory_full_avg60", RollupPrefix: "psi_memory_full_avg60"},
	"pressure.memory.full_avg300": {Source: metricSourceServer, Column: "psi_memory_full_avg300", RollupPrefix: "psi_memory_full_avg300"},
	"pressure.io.some_avg10":      {Source: metricSourceServer, Column: "psi_io_some_avg10", RollupPrefix: "psi_io_some_avg10"},
	"pressure.io.some_avg60":      {Source: metricSourceServer, Column: "psi_io_some_avg60", RollupPrefix: "psi_io_some_avg60"},
	"pressure.io.some_avg300":     {Source: metricSourceServer, Column: "psi_io_some_avg300", RollupPrefix: "psi_io_some_avg300"},
	"pressure.io.full_avg10":      {Source: metricSourceServer, Column: "psi_io_full_avg10", RollupPrefix: "psi_io_full_avg10"},
	"pressure.io.full_avg60":      {Source: metricSourceServer, Column: "psi_io_full_avg60", RollupPrefix: "psi_io_full_avg60"},
	"pressure.io.full_avg300":     {Source: metricSourceServer, Column: "psi_io_full_avg300", RollupPrefix: "psi_io_full_avg300"},
	"disk.temp_c": {
		Source:        metricSourceDiskPhysical,
		Column:        "temp_c",
		RollupPrefix:  "temp_c",
		DeviceColumns: []string{"name", "ref", "path"},
	},
	"disk.read_bps": {
		Source:        metricSourceDiskIO,
		Column:        "read_rate_bytes_per_sec",
		RollupPrefix:  "read_bps",
		RequireDevice: true,
		DeviceColumns: []string{"name", "ref"},
	},
	"disk.write_bps": {
		Source:        metricSourceDiskIO,
		Column:        "write_rate_bytes_per_sec",
		RollupPrefix:  "write_bps",
		RequireDevice: true,
		DeviceColumns: []string{"name", "ref"},
	},
	"disk.read_iops": {
		Source:        metricSourceDiskIO,
		Column:        "read_iops",
		RollupPrefix:  "read_iops",
		RequireDevice: true,
		DeviceColumns: []string{"name", "ref"},
	},
	"disk.write_iops": {
		Source:        metricSourceDiskIO,
		Column:        "write_iops",
		RollupPrefix:  "write_iops",
		RequireDevice: true,
		DeviceColumns: []string{"name", "ref"},
	},
	"disk.iops": {
		Source:        metricSourceDiskIO,
		Column:        "iops",
		RollupPrefix:  "iops",
		RequireDevice: true,
		DeviceColumns: []string{"name", "ref"},
	},
	"disk.used": {
		Source:        metricSourceDiskUsage,
		Column:        "used",
		RollupPrefix:  "used_bytes",
		RequireDevice: true,
		DeviceColumns: []string{"name", "ref", "mountpoint"},
	},
	"disk.used_ratio": {
		Source:        metricSourceDiskUsage,
		Column:        "used_ratio",
		RollupPrefix:  "used_ratio",
		RequireDevice: true,
		DeviceColumns: []string{"name", "ref", "mountpoint"},
	},
}

func HasHistory(metric string) bool {
	_, ok := historyDefs[metric]
	return ok
}

func HistoryNeedsDevice(metric string) bool {
	def, ok := historyDefs[metric]
	if !ok {
		return false
	}
	return def.RequireDevice
}

func (s *Store) FetchHistory(ctx context.Context, q HistoryQuery) ([]HistoryPoint, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store: db is nil")
	}
	def, ok := historyDefs[q.Metric]
	if !ok {
		return nil, fmt.Errorf("invalid metric")
	}
	if def.RequireDevice && strings.TrimSpace(q.Device) == "" {
		return nil, fmt.Errorf("device is required")
	}
	if q.ServerID <= 0 {
		return nil, fmt.Errorf("invalid server id")
	}
	if q.Step <= 0 {
		return nil, fmt.Errorf("invalid step")
	}
	switch q.Aggregation {
	case HistoryAggregationAvg, HistoryAggregationMax, HistoryAggregationMin, HistoryAggregationLast:
	default:
		return nil, fmt.Errorf("invalid history aggregation %q", q.Aggregation)
	}
	if q.Since.IsZero() || q.Until.IsZero() {
		return nil, fmt.Errorf("invalid time range")
	}

	interval := formatInterval(q.Step)
	switch q.Resolution {
	case HistoryResolutionRaw:
		return s.fetchRaw(ctx, def, q, interval)
	case HistoryResolution15m, HistoryResolution1h:
		if def.RollupPrefix == "" {
			return nil, fmt.Errorf("metric %q has no rollup source", q.Metric)
		}
		return s.fetchRollup(ctx, def, q, interval)
	default:
		return nil, fmt.Errorf("invalid history resolution %d", q.Resolution)
	}
}

func (s *Store) fetchRaw(ctx context.Context, def metricDef, q HistoryQuery, interval string) ([]HistoryPoint, error) {
	table, err := rawTableName(def.Source)
	if err != nil {
		return nil, err
	}
	expr, err := aggregationSelect(q.Aggregation, def.Column, "collected_at")
	if err != nil {
		return nil, err
	}
	where, vals := deviceFilter(def, q.Device)
	query := fmt.Sprintf(
		"SELECT time_bucket(?, collected_at) AS ts, %s AS value FROM %s WHERE server_id = ? AND collected_at >= ? AND collected_at <= ?%s GROUP BY ts ORDER BY ts",
		expr,
		table,
		where,
	)

	args := []any{interval, q.ServerID, q.Since, q.Until}
	args = append(args, vals...)
	points := make([]HistoryPoint, 0)
	if err := s.db.WithContext(ctx).Raw(query, args...).Scan(&points).Error; err != nil {
		return nil, err
	}
	return points, nil
}

func (s *Store) fetchRollup(ctx context.Context, def metricDef, q HistoryQuery, interval string) ([]HistoryPoint, error) {
	base, err := rollupBase(q.Resolution)
	if err != nil {
		return nil, err
	}
	if q.Step < base || q.Step%base != 0 {
		return nil, fmt.Errorf("history step %s is not aligned with %s rollup", q.Step, base)
	}
	table, err := rollupTableName(def.Source, base)
	if err != nil {
		return nil, err
	}
	expr, err := rollupAggregationSelect(q.Aggregation, def.RollupPrefix, q.Resolution)
	if err != nil {
		return nil, err
	}
	where, vals := deviceFilter(def, q.Device)
	query := fmt.Sprintf(
		"SELECT time_bucket(?, bucket) AS ts, %s AS value FROM %s WHERE server_id = ? AND bucket >= ? AND bucket <= ?%s GROUP BY ts ORDER BY ts",
		expr,
		table,
		where,
	)
	points := make([]HistoryPoint, 0)
	args := []any{interval, q.ServerID, q.Since, q.Until}
	args = append(args, vals...)
	if err := s.db.WithContext(ctx).Raw(query, args...).Scan(&points).Error; err != nil {
		return nil, err
	}
	return points, nil
}

func rawTableName(source metricSource) (string, error) {
	switch source {
	case metricSourceServer:
		return "server_metrics", nil
	case metricSourceDiskIO:
		return "disk_metrics", nil
	case metricSourceDiskUsage:
		return "disk_usage_metrics", nil
	case metricSourceDiskPhysical:
		return "disk_physical_metrics", nil
	default:
		return "", fmt.Errorf("invalid metric source %d", source)
	}
}

func rollupTableName(source metricSource, base time.Duration) (string, error) {
	var prefix string
	switch source {
	case metricSourceServer:
		prefix = "server_metrics"
	case metricSourceDiskIO:
		prefix = "disk_metrics"
	case metricSourceDiskUsage:
		prefix = "disk_usage_metrics"
	case metricSourceDiskPhysical:
		prefix = "disk_physical_metrics"
	default:
		return "", fmt.Errorf("invalid rollup metric source %d", source)
	}
	return rollupTableByBase(prefix, base)
}

func rollupBase(resolution HistoryResolution) (time.Duration, error) {
	switch resolution {
	case HistoryResolution15m:
		return 15 * time.Minute, nil
	case HistoryResolution1h:
		return time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid rollup resolution %d", resolution)
	}
}

func rollupTableByBase(prefix string, base time.Duration) (string, error) {
	switch base {
	case 15 * time.Minute:
		return prefix + "_15m", nil
	case time.Hour:
		return prefix + "_1h", nil
	default:
		return "", fmt.Errorf("invalid rollup base %s", base)
	}
}

func formatInterval(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds <= 0 {
		seconds = 1
	}
	return fmt.Sprintf("%d seconds", seconds)
}

func aggregationSelect(aggregation HistoryAggregation, col, ts string) (string, error) {
	switch aggregation {
	case HistoryAggregationAvg:
		return fmt.Sprintf("avg(%s)", col), nil
	case HistoryAggregationMax:
		return fmt.Sprintf("max(%s)", col), nil
	case HistoryAggregationMin:
		return fmt.Sprintf("min(%s)", col), nil
	case HistoryAggregationLast:
		return fmt.Sprintf("last(%s, %s)", col, ts), nil
	default:
		return "", fmt.Errorf("invalid history aggregation %q", aggregation)
	}
}

func rollupAggregationSelect(aggregation HistoryAggregation, prefix string, resolution HistoryResolution) (string, error) {
	col := fmt.Sprintf("%s_%s", prefix, aggregation)
	switch aggregation {
	case HistoryAggregationAvg:
		if resolution == HistoryResolution15m {
			return fmt.Sprintf(
				"sum(%s_avg * %s_count) / nullif(sum(%s_count), 0)",
				prefix,
				prefix,
				prefix,
			), nil
		}
		return fmt.Sprintf("avg(%s)", col), nil
	case HistoryAggregationMax:
		return fmt.Sprintf("max(%s)", col), nil
	case HistoryAggregationMin:
		return fmt.Sprintf("min(%s)", col), nil
	case HistoryAggregationLast:
		return fmt.Sprintf("last(%s, bucket)", col), nil
	default:
		return "", fmt.Errorf("invalid history aggregation %q", aggregation)
	}
}

func deviceFilter(def metricDef, device string) (string, []any) {
	device = strings.TrimSpace(device)
	if device == "" || len(def.DeviceColumns) == 0 {
		return "", nil
	}
	conds := make([]string, 0, len(def.DeviceColumns))
	args := make([]any, 0, len(def.DeviceColumns))
	for _, col := range def.DeviceColumns {
		conds = append(conds, fmt.Sprintf("%s = ?", col))
		args = append(args, device)
	}
	return " AND (" + strings.Join(conds, " OR ") + ")", args
}
