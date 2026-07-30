package traffic

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	trafficstore "dash/internal/store/traffic"
)

type rebuildModeStore struct {
	mode trafficstore.UsageMode
}

func (s *rebuildModeStore) GetSettings(context.Context) (trafficstore.Settings, error) {
	mode := s.mode
	if mode == "" {
		mode = trafficstore.UsageBilling
	}
	return trafficstore.Settings{UsageMode: mode}, nil
}

type blockingRebuildStore struct {
	rebuildModeStore
	started chan int64
	release chan struct{}
	err     error
}

func newBlockingRebuildStore() *blockingRebuildStore {
	return &blockingRebuildStore{
		started: make(chan int64, 1),
		release: make(chan struct{}),
	}
}

func (s *blockingRebuildStore) ServerTrafficSource(context.Context, int64, time.Time) (trafficstore.ServerTrafficSource, error) {
	start := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	return trafficstore.ServerTrafficSource{
		Start:  start,
		End:    start.Add(5 * time.Minute),
		Ifaces: []string{"eth0"},
	}, nil
}

func (s *blockingRebuildStore) RebuildTraffic5mChunk(ctx context.Context, serverID int64, _ []string, _, _ time.Time) error {
	s.started <- serverID
	select {
	case <-s.release:
		return s.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type recordingRebuildStore struct {
	rebuildModeStore
	source     trafficstore.ServerTrafficSource
	sourceFrom time.Time
	chunkStart time.Time
	chunkEnd   time.Time
}

func (s *recordingRebuildStore) ServerTrafficSource(_ context.Context, _ int64, start time.Time) (trafficstore.ServerTrafficSource, error) {
	s.sourceFrom = start
	return s.source, nil
}

func (s *recordingRebuildStore) RebuildTraffic5mChunk(_ context.Context, _ int64, _ []string, start, end time.Time) error {
	if s.chunkStart.IsZero() {
		s.chunkStart = start
	}
	s.chunkEnd = end
	return nil
}

type blockingWriteGate struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingWriteGate() *blockingWriteGate {
	return &blockingWriteGate{
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
}

func (g *blockingWriteGate) with(ctx context.Context, fn func(context.Context) error) error {
	g.once.Do(func() { g.entered <- struct{}{} })
	select {
	case <-g.release:
		return fn(ctx)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func rebuildTestNow() time.Time {
	return time.Date(2026, time.April, 1, 1, 0, 0, 0, time.UTC)
}

func TestRebuildRunnerRejectsConcurrentStart(t *testing.T) {
	store := newBlockingRebuildStore()
	runner := newRebuildRunner(context.Background(), store, newWriteGate(), 45*24*time.Hour)
	runner.now = rebuildTestNow
	defer runner.Stop()

	state, err := runner.Start(7)
	if err != nil || !state.Running || state.Status != RebuildRunning || state.ServerID != 7 {
		t.Fatalf("Start(7) = %#v/%v, want running", state, err)
	}
	if started := waitRebuildStarted(t, store.started); started != 7 {
		t.Fatalf("started server = %d, want 7", started)
	}

	state, err = runner.Start(8)
	if !errors.Is(err, ErrRebuildRunning) {
		t.Fatalf("Start(8) error = %v, want %v", err, ErrRebuildRunning)
	}
	if !state.Running || state.ServerID != 7 {
		t.Fatalf("Start(8) state = %#v, want current node 7", state)
	}

	close(store.release)
	state = waitRebuildStatus(t, runner, 7, RebuildCompleted)
	if state.Running || state.FinishedAt == nil || state.Error != "" {
		t.Fatalf("completed state = %#v", state)
	}
}

func TestRebuildRunnerRequiresBilling(t *testing.T) {
	store := newBlockingRebuildStore()
	store.mode = trafficstore.UsageLite
	runner := newRebuildRunner(context.Background(), store, newWriteGate(), 45*24*time.Hour)
	defer runner.Stop()

	state, err := runner.Start(7)
	if !errors.Is(err, ErrRebuildRequiresBilling) {
		t.Fatalf("Start() error = %v, want %v", err, ErrRebuildRequiresBilling)
	}
	if state.Status != RebuildIdle || state.Running {
		t.Fatalf("Start() state = %#v, want idle", state)
	}
}

func TestRebuildRunnerUsesTrafficWriteGate(t *testing.T) {
	store := newBlockingRebuildStore()
	gate := newBlockingWriteGate()
	runner := newRebuildRunner(context.Background(), store, gate, 45*24*time.Hour)
	runner.now = rebuildTestNow
	defer runner.Stop()

	if _, err := runner.Start(7); err != nil {
		t.Fatalf("Start(7) error = %v", err)
	}
	select {
	case <-gate.entered:
	case <-time.After(time.Second):
		t.Fatal("rebuild did not enter write gate")
	}
	select {
	case started := <-store.started:
		t.Fatalf("rebuild started while gate was held: %d", started)
	default:
	}

	close(gate.release)
	if started := waitRebuildStarted(t, store.started); started != 7 {
		t.Fatalf("started server = %d, want 7", started)
	}
	close(store.release)
	waitRebuildStatus(t, runner, 7, RebuildCompleted)
}

func TestRebuildRunnerStopCancelsTask(t *testing.T) {
	store := newBlockingRebuildStore()
	runner := newRebuildRunner(context.Background(), store, newWriteGate(), 45*24*time.Hour)
	runner.now = rebuildTestNow

	if _, err := runner.Start(7); err != nil {
		t.Fatalf("Start(7) error = %v", err)
	}
	if started := waitRebuildStarted(t, store.started); started != 7 {
		t.Fatalf("started server = %d, want 7", started)
	}

	runner.Stop()
	state := runner.Current()
	if state.Status != RebuildFailed || state.Running || state.FinishedAt == nil {
		t.Fatalf("Current() after Stop() = %#v, want failed finished state", state)
	}
	if state.Code != "traffic_rebuild_canceled" {
		t.Fatalf("Current() after Stop() code = %q", state.Code)
	}
}

func TestRebuildRunnerClampsToTrafficRetention(t *testing.T) {
	now := time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)
	floor := now.Add(-24 * time.Hour)
	store := &recordingRebuildStore{
		source: trafficstore.ServerTrafficSource{
			Start:  floor.Add(-24 * time.Hour),
			End:    floor.Add(2 * time.Hour),
			Ifaces: []string{"eth0"},
		},
	}
	runner := newRebuildRunner(context.Background(), store, newWriteGate(), 24*time.Hour)
	runner.now = func() time.Time { return now }

	if err := runner.rebuild(context.Background(), 7); err != nil {
		t.Fatalf("rebuild() error = %v", err)
	}
	if want := floor.Add(-5 * time.Minute); !store.sourceFrom.Equal(want) {
		t.Fatalf("source start = %s, want %s", store.sourceFrom, want)
	}
	if !store.chunkStart.Equal(floor) || !store.chunkEnd.Equal(floor.Add(2*time.Hour)) {
		t.Fatalf("chunk range = %s..%s", store.chunkStart, store.chunkEnd)
	}
}

func waitRebuildStarted(t *testing.T, started <-chan int64) int64 {
	t.Helper()
	select {
	case serverID := <-started:
		return serverID
	case <-time.After(time.Second):
		t.Fatal("rebuild did not start")
		return 0
	}
}

func waitRebuildStatus(t *testing.T, runner *RebuildRunner, serverID int64, want RebuildStatus) RebuildState {
	t.Helper()
	deadline := time.After(time.Second)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			state := runner.Current()
			t.Fatalf("Current() = %#v, want %d/%s", state, serverID, want)
		case <-tick.C:
			state := runner.Current()
			if state.ServerID == serverID && state.Status == want {
				return state
			}
		}
	}
}
