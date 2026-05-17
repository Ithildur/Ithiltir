package metrics

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

	netInBytes, netOutBytes, netInBps, netOutBps := aggregateNetwork(m.Network)
	raidHealth := raidOverallHealth(m.Raid)

	mem := m.Memory
	conn := m.Connections
	processes := m.Processes
	return model.MetricsSnapshot{
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
		MemTotal:          int64(mem.Total),
		MemUsed:           int64(mem.Used),
		MemAvailable:      int64(mem.Available),
		MemBuffers:        int64(mem.Buffers),
		MemCached:         int64(mem.Cached),
		MemUsedRatio:      mem.UsedRatio,
		SwapTotal:         int64(mem.SwapTotal),
		SwapUsed:          int64(mem.SwapUsed),
		SwapFree:          int64(mem.SwapFree),
		SwapUsedRatio:     mem.SwapUsedRatio,
		NetInBytes:        int64(netInBytes),
		NetOutBytes:       int64(netOutBytes),
		NetInBps:          netInBps,
		NetOutBps:         netOutBps,
		ProcessCount:      int32(processes.ProcessCount),
		TCPConn:           int32(conn.TCPCount),
		UDPConn:           int32(conn.UDPCount),
		UptimeSeconds:     int64(m.System.UptimeSeconds),
		RaidSupported:     m.Raid.Supported,
		RaidAvailable:     m.Raid.Available,
		RaidOverallHealth: raidHealth,
		Raid:              raidJSON,
		Thermal:           thermalJSON,
	}, nil
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
			Total:         uint64(snap.MemTotal),
			Used:          uint64(snap.MemUsed),
			Available:     uint64(snap.MemAvailable),
			Buffers:       uint64(snap.MemBuffers),
			Cached:        uint64(snap.MemCached),
			UsedRatio:     snap.MemUsedRatio,
			SwapTotal:     uint64(snap.SwapTotal),
			SwapUsed:      uint64(snap.SwapUsed),
			SwapFree:      uint64(snap.SwapFree),
			SwapUsedRatio: snap.SwapUsedRatio,
		},
		Disk: DiskMetrics{
			Logical: nil,
			BaseIO:  nil,
		},
		Network: nil,
		System: SystemMetrics{
			Alive:         true,
			UptimeSeconds: uint64(snap.UptimeSeconds),
			Uptime:        formatUptime(uint64(snap.UptimeSeconds)),
		},
		Processes:   ProcessMetrics{ProcessCount: int(snap.ProcessCount)},
		Connections: ConnectionMetrics{TCPCount: int(snap.TCPConn), UDPCount: int(snap.UDPConn)},
		Raid:        raid,
		Thermal:     thermal,
	}

	return m, nil
}

func BuildNodeReport(server model.Server, metric model.ServerMetric) (NodeReport, error) {
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

func aggregateNetwork(interfaces []NetIOMetrics) (inBytes uint64, outBytes uint64, inBps float64, outBps float64) {
	for _, iface := range interfaces {
		inBytes += iface.BytesRecv
		outBytes += iface.BytesSent
		inBps += iface.RecvRateBytesPerSec
		outBps += iface.SentRateBytesPerSec
	}
	return
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
		if array.Health == "" {
			continue
		}
		if array.Health == "degraded" || array.Health == "syncing" {
			return array.Health
		}
		status = array.Health
	}
	return status
}

func formatUptime(seconds uint64) string {
	if seconds == 0 {
		return ""
	}
	dur := time.Duration(seconds) * time.Second
	days := dur / (24 * time.Hour)
	dur -= days * 24 * time.Hour
	hours := dur / time.Hour
	dur -= hours * time.Hour
	minutes := dur / time.Minute
	return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
}
