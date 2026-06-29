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
)

const (
	StatusIdle      Status = "idle"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"

	ActionUpdate    Action = "update"
	ActionReinstall Action = "reinstall"

	ChannelRelease    Channel = appversion.ChannelRelease
	ChannelPrerelease Channel = appversion.ChannelPrerelease

	updateStateDirName = "dash-update"
	updateStatusName   = "status.env"
	updateLogName      = "update.log"
	updateRunnerName   = "run.sh"
	updateLogTailBytes = 16 * 1024
	updateStartGrace   = 15 * time.Second
	updateCommandLimit = 10 * time.Second
	updateRemoteURL    = "https://github.com/Ithildur/Ithiltir.git"
)

var (
	ErrRunning        = errors.New("dash update is running")
	ErrUnavailable    = errors.New("dash update unavailable")
	ErrInvalidRequest = errors.New("invalid dash update request")
)

type Status string
type Action string
type Channel = appversion.Channel
type VersionStatus string

type Runner struct {
	mu sync.Mutex
}

const (
	VersionAvailable VersionStatus = "available"
	VersionCurrent   VersionStatus = "current"
	VersionAhead     VersionStatus = "ahead"
	VersionUnknown   VersionStatus = "unknown"
)

type RunInput struct {
	Action  Action
	Channel Channel
	Lang    string
}

type State struct {
	ID                string  `json:"id,omitempty"`
	Status            Status  `json:"status"`
	Action            Action  `json:"action,omitempty"`
	Channel           Channel `json:"channel,omitempty"`
	StartedAt         string  `json:"started_at,omitempty"`
	FinishedAt        string  `json:"finished_at,omitempty"`
	ExitCode          *int    `json:"exit_code,omitempty"`
	LogTail           string  `json:"log_tail,omitempty"`
	Available         bool    `json:"available"`
	UnavailableReason string  `json:"unavailable_reason,omitempty"`
}

type statusFile struct {
	ID         string
	Unit       string
	Status     Status
	Action     Action
	Channel    Channel
	StartedAt  string
	FinishedAt string
	ExitCode   *int
	LogFile    string
}

type runnerPaths struct {
	home       string
	stateDir   string
	statusPath string
	logPath    string
	runnerPath string
	scriptPath string
}

type Check struct {
	CurrentVersion     string        `json:"current_version"`
	CurrentChannel     Channel       `json:"current_channel,omitempty"`
	TargetChannel      Channel       `json:"target_channel"`
	LatestVersion      string        `json:"latest_version"`
	VersionStatus      VersionStatus `json:"version_status"`
	BundledNodeVersion string        `json:"bundled_node_version"`
}

func NewRunner() *Runner {
	return &Runner{}
}

func (r *Runner) Status(ctx context.Context) State {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.statusLocked(ctx)
}

func (r *Runner) statusLocked(ctx context.Context) State {
	view := State{Status: StatusIdle}
	paths, pathErr := r.paths()
	var statusErr error

	if pathErr == nil {
		item, err := readUpdateStatusFile(paths.statusPath)
		switch {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			statusErr = err
		default:
			item.LogFile = paths.logPath
			var reconcileErr error
			item, reconcileErr = r.reconcileRunningStatus(ctx, item, paths)
			if reconcileErr != nil {
				statusErr = reconcileErr
			}
			view.ID = item.ID
			view.Status = item.Status
			view.Action = item.Action
			view.Channel = item.Channel
			view.StartedAt = item.StartedAt
			view.FinishedAt = item.FinishedAt
			view.ExitCode = item.ExitCode
			view.LogTail = readLogTail(paths.logPath)
		}
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
	} else if err := r.availableWithPaths(paths); err != nil {
		view.Available = false
		view.UnavailableReason = updateUnavailableReason(err)
	} else {
		view.Available = true
	}
	return view
}

func (r *Runner) reconcileRunningStatus(ctx context.Context, item statusFile, paths runnerPaths) (statusFile, error) {
	if item.Status != StatusRunning || runningStatusFresh(item.StartedAt) {
		return item, nil
	}
	active, ok := systemdUnitActive(ctx, item.Unit)
	if !ok || active {
		return item, nil
	}
	item.Status = StatusFailed
	item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	item.ExitCode = nil
	if err := writeUpdateStatusFile(paths.statusPath, item); err != nil {
		return item, fmt.Errorf("write reconciled update status: %w", err)
	}
	return item, nil
}

func (r *Runner) Start(ctx context.Context, in RunInput) (State, error) {
	if strings.TrimSpace(string(in.Action)) == "" {
		return r.Status(ctx), fmt.Errorf("%w: action is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(string(in.Channel)) == "" {
		return r.Status(ctx), fmt.Errorf("%w: channel is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(in.Lang) == "" {
		return r.Status(ctx), fmt.Errorf("%w: lang is required", ErrInvalidRequest)
	}

	action, ok := ParseAction(in.Action)
	if !ok {
		return r.Status(ctx), fmt.Errorf("%w: invalid action", ErrInvalidRequest)
	}
	channel, ok := ParseChannel(in.Channel)
	if !ok {
		return r.Status(ctx), fmt.Errorf("%w: invalid channel", ErrInvalidRequest)
	}
	lang, ok := normalizeUpdateLang(in.Lang)
	if !ok {
		return r.Status(ctx), fmt.Errorf("%w: invalid lang", ErrInvalidRequest)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current := r.statusLocked(ctx)
	if current.Status == StatusRunning {
		return current, ErrRunning
	}
	paths, err := r.paths()
	if err != nil {
		return current, err
	}
	if err := r.availableWithPaths(paths); err != nil {
		return current, err
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		return current, fmt.Errorf("%w: create state dir: %v", ErrUnavailable, err)
	}

	systemdRun, err := exec.LookPath("systemd-run")
	if err != nil {
		return current, fmt.Errorf("%w: systemd-run not found", ErrUnavailable)
	}
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		return current, fmt.Errorf("%w: bash not found", ErrUnavailable)
	}

	now := time.Now().UTC()
	id := now.Format("20060102T150405Z")
	startedAt := now.Format(time.RFC3339)
	unit := "ithiltir-dash-update-" + id

	if err := os.WriteFile(paths.runnerPath, []byte(updateRunnerScript), 0o755); err != nil {
		return current, fmt.Errorf("%w: write runner script: %v", ErrUnavailable, err)
	}
	if err := os.WriteFile(paths.logPath, nil, 0o600); err != nil {
		return current, fmt.Errorf("%w: initialize log: %v", ErrUnavailable, err)
	}
	if err := writeUpdateStatusFile(paths.statusPath, statusFile{
		ID:        id,
		Unit:      unit,
		Status:    StatusRunning,
		Action:    action,
		Channel:   channel,
		StartedAt: startedAt,
		LogFile:   paths.logPath,
	}); err != nil {
		return current, fmt.Errorf("%w: write status: %v", ErrUnavailable, err)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, updateCommandLimit)
	cmd := exec.CommandContext(cmdCtx, systemdRun,
		"--unit", unit,
		"--property=Type=exec",
		"--setenv=DASH_HOME="+paths.home,
		bashPath,
		paths.runnerPath,
		paths.statusPath,
		paths.logPath,
		paths.scriptPath,
		string(action),
		string(channel),
		lang,
		id,
		startedAt,
		unit,
	)
	out, err := cmd.CombinedOutput()
	cmdErr := cmdCtx.Err()
	cancel()
	if err != nil {
		exitCode := 1
		logErr := os.WriteFile(paths.logPath, out, 0o600)
		statusErr := writeUpdateStatusFile(paths.statusPath, statusFile{
			ID:         id,
			Unit:       unit,
			Status:     StatusFailed,
			Action:     action,
			Channel:    channel,
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC().Format(time.RFC3339),
			ExitCode:   &exitCode,
			LogFile:    paths.logPath,
		})
		var startErr error
		if errors.Is(cmdErr, context.DeadlineExceeded) {
			startErr = fmt.Errorf("%w: start systemd unit timed out", ErrUnavailable)
		} else {
			startErr = fmt.Errorf("%w: start systemd unit: %v", ErrUnavailable, err)
		}
		return r.statusLocked(ctx), errors.Join(startErr, logErr, statusErr)
	}

	return r.statusLocked(ctx), nil
}

func (r *Runner) Check(ctx context.Context, channel Channel) (Check, error) {
	if err := r.available(); err != nil {
		return Check{}, err
	}

	latest, err := latestRemoteVersion(ctx, channel)
	if err != nil {
		return Check{}, err
	}

	current := strings.TrimSpace(appversion.CurrentString())
	currentChannel, _ := updateChannelForVersion(current)
	return Check{
		CurrentVersion:     current,
		CurrentChannel:     currentChannel,
		TargetChannel:      channel,
		LatestVersion:      latest,
		VersionStatus:      versionStatus(current, latest),
		BundledNodeVersion: strings.TrimSpace(appversion.BundledNodeString()),
	}, nil
}

func (r *Runner) available() error {
	paths, err := r.paths()
	if err != nil {
		return err
	}
	return r.availableWithPaths(paths)
}

func (r *Runner) availableWithPaths(paths runnerPaths) error {
	if err := requireCommand("systemd-run"); err != nil {
		return err
	}
	if err := requireCommand("bash"); err != nil {
		return err
	}
	if err := requireCommand("git"); err != nil {
		return err
	}
	if err := requireCommand("tar"); err != nil {
		return err
	}
	if err := requireCommand("systemctl"); err != nil {
		return err
	}
	if !hasCommand("curl") && !hasCommand("wget") {
		return fmt.Errorf("%w: curl or wget not found", ErrUnavailable)
	}
	info, err := os.Stat(paths.scriptPath)
	if err != nil {
		return fmt.Errorf("%w: update_dash_linux.sh not found: %v", ErrUnavailable, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%w: update_dash_linux.sh is a directory", ErrUnavailable)
	}
	return nil
}

func requireCommand(name string) error {
	if hasCommand(name) {
		return nil
	}
	return fmt.Errorf("%w: %s not found", ErrUnavailable, name)
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (r *Runner) paths() (runnerPaths, error) {
	home, err := config.HomeDir()
	if err != nil {
		return runnerPaths{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	stateDir := filepath.Join(home, "runtime", updateStateDirName)
	scriptPath, err := filepath.Abs(filepath.Join(home, "update_dash_linux.sh"))
	if err != nil {
		return runnerPaths{}, fmt.Errorf("%w: resolve update_dash_linux.sh: %v", ErrUnavailable, err)
	}
	return runnerPaths{
		home:       home,
		stateDir:   stateDir,
		statusPath: filepath.Join(stateDir, updateStatusName),
		logPath:    filepath.Join(stateDir, updateLogName),
		runnerPath: filepath.Join(stateDir, updateRunnerName),
		scriptPath: scriptPath,
	}, nil
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

func latestRemoteVersion(ctx context.Context, channel Channel) (string, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("%w: git not found", ErrUnavailable)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, updateCommandLimit)
	out, err := exec.CommandContext(
		cmdCtx,
		git,
		"ls-remote",
		"--tags",
		"--refs",
		updateRemoteURL,
	).Output()
	cmdErr := cmdCtx.Err()
	cancel()
	if errors.Is(cmdErr, context.DeadlineExceeded) {
		return "", fmt.Errorf("fetch dash update tags timed out: %w", cmdErr)
	}
	if err != nil {
		return "", fmt.Errorf("fetch dash update tags: %w", err)
	}

	latest, ok := latestVersionFromRefs(string(out), channel)
	if !ok {
		return "", fmt.Errorf("no %s dash update tags found", channel)
	}
	return latest, nil
}

func latestVersionFromRefs(refs string, channel Channel) (string, bool) {
	var versions []string
	for _, line := range strings.Split(refs, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		tag := strings.TrimPrefix(fields[1], "refs/tags/")
		versions = append(versions, tag)
	}
	return appversion.LatestInChannel(versions, channel)
}

func runningStatusFresh(startedAt string) bool {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(startedAt))
	if err != nil {
		return false
	}
	return time.Since(t) < updateStartGrace
}

func systemdUnitActive(ctx context.Context, unit string) (bool, bool) {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return false, false
	}
	systemctl, err := exec.LookPath("systemctl")
	if err != nil {
		return false, false
	}
	cmdCtx, cancel := context.WithTimeout(ctx, updateCommandLimit)
	err = exec.CommandContext(cmdCtx, systemctl, "is-active", "--quiet", unit).Run()
	cmdErr := cmdCtx.Err()
	cancel()
	if errors.Is(cmdErr, context.DeadlineExceeded) {
		return false, false
	}
	if err == nil {
		return true, true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, true
	}
	return false, false
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
	startedAt, err := requiredStatusField(fields, "started_at")
	if err != nil {
		return statusFile{}, err
	}
	item := statusFile{
		ID:         id,
		Unit:       unit,
		Status:     status,
		Action:     action,
		Channel:    channel,
		StartedAt:  startedAt,
		FinishedAt: fields["finished_at"],
	}
	if raw := fields["exit_code"]; raw != "" {
		code, err := strconv.Atoi(raw)
		if err != nil {
			return statusFile{}, fmt.Errorf("invalid exit_code %q: %w", raw, err)
		}
		item.ExitCode = &code
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
		fields[key] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return fields, nil
}

func writeUpdateStatusFile(path string, item statusFile) error {
	tmp := path + ".tmp"
	var b strings.Builder
	writeStatusField(&b, "id", item.ID)
	writeStatusField(&b, "unit", item.Unit)
	writeStatusField(&b, "status", string(item.Status))
	writeStatusField(&b, "action", string(item.Action))
	writeStatusField(&b, "channel", string(item.Channel))
	writeStatusField(&b, "started_at", item.StartedAt)
	writeStatusField(&b, "finished_at", item.FinishedAt)
	if item.ExitCode != nil {
		writeStatusField(&b, "exit_code", strconv.Itoa(*item.ExitCode))
	} else {
		writeStatusField(&b, "exit_code", "")
	}
	writeStatusField(&b, "log_file", item.LogFile)

	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func writeStatusField(b *strings.Builder, key, value string) {
	value = strings.NewReplacer("\n", " ", "\r", " ").Replace(value)
	b.WriteString(key)
	b.WriteByte('=')
	b.WriteString(value)
	b.WriteByte('\n')
}

func readLogTail(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return ""
	}
	if info.Size() > updateLogTailBytes {
		if _, err := f.Seek(-updateLogTailBytes, io.SeekEnd); err != nil {
			return ""
		}
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(data), "\n")
}

func updateUnavailableReason(err error) string {
	reason := err.Error()
	prefix := ErrUnavailable.Error() + ": "
	if strings.HasPrefix(reason, prefix) {
		return strings.TrimPrefix(reason, prefix)
	}
	return reason
}

const updateRunnerScript = `#!/usr/bin/env bash
set -euo pipefail
umask 077

status_file="$1"
log_file="$2"
script_path="$3"
action="$4"
channel="$5"
lang="$6"
id="$7"
started_at="$8"
unit="$9"

write_status() {
  local status="$1"
  local finished_at="$2"
  local exit_code="$3"
  local tmp="${status_file}.tmp"
  {
    printf 'id=%s\n' "$id"
    printf 'unit=%s\n' "$unit"
    printf 'status=%s\n' "$status"
    printf 'action=%s\n' "$action"
    printf 'channel=%s\n' "$channel"
    printf 'started_at=%s\n' "$started_at"
    printf 'finished_at=%s\n' "$finished_at"
    printf 'exit_code=%s\n' "$exit_code"
    printf 'log_file=%s\n' "$log_file"
  } > "$tmp"
  mv "$tmp" "$status_file"
}

write_status "running" "" ""

args=("bash" "$script_path")
if [[ "$action" == "reinstall" ]]; then
  args+=("reinstall")
fi
if [[ "$channel" == "prerelease" ]]; then
  args+=("--test")
fi
args+=("--yes" "--lang" "$lang")

set +e
"${args[@]}" > "$log_file" 2>&1
code=$?
set -e

finished_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
if [[ "$code" -eq 0 ]]; then
  write_status "completed" "$finished_at" "$code"
else
  write_status "failed" "$finished_at" "$code"
fi
exit "$code"
`
