package dashupdate

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"dash/internal/config"
	appversion "dash/internal/version"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

const (
	StatusIdle      Status = "idle"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"

	ActionUpdate    Action = "update"
	ActionReinstall Action = "reinstall"

	OriginManual Origin = "manual"
	OriginAuto   Origin = "auto"

	ChannelRelease    Channel = appversion.ChannelRelease
	ChannelPrerelease Channel = appversion.ChannelPrerelease

	updateStateDirName  = "dash-update"
	updateJobsDirName   = "jobs"
	updateCurrentName   = "current"
	updateStartLockName = "start.lock"
	updateStatusName    = "status.env"
	updateLogName       = "update.log"
	updateLogTailBytes  = 16 * 1024
	updateStartGrace    = 15 * time.Second
	updateCommandLimit  = 10 * time.Second
	availabilityTTL     = time.Minute
)

var (
	ErrRunning        = errors.New("dash update is running")
	ErrUnavailable    = errors.New("dash update unavailable")
	ErrInvalidRequest = errors.New("invalid dash update request")
	ErrAlreadyCurrent = errors.New("dash is already current")
	errPlanInput      = errors.New("invalid update plan input")
)

type Status string
type Action string
type Origin string
type Channel = appversion.Channel
type VersionStatus string

type systemdUnitState uint8

const (
	systemdUnitMissing systemdUnitState = iota
	systemdUnitBusy
	systemdUnitStopped
)

type Runner struct {
	mu             sync.Mutex
	availabilityMu sync.Mutex
	availability   availabilityCache
	availabilitySF singleflight.Group
	source         releaseSource
}

const (
	VersionAvailable VersionStatus = "available"
	VersionCurrent   VersionStatus = "current"
	VersionAhead     VersionStatus = "ahead"
	VersionUnknown   VersionStatus = "unknown"
)

type RunInput struct {
	Action                  Action
	Channel                 Channel
	Lang                    string
	Origin                  Origin
	TargetVersion           string
	ExpectedCurrentVersion  string
	ExpectedInstallRevision string
}

type State struct {
	ID                string  `json:"id,omitempty"`
	Status            Status  `json:"status"`
	Action            Action  `json:"action,omitempty"`
	Channel           Channel `json:"channel,omitempty"`
	TargetVersion     string  `json:"target_version,omitempty"`
	Phase             Phase   `json:"phase,omitempty"`
	FailureCode       string  `json:"failure_code,omitempty"`
	RecoveryPath      string  `json:"recovery_path,omitempty"`
	StartedAt         string  `json:"started_at,omitempty"`
	FinishedAt        string  `json:"finished_at,omitempty"`
	ExitCode          *int    `json:"exit_code,omitempty"`
	LogTail           string  `json:"log_tail,omitempty"`
	Available         bool    `json:"available"`
	UnavailableReason string  `json:"unavailable_reason,omitempty"`
	Origin            Origin  `json:"-"`
}

type statusFile struct {
	ID                      string
	Unit                    string
	Status                  Status
	Action                  Action
	Channel                 Channel
	Origin                  Origin
	TargetVersion           string
	ExpectedCurrentVersion  string
	ExpectedInstallRevision string
	Phase                   Phase
	FailureCode             string
	RecoveryPath            string
	StartedAt               string
	FinishedAt              string
	ExitCode                *int
	ExecutorPID             int
	LogFile                 string
}

type runnerPaths struct {
	home            string
	stateDir        string
	jobsDir         string
	currentPath     string
	startLockPath   string
	statusPath      string
	logPath         string
	dashPath        string
	transactionPath string
	blockPath       string
}

type taskPaths struct {
	dir         string
	requestPath string
	statusPath  string
	logPath     string
}

type availabilityCache struct {
	paths     runnerPaths
	err       error
	expiresAt time.Time
}

type Check struct {
	CurrentVersion     string        `json:"current_version"`
	CurrentChannel     Channel       `json:"current_channel,omitempty"`
	TargetChannel      Channel       `json:"target_channel"`
	LatestVersion      string        `json:"latest_version"`
	InstallRevision    string        `json:"install_revision"`
	VersionStatus      VersionStatus `json:"version_status"`
	BundledNodeVersion string        `json:"bundled_node_version"`
}

func NewRunner() *Runner {
	return &Runner{source: newReleaseSource()}
}

func (r *Runner) Status(ctx context.Context) State {
	r.mu.Lock()
	view, paths, checkAvailability := r.taskStatusLocked(ctx)
	r.mu.Unlock()
	return r.withAvailability(ctx, view, paths, checkAvailability)
}

func (r *Runner) statusLocked(ctx context.Context) State {
	view, paths, checkAvailability := r.taskStatusLocked(ctx)
	return r.withAvailability(ctx, view, paths, checkAvailability)
}

func (r *Runner) taskStatusLocked(ctx context.Context) (State, runnerPaths, bool) {
	view := State{Status: StatusIdle}
	paths, pathErr := r.paths()
	var statusErr error

	if pathErr == nil {
		view, statusErr = r.taskState(ctx, paths)
	}

	if view.Status == "" {
		view.Status = StatusIdle
	}
	if pathErr != nil {
		view.Available = false
		view.UnavailableReason = updateUnavailableReason(pathErr)
	} else if statusErr != nil {
		view.Available = false
		view.UnavailableReason = "read update status: " + statusErr.Error()
	} else {
		return view, paths, true
	}
	return view, paths, false
}

func (r *Runner) taskState(ctx context.Context, paths runnerPaths) (State, error) {
	view := State{Status: StatusIdle}
	task := paths.currentTask()
	item, err := readUpdateStatusFile(task.statusPath)
	if errors.Is(err, os.ErrNotExist) {
		return view, nil
	}
	if err != nil {
		return view, err
	}
	item.LogFile = task.logPath
	item, reconcileErr := r.reconcileRunningStatus(ctx, item, task, paths)
	return stateFromStatus(item, task.logPath), reconcileErr
}

func (r *Runner) withAvailability(ctx context.Context, view State, paths runnerPaths, check bool) State {
	if !check {
		return view
	}
	if err := r.availableWithPaths(ctx, paths); err != nil {
		view.Available = false
		view.UnavailableReason = updateUnavailableReason(err)
	} else {
		view.Available = true
	}
	return view
}

func (r *Runner) reconcileRunningStatus(ctx context.Context, item statusFile, task taskPaths, paths runnerPaths) (statusFile, error) {
	item, err := reconcileDoneTransaction(item, task, paths)
	if err != nil {
		return item, err
	}
	if item.Status != StatusRunning || runningStatusFresh(item.StartedAt) {
		return item, nil
	}
	if strings.HasPrefix(item.Unit, "manual-") {
		if manualUpdateRunning(item.ExecutorPID) {
			return item, nil
		}
		return r.settleStoppedStatus(item, task, paths)
	}
	unitState, err := systemdUnitStatus(ctx, item.Unit)
	if err != nil {
		return item, err
	}
	if unitState == systemdUnitBusy {
		return item, nil
	}
	return r.settleStoppedStatus(item, task, paths)
}

func (r *Runner) settleStoppedStatus(item statusFile, task taskPaths, paths runnerPaths) (statusFile, error) {
	// The executor writes its terminal status before it exits. Re-read after the
	// process or unit has stopped so reconciliation cannot overwrite that final
	// write with a synthetic failure based on an older snapshot.
	latest, err := readUpdateStatusFile(task.statusPath)
	if err != nil {
		return item, fmt.Errorf("re-read stopped update status: %w", err)
	}
	if latest.ID != item.ID {
		return item, fmt.Errorf("stopped update status changed from job %q to %q", item.ID, latest.ID)
	}
	latest, err = reconcileDoneTransaction(latest, task, paths)
	if err != nil {
		return latest, err
	}
	if latest.Status != StatusRunning {
		return latest, nil
	}
	return r.failStaleStatus(latest, task, paths)
}

func reconcileDoneTransaction(item statusFile, task taskPaths, paths runnerPaths) (statusFile, error) {
	txn, err := readTransaction(paths.transactionPath)
	if errors.Is(err, os.ErrNotExist) {
		return item, nil
	}
	if err != nil {
		return item, fmt.Errorf("read completed update transaction: %w", err)
	}
	if txn.Phase != PhaseDone {
		return item, nil
	}
	if txn.JobID != item.ID {
		return item, fmt.Errorf("completed update transaction belongs to job %q, want %q", txn.JobID, item.ID)
	}
	if item.Status == StatusRunning {
		item, err = statusFromDoneTransaction(item, txn, time.Now())
		if err != nil {
			return item, err
		}
		if err := writeUpdateStatusFile(task.statusPath, item); err != nil {
			return item, fmt.Errorf("persist completed update status: %w", err)
		}
	}
	if err := removeDoneTransaction(paths.transactionPath, txn.JobID); err != nil {
		return item, err
	}
	return item, nil
}

func (r *Runner) failStaleStatus(item statusFile, task taskPaths, paths runnerPaths) (statusFile, error) {
	item.Status = StatusFailed
	item.FailureCode = "executor_lost"
	pending, err := recoveryStatePending(paths)
	if err != nil {
		return item, err
	}
	if pending {
		item.FailureCode = "recovery_required"
		if txn, err := readTransaction(paths.transactionPath); err == nil && txn.JobID == item.ID {
			item.RecoveryPath = txn.TempRoot
		}
	}
	item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	item.ExitCode = nil
	if err := writeUpdateStatusFile(task.statusPath, item); err != nil {
		return item, fmt.Errorf("write reconciled update status: %w", err)
	}
	return item, nil
}

func stateFromStatus(item statusFile, logPath string) State {
	view := State{
		ID:            item.ID,
		Status:        item.Status,
		Action:        item.Action,
		Channel:       item.Channel,
		TargetVersion: item.TargetVersion,
		Phase:         item.Phase,
		FailureCode:   item.FailureCode,
		RecoveryPath:  item.RecoveryPath,
		StartedAt:     item.StartedAt,
		FinishedAt:    item.FinishedAt,
		ExitCode:      item.ExitCode,
		Origin:        item.Origin,
	}
	if logTail, err := readLogTail(logPath); err == nil {
		view.LogTail = logTail
	}
	return view
}

func failQueuedJob(task taskPaths, item statusFile, cause error) error {
	exitCode := 1
	item.Status = StatusFailed
	item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	item.ExitCode = &exitCode
	logErr := os.WriteFile(task.logPath, []byte(cause.Error()+"\n"), 0o600)
	statusErr := writeUpdateStatusFile(task.statusPath, item)
	return errors.Join(cause, logErr, statusErr)
}

func (r *Runner) Start(ctx context.Context, in RunInput) (State, error) {
	plan, err := r.Prepare(ctx, in)
	if err != nil {
		return r.Status(ctx), err
	}
	return r.StartPrepared(ctx, plan)
}

func (r *Runner) Prepare(ctx context.Context, in RunInput) (Plan, error) {
	plan, err := r.resolvePlan(ctx, in)
	if errors.Is(err, errPlanInput) {
		err = fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	return plan, err
}

func (r *Runner) StartPrepared(ctx context.Context, plan Plan) (State, error) {
	if err := plan.validate(); err != nil {
		return r.Status(ctx), fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	paths, err := r.paths()
	if err != nil {
		return r.statusLocked(ctx), err
	}
	if err := r.availableWithPaths(ctx, paths); err != nil {
		return r.statusLocked(ctx), err
	}
	if err := os.MkdirAll(paths.jobsDir, 0o700); err != nil {
		return r.statusLocked(ctx), fmt.Errorf("%w: create state dir: %w", ErrUnavailable, err)
	}
	startLock, err := acquireStartLock(paths.startLockPath)
	if err != nil {
		return r.statusLocked(ctx), err
	}
	defer startLock.Close()

	current := r.statusLocked(ctx)
	if current.Status == StatusRunning {
		return current, ErrRunning
	}

	systemdRun, err := exec.LookPath("systemd-run")
	if err != nil {
		return current, fmt.Errorf("%w: systemd-run not found", ErrUnavailable)
	}
	now := time.Now().UTC()
	id := newUpdateJobID(now)
	startedAt := now.Format(time.RFC3339)
	unit := "ithiltir-dash-update-" + id
	task := paths.job(id)
	if err := os.Mkdir(task.dir, 0o700); err != nil {
		return current, fmt.Errorf("%w: create job state dir: %w", ErrUnavailable, err)
	}
	removeTask := func(err error) (State, error) {
		return current, errors.Join(err, os.RemoveAll(task.dir))
	}

	if err := os.WriteFile(task.logPath, nil, 0o600); err != nil {
		return removeTask(fmt.Errorf("%w: initialize log: %w", ErrUnavailable, err))
	}
	request := jobRequest{ID: id, Unit: unit, Plan: plan}
	if err := writeJobRequest(task.requestPath, request); err != nil {
		return removeTask(fmt.Errorf("%w: write update request: %w", ErrUnavailable, err))
	}
	item := statusForRequest(request, task.logPath, now)
	if err := writeUpdateStatusFile(task.statusPath, item); err != nil {
		return removeTask(fmt.Errorf("%w: write status: %w", ErrUnavailable, err))
	}
	committed, err := paths.switchCurrent(id)
	if err != nil {
		jobErr := fmt.Errorf("%w: select current job: %w", ErrUnavailable, err)
		if !committed {
			return removeTask(jobErr)
		}
		jobErr = failQueuedJob(task, item, jobErr)
		return r.statusLocked(context.WithoutCancel(ctx)), jobErr
	}
	if err := paths.installCurrentAliases(); err != nil {
		jobErr := fmt.Errorf("%w: install current job aliases: %w", ErrUnavailable, err)
		jobErr = failQueuedJob(task, item, jobErr)
		return r.statusLocked(context.WithoutCancel(ctx)), jobErr
	}
	acceptedState := func(item statusFile) State {
		view := stateFromStatus(item, task.logPath)
		view.Available = true
		return view
	}

	cmdCtx, cancel := context.WithTimeout(ctx, updateCommandLimit)
	cmd := exec.CommandContext(cmdCtx, systemdRun,
		"--unit", unit,
		"--property=Type=exec",
		"--property=UMask=0077",
		"--setenv=DASH_HOME="+paths.home,
		"--",
		paths.dashPath,
		"update",
		"execute",
		"--job-id", id,
	)
	out, err := cmd.CombinedOutput()
	cmdErr := cmdCtx.Err()
	cancel()
	if err != nil {
		submissionUncertain := cmdErr != nil
		var startErr error
		if errors.Is(cmdErr, context.DeadlineExceeded) {
			startErr = fmt.Errorf("%w: start systemd unit timed out", ErrUnavailable)
		} else {
			startErr = fmt.Errorf("%w: start systemd unit: %w", ErrUnavailable, err)
		}

		// systemd-run is only the submission client. A timeout or transport error
		// does not prove that systemd rejected the unit, so resolve the unit outcome
		// before writing a terminal failure over a potentially running updater.
		latest, statusReadErr := readUpdateStatusFile(task.statusPath)
		if statusReadErr == nil {
			switch latest.Status {
			case StatusCompleted:
				return acceptedState(latest), nil
			case StatusFailed:
				return acceptedState(latest), startErr
			}
		}
		if submissionUncertain {
			if statusReadErr == nil {
				return acceptedState(latest), nil
			}
			return acceptedState(item), nil
		}
		settleCtx := ctx

		recordFailure := func(extraErr error) (State, error) {
			exitCode := 1
			logErr := os.WriteFile(task.logPath, out, 0o600)
			statusErr := writeUpdateStatusFile(task.statusPath, statusFile{
				ID:                      id,
				Unit:                    unit,
				Status:                  StatusFailed,
				Action:                  plan.Action,
				Channel:                 plan.Channel,
				Origin:                  plan.Origin,
				TargetVersion:           plan.TargetVersion,
				ExpectedCurrentVersion:  plan.ExpectedCurrentVersion,
				ExpectedInstallRevision: plan.ExpectedInstallRevision,
				Phase:                   PhaseQueued,
				FailureCode:             "submission_failed",
				StartedAt:               startedAt,
				FinishedAt:              time.Now().UTC().Format(time.RFC3339),
				ExitCode:                &exitCode,
				LogFile:                 task.logPath,
			})
			return r.statusLocked(settleCtx), errors.Join(startErr, statusReadErr, extraErr, logErr, statusErr)
		}

		unitState, inspectErr := systemdUnitStatus(settleCtx, unit)
		if inspectErr != nil {
			return recordFailure(fmt.Errorf("verify submitted systemd unit: %w", inspectErr))
		}
		if unitState == systemdUnitBusy {
			return r.statusLocked(settleCtx), nil
		}
		if settled, settledErr := readUpdateStatusFile(task.statusPath); settledErr == nil {
			switch settled.Status {
			case StatusCompleted:
				return r.statusLocked(settleCtx), nil
			case StatusFailed:
				return r.statusLocked(settleCtx), startErr
			}
		}
		return recordFailure(nil)
	}

	return acceptedState(item), nil
}

func (r *Runner) resolvePlan(ctx context.Context, in RunInput) (Plan, error) {
	if strings.TrimSpace(string(in.Action)) == "" {
		return Plan{}, fmt.Errorf("%w: action is required", errPlanInput)
	}
	action, ok := ParseAction(in.Action)
	if !ok {
		return Plan{}, fmt.Errorf("%w: invalid action", errPlanInput)
	}
	channel, ok := ParseChannel(in.Channel)
	if !ok {
		return Plan{}, fmt.Errorf("%w: invalid channel", errPlanInput)
	}
	lang, ok := normalizeUpdateLang(in.Lang)
	if !ok {
		return Plan{}, fmt.Errorf("%w: invalid lang", errPlanInput)
	}
	origin := OriginManual
	if strings.TrimSpace(string(in.Origin)) != "" {
		origin, ok = ParseOrigin(in.Origin)
		if !ok {
			return Plan{}, fmt.Errorf("%w: invalid origin", errPlanInput)
		}
	}

	target := strings.TrimSpace(in.TargetVersion)
	expectedVersion := strings.TrimSpace(in.ExpectedCurrentVersion)
	expectedRevision := strings.TrimSpace(in.ExpectedInstallRevision)
	provided := 0
	for _, value := range []string{target, expectedVersion, expectedRevision} {
		if value != "" {
			provided++
		}
	}
	if provided != 0 && provided != 3 {
		return Plan{}, fmt.Errorf("%w: target_version, expected_current_version, and expected_install_revision must be provided together", errPlanInput)
	}
	if provided == 0 {
		check, err := r.Check(ctx, channel)
		if err != nil {
			return Plan{}, err
		}
		target = check.LatestVersion
		expectedVersion = check.CurrentVersion
		expectedRevision = check.InstallRevision
	}

	plan := Plan{
		Action:                  action,
		Channel:                 channel,
		TargetVersion:           target,
		ExpectedCurrentVersion:  expectedVersion,
		ExpectedInstallRevision: expectedRevision,
		Origin:                  origin,
		Lang:                    lang,
		ServiceManager:          "auto",
	}
	if err := plan.validate(); err != nil {
		if provided == 3 {
			return Plan{}, fmt.Errorf("%w: %w", errPlanInput, err)
		}
		return Plan{}, err
	}
	cmp, err := appversion.Compare(plan.TargetVersion, plan.ExpectedCurrentVersion)
	if err != nil {
		if provided == 3 {
			return Plan{}, fmt.Errorf("%w: %w", errPlanInput, err)
		}
		return Plan{}, err
	}
	if action == ActionUpdate && cmp < 0 {
		return Plan{}, fmt.Errorf("%w: target version %s is older than current version %s", errPlanInput, plan.TargetVersion, plan.ExpectedCurrentVersion)
	}
	if action == ActionUpdate && cmp == 0 {
		return Plan{}, fmt.Errorf("%w: target version %s equals current version", ErrAlreadyCurrent, plan.TargetVersion)
	}
	if action == ActionReinstall && cmp < 0 {
		return Plan{}, fmt.Errorf("%w: target version %s is older than current version %s", errPlanInput, plan.TargetVersion, plan.ExpectedCurrentVersion)
	}
	return plan, nil
}

func (r *Runner) Check(ctx context.Context, channel Channel) (Check, error) {
	paths, err := r.paths()
	if err != nil {
		return Check{}, err
	}
	var asset releaseAsset
	var installed installedState
	group, checkCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return r.availableWithPaths(checkCtx, paths)
	})
	group.Go(func() error {
		var err error
		asset, err = r.source.Latest(checkCtx, channel)
		if err != nil {
			return fmt.Errorf("fetch latest Dash release: %w", err)
		}
		return nil
	})
	group.Go(func() error {
		var err error
		installed, err = inspectInstalled(checkCtx, paths.home)
		if err != nil {
			return fmt.Errorf("inspect installed Dash: %w", err)
		}
		return nil
	})
	if err := group.Wait(); err != nil {
		return Check{}, err
	}
	currentChannel, _ := updateChannelForVersion(installed.Version)
	return Check{
		CurrentVersion:     installed.Version,
		CurrentChannel:     currentChannel,
		TargetChannel:      channel,
		LatestVersion:      asset.Version,
		InstallRevision:    installed.Revision,
		VersionStatus:      versionStatus(installed.Version, asset.Version),
		BundledNodeVersion: strings.TrimSpace(appversion.BundledNodeString()),
	}, nil
}

func (r *Runner) availableWithPaths(ctx context.Context, paths runnerPaths) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: check availability: %w", ErrUnavailable, err)
	}
	if err := checkRecoveryState(paths); err != nil {
		return err
	}
	now := time.Now()
	r.availabilityMu.Lock()
	cached := r.availability
	r.availabilityMu.Unlock()
	if cached.paths == paths && now.Before(cached.expiresAt) {
		if cached.err != nil {
			return cached.err
		}
		return checkRecoveryState(paths)
	}

	key := paths.home + "\x00" + paths.dashPath
	ch := r.availabilitySF.DoChan(key, func() (any, error) {
		now := time.Now()
		r.availabilityMu.Lock()
		cached := r.availability
		r.availabilityMu.Unlock()
		if cached.paths == paths && now.Before(cached.expiresAt) {
			return nil, cached.err
		}

		err := checkAvailability(context.WithoutCancel(ctx), paths)
		r.availabilityMu.Lock()
		r.availability = availabilityCache{
			paths:     paths,
			err:       err,
			expiresAt: time.Now().Add(availabilityTTL),
		}
		r.availabilityMu.Unlock()
		return nil, err
	})
	select {
	case result := <-ch:
		if result.Err != nil {
			return result.Err
		}
		return checkRecoveryState(paths)
	case <-ctx.Done():
		return fmt.Errorf("%w: check availability: %w", ErrUnavailable, ctx.Err())
	}
}

func checkAvailability(ctx context.Context, paths runnerPaths) error {
	for _, name := range []string{"systemd-run", "systemctl"} {
		if err := requireCommand(name); err != nil {
			return err
		}
	}
	cmdCtx, cancel := context.WithTimeout(ctx, updateCommandLimit)
	defer cancel()
	if err := requireSystemdManager(cmdCtx); err != nil {
		return err
	}
	info, err := os.Stat(paths.dashPath)
	if err != nil {
		return fmt.Errorf("%w: Dash binary not found: %w", ErrUnavailable, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("%w: Dash binary is not executable", ErrUnavailable)
	}
	version, err := updateCommandVersion(cmdCtx, paths.dashPath)
	if err != nil {
		return fmt.Errorf("%w: inspect Dash binary: %w", ErrUnavailable, err)
	}
	if version != strings.TrimSpace(appversion.CurrentString()) {
		return fmt.Errorf("%w: installed Dash version %s does not match running Dash %s", ErrUnavailable, version, appversion.CurrentString())
	}
	return nil
}

func checkRecoveryState(paths runnerPaths) error {
	pending, err := recoveryStatePending(paths)
	if err != nil {
		return fmt.Errorf("%w: inspect update recovery state: %w", ErrUnavailable, err)
	}
	if pending {
		return fmt.Errorf("%w: unfinished Dash update requires 'dash update recover'", ErrUnavailable)
	}
	return nil
}

func recoveryStatePending(paths runnerPaths) (bool, error) {
	txn, err := readTransaction(paths.transactionPath)
	if err == nil {
		if txn.Phase != PhaseDone {
			return true, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if _, err := os.Lstat(paths.blockPath); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return false, nil
}

func requireSystemdManager(ctx context.Context) error {
	path, err := exec.LookPath("systemctl")
	if err != nil {
		return fmt.Errorf("%w: systemctl not found", ErrUnavailable)
	}
	out, err := exec.CommandContext(ctx, path, "show", "--property=Version", "--value").CombinedOutput()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%w: inspect systemd manager: %w", ErrUnavailable, ctxErr)
	}
	detail := strings.TrimSpace(string(out))
	if err != nil {
		if detail == "" {
			return fmt.Errorf("%w: systemd manager is unavailable: %w", ErrUnavailable, err)
		}
		return fmt.Errorf("%w: systemd manager is unavailable: %s: %w", ErrUnavailable, detail, err)
	}
	if detail == "" {
		return fmt.Errorf("%w: systemd manager returned an empty version", ErrUnavailable)
	}
	return nil
}

func requireCommand(name string) error {
	if _, err := exec.LookPath(name); err == nil {
		return nil
	}
	return fmt.Errorf("%w: %s not found", ErrUnavailable, name)
}

func (r *Runner) paths() (runnerPaths, error) {
	home, err := config.HomeDir()
	if err != nil {
		return runnerPaths{}, fmt.Errorf("%w: resolve home directory: %w", ErrUnavailable, err)
	}
	paths := runnerPathsForHome(home)
	dashPath, err := filepath.Abs(paths.dashPath)
	if err != nil {
		return runnerPaths{}, fmt.Errorf("%w: resolve Dash binary: %w", ErrUnavailable, err)
	}
	paths.dashPath = dashPath
	return paths, nil
}

func runnerPathsForHome(home string) runnerPaths {
	stateDir := filepath.Join(home, "runtime", updateStateDirName)
	return runnerPaths{
		home:            home,
		stateDir:        stateDir,
		jobsDir:         filepath.Join(stateDir, updateJobsDirName),
		currentPath:     filepath.Join(stateDir, updateCurrentName),
		startLockPath:   filepath.Join(stateDir, updateStartLockName),
		statusPath:      filepath.Join(stateDir, updateStatusName),
		logPath:         filepath.Join(stateDir, updateLogName),
		dashPath:        filepath.Join(home, "bin", "dash"),
		transactionPath: filepath.Join(stateDir, updateTransactionName),
		blockPath:       filepath.Join(stateDir, updateBlockName),
	}
}

func (p runnerPaths) job(id string) taskPaths {
	dir := filepath.Join(p.jobsDir, id)
	return taskPaths{
		dir:         dir,
		requestPath: filepath.Join(dir, updateRequestName),
		statusPath:  filepath.Join(dir, updateStatusName),
		logPath:     filepath.Join(dir, updateLogName),
	}
}

func (p runnerPaths) currentTask() taskPaths {
	if _, err := os.Lstat(p.currentPath); err == nil || !errors.Is(err, os.ErrNotExist) {
		return taskPaths{
			dir:         p.currentPath,
			requestPath: filepath.Join(p.currentPath, updateRequestName),
			statusPath:  filepath.Join(p.currentPath, updateStatusName),
			logPath:     filepath.Join(p.currentPath, updateLogName),
		}
	}
	return taskPaths{
		dir:         p.stateDir,
		requestPath: filepath.Join(p.stateDir, updateRequestName),
		statusPath:  p.statusPath,
		logPath:     p.logPath,
	}
}

func (p runnerPaths) switchCurrent(id string) (bool, error) {
	if filepath.Base(id) != id || id == "." || id == ".." {
		return false, fmt.Errorf("invalid job id %q", id)
	}
	return replaceSymlink(p.currentPath, filepath.Join(updateJobsDirName, id))
}

func (p runnerPaths) installCurrentAliases() error {
	_, statusErr := replaceSymlink(p.statusPath, filepath.Join(updateCurrentName, updateStatusName))
	_, logErr := replaceSymlink(p.logPath, filepath.Join(updateCurrentName, updateLogName))
	return errors.Join(statusErr, logErr)
}

func replaceSymlink(path, target string) (bool, error) {
	tmp := path + ".tmp"
	if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := os.Symlink(target, tmp); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return false, errors.Join(err, os.Remove(tmp))
	}
	return true, syncDirectory(filepath.Dir(path))
}

func ParseAction(action Action) (Action, bool) {
	switch Action(strings.TrimSpace(string(action))) {
	case ActionUpdate:
		return ActionUpdate, true
	case ActionReinstall:
		return ActionReinstall, true
	default:
		return ActionUpdate, false
	}
}

func ParseOrigin(origin Origin) (Origin, bool) {
	switch Origin(strings.TrimSpace(string(origin))) {
	case OriginManual:
		return OriginManual, true
	case OriginAuto:
		return OriginAuto, true
	default:
		return OriginManual, false
	}
}

func ParseChannel(channel Channel) (Channel, bool) {
	normalized, err := appversion.ParseChannel(string(channel))
	if err != nil {
		return ChannelRelease, false
	}
	return normalized, true
}

func normalizeUpdateLang(lang string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "zh":
		return "zh", true
	case "en":
		return "en", true
	default:
		return "zh", false
	}
}

func updateChannelForVersion(raw string) (Channel, bool) {
	channel, err := appversion.ChannelFor(raw)
	if err != nil {
		return "", false
	}
	return Channel(channel), true
}

func versionStatus(current, latest string) VersionStatus {
	cmp, err := appversion.Compare(current, latest)
	if err != nil {
		return VersionUnknown
	}
	switch {
	case cmp < 0:
		return VersionAvailable
	case cmp > 0:
		return VersionAhead
	default:
		return VersionCurrent
	}
}

func runningStatusFresh(startedAt string) bool {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(startedAt))
	if err != nil {
		return false
	}
	return time.Since(t) < updateStartGrace
}

func systemdUnitStatus(ctx context.Context, unit string) (systemdUnitState, error) {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return systemdUnitMissing, fmt.Errorf("systemd unit is empty")
	}
	systemctl, err := exec.LookPath("systemctl")
	if err != nil {
		return systemdUnitMissing, fmt.Errorf("find systemctl: %w", err)
	}
	cmdCtx, cancel := context.WithTimeout(ctx, updateCommandLimit)
	out, err := exec.CommandContext(
		cmdCtx,
		systemctl,
		"show",
		"--property=LoadState",
		"--property=ActiveState",
		unit,
	).CombinedOutput()
	cmdErr := cmdCtx.Err()
	cancel()
	if errors.Is(cmdErr, context.DeadlineExceeded) {
		return systemdUnitMissing, fmt.Errorf("inspect systemd unit %q: %w", unit, cmdErr)
	}
	fields, parseErr := parseSystemdUnitProperties(string(out))
	if fields["LoadState"] == "not-found" {
		return systemdUnitMissing, nil
	}
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			return systemdUnitMissing, fmt.Errorf("inspect systemd unit %q: %w", unit, err)
		}
		return systemdUnitMissing, fmt.Errorf("inspect systemd unit %q: %s: %w", unit, detail, err)
	}
	if parseErr != nil {
		return systemdUnitMissing, fmt.Errorf("inspect systemd unit %q: %w", unit, parseErr)
	}
	if fields["LoadState"] == "" {
		return systemdUnitMissing, fmt.Errorf("inspect systemd unit %q: LoadState is empty", unit)
	}
	switch fields["ActiveState"] {
	case "inactive", "failed":
		return systemdUnitStopped, nil
	case "":
		return systemdUnitMissing, fmt.Errorf("inspect systemd unit %q: ActiveState is empty", unit)
	default:
		return systemdUnitBusy, nil
	}
}

func parseSystemdUnitProperties(raw string) (map[string]string, error) {
	fields := make(map[string]string)
	for lineNo, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fields, fmt.Errorf("line %d: expected key=value", lineNo+1)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return fields, fmt.Errorf("line %d: empty key", lineNo+1)
		}
		if _, exists := fields[key]; exists {
			return fields, fmt.Errorf("line %d: duplicate key %q", lineNo+1, key)
		}
		fields[key] = strings.TrimSpace(value)
	}
	return fields, nil
}

func readUpdateStatusFile(path string) (statusFile, error) {
	fields, err := readStatusFields(path)
	if err != nil {
		return statusFile{}, err
	}

	status, ok := parseStatus(Status(fields["status"]))
	if !ok {
		return statusFile{}, fmt.Errorf("invalid status %q", fields["status"])
	}
	id, err := requiredStatusField(fields, "id")
	if err != nil {
		return statusFile{}, err
	}
	unit, err := requiredStatusField(fields, "unit")
	if err != nil {
		return statusFile{}, err
	}
	action, ok := ParseAction(Action(fields["action"]))
	if !ok {
		return statusFile{}, fmt.Errorf("invalid action %q", fields["action"])
	}
	channel, ok := ParseChannel(Channel(fields["channel"]))
	if !ok {
		return statusFile{}, fmt.Errorf("invalid channel %q", fields["channel"])
	}
	var origin Origin
	if raw := strings.TrimSpace(fields["origin"]); raw != "" {
		origin, ok = ParseOrigin(Origin(raw))
		if !ok {
			return statusFile{}, fmt.Errorf("invalid origin %q", raw)
		}
	}
	startedAt, err := requiredStatusField(fields, "started_at")
	if err != nil {
		return statusFile{}, err
	}
	if _, err := time.Parse(time.RFC3339, startedAt); err != nil {
		return statusFile{}, fmt.Errorf("invalid started_at %q: %w", startedAt, err)
	}
	finishedAt := fields["finished_at"]
	if finishedAt != "" {
		if _, err := time.Parse(time.RFC3339, finishedAt); err != nil {
			return statusFile{}, fmt.Errorf("invalid finished_at %q: %w", finishedAt, err)
		}
	}
	item := statusFile{
		ID:                      id,
		Unit:                    unit,
		Status:                  status,
		Action:                  action,
		Channel:                 channel,
		Origin:                  origin,
		TargetVersion:           strings.TrimSpace(fields["target_version"]),
		ExpectedCurrentVersion:  strings.TrimSpace(fields["expected_current_version"]),
		ExpectedInstallRevision: strings.TrimSpace(fields["expected_install_revision"]),
		Phase:                   Phase(strings.TrimSpace(fields["phase"])),
		FailureCode:             strings.TrimSpace(fields["failure_code"]),
		RecoveryPath:            strings.TrimSpace(fields["recovery_path"]),
		StartedAt:               startedAt,
		FinishedAt:              finishedAt,
		LogFile:                 strings.TrimSpace(fields["log_file"]),
	}
	if item.Phase != "" && !validUpdatePhase(item.Phase) {
		return statusFile{}, fmt.Errorf("invalid update phase %q", item.Phase)
	}
	if raw := fields["exit_code"]; raw != "" {
		code, err := strconv.Atoi(raw)
		if err != nil {
			return statusFile{}, fmt.Errorf("invalid exit_code %q: %w", raw, err)
		}
		item.ExitCode = &code
	}
	if raw := fields["executor_pid"]; raw != "" {
		pid, err := strconv.Atoi(raw)
		if err != nil || pid < 0 {
			return statusFile{}, fmt.Errorf("invalid executor_pid %q", raw)
		}
		item.ExecutorPID = pid
	}
	return item, nil
}

func parseStatus(status Status) (Status, bool) {
	switch Status(strings.TrimSpace(string(status))) {
	case StatusRunning:
		return StatusRunning, true
	case StatusCompleted:
		return StatusCompleted, true
	case StatusFailed:
		return StatusFailed, true
	default:
		return StatusIdle, false
	}
}

func requiredStatusField(fields map[string]string, key string) (string, error) {
	value := strings.TrimSpace(fields[key])
	if value == "" {
		return "", fmt.Errorf("missing %s", key)
	}
	return value, nil
}

func readStatusFields(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fields := make(map[string]string)
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected key=value", path, lineNo)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("%s:%d: empty key", path, lineNo)
		}
		if _, exists := fields[key]; exists {
			return nil, fmt.Errorf("%s:%d: duplicate key %q", path, lineNo, key)
		}
		fields[key] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return fields, nil
}

func writeUpdateStatusFile(path string, item statusFile) error {
	var b strings.Builder
	writeStatusField(&b, "id", item.ID)
	writeStatusField(&b, "unit", item.Unit)
	writeStatusField(&b, "status", string(item.Status))
	writeStatusField(&b, "action", string(item.Action))
	writeStatusField(&b, "channel", string(item.Channel))
	writeStatusField(&b, "origin", string(item.Origin))
	writeStatusField(&b, "target_version", item.TargetVersion)
	writeStatusField(&b, "expected_current_version", item.ExpectedCurrentVersion)
	writeStatusField(&b, "expected_install_revision", item.ExpectedInstallRevision)
	writeStatusField(&b, "phase", string(item.Phase))
	writeStatusField(&b, "failure_code", item.FailureCode)
	writeStatusField(&b, "recovery_path", item.RecoveryPath)
	writeStatusField(&b, "started_at", item.StartedAt)
	writeStatusField(&b, "finished_at", item.FinishedAt)
	if item.ExitCode != nil {
		writeStatusField(&b, "exit_code", strconv.Itoa(*item.ExitCode))
	} else {
		writeStatusField(&b, "exit_code", "")
	}
	writeStatusField(&b, "log_file", item.LogFile)
	if item.ExecutorPID > 0 {
		writeStatusField(&b, "executor_pid", strconv.Itoa(item.ExecutorPID))
	} else {
		writeStatusField(&b, "executor_pid", "")
	}

	return writePrivateFile(path, []byte(b.String()))
}

func manualUpdateRunning(pid int) bool {
	if pid <= 1 {
		return false
	}
	exe, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if err != nil {
		return false
	}
	if filepath.Base(strings.TrimSuffix(exe, " (deleted)")) != "dash" {
		return false
	}
	cmdline, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	return err == nil && dashUpdateCommandLine(cmdline)
}

func dashUpdateCommandLine(raw []byte) bool {
	args := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
	return len(args) >= 2 && args[1] == "update"
}

func writeStatusField(b *strings.Builder, key, value string) {
	value = strings.NewReplacer("\n", " ", "\r", " ").Replace(value)
	b.WriteString(key)
	b.WriteByte('=')
	b.WriteString(value)
	b.WriteByte('\n')
}

func readLogTail(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("log path is empty")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("log path is not a regular file")
	}
	if info.Size() > updateLogTailBytes {
		if _, err := f.Seek(-updateLogTailBytes, io.SeekEnd); err != nil {
			return "", err
		}
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\n"), nil
}

func updateUnavailableReason(err error) string {
	reason := err.Error()
	prefix := ErrUnavailable.Error() + ": "
	if strings.HasPrefix(reason, prefix) {
		return strings.TrimPrefix(reason, prefix)
	}
	return reason
}
