package metrics

import (
	"math"
	"strings"
	"testing"
)

func TestValidateReportRejectsInvalidNumericState(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*NodeReport)
	}{
		{
			name: "negative memory",
			mutate: func(report *NodeReport) {
				report.Metrics.Memory.Used = -1
			},
		},
		{
			name: "negative disk counter",
			mutate: func(report *NodeReport) {
				report.Metrics.Disk.BaseIO = []DiskBaseIOMetrics{{
					Kind:      "disk",
					Name:      "sda",
					ReadBytes: -1,
				}}
			},
		},
		{
			name: "logical disk total overflow",
			mutate: func(report *NodeReport) {
				report.Metrics.Disk.Logical = []DiskLogicalMetrics{{
					Kind: "disk",
					Name: "root",
					Used: math.MaxInt64,
					Free: 1,
				}}
			},
		},
		{
			name: "negative network byte counter",
			mutate: func(report *NodeReport) {
				report.Metrics.Network = []NetIOMetrics{{
					Name:      "eth0",
					BytesRecv: -1,
				}}
			},
		},
		{
			name: "negative process count",
			mutate: func(report *NodeReport) {
				report.Metrics.Processes.ProcessCount = -1
			},
		},
		{
			name: "negative pressure total",
			mutate: func(report *NodeReport) {
				report.Metrics.Pressure = &Pressure{
					CPU: &PressureResource{
						Some: &PressureStats{Total: -1},
					},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := validNumericReport()
			tt.mutate(&report)
			if err := ValidateReport(report); err == nil {
				t.Fatal("ValidateReport() error = nil")
			}
		})
	}
}

func TestValidateReportRejectsOversizedStoredText(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*NodeReport)
	}{
		{
			name: "disk name",
			mutate: func(report *NodeReport) {
				report.Metrics.Disk.BaseIO = []DiskBaseIOMetrics{{
					Kind: "disk",
					Name: strings.Repeat("d", maxDiskNameChars+1),
				}}
			},
		},
		{
			name: "disk ref",
			mutate: func(report *NodeReport) {
				report.Metrics.Disk.BaseIO = []DiskBaseIOMetrics{{
					Kind: "disk",
					Name: "sda",
					Ref:  strings.Repeat("r", maxDiskRefChars+1),
				}}
			},
		},
		{
			name: "interface",
			mutate: func(report *NodeReport) {
				report.Metrics.Network = []NetIOMetrics{{
					Name: strings.Repeat("e", maxInterfaceChars+1),
				}}
			},
		},
		{
			name: "raid health",
			mutate: func(report *NodeReport) {
				report.Metrics.Raid.Arrays = []RaidArray{{
					Name:    "md0",
					Status:  "active",
					Health:  strings.Repeat("h", maxRaidHealthChars+1),
					Members: []RaidMember{},
				}}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := validNumericReport()
			tt.mutate(&report)
			if err := ValidateReport(report); err == nil {
				t.Fatal("ValidateReport() error = nil")
			}
		})
	}
}

func TestToSnapshotRejectsNetworkAggregateOverflow(t *testing.T) {
	_, err := ToSnapshot(Metrics{
		Network: []NetIOMetrics{
			{Name: "eth0", BytesRecv: math.MaxInt64},
			{Name: "eth1", BytesRecv: 1},
		},
	})
	if err == nil {
		t.Fatal("ToSnapshot() error = nil")
	}
}

func validNumericReport() NodeReport {
	return NodeReport{
		Metrics: Metrics{
			Disk: DiskMetrics{
				Physical:    []DiskPhysicalMetrics{},
				Logical:     []DiskLogicalMetrics{},
				Filesystems: []DiskFilesystemMetrics{},
				BaseIO:      []DiskBaseIOMetrics{},
			},
			Network: []NetIOMetrics{},
			System:  SystemMetrics{Uptime: "0d 0h 0m"},
			Raid:    RaidMetrics{Arrays: []RaidArray{}},
		},
	}
}
