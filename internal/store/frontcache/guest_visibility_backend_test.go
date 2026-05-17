package frontcache

import (
	"context"
	"testing"
	"time"

	"dash/internal/infra/cachekeys"
	"dash/internal/model"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisGuestVisibilityMissingIDsClearsMeta(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	st := New(nil, client)
	if err := st.replaceGuestVisibleIDs(ctx, map[int64]struct{}{1: {}}); err != nil {
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

	st := New(nil, client)
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

	st := New(nil, client)
	ids := []int64{1}

	if err := st.replaceGuestVisibleIDs(ctx, map[int64]struct{}{}); err != nil {
		t.Fatalf("replace empty guest visibility: %v", err)
	}
	got, ok, err := st.loadGuestVisibleIDs(ctx, ids)
	if err != nil || !ok || len(got) != 0 {
		t.Fatalf("published empty guest visibility should be a legal empty set, ok=%v got=%v err=%v", ok, got, err)
	}
}

func TestEnsureGuestVisibleIDsRebuildsAfterMetaClear(t *testing.T) {
	ctx := context.Background()
	db := newFrontCacheDB(t)
	if err := db.Create(&[]model.Server{
		{ID: 1, Name: "one", Hostname: "one", Secret: "one", IsGuestVisible: true},
		{ID: 2, Name: "two", Hostname: "two", Secret: "two", IsGuestVisible: false},
	}).Error; err != nil {
		t.Fatalf("create servers: %v", err)
	}

	st := New(db, nil)
	if err := st.replaceGuestVisibleIDs(ctx, map[int64]struct{}{1: {}}); err != nil {
		t.Fatalf("replace guest visibility: %v", err)
	}
	if err := st.ClearGuestVisibilityMeta(ctx); err != nil {
		t.Fatalf("clear guest visibility meta: %v", err)
	}

	got, err := st.EnsureGuestVisibleIDs(ctx, []int64{1, 2}, GuestVisibilityOptions{
		CacheTimeout: time.Second,
		BuildTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("ensure guest visibility: %v", err)
	}
	if _, ok := got[1]; !ok {
		t.Fatalf("expected guest-visible server 1")
	}
	if _, ok := got[2]; ok {
		t.Fatalf("server 2 should not be guest-visible")
	}
	if _, ok, err := st.loadGuestVisibleIDs(ctx, []int64{1, 2}); err != nil || !ok {
		t.Fatalf("ensure should republish guest visibility, ok=%v err=%v", ok, err)
	}
}

func TestGuestVisibilitySurvivesStoreRestart(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	first := New(nil, client)
	if err := first.replaceGuestVisibleIDs(ctx, map[int64]struct{}{1: {}}); err != nil {
		t.Fatalf("replace guest visibility: %v", err)
	}

	second := New(nil, client)
	got, ok, err := second.loadGuestVisibleIDs(ctx, []int64{1})
	if err != nil || !ok {
		t.Fatalf("expected redis-backed guest visibility hit after store restart, ok=%v err=%v", ok, err)
	}
	if _, hit := got[1]; !hit {
		t.Fatalf("expected guest-visible id after store restart")
	}
}
