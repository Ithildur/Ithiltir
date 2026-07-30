//go:build linux

package dashupdate

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dash/internal/config"
	appversion "dash/internal/version"
)

const updateUsageExitCode = 2

type jobReporter struct {
	mu         sync.Mutex
	statusPath string
	out        io.Writer
	item       statusFile
}

type manualUpdateOptions struct {
	action         Action
	channel        Channel
	lang           string
	serviceManager string
	checkOnly      bool
	assumeYes      bool
	help           bool
}

// RunCommand runs the native Dash update command-line interface.
func RunCommand(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	if len(args) == 1 && (args[0] == "--version" || args[0] == "-v" || args[0] == "version") {
		fmt.Fprintln(stdout, appversion.CurrentString())
		return 0
	}
	if len(args) == 1 && args[0] == "--node-version" {
		fmt.Fprintln(stdout, appversion.BundledNodeString())
		return 0
	}
	if len(args) > 0 && args[0] == "execute" {
		jobID, err := parseExecuteArgs(args[1:])
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return updateUsageExitCode
		}
		reported, err := executeJob(ctx, jobID, stdout)
		if err != nil {
			if !reported {
				fmt.Fprintln(stderr, "error:", err)
			}
			return 1
		}
		return 0
	}
	if len(args) > 0 && args[0] == "recover" {
		if len(args) != 1 {
			fmt.Fprintln(stderr, "error: recover does not accept arguments")
			return updateUsageExitCode
		}
		if err := requireUpdateRoot(); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		home, err := updateHome()
		if err == nil {
			var paths installPaths
			paths, err = newInstallPaths(home)
			if err == nil {
				err = recoverUpdate(ctx, paths, stdout)
			}
		}
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		fmt.Fprintln(stdout, "Dash update recovery completed.")
		return 0
	}

	opts, err := parseManualUpdateArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		printUpdateUsage(stderr, defaultUpdateLang())
		return updateUsageExitCode
	}
	if opts.help {
		printUpdateUsage(stdout, opts.lang)
		return 0
	}
	if err := runManualUpdate(ctx, opts, stdin, stdout); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

func parseExecuteArgs(args []string) (string, error) {
	if len(args) != 2 || args[0] != "--job-id" || !validJobID(args[1]) {
		return "", errors.New("usage: dash update execute --job-id <id>")
	}
	return args[1], nil
}

func parseManualUpdateArgs(args []string) (manualUpdateOptions, error) {
	opts := manualUpdateOptions{
		action:         ActionUpdate,
		channel:        ChannelRelease,
		lang:           defaultUpdateLang(),
		serviceManager: "auto",
	}
	actionSet := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "update" || arg == "reinstall":
			if actionSet {
				return manualUpdateOptions{}, errors.New("only one command can be specified")
			}
			opts.action = Action(arg)
			actionSet = true
		case arg == "check" || arg == "--check":
			opts.checkOnly = true
		case arg == "--test":
			opts.channel = ChannelPrerelease
		case arg == "-y" || arg == "--yes":
			opts.assumeYes = true
		case arg == "-h" || arg == "--help":
			opts.help = true
		case arg == "--lang" || arg == "--service-manager":
			if i+1 >= len(args) {
				return manualUpdateOptions{}, fmt.Errorf("missing value for %s", arg)
			}
			i++
			if arg == "--lang" {
				opts.lang = strings.TrimSpace(args[i])
			} else {
				opts.serviceManager = strings.TrimSpace(args[i])
			}
		case strings.HasPrefix(arg, "--lang="):
			opts.lang = strings.TrimSpace(strings.TrimPrefix(arg, "--lang="))
		case strings.HasPrefix(arg, "--service-manager="):
			opts.serviceManager = strings.TrimSpace(strings.TrimPrefix(arg, "--service-manager="))
		case strings.HasPrefix(arg, "-"):
			return manualUpdateOptions{}, fmt.Errorf("unknown option %q", arg)
		default:
			return manualUpdateOptions{}, fmt.Errorf("unknown command %q", arg)
		}
	}
	if _, ok := normalizeUpdateLang(opts.lang); !ok {
		return manualUpdateOptions{}, fmt.Errorf("invalid language %q", opts.lang)
	}
	switch opts.serviceManager {
	case "auto", "systemd", "none":
	default:
		return manualUpdateOptions{}, fmt.Errorf("invalid service manager %q", opts.serviceManager)
	}
	return opts, nil
}

func runManualUpdate(ctx context.Context, opts manualUpdateOptions, stdin io.Reader, stdout io.Writer) error {
	home, err := updateHome()
	if err != nil {
		return err
	}
	installed, err := inspectInstalled(ctx, home)
	if err != nil {
		return err
	}
	asset, err := newReleaseSource().Latest(ctx, opts.channel)
	if err != nil {
		return fmt.Errorf("fetch latest Dash release: %w", err)
	}
	currentChannel, err := appversion.ChannelFor(installed.Version)
	if err != nil {
		return err
	}
	cmp, err := appversion.Compare(asset.Version, installed.Version)
	if err != nil {
		return err
	}

	printUpdateSelection(stdout, opts.lang, installed.Version, currentChannel, opts.channel, asset.Version, opts.action)
	if opts.channel == ChannelRelease && currentChannel == appversion.ChannelPrerelease && cmp < 0 {
		return errors.New(updateText(
			opts.lang,
			"当前部署的 prerelease 高于最新 release；请使用 --test 保持测试通道",
			"the installed prerelease is newer than the latest release; use --test to stay on the test channel",
		))
	}
	if opts.action == ActionUpdate && cmp <= 0 {
		fmt.Fprintln(stdout, updateText(opts.lang, "Dash 已是最新版本。", "Dash is up to date."))
		return nil
	}
	if opts.action == ActionReinstall && cmp < 0 {
		return fmt.Errorf("refusing to reinstall older target %s over %s", asset.Version, installed.Version)
	}
	if opts.checkOnly {
		return nil
	}
	if !opts.assumeYes && !confirmManualUpdate(stdin, stdout, opts, installed.Version, asset.Version) {
		fmt.Fprintln(stdout, updateText(opts.lang, "已取消操作。", "Operation canceled."))
		return nil
	}
	if err := requireUpdateRoot(); err != nil {
		return err
	}

	paths, err := newInstallPaths(home)
	if err != nil {
		return err
	}
	request := jobRequest{
		Plan: Plan{
			Action:                  opts.action,
			Channel:                 opts.channel,
			TargetVersion:           asset.Version,
			ExpectedCurrentVersion:  installed.Version,
			ExpectedInstallRevision: installed.Revision,
			Origin:                  OriginManual,
			Lang:                    opts.lang,
			ServiceManager:          opts.serviceManager,
		},
	}
	return runManualJob(ctx, paths, request, stdout)
}

func executeJob(ctx context.Context, id string, stdout io.Writer) (bool, error) {
	if err := requireUpdateRoot(); err != nil {
		return false, err
	}
	home, err := updateHome()
	if err != nil {
		return false, err
	}
	paths, err := newInstallPaths(home)
	if err != nil {
		return false, err
	}
	runnerPaths := runnerPathsForHome(home)
	task := runnerPaths.job(id)
	request, err := readJobRequest(task.requestPath)
	if err != nil {
		return false, fmt.Errorf("read update request: %w", err)
	}
	if request.ID != id {
		return false, fmt.Errorf("update request ID is %q, want %q", request.ID, id)
	}
	reporter, closeLog, err := openJobReporter(task, request, stdout)
	if err != nil {
		return false, err
	}
	defer closeLog()
	return true, applyUpdate(ctx, paths, request, reporter)
}

func runManualJob(ctx context.Context, paths installPaths, request jobRequest, stdout io.Writer) error {
	runnerPaths := runnerPathsForHome(paths.home)
	if err := os.MkdirAll(runnerPaths.jobsDir, 0o700); err != nil {
		return fmt.Errorf("create update state directory: %w", err)
	}
	startLock, err := acquireStartLock(runnerPaths.startLockPath)
	if err != nil {
		return err
	}
	defer startLock.Close()

	var runner Runner
	current, err := runner.taskState(ctx, runnerPaths)
	if err != nil {
		return fmt.Errorf("inspect current update job: %w", err)
	}
	if current.Status == StatusRunning {
		return ErrRunning
	}
	updateLock, err := acquireUpdateLock(paths.lockFile)
	if err != nil {
		return err
	}
	defer updateLock.Close()

	now := time.Now().UTC()
	request.ID = newUpdateJobID(now)
	request.Unit = "manual-" + request.ID
	task := runnerPaths.job(request.ID)
	if err := os.Mkdir(task.dir, 0o700); err != nil {
		return err
	}
	removeTask := true
	defer func() {
		if removeTask {
			_ = os.RemoveAll(task.dir)
		}
	}()
	if err := writePrivateFile(task.logPath, nil); err != nil {
		return err
	}
	if err := writeJobRequest(task.requestPath, request); err != nil {
		return err
	}
	item := statusForRequest(request, task.logPath, now)
	if err := writeUpdateStatusFile(task.statusPath, item); err != nil {
		return err
	}
	committed, err := runnerPaths.switchCurrent(request.ID)
	if committed {
		removeTask = false
	}
	if err != nil {
		jobErr := fmt.Errorf("select current update job: %w", err)
		if committed {
			return failQueuedJob(task, item, jobErr)
		}
		return jobErr
	}
	if err := runnerPaths.installCurrentAliases(); err != nil {
		return failQueuedJob(task, item, fmt.Errorf("install current update aliases: %w", err))
	}
	reporter, closeLog, err := openJobReporter(task, request, stdout)
	if err != nil {
		return err
	}
	defer closeLog()
	return runUpdateLocked(ctx, paths, request, reporter)
}

func openJobReporter(task taskPaths, request jobRequest, stdout io.Writer) (*jobReporter, func(), error) {
	item, err := readUpdateStatusFile(task.statusPath)
	if err != nil {
		return nil, func() {}, fmt.Errorf("read update status: %w", err)
	}
	if item.ID != request.ID || item.Unit != request.Unit || item.Status != StatusRunning {
		return nil, func() {}, errors.New("update status does not match the queued request")
	}
	item.ExecutorPID = os.Getpid()
	if err := writeUpdateStatusFile(task.statusPath, item); err != nil {
		return nil, func() {}, fmt.Errorf("record update executor: %w", err)
	}
	log, err := os.OpenFile(task.logPath, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open update log: %w", err)
	}
	out := io.Writer(log)
	if stdout != nil {
		out = io.MultiWriter(log, stdout)
	}
	return &jobReporter{statusPath: task.statusPath, out: out, item: item}, func() { _ = log.Close() }, nil
}

func (r *jobReporter) printf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.out != nil {
		_, _ = fmt.Fprintf(r.out, format, args...)
	}
}

func (r *jobReporter) phase(phase Phase) error {
	if !validUpdatePhase(phase) || phase == PhaseDone {
		return fmt.Errorf("invalid running update phase %q", phase)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if updatePhaseOrder(phase) < updatePhaseOrder(r.item.Phase) {
		return fmt.Errorf("update phase cannot move from %s back to %s", r.item.Phase, phase)
	}
	r.item.Phase = phase
	return writeUpdateStatusFile(r.statusPath, r.item)
}

func (r *jobReporter) setRecoveryPath(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.item.RecoveryPath = filepath.Clean(path)
	if err := writeUpdateStatusFile(r.statusPath, r.item); err != nil && r.out != nil {
		_, _ = fmt.Fprintf(r.out, "warning: persist recovery path: %v\n", err)
	}
	if r.out != nil {
		_, _ = fmt.Fprintf(r.out, "recovery required: run '<DASH_HOME>/bin/dash update recover' as root\nrecovery files: %s\n", r.item.RecoveryPath)
	}
}

func (r *jobReporter) finish(updateErr error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	code := 0
	r.item.Status = StatusCompleted
	r.item.FailureCode = ""
	if updateErr != nil {
		code = 1
		r.item.Status = StatusFailed
		r.item.FailureCode = updateFailureCode(updateErr, r.item.RecoveryPath)
		if r.out != nil {
			_, _ = fmt.Fprintf(r.out, "error: %v\n", updateErr)
		}
	} else {
		r.item.Phase = PhaseDone
	}
	r.item.ExitCode = &code
	r.item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	return writeUpdateStatusFile(r.statusPath, r.item)
}

func updateFailureCode(err error, recoveryPath string) string {
	switch {
	case strings.TrimSpace(recoveryPath) != "", errors.Is(err, ErrRecoveryRequired):
		return "recovery_required"
	case errors.Is(err, ErrUpdateConflict):
		return "install_changed"
	case errors.Is(err, ErrRunning):
		return "update_running"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "update_interrupted"
	default:
		return "update_failed"
	}
}

func updateHome() (string, error) {
	if strings.TrimSpace(os.Getenv("DASH_HOME")) != "" {
		return config.HomeDir()
	}
	if exe, err := os.Executable(); err == nil {
		if resolved, resolveErr := filepath.EvalSymlinks(exe); resolveErr == nil {
			exe = resolved
		}
		if home := homeFromDashExecutable(exe); home != "" {
			return home, nil
		}
	}
	home, err := config.HomeDir()
	if err != nil {
		return "", err
	}
	if parent := filepath.Dir(home); filepath.Base(parent) == "releases" {
		home = filepath.Dir(parent)
	}
	return home, nil
}

func homeFromDashExecutable(exe string) string {
	exe = filepath.Clean(exe)
	binDir := filepath.Dir(exe)
	if filepath.Base(binDir) != "bin" {
		return ""
	}
	parent := filepath.Dir(binDir)
	if releases := filepath.Dir(parent); filepath.Base(releases) == "releases" {
		return filepath.Dir(releases)
	}
	if info, err := os.Stat(filepath.Join(parent, "configs")); err == nil && info.IsDir() {
		return parent
	}
	return ""
}

func requireUpdateRoot() error {
	if os.Geteuid() != 0 {
		return errors.New("root privileges are required to update Dash")
	}
	return nil
}

func confirmManualUpdate(stdin io.Reader, stdout io.Writer, opts manualUpdateOptions, current, target string) bool {
	if opts.action == ActionReinstall {
		fmt.Fprintf(stdout, updateText(opts.lang, "是否重新安装 Dash %s（当前 %s）？[y/N] ", "Reinstall Dash %s over current %s? [y/N] "), target, current)
	} else {
		fmt.Fprintf(stdout, updateText(opts.lang, "是否将 Dash 从 %s 更新到 %s？[y/N] ", "Update Dash from %s to %s? [y/N] "), current, target)
	}
	reader := bufio.NewReader(stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

func printUpdateSelection(w io.Writer, lang string, current string, currentChannel, targetChannel appversion.Channel, latest string, action Action) {
	fmt.Fprintf(w, updateText(lang, "当前版本：%s\n", "current: %s\n"), current)
	fmt.Fprintf(w, updateText(lang, "当前通道：%s\n", "current channel: %s\n"), currentChannel)
	fmt.Fprintf(w, updateText(lang, "目标通道：%s\n", "target channel: %s\n"), targetChannel)
	fmt.Fprintf(w, updateText(lang, "最新版本：%s\n", "latest: %s\n"), latest)
	if action == ActionReinstall {
		fmt.Fprintln(w, updateText(lang, "操作：重新安装目标版本", "action: reinstall target version"))
	} else {
		fmt.Fprintln(w, updateText(lang, "操作：更新", "action: update"))
	}
}

func printUpdateUsage(w io.Writer, lang string) {
	if lang == "zh" {
		fmt.Fprintln(w, `用法：
  dash update [--check] [--test] [-y|--yes] [--lang zh|en] [--service-manager auto|systemd|none]
  dash update reinstall [--check] [--test] [-y|--yes] [--lang zh|en] [--service-manager auto|systemd|none]
  dash update recover

命令：
  update       默认命令；仅当目标通道有更高版本时安装
  reinstall    即使版本号相同，也重新安装目标通道最新包
  recover      恢复未完成的更新事务

选项：
  --check      只检查版本，不安装
  --test       选择最新 prerelease；默认选择最新 release
  -y, --yes    不交互确认
  --lang       输出语言：zh 或 en
  --service-manager  已安装的运行方式；默认自动检测
  -h, --help   显示帮助`)
		return
	}
	fmt.Fprintln(w, `Usage:
  dash update [--check] [--test] [-y|--yes] [--lang zh|en] [--service-manager auto|systemd|none]
  dash update reinstall [--check] [--test] [-y|--yes] [--lang zh|en] [--service-manager auto|systemd|none]
  dash update recover

Commands:
  update       Default command; install only when the target channel has a newer version
  reinstall    Reinstall the latest target-channel package even when its version is unchanged
  recover      Resume or roll back an unfinished update transaction

Options:
  --check      Check versions without installing
  --test       Select the latest prerelease; the default is the latest release
  -y, --yes    Do not ask for confirmation
  --lang       Output language: zh or en
  --service-manager  Installed runtime mode; defaults to auto detection
  -h, --help   Show this help`)
}

func defaultUpdateLang() string {
	if lang := strings.TrimSpace(os.Getenv("SCRIPT_LANG")); lang == "zh" || lang == "en" {
		return lang
	}
	locale := strings.ToLower(strings.TrimSpace(os.Getenv("LANG")))
	if strings.HasPrefix(locale, "zh") {
		return "zh"
	}
	return "en"
}

func updateText(lang, zh, en string) string {
	if lang == "zh" {
		return zh
	}
	return en
}
