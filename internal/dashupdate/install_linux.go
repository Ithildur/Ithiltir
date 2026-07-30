//go:build linux

package dashupdate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	appversion "dash/internal/version"

	"golang.org/x/sys/unix"
)

var (
	ErrUpdateConflict   = errors.New("installed Dash changed after the update check")
	ErrRecoveryRequired = errors.New("an unfinished Dash update requires recovery")
)

const (
	serviceCommandTimeout = 2 * time.Minute
	serviceStartTimeout   = 20 * time.Second
	serviceStartStability = 5 * time.Second
	serviceStatePoll      = 250 * time.Millisecond
)

type installPaths struct {
	home            string
	releases        string
	current         string
	layoutMarker    string
	bin             string
	config          string
	serviceFile     string
	manualRun       string
	stateDir        string
	transactionFile string
	blockFile       string
	lockFile        string
	serviceDropIn   string
}

type installOps struct {
	stop     func(context.Context, installPaths, string, stepLogger) error
	activate func(installPaths, string, bool) error
	migrate  func(context.Context, installPaths, updateTransaction, stepLogger) error
	start    func(context.Context, string, stepLogger) error
}

func nativeInstallOps() installOps {
	return installOps{
		stop:     stopDash,
		activate: activateRelease,
		migrate:  runCandidateMigration,
		start:    startDash,
	}
}

func newInstallPaths(home string) (installPaths, error) {
	home = filepath.Clean(strings.TrimSpace(home))
	if !filepath.IsAbs(home) || home == string(filepath.Separator) {
		return installPaths{}, fmt.Errorf("invalid Dash home %q", home)
	}
	stateDir := filepath.Join(home, "runtime", updateStateDirName)
	return installPaths{
		home:            home,
		releases:        filepath.Join(home, "releases"),
		current:         filepath.Join(home, "current"),
		layoutMarker:    filepath.Join(home, ".release-layout-v1"),
		bin:             filepath.Join(home, "bin", "dash"),
		config:          filepath.Join(home, "configs", "config.local.yaml"),
		serviceFile:     "/etc/systemd/system/dash.service",
		manualRun:       filepath.Join(home, "run_dash.sh"),
		stateDir:        stateDir,
		transactionFile: filepath.Join(stateDir, updateTransactionName),
		blockFile:       filepath.Join(stateDir, updateBlockName),
		lockFile:        filepath.Join(home, "runtime", "update.lock"),
		serviceDropIn:   "/etc/systemd/system/dash.service.d/dash-update.conf",
	}, nil
}

func applyUpdate(ctx context.Context, paths installPaths, req jobRequest, report *jobReporter) error {
	lock, err := acquireUpdateLock(paths.lockFile)
	if err != nil {
		return err
	}
	defer lock.Close()
	return runUpdateLocked(ctx, paths, req, report)
}

func runUpdateLocked(ctx context.Context, paths installPaths, req jobRequest, report *jobReporter) error {
	updateErr := applyUpdateLocked(ctx, paths, req, report)
	if err := report.finish(updateErr); err != nil {
		return errors.Join(updateErr, fmt.Errorf("persist update result: %w", err))
	}
	if err := removeDoneTransaction(paths.transactionFile, req.ID); err != nil {
		report.printf("warning: clean completed update transaction: %v\n", err)
	}
	return updateErr
}

func applyUpdateLocked(ctx context.Context, paths installPaths, req jobRequest, report *jobReporter) error {
	return applyUpdateLockedWithOps(ctx, paths, req, report, nativeInstallOps())
}

func applyUpdateLockedWithOps(
	ctx context.Context,
	paths installPaths,
	req jobRequest,
	report *jobReporter,
	ops installOps,
) error {
	if err := report.phase(PhaseLocked); err != nil {
		return err
	}
	if err := ensureNoPendingUpdate(paths); err != nil {
		return err
	}

	installed, err := inspectInstalled(ctx, paths.home)
	if err != nil {
		return err
	}
	needed, err := updateNeeded(req.Plan, installed)
	if err != nil {
		return err
	}
	if !needed {
		report.printf("Dash is already at %s; nothing to update.\n", installed.Version)
		return nil
	}
	manager, err := detectServiceManager(paths, req.Plan.ServiceManager)
	if err != nil {
		return err
	}
	txn, err := stageUpdate(ctx, paths, req, installed, manager, report)
	if err != nil {
		return err
	}
	txn, err = switchRelease(ctx, paths, txn, report, ops)
	if err != nil {
		if txn.MigrationStarted {
			report.setRecoveryPath(txn.TempRoot)
		}
		return err
	}
	if err := finishForward(ctx, paths, txn, report, ops); err != nil {
		report.setRecoveryPath(txn.TempRoot)
		return err
	}
	if txn.TempRoot != "" {
		if err := os.RemoveAll(txn.TempRoot); err != nil {
			report.printf("warning: remove update staging directory: %v\n", err)
		}
	}
	if err := pruneReleases(paths); err != nil {
		report.printf("warning: prune old releases: %v\n", err)
	}
	report.printf("updated: %s\n", req.Plan.TargetVersion)
	return nil
}

func ensureNoPendingUpdate(paths installPaths) error {
	for _, stateFile := range []string{paths.transactionFile, paths.blockFile} {
		if _, err := os.Lstat(stateFile); err == nil {
			return fmt.Errorf("%w: run dash update recover", ErrRecoveryRequired)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect update state: %w", err)
		}
	}
	return nil
}

func updateNeeded(plan Plan, installed installedState) (bool, error) {
	if installed.Version != plan.ExpectedCurrentVersion || installed.Revision != plan.ExpectedInstallRevision {
		return false, fmt.Errorf(
			"%w: current=%s revision=%s",
			ErrUpdateConflict,
			installed.Version,
			installed.Revision,
		)
	}
	cmp, err := appversion.Compare(plan.TargetVersion, installed.Version)
	if err != nil {
		return false, err
	}
	switch {
	case plan.Action == ActionUpdate && cmp < 0:
		return false, fmt.Errorf("%w: target %s is not newer than %s", ErrUpdateConflict, plan.TargetVersion, installed.Version)
	case plan.Action == ActionUpdate && cmp == 0:
		return false, nil
	case plan.Action == ActionReinstall && cmp < 0:
		return false, fmt.Errorf("%w: target %s is older than %s", ErrUpdateConflict, plan.TargetVersion, installed.Version)
	default:
		return true, nil
	}
}

func stageUpdate(
	ctx context.Context,
	paths installPaths,
	req jobRequest,
	installed installedState,
	manager string,
	report *jobReporter,
) (updateTransaction, error) {
	asset, err := newReleaseSource().Find(ctx, req.Plan.TargetVersion)
	if err != nil {
		return updateTransaction{}, fmt.Errorf("resolve target release: %w", err)
	}
	tempRoot, err := os.MkdirTemp(filepath.Dir(paths.home), ".ithiltir-dash-update-")
	if err != nil {
		return updateTransaction{}, fmt.Errorf("create update staging directory: %w", err)
	}
	if err := os.Chmod(tempRoot, 0o700); err != nil {
		_ = os.RemoveAll(tempRoot)
		return updateTransaction{}, fmt.Errorf("protect update staging directory: %w", err)
	}
	retainTemp := false
	defer func() {
		if !retainTemp {
			_ = os.RemoveAll(tempRoot)
		}
	}()

	if err := report.phase(PhaseDownload); err != nil {
		return updateTransaction{}, err
	}
	report.printf("download: %s\n", asset.URL)
	archive := filepath.Join(tempRoot, "dash.tar.gz")
	if err := downloadRelease(ctx, asset, archive); err != nil {
		return updateTransaction{}, err
	}
	if err := report.phase(PhaseValidate); err != nil {
		return updateTransaction{}, err
	}
	extractDir := filepath.Join(tempRoot, "extract")
	pkgRoot, err := extractRelease(archive, extractDir)
	if err != nil {
		return updateTransaction{}, err
	}
	if _, err := validateReleasePackage(ctx, pkgRoot, req.Plan.TargetVersion); err != nil {
		return updateTransaction{}, err
	}
	if info, err := os.Stat(paths.config); err != nil || !info.Mode().IsRegular() {
		if err == nil {
			err = errors.New("not a regular file")
		}
		return updateTransaction{}, fmt.Errorf("invalid Dash config %s: %w", paths.config, err)
	}
	if err := normalizeReleaseModes(pkgRoot); err != nil {
		return updateTransaction{}, err
	}
	if err := syncReleaseTree(pkgRoot); err != nil {
		return updateTransaction{}, fmt.Errorf("persist validated release: %w", err)
	}

	candidateDir, candidateTarget, err := prepareRelease(paths, pkgRoot, req.Plan.TargetVersion, req.ID)
	if err != nil {
		return updateTransaction{}, err
	}
	retainCandidate := false
	defer func() {
		if !retainCandidate {
			_ = os.RemoveAll(candidateDir)
		}
	}()
	wasActive, err := serviceIsActive(ctx, manager)
	if err != nil {
		return updateTransaction{}, err
	}
	backupDir := ""
	if installed.Legacy {
		backupDir = filepath.Join(tempRoot, "backup", "legacy")
		if err := backupLegacyInstall(paths, backupDir); err != nil {
			return updateTransaction{}, fmt.Errorf("back up legacy Dash install: %w", err)
		}
	}
	txn := updateTransaction{
		FormatVersion:    1,
		JobID:            req.ID,
		Phase:            PhasePrepared,
		PreviousTarget:   installed.Target,
		CandidateTarget:  candidateTarget,
		CandidateDir:     candidateDir,
		TempRoot:         tempRoot,
		BackupDir:        backupDir,
		Legacy:           installed.Legacy,
		ServiceManager:   manager,
		ServiceWasActive: wasActive,
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
	}
	committed, err := writeTransactionState(paths.transactionFile, txn)
	if committed {
		retainTemp = true
		retainCandidate = true
	}
	if err != nil {
		if committed {
			report.setRecoveryPath(tempRoot)
		}
		return updateTransaction{}, fmt.Errorf("persist update transaction: %w", err)
	}
	return txn, nil
}

type installReporter interface {
	stepLogger
	phase(Phase) error
}

func switchRelease(
	ctx context.Context,
	paths installPaths,
	txn updateTransaction,
	report installReporter,
	ops installOps,
) (updateTransaction, error) {
	if err := ensureServiceRecoveryGuard(ctx, paths, txn.ServiceManager, report); err != nil {
		return txn, rollbackBeforeMigration(ctx, paths, txn, ops, fmt.Errorf("install update recovery guard: %w", err))
	}
	if err := writeAtomicFile(paths.blockFile, []byte(txn.JobID+"\n"), 0o600); err != nil {
		return txn, rollbackBeforeMigration(ctx, paths, txn, ops, fmt.Errorf("persist update block: %w", err))
	}
	if err := report.phase(PhasePrepared); err != nil {
		return txn, rollbackBeforeMigration(ctx, paths, txn, ops, err)
	}
	if err := reachPhase(paths.transactionFile, &txn, PhaseStopping, report); err != nil {
		return txn, rollbackBeforeMigration(ctx, paths, txn, ops, err)
	}
	if err := ops.stop(ctx, paths, txn.ServiceManager, report); err != nil {
		return txn, rollbackBeforeMigration(ctx, paths, txn, ops, fmt.Errorf("stop Dash: %w", err))
	}
	if err := reachPhase(paths.transactionFile, &txn, PhaseSwitching, report); err != nil {
		return txn, rollbackBeforeMigration(ctx, paths, txn, ops, err)
	}
	if err := ops.activate(paths, txn.CandidateTarget, txn.Legacy); err != nil {
		return txn, rollbackBeforeMigration(ctx, paths, txn, ops, fmt.Errorf("activate release: %w", err))
	}

	// From this persisted point onward the schema may be newer than the old
	// binary. Recovery must finish forward with the candidate release.
	txn.Phase = PhaseMigrating
	txn.MigrationStarted = true
	if err := writeTransaction(paths.transactionFile, txn); err != nil {
		// The atomic rename may already have committed MigrationStarted even when
		// the following directory sync reports an error. Keep the candidate and
		// block in place; recovery can safely decide from the transaction on disk.
		return txn, fmt.Errorf("persist migration boundary: %w", err)
	}
	if err := report.phase(PhaseMigrating); err != nil {
		return txn, err
	}
	return txn, nil
}

func reachPhase(path string, txn *updateTransaction, phase Phase, report installReporter) error {
	current := updatePhaseOrder(txn.Phase)
	next := updatePhaseOrder(phase)
	if current < 0 {
		return fmt.Errorf("invalid current update phase %q", txn.Phase)
	}
	if next < 0 || phase == PhaseDone {
		return fmt.Errorf("invalid running update phase %q", phase)
	}
	if next < current {
		return nil
	}
	if next > current {
		txn.Phase = phase
		if err := writeTransaction(path, *txn); err != nil {
			return err
		}
	}
	return report.phase(phase)
}

func finishForward(
	ctx context.Context,
	paths installPaths,
	txn updateTransaction,
	report installReporter,
	ops installOps,
) error {
	if err := ops.migrate(ctx, paths, txn, report); err != nil {
		return fmt.Errorf("migrate Dash database: %w", err)
	}
	if err := reachPhase(paths.transactionFile, &txn, PhaseStarting, report); err != nil {
		return err
	}
	if err := removeUpdateBlock(paths); err != nil {
		return fmt.Errorf("remove update block: %w", err)
	}
	if txn.ServiceWasActive {
		if err := ops.start(ctx, txn.ServiceManager, report); err != nil {
			return holdDashForRecovery(
				ctx,
				paths,
				txn,
				ops,
				report,
				fmt.Errorf("start updated Dash: %w", err),
			)
		}
	}
	if err := reachPhase(paths.transactionFile, &txn, PhaseCleanup, report); err != nil {
		return err
	}
	if err := markTransactionDone(paths.transactionFile, txn); err != nil {
		return err
	}
	return nil
}

func recoverUpdate(ctx context.Context, paths installPaths, report io.Writer) error {
	lock, err := acquireUpdateLock(paths.lockFile)
	if err != nil {
		return err
	}
	defer lock.Close()
	return recoverUpdateLocked(ctx, paths, report, nativeInstallOps())
}

func recoverUpdateLocked(ctx context.Context, paths installPaths, report io.Writer, ops installOps) error {
	txn, err := readTransaction(paths.transactionFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if _, blockErr := os.Lstat(paths.blockFile); blockErr == nil {
				return errors.New("update block exists without a transaction; manual repair is required")
			} else if !errors.Is(blockErr, os.ErrNotExist) {
				return fmt.Errorf("inspect orphaned update block: %w", blockErr)
			}
			return errors.New("no unfinished Dash update")
		}
		return fmt.Errorf("read update transaction: %w", err)
	}
	if err := validateTransactionPaths(paths, txn); err != nil {
		return err
	}
	log := textReporter{w: report}
	if txn.Phase == PhaseDone {
		return finalizeRecovery(paths, txn, log)
	}
	if err := ensureServiceRecoveryGuard(ctx, paths, txn.ServiceManager, log); err != nil {
		return fmt.Errorf("install update recovery guard: %w", err)
	}
	if err := writeAtomicFile(paths.blockFile, []byte(txn.JobID+"\n"), 0o600); err != nil {
		return fmt.Errorf("persist update block: %w", err)
	}
	if err := ops.stop(ctx, paths, txn.ServiceManager, log); err != nil {
		return fmt.Errorf("stop Dash for recovery: %w", err)
	}
	if !txn.MigrationStarted {
		if err := rollbackBeforeMigration(ctx, paths, txn, ops, nil); err != nil {
			return err
		}
		txn.Phase = PhaseDone
		return finalizeRecovery(paths, txn, log)
	}
	fmt.Fprintln(report, "resuming Dash migration with the candidate release")
	if err := ops.activate(paths, txn.CandidateTarget, txn.Legacy); err != nil {
		return fmt.Errorf("restore candidate release entrypoint: %w", err)
	}
	if err := finishForward(ctx, paths, txn, log, ops); err != nil {
		return err
	}
	txn.Phase = PhaseDone
	if err := finalizeRecovery(paths, txn, log); err != nil {
		return err
	}
	if err := pruneReleases(paths); err != nil {
		fmt.Fprintf(report, "warning: prune old releases: %v\n", err)
	}
	return nil
}

func finalizeRecovery(paths installPaths, txn updateTransaction, log stepLogger) error {
	if txn.Phase != PhaseDone {
		return errors.New("cannot finalize an unfinished update transaction")
	}
	if err := recordRecoveryResult(paths, txn); err != nil {
		return err
	}
	if err := removeDoneTransaction(paths.transactionFile, txn.JobID); err != nil {
		return err
	}
	if txn.TempRoot != "" {
		if err := os.RemoveAll(txn.TempRoot); err != nil {
			log.printf("warning: remove update staging directory: %v\n", err)
		}
	}
	return nil
}

func recordRecoveryResult(paths installPaths, txn updateTransaction) error {
	task := runnerPathsForHome(paths.home).job(txn.JobID)
	item, err := readUpdateStatusFile(task.statusPath)
	if err != nil {
		return fmt.Errorf("read recovered update status: %w", err)
	}
	item, err = statusFromDoneTransaction(item, txn, time.Now())
	if err != nil {
		return fmt.Errorf("derive recovered update status: %w", err)
	}
	if err := writeUpdateStatusFile(task.statusPath, item); err != nil {
		return fmt.Errorf("persist recovered update status: %w", err)
	}
	return nil
}

func acquireUpdateLock(path string) (*os.File, error) {
	return acquireLock(path, 0, "update")
}

func acquireStartLock(path string) (*os.File, error) {
	return acquireLock(path, uint32(os.Geteuid()), "update start")
}

func acquireLock(path string, owner uint32, label string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create %s lock directory: %w", label, err)
	}
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s lock: %w", label, err)
	}
	f := os.NewFile(uintptr(fd), path)
	closeOnError := func(err error) (*os.File, error) {
		_ = f.Close()
		return nil, err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return closeOnError(fmt.Errorf("inspect %s lock: %w", label, err))
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Uid != owner || stat.Mode&0o022 != 0 {
		return closeOnError(fmt.Errorf("%s lock must be a uid %d-owned non-writable regular file", label, owner))
	}
	if err := unix.Fchmod(fd, 0o644); err != nil {
		return closeOnError(fmt.Errorf("protect %s lock: %w", label, err))
	}
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) {
			return closeOnError(ErrRunning)
		}
		return closeOnError(fmt.Errorf("lock Dash %s: %w", label, err))
	}
	return f, nil
}

func prepareRelease(paths installPaths, pkgRoot, version, jobID string) (string, string, error) {
	if err := os.MkdirAll(paths.releases, 0o755); err != nil {
		return "", "", err
	}
	name := version + "-" + time.Now().UTC().Format("20060102T150405Z") + "-" + jobID
	dir := filepath.Join(paths.releases, name)
	if !pathWithin(paths.releases, dir) {
		return "", "", errors.New("invalid release path")
	}
	if _, err := os.Lstat(dir); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			err = os.ErrExist
		}
		return "", "", fmt.Errorf("prepare release: %w", err)
	}
	if err := os.Rename(pkgRoot, dir); err != nil {
		return "", "", fmt.Errorf("move prepared release: %w", err)
	}
	if err := syncDirectory(paths.releases); err != nil {
		return "", "", errors.Join(fmt.Errorf("persist prepared release: %w", err), os.RemoveAll(dir))
	}
	return dir, filepath.ToSlash(filepath.Join("releases", name)), nil
}

func normalizeReleaseModes(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if err := os.Chmod(path, 0o755); err != nil {
				return err
			}
			return os.Chown(path, 0, 0)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("release path is not regular: %s", path)
		}
		mode := os.FileMode(0o644)
		if info.Mode().Perm()&0o111 != 0 || path == filepath.Join(root, "bin", "dash") {
			mode = 0o755
		}
		if err := os.Chmod(path, mode); err != nil {
			return err
		}
		return os.Chown(path, 0, 0)
	})
}

func syncReleaseTree(root string) error {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		return errors.Join(f.Sync(), f.Close())
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := syncDirectory(dirs[i]); err != nil {
			return err
		}
	}
	return nil
}

func detectServiceManager(paths installPaths, requested string) (string, error) {
	requested = normalizedServiceManager(requested)
	systemd := false
	if info, err := os.Stat("/run/systemd/system"); err == nil && info.IsDir() {
		if info, err := os.Stat(paths.serviceFile); err == nil && info.Mode().IsRegular() {
			systemd = true
		}
	}
	manual := false
	if info, err := os.Stat(paths.manualRun); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
		manual = true
	}
	switch requested {
	case "auto":
		if systemd {
			return "systemd", nil
		}
		if manual {
			return "none", nil
		}
		return "", errors.New("cannot detect installed Dash service manager")
	case "systemd":
		if !systemd {
			return "", errors.New("dash systemd service is unavailable")
		}
		return "systemd", nil
	case "none":
		if !manual {
			return "", errors.New("dash manual runner is unavailable")
		}
		return "none", nil
	default:
		return "", fmt.Errorf("invalid service manager %q", requested)
	}
}

func serviceIsActive(ctx context.Context, manager string) (bool, error) {
	if manager == "none" {
		return false, nil
	}
	cmdCtx, cancel := context.WithTimeout(ctx, serviceCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "systemctl", "is-active", "--quiet", "dash.service")
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 3 {
		return false, nil
	}
	if cmdCtx.Err() != nil {
		return false, cmdCtx.Err()
	}
	return false, err
}

type stepLogger interface {
	printf(string, ...any)
}

type textReporter struct{ w io.Writer }

func (r textReporter) printf(format string, args ...any) {
	if r.w != nil {
		_, _ = fmt.Fprintf(r.w, format, args...)
	}
}

func (textReporter) phase(Phase) error {
	return nil
}

func stopDash(ctx context.Context, paths installPaths, manager string, log stepLogger) error {
	if manager == "systemd" {
		log.printf("stopping dash.service\n")
		if err := runSystemctl(ctx, log, "stop", "dash.service"); err != nil {
			return err
		}
	}
	return stopManualDash(ctx, paths, log)
}

func startDash(ctx context.Context, manager string, log stepLogger) error {
	if manager == "none" {
		return nil
	}
	log.printf("starting dash.service\n")
	if err := runSystemctl(ctx, log, "start", "dash.service"); err != nil {
		return err
	}
	if err := waitDashStable(ctx, serviceStartStability, serviceStatePoll); err != nil {
		return fmt.Errorf("dash.service did not become stable: %w", err)
	}
	return nil
}

func runSystemctl(ctx context.Context, log stepLogger, args ...string) error {
	cmdCtx, cancel := context.WithTimeout(ctx, serviceCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "systemctl", args...)
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		log.printf("%s", out)
		if out[len(out)-1] != '\n' {
			log.printf("\n")
		}
	}
	if cmdCtx.Err() != nil {
		return cmdCtx.Err()
	}
	return err
}

type dashServiceState struct {
	active   string
	sub      string
	restarts uint64
}

func inspectDashService(ctx context.Context) (dashServiceState, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, serviceCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(
		cmdCtx,
		"systemctl",
		"show",
		"dash.service",
		"--property=ActiveState",
		"--property=SubState",
		"--property=NRestarts",
		"--no-pager",
	).CombinedOutput()
	if cmdCtx.Err() != nil {
		return dashServiceState{}, cmdCtx.Err()
	}
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			return dashServiceState{}, err
		}
		return dashServiceState{}, fmt.Errorf("%s: %w", detail, err)
	}
	return parseDashServiceState(out)
}

func parseDashServiceState(raw []byte) (dashServiceState, error) {
	fields := make(map[string]string, 3)
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" {
			return dashServiceState{}, fmt.Errorf("invalid systemd service state line %q", line)
		}
		fields[key] = strings.TrimSpace(value)
	}
	active := fields["ActiveState"]
	sub := fields["SubState"]
	if active == "" || sub == "" {
		return dashServiceState{}, errors.New("systemd service state is incomplete")
	}
	restarts, err := strconv.ParseUint(fields["NRestarts"], 10, 64)
	if err != nil {
		return dashServiceState{}, fmt.Errorf("invalid systemd restart count %q: %w", fields["NRestarts"], err)
	}
	return dashServiceState{active: active, sub: sub, restarts: restarts}, nil
}

func waitDashStable(ctx context.Context, stableFor, poll time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, serviceStartTimeout)
	defer cancel()
	var stableSince time.Time
	var restartCount uint64
	restartObserved := false
	for {
		state, err := inspectDashService(waitCtx)
		if err != nil {
			return err
		}
		if !restartObserved {
			restartCount = state.restarts
			restartObserved = true
		} else if state.restarts != restartCount {
			return fmt.Errorf("dash.service restarted during startup: before=%d after=%d", restartCount, state.restarts)
		}
		if state.active == "active" && state.sub == "running" {
			if stableSince.IsZero() {
				stableSince = time.Now()
			}
			if time.Since(stableSince) >= stableFor {
				return nil
			}
		} else {
			if !stableSince.IsZero() || state.active != "activating" {
				return fmt.Errorf("dash.service state is %s/%s", state.active, state.sub)
			}
		}
		timer := time.NewTimer(poll)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			return waitCtx.Err()
		case <-timer.C:
		}
	}
}

func holdDashForRecovery(
	ctx context.Context,
	paths installPaths,
	txn updateTransaction,
	ops installOps,
	log stepLogger,
	cause error,
) error {
	blockErr := writeAtomicFile(paths.blockFile, []byte(txn.JobID+"\n"), 0o600)
	if blockErr != nil {
		blockErr = fmt.Errorf("restore update block after failed start: %w", blockErr)
	}
	stopErr := ops.stop(ctx, paths, txn.ServiceManager, log)
	if stopErr != nil {
		stopErr = fmt.Errorf("stop Dash after failed start: %w", stopErr)
	}
	return errors.Join(cause, blockErr, stopErr)
}

func stopManualDash(ctx context.Context, paths installPaths, log stepLogger) error {
	pids, err := installedDashPIDs(paths)
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		return nil
	}
	log.printf("stopping manual Dash processes: %v\n", pids)
	for _, pid := range pids {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		remaining, err := installedDashPIDs(paths)
		if err != nil {
			return err
		}
		if len(remaining) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	remaining, err := installedDashPIDs(paths)
	if err != nil {
		return err
	}
	for _, pid := range remaining {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	time.Sleep(100 * time.Millisecond)
	remaining, err = installedDashPIDs(paths)
	if err != nil {
		return err
	}
	if len(remaining) != 0 {
		return fmt.Errorf("dash processes did not stop: %v", remaining)
	}
	return nil
}

func installedDashPIDs(paths installPaths) ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	self := os.Getpid()
	var pids []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 1 || pid == self {
			continue
		}
		exe, err := os.Readlink(filepath.Join("/proc", entry.Name(), "exe"))
		if err != nil {
			continue
		}
		exe = strings.TrimSuffix(exe, " (deleted)")
		if !installedDashExecutable(paths, exe) {
			continue
		}
		cmdline, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil || !dashServerCommandLine(cmdline) {
			continue
		}
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	return pids, nil
}

func dashServerCommandLine(raw []byte) bool {
	args := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
	if len(args) == 0 || args[0] == "" {
		return false
	}
	if len(args) == 1 {
		return true
	}
	switch args[1] {
	case "--version", "-v", "migrate", "update", "check-redis", "pack-theme":
		return false
	default:
		return true
	}
}

func installedDashExecutable(paths installPaths, exe string) bool {
	exe = filepath.Clean(exe)
	if exe == filepath.Clean(paths.bin) {
		return true
	}
	rel, err := filepath.Rel(paths.releases, exe)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	parts := strings.Split(filepath.ToSlash(filepath.Clean(rel)), "/")
	return len(parts) == 3 && parts[0] != "" && parts[0] != "." && parts[0] != ".." && parts[1] == "bin" && parts[2] == "dash"
}

func activateRelease(paths installPaths, target string, legacy bool) error {
	if !validReleaseTarget(target) {
		return fmt.Errorf("invalid release target %q", target)
	}
	if legacy {
		if err := switchCurrent(paths.current, target); err != nil {
			return err
		}
		if err := installCompatibilityAliases(paths, true); err != nil {
			return err
		}
		return writeAtomicFile(paths.layoutMarker, nil, 0o644)
	}
	if err := installCompatibilityAliases(paths, false); err != nil {
		return err
	}
	return switchCurrent(paths.current, target)
}

func switchCurrent(path, target string) error {
	if !validReleaseTarget(target) {
		return fmt.Errorf("invalid release target %q", target)
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("current path is not a symlink: %s", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tmp := path + ".new-" + strconv.Itoa(os.Getpid())
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func installCompatibilityAliases(paths installPaths, removeLegacy bool) error {
	aliases := []struct {
		path   string
		target string
	}{
		{filepath.Join(paths.home, "bin"), "current/bin"},
		{filepath.Join(paths.home, "dist"), "current/dist"},
		{filepath.Join(paths.home, "deploy"), "current/deploy"},
		{filepath.Join(paths.home, "install_dash_linux.sh"), "current/install_dash_linux.sh"},
		{filepath.Join(paths.home, "update_dash_linux.sh"), "current/update_dash_linux.sh"},
		{filepath.Join(paths.home, "release.env"), "current/release.env"},
	}
	for _, item := range aliases {
		if err := replaceInstallAlias(item.path, item.target, removeLegacy); err != nil {
			return err
		}
	}
	configDir := filepath.Join(paths.home, "configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	if err := replaceInstallAlias(filepath.Join(configDir, "config.example.yaml"), "../current/configs/config.example.yaml", removeLegacy); err != nil {
		return err
	}
	return tightenInstallPermissions(paths)
}

func replaceInstallAlias(name, target string, removeLegacy bool) error {
	if info, err := os.Lstat(name); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			if !removeLegacy {
				return fmt.Errorf("refusing to replace non-symlink path %s", name)
			}
			if err := os.RemoveAll(name); err != nil {
				return err
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tmp := name + ".new-" + strconv.Itoa(os.Getpid())
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, name); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return syncDirectory(filepath.Dir(name))
}

func runCandidateMigration(ctx context.Context, paths installPaths, txn updateTransaction, log stepLogger) error {
	if !pathWithin(paths.releases, txn.CandidateDir) {
		return errors.New("candidate release is outside the release directory")
	}
	bin := filepath.Join(txn.CandidateDir, "bin", "dash")
	cmd := exec.CommandContext(ctx, bin, "migrate", "-config", paths.config)
	cmd.Env = append(os.Environ(), "DASH_HOME="+paths.home)
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		log.printf("%s", out)
		if out[len(out)-1] != '\n' {
			log.printf("\n")
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func rollbackBeforeMigration(
	ctx context.Context,
	paths installPaths,
	txn updateTransaction,
	ops installOps,
	cause error,
) error {
	if txn.MigrationStarted {
		return cause
	}
	if err := validateTransactionPaths(paths, txn); err != nil {
		return errors.Join(cause, err)
	}
	var restoreErr error
	if txn.Legacy {
		restoreErr = restoreLegacyInstall(paths, txn.BackupDir)
	} else if txn.PreviousTarget != "" {
		restoreErr = switchCurrent(paths.current, txn.PreviousTarget)
		if restoreErr == nil {
			restoreErr = installCompatibilityAliases(paths, false)
		}
	}
	if restoreErr == nil {
		restoreErr = removeUpdateBlock(paths)
	}
	if restoreErr == nil && txn.ServiceWasActive {
		restoreErr = ops.start(ctx, txn.ServiceManager, textReporter{w: io.Discard})
		if restoreErr != nil {
			restoreErr = holdDashForRecovery(
				ctx,
				paths,
				txn,
				ops,
				textReporter{w: io.Discard},
				restoreErr,
			)
		}
	}
	if restoreErr == nil {
		restoreErr = markTransactionDone(paths.transactionFile, txn)
	}
	if restoreErr != nil {
		return errors.Join(cause, restoreErr)
	}
	var cleanupErr error
	if txn.CandidateDir != "" {
		cleanupErr = errors.Join(cleanupErr, os.RemoveAll(txn.CandidateDir))
	}
	if txn.TempRoot != "" {
		cleanupErr = errors.Join(cleanupErr, os.RemoveAll(txn.TempRoot))
	}
	if cause != nil {
		return errors.Join(cause, cleanupErr)
	}
	return nil
}

func backupLegacyInstall(paths installPaths, backup string) error {
	if err := os.MkdirAll(backup, 0o700); err != nil {
		return err
	}
	for _, rel := range managedLegacyPaths() {
		src := filepath.Join(paths.home, rel)
		if _, err := os.Lstat(src); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := copyTree(src, filepath.Join(backup, rel)); err != nil {
			return err
		}
	}
	return nil
}

func restoreLegacyInstall(paths installPaths, backup string) error {
	if !pathWithin(filepath.Dir(paths.home), backup) {
		return errors.New("legacy backup is outside the installation filesystem")
	}
	if info, err := os.Stat(backup); err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("not a directory")
		}
		return fmt.Errorf("invalid legacy backup: %w", err)
	}
	for _, rel := range managedLegacyPaths() {
		if err := os.RemoveAll(filepath.Join(paths.home, rel)); err != nil {
			return err
		}
	}
	for _, path := range []string{paths.current, paths.layoutMarker} {
		if _, err := removeDurable(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	entries, err := os.ReadDir(backup)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := copyTree(filepath.Join(backup, entry.Name()), filepath.Join(paths.home, entry.Name())); err != nil {
			return err
		}
	}
	return tightenInstallPermissions(paths)
}

func managedLegacyPaths() []string {
	return []string{"bin", "configs", "dist", "deploy", "install_dash_linux.sh", "update_dash_linux.sh", "release.env"}
}

func copyTree(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case info.IsDir():
		if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyTree(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
				return err
			}
		}
		return errors.Join(syncDirectory(dst), syncDirectory(filepath.Dir(dst)))
	case info.Mode().IsRegular():
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		syncErr := out.Sync()
		closeErr := errors.Join(in.Close(), out.Close())
		if err := errors.Join(copyErr, syncErr, closeErr); err != nil {
			return err
		}
		return syncDirectory(filepath.Dir(dst))
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.Symlink(target, dst); err != nil {
			return err
		}
		return syncDirectory(filepath.Dir(dst))
	default:
		return fmt.Errorf("unsupported legacy path type: %s", src)
	}
}

func tightenInstallPermissions(paths installPaths) error {
	for _, item := range []struct {
		path string
		mode os.FileMode
	}{
		{paths.config, 0o600},
		{paths.serviceFile, 0o600},
		{paths.manualRun, 0o700},
	} {
		info, err := os.Stat(item.path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("sensitive path is not regular: %s", item.path)
		}
		if err := os.Chown(item.path, 0, 0); err != nil {
			return err
		}
		if err := os.Chmod(item.path, item.mode); err != nil {
			return err
		}
	}
	return nil
}

func pruneReleases(paths installPaths) error {
	target, err := os.Readlink(paths.current)
	if err != nil {
		return fmt.Errorf("read current release: %w", err)
	}
	if !validReleaseTarget(target) {
		return fmt.Errorf("invalid current release target %q", target)
	}
	current := filepath.Base(target)
	entries, err := os.ReadDir(paths.releases)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == current {
			continue
		}
		path := filepath.Join(paths.releases, entry.Name())
		if !pathWithin(paths.releases, path) {
			return fmt.Errorf("invalid release path %s", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unexpected release entry %s", path)
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return syncDirectory(paths.releases)
}

func validateTransactionPaths(paths installPaths, txn updateTransaction) error {
	if txn.CandidateDir == "" || !filepath.IsAbs(txn.CandidateDir) || !pathWithin(paths.releases, txn.CandidateDir) {
		return errors.New("transaction candidate is outside the release directory")
	}
	if txn.TempRoot == "" || !filepath.IsAbs(txn.TempRoot) || !pathWithin(filepath.Dir(paths.home), txn.TempRoot) ||
		!strings.HasPrefix(filepath.Base(txn.TempRoot), ".ithiltir-dash-update-") {
		return errors.New("transaction staging directory is outside the installation filesystem")
	}
	if txn.Legacy && (txn.BackupDir == "" || !filepath.IsAbs(txn.BackupDir) || !pathWithin(txn.TempRoot, txn.BackupDir)) {
		return errors.New("transaction legacy backup is outside the staging directory")
	}
	wantCandidate := filepath.Join(paths.home, filepath.FromSlash(txn.CandidateTarget))
	if filepath.Clean(txn.CandidateDir) != filepath.Clean(wantCandidate) {
		return errors.New("transaction candidate target does not match its release directory")
	}
	if txn.ServiceManager != "systemd" && txn.ServiceManager != "none" {
		return fmt.Errorf("invalid transaction service manager %q", txn.ServiceManager)
	}
	return nil
}

func ensureServiceRecoveryGuard(ctx context.Context, paths installPaths, manager string, log stepLogger) error {
	if manager != "systemd" {
		return nil
	}
	dir := filepath.Dir(paths.serviceDropIn)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.Chown(dir, 0, 0); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		return err
	}
	condition, err := systemdConditionPath(paths.blockFile)
	if err != nil {
		return err
	}
	content := "[Unit]\nConditionPathExists=" + condition + "\n"
	if err := writeAtomicFile(paths.serviceDropIn, []byte(content), 0o644); err != nil {
		return err
	}
	if err := os.Chown(paths.serviceDropIn, 0, 0); err != nil {
		return err
	}
	return runSystemctl(ctx, log, "daemon-reload")
}

func systemdConditionPath(path string) (string, error) {
	if !filepath.IsAbs(path) || strings.ContainsAny(path, " \t\r\n\\\"") {
		return "", fmt.Errorf("dash home cannot be represented safely in a systemd condition: %q", path)
	}
	return "!" + strings.ReplaceAll(path, "%", "%%"), nil
}

func removeUpdateBlock(paths installPaths) error {
	_, err := removeDurable(paths.blockFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func pathWithin(root, candidate string) bool {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
