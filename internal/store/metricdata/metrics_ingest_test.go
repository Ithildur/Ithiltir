package metricdata

import (
	"context"
	"net/url"
	"testing"
	"time"

	"dash/internal/metrics"
	"dash/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSaveMetricsKeepsNewestCurrentProjection(t *testing.T) {
	ctx := context.Background()
	st := newMetricIngestStore(t)

	newerAt := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	olderAt := newerAt.Add(-time.Minute)
	if err := st.SaveMetrics(ctx, MetricsSample{
		ServerID: 1,
		Metric:   testServerMetric(newerAt, 0.8),
		DiskIO:   []metrics.DiskBaseIOMetrics{{Name: "sda", Role: "primary", ReadBytes: 100}},
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
		ServerID: 1,
		Metric:   testServerMetric(olderAt, 0.2),
		DiskIO:   []metrics.DiskBaseIOMetrics{{Name: "sdb", Role: "primary", ReadBytes: 20}},
		DiskUsage: []metrics.DiskLogicalMetrics{{
			Name:  "old-root",
			Total: 1000,
			Used:  200,
			Free:  800,
		}},
		Network: []metrics.NetIOMetrics{{Name: "eth1", BytesRecv: 20, RecvRateBytesPerSec: 2}},
	}); err != nil {
		t.Fatalf("SaveMetrics(older) error = %v", err)
	}

	var current model.ServerCurrentMetric
	if err := st.db.WithContext(ctx).First(&current, "server_id = ?", 1).Error; err != nil {
		t.Fatalf("First(ServerCurrentMetric) error = %v", err)
	}
	if !current.CollectedAt.Equal(newerAt) || current.CPUUsageRatio != 0.8 {
		t.Fatalf("current metric = collected_at %v cpu %v, want newer sample", current.CollectedAt, current.CPUUsageRatio)
	}

	var nic model.ServerCurrentNICMetric
	if err := st.db.WithContext(ctx).First(&nic, "server_id = ?", 1).Error; err != nil {
		t.Fatalf("First(ServerCurrentNICMetric) error = %v", err)
	}
	if nic.Iface != "eth0" || nic.BytesRecv != 100 {
		t.Fatalf("current nic = %+v, want newer nic", nic)
	}
}

func TestSaveMetricsPersistsPhysicalDiskTemperatures(t *testing.T) {
	ctx := context.Background()
	st := newMetricIngestStore(t)

	collectedAt := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	diskTemp := 41.0
	if err := st.SaveMetrics(ctx, MetricsSample{
		ServerID: 1,
		Metric:   testServerMetric(collectedAt, 0.8),
		DiskSmart: &metrics.DiskSmart{Devices: []metrics.DiskSmartDevice{
			{Name: "sda", DeviceType: "sat", Protocol: "ATA", TempC: &diskTemp},
			{Name: "vda", DevicePath: "/dev/vda", DeviceType: "scsi", Protocol: "SCSI", Serial: "virt", TempC: &diskTemp},
			{Name: "md1", DevicePath: "/dev/md1", DeviceType: "sat", Protocol: "ATA", TempC: &diskTemp},
		}},
	}); err != nil {
		t.Fatalf("SaveMetrics() error = %v", err)
	}

	var physical model.DiskPhysicalMetric
	if err := st.db.WithContext(ctx).First(&physical, "server_id = ? AND name = ?", 1, "sda").Error; err != nil {
		t.Fatalf("First(DiskPhysicalMetric) error = %v", err)
	}
	if physical.TempC != diskTemp {
		t.Fatalf("disk temp = %.1f, want %.1f", physical.TempC, diskTemp)
	}
	var physicalCount int64
	if err := st.db.WithContext(ctx).Model(&model.DiskPhysicalMetric{}).Where("server_id = ?", 1).Count(&physicalCount).Error; err != nil {
		t.Fatalf("Count(DiskPhysicalMetric) error = %v", err)
	}
	if physicalCount != 1 {
		t.Fatalf("disk physical rows = %d, want 1", physicalCount)
	}
}

func newMetricIngestStore(t *testing.T) *Store {
	t.Helper()

	dsn := "file:" + url.QueryEscape(t.Name()) + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(
		&model.ServerMetric{},
		&model.DiskMetric{},
		&model.DiskPhysicalMetric{},
		&model.DiskUsageMetric{},
		&model.NICMetric{},
		&model.ServerCurrentMetric{},
		&model.ServerCurrentDiskMetric{},
		&model.ServerCurrentDiskUsageMetric{},
		&model.ServerCurrentNICMetric{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return New(db)
}

func testServerMetric(collectedAt time.Time, cpuUsageRatio float64) model.ServerMetric {
	return model.ServerMetric{
		ServerID:    1,
		CollectedAt: collectedAt,
		MetricsSnapshot: model.MetricsSnapshot{
			CPUUsageRatio: cpuUsageRatio,
			MemTotal:      1000,
			MemUsed:       int64(cpuUsageRatio * 1000),
		},
	}
}
