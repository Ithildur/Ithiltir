package alert

import (
	"context"
	"testing"

	"dash/internal/metrics"
)

func TestDirtyQueueCoalescesServerWork(t *testing.T) {
	q := newDirtyQueue()
	q.mark(7, nil)
	q.mark(7, nil)

	if got := len(q.ready) - q.head; got != 1 {
		t.Fatalf("queued work = %d, want 1", got)
	}
	id, _, ok := q.next(t.Context())
	if !ok || id != 7 {
		t.Fatalf("next() = %d, %v, want 7, true", id, ok)
	}

	q.mark(7, nil)
	q.mark(7, nil)
	q.finish(7, false)
	if got := len(q.ready) - q.head; got != 1 {
		t.Fatalf("work queued while running = %d, want 1", got)
	}
	id, _, ok = q.next(t.Context())
	if !ok || id != 7 {
		t.Fatalf("next() after dirty-again = %d, %v, want 7, true", id, ok)
	}
	q.finish(7, false)
	if _, ok := q.states[7]; ok {
		t.Fatal("completed server remains in queue state")
	}
}

func TestDirtyQueueKeepsLatestMetricsSnapshot(t *testing.T) {
	q := newDirtyQueue()
	first := metrics.NodeView{CPU: metrics.CPU{UsageRatio: 0.1}}
	latest := metrics.NodeView{CPU: metrics.CPU{UsageRatio: 0.9}}
	q.mark(7, &first)
	q.mark(7, &latest)

	id, snapshot, ok := q.next(t.Context())
	if !ok || id != 7 || snapshot == nil {
		t.Fatalf("next() = %d, %+v, %v", id, snapshot, ok)
	}
	if snapshot.CPU.UsageRatio != latest.CPU.UsageRatio {
		t.Fatalf("snapshot CPU ratio = %v, want %v", snapshot.CPU.UsageRatio, latest.CPU.UsageRatio)
	}
}

func TestDirtyQueueRetriesAndStopsOnCancellation(t *testing.T) {
	q := newDirtyQueue()
	q.mark(9, nil)
	id, _, ok := q.next(t.Context())
	if !ok || id != 9 {
		t.Fatalf("next() = %d, %v, want 9, true", id, ok)
	}
	q.finish(9, true)
	id, _, ok = q.next(t.Context())
	if !ok || id != 9 {
		t.Fatalf("next() after retry = %d, %v, want 9, true", id, ok)
	}
	q.finish(9, false)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if id, _, ok := q.next(ctx); ok || id != 0 {
		t.Fatalf("next(canceled) = %d, %v, want 0, false", id, ok)
	}
}
