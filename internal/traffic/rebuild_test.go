package traffic

import (
	"context"
	"errors"
	"testing"
	"time"

	trafficstore "dash/internal/store/traffic"
)

type blockingRebuildStore struct {
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

func rebuildTestNow() time.Time {
	return time.Date(2026, time.April, 1, 1, 0, 0, 0, time.UTC)
}

func (s *blockingRebuildStore) ServerTrafficSource(context.Context, int64, time.Time) (trafficstore.ServerTrafficSource, error) {
	start := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	return trafficstore.ServerTrafficSource{
		Start:  start,
		End:    start.Add(5 * time.Minute),
		Ifaces: []string{"eth0"},
	}, nil
}

func (s *blockingRebuildStore) DeleteTrafficMonthlySnapshots(context.Context, int64, time.Time, time.Time) error {
	return nil
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
	source      trafficstore.ServerTrafficSource
	sourceFrom  time.Time
	deleteStart time.Time
	chunkStart  time.Time
	chunkEnd    time.Time
}

func (s *recordingRebuildStore) ServerTrafficSource(_ context.Context, _ int64, start time.Time) (trafficstore.ServerTrafficSource, error) {
	s.sourceFrom = start
	return s.source, nil
}

func (s *recordingRebuildStore) DeleteTrafficMonthlySnapshots(_ context.Context, _ int64, start, _ time.Time) error {
	s.deleteStart = start
	return nil
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
}

func newBlockingWriteGate() *blockingWriteGate {
	return &blockingWriteGate{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (g *blockingWriteGate) with(ctx context.Context, fn func(context.Context) error) error {
	close(g.entered)
	select {
	case <-g.release:
		return fn(ctx)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestRebuildRunnerRejectsConcurrentStart(t *testing.T) {
	store := newBlockingRebuildStore()
	runner := newRebuildRunner(context.Background(), store, nil, 45*24*time.Hour)
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

	state, err = runner.Start(7)
	if !errors.Is(err, ErrRebuildRunning) {
		t.Fatalf("Start(7) again error = %v, want %v", err, ErrRebuildRunning)
	}
	if !state.Running || state.ServerID != 7 {
		t.Fatalf("Start(7) again state = %#v, want current node 7", state)
	}

	close(store.release)
	state = waitRebuildStatus(t, runner, 7, RebuildCompleted)
	if state.Running || state.FinishedAt == nil || state.Error != "" {
		t.Fatalf("completed state = %#v", state)
	}
	if current := runner.Current(); current.Status != RebuildCompleted || current.Running || current.ServerID != 7 {
		t.Fatalf("Current() = %#v, want last completed state", current)
	}
}

func TestRebuildRunnerWaitsForTrafficWrite(t *testing.T) {
	store := newBlockingRebuildStore()
	gate := newBlockingWriteGate()
	runner := newRebuildRunner(context.Background(), store, gate, 45*24*time.Hour)
	runner.now = rebuildTestNow
	defer runner.Stop()

	state, err := runner.Start(7)
	if err != nil || !state.Running {
		t.Fatalf("Start(7) = %#v/%v, want running", state, err)
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
	runner := newRebuildRunner(context.Background(), store, nil, 45*24*time.Hour)
	runner.now = rebuildTestNow

	state, err := runner.Start(7)
	if err != nil || !state.Running {
		t.Fatalf("Start(7) = %#v/%v, want running", state, err)
	}
	if started := waitRebuildStarted(t, store.started); started != 7 {
		t.Fatalf("started server = %d, want 7", started)
	}

	runner.Stop()

	state = runner.Current()
	if state.Status != RebuildFailed || state.Running || state.FinishedAt == nil {
		t.Fatalf("Current() after Stop() = %#v, want failed finished state", state)
	}
	if state.Code != "traffic_rebuild_canceled" || state.Error != "traffic rebuild canceled" {
		t.Fatalf("Current() after Stop() failure detail = %q/%q", state.Code, state.Error)
	}
}

func TestRebuildRunnerRejectsStartAfterStop(t *testing.T) {
	store := newBlockingRebuildStore()
	runner := newRebuildRunner(context.Background(), store, nil, 45*24*time.Hour)

	runner.Stop()

	state, err := runner.Start(7)
	if !errors.Is(err, ErrRebuildStopped) {
		t.Fatalf("Start() after Stop() error = %v, want %v", err, ErrRebuildStopped)
	}
	if state.Status != RebuildIdle || state.Running {
		t.Fatalf("Start() after Stop() state = %#v, want idle", state)
	}
	select {
	case started := <-store.started:
		t.Fatalf("rebuild started after Stop(): %d", started)
	default:
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
	runner := newRebuildRunner(context.Background(), store, nil, 24*time.Hour)
	runner.now = func() time.Time { return now }

	if err := runner.rebuild(context.Background(), 7); err != nil {
		t.Fatalf("rebuild() error = %v", err)
	}
	if want := floor.Add(-5 * time.Minute); !store.sourceFrom.Equal(want) {
		t.Fatalf("source start = %s, want %s", store.sourceFrom, want)
	}
	if !store.deleteStart.Equal(floor) {
		t.Fatalf("delete start = %s, want %s", store.deleteStart, floor)
	}
	if !store.chunkStart.Equal(floor) {
		t.Fatalf("chunk start = %s, want %s", store.chunkStart, floor)
	}
	if !store.chunkEnd.Equal(floor.Add(2 * time.Hour)) {
		t.Fatalf("chunk end = %s, want %s", store.chunkEnd, floor.Add(2*time.Hour))
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
