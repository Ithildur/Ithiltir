//go:build linux

package dashupdate

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateTransactionPathsBindsTargetToCandidate(t *testing.T) {
	home := filepath.Join(t.TempDir(), "Ithiltir-dash")
	paths, err := newInstallPaths(home)
	if err != nil {
		t.Fatalf("newInstallPaths() error = %v", err)
	}
	tempRoot := filepath.Join(filepath.Dir(home), ".ithiltir-dash-update-test")
	txn := updateTransaction{
		CandidateTarget: "releases/1.1.0-job",
		CandidateDir:    filepath.Join(home, "releases", "1.1.0-job"),
		TempRoot:        tempRoot,
		ServiceManager:  "none",
	}
	if err := validateTransactionPaths(paths, txn); err != nil {
		t.Fatalf("validateTransactionPaths() error = %v", err)
	}
	txn.CandidateTarget = "releases/other"
	if err := validateTransactionPaths(paths, txn); err == nil {
		t.Fatal("validateTransactionPaths() accepted mismatched target and candidate")
	}
}

func TestInstalledDashExecutableOnlyMatchesManagedBinary(t *testing.T) {
	home := filepath.Join(t.TempDir(), "Ithiltir-dash")
	paths, err := newInstallPaths(home)
	if err != nil {
		t.Fatalf("newInstallPaths() error = %v", err)
	}
	managed := filepath.Join(home, "releases", "1.0.0-job", "bin", "dash")
	if !installedDashExecutable(paths, managed) {
		t.Fatalf("installedDashExecutable(%q) = false", managed)
	}
	for _, path := range []string{
		filepath.Join(home, "releases", "1.0.0-job", "dash"),
		filepath.Join(home, "releases", "bin", "dash"),
		filepath.Join(home, "other", "bin", "dash"),
	} {
		if installedDashExecutable(paths, path) {
			t.Fatalf("installedDashExecutable(%q) = true", path)
		}
	}
}

func TestDashServerCommandLineExcludesMaintenanceCommands(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "server", raw: "/opt/Ithiltir-dash/bin/dash\x00", want: true},
		{name: "server flags", raw: "/opt/Ithiltir-dash/bin/dash\x00--debug\x00", want: true},
		{name: "migrate", raw: "/opt/Ithiltir-dash/bin/dash\x00migrate\x00"},
		{name: "update", raw: "/opt/Ithiltir-dash/bin/dash\x00update\x00execute\x00"},
		{name: "redis check", raw: "/opt/Ithiltir-dash/bin/dash\x00check-redis\x00"},
		{name: "theme pack", raw: "/opt/Ithiltir-dash/bin/dash\x00pack-theme\x00"},
		{name: "version", raw: "/opt/Ithiltir-dash/bin/dash\x00--version\x00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dashServerCommandLine([]byte(tt.raw)); got != tt.want {
				t.Fatalf("dashServerCommandLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseManualUpdateArgsKeepsShellCompatibility(t *testing.T) {
	opts, err := parseManualUpdateArgs([]string{"reinstall", "--check", "--test", "--lang=zh", "--service-manager", "none"})
	if err != nil {
		t.Fatalf("parseManualUpdateArgs() error = %v", err)
	}
	if opts.action != ActionReinstall || opts.channel != ChannelPrerelease || !opts.checkOnly || opts.lang != "zh" || opts.serviceManager != "none" {
		t.Fatalf("parseManualUpdateArgs() = %+v", opts)
	}
}

func TestSystemdConditionPathEscapesSpecifiers(t *testing.T) {
	got, err := systemdConditionPath(`/opt/Dash%i/update.block`)
	if err != nil {
		t.Fatalf("systemdConditionPath() error = %v", err)
	}
	if !strings.Contains(got, "%%i") || !strings.HasPrefix(got, "!") {
		t.Fatalf("systemdConditionPath() = %q", got)
	}
	if _, err := systemdConditionPath(`/opt/Dash with spaces/update.block`); err == nil {
		t.Fatal("systemdConditionPath() accepted a path with spaces")
	}
}

func TestWaitDashStableRequiresContinuousRunningState(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	writeTestCommand(t, filepath.Join(bin, "systemctl"), "printf 'ActiveState=active\\nSubState=running\\nNRestarts=0\\n'\n")
	if err := waitDashStable(context.Background(), 20*time.Millisecond, 5*time.Millisecond); err != nil {
		t.Fatalf("waitDashStable() error = %v", err)
	}
}

func TestWaitDashStableRejectsRestart(t *testing.T) {
	bin := t.TempDir()
	count := filepath.Join(t.TempDir(), "count")
	t.Setenv("PATH", bin)
	t.Setenv("DASHUPDATE_RESTART_COUNT", count)
	writeTestCommand(t, filepath.Join(bin, "systemctl"), "if [ -f \"$DASHUPDATE_RESTART_COUNT\" ]; then restarts=1; else : > \"$DASHUPDATE_RESTART_COUNT\"; restarts=0; fi\nprintf 'ActiveState=active\\nSubState=running\\nNRestarts=%s\\n' \"$restarts\"\n")
	err := waitDashStable(context.Background(), time.Second, 5*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "restarted during startup") {
		t.Fatalf("waitDashStable() error = %v, want restart failure", err)
	}
}

func TestRecordRecoveryResultReflectsFinalOutcome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "Ithiltir-dash")
	paths, err := newInstallPaths(home)
	if err != nil {
		t.Fatalf("newInstallPaths() error = %v", err)
	}
	for _, tt := range []struct {
		name        string
		forward     bool
		wantStatus  Status
		wantPhase   Phase
		wantFailure string
		wantCode    int
	}{
		{name: "rollback", wantStatus: StatusFailed, wantPhase: PhaseSwitching, wantFailure: "rolled_back", wantCode: 1},
		{name: "forward", forward: true, wantStatus: StatusCompleted, wantPhase: PhaseDone, wantCode: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			id := "recovery-" + tt.name
			task := runnerPathsForHome(home).job(id)
			item := statusFile{
				ID:           id,
				Unit:         "manual-" + id,
				Status:       StatusFailed,
				Action:       ActionUpdate,
				Channel:      ChannelRelease,
				Origin:       OriginManual,
				Phase:        PhaseSwitching,
				FailureCode:  "recovery_required",
				RecoveryPath: "/tmp/recovery",
				StartedAt:    time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
			}
			if err := writeUpdateStatusFile(task.statusPath, item); err != nil {
				t.Fatalf("writeUpdateStatusFile() error = %v", err)
			}
			txn := updateTransaction{
				JobID:            id,
				Phase:            PhaseDone,
				MigrationStarted: tt.forward,
			}
			if err := recordRecoveryResult(paths, txn); err != nil {
				t.Fatalf("recordRecoveryResult() error = %v", err)
			}
			got, err := readUpdateStatusFile(task.statusPath)
			if err != nil {
				t.Fatalf("readUpdateStatusFile() error = %v", err)
			}
			if got.Status != tt.wantStatus || got.Phase != tt.wantPhase || got.FailureCode != tt.wantFailure || got.RecoveryPath != "" || got.ExitCode == nil || *got.ExitCode != tt.wantCode {
				t.Fatalf("recovered status = %+v", got)
			}
		})
	}
}

func TestSwitchFailureRollsBackBeforeMigration(t *testing.T) {
	stopFailure := errors.New("stop failure")
	activateFailure := errors.New("activate failure")
	tests := []struct {
		name string
		ops  func() installOps
		want error
	}{
		{
			name: "stop",
			ops: func() installOps {
				ops := testInstallOps()
				ops.stop = func(context.Context, installPaths, string, stepLogger) error {
					return stopFailure
				}
				return ops
			},
			want: stopFailure,
		},
		{
			name: "activate",
			ops: func() installOps {
				ops := testInstallOps()
				ops.activate = func(installPaths, string, bool) error {
					return activateFailure
				}
				return ops
			},
			want: activateFailure,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths, txn := prepareInstallTransaction(t, "rollback-"+tt.name, PhasePrepared, false, false)
			if _, err := switchRelease(
				context.Background(),
				paths,
				txn,
				textReporter{w: io.Discard},
				tt.ops(),
			); !errors.Is(err, tt.want) {
				t.Fatalf("switchRelease() error = %v, want %v", err, tt.want)
			}

			persisted, err := readTransaction(paths.transactionFile)
			if err != nil {
				t.Fatalf("readTransaction() error = %v", err)
			}
			if persisted.Phase != PhaseDone || persisted.MigrationStarted {
				t.Fatalf("rolled back transaction = %+v", persisted)
			}
			for _, path := range []string{paths.blockFile, txn.CandidateDir, txn.TempRoot} {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("rolled back path %s still exists: %v", path, err)
				}
			}
			target, err := os.Readlink(paths.current)
			if err != nil {
				t.Fatalf("Readlink(current) error = %v", err)
			}
			if target != txn.PreviousTarget {
				t.Fatalf("current target = %q, want %q", target, txn.PreviousTarget)
			}
			if err := recoverUpdateLocked(context.Background(), paths, io.Discard, testInstallOps()); err != nil {
				t.Fatalf("recoverUpdateLocked() error = %v", err)
			}
			status, err := readUpdateStatusFile(runnerPathsForHome(paths.home).job(txn.JobID).statusPath)
			if err != nil {
				t.Fatalf("readUpdateStatusFile() error = %v", err)
			}
			if status.Status != StatusFailed || status.FailureCode != "rolled_back" {
				t.Fatalf("recovered status = %+v", status)
			}
		})
	}
}

func TestForwardFailureResumesThroughSharedCompletion(t *testing.T) {
	failure := errors.New("injected forward failure")
	tests := []struct {
		name      string
		active    bool
		wantPhase Phase
		inject    func(*installOps) func(*testing.T)
	}{
		{
			name:      "migration",
			wantPhase: PhaseMigrating,
			inject: func(ops *installOps) func(*testing.T) {
				calls := 0
				ops.migrate = func(context.Context, installPaths, updateTransaction, stepLogger) error {
					calls++
					if calls == 1 {
						return failure
					}
					return nil
				}
				return func(t *testing.T) {
					t.Helper()
					if calls != 2 {
						t.Fatalf("migration calls = %d, want 2", calls)
					}
				}
			},
		},
		{
			name:      "start",
			active:    true,
			wantPhase: PhaseStarting,
			inject: func(ops *installOps) func(*testing.T) {
				calls := 0
				ops.start = func(context.Context, string, stepLogger) error {
					calls++
					if calls == 1 {
						return failure
					}
					return nil
				}
				return func(t *testing.T) {
					t.Helper()
					if calls != 2 {
						t.Fatalf("start calls = %d, want 2", calls)
					}
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths, txn := prepareInstallTransaction(t, "forward-"+tt.name, PhaseMigrating, true, tt.active)
			ops := testInstallOps()
			verify := tt.inject(&ops)

			err := finishForward(context.Background(), paths, txn, textReporter{w: io.Discard}, ops)
			if !errors.Is(err, failure) {
				t.Fatalf("finishForward() error = %v, want %v", err, failure)
			}
			persisted, err := readTransaction(paths.transactionFile)
			if err != nil {
				t.Fatalf("readTransaction() error = %v", err)
			}
			if persisted.Phase != tt.wantPhase || !persisted.MigrationStarted {
				t.Fatalf("unfinished transaction = %+v", persisted)
			}
			if _, err := os.Stat(paths.blockFile); err != nil {
				t.Fatalf("recovery block is missing: %v", err)
			}

			if err := recoverUpdateLocked(context.Background(), paths, io.Discard, ops); err != nil {
				t.Fatalf("recoverUpdateLocked() error = %v", err)
			}
			for _, path := range []string{paths.transactionFile, paths.blockFile, txn.TempRoot} {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("completed recovery path %s still exists: %v", path, err)
				}
			}
			status, err := readUpdateStatusFile(runnerPathsForHome(paths.home).job(txn.JobID).statusPath)
			if err != nil {
				t.Fatalf("readUpdateStatusFile() error = %v", err)
			}
			if status.Status != StatusCompleted || status.Phase != PhaseDone || status.FailureCode != "" {
				t.Fatalf("recovered status = %+v", status)
			}
			verify(t)
		})
	}
}

func testInstallOps() installOps {
	return installOps{
		stop: func(context.Context, installPaths, string, stepLogger) error {
			return nil
		},
		activate: func(installPaths, string, bool) error {
			return nil
		},
		migrate: func(context.Context, installPaths, updateTransaction, stepLogger) error {
			return nil
		},
		start: func(context.Context, string, stepLogger) error {
			return nil
		},
	}
}

func prepareInstallTransaction(
	t *testing.T,
	id string,
	phase Phase,
	migrationStarted bool,
	serviceWasActive bool,
) (installPaths, updateTransaction) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "Ithiltir-dash")
	paths, err := newInstallPaths(home)
	if err != nil {
		t.Fatalf("newInstallPaths() error = %v", err)
	}
	previousTarget := filepath.ToSlash(filepath.Join("releases", "1.0.0-"+id))
	candidateTarget := filepath.ToSlash(filepath.Join("releases", "1.1.0-"+id))
	previousDir := filepath.Join(home, filepath.FromSlash(previousTarget))
	candidateDir := filepath.Join(home, filepath.FromSlash(candidateTarget))
	tempRoot := filepath.Join(root, ".ithiltir-dash-update-"+id)
	for _, dir := range []string{previousDir, candidateDir, tempRoot} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
	}
	currentTarget := previousTarget
	if migrationStarted {
		currentTarget = candidateTarget
	}
	if err := os.Symlink(currentTarget, paths.current); err != nil {
		t.Fatalf("Symlink(current) error = %v", err)
	}
	txn := updateTransaction{
		FormatVersion:    1,
		JobID:            id,
		Phase:            phase,
		PreviousTarget:   previousTarget,
		CandidateTarget:  candidateTarget,
		CandidateDir:     candidateDir,
		TempRoot:         tempRoot,
		ServiceManager:   "none",
		ServiceWasActive: serviceWasActive,
		MigrationStarted: migrationStarted,
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
	}
	if err := writeTransaction(paths.transactionFile, txn); err != nil {
		t.Fatalf("writeTransaction() error = %v", err)
	}
	if err := writeAtomicFile(paths.blockFile, []byte(id+"\n"), 0o600); err != nil {
		t.Fatalf("writeAtomicFile(block) error = %v", err)
	}
	task := runnerPathsForHome(home).job(id)
	if err := writeUpdateStatusFile(task.statusPath, statusFile{
		ID:            id,
		Unit:          "manual-" + id,
		Status:        StatusFailed,
		Action:        ActionUpdate,
		Channel:       ChannelRelease,
		Origin:        OriginManual,
		TargetVersion: "1.1.0",
		Phase:         phase,
		FailureCode:   "recovery_required",
		RecoveryPath:  tempRoot,
		StartedAt:     time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("writeUpdateStatusFile() error = %v", err)
	}
	return paths, txn
}
