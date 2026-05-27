package metricdata

import (
	"context"
	"testing"
	"time"

	"dash/internal/metrics"
	"dash/internal/model"
	pgtest "dash/internal/testutil/postgres"
)

func TestIntegrationSaveMetricsCurrentProjection(t *testing.T) {
	ctx := context.Background()
	st := New(pgtest.NewDB(t))
	srv := createMetricTestServer(t, st, "metric-node")

	newerAt := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	olderAt := newerAt.Add(-time.Minute)
	diskTemp := 41.0
	if err := st.SaveMetrics(ctx, MetricsSample{
		ServerID: srv.ID,
		Metric:   testServerMetric(srv.ID, newerAt, 0.8),
		DiskIO:   []metrics.DiskBaseIOMetrics{{Name: "sda", Role: "primary", ReadBytes: 100}},
		DiskSmart: &metrics.DiskSmart{Devices: []metrics.DiskSmartDevice{
			{Name: "sda", DeviceType: "sat", Protocol: "ATA", TempC: &diskTemp},
			{Name: "vda", DevicePath: "/dev/vda", DeviceType: "scsi", Protocol: "SCSI", Serial: "virt", TempC: &diskTemp},
			{Name: "md1", DevicePath: "/dev/md1", DeviceType: "sat", Protocol: "ATA", TempC: &diskTemp},
		}},
		DiskUsage: []metrics.DiskLogicalMetrics{{
			Name:  "root",
			Total: 1000,
			Used:  400,
			Free:  600,
		}},
		Network: []metrics.NetIOMetrics{{Name: "eth0", BytesRecv: 100, RecvRateBytesPerSec: 10}},
	}); err != nil {
		t.Fatalf("SaveMetrics(newer) error = %v", err)
	}
	if err := st.SaveMetrics(ctx, MetricsSample{
		ServerID:  srv.ID,
		Metric:    testServerMetric(srv.ID, olderAt, 0.2),
		DiskIO:    []metrics.DiskBaseIOMetrics{{Name: "sdb", Role: "primary", ReadBytes: 20}},
		DiskUsage: []metrics.DiskLogicalMetrics{{Name: "old-root", Total: 1000, Used: 200, Free: 800}},
		Network:   []metrics.NetIOMetrics{{Name: "eth1", BytesRecv: 20, RecvRateBytesPerSec: 2}},
	}); err != nil {
		t.Fatalf("SaveMetrics(older) error = %v", err)
	}

	var current model.ServerCurrentMetric
	if err := st.db.WithContext(ctx).First(&current, "server_id = ?", srv.ID).Error; err != nil {
		t.Fatalf("First(ServerCurrentMetric) error = %v", err)
	}
	if !current.CollectedAt.Equal(newerAt) || current.CPUUsageRatio != 0.8 {
		t.Fatalf("current metric = collected_at %v cpu %v, want newer sample", current.CollectedAt, current.CPUUsageRatio)
	}

	var nic model.ServerCurrentNICMetric
	if err := st.db.WithContext(ctx).First(&nic, "server_id = ?", srv.ID).Error; err != nil {
		t.Fatalf("First(ServerCurrentNICMetric) error = %v", err)
	}
	if nic.Iface != "eth0" || nic.BytesRecv != 100 {
		t.Fatalf("current nic = %+v, want newer nic", nic)
	}

	var physical model.DiskPhysicalMetric
	if err := st.db.WithContext(ctx).First(&physical, "server_id = ? AND name = ?", srv.ID, "sda").Error; err != nil {
		t.Fatalf("First(DiskPhysicalMetric) error = %v", err)
	}
	if physical.TempC != diskTemp {
		t.Fatalf("disk temp = %.1f, want %.1f", physical.TempC, diskTemp)
	}
	var physicalCount int64
	if err := st.db.WithContext(ctx).Model(&model.DiskPhysicalMetric{}).Where("server_id = ?", srv.ID).Count(&physicalCount).Error; err != nil {
		t.Fatalf("Count(DiskPhysicalMetric) error = %v", err)
	}
	if physicalCount != 1 {
		t.Fatalf("disk physical rows = %d, want 1", physicalCount)
	}
}

func createMetricTestServer(t *testing.T, st *Store, name string) model.Server {
	t.Helper()

	srv := model.Server{
		Name:     name,
		Hostname: name,
		Secret:   name + "-secret",
	}
	if err := st.db.Create(&srv).Error; err != nil {
		t.Fatalf("Create(server) error = %v", err)
	}
	return srv
}

func testServerMetric(serverID int64, collectedAt time.Time, cpuUsageRatio float64) model.ServerMetric {
	return model.ServerMetric{
		ServerID:    serverID,
		CollectedAt: collectedAt,
		MetricsSnapshot: model.MetricsSnapshot{
			CPUUsageRatio: cpuUsageRatio,
			MemTotal:      1000,
			MemUsed:       int64(cpuUsageRatio * 1000),
		},
	}
}
