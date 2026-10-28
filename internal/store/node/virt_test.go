package node

import (
	"errors"
	"testing"
	"time"

	"dash/internal/model"
	"dash/internal/virt"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func TestVirtMemoryOwnership(t *testing.T) {
	s := newTestStore(nil, nil)
	server := model.Server{ID: 1, Secret: "virt-memory-secret", VMCPUThreshold: 80, VMMemoryThreshold: 90}
	if err := s.SyncServerCache(t.Context(), server); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	sample := virt.Snapshot{Schema: 1, Provider: "pve", Host: "pve", CollectedAt: now, LastSuccessAt: &now, TTLSeconds: 90, Status: "ok", VMs: []virt.VM{{ID: 100, Name: "guest", Status: "running", IPs: []string{"192.0.2.1"}}}}
	if err := s.SaveVirt(t.Context(), server.ID, server.Secret, sample, now); err != nil {
		t.Fatal(err)
	}
	sample.VMs[0].IPs[0] = "192.0.2.2"
	first, err := s.Virt(t.Context(), server.ID)
	if err != nil || first.Snapshot.VMs[0].IPs[0] != "192.0.2.1" {
		t.Fatalf("caller changed stored snapshot: %+v %v", first, err)
	}
	first.Snapshot.VMs[0].IPs[0] = "192.0.2.3"
	second, err := s.Virt(t.Context(), server.ID)
	if err != nil || second.Snapshot.VMs[0].IPs[0] != "192.0.2.1" {
		t.Fatalf("reader changed stored snapshot: %+v %v", second, err)
	}
	sample.CollectedAt = now.Add(time.Millisecond)
	sample.LastSuccessAt = new(sample.CollectedAt)
	sample.VMs[0].Name = "replacement"
	if err := s.SaveVirt(t.Context(), server.ID, server.Secret, sample, now); err != nil {
		t.Fatal(err)
	}
	latest, err := s.Virt(t.Context(), server.ID)
	if err != nil || latest.Snapshot == nil || latest.Snapshot.VMs[0].Name != "replacement" || len(s.mem.virt) != 1 {
		t.Fatalf("replacement did not retain only the latest snapshot: %+v %v", latest, err)
	}
	s.removeServerState(t.Context(), server.ID, server.Secret)
	if len(s.mem.virt) != 0 {
		t.Fatal("deleted node retained VM memory")
	}
}

func TestVirtCacheTTL(t *testing.T) {
	const retention = 10 * time.Minute
	for _, mode := range []string{"memory", "redis"} {
		t.Run(mode, func(t *testing.T) {
			var client *redis.Client
			var cache *miniredis.Miniredis
			if mode == "redis" {
				cache = miniredis.RunT(t)
				client = redis.NewClient(&redis.Options{Addr: cache.Addr(), ContextTimeoutEnabled: true, MaxRetries: -1})
				t.Cleanup(func() { _ = client.Close() })
			}
			s := newTestStore(nil, client)
			server := model.Server{ID: 1, Secret: "virt-cache-secret", VMCPUThreshold: 80, VMMemoryThreshold: 90}
			if err := s.SyncServerCache(t.Context(), server); err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			received := now.Add(-9 * time.Minute)
			sample := virt.Snapshot{Schema: 1, Provider: "pve", Host: "pve", CollectedAt: received, LastSuccessAt: new(received), TTLSeconds: 90, Status: "ok", VMs: []virt.VM{}}
			put := func(sample virt.Snapshot, at time.Time) {
				t.Helper()
				if err := s.SaveVirt(t.Context(), server.ID, server.Secret, sample, at); err != nil {
					t.Fatal(err)
				}
			}
			put(sample, received)
			put(sample, now)
			older := sample
			older.CollectedAt = sample.CollectedAt.Add(-time.Nanosecond)
			older.LastSuccessAt = new(older.CollectedAt)
			put(older, now)
			view, err := s.Virt(t.Context(), server.ID)
			if err != nil || view.Snapshot == nil || !view.ReceivedAt.Equal(received) || !view.Stale {
				t.Fatalf("duplicate/older sample refreshed receipt or lost stale snapshot: %+v %v", view, err)
			}
			if cache != nil {
				if ttl := cache.TTL(virtKey(server.ID)); ttl <= 0 || ttl > time.Minute {
					t.Fatalf("duplicate renewed TTL: %s", ttl)
				}
				if len(s.mem.virt) != 0 {
					t.Fatal("Redis mode retained a second memory snapshot")
				}
			}
			// Adjacent nanosecond timestamps must stay ordered in the Redis script.
			sample.CollectedAt = sample.CollectedAt.Add(time.Nanosecond)
			sample.LastSuccessAt = new(sample.CollectedAt)
			put(sample, now)
			view, err = s.Virt(t.Context(), server.ID)
			if err != nil || view.Snapshot == nil || !view.Snapshot.CollectedAt.Equal(sample.CollectedAt) || !view.ReceivedAt.Equal(now) {
				t.Fatalf("newer sample was not stored: %+v %v", view, err)
			}
			if cache != nil {
				remaining := time.Until(now.Add(retention))
				if ttl := cache.TTL(virtKey(server.ID)); ttl < remaining-time.Second || ttl > retention {
					t.Fatalf("new sample TTL: %s, want remaining portion of ten minutes (%s)", ttl, remaining)
				}
			}
			restarted := newTestStore(nil, client)
			if err := restarted.SyncServerCache(t.Context(), server); err != nil {
				t.Fatal(err)
			}
			view, err = restarted.Virt(t.Context(), server.ID)
			if err != nil || (view.Snapshot != nil) != (client != nil) {
				t.Fatalf("restart cache contract: %+v %v", view, err)
			}

			// Expiry uses real receipt time; no ten-minute wait or test-only clock.
			server.ID = 2
			server.Secret = "virt-expiring-secret"
			if err := s.SyncServerCache(t.Context(), server); err != nil {
				t.Fatal(err)
			}
			received = time.Now().UTC().Add(-retention + time.Second)
			sample.CollectedAt = received
			sample.LastSuccessAt = new(received)
			put(sample, received)
			view, err = s.Virt(t.Context(), server.ID)
			if err != nil || view.Snapshot == nil {
				t.Fatalf("snapshot expired early: %+v %v", view, err)
			}
			timer := time.NewTimer(time.Until(received.Add(retention)) + 20*time.Millisecond)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-t.Context().Done():
				t.Fatal(t.Context().Err())
			}
			if cache != nil {
				// miniredis TTLs only advance when explicitly fast-forwarded.
				cache.FastForward(time.Second)
			}
			view, err = s.Virt(t.Context(), server.ID)
			if err != nil || view.Snapshot != nil || view.ReceivedAt != nil || !view.Stale || view.VMCPUThreshold != 80 {
				t.Fatalf("expired snapshot view: %+v %v", view, err)
			}
			put(sample, time.Now().UTC())
			view, err = s.Virt(t.Context(), server.ID)
			if err != nil || view.Snapshot != nil {
				t.Fatalf("old report resurrected expired data: %+v %v", view, err)
			}
			if cache != nil && cache.Exists(virtKey(server.ID)) {
				t.Fatal("expired Redis key retained")
			}
			if _, exists := s.mem.virt[server.ID]; exists {
				t.Fatal("expired memory snapshot retained after read")
			}
			s.removeServerState(t.Context(), 1, "virt-cache-secret")
			if cache != nil && cache.Exists(virtKey(1)) {
				t.Fatal("deleted node retained Redis key")
			}
		})
	}
}

func TestVirtRedisFailure(t *testing.T) {
	cache := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: cache.Addr(), ContextTimeoutEnabled: true, MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	s := newTestStore(nil, client)
	server := model.Server{ID: 1, Secret: "virt-failure-secret"}
	if err := s.SyncServerCache(t.Context(), server); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	sample := virt.Snapshot{Schema: 1, Provider: "pve", Host: "pve", CollectedAt: now, LastSuccessAt: &now, TTLSeconds: 90, Status: "ok", VMs: []virt.VM{}}
	if err := s.SaveVirt(t.Context(), 1, server.Secret, sample, now); err != nil {
		t.Fatal(err)
	}
	cache.SetError("ERR cache unavailable")
	if _, err := s.Virt(t.Context(), 1); err == nil {
		t.Fatal("cache error treated as empty snapshot")
	}
	sample.CollectedAt = sample.CollectedAt.Add(time.Nanosecond)
	sample.LastSuccessAt = new(sample.CollectedAt)
	if err := s.SaveVirt(t.Context(), 1, server.Secret, sample, now); err == nil {
		t.Fatal("cache write failure hidden")
	}
	if len(s.mem.virt) != 0 {
		t.Fatal("cache failure fell back to memory")
	}
	s.removeServerState(t.Context(), 1, server.Secret)
	if _, err := s.Virt(t.Context(), 1); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cleanup failure exposed deleted node: %v", err)
	}
	cache.SetError("")
	cache.FastForward(10 * time.Minute)
	if cache.Exists(virtKey(1)) {
		t.Fatal("failed cleanup left a key without expiry")
	}
}

func TestIntegrationVirtRedisDeletion(t *testing.T) {
	s, cache := newRedisIntegrationStore(t)
	server, err := s.CreateNode(t.Context(), "virt-delete-secret")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	sample := virt.Snapshot{Schema: 1, Provider: "pve", Host: "pve", CollectedAt: now, LastSuccessAt: &now, TTLSeconds: 90, Status: "ok", VMs: []virt.VM{}}
	if err := s.SaveVirt(t.Context(), server.ID, server.Secret, sample, now); err != nil {
		t.Fatal(err)
	}
	rotated := "virt-rotated-secret"
	if err := s.UpdateNode(t.Context(), server.ID, NodeUpdate{Secret: &rotated}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveVirt(t.Context(), server.ID, server.Secret, sample, now); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("old secret accepted: %v", err)
	}
	cache.SetError("ERR cache unavailable")
	if err := s.DeleteNode(t.Context(), server.ID); !errors.Is(err, ErrFrontCacheUpdate) {
		t.Fatalf("delete did not roll back: %v", err)
	}
	cache.SetError("")
	view, err := s.Virt(t.Context(), server.ID)
	if err != nil || view.Snapshot == nil {
		t.Fatalf("rotation/rollback lost snapshot: %+v %v", view, err)
	}
	if err := s.DeleteNode(t.Context(), server.ID); err != nil {
		t.Fatal(err)
	}
	if cache.Exists(virtKey(server.ID)) {
		t.Fatal("committed deletion retained VM cache")
	}
	if _, err := s.Virt(t.Context(), server.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted node readable: %v", err)
	}
}
