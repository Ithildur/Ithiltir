package traffic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"dash/internal/infra"
	trafficstore "dash/internal/store/traffic"
)

type RebuildStatus string

const (
	rebuildChunkSize    = 6 * time.Hour
	rebuildChunkTimeout = 30 * time.Second
)

const (
	RebuildIdle      RebuildStatus = "idle"
	RebuildRunning   RebuildStatus = "running"
	RebuildCompleted RebuildStatus = "completed"
	RebuildFailed    RebuildStatus = "failed"
)

var (
	ErrRebuildRunning = errors.New("traffic rebuild is running")
	ErrRebuildStopped = errors.New("traffic rebuild runner is stopped")
)

type RebuildState struct {
	ServerID   int64         `json:"server_id"`
	Status     RebuildStatus `json:"status"`
	Running    bool          `json:"running"`
	Code       string        `json:"code,omitempty"`
	StartedAt  *time.Time    `json:"started_at,omitempty"`
	FinishedAt *time.Time    `json:"finished_at,omitempty"`
	Error      string        `json:"error,omitempty"`
}

type rebuildStore interface {
	ServerTrafficSource(context.Context, int64, time.Time) (trafficstore.ServerTrafficSource, error)
	DeleteTrafficMonthlySnapshots(context.Context, int64, time.Time, time.Time) error
	RebuildTraffic5mChunk(context.Context, int64, []string, time.Time, time.Time) error
}

type trafficWriteGate interface {
	with(context.Context, func(context.Context) error) error
}

type RebuildRunner struct {
	store   rebuildStore
	gate    trafficWriteGate
	ctx     context.Context
	stop    context.CancelFunc
	mu      sync.Mutex
	wg      sync.WaitGroup
	state   RebuildState
	stopped bool
	retain  time.Duration
	now     func() time.Time
}

func newRebuildRunner(ctx context.Context, store rebuildStore, gate trafficWriteGate, retain time.Duration) *RebuildRunner {
	if ctx == nil {
		ctx = context.Background()
	}
	if gate == nil {
		gate = newWriteGate()
	}
	if retain <= 0 {
		retain = trafficRetention(0)
	}
	runCtx, stop := context.WithCancel(ctx)
	return &RebuildRunner{store: store, gate: gate, ctx: runCtx, stop: stop, retain: retain, now: time.Now}
}

func (r *RebuildRunner) Current() RebuildState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentLocked()
}

func (r *RebuildRunner) currentLocked() RebuildState {
	state := r.state
	if state.Status == "" {
		state.Status = RebuildIdle
	}
	state.Running = state.Status == RebuildRunning
	return state.clone()
}

func (r *RebuildRunner) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	r.stopped = true
	stop := r.stop
	r.mu.Unlock()

	if stop != nil {
		stop()
	}
	r.wg.Wait()
}

func (r *RebuildRunner) Start(serverID int64) (RebuildState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stopped {
		return r.currentLocked(), ErrRebuildStopped
	}
	if r.state.Status == RebuildRunning {
		return r.currentLocked(), ErrRebuildRunning
	}

	startedAt := time.Now().UTC()
	state := RebuildState{
		ServerID:  serverID,
		Status:    RebuildRunning,
		Running:   true,
		StartedAt: &startedAt,
	}
	r.state = state
	r.wg.Add(1)

	go r.run(serverID)
	return state.clone(), nil
}

func (r *RebuildRunner) run(serverID int64) {
	defer r.wg.Done()

	var err error
	if r.store == nil {
		err = fmt.Errorf("traffic store is nil")
	} else {
		err = r.gate.with(r.ctx, func(c context.Context) error {
			return r.rebuild(c, serverID)
		})
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		infra.WithModule("traffic").Warn("traffic rebuild failed", err, slog.Int64("server_id", serverID))
	}
	r.finish(serverID, err)
}

func (r *RebuildRunner) rebuild(ctx context.Context, serverID int64) error {
	floor := r.now().UTC().Add(-r.retain).Truncate(5 * time.Minute)
	sourceFrom := floor.Add(-5 * time.Minute)
	source, err := infra.WithPGReadTimeout(ctx, func(c context.Context) (trafficstore.ServerTrafficSource, error) {
		return r.store.ServerTrafficSource(c, serverID, sourceFrom)
	})
	if err != nil {
		return fmt.Errorf("load server traffic source %d: %w", serverID, err)
	}
	if source.Start.Before(floor) {
		source.Start = floor
	}
	if !source.End.After(source.Start) {
		return nil
	}

	// Invalidate monthly snapshots once rebuild starts; old snapshots are no longer trusted.
	if _, err := infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
		return struct{}{}, r.store.DeleteTrafficMonthlySnapshots(c, serverID, source.Start, source.End)
	}); err != nil {
		return fmt.Errorf("delete server traffic monthly snapshots %d %s..%s: %w", serverID, source.Start, source.End, err)
	}

	for cursor := source.Start; cursor.Before(source.End); {
		next := cursor.Add(rebuildChunkSize)
		if next.After(source.End) {
			next = source.End
		}
		chunkCtx, cancel := context.WithTimeout(ctx, rebuildChunkTimeout)
		err := r.store.RebuildTraffic5mChunk(chunkCtx, serverID, source.Ifaces, cursor, next)
		cancel()
		if err != nil {
			return fmt.Errorf("rebuild server traffic 5m %d %s..%s: %w", serverID, cursor, next, err)
		}
		cursor = next
	}
	return nil
}

func (r *RebuildRunner) finish(serverID int64, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state.Status != RebuildRunning || r.state.ServerID != serverID {
		return
	}

	state := r.state
	finishedAt := time.Now().UTC()
	state.Running = false
	state.FinishedAt = &finishedAt
	if err != nil {
		code, msg := rebuildFailure(err)
		state.Status = RebuildFailed
		state.Code = code
		state.Error = msg
	} else {
		state.Status = RebuildCompleted
		state.Code = ""
		state.Error = ""
	}
	r.state = state
}

func rebuildFailure(err error) (string, string) {
	switch {
	case errors.Is(err, context.Canceled):
		return "traffic_rebuild_canceled", "traffic rebuild canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "traffic_rebuild_timeout", "traffic rebuild timed out"
	default:
		return "traffic_rebuild_failed", "traffic rebuild failed"
	}
}

func (s RebuildState) clone() RebuildState {
	out := s
	if s.StartedAt != nil {
		startedAt := *s.StartedAt
		out.StartedAt = &startedAt
	}
	if s.FinishedAt != nil {
		finishedAt := *s.FinishedAt
		out.FinishedAt = &finishedAt
	}
	return out
}
