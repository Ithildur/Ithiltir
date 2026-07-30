package node

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dash/internal/metrics"
)

func TestNormalizeStaticRejectsNegativeCapacity(t *testing.T) {
	snapshot := metrics.StaticMetrics{
		Version:               "1.0.0",
		Timestamp:             time.Now(),
		ReportIntervalSeconds: 10,
		Memory:                metrics.StaticMemory{Total: -1},
		System: metrics.StaticSystem{
			Hostname:        "node",
			OS:              "linux",
			Platform:        "linux",
			PlatformVersion: "1",
			KernelVersion:   "1",
			Arch:            "amd64",
		},
	}

	if err := normalizeStatic(&snapshot); err == nil {
		t.Fatal("normalizeStatic() error = nil")
	}
}

func TestNormalizeStaticRejectsOversizedStoredText(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*metrics.StaticMetrics)
	}{
		{
			name: "kernel version",
			mutate: func(snapshot *metrics.StaticMetrics) {
				snapshot.System.KernelVersion = strings.Repeat("k", 256)
			},
		},
		{
			name: "root filesystem type",
			mutate: func(snapshot *metrics.StaticMetrics) {
				snapshot.Disk.Logical = []metrics.StaticDiskLogical{{
					Mountpoint: "/",
					Mountpoints: map[string]metrics.StaticDiskMountpoint{
						"/": {FSType: strings.Repeat("f", 33)},
					},
				}}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := metrics.StaticMetrics{
				Version:               "1.0.0",
				Timestamp:             time.Now(),
				ReportIntervalSeconds: 10,
				System: metrics.StaticSystem{
					Hostname:        "node",
					OS:              "linux",
					Platform:        "linux",
					PlatformVersion: "1",
					KernelVersion:   "1",
					Arch:            "amd64",
				},
			}
			tt.mutate(&snapshot)
			if err := normalizeStatic(&snapshot); err == nil {
				t.Fatal("normalizeStatic() error = nil")
			}
		})
	}
}

func TestStaticPatchPreservesUnavailableHardware(t *testing.T) {
	snapshot := metrics.StaticMetrics{
		Version:               "1.0.0",
		ReportIntervalSeconds: 10,
		System: metrics.StaticSystem{
			Hostname:        "node-1",
			OS:              "linux",
			Platform:        "ubuntu",
			PlatformVersion: "24.04",
			KernelVersion:   "6.8.0",
			Arch:            "amd64",
		},
	}
	req := httptest.NewRequest("POST", "/api/node/static", nil)

	patch := staticPatch(snapshot, metrics.StaticDiskLogical{}, false, req)
	if patch.CPUModel != nil || patch.CPUCoresLog != nil || patch.MemTotal != nil ||
		patch.SwapTotal != nil || patch.Disk != nil {
		t.Fatalf("staticPatch() marked unavailable hardware as observed: %+v", patch)
	}
	if patch.Hostname == nil || *patch.Hostname != snapshot.System.Hostname {
		t.Fatalf("staticPatch() hostname = %v, want %q", patch.Hostname, snapshot.System.Hostname)
	}
}

func TestStaticPatchKeepsDiskFieldsInOneObservation(t *testing.T) {
	snapshot := metrics.StaticMetrics{Version: "1.0.0", ReportIntervalSeconds: 10}
	req := httptest.NewRequest("POST", "/api/node/static", nil)

	missingIdentity := staticPatch(snapshot, metrics.StaticDiskLogical{Total: 1024}, true, req)
	if missingIdentity.Disk != nil {
		t.Fatalf("staticPatch() disk without identity = %+v, want no observation", missingIdentity.Disk)
	}

	observed := staticPatch(snapshot, metrics.StaticDiskLogical{
		Mountpoint: "/data",
		Total:      2048,
		Mountpoints: map[string]metrics.StaticDiskMountpoint{
			"/data": {FSType: "xfs"},
		},
	}, true, req)
	if observed.Disk == nil || observed.Disk.Path != "/data" ||
		observed.Disk.FSType != "xfs" || observed.Disk.Total != 2048 {
		t.Fatalf("staticPatch() disk = %+v, want one complete observation", observed.Disk)
	}
}

func TestStaticPatchKeepsExplicitZeroSwap(t *testing.T) {
	zero := int64(0)
	snapshot := metrics.StaticMetrics{
		Version:               "1.0.0",
		ReportIntervalSeconds: 10,
		Memory: metrics.StaticMemory{
			SwapTotal: &zero,
		},
	}
	req := httptest.NewRequest("POST", "/api/node/static", nil)

	patch := staticPatch(snapshot, metrics.StaticDiskLogical{}, false, req)
	if patch.SwapTotal == nil || *patch.SwapTotal != 0 {
		t.Fatalf("staticPatch() swap total = %v, want explicit zero", patch.SwapTotal)
	}
}
