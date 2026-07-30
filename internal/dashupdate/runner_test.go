//go:build linux

package dashupdate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	appversion "dash/internal/version"
)

func TestStatusAvailabilityHonorsCanceledContext(t *testing.T) {
	prepareRunnerTestHome(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	state := NewRunner().Status(ctx)
	if state.Available {
		t.Fatal("Status() available = true for canceled request")
	}
	if !strings.Contains(state.UnavailableReason, context.Canceled.Error()) {
		t.Fatalf("Status() unavailable reason = %q, want context cancellation", state.UnavailableReason)
	}
}

func TestConcurrentStatusSharesAvailabilityCheck(t *testing.T) {
	_, bin := prepareRunnerTestHome(t)
	countPath := filepath.Join(t.TempDir(), "availability-count")
	writeTestCommand(t, filepath.Join(bin, "dash"), "printf x >> \"$DASHUPDATE_COUNT_FILE\"\nsleep 0.05\nprintf '%s\\n' '"+appversion.CurrentString()+"'\n")
	writeTestCommand(t, filepath.Join(bin, "systemctl"), "printf x >> \"$DASHUPDATE_COUNT_FILE\"\nsleep 0.05\nprintf '257\\n'\n")
	t.Setenv("DASHUPDATE_COUNT_FILE", countPath)

	runner := NewRunner()
	const callers = 12
	states := make(chan State, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			states <- runner.Status(context.Background())
		}()
	}
	wg.Wait()
	close(states)
	for state := range states {
		if !state.Available {
			t.Fatalf("Status() unavailable: %s", state.UnavailableReason)
		}
	}

	count, err := os.ReadFile(countPath)
	if err != nil {
		t.Fatalf("read availability count: %v", err)
	}
	if len(count) != 2 {
		t.Fatalf("availability command count = %d, want 2", len(count))
	}
}

func TestStatusUnavailableWhenSystemdManagerCannotBeReached(t *testing.T) {
	_, bin := prepareRunnerTestHome(t)
	writeTestCommand(t, filepath.Join(bin, "systemctl"), "printf '%s\\n' 'System has not been booted with systemd' >&2\nexit 1\n")

	state := NewRunner().Status(context.Background())
	if state.Available {
		t.Fatal("Status() available = true without a reachable systemd manager")
	}
	if !strings.Contains(state.UnavailableReason, "systemd manager is unavailable") {
		t.Fatalf("Status() unavailable reason = %q, want systemd manager error", state.UnavailableReason)
	}
}

func TestStatusDoesNotCacheRecoveryState(t *testing.T) {
	home, _ := prepareRunnerTestHome(t)
	runner := NewRunner()
	if state := runner.Status(context.Background()); !state.Available {
		t.Fatalf("initial Status() unavailable: %s", state.UnavailableReason)
	}

	paths := runnerPathsForHome(home)
	if err := writePrivateFile(paths.blockPath, []byte("interrupted-job\n")); err != nil {
		t.Fatalf("write update block: %v", err)
	}
	if state := runner.Status(context.Background()); state.Available || !strings.Contains(state.UnavailableReason, "recover") {
		t.Fatalf("blocked Status() = %+v, want immediate recovery requirement", state)
	}
	if _, err := removeDurable(paths.blockPath); err != nil {
		t.Fatalf("remove update block: %v", err)
	}
	if state := runner.Status(context.Background()); !state.Available {
		t.Fatalf("recovered Status() unavailable through stale cache: %s", state.UnavailableReason)
	}
}

func TestStartRecordsPinnedPlanWhenSystemdSubmissionIsRejected(t *testing.T) {
	home, bin := prepareRunnerTestHome(t)
	argsPath := filepath.Join(home, "systemd-run-args")
	writeTestCommand(t, filepath.Join(bin, "systemd-run"), "printf '%s\\n' \"$@\" > \"$DASHUPDATE_SYSTEMD_RUN_ARGS\"\nexit 1\n")
	writeTestCommand(t, filepath.Join(bin, "systemctl"), systemctlTestScript("not-found"))
	t.Setenv("DASHUPDATE_SYSTEMD_RUN_ARGS", argsPath)

	runner := NewRunner()
	input := pinnedRunInput(OriginAuto)
	state, err := runner.Start(context.Background(), input)
	if err == nil {
		t.Fatal("Start() error = nil, want rejected submission error")
	}
	if state.Status != StatusFailed || state.FailureCode != "submission_failed" {
		t.Fatalf("Start() state = %+v, want submission failure", state)
	}

	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read systemd-run args: %v", err)
	}
	argText := string(args)
	for _, want := range []string{"--property=Type=exec", "--property=UMask=0077", filepath.Join(home, "bin", "dash"), "update", "execute", "--job-id"} {
		if !strings.Contains(argText, want) {
			t.Fatalf("systemd-run args %q do not contain %q", argText, want)
		}
	}
	if strings.Contains(argText, "run.sh") || strings.Contains(argText, "update_dash_linux.sh") {
		t.Fatalf("systemd-run still delegates to a shell updater: %q", argText)
	}

	paths, err := runner.paths()
	if err != nil {
		t.Fatalf("paths() error = %v", err)
	}
	request, err := readJobRequest(paths.job(state.ID).requestPath)
	if err != nil {
		t.Fatalf("readJobRequest() error = %v", err)
	}
	if request.Plan.TargetVersion != input.TargetVersion || request.Plan.ExpectedCurrentVersion != input.ExpectedCurrentVersion || request.Plan.ExpectedInstallRevision != input.ExpectedInstallRevision {
		t.Fatalf("stored plan = %+v, want pinned input %+v", request.Plan, input)
	}
	jobs, err := pendingFinishedAutoJobs(paths)
	if err != nil {
		t.Fatalf("pendingFinishedAutoJobs() error = %v", err)
	}
	if len(jobs) != 1 || jobs[0].status.ID != state.ID {
		t.Fatalf("pending finished jobs = %+v, want auto job %q", jobs, state.ID)
	}

	firstID := state.ID
	second, secondErr := runner.Start(context.Background(), pinnedRunInput(OriginManual))
	if secondErr == nil {
		t.Fatal("second Start() error = nil, want rejected submission error")
	}
	if second.ID == "" || second.ID == firstID || second.Origin != OriginManual {
		t.Fatalf("second Start() state = %+v, want distinct manual job", second)
	}
	if err := cleanupUpdateJobs(paths, second.ID); err != nil {
		t.Fatalf("cleanupUpdateJobs(pending) error = %v", err)
	}
	if _, err := os.Stat(paths.job(firstID).dir); err != nil {
		t.Fatalf("unacknowledged auto job was removed: %v", err)
	}
	if err := markAutoJobHandled(jobs[0].handledPath); err != nil {
		t.Fatalf("markAutoJobHandled() error = %v", err)
	}
	if err := cleanupUpdateJobs(paths, second.ID); err != nil {
		t.Fatalf("cleanupUpdateJobs() error = %v", err)
	}
	if _, err := os.Stat(paths.job(firstID).dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("first acknowledged job still exists or cannot be inspected: %v", err)
	}
}

func TestStartDoesNotPublishWhileAnotherStarterHoldsLock(t *testing.T) {
	home, _ := prepareRunnerTestHome(t)
	paths := runnerPathsForHome(home)
	lock, err := acquireStartLock(paths.startLockPath)
	if err != nil {
		t.Fatalf("acquireStartLock() error = %v", err)
	}
	defer lock.Close()

	state, err := NewRunner().Start(context.Background(), pinnedRunInput(OriginManual))
	if !errors.Is(err, ErrRunning) {
		t.Fatalf("Start() error = %v, want ErrRunning", err)
	}
	if state.Status != StatusIdle {
		t.Fatalf("Start() state = %+v, want idle state", state)
	}
	if _, err := os.Lstat(paths.currentPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("current job was published while start lock was held: %v", err)
	}
	jobs, err := os.ReadDir(paths.jobsDir)
	if err != nil {
		t.Fatalf("ReadDir(jobs) error = %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("jobs created while start lock was held: %v", jobs)
	}
}

func TestManualJobDoesNotReplaceRunningCurrent(t *testing.T) {
	home, _ := prepareRunnerTestHome(t)
	paths := runnerPathsForHome(home)
	if err := os.MkdirAll(paths.jobsDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(jobs) error = %v", err)
	}
	existing := paths.job("existing-job")
	if err := os.Mkdir(existing.dir, 0o700); err != nil {
		t.Fatalf("Mkdir(existing job) error = %v", err)
	}
	if err := writePrivateFile(existing.logPath, nil); err != nil {
		t.Fatalf("write existing log: %v", err)
	}
	existingRequest := jobRequest{
		ID:   "existing-job",
		Unit: "ithiltir-dash-update-existing-job",
		Plan: pinnedPlan(OriginAuto),
	}
	if err := writeUpdateStatusFile(
		existing.statusPath,
		statusForRequest(existingRequest, existing.logPath, time.Now()),
	); err != nil {
		t.Fatalf("write existing status: %v", err)
	}
	committed, err := paths.switchCurrent(existingRequest.ID)
	if err != nil {
		t.Fatalf("switchCurrent(existing) error = %v", err)
	}
	if !committed {
		t.Fatal("switchCurrent(existing) committed = false")
	}

	install, err := newInstallPaths(home)
	if err != nil {
		t.Fatalf("newInstallPaths() error = %v", err)
	}
	err = runManualJob(
		context.Background(),
		install,
		jobRequest{Plan: pinnedPlan(OriginManual)},
		io.Discard,
	)
	if !errors.Is(err, ErrRunning) {
		t.Fatalf("runManualJob() error = %v, want ErrRunning", err)
	}
	target, err := os.Readlink(paths.currentPath)
	if err != nil {
		t.Fatalf("Readlink(current) error = %v", err)
	}
	if want := filepath.Join(updateJobsDirName, existingRequest.ID); target != want {
		t.Fatalf("current target = %q, want %q", target, want)
	}
}

func TestStartKeepsRunningWhenSubmittedUnitIsActivating(t *testing.T) {
	_, bin := prepareRunnerTestHome(t)
	writeTestCommand(t, filepath.Join(bin, "systemd-run"), "exit 1\n")
	writeTestCommand(t, filepath.Join(bin, "systemctl"), systemctlTestScript("activating"))

	state, err := NewRunner().Start(context.Background(), pinnedRunInput(OriginManual))
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if state.Status != StatusRunning {
		t.Fatalf("Start() status = %q, want running", state.Status)
	}
}

func TestStartKeepsRunningWhenSubmissionContextIsCanceled(t *testing.T) {
	home, bin := prepareRunnerTestHome(t)
	startedPath := filepath.Join(home, "systemd-run-started")
	writeTestCommand(t, filepath.Join(bin, "systemd-run"), ": > \"$DASHUPDATE_SYSTEMD_RUN_STARTED\"\nexec sleep 30\n")
	writeTestCommand(t, filepath.Join(bin, "systemctl"), systemctlTestScript("not-found"))
	t.Setenv("DASHUPDATE_SYSTEMD_RUN_STARTED", startedPath)

	runner := NewRunner()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type startResult struct {
		state State
		err   error
	}
	result := make(chan startResult, 1)
	go func() {
		state, err := runner.Start(ctx, pinnedRunInput(OriginManual))
		result <- startResult{state: state, err: err}
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(startedPath); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat systemd-run marker: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("systemd-run did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	got := <-result
	if got.err != nil {
		t.Fatalf("Start() error = %v", got.err)
	}
	if got.state.Status != StatusRunning {
		t.Fatalf("Start() status = %q, want running", got.state.Status)
	}
}

func TestJobReporterPreservesOriginAndCompletesPinnedPlan(t *testing.T) {
	dir := t.TempDir()
	task := taskPaths{
		dir:         dir,
		requestPath: filepath.Join(dir, updateRequestName),
		statusPath:  filepath.Join(dir, updateStatusName),
		logPath:     filepath.Join(dir, updateLogName),
	}
	request := jobRequest{
		ID:   "auto-job",
		Unit: "ithiltir-dash-update-auto-job",
		Plan: Plan{
			Action:                  ActionUpdate,
			Channel:                 ChannelRelease,
			TargetVersion:           "1.1.0",
			ExpectedCurrentVersion:  "1.0.0",
			ExpectedInstallRevision: strings.Repeat("a", 64),
			Origin:                  OriginAuto,
			Lang:                    "en",
			ServiceManager:          "auto",
		},
	}
	if err := writePrivateFile(task.logPath, nil); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if err := writeJobRequest(task.requestPath, request); err != nil {
		t.Fatalf("writeJobRequest() error = %v", err)
	}
	if err := writeUpdateStatusFile(task.statusPath, statusForRequest(request, task.logPath, time.Now())); err != nil {
		t.Fatalf("writeUpdateStatusFile() error = %v", err)
	}
	reporter, closeLog, err := openJobReporter(task, request, nil)
	if err != nil {
		t.Fatalf("openJobReporter() error = %v", err)
	}
	defer closeLog()
	if err := reporter.phase(PhaseLocked); err != nil {
		t.Fatalf("phase() error = %v", err)
	}
	if err := reporter.finish(nil); err != nil {
		t.Fatalf("finish() error = %v", err)
	}

	item, err := readUpdateStatusFile(task.statusPath)
	if err != nil {
		t.Fatalf("readUpdateStatusFile() error = %v", err)
	}
	if item.Status != StatusCompleted || item.Origin != OriginAuto || item.TargetVersion != "1.1.0" || item.Phase != PhaseDone {
		t.Fatalf("completed status = %+v", item)
	}
}

func TestReconcileFailsWhenSystemdUnitIsUnknown(t *testing.T) {
	bin := t.TempDir()
	writeTestCommand(t, filepath.Join(bin, "systemctl"), "printf 'LoadState=not-found\\nActiveState=inactive\\n'\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	item := staleRunningStatus("unknown-unit")
	paths := runnerPathsForHome(t.TempDir())
	task := paths.job(item.ID)
	if err := writeUpdateStatusFile(task.statusPath, item); err != nil {
		t.Fatalf("writeUpdateStatusFile() error = %v", err)
	}
	got, err := NewRunner().reconcileRunningStatus(context.Background(), item, task, paths)
	if err != nil {
		t.Fatalf("reconcileRunningStatus() error = %v", err)
	}
	if got.Status != StatusFailed || got.FailureCode != "executor_lost" {
		t.Fatalf("reconcileRunningStatus() = %+v, want executor_lost failure", got)
	}
}

func TestReconcileKeepsActivatingSystemdUnitRunning(t *testing.T) {
	bin := t.TempDir()
	writeTestCommand(t, filepath.Join(bin, "systemctl"), "printf 'LoadState=loaded\\nActiveState=activating\\n'\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	item := staleRunningStatus("activating-unit")
	paths := runnerPathsForHome(t.TempDir())
	task := paths.job(item.ID)
	if err := writeUpdateStatusFile(task.statusPath, item); err != nil {
		t.Fatalf("writeUpdateStatusFile() error = %v", err)
	}
	got, err := NewRunner().reconcileRunningStatus(context.Background(), item, task, paths)
	if err != nil {
		t.Fatalf("reconcileRunningStatus() error = %v", err)
	}
	if got.Status != StatusRunning {
		t.Fatalf("reconcileRunningStatus() status = %q, want running", got.Status)
	}
}

func TestReconcilePreservesExecutorTerminalStatus(t *testing.T) {
	bin := t.TempDir()
	writeTestCommand(t, filepath.Join(bin, "systemctl"), "printf 'LoadState=loaded\\nActiveState=inactive\\n'\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	stale := staleRunningStatus("finished-unit")
	finished := stale
	finished.Status = StatusCompleted
	finished.Phase = PhaseDone
	finished.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	code := 0
	finished.ExitCode = &code
	paths := runnerPathsForHome(t.TempDir())
	task := paths.job(stale.ID)
	if err := writeUpdateStatusFile(task.statusPath, finished); err != nil {
		t.Fatalf("writeUpdateStatusFile() error = %v", err)
	}
	got, err := NewRunner().reconcileRunningStatus(context.Background(), stale, task, paths)
	if err != nil {
		t.Fatalf("reconcileRunningStatus() error = %v", err)
	}
	if got.Status != StatusCompleted || got.Phase != PhaseDone {
		t.Fatalf("reconcileRunningStatus() = %+v, want executor terminal status", got)
	}
}

func TestReconcileFailurePreservesLatestRunningStatus(t *testing.T) {
	bin := t.TempDir()
	writeTestCommand(t, filepath.Join(bin, "systemctl"), "printf 'LoadState=loaded\\nActiveState=inactive\\n'\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	stale := staleRunningStatus("stopped-unit")
	latest := stale
	latest.Phase = PhaseCleanup
	latest.ExecutorPID = 42
	paths := runnerPathsForHome(t.TempDir())
	task := paths.job(stale.ID)
	if err := writeUpdateStatusFile(task.statusPath, latest); err != nil {
		t.Fatalf("writeUpdateStatusFile() error = %v", err)
	}

	got, err := NewRunner().reconcileRunningStatus(context.Background(), stale, task, paths)
	if err != nil {
		t.Fatalf("reconcileRunningStatus() error = %v", err)
	}
	if got.Status != StatusFailed || got.FailureCode != "executor_lost" {
		t.Fatalf("reconcileRunningStatus() = %+v, want executor_lost failure", got)
	}
	if got.Phase != latest.Phase || got.ExecutorPID != latest.ExecutorPID {
		t.Fatalf("reconcileRunningStatus() overwrote latest fields: got=%+v latest=%+v", got, latest)
	}
}

func TestReconcileMarksInterruptedTransactionForRecovery(t *testing.T) {
	bin := t.TempDir()
	writeTestCommand(t, filepath.Join(bin, "systemctl"), "printf 'LoadState=loaded\\nActiveState=failed\\n'\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	item := staleRunningStatus("recovery-unit")
	paths := runnerPathsForHome(t.TempDir())
	task := paths.job(item.ID)
	if err := writeUpdateStatusFile(task.statusPath, item); err != nil {
		t.Fatalf("writeUpdateStatusFile() error = %v", err)
	}
	recoveryRoot := filepath.Join(filepath.Dir(paths.home), ".ithiltir-dash-update-recovery")
	if err := writeTransaction(paths.transactionPath, updateTransaction{
		JobID:            item.ID,
		Phase:            PhaseMigrating,
		TempRoot:         recoveryRoot,
		MigrationStarted: true,
	}); err != nil {
		t.Fatalf("writeTransaction() error = %v", err)
	}

	got, err := NewRunner().reconcileRunningStatus(context.Background(), item, task, paths)
	if err != nil {
		t.Fatalf("reconcileRunningStatus() error = %v", err)
	}
	if got.Status != StatusFailed || got.FailureCode != "recovery_required" || got.RecoveryPath != recoveryRoot {
		t.Fatalf("reconcileRunningStatus() = %+v, want recovery-required failure", got)
	}
}

func prepareRunnerTestHome(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, "configs"), 0o700); err != nil {
		t.Fatalf("Mkdir(configs) error = %v", err)
	}
	bin := filepath.Join(home, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatalf("Mkdir(bin) error = %v", err)
	}
	writeTestCommand(t, filepath.Join(bin, "dash"), "printf '%s\\n' '"+appversion.CurrentString()+"'\n")
	writeTestCommand(t, filepath.Join(bin, "systemd-run"), "exit 0\n")
	writeTestCommand(t, filepath.Join(bin, "systemctl"), "printf '257\\n'\n")
	t.Setenv("DASH_HOME", home)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return home, bin
}

func writeTestCommand(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("write command %s: %v", path, err)
	}
}

func pinnedRunInput(origin Origin) RunInput {
	return RunInput{
		Action:                  ActionUpdate,
		Channel:                 ChannelRelease,
		Lang:                    "en",
		Origin:                  origin,
		TargetVersion:           "1.1.0",
		ExpectedCurrentVersion:  appversion.CurrentString(),
		ExpectedInstallRevision: strings.Repeat("a", 64),
	}
}

func pinnedPlan(origin Origin) Plan {
	input := pinnedRunInput(origin)
	return Plan{
		Action:                  input.Action,
		Channel:                 input.Channel,
		TargetVersion:           input.TargetVersion,
		ExpectedCurrentVersion:  input.ExpectedCurrentVersion,
		ExpectedInstallRevision: input.ExpectedInstallRevision,
		Origin:                  input.Origin,
		Lang:                    input.Lang,
		ServiceManager:          "auto",
	}
}

func TestPrepareRequiresReinstallForCurrentVersion(t *testing.T) {
	runner := NewRunner()
	input := RunInput{
		Action:                  ActionUpdate,
		Channel:                 ChannelRelease,
		Lang:                    "en",
		Origin:                  OriginManual,
		TargetVersion:           "1.0.0",
		ExpectedCurrentVersion:  "1.0.0",
		ExpectedInstallRevision: strings.Repeat("a", 64),
	}
	if _, err := runner.Prepare(context.Background(), input); !errors.Is(err, ErrAlreadyCurrent) {
		t.Fatalf("Prepare(update current) error = %v, want ErrAlreadyCurrent", err)
	}

	input.Action = ActionReinstall
	if _, err := runner.Prepare(context.Background(), input); err != nil {
		t.Fatalf("Prepare(reinstall current) error = %v", err)
	}
}

func TestDashUpdateCommandLineDoesNotMatchServerProcess(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "server", raw: "/opt/Ithiltir-dash/bin/dash\x00--debug\x00"},
		{name: "migrate", raw: "/opt/Ithiltir-dash/bin/dash\x00migrate\x00"},
		{name: "update", raw: "/opt/Ithiltir-dash/bin/dash\x00update\x00execute\x00--job-id\x00job\x00", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dashUpdateCommandLine([]byte(tt.raw)); got != tt.want {
				t.Fatalf("dashUpdateCommandLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReconcileDoneTransactionPersistsOutcomeBeforeCleanup(t *testing.T) {
	tests := []struct {
		name        string
		forward     bool
		wantStatus  Status
		wantPhase   Phase
		wantFailure string
		wantCode    int
	}{
		{name: "rollback", wantStatus: StatusFailed, wantPhase: PhaseSwitching, wantFailure: "rolled_back", wantCode: 1},
		{name: "forward", forward: true, wantStatus: StatusCompleted, wantPhase: PhaseDone, wantCode: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			paths := runnerPathsForHome(home)
			id := "done-" + tt.name
			task := paths.job(id)
			item := statusFile{
				ID:        id,
				Unit:      "manual-" + id,
				Status:    StatusRunning,
				Action:    ActionUpdate,
				Channel:   ChannelRelease,
				Phase:     PhaseSwitching,
				StartedAt: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
			}
			if err := writeUpdateStatusFile(task.statusPath, item); err != nil {
				t.Fatalf("writeUpdateStatusFile() error = %v", err)
			}
			txn := updateTransaction{
				FormatVersion:    1,
				JobID:            id,
				Phase:            PhaseDone,
				MigrationStarted: tt.forward,
			}
			if err := writeTransaction(paths.transactionPath, txn); err != nil {
				t.Fatalf("writeTransaction() error = %v", err)
			}

			got, err := reconcileDoneTransaction(item, task, paths)
			if err != nil {
				t.Fatalf("reconcileDoneTransaction() error = %v", err)
			}
			if got.Status != tt.wantStatus || got.Phase != tt.wantPhase || got.FailureCode != tt.wantFailure || got.ExitCode == nil || *got.ExitCode != tt.wantCode {
				t.Fatalf("reconciled status = %+v", got)
			}
			if _, err := os.Stat(paths.transactionPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("completed transaction still exists: %v", err)
			}
			persisted, err := readUpdateStatusFile(task.statusPath)
			if err != nil {
				t.Fatalf("readUpdateStatusFile() error = %v", err)
			}
			if persisted.Status != got.Status || persisted.Phase != got.Phase || persisted.FailureCode != got.FailureCode {
				t.Fatalf("persisted status = %+v, reconciled = %+v", persisted, got)
			}
		})
	}
}

func TestReconcileDoneTransactionKeepsEvidenceWhenStatusWriteFails(t *testing.T) {
	home := t.TempDir()
	paths := runnerPathsForHome(home)
	id := "done-write-failure"
	txn := updateTransaction{
		FormatVersion:    1,
		JobID:            id,
		Phase:            PhaseDone,
		MigrationStarted: true,
	}
	if err := writeTransaction(paths.transactionPath, txn); err != nil {
		t.Fatalf("writeTransaction() error = %v", err)
	}
	blocked := filepath.Join(home, "blocked")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatalf("write blocked parent: %v", err)
	}
	item := statusFile{ID: id, Status: StatusRunning, Phase: PhaseCleanup}
	_, err := reconcileDoneTransaction(item, taskPaths{statusPath: filepath.Join(blocked, "status.env")}, paths)
	if err == nil {
		t.Fatal("reconcileDoneTransaction() succeeded with an unwritable status path")
	}
	if _, err := os.Stat(paths.transactionPath); err != nil {
		t.Fatalf("completed transaction evidence was removed: %v", err)
	}
}

func systemctlTestScript(unitState string) string {
	unitResult := "printf 'LoadState=not-found\\nActiveState=inactive\\n'\n"
	if unitState == "activating" {
		unitResult = "printf 'LoadState=loaded\\nActiveState=activating\\n'\n"
	}
	return fmt.Sprintf("if [ \"$1\" = show ] && [ \"$2\" = --property=Version ]; then printf '257\\n'; exit 0; fi\n%s", unitResult)
}

func staleRunningStatus(id string) statusFile {
	return statusFile{
		ID:        id,
		Unit:      "ithiltir-dash-update-" + id,
		Status:    StatusRunning,
		Action:    ActionUpdate,
		Channel:   ChannelRelease,
		StartedAt: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
	}
}
