package node

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"maps"
	"strconv"
	"time"

	"dash/internal/config"
	"dash/internal/infra/cachekeys"
	"dash/internal/virt"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const virtTTL = 10 * time.Minute

// Fixed-width UTC timestamps retain nanosecond ordering without Lua number rounding.
const virtTimeFormat = "2006-01-02T15:04:05.000000000Z"

var putVirtScript = redis.NewScript(`
local previous = redis.call('HGET', KEYS[1], 'collected_at')
if previous and previous >= ARGV[1] then return 0 end
redis.call('HSET', KEYS[1], 'collected_at', ARGV[1], 'received_at', ARGV[2], 'snapshot', ARGV[3])
redis.call('PEXPIREAT', KEYS[1], ARGV[4])
return 1
`)

// virtEntry owns one immutable encoded snapshot, without retaining a second
// decoded copy. Readers decode their own view outside the cache lock.
type virtEntry struct {
	collectedAt time.Time
	receivedAt  time.Time
	snapshot    []byte
}

// SaveVirt serializes authentication and replacement with node deletion and key
// rotation. A retry of the same sample never refreshes its received timestamp.
func (s *Store) SaveVirt(ctx context.Context, id int64, secret string, snapshot virt.Snapshot, receivedAt time.Time) error {
	return s.WithMetricsIngest(ctx, id, func() error {
		server, err := s.GetServerBySecret(ctx, secret)
		if err != nil {
			return err
		}
		if server.ID != id {
			return gorm.ErrRecordNotFound
		}
		// Reports already older than the retention window must not resurrect an
		// expired cache entry. A report can still be accepted without being cached.
		if !time.Now().Before(receivedAt.Add(virtTTL)) || !snapshot.CollectedAt.After(receivedAt.Add(-virtTTL)) {
			return nil
		}
		raw, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		if s.redis != nil {
			cacheCtx, cancel := context.WithTimeout(ctx, config.RedisWriteTimeout)
			defer cancel()
			return putVirtScript.Run(cacheCtx, s.redis, []string{virtKey(id)}, snapshot.CollectedAt.UTC().Format(virtTimeFormat), receivedAt.UTC().Format(time.RFC3339Nano), raw, receivedAt.Add(virtTTL).UnixMilli()).Err()
		}
		s.mem.virtMu.Lock()
		defer s.mem.virtMu.Unlock()
		if previous, exists := s.mem.virt[id]; exists && !snapshot.CollectedAt.After(previous.collectedAt) {
			return nil
		}
		now := time.Now()
		maps.DeleteFunc(s.mem.virt, func(_ int64, entry virtEntry) bool {
			return !now.Before(entry.receivedAt.Add(virtTTL))
		})
		s.mem.virt[id] = virtEntry{collectedAt: snapshot.CollectedAt, receivedAt: receivedAt, snapshot: raw}
		return nil
	})
}

func virtKey(id int64) string {
	return cachekeys.RedisKeyVirtSnapshotPrefix + strconv.FormatInt(id, 10)
}

func (s *Store) readVirt(ctx context.Context, id int64) (virtEntry, bool, error) {
	if s.redis == nil {
		s.mem.virtMu.Lock()
		defer s.mem.virtMu.Unlock()
		entry, exists := s.mem.virt[id]
		if exists && !time.Now().Before(entry.receivedAt.Add(virtTTL)) {
			delete(s.mem.virt, id)
			return virtEntry{}, false, nil
		}
		return entry, exists, nil
	}
	cacheCtx, cancel := context.WithTimeout(ctx, config.RedisFetchTimeout)
	defer cancel()
	fields, err := s.redis.HGetAll(cacheCtx, virtKey(id)).Result()
	if err != nil || len(fields) == 0 {
		return virtEntry{}, false, err
	}
	receivedAt, err := time.Parse(time.RFC3339Nano, fields["received_at"])
	if err != nil {
		return virtEntry{}, false, fmt.Errorf("VM cache receipt: %w", err)
	}
	collectedAt, err := time.Parse(virtTimeFormat, fields["collected_at"])
	if err != nil {
		return virtEntry{}, false, fmt.Errorf("VM cache collection time: %w", err)
	}
	if !time.Now().Before(receivedAt.Add(virtTTL)) {
		return virtEntry{}, false, nil
	}
	return virtEntry{collectedAt: collectedAt, receivedAt: receivedAt, snapshot: []byte(fields["snapshot"])}, true, nil
}

type VirtView struct {
	VMMemoryEnabled   bool           `json:"vm_memory_enabled"`
	VMCPUThreshold    int            `json:"vm_cpu_threshold"`
	VMMemoryThreshold int            `json:"vm_memory_threshold"`
	Snapshot          *virt.Snapshot `json:"snapshot"`
	ReceivedAt        *time.Time     `json:"received_at"`
	Stale             bool           `json:"stale"`
}

func (s *Store) Virt(ctx context.Context, id int64) (VirtView, error) {
	view := VirtView{Stale: true}
	if err := ctx.Err(); err != nil {
		return view, err
	}
	s.mem.authMu.RLock()
	entry, exists := s.mem.authByID[id]
	if !exists {
		s.mem.authMu.RUnlock()
		return view, gorm.ErrRecordNotFound
	}
	view.VMCPUThreshold = entry.meta.VMCPUThreshold
	view.VMMemoryEnabled = entry.meta.VMMemoryEnabled
	view.VMMemoryThreshold = entry.meta.VMMemoryThreshold
	s.mem.authMu.RUnlock()
	row, exists, err := s.readVirt(ctx, id)
	if err != nil {
		return view, err
	}
	// Deletion may commit while a Redis read is in flight.
	s.mem.authMu.RLock()
	_, active := s.mem.authByID[id]
	s.mem.authMu.RUnlock()
	if !active {
		return VirtView{}, gorm.ErrRecordNotFound
	}
	if !exists {
		return view, nil
	}
	var snapshot virt.Snapshot
	if err := json.Unmarshal(row.snapshot, &snapshot); err != nil {
		return view, err
	}
	view.Snapshot, view.ReceivedAt = &snapshot, &row.receivedAt
	now := time.Now()
	for i := range snapshot.VMs {
		snapshot.VMs[i].Expire(now)
	}
	ttl := time.Duration(snapshot.TTLSeconds) * time.Second
	view.Stale = snapshot.LastSuccessAt == nil || time.Since(*snapshot.LastSuccessAt) > ttl || time.Since(row.receivedAt) > ttl
	return view, nil
}
