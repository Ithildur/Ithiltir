package node

import (
	"context"
	"strconv"
	"testing"
	"time"

	"dash/internal/infra/cachekeys"
	"dash/internal/metrics"
	"dash/internal/model"
	"dash/internal/store/frontcache"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestUpdateNodePatchesFrontNodeSnapshot(t *testing.T) {
	st := newSQLiteStore(t)
	ctx := context.Background()
	srv := createCacheSyncServer(t, st, "old", 1)
	putFrontNodeSnapshot(t, st, srv, "old", 1)

	name := "new"
	order := 7
	if err := st.UpdateNode(ctx, srv.ID, NodeUpdate{Name: &name, DisplayOrder: &order}); err != nil {
		t.Fatalf("UpdateNode() error = %v", err)
	}

	got, err := st.front.LoadFrontNodeSnapshot(ctx, srv.ID)
	if err != nil {
		t.Fatalf("LoadFrontNodeSnapshot() error = %v", err)
	}
	if got == nil {
		t.Fatalf("LoadFrontNodeSnapshot() returned nil")
	}
	if got.Node.Title != name {
		t.Fatalf("front title = %q, want %q", got.Node.Title, name)
	}
	if got.Node.Order != order {
		t.Fatalf("front order = %d, want %d", got.Node.Order, order)
	}
	if hasText(got.Node.SearchText, "old") {
		t.Fatalf("front search text still contains old title: %v", got.Node.SearchText)
	}
	if !hasText(got.Node.SearchText, name) {
		t.Fatalf("front search text missing new title: %v", got.Node.SearchText)
	}
}

func TestDeleteNodeRemovesFrontNodeSnapshot(t *testing.T) {
	st := newSQLiteStore(t)
	ctx := context.Background()
	srv := createCacheSyncServer(t, st, "node", 1)
	putFrontNodeSnapshot(t, st, srv, "node", 1)

	if err := st.DeleteNode(ctx, srv.ID); err != nil {
		t.Fatalf("DeleteNode() error = %v", err)
	}

	got, err := st.front.LoadFrontNodeSnapshot(ctx, srv.ID)
	if err != nil {
		t.Fatalf("LoadFrontNodeSnapshot() error = %v", err)
	}
	if got != nil {
		t.Fatalf("deleted node snapshot still exists")
	}
	ids, err := st.front.ListFrontSnapshotIDs(ctx)
	if err != nil {
		t.Fatalf("ListFrontSnapshotIDs() error = %v", err)
	}
	for _, id := range ids {
		if id == srv.ID {
			t.Fatalf("deleted node id still listed in front snapshot ids")
		}
	}
}

func TestCreateNodeClearsFrontSnapshotMeta(t *testing.T) {
	ctx := context.Background()
	db := newSQLiteDB(t)
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	front := frontcache.New(db, client)
	st := New(db, client, front)
	if _, err := front.EnsureSnapshot(ctx, frontcache.FrontSnapshotOptions{
		CacheTimeout:  time.Second,
		BuildTimeout:  time.Second,
		StaleAfterSec: 60,
	}); err != nil {
		t.Fatalf("EnsureSnapshot() error = %v", err)
	}
	if exists := client.Exists(ctx, cachekeys.RedisKeyFrontMeta).Val(); exists != 1 {
		t.Fatalf("front meta before create exists=%d, want 1", exists)
	}

	if _, err := st.CreateNode(ctx, "secret"); err != nil {
		t.Fatalf("CreateNode() error = %v", err)
	}
	if exists := client.Exists(ctx, cachekeys.RedisKeyFrontMeta).Val(); exists != 0 {
		t.Fatalf("front meta after create exists=%d, want 0", exists)
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

func putFrontNodeSnapshot(t *testing.T, st *Store, srv model.Server, title string, order int) {
	t.Helper()

	node := metrics.NodeView{
		Node: metrics.NodeMeta{
			ID:         strconv.FormatInt(srv.ID, 10),
			Title:      title,
			Order:      order,
			SearchText: []string{title, "linux"},
		},
	}
	if err := st.front.PutNodeSnapshot(context.Background(), node); err != nil {
		t.Fatalf("PutNodeSnapshot() error = %v", err)
	}
}

func hasText(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
