package metricdata

import (
	"context"
	"strings"
	"testing"
	"time"

	"dash/internal/metrics"
	"dash/internal/model"
	pgtest "dash/internal/testutil/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMetricRowsRuntimeJSONStorage(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=127.0.0.1 user=test dbname=test password=test sslmode=disable",
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open dry-run gorm: %v", err)
	}

	metric := testServerMetric(1, time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC), 0.8)
	result := db.Session(&gorm.Session{DryRun: true}).Create(&metric)
	if result.Error != nil {
		t.Fatalf("insertServerMetric dry-run error = %v", result.Error)
	}
	sql := result.Statement.SQL.String()
	if strings.Contains(sql, `"raid"`) || strings.Contains(sql, `"thermal"`) {
		t.Fatalf("history insert SQL contains runtime JSON columns: %s", sql)
	}
	if !strings.Contains(sql, "cpu_usage_ratio") {
		t.Fatalf("history insert SQL missing metric columns: %s", sql)
	}

	current := model.ServerCurrentMetric{
		ServerID:    1,
		CollectedAt: metric.CollectedAt,
		MetricsSnapshot: model.MetricsSnapshot{
			MetricValues:  metric.MetricValues,
			MetricRuntime: testMetricRuntime(),
		},
	}
	currentResult := db.Session(&gorm.Session{DryRun: true}).Create(&current)
	if currentResult.Error != nil {
		t.Fatalf("current metric dry-run error = %v", currentResult.Error)
	}
	currentSQL := currentResult.Statement.SQL.String()
	if !strings.Contains(currentSQL, `"raid"`) || !strings.Contains(currentSQL, `"thermal"`) {
		t.Fatalf("current insert SQL missing runtime JSON columns: %s", currentSQL)
	}
}

func TestIntegrationSaveMetricsCurrentProjection(t *testing.T) {
	ctx := context.Background()
	st := New(pgtest.NewDB(t))
	srv := createMetricTestServer(t, st, "metric-node")

	newerAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	olderAt := newerAt.Add(-time.Minute)
	diskTemp := 41.0
	diskName := strings.Repeat("d", 255)
	diskRef := "disk:" + strings.Repeat("r", 315)
	diskPath := "/" + strings.Repeat("device/", 45)
	mountpoint := "/" + strings.Repeat("mount/", 45)
	updated, err := st.SaveMetrics(ctx, MetricsSample{
		ServerID: srv.ID,
		Metric:   testServerMetric(srv.ID, newerAt, 0.8),
		Runtime:  testMetricRuntime(),
		Updates:  map[string]any{"ip": "203.0.113.2"},
		DiskIO: []metrics.DiskBaseIOMetrics{{
			Kind:       "disk",
			Name:       diskName,
			Ref:        diskRef,
			DevicePath: diskPath,
			Role:       "primary",
			ReadBytes:  100,
		}},
		DiskSmart: &metrics.DiskSmart{Devices: []metrics.DiskSmartDevice{
			{Name: diskName, Ref: diskRef, DevicePath: diskPath, DeviceType: "sat", Protocol: "ATA", TempC: &diskTemp},
			{Name: "vda", DevicePath: "/dev/vda", DeviceType: "scsi", Protocol: "SCSI", Serial: "virt", TempC: &diskTemp},
			{Name: "md1", DevicePath: "/dev/md1", DeviceType: "sat", Protocol: "ATA", TempC: &diskTemp},
		}},
		DiskUsage: []metrics.DiskLogicalMetrics{{
			Kind:        "disk",
			Name:        diskName,
			Ref:         diskRef,
			DevicePath:  diskPath,
			Mountpoint:  mountpoint,
			Mountpoints: map[string]metrics.DiskMountpointMetrics{mountpoint: {FSType: "xfs"}},
			Total:       1000,
			Used:        400,
			Free:        600,
		}},
		Network: []metrics.NetIOMetrics{{Name: "eth0", BytesRecv: 100, RecvRateBytesPerSec: 10}},
	})
	if err != nil {
		t.Fatalf("SaveMetrics(newer) error = %v", err)
	}
	if !updated {
		t.Fatal("SaveMetrics(newer) did not update current projection")
	}
	updated, err = st.SaveMetrics(ctx, MetricsSample{
		ServerID:  srv.ID,
		Metric:    testServerMetric(srv.ID, olderAt, 0.2),
		Runtime:   testMetricRuntime(),
		Updates:   map[string]any{"ip": "198.51.100.2"},
		DiskIO:    []metrics.DiskBaseIOMetrics{{Name: "sdb", Role: "primary", ReadBytes: 20}},
		DiskUsage: []metrics.DiskLogicalMetrics{{Name: "old-root", Total: 1000, Used: 200, Free: 800}},
		Network:   []metrics.NetIOMetrics{{Name: "eth1", BytesRecv: 20, RecvRateBytesPerSec: 2}},
	})
	if err != nil {
		t.Fatalf("SaveMetrics(older) error = %v", err)
	}
	if updated {
		t.Fatal("SaveMetrics(older) unexpectedly updated current projection")
	}
	var gotServer model.Server
	if err := st.db.WithContext(ctx).First(&gotServer, srv.ID).Error; err != nil {
		t.Fatalf("First(Server) error = %v", err)
	}
	if gotServer.IP == nil || *gotServer.IP != "203.0.113.2" {
		t.Fatalf("server IP = %v, want newer observation", gotServer.IP)
	}

	var current model.ServerCurrentMetric
	if err := st.db.WithContext(ctx).First(&current, "server_id = ?", srv.ID).Error; err != nil {
		t.Fatalf("First(ServerCurrentMetric) error = %v", err)
	}
	if !current.CollectedAt.Equal(newerAt) || current.CPUUsageRatio != 0.8 {
		t.Fatalf("current metric = collected_at %v cpu %v, want newer sample", current.CollectedAt, current.CPUUsageRatio)
	}
	if current.PSIMemFullAvg300 == nil || *current.PSIMemFullAvg300 != 1.5 {
		t.Fatalf("current memory PSI full avg300 = %v, want 1.5", current.PSIMemFullAvg300)
	}
	if len(current.Raid) == 0 || len(current.Thermal) == 0 {
		t.Fatalf("current runtime JSON missing: raid=%q thermal=%q", current.Raid, current.Thermal)
	}

	var historyRuntime struct {
		RaidIsNull    bool
		ThermalIsNull bool
	}
	if err := st.db.WithContext(ctx).
		Raw("SELECT raid IS NULL AS raid_is_null, thermal IS NULL AS thermal_is_null FROM server_metrics WHERE server_id = ? AND collected_at = ?", srv.ID, newerAt).
		Scan(&historyRuntime).Error; err != nil {
		t.Fatalf("load history runtime columns: %v", err)
	}
	if !historyRuntime.RaidIsNull || !historyRuntime.ThermalIsNull {
		t.Fatalf("history runtime JSON = raid null %v thermal null %v, want both null", historyRuntime.RaidIsNull, historyRuntime.ThermalIsNull)
	}

	var nic model.ServerCurrentNICMetric
	if err := st.db.WithContext(ctx).First(&nic, "server_id = ?", srv.ID).Error; err != nil {
		t.Fatalf("First(ServerCurrentNICMetric) error = %v", err)
	}
	if nic.Iface != "eth0" || nic.BytesRecv != 100 {
		t.Fatalf("current nic = %+v, want newer nic", nic)
	}

	var physical model.DiskPhysicalMetric
	if err := st.db.WithContext(ctx).First(&physical, "server_id = ? AND name = ?", srv.ID, diskName).Error; err != nil {
		t.Fatalf("First(DiskPhysicalMetric) error = %v", err)
	}
	if physical.Ref != diskRef || physical.Path != diskPath || physical.TempC != diskTemp {
		t.Fatalf("physical disk = %+v, want expanded identity and temp %.1f", physical, diskTemp)
	}
	var currentDisk model.ServerCurrentDiskMetric
	if err := st.db.WithContext(ctx).First(&currentDisk, "server_id = ? AND name = ?", srv.ID, diskName).Error; err != nil {
		t.Fatalf("First(ServerCurrentDiskMetric) error = %v", err)
	}
	if currentDisk.Ref != diskRef || currentDisk.Path != diskPath {
		t.Fatalf("current disk identity = ref %q path %q", currentDisk.Ref, currentDisk.Path)
	}
	var currentUsage model.ServerCurrentDiskUsageMetric
	if err := st.db.WithContext(ctx).First(&currentUsage, "server_id = ? AND name = ?", srv.ID, diskName).Error; err != nil {
		t.Fatalf("First(ServerCurrentDiskUsageMetric) error = %v", err)
	}
	if currentUsage.Ref != diskRef || currentUsage.Path != diskPath || currentUsage.Mountpoint != mountpoint {
		t.Fatalf("current disk usage identity = ref %q path %q mountpoint %q", currentUsage.Ref, currentUsage.Path, currentUsage.Mountpoint)
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
	pressure := cpuUsageRatio + 0.7
	return model.ServerMetric{
		ServerID:    serverID,
		CollectedAt: collectedAt,
		MetricValues: model.MetricValues{
			CPUUsageRatio:    cpuUsageRatio,
			MemTotal:         1000,
			MemUsed:          int64(cpuUsageRatio * 1000),
			PSIMemFullAvg300: &pressure,
		},
	}
}

func testMetricRuntime() model.MetricRuntime {
	return model.MetricRuntime{
		Raid:    []byte(`{"supported":true,"available":true,"arrays":[{"name":"md0","status":"clean","active":2,"working":2,"failed":0,"health":"ok","members":[{"name":"sda","state":"active"}]}]}`),
		Thermal: []byte(`{"status":"ok","sensors":[{"kind":"cpu","name":"package","sensor_key":"cpu:package","source":"lm-sensors","status":"ok","temp_c":52.5}]}`),
	}
}
