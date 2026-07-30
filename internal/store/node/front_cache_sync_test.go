package node

import (
	"context"
	"errors"
	"testing"
	"time"

	"dash/internal/model"
	"dash/internal/store/frontcache"
	pgtest "dash/internal/testutil/postgres"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestIntegrationCreateNodeRollsBackWhenRedisIsUnavailable(t *testing.T) {
	st, redisServer := newRedisIntegrationStore(t)
	redisServer.Close()

	const secret = "create-redis-failure-secret"
	if _, err := st.CreateNode(context.Background(), secret); !errors.Is(err, ErrFrontCacheUpdate) {
		t.Fatalf("CreateNode() error = %v, want ErrFrontCacheUpdate", err)
	}

	var count int64
	if err := st.db.Model(&model.Server{}).Where("secret = ?", secret).Count(&count).Error; err != nil {
		t.Fatalf("Count(server) error = %v", err)
	}
	if count != 0 {
		t.Fatalf("server count = %d, want 0", count)
	}
}

func TestIntegrationDeleteNodeRollsBackWhenRedisIsUnavailable(t *testing.T) {
	st, redisServer := newRedisIntegrationStore(t)
	srv := createCacheSyncServer(t, st, "delete-redis-failure", 1)
	if err := st.SyncServerCache(context.Background(), srv); err != nil {
		t.Fatalf("SyncServerCache() error = %v", err)
	}
	redisServer.Close()

	if err := st.DeleteNode(context.Background(), srv.ID); !errors.Is(err, ErrFrontCacheUpdate) {
		t.Fatalf("DeleteNode() error = %v, want ErrFrontCacheUpdate", err)
	}

	var stored model.Server
	if err := st.db.First(&stored, srv.ID).Error; err != nil {
		t.Fatalf("First(server) error = %v", err)
	}
	if stored.IsDeleted {
		t.Fatal("server is_deleted = true, want false")
	}
	if _, err := st.GetServerBySecret(context.Background(), srv.Secret); err != nil {
		t.Fatalf("GetServerBySecret() error = %v", err)
	}
}

func TestIntegrationUpdateDisplayOrderRollsBackWhenRedisIsUnavailable(t *testing.T) {
	st, redisServer := newRedisIntegrationStore(t)
	first := createCacheSyncServer(t, st, "order-first", 2)
	second := createCacheSyncServer(t, st, "order-second", 1)
	if err := st.SyncServerCache(context.Background(), first); err != nil {
		t.Fatalf("SyncServerCache(first) error = %v", err)
	}
	if err := st.SyncServerCache(context.Background(), second); err != nil {
		t.Fatalf("SyncServerCache(second) error = %v", err)
	}
	redisServer.Close()

	if err := st.UpdateDisplayOrder(context.Background(), []int64{second.ID, first.ID}); !errors.Is(err, ErrFrontCacheUpdate) {
		t.Fatalf("UpdateDisplayOrder() error = %v, want ErrFrontCacheUpdate", err)
	}

	var stored []model.Server
	if err := st.db.Where("id IN ?", []int64{first.ID, second.ID}).Order("id ASC").Find(&stored).Error; err != nil {
		t.Fatalf("load stored order: %v", err)
	}
	if len(stored) != 2 || stored[0].DisplayOrder != 2 || stored[1].DisplayOrder != 1 {
		t.Fatalf("stored display order = %+v, want original order", stored)
	}
	for _, srv := range []model.Server{first, second} {
		cached, err := st.GetServerBySecret(context.Background(), srv.Secret)
		if err != nil {
			t.Fatalf("GetServerBySecret(%q) error = %v", srv.Secret, err)
		}
		if cached.DisplayOrder != srv.DisplayOrder {
			t.Fatalf("cached display order for %d = %d, want %d", srv.ID, cached.DisplayOrder, srv.DisplayOrder)
		}
	}
}

func TestIntegrationUpdateNodeRollsBackWhenRedisIsUnavailable(t *testing.T) {
	st, redisServer := newRedisIntegrationStore(t)
	srv := createCacheSyncServer(t, st, "update-redis-failure", 1)
	if err := st.SyncServerCache(context.Background(), srv); err != nil {
		t.Fatalf("SyncServerCache() error = %v", err)
	}
	redisServer.Close()

	name := "not-committed"
	if err := st.UpdateNode(context.Background(), srv.ID, NodeUpdate{Name: &name}); !errors.Is(err, ErrFrontCacheUpdate) {
		t.Fatalf("UpdateNode() error = %v, want ErrFrontCacheUpdate", err)
	}

	var stored model.Server
	if err := st.db.First(&stored, srv.ID).Error; err != nil {
		t.Fatalf("First(server) error = %v", err)
	}
	if stored.Name != srv.Name {
		t.Fatalf("stored name = %q, want %q", stored.Name, srv.Name)
	}
	cached, err := st.GetServerBySecret(context.Background(), srv.Secret)
	if err != nil {
		t.Fatalf("GetServerBySecret() error = %v", err)
	}
	if cached.Name != srv.Name {
		t.Fatalf("cached name = %q, want %q", cached.Name, srv.Name)
	}
}

func TestIntegrationUpdateStaticRollsBackWhenRedisIsUnavailable(t *testing.T) {
	st, redisServer := newRedisIntegrationStore(t)
	srv := createCacheSyncServer(t, st, "static-redis-failure", 1)
	seedFrontSnapshot(t, st, srv, 0.25)
	redisServer.Close()

	static := ServerStaticPatch{
		Hostname: strPtr("not-committed"),
		OS:       strPtr("linux"),
		Arch:     strPtr("amd64"),
	}
	if err := st.UpdateStatic(context.Background(), srv.Secret, srv.ID, static); !errors.Is(err, ErrFrontCacheUpdate) {
		t.Fatalf("UpdateStatic() error = %v, want ErrFrontCacheUpdate", err)
	}

	var stored model.Server
	if err := st.db.First(&stored, srv.ID).Error; err != nil {
		t.Fatalf("First(server) error = %v", err)
	}
	if stored.Hostname != srv.Hostname {
		t.Fatalf("stored hostname = %q, want %q", stored.Hostname, srv.Hostname)
	}
}

func TestIntegrationUpdateNodeInvalidatesMetadataButKeepsRuntime(t *testing.T) {
	st := newIntegrationStore(t)
	ctx := context.Background()
	srv := createCacheSyncServer(t, st, "old", 1)
	seedFrontSnapshot(t, st, srv, 0.25)
	if err := st.db.Model(&model.ServerCurrentMetric{}).
		Where("server_id = ?", srv.ID).
		Update("cpu_usage_ratio", 0.9).Error; err != nil {
		t.Fatalf("update current metric: %v", err)
	}

	name := "new"
	order := 7
	if err := st.UpdateNode(ctx, srv.ID, NodeUpdate{Name: &name, DisplayOrder: &order}); err != nil {
		t.Fatalf("UpdateNode() error = %v", err)
	}

	nodes, err := st.front.EnsureSnapshot(ctx, frontcache.FrontSnapshotOptions{
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
	if nodes[0].Node.Title != name || nodes[0].Node.Order != order {
		t.Fatalf("rebuilt metadata = %+v, want title %q and order %d", nodes[0].Node, name, order)
	}
	if nodes[0].CPU.UsageRatio != 0.25 {
		t.Fatalf("rebuilt CPU usage = %v, want retained runtime 0.25", nodes[0].CPU.UsageRatio)
	}
}

func TestIntegrationUpdateStaticWithoutFrontFieldsDoesNotRequireRedis(t *testing.T) {
	st, redisServer := newRedisIntegrationStore(t)
	srv := createCacheSyncServer(t, st, "static-runtime-only", 1)
	redisServer.Close()

	hostname := "updated-hostname"
	if err := st.UpdateStatic(context.Background(), srv.Secret, srv.ID, ServerStaticPatch{Hostname: &hostname}); err != nil {
		t.Fatalf("UpdateStatic() error = %v", err)
	}

	var stored model.Server
	if err := st.db.First(&stored, srv.ID).Error; err != nil {
		t.Fatalf("First(server) error = %v", err)
	}
	if stored.Hostname != hostname {
		t.Fatalf("stored hostname = %q, want %q", stored.Hostname, hostname)
	}
}

func TestIntegrationDeleteNodeRemovesFrontNodeSnapshot(t *testing.T) {
	st := newIntegrationStore(t)
	ctx := context.Background()
	srv := createCacheSyncServer(t, st, "node", 1)
	seedFrontSnapshot(t, st, srv, 0.25)

	if err := st.DeleteNode(ctx, srv.ID); err != nil {
		t.Fatalf("DeleteNode() error = %v", err)
	}

	nodes, err := st.front.EnsureSnapshot(ctx, frontcache.FrontSnapshotOptions{
		CacheTimeout:  time.Second,
		BuildTimeout:  time.Second,
		StaleAfterSec: 60,
	})
	if err != nil {
		t.Fatalf("EnsureSnapshot() error = %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("EnsureSnapshot() = %+v, want deleted node absent", nodes)
	}
}

func createCacheSyncServer(t *testing.T, st *Store, name string, order int) model.Server {
	t.Helper()

	srv := model.Server{
		Name:         name,
		Hostname:     name,
		Secret:       name + "-secret",
		DisplayOrder: order,
	}
	if err := st.db.Create(&srv).Error; err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return srv
}

func seedFrontSnapshot(t *testing.T, st *Store, srv model.Server, cpuUsage float64) {
	t.Helper()

	collectedAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	current := model.ServerCurrentMetric{
		ServerID:    srv.ID,
		CollectedAt: collectedAt,
		MetricsSnapshot: model.MetricsSnapshot{
			MetricValues: model.MetricValues{
				CPUUsageRatio: cpuUsage,
			},
		},
	}
	if err := st.db.Create(&current).Error; err != nil {
		t.Fatalf("Create(ServerCurrentMetric) error = %v", err)
	}
	nodes, err := st.front.EnsureSnapshot(context.Background(), frontcache.FrontSnapshotOptions{
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
}

func newRedisIntegrationStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()

	db := pgtest.NewDB(t)
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return newTestStore(db, client), redisServer
}
