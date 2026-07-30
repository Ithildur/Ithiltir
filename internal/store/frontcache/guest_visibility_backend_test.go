package frontcache

import (
	"context"
	"sync"
	"testing"
	"time"

	"dash/internal/infra/cachekeys"
	"dash/internal/model"
	pgtest "dash/internal/testutil/postgres"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func TestRedisGuestVisibilityMissingIDsClearsMeta(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	st := newTestStore(nil, client)
	if err := st.backend.replaceGuestVisibleIDs(ctx, map[int64]struct{}{1: {}}); err != nil {
		t.Fatalf("replace guest visibility: %v", err)
	}
	if err := client.Del(ctx, cachekeys.RedisKeyGuestVisibleIDs).Err(); err != nil {
		t.Fatalf("delete guest visibility ids: %v", err)
	}

	if _, ok, err := st.loadGuestVisibleIDs(ctx, []int64{1}); err != nil || ok {
		t.Fatalf("missing guest visibility ids should miss without error, ok=%v err=%v", ok, err)
	}
	if exists := client.Exists(ctx, cachekeys.RedisKeyGuestVisibilityMeta).Val(); exists != 0 {
		t.Fatalf("missing guest visibility ids should clear guest visibility meta")
	}
}

func TestRedisGuestVisibilityCorruptMetaClearsMeta(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	st := newTestStore(nil, client)
	if err := client.Set(ctx, cachekeys.RedisKeyGuestVisibilityMeta, "{", 0).Err(); err != nil {
		t.Fatalf("write corrupt guest visibility meta: %v", err)
	}

	if _, ok, err := st.loadGuestVisibleIDs(ctx, []int64{1}); err != nil || ok {
		t.Fatalf("corrupt guest visibility meta should miss without error, ok=%v err=%v", ok, err)
	}
	if exists := client.Exists(ctx, cachekeys.RedisKeyGuestVisibilityMeta).Val(); exists != 0 {
		t.Fatalf("corrupt guest visibility meta should clear guest visibility meta")
	}
}

func TestGuestVisibilityPublishedEmptySet(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	st := newTestStore(nil, client)
	ids := []int64{1}

	if err := st.backend.replaceGuestVisibleIDs(ctx, map[int64]struct{}{}); err != nil {
		t.Fatalf("replace empty guest visibility: %v", err)
	}
	got, ok, err := st.loadGuestVisibleIDs(ctx, ids)
	if err != nil || !ok || len(got) != 0 {
		t.Fatalf("published empty guest visibility should be a legal empty set, ok=%v got=%v err=%v", ok, got, err)
	}
}

func TestIntegrationEnsureGuestVisibleIDs(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)

	t.Run("rebuild after metadata clear", func(t *testing.T) {
		servers := []model.Server{
			{Name: "one", Hostname: "one", Secret: "one", IsGuestVisible: true},
			{Name: "two", Hostname: "two", Secret: "two", IsGuestVisible: false},
		}
		if err := db.Create(&servers).Error; err != nil {
			t.Fatalf("create servers: %v", err)
		}
		visibleID := servers[0].ID
		hiddenID := servers[1].ID

		st := newTestStore(db, nil)
		if err := st.backend.replaceGuestVisibleIDs(ctx, map[int64]struct{}{visibleID: {}}); err != nil {
			t.Fatalf("replace guest visibility: %v", err)
		}
		if err := st.ClearGuestVisibilityMeta(ctx); err != nil {
			t.Fatalf("clear guest visibility meta: %v", err)
		}

		got, err := st.EnsureGuestVisibleIDs(ctx, []int64{visibleID, hiddenID}, GuestVisibilityOptions{
			CacheTimeout: time.Second,
			BuildTimeout: time.Second,
		})
		if err != nil {
			t.Fatalf("ensure guest visibility: %v", err)
		}
		if _, ok := got[visibleID]; !ok {
			t.Fatalf("expected guest-visible server %d", visibleID)
		}
		if _, ok := got[hiddenID]; ok {
			t.Fatalf("server %d should not be guest-visible", hiddenID)
		}
		if _, ok, err := st.loadGuestVisibleIDs(ctx, []int64{visibleID, hiddenID}); err != nil || !ok {
			t.Fatalf("ensure should republish guest visibility, ok=%v err=%v", ok, err)
		}
	})

	t.Run("retry stale build", func(t *testing.T) {
		server := model.Server{Name: "visible", Hostname: "visible", Secret: "visible", IsGuestVisible: true}
		if err := db.Create(&server).Error; err != nil {
			t.Fatalf("create server: %v", err)
		}
		st := newTestStore(db, nil)

		var once sync.Once
		var bumpErr error
		if err := db.Callback().Query().After("gorm:query").Register("test:invalidate-guest-build", func(*gorm.DB) {
			once.Do(func() {
				bumpErr = st.projection.Mutate(func() error { return nil })
			})
		}); err != nil {
			t.Fatalf("register query callback: %v", err)
		}

		allowed, err := st.EnsureGuestVisibleIDs(ctx, []int64{server.ID}, GuestVisibilityOptions{
			CacheTimeout: time.Second,
			BuildTimeout: time.Second,
		})
		if err != nil {
			t.Fatalf("EnsureGuestVisibleIDs() error = %v", err)
		}
		if bumpErr != nil {
			t.Fatalf("projection mutation error = %v", bumpErr)
		}
		if _, ok := allowed[server.ID]; !ok {
			t.Fatalf("server %d was not returned as guest-visible", server.ID)
		}
		if _, ok, err := st.loadGuestVisibleIDs(ctx, []int64{server.ID}); err != nil || !ok {
			t.Fatalf("retried guest visibility was not published, ok=%v err=%v", ok, err)
		}
	})
}

func TestGuestVisibilitySurvivesStoreRestart(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	first := newTestStore(nil, client)
	if err := first.backend.replaceGuestVisibleIDs(ctx, map[int64]struct{}{1: {}}); err != nil {
		t.Fatalf("replace guest visibility: %v", err)
	}

	second := newTestStore(nil, client)
	got, ok, err := second.loadGuestVisibleIDs(ctx, []int64{1})
	if err != nil || !ok {
		t.Fatalf("expected redis-backed guest visibility hit after store restart, ok=%v err=%v", ok, err)
	}
	if _, hit := got[1]; !hit {
		t.Fatalf("expected guest-visible id after store restart")
	}
}
