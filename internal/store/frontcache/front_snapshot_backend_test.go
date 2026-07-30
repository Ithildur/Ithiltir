package frontcache

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"dash/internal/infra/cachekeys"
	"dash/internal/metrics"
	"dash/internal/model"
	"dash/internal/store/frontprojection"
	pgtest "dash/internal/testutil/postgres"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type blockingReplaceBackend struct {
	cacheBackend
	entered chan struct{}
	release chan struct{}
}

func (b *blockingReplaceBackend) replaceSnapshot(ctx context.Context, nodes []frontNodeProjection) error {
	close(b.entered)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-b.release:
		return b.cacheBackend.replaceSnapshot(ctx, nodes)
	}
}

func newTestStore(db *gorm.DB, client *redis.Client) *Store {
	return New(db, client, frontprojection.New())
}

func TestIntegrationSnapshotRebuildSerializesOnlyCachePublish(t *testing.T) {
	st := newTestStore(pgtest.NewDB(t), nil)
	backend := &blockingReplaceBackend{
		cacheBackend: st.backend,
		entered:      make(chan struct{}),
		release:      make(chan struct{}),
	}
	st.backend = backend

	done := make(chan error, 1)
	go func() {
		_, err := st.rebuildSnapshot(t.Context(), 5*time.Second, 5*time.Second, 60)
		done <- err
	}()
	select {
	case <-backend.entered:
	case err := <-done:
		t.Fatalf("rebuildSnapshot() before publish = %v", err)
	}

	mutationEntered := make(chan struct{})
	mutationDone := make(chan error, 1)
	go func() {
		mutationDone <- st.projection.Mutate(func() error {
			close(mutationEntered)
			return nil
		})
	}()
	select {
	case <-mutationEntered:
		t.Fatal("projection mutation entered while snapshot cache publish was active")
	case <-time.After(20 * time.Millisecond):
	}
	close(backend.release)
	if err := <-done; err != nil {
		t.Fatalf("rebuildSnapshot() error = %v", err)
	}
	if err := <-mutationDone; err != nil {
		t.Fatalf("projection mutation error = %v", err)
	}
}

func TestRuntimeUpdatesDoNotChangeProjectionVersion(t *testing.T) {
	st := newTestStore(nil, nil)
	node := testNode("1", "node")
	if err := replaceTestSnapshot(st, t.Context(), []metrics.NodeView{node}); err != nil {
		t.Fatalf("replaceTestSnapshot() error = %v", err)
	}
	version := st.currentProjectionVersion()
	for i := 0; i < 10; i++ {
		node.CPU.UsageRatio = float64(i) / 10
		if err := st.PutNodeRuntime(t.Context(), node, 0, 0); err != nil {
			t.Fatalf("PutNodeRuntime() error = %v", err)
		}
	}
	if got := st.currentProjectionVersion(); got != version {
		t.Fatalf("projection version = %d, want %d after runtime-only writes", got, version)
	}
}

func TestUnknownRuntimeChangesProjectionVersionOnce(t *testing.T) {
	st := newTestStore(nil, nil)
	version := st.currentProjectionVersion()
	node := testNode("1", "unknown")
	for range 10 {
		if err := st.PutNodeRuntime(t.Context(), node, 0, 0); err != nil {
			t.Fatalf("PutNodeRuntime() error = %v", err)
		}
	}
	if got := st.currentProjectionVersion(); got != version+1 {
		t.Fatalf("projection version = %d, want %d after repeated unknown runtime", got, version+1)
	}
}

func TestStaleProjectionBuildDoesNotPublish(t *testing.T) {
	st := newTestStore(nil, nil)
	version := st.currentProjectionVersion()
	if err := st.projection.Mutate(func() error { return nil }); err != nil {
		t.Fatalf("projection mutation error = %v", err)
	}

	called := false
	published, err := st.publishProjectionIfCurrent(version, func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("publishProjectionIfCurrent() error = %v", err)
	}
	if published || called {
		t.Fatalf("stale build published=%v called=%v, want both false", published, called)
	}
}

func TestIntegrationEnsureSnapshotPublishesAfterMiss(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(pgtest.NewDB(t), nil)

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

func TestIntegrationEnsureSnapshotRetriesStaleBuild(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)
	st := newTestStore(db, nil)

	var once sync.Once
	var bumpErr error
	if err := db.Callback().Query().After("gorm:query").Register("test:invalidate-front-build", func(*gorm.DB) {
		once.Do(func() {
			bumpErr = st.projection.Mutate(func() error { return nil })
		})
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}

	if _, err := st.EnsureSnapshot(ctx, FrontSnapshotOptions{
		CacheTimeout:  time.Second,
		BuildTimeout:  time.Second,
		StaleAfterSec: 60,
	}); err != nil {
		t.Fatalf("EnsureSnapshot() error = %v", err)
	}
	if bumpErr != nil {
		t.Fatalf("projection mutation error = %v", bumpErr)
	}
	if _, ok, err := st.fetchSnapshotCache(ctx); err != nil || !ok {
		t.Fatalf("retried snapshot was not published, ok=%v err=%v", ok, err)
	}
}

func TestIntegrationEnsureSnapshotFailsClosedWhenBuildKeepsChanging(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)
	st := newTestStore(db, nil)

	var bumpErr error
	if err := db.Callback().Query().After("gorm:query").Register("test:keep-invalidating-front-build", func(*gorm.DB) {
		if bumpErr == nil {
			bumpErr = st.projection.Mutate(func() error { return nil })
		}
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}

	_, err := st.EnsureSnapshot(ctx, FrontSnapshotOptions{
		CacheTimeout:  time.Second,
		BuildTimeout:  time.Second,
		StaleAfterSec: 60,
	})
	if !errors.Is(err, errProjectionChanged) {
		t.Fatalf("EnsureSnapshot() error = %v, want projection-changed failure", err)
	}
	if bumpErr != nil {
		t.Fatalf("projection mutation error = %v", bumpErr)
	}
	if _, ok, err := st.fetchSnapshotCache(ctx); err != nil || ok {
		t.Fatalf("stale snapshot was published, ok=%v err=%v", ok, err)
	}
}

func TestIntegrationFetchFrontNodesReadsCurrentMetrics(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)
	st := newTestStore(db, nil)

	collectedAt := recentCollectedAt()
	srv := model.Server{
		Name:           "node-a",
		Hostname:       "node-a.local",
		Secret:         "secret-a",
		IsGuestVisible: true,
		Tags:           datatypes.JSON([]byte(`["edge","db"]`)),
	}
	if err := db.Create(&srv).Error; err != nil {
		t.Fatalf("Create(Server) error = %v", err)
	}
	if err := db.Create(&model.ServerMetric{
		ServerID:    srv.ID,
		CollectedAt: collectedAt.Add(time.Hour),
		MetricValues: model.MetricValues{
			CPUUsageRatio: 0.99,
			MemTotal:      1000,
			MemUsed:       900,
		},
	}).Error; err != nil {
		t.Fatalf("Create(ServerMetric) error = %v", err)
	}
	if err := db.Create(&model.ServerCurrentMetric{
		ServerID:    srv.ID,
		CollectedAt: collectedAt,
		MetricsSnapshot: model.MetricsSnapshot{
			MetricValues: model.MetricValues{
				CPUUsageRatio: 0.25,
				MemTotal:      1000,
				MemUsed:       250,
			},
		},
	}).Error; err != nil {
		t.Fatalf("Create(ServerCurrentMetric) error = %v", err)
	}
	if err := db.Create(&model.ServerCurrentNICMetric{
		ServerID:            srv.ID,
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

func TestIntegrationFetchCurrentNodeDoesNotDependOnRedis(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	st := newTestStore(db, client)

	server := model.Server{Name: "node-a", Hostname: "node-a", Secret: "secret-a"}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("Create(Server) error = %v", err)
	}
	collectedAt := recentCollectedAt()
	if err := db.Create(&model.ServerCurrentMetric{
		ServerID:    server.ID,
		CollectedAt: collectedAt,
		MetricsSnapshot: model.MetricsSnapshot{MetricValues: model.MetricValues{
			CPUUsageRatio: 0.75,
		}},
	}).Error; err != nil {
		t.Fatalf("Create(ServerCurrentMetric) error = %v", err)
	}
	redisServer.Close()

	node, err := st.FetchCurrentNode(ctx, server.ID, 60)
	if err != nil {
		t.Fatalf("FetchCurrentNode() error = %v", err)
	}
	if node == nil || node.CPU.UsageRatio != 0.75 {
		t.Fatalf("FetchCurrentNode() = %+v, want PostgreSQL current metric", node)
	}
}

func TestIntegrationFrontNodesComposeRuntimeFields(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	st := newTestStore(db, client)

	collectedAt := recentCollectedAt()
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
	server := model.Server{
		Name:           "node-a",
		Hostname:       "node-a.local",
		Secret:         "secret-a",
		IsGuestVisible: true,
	}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("Create(Server) error = %v", err)
	}
	if err := db.Create(&model.ServerCurrentMetric{
		ServerID:    server.ID,
		CollectedAt: collectedAt,
		MetricsSnapshot: model.MetricsSnapshot{
			MetricValues: model.MetricValues{
				CPUUsageRatio: 0.25,
				MemTotal:      1000,
				MemUsed:       250,
			},
			MetricRuntime: model.MetricRuntime{
				Thermal: thermalRaw,
			},
		},
	}).Error; err != nil {
		t.Fatalf("Create(ServerCurrentMetric) error = %v", err)
	}

	temp := 51.5
	health := "passed"
	cached := testNode(strconv.FormatInt(server.ID, 10), "node-a")
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
	if err := replaceTestSnapshot(st, ctx, []metrics.NodeView{cached}); err != nil {
		t.Fatalf("replaceTestSnapshot() error = %v", err)
	}
	if err := st.PutNodeRuntime(ctx, cached, 0, 0); err != nil {
		t.Fatalf("PutNodeRuntime() error = %v", err)
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

func TestApplyRuntimeMatchesReceivedAtInstant(t *testing.T) {
	temp := 51.5
	thermalTemp := 64.25
	node := metrics.NodeView{
		Observation: metrics.Observation{
			ReceivedAt: "2026-05-16T20:00:00+08:00",
		},
	}

	applySmartRuntime(&node, &frontSmartRuntime{
		ReceivedAt: "2026-05-16T12:00:00Z",
		Smart: &metrics.DiskSmart{Devices: []metrics.DiskSmartDevice{{
			Name:       "nvme0n1",
			DeviceType: "nvme",
			Protocol:   "NVMe",
			TempC:      &temp,
		}}},
	})
	applyThermalRuntime(&node, &frontThermalRuntime{
		ReceivedAt: "2026-05-16T12:00:00Z",
		Thermal: &metrics.Thermal{Sensors: []metrics.ThermalSensor{{
			TempC: &thermalTemp,
		}}},
	})

	assertRuntimeFields(t, node, temp, thermalTemp)
}

func recentCollectedAt() time.Time {
	return time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
}

func TestRedisFrontSnapshotCorruptionClearsPublishedState(t *testing.T) {
	tests := []struct {
		name   string
		seed   bool
		mutate func(*testing.T, context.Context, *redis.Client) string
	}{
		{
			name: "corrupt node",
			seed: true,
			mutate: func(t *testing.T, ctx context.Context, client *redis.Client) string {
				if err := client.Set(ctx, cachekeys.RedisKeyFrontNodeSnapshotPrefix+"1", "{", 0).Err(); err != nil {
					t.Fatalf("write corrupt front snapshot: %v", err)
				}
				return ""
			},
		},
		{
			name: "mismatched node ID",
			seed: true,
			mutate: func(t *testing.T, ctx context.Context, client *redis.Client) string {
				raw, err := json.Marshal(testNode("2", "two"))
				if err != nil {
					t.Fatalf("marshal front snapshot: %v", err)
				}
				if err := client.Set(ctx, cachekeys.RedisKeyFrontNodeSnapshotPrefix+"1", raw, 0).Err(); err != nil {
					t.Fatalf("write mismatched front snapshot: %v", err)
				}
				return ""
			},
		},
		{
			name: "missing node IDs",
			seed: true,
			mutate: func(t *testing.T, ctx context.Context, client *redis.Client) string {
				if err := client.Del(ctx, cachekeys.RedisKeyFrontNodeIDs).Err(); err != nil {
					t.Fatalf("delete front ids: %v", err)
				}
				return ""
			},
		},
		{
			name: "corrupt metadata",
			mutate: func(t *testing.T, ctx context.Context, client *redis.Client) string {
				if err := client.Set(ctx, cachekeys.RedisKeyFrontMeta, "{", 0).Err(); err != nil {
					t.Fatalf("write corrupt front meta: %v", err)
				}
				return ""
			},
		},
		{
			name: "corrupt SMART runtime",
			seed: true,
			mutate: func(t *testing.T, ctx context.Context, client *redis.Client) string {
				key := cachekeys.RedisKeyFrontNodeSmartPrefix + "1"
				if err := client.Set(ctx, key, "{", 0).Err(); err != nil {
					t.Fatalf("write corrupt smart runtime: %v", err)
				}
				return key
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			srv := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
			t.Cleanup(func() { _ = client.Close() })
			st := newTestStore(nil, client)
			if tt.seed {
				if err := replaceTestSnapshot(st, ctx, []metrics.NodeView{testNode("1", "one")}); err != nil {
					t.Fatalf("replace front snapshot: %v", err)
				}
			}
			corruptKey := tt.mutate(t, ctx, client)

			if _, ok, err := st.fetchSnapshotCache(ctx); err != nil || ok {
				t.Fatalf("corrupt front snapshot should miss, ok=%v err=%v", ok, err)
			}
			if exists := client.Exists(ctx, cachekeys.RedisKeyFrontMeta).Val(); exists != 0 {
				t.Fatal("corrupt front snapshot should clear front meta")
			}
			if corruptKey != "" && client.Exists(ctx, corruptKey).Val() != 0 {
				t.Fatalf("corrupt runtime key %q was not deleted", corruptKey)
			}
		})
	}
}

func TestRedisUnknownRuntimeInvalidatesWithoutJoiningCatalog(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	st := newTestStore(nil, client)
	if err := replaceTestSnapshot(st, ctx, nil); err != nil {
		t.Fatalf("replace empty front snapshot: %v", err)
	}
	if err := st.PutNodeRuntime(ctx, testNode("7", "unknown"), 0, 0); err != nil {
		t.Fatalf("put unknown runtime: %v", err)
	}

	if client.SIsMember(ctx, cachekeys.RedisKeyFrontNodeIDs, "7").Val() {
		t.Fatal("runtime-only node joined the PostgreSQL-derived catalog")
	}
	if exists := client.Exists(ctx, cachekeys.RedisKeyFrontNodeSnapshotPrefix+"7").Val(); exists != 1 {
		t.Fatalf("runtime key count = %d, want 1", exists)
	}
	if exists := client.Exists(ctx, cachekeys.RedisKeyFrontMeta).Val(); exists != 0 {
		t.Fatal("unknown runtime did not invalidate the front catalog")
	}
}

func TestRedisMetadataRebuildKeepsNewerRuntime(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	st := newTestStore(nil, client)
	old := testNode("1", "old")
	old.Observation.ReceivedAt = "2026-07-19T10:00:00Z"
	old.Observation.ObservedAt = old.Observation.ReceivedAt
	old.CPU.UsageRatio = 0.1
	if err := replaceTestSnapshot(st, ctx, []metrics.NodeView{old}); err != nil {
		t.Fatalf("replace initial snapshot: %v", err)
	}

	latest := old
	latest.Observation.ReceivedAt = "2026-07-19T10:01:00Z"
	latest.Observation.ObservedAt = latest.Observation.ReceivedAt
	latest.CPU.UsageRatio = 0.9
	if err := st.PutNodeRuntime(ctx, latest, 0, 0); err != nil {
		t.Fatalf("put latest runtime: %v", err)
	}

	rebuilt := old
	rebuilt.Node.Title = "renamed"
	if err := st.backend.replaceSnapshot(ctx, []frontNodeProjection{frontNodeProjectionFromView(rebuilt)}); err != nil {
		t.Fatalf("replace metadata projection: %v", err)
	}

	nodes, ok, err := st.fetchSnapshotCache(ctx)
	if err != nil || !ok || len(nodes) != 1 {
		t.Fatalf("fetch rebuilt snapshot: len=%d ok=%v err=%v", len(nodes), ok, err)
	}
	if nodes[0].Node.Title != "renamed" {
		t.Fatalf("title = %q, want renamed", nodes[0].Node.Title)
	}
	if nodes[0].Observation.ReceivedAt != latest.Observation.ReceivedAt || nodes[0].CPU.UsageRatio != latest.CPU.UsageRatio {
		t.Fatalf("runtime = %+v, want latest sample %+v", nodes[0], latest)
	}
}

func testNode(id, title string) metrics.NodeView {
	return metrics.NodeView{Node: metrics.NodeMeta{ID: id, Title: title}}
}

func replaceTestSnapshot(st *Store, ctx context.Context, nodes []metrics.NodeView) error {
	projections := make([]frontNodeProjection, 0, len(nodes))
	for _, node := range nodes {
		projections = append(projections, frontNodeProjectionFromView(node))
	}
	return st.backend.replaceSnapshot(ctx, projections)
}

func hasFrontSearchText(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
