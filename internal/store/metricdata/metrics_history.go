package metricdata

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type HistoryAggregation string

const (
	HistoryAggregationAvg  HistoryAggregation = "avg"
	HistoryAggregationMax  HistoryAggregation = "max"
	HistoryAggregationMin  HistoryAggregation = "min"
	HistoryAggregationLast HistoryAggregation = "last"
)

type HistoryQuery struct {
	ServerID    int64
	Metric      string
	Aggregation HistoryAggregation
	Device      string
	Step        time.Duration
	Since       time.Time
	Until       time.Time
	UseRollup   bool
	RollupBase  time.Duration
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
	"cpu.usage_ratio": {Source: metricSourceServer, Column: "cpu_usage_ratio", RollupPrefix: "cpu_usage_ratio"},
	"cpu.load1":       {Source: metricSourceServer, Column: "load1", RollupPrefix: "load1"},
	"cpu.load5":       {Source: metricSourceServer, Column: "load5", RollupPrefix: "load5"},
	"cpu.load15":      {Source: metricSourceServer, Column: "load15", RollupPrefix: "load15"},
	"cpu.temp_c":      {Source: metricSourceServer, Column: "cpu_temp_c"},
	"mem.used":        {Source: metricSourceServer, Column: "mem_used", RollupPrefix: "mem_used"},
	"mem.used_ratio":  {Source: metricSourceServer, Column: "mem_used_ratio", RollupPrefix: "mem_used_ratio"},
	"proc.count":      {Source: metricSourceServer, Column: "process_count", RollupPrefix: "process_count"},
	"net.recv_bps":    {Source: metricSourceServer, Column: "net_in_bps", RollupPrefix: "net_in_bps"},
	"net.sent_bps":    {Source: metricSourceServer, Column: "net_out_bps", RollupPrefix: "net_out_bps"},
	"conn.tcp":        {Source: metricSourceServer, Column: "tcp_conn", RollupPrefix: "tcp_conn"},
	"conn.udp":        {Source: metricSourceServer, Column: "udp_conn", RollupPrefix: "udp_conn"},
	"disk.temp_c": {
		Source:        metricSourceDiskPhysical,
		Column:        "temp_c",
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
	if q.Since.IsZero() || q.Until.IsZero() {
		return nil, fmt.Errorf("invalid time range")
	}

	interval := formatInterval(q.Step)
	switch {
	case !q.UseRollup || def.RollupPrefix == "":
		return s.fetchRaw(ctx, def, q, interval)
	default:
		return s.fetchRollup(ctx, def, q, interval)
	}
}

func (s *Store) fetchRaw(ctx context.Context, def metricDef, q HistoryQuery, interval string) ([]HistoryPoint, error) {
	table := rawTableName(def.Source)
	expr := aggregationSelect(q.Aggregation, def.Column, "collected_at")
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
	base := q.RollupBase
	if base <= 0 {
		return nil, fmt.Errorf("invalid rollup base")
	}
	table := rollupTableName(def.Source, base)
	rollupCol := fmt.Sprintf("%s_%s", def.RollupPrefix, q.Aggregation)
	where, vals := deviceFilter(def, q.Device)
	if base == q.Step {
		query := fmt.Sprintf(
			"SELECT bucket AS ts, %s AS value FROM %s WHERE server_id = ? AND bucket >= ? AND bucket <= ?%s ORDER BY bucket",
			rollupCol,
			table,
			where,
		)
		points := make([]HistoryPoint, 0)
		args := []any{q.ServerID, q.Since, q.Until}
		args = append(args, vals...)
		if err := s.db.WithContext(ctx).Raw(query, args...).Scan(&points).Error; err != nil {
			return nil, err
		}
		return points, nil
	}

	expr := aggregationSelect(q.Aggregation, rollupCol, "bucket")
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

func rawTableName(source metricSource) string {
	switch source {
	case metricSourceServer:
		return "server_metrics"
	case metricSourceDiskIO:
		return "disk_metrics"
	case metricSourceDiskUsage:
		return "disk_usage_metrics"
	case metricSourceDiskPhysical:
		return "disk_physical_metrics"
	default:
		return "server_metrics"
	}
}

func rollupTableName(source metricSource, base time.Duration) string {
	switch source {
	case metricSourceServer:
		return rollupTableByBase("server_metrics", base)
	case metricSourceDiskIO:
		return rollupTableByBase("disk_metrics", base)
	case metricSourceDiskUsage:
		return rollupTableByBase("disk_usage_metrics", base)
	default:
		return rollupTableByBase("server_metrics", base)
	}
}

func rollupTableByBase(prefix string, base time.Duration) string {
	if base >= time.Hour {
		return prefix + "_1h"
	}
	return prefix + "_15m"
}

func formatInterval(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds <= 0 {
		seconds = 1
	}
	return fmt.Sprintf("%d seconds", seconds)
}

func aggregationSelect(aggregation HistoryAggregation, col, ts string) string {
	switch aggregation {
	case HistoryAggregationMax:
		return fmt.Sprintf("max(%s)", col)
	case HistoryAggregationMin:
		return fmt.Sprintf("min(%s)", col)
	case HistoryAggregationLast:
		return fmt.Sprintf("last(%s, %s)", col, ts)
	default:
		return fmt.Sprintf("avg(%s)", col)
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
