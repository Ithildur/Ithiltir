//go:build linux

package dashupdate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dash/internal/notify"
)

type enqueueReply struct {
	status notify.EnqueueStatus
	err    error
}

type notificationQueueStub struct {
	replies []enqueueReply
	calls   int
}

func (q *notificationQueueStub) EnqueueDefault(
	context.Context,
	string,
	notify.Messages,
) (notify.EnqueueStatus, error) {
	reply := q.replies[min(q.calls, len(q.replies)-1)]
	q.calls++
	return reply.status, reply.err
}

func TestFinishedAutoJobTreatsTerminalEnqueueStatusesAsHandled(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		status notify.EnqueueStatus
		err    error
	}{
		{name: "queued", id: "queued", status: notify.EnqueueQueued},
		{
			name:   "queued with target refresh warning",
			id:     "queued-warning",
			status: notify.EnqueueQueued,
			err:    errors.New("refresh notification targets"),
		},
		{name: "no targets", id: "no-targets", status: notify.EnqueueSkippedNoTargets},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := filepath.Join(t.TempDir(), "Ithiltir-dash")
			t.Setenv("DASH_HOME", home)
			paths, task := writeFinishedAutoJob(t, home, "handled-"+tt.id)
			if _, err := paths.switchCurrent(filepath.Base(task.dir)); err != nil {
				t.Fatalf("switchCurrent() error = %v", err)
			}
			runner := NewRunner()
			runner.availability = availabilityCache{
				paths:     paths,
				expiresAt: time.Now().Add(time.Minute),
			}
			queue := &notificationQueueStub{replies: []enqueueReply{{status: tt.status, err: tt.err}}}
			service := NewService(nil, queue, runner, "en")

			service.notifyFinishedAutoUpdate(context.Background())
			service.notifyFinishedAutoUpdate(context.Background())

			if queue.calls != 1 {
				t.Fatalf("notification enqueue calls = %d, want 1", queue.calls)
			}
			if _, err := os.Stat(filepath.Join(task.dir, updateHandledName)); err != nil {
				t.Fatalf("handled marker is missing: %v", err)
			}
			if _, err := os.Stat(filepath.Join(task.dir, updateNotifiedName)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("legacy notification marker was written: %v", err)
			}
		})
	}
}

func TestFinishedAutoJobRetriesEnqueueError(t *testing.T) {
	home := filepath.Join(t.TempDir(), "Ithiltir-dash")
	t.Setenv("DASH_HOME", home)
	paths, task := writeFinishedAutoJob(t, home, "retry-enqueue")
	if _, err := paths.switchCurrent(filepath.Base(task.dir)); err != nil {
		t.Fatalf("switchCurrent() error = %v", err)
	}
	runner := NewRunner()
	runner.availability = availabilityCache{
		paths:     paths,
		expiresAt: time.Now().Add(time.Minute),
	}
	queue := &notificationQueueStub{replies: []enqueueReply{
		{err: errors.New("temporary database failure")},
		{status: notify.EnqueueSkippedNoTargets},
	}}
	service := NewService(nil, queue, runner, "en")

	service.notifyFinishedAutoUpdate(context.Background())
	if _, err := os.Stat(filepath.Join(task.dir, updateHandledName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed enqueue wrote handled marker: %v", err)
	}
	service.notifyFinishedAutoUpdate(context.Background())
	service.notifyFinishedAutoUpdate(context.Background())

	if queue.calls != 2 {
		t.Fatalf("notification enqueue calls = %d, want 2", queue.calls)
	}
	if _, err := os.Stat(filepath.Join(task.dir, updateHandledName)); err != nil {
		t.Fatalf("handled marker is missing after retry: %v", err)
	}
}

func TestLegacyNotifiedMarkerRemainsHandled(t *testing.T) {
	home := filepath.Join(t.TempDir(), "Ithiltir-dash")
	paths, task := writeFinishedAutoJob(t, home, "legacy-notified")
	if err := os.WriteFile(filepath.Join(task.dir, updateNotifiedName), nil, 0o600); err != nil {
		t.Fatalf("write legacy marker: %v", err)
	}

	jobs, err := pendingFinishedAutoJobs(paths)
	if err != nil {
		t.Fatalf("pendingFinishedAutoJobs() error = %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("legacy-notified jobs = %d, want 0", len(jobs))
	}
	if err := cleanupUpdateJobs(paths, "current-job"); err != nil {
		t.Fatalf("cleanupUpdateJobs() error = %v", err)
	}
	if _, err := os.Stat(task.dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy-notified job still exists: %v", err)
	}
}

func TestLegacyAutoStateTreatsNoTargetsAsHandled(t *testing.T) {
	home := filepath.Join(t.TempDir(), "Ithiltir-dash")
	paths, task := writeFinishedAutoJob(t, home, "legacy-auto-state")
	state := autoState{LastStartedID: filepath.Base(task.dir)}
	autoStatePath := filepath.Join(paths.stateDir, updateAutoStateName)
	if err := writeAutoStateFile(autoStatePath, state); err != nil {
		t.Fatalf("writeAutoStateFile() error = %v", err)
	}
	queue := &notificationQueueStub{replies: []enqueueReply{{status: notify.EnqueueSkippedNoTargets}}}
	service := NewService(nil, queue, nil, "en")

	service.notifyLegacyFinishedAutoUpdate(context.Background(), paths, State{
		ID:            state.LastStartedID,
		Status:        StatusCompleted,
		Action:        ActionUpdate,
		Channel:       ChannelRelease,
		TargetVersion: "1.1.0",
		Origin:        OriginAuto,
	})

	got, err := readAutoStateFile(autoStatePath)
	if err != nil {
		t.Fatalf("readAutoStateFile() error = %v", err)
	}
	if got.LastFinishedID != state.LastStartedID {
		t.Fatalf("last finished ID = %q, want %q", got.LastFinishedID, state.LastStartedID)
	}
	if queue.calls != 1 {
		t.Fatalf("notification enqueue calls = %d, want 1", queue.calls)
	}
}

func writeFinishedAutoJob(t *testing.T, home, id string) (runnerPaths, taskPaths) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, "configs"), 0o700); err != nil {
		t.Fatalf("create Dash home marker: %v", err)
	}
	paths := runnerPathsForHome(home)
	task := paths.job(id)
	exitCode := 0
	now := time.Now().UTC()
	if err := writeUpdateStatusFile(task.statusPath, statusFile{
		ID:            id,
		Unit:          "ithiltir-dash-update-" + id,
		Status:        StatusCompleted,
		Action:        ActionUpdate,
		Channel:       ChannelRelease,
		Origin:        OriginAuto,
		TargetVersion: "1.1.0",
		Phase:         PhaseDone,
		StartedAt:     now.Add(-time.Minute).Format(time.RFC3339),
		FinishedAt:    now.Format(time.RFC3339),
		ExitCode:      &exitCode,
		LogFile:       task.logPath,
	}); err != nil {
		t.Fatalf("writeUpdateStatusFile() error = %v", err)
	}
	if err := os.WriteFile(task.logPath, nil, 0o600); err != nil {
		t.Fatalf("write update log: %v", err)
	}
	return paths, task
}
