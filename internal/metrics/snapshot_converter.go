package metrics

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"dash/internal/model"
)

// ToSnapshot converts Metrics to model.MetricsSnapshot for database storage.
func ToSnapshot(m Metrics) (model.MetricsSnapshot, error) {
	raidJSON, err := json.Marshal(m.Raid)
	if err != nil {
		return model.MetricsSnapshot{}, fmt.Errorf("marshal raid: %w", err)
	}
	var thermalJSON []byte
	if thermal := normalizeThermal(m.Thermal); thermal != nil {
		thermalJSON, err = json.Marshal(thermal)
		if err != nil {
			return model.MetricsSnapshot{}, fmt.Errorf("marshal thermal: %w", err)
		}
	}

	netInBytes, netOutBytes, netInBps, netOutBps, err := aggregateNetwork(m.Network)
	if err != nil {
		return model.MetricsSnapshot{}, err
	}
	raidHealth := raidOverallHealth(m.Raid)

	mem := m.Memory
	conn := m.Connections
	processes := m.Processes
	snap := model.MetricsSnapshot{
		MetricValues: model.MetricValues{
			CPUUsageRatio:     m.CPU.UsageRatio,
			Load1:             m.CPU.Load1,
			Load5:             m.CPU.Load5,
			Load15:            m.CPU.Load15,
			CPUUser:           m.CPU.Times.User,
			CPUSystem:         m.CPU.Times.System,
			CPUIdle:           m.CPU.Times.Idle,
			CPUIowait:         m.CPU.Times.Iowait,
			CPUSteal:          m.CPU.Times.Steal,
			CPUTempC:          maxThermalTempC(m.Thermal, isCPUSensor),
			MemTotal:          mem.Total,
			MemUsed:           mem.Used,
			MemAvailable:      mem.Available,
			MemBuffers:        mem.Buffers,
			MemCached:         mem.Cached,
			MemUsedRatio:      mem.UsedRatio,
			SwapTotal:         mem.SwapTotal,
			SwapUsed:          mem.SwapUsed,
			SwapFree:          mem.SwapFree,
			SwapUsedRatio:     mem.SwapUsedRatio,
			NetInBytes:        netInBytes,
			NetOutBytes:       netOutBytes,
			NetInBps:          netInBps,
			NetOutBps:         netOutBps,
			ProcessCount:      processes.ProcessCount,
			TCPConn:           conn.TCPCount,
			UDPConn:           conn.UDPCount,
			UptimeSeconds:     m.System.UptimeSeconds,
			RaidSupported:     m.Raid.Supported,
			RaidAvailable:     m.Raid.Available,
			RaidOverallHealth: raidHealth,
		},
		MetricRuntime: model.MetricRuntime{
			Raid:    raidJSON,
			Thermal: thermalJSON,
		},
	}
	applyPressureSnapshot(&snap, m.Pressure)
	return snap, nil
}

func metricsFromSnapshot(snap model.MetricsSnapshot) (Metrics, error) {
	var raid RaidMetrics
	if len(snap.Raid) > 0 {
		if err := json.Unmarshal(snap.Raid, &raid); err != nil {
			return Metrics{}, fmt.Errorf("unmarshal raid: %w", err)
		}
	}
	var thermal *Thermal
	if len(snap.Thermal) > 0 {
		var value Thermal
		if err := json.Unmarshal(snap.Thermal, &value); err != nil {
			return Metrics{}, fmt.Errorf("unmarshal thermal: %w", err)
		}
		thermal = normalizeThermal(&value)
	}

	m := Metrics{
		CPU: CPUMetrics{
			UsageRatio: snap.CPUUsageRatio,
			Load1:      snap.Load1,
			Load5:      snap.Load5,
			Load15:     snap.Load15,
			Times: CPUTimes{
				User:   snap.CPUUser,
				System: snap.CPUSystem,
				Idle:   snap.CPUIdle,
				Iowait: snap.CPUIowait,
				Steal:  snap.CPUSteal,
			},
		},
		Memory: MemoryMetrics{
			Total:         snap.MemTotal,
			Used:          snap.MemUsed,
			Available:     snap.MemAvailable,
			Buffers:       snap.MemBuffers,
			Cached:        snap.MemCached,
			UsedRatio:     snap.MemUsedRatio,
			SwapTotal:     snap.SwapTotal,
			SwapUsed:      snap.SwapUsed,
			SwapFree:      snap.SwapFree,
			SwapUsedRatio: snap.SwapUsedRatio,
		},
		Disk: DiskMetrics{
			Logical: nil,
			BaseIO:  nil,
		},
		Network: nil,
		System: SystemMetrics{
			Alive:         true,
			UptimeSeconds: snap.UptimeSeconds,
			Uptime:        formatUptime(snap.UptimeSeconds),
		},
		Processes:   ProcessMetrics{ProcessCount: snap.ProcessCount},
		Connections: ConnectionMetrics{TCPCount: snap.TCPConn, UDPCount: snap.UDPConn},
		Raid:        raid,
		Thermal:     thermal,
		Pressure:    pressureFromSnapshot(snap),
	}

	return m, nil
}

func BuildNodeReport(server model.Server, metric model.ServerCurrentMetric) (NodeReport, error) {
	m, err := metricsFromSnapshot(metric.MetricsSnapshot)
	if err != nil {
		return NodeReport{}, err
	}

	displayHost := server.Hostname
	if name := strings.TrimSpace(server.Name); name != "" && name != "Untitled" {
		displayHost = name
	}

	sentAt := ""
	if metric.ReportedAt != nil {
		sentAt = FormatTimestamp(*metric.ReportedAt)
	}

	return NodeReport{
		Version:      stringOrEmpty(server.AgentVersion),
		Hostname:     displayHost,
		Timestamp:    metric.CollectedAt,
		Metrics:      m,
		SentAt:       sentAt,
		ServerID:     server.ID,
		DisplayOrder: server.DisplayOrder,
	}, nil
}

func aggregateNetwork(interfaces []NetIOMetrics) (inBytes int64, outBytes int64, inBps float64, outBps float64, err error) {
	for _, iface := range interfaces {
		if iface.BytesRecv < 0 || iface.BytesSent < 0 {
			return 0, 0, 0, 0, fmt.Errorf("invalid network counters")
		}
		if inBytes > math.MaxInt64-iface.BytesRecv || outBytes > math.MaxInt64-iface.BytesSent {
			return 0, 0, 0, 0, fmt.Errorf("network counters overflow")
		}
		inBytes += iface.BytesRecv
		outBytes += iface.BytesSent
		inBps += iface.RecvRateBytesPerSec
		outBps += iface.SentRateBytesPerSec
		if math.IsInf(inBps, 0) || math.IsInf(outBps, 0) {
			return 0, 0, 0, 0, fmt.Errorf("network rates overflow")
		}
	}
	return inBytes, outBytes, inBps, outBps, nil
}

func maxThermalTempC(thermal *Thermal, match func(ThermalSensor) bool) *float64 {
	if thermal == nil {
		return nil
	}
	var max float64
	ok := false
	for _, sensor := range thermal.Sensors {
		if !validTempC(sensor.TempC) || !match(sensor) {
			continue
		}
		if !ok || *sensor.TempC > max {
			max = *sensor.TempC
			ok = true
		}
	}
	if !ok {
		return nil
	}
	return &max
}

func isCPUSensor(sensor ThermalSensor) bool {
	return strings.EqualFold(strings.TrimSpace(sensor.Kind), "cpu")
}

func raidOverallHealth(raid RaidMetrics) string {
	if len(raid.Arrays) == 0 {
		return ""
	}
	status := "healthy"
	for _, array := range raid.Arrays {
		health := strings.TrimSpace(array.Health)
		if health == "" {
			continue
		}
		if health == "degraded" || health == "syncing" {
			return health
		}
		status = health
	}
	return status
}

func formatUptime(seconds int64) string {
	if seconds == 0 {
		return ""
	}
	days := seconds / (24 * 60 * 60)
	seconds %= 24 * 60 * 60
	hours := seconds / (60 * 60)
	seconds %= 60 * 60
	minutes := seconds / 60
	return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
}
