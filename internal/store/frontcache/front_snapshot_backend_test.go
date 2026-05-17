package frontcache

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"testing"
	"time"

	"dash/internal/infra/cachekeys"
	"dash/internal/metrics"
	"dash/internal/model"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnsureSnapshotPublishesAfterMiss(t *testing.T) {
	ctx := context.Background()
	db := newFrontCacheDB(t)
	st := New(db, nil)

	nodes, err := st.EnsureSnapshot(ctx, FrontSnapshotOptions{
		CacheTimeout:  time.Second,
		BuildTimeout:  time.Second,
		StaleAfterSec: 60,
	})
	if err != nil || len(nodes) != 0 {
		t.Fatalf("ensure snapshot after miss: len=%d err=%v", len(nodes), err)
	}
	if _, ok, err := st.fetchSnapshotCache(ctx); err != nil || !ok {
		t.Fatalf("ensure snapshot should publish front meta, ok=%v err=%v", ok, err)
	}
}

func TestFetchFrontNodesReadsCurrentMetrics(t *testing.T) {
	ctx := context.Background()
	db := newFrontCacheDB(t)
	st := New(db, nil)

	collectedAt := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	if err := db.Create(&model.Server{
		ID:             1,
		Name:           "node-a",
		Hostname:       "node-a.local",
		Secret:         "secret-a",
		IsGuestVisible: true,
		Tags:           datatypes.JSON([]byte(`["edge","db"]`)),
	}).Error; err != nil {
		t.Fatalf("Create(Server) error = %v", err)
	}
	if err := db.Create(&model.ServerMetric{
		ServerID:    1,
		CollectedAt: collectedAt.Add(time.Hour),
		MetricsSnapshot: model.MetricsSnapshot{
			CPUUsageRatio: 0.99,
			MemTotal:      1000,
			MemUsed:       900,
		},
	}).Error; err != nil {
		t.Fatalf("Create(ServerMetric) error = %v", err)
	}
	if err := db.Create(&model.ServerCurrentMetric{
		ServerID:    1,
		CollectedAt: collectedAt,
		MetricsSnapshot: model.MetricsSnapshot{
			CPUUsageRatio: 0.25,
			MemTotal:      1000,
			MemUsed:       250,
		},
	}).Error; err != nil {
		t.Fatalf("Create(ServerCurrentMetric) error = %v", err)
	}
	if err := db.Create(&model.ServerCurrentNICMetric{
		ServerID:            1,
		Iface:               "eth0",
		CollectedAt:         collectedAt,
		BytesRecv:           10,
		BytesSent:           20,
		RecvRateBytesPerSec: 1.5,
		SentRateBytesPerSec: 2.5,
	}).Error; err != nil {
		t.Fatalf("Create(ServerCurrentNICMetric) error = %v", err)
	}

	nodes, err := st.FetchFrontNodes(ctx, 60, 0, 0, false)
	if err != nil {
		t.Fatalf("FetchFrontNodes() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("FetchFrontNodes() len = %d, want 1", len(nodes))
	}
	if nodes[0].CPU.UsageRatio != 0.25 {
		t.Fatalf("FetchFrontNodes() CPU ratio = %v, want current metric", nodes[0].CPU.UsageRatio)
	}
	if nodes[0].Network.Total.BytesRecv != 10 || nodes[0].Network.Total.RecvBPS != 1.5 {
		t.Fatalf("FetchFrontNodes() network total = %+v, want current nic", nodes[0].Network.Total)
	}
	if got := nodes[0].Node.Tags; len(got) != 2 || got[0] != "edge" || got[1] != "db" {
		t.Fatalf("FetchFrontNodes() tags = %v, want [edge db]", got)
	}
	if got := nodes[0].Node.SearchText; !hasFrontSearchText(got, "edge") || !hasFrontSearchText(got, "db") {
		t.Fatalf("FetchFrontNodes() search_text = %v, want tags indexed", got)
	}
}

func TestFrontNodesComposeRuntimeFields(t *testing.T) {
	ctx := context.Background()
	db := newFrontCacheDB(t)
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	st := New(db, client)

	collectedAt := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	thermalTemp := 64.25
	thermalRaw, err := json.Marshal(metrics.Thermal{
		Status: "ok",
		Sensors: []metrics.ThermalSensor{{
			Kind:      "cpu",
			Name:      "Package id 0",
			SensorKey: "coretemp.package_id_0",
			Source:    "sensors",
			Status:    "ok",
			TempC:     &thermalTemp,
		}},
	})
	if err != nil {
		t.Fatalf("Marshal(Thermal) error = %v", err)
	}
	if err := db.Create(&model.Server{
		ID:             1,
		Name:           "node-a",
		Hostname:       "node-a.local",
		Secret:         "secret-a",
		IsGuestVisible: true,
	}).Error; err != nil {
		t.Fatalf("Create(Server) error = %v", err)
	}
	if err := db.Create(&model.ServerCurrentMetric{
		ServerID:    1,
		CollectedAt: collectedAt,
		MetricsSnapshot: model.MetricsSnapshot{
			CPUUsageRatio: 0.25,
			MemTotal:      1000,
			MemUsed:       250,
			Thermal:       thermalRaw,
		},
	}).Error; err != nil {
		t.Fatalf("Create(ServerCurrentMetric) error = %v", err)
	}

	temp := 51.5
	health := "passed"
	cached := testNode("1", "node-a")
	cached.Observation.ReceivedAt = metrics.FormatTimestamp(collectedAt)
	cached.Disk.Smart = &metrics.DiskSmart{
		Status: "ok",
		Devices: []metrics.DiskSmartDevice{{
			Name:       "nvme0n1",
			DeviceType: "nvme",
			Protocol:   "NVMe",
			Source:     "smartctl",
			Status:     "ok",
			Health:     &health,
			TempC:      &temp,
		}},
	}
	if err := st.PutNodeSnapshot(ctx, cached); err != nil {
		t.Fatalf("PutNodeSnapshot() error = %v", err)
	}
	if err := st.ClearFrontMeta(ctx); err != nil {
		t.Fatalf("ClearFrontMeta() error = %v", err)
	}

	pagedNodes, err := st.FetchFrontNodes(ctx, 60, 1, 0, true)
	if err != nil {
		t.Fatalf("FetchFrontNodes() error = %v", err)
	}
	if len(pagedNodes) != 1 {
		t.Fatalf("FetchFrontNodes() len = %d, want 1", len(pagedNodes))
	}
	assertRuntimeFields(t, pagedNodes[0], temp, thermalTemp)

	nodes, err := st.EnsureSnapshot(ctx, FrontSnapshotOptions{
		CacheTimeout:  time.Second,
		BuildTimeout:  time.Second,
		StaleAfterSec: 60,
	})
	if err != nil {
		t.Fatalf("EnsureSnapshot() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("EnsureSnapshot() len = %d, want 1", len(nodes))
	}
	assertRuntimeFields(t, nodes[0], temp, thermalTemp)
}

func assertRuntimeFields(t *testing.T, node metrics.NodeView, smartTemp, thermalTemp float64) {
	t.Helper()

	if node.Disk.Smart == nil || len(node.Disk.Smart.Devices) != 1 {
		t.Fatalf("node smart = %+v, want cached runtime SMART", node.Disk.Smart)
	}
	gotSmartTemp := node.Disk.Smart.Devices[0].TempC
	if gotSmartTemp == nil || *gotSmartTemp != smartTemp {
		t.Fatalf("node SMART temp = %v, want %.1f", gotSmartTemp, smartTemp)
	}
	if len(node.Disk.TemperatureDevices) != 1 || node.Disk.TemperatureDevices[0] != "nvme0n1" {
		t.Fatalf("node disk temperature devices = %v, want [nvme0n1]", node.Disk.TemperatureDevices)
	}
	if node.Thermal == nil || len(node.Thermal.Sensors) != 1 {
		t.Fatalf("node thermal = %+v, want DB-backed runtime thermal", node.Thermal)
	}
	gotThermalTemp := node.Thermal.Sensors[0].TempC
	if gotThermalTemp == nil || *gotThermalTemp != thermalTemp {
		t.Fatalf("node thermal temp = %v, want %.2f", gotThermalTemp, thermalTemp)
	}
}

func TestReplaceFrontSnapshotRejectsMissingID(t *testing.T) {
	ctx := context.Background()
	st := New(nil, nil)

	if err := st.replaceFrontSnapshot(ctx, []metrics.NodeView{{}}); err == nil {
		t.Fatalf("replace front snapshot should reject node without id")
	}
	if err := st.replaceFrontSnapshot(ctx, []metrics.NodeView{
		testNode("1", "one"),
		testNode("1", "duplicate"),
	}); !errors.Is(err, errDuplicateFrontSnapshotID) {
		t.Fatalf("replace front snapshot duplicate id err=%v", err)
	}
	if _, ok, err := st.fetchSnapshotCache(ctx); err != nil || ok {
		t.Fatalf("failed front snapshot replace must not publish meta, ok=%v err=%v", ok, err)
	}
}

func TestRedisFrontSnapshotCorruptNodeClearsMeta(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	st := New(nil, client)
	if err := st.replaceFrontSnapshot(ctx, []metrics.NodeView{testNode("1", "one")}); err != nil {
		t.Fatalf("replace front snapshot: %v", err)
	}
	if err := client.Set(ctx, cachekeys.RedisKeyFrontNodeSnapshotPrefix+"1", "{", 0).Err(); err != nil {
		t.Fatalf("write corrupt front snapshot: %v", err)
	}

	if _, ok, err := st.fetchSnapshotCache(ctx); err != nil || ok {
		t.Fatalf("corrupt front snapshot should miss, ok=%v err=%v", ok, err)
	}
	if exists := client.Exists(ctx, cachekeys.RedisKeyFrontMeta).Val(); exists != 0 {
		t.Fatalf("corrupt front snapshot should clear front meta")
	}
}

func TestRedisFrontSnapshotMismatchedNodeIDClearsMeta(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	st := New(nil, client)
	if err := st.replaceFrontSnapshot(ctx, []metrics.NodeView{testNode("1", "one")}); err != nil {
		t.Fatalf("replace front snapshot: %v", err)
	}
	raw, err := json.Marshal(testNode("2", "two"))
	if err != nil {
		t.Fatalf("marshal front snapshot: %v", err)
	}
	if err := client.Set(ctx, cachekeys.RedisKeyFrontNodeSnapshotPrefix+"1", raw, 0).Err(); err != nil {
		t.Fatalf("write mismatched front snapshot: %v", err)
	}

	if _, ok, err := st.fetchSnapshotCache(ctx); err != nil || ok {
		t.Fatalf("mismatched front snapshot id should miss, ok=%v err=%v", ok, err)
	}
	if exists := client.Exists(ctx, cachekeys.RedisKeyFrontMeta).Val(); exists != 0 {
		t.Fatalf("mismatched front snapshot id should clear front meta")
	}
}

func TestRedisFrontSnapshotMissingIDsClearsMeta(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	st := New(nil, client)
	if err := st.replaceFrontSnapshot(ctx, []metrics.NodeView{testNode("1", "one")}); err != nil {
		t.Fatalf("replace front snapshot: %v", err)
	}
	if err := client.Del(ctx, cachekeys.RedisKeyFrontNodeIDs).Err(); err != nil {
		t.Fatalf("delete front ids: %v", err)
	}

	if _, ok, err := st.fetchSnapshotCache(ctx); err != nil || ok {
		t.Fatalf("missing front ids should miss without error, ok=%v err=%v", ok, err)
	}
	if exists := client.Exists(ctx, cachekeys.RedisKeyFrontMeta).Val(); exists != 0 {
		t.Fatalf("missing front ids should clear front meta")
	}
}

func TestRedisFrontSnapshotCorruptMetaClearsMeta(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	st := New(nil, client)
	if err := client.Set(ctx, cachekeys.RedisKeyFrontMeta, "{", 0).Err(); err != nil {
		t.Fatalf("write corrupt front meta: %v", err)
	}

	if _, ok, err := st.fetchSnapshotCache(ctx); err != nil || ok {
		t.Fatalf("corrupt front meta should miss without error, ok=%v err=%v", ok, err)
	}
	if exists := client.Exists(ctx, cachekeys.RedisKeyFrontMeta).Val(); exists != 0 {
		t.Fatalf("corrupt front meta should clear front meta")
	}
}

func TestRedisFrontSnapshotCorruptSmartRuntimeClearsMetaAndRuntime(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	st := New(nil, client)
	if err := st.replaceFrontSnapshot(ctx, []metrics.NodeView{testNode("1", "one")}); err != nil {
		t.Fatalf("replace front snapshot: %v", err)
	}
	smartKey := cachekeys.RedisKeyFrontNodeSmartPrefix + "1"
	if err := client.Set(ctx, smartKey, "{", 0).Err(); err != nil {
		t.Fatalf("write corrupt smart runtime: %v", err)
	}

	if _, ok, err := st.fetchSnapshotCache(ctx); err != nil || ok {
		t.Fatalf("corrupt smart runtime should miss, ok=%v err=%v", ok, err)
	}
	if exists := client.Exists(ctx, cachekeys.RedisKeyFrontMeta).Val(); exists != 0 {
		t.Fatalf("corrupt smart runtime should clear front meta")
	}
	if exists := client.Exists(ctx, smartKey).Val(); exists != 0 {
		t.Fatalf("corrupt smart runtime should be deleted")
	}
}

func TestRedisPatchAndRemoveNodeSnapshot(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	st := New(nil, client)
	node := testNode("1", "old")
	node.Node.Order = 1
	node.Node.SearchText = []string{"old", "linux"}
	if err := st.replaceFrontSnapshot(ctx, []metrics.NodeView{node}); err != nil {
		t.Fatalf("replace front snapshot: %v", err)
	}

	name := "new"
	order := 7
	if err := st.PatchNodeSnapshot(ctx, 1, &name, &order); err != nil {
		t.Fatalf("patch node snapshot: %v", err)
	}
	got, err := st.LoadFrontNodeSnapshot(ctx, 1)
	if err != nil {
		t.Fatalf("load patched node snapshot: %v", err)
	}
	if got == nil || got.Node.Title != name || got.Node.Order != order {
		t.Fatalf("patched snapshot = %+v, want title=%q order=%d", got, name, order)
	}
	if hasFrontSearchText(got.Node.SearchText, "old") {
		t.Fatalf("patched search text still contains old title: %v", got.Node.SearchText)
	}

	if err := st.RemoveNodeSnapshot(ctx, 1); err != nil {
		t.Fatalf("remove node snapshot: %v", err)
	}
	got, err = st.LoadFrontNodeSnapshot(ctx, 1)
	if err != nil {
		t.Fatalf("load removed node snapshot: %v", err)
	}
	if got != nil {
		t.Fatalf("removed snapshot still exists")
	}
	ids, err := st.ListFrontSnapshotIDs(ctx)
	if err != nil {
		t.Fatalf("list front snapshot ids: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("front snapshot ids after remove = %v, want empty", ids)
	}
	if exists := client.Exists(ctx, cachekeys.RedisKeyFrontMeta).Val(); exists != 0 {
		t.Fatalf("remove node snapshot should clear front meta")
	}
}

func testNode(id, title string) metrics.NodeView {
	return metrics.NodeView{Node: metrics.NodeMeta{ID: id, Title: title}}
}

func hasFrontSearchText(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func newFrontCacheDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:" + url.QueryEscape(t.Name()) + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(
		&model.Server{},
		&model.ServerMetric{},
		&model.ServerCurrentMetric{},
		&model.ServerCurrentDiskMetric{},
		&model.ServerCurrentDiskUsageMetric{},
		&model.ServerCurrentNICMetric{},
		&model.SystemSetting{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}
