package dashupdate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dash/internal/infra"
	"dash/internal/lang"
	"dash/internal/notify"
	systemstore "dash/internal/store/system"
	kitlog "github.com/Ithildur/EiluneKit/logging"
)

const (
	autoCheckInterval   = 6 * time.Hour
	autoStatusInterval  = 30 * time.Second
	autoStartupDelay    = 30 * time.Second
	updateAutoStateName = "auto.env"
	updateHandledName   = "finish.handled"
	updateNotifiedName  = "finish.notified"
)

type Service struct {
	system        *systemstore.Store
	notifications notificationQueue
	runner        *Runner
	language      string
	logger        *kitlog.Helper
}

type notificationQueue interface {
	EnqueueDefault(context.Context, string, notify.Messages) (notify.EnqueueStatus, error)
}

type autoState struct {
	LastAvailableKey string
	LastStartedID    string
	LastFinishedID   string
}

type finishedAutoJob struct {
	status      State
	handledPath string
}

func NewService(
	system *systemstore.Store,
	notifications notificationQueue,
	runner *Runner,
	language string,
) *Service {
	return &Service{
		system:        system,
		notifications: notifications,
		runner:        runner,
		language:      language,
		logger:        infra.WithModule("dash-update"),
	}
}

func (s *Service) Run(ctx context.Context) error {
	if s == nil || s.system == nil || s.notifications == nil || s.runner == nil {
		return fmt.Errorf("dash update service is not initialized")
	}
	if ctx == nil {
		return fmt.Errorf("dash update context is nil")
	}

	startup := time.NewTimer(autoStartupDelay)
	defer startup.Stop()
	checkTicker := time.NewTicker(autoCheckInterval)
	defer checkTicker.Stop()
	statusTicker := time.NewTicker(autoStatusInterval)
	defer statusTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-startup.C:
			s.tick(ctx)
		case <-checkTicker.C:
			s.tick(ctx)
		case <-statusTicker.C:
			s.notifyFinishedAutoUpdate(ctx)
		}
	}
}

func (s *Service) tick(ctx context.Context) {
	s.notifyFinishedAutoUpdate(ctx)
	if err := s.checkAndAct(ctx); err != nil {
		s.logger.Warn("dash update auto check failed", err)
	}
}

func (s *Service) checkAndAct(ctx context.Context) error {
	policy, err := s.loadPolicy(ctx)
	if err != nil {
		return err
	}
	if policy.Mode == systemstore.DashUpdateModeManual {
		return nil
	}

	status := s.runner.Status(ctx)
	if status.Status == StatusRunning {
		return nil
	}

	check, err := s.runner.Check(ctx, Channel(policy.Channel))
	if err != nil {
		return fmt.Errorf("check dash update: %w", err)
	}
	if check.VersionStatus != VersionAvailable {
		return nil
	}

	availableKey := fmt.Sprintf("%s:%s", check.TargetChannel, check.LatestVersion)
	if policy.Mode == systemstore.DashUpdateModeNotify {
		autoStatePath, err := s.autoStatePath()
		if err != nil {
			return err
		}
		state, err := readAutoStateFile(autoStatePath)
		if err != nil {
			return err
		}
		if state.LastAvailableKey == availableKey {
			return nil
		}
		if err := s.enqueueNotification(
			ctx,
			"dash-update:available:"+availableKey,
			availableMessages(check),
		); err != nil {
			return err
		}
		state.LastAvailableKey = availableKey
		return writeAutoStateFile(autoStatePath, state)
	}

	if policy.Mode != systemstore.DashUpdateModeAuto {
		return nil
	}

	next, err := s.runner.Start(ctx, RunInput{
		Action:                  ActionUpdate,
		Channel:                 Channel(policy.Channel),
		Lang:                    s.updateLang(),
		Origin:                  OriginAuto,
		TargetVersion:           check.LatestVersion,
		ExpectedCurrentVersion:  check.CurrentVersion,
		ExpectedInstallRevision: check.InstallRevision,
	})
	if err != nil {
		if errors.Is(err, ErrRunning) {
			return nil
		}
		if next.ID != "" && next.Origin == OriginAuto && isTerminalUpdateStatus(next.Status) {
			s.notifyFinishedAutoUpdate(ctx)
			return err
		}
		notifyErr := s.enqueueNotification(
			ctx,
			"dash-update:auto-start-failed:"+availableKey,
			startFailedMessages(check, err),
		)
		if notifyErr != nil {
			return errors.Join(err, notifyErr)
		}
		return err
	}
	return nil
}

func (s *Service) notifyFinishedAutoUpdate(ctx context.Context) {
	paths, err := s.runner.paths()
	if err != nil {
		s.logger.Warn("resolve dash update state path failed", err)
		return
	}
	current := s.runner.Status(ctx)
	jobs, scanErr := pendingFinishedAutoJobs(paths)
	if scanErr != nil {
		s.logger.Warn("scan finished dash auto updates failed", scanErr)
	}
	for _, job := range jobs {
		err := s.enqueueNotification(
			ctx,
			"dash-update:finished:"+job.status.ID,
			finishedMessages(job.status),
		)
		if err != nil {
			s.logger.Warn("enqueue dash update finish notification failed", err)
			continue
		}
		if err := markAutoJobHandled(job.handledPath); err != nil {
			s.logger.Warn("mark dash update finish notification handled failed", err)
		}
	}
	s.notifyLegacyFinishedAutoUpdate(ctx, paths, current)
	if err := cleanupUpdateJobs(paths, current.ID); err != nil {
		s.logger.Warn("clean up finished dash update jobs failed", err)
	}
}

func (s *Service) notifyLegacyFinishedAutoUpdate(ctx context.Context, paths runnerPaths, status State) {
	autoStatePath := filepath.Join(paths.stateDir, updateAutoStateName)
	state, err := readAutoStateFile(autoStatePath)
	if err != nil {
		s.logger.Warn("read legacy dash update auto state failed", err)
		return
	}
	if state.LastStartedID == "" || state.LastFinishedID == state.LastStartedID {
		return
	}
	if status.ID != state.LastStartedID {
		return
	}
	switch status.Status {
	case StatusCompleted, StatusFailed:
	default:
		return
	}

	if err := s.enqueueNotification(
		ctx,
		"dash-update:finished:"+state.LastStartedID,
		finishedMessages(status),
	); err != nil {
		s.logger.Warn("enqueue dash update finish notification failed", err)
		return
	}
	state.LastFinishedID = state.LastStartedID
	if err := writeAutoStateFile(autoStatePath, state); err != nil {
		s.logger.Warn("write legacy dash update auto state failed", err)
	}
}

func (s *Service) loadPolicy(ctx context.Context) (systemstore.DashUpdatePolicy, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (systemstore.DashUpdatePolicy, error) {
		return s.system.GetDashUpdatePolicy(c)
	})
}

func (s *Service) enqueueNotification(ctx context.Context, key string, messages notify.Messages) error {
	status, err := s.notifications.EnqueueDefault(ctx, key, messages)
	switch status {
	case notify.EnqueueQueued, notify.EnqueueSkippedNoTargets:
		if err != nil {
			s.logger.Warn("notification request handled with warning", err)
		}
		return nil
	default:
		if err != nil {
			return fmt.Errorf("enqueue dash update notification: %w", err)
		}
		return fmt.Errorf("enqueue dash update notification returned invalid status %q", status)
	}
}

func availableMessages(check Check) notify.Messages {
	metadata := updateMessageMetadata("available", check)
	return notify.Messages{
		Chinese: notify.Message{
			Title: "Dash 有可用更新",
			Body: strings.Join([]string{
				"当前版本: " + check.CurrentVersion,
				"最新版本: " + check.LatestVersion,
				"更新通道: " + string(check.TargetChannel),
			}, "\n"),
			Metadata: metadata,
		},
		English: notify.Message{
			Title: "Dash update available",
			Body: strings.Join([]string{
				"Current: " + check.CurrentVersion,
				"Latest: " + check.LatestVersion,
				"Channel: " + string(check.TargetChannel),
			}, "\n"),
			Metadata: metadata,
		},
	}
}

func startFailedMessages(check Check, startErr error) notify.Messages {
	metadata := updateMessageMetadata("auto_start_failed", check)
	return notify.Messages{
		Chinese: notify.Message{
			Title:    "Dash 自动更新启动失败",
			Body:     updateStartFailedBody(check, startErr, false),
			Metadata: metadata,
		},
		English: notify.Message{
			Title:    "Dash auto update failed to start",
			Body:     updateStartFailedBody(check, startErr, true),
			Metadata: metadata,
		},
	}
}

func finishedMessages(status State) notify.Messages {
	if status.Status == StatusCompleted {
		metadata := statusMessageMetadata("auto_completed", status)
		return notify.Messages{
			Chinese: notify.Message{
				Title:    "Dash 自动更新完成",
				Body:     finishedBody(status, false),
				Metadata: metadata,
			},
			English: notify.Message{
				Title:    "Dash auto update completed",
				Body:     finishedBody(status, true),
				Metadata: metadata,
			},
		}
	}
	metadata := statusMessageMetadata("auto_failed", status)
	return notify.Messages{
		Chinese: notify.Message{
			Title:    "Dash 自动更新失败",
			Body:     finishedBody(status, false),
			Metadata: metadata,
		},
		English: notify.Message{
			Title:    "Dash auto update failed",
			Body:     finishedBody(status, true),
			Metadata: metadata,
		},
	}
}

func updateStartFailedBody(check Check, err error, english bool) string {
	if english {
		return strings.Join([]string{
			"Current: " + check.CurrentVersion,
			"Target: " + check.LatestVersion,
			"Channel: " + string(check.TargetChannel),
			"Error: " + err.Error(),
		}, "\n")
	}
	return strings.Join([]string{
		"当前版本: " + check.CurrentVersion,
		"目标版本: " + check.LatestVersion,
		"更新通道: " + string(check.TargetChannel),
		"错误: " + err.Error(),
	}, "\n")
}

func finishedBody(status State, english bool) string {
	var lines []string
	if english {
		lines = append(lines,
			"Action: "+string(status.Action),
			"Channel: "+string(status.Channel),
			"Started: "+status.StartedAt,
			"Finished: "+status.FinishedAt,
		)
		if status.ExitCode != nil {
			lines = append(lines, fmt.Sprintf("Exit code: %d", *status.ExitCode))
		}
		if status.TargetVersion != "" {
			lines = append(lines, "Target: "+status.TargetVersion)
		}
		if status.Status == StatusFailed && status.FailureCode != "" {
			lines = append(lines, "Failure: "+status.FailureCode)
		}
		if status.RecoveryPath != "" {
			lines = append(lines, "Recovery files: "+status.RecoveryPath)
		}
		if status.Status == StatusFailed && status.LogTail != "" {
			lines = append(lines, "", "Recent log:", status.LogTail)
		}
		return strings.Join(lines, "\n")
	}

	lines = append(lines,
		"操作: "+string(status.Action),
		"更新通道: "+string(status.Channel),
		"开始时间: "+status.StartedAt,
		"结束时间: "+status.FinishedAt,
	)
	if status.ExitCode != nil {
		lines = append(lines, fmt.Sprintf("退出码: %d", *status.ExitCode))
	}
	if status.TargetVersion != "" {
		lines = append(lines, "目标版本: "+status.TargetVersion)
	}
	if status.Status == StatusFailed && status.FailureCode != "" {
		lines = append(lines, "失败代码: "+status.FailureCode)
	}
	if status.RecoveryPath != "" {
		lines = append(lines, "恢复文件: "+status.RecoveryPath)
	}
	if status.Status == StatusFailed && status.LogTail != "" {
		lines = append(lines, "", "最近日志:", status.LogTail)
	}
	return strings.Join(lines, "\n")
}

func updateMessageMetadata(event string, check Check) map[string]string {
	return map[string]string{
		"kind":            "dash_update",
		"event":           event,
		"current_version": check.CurrentVersion,
		"latest_version":  check.LatestVersion,
		"channel":         string(check.TargetChannel),
	}
}

func statusMessageMetadata(event string, status State) map[string]string {
	return map[string]string{
		"kind":           "dash_update",
		"event":          event,
		"job_id":         status.ID,
		"status":         string(status.Status),
		"action":         string(status.Action),
		"channel":        string(status.Channel),
		"target_version": status.TargetVersion,
		"failure_code":   status.FailureCode,
	}
}

func (s *Service) updateLang() string {
	if s.isEnglish() {
		return "en"
	}
	return "zh"
}

func (s *Service) isEnglish() bool {
	return lang.Normalize(s.language) == lang.English
}

func (s *Service) autoStatePath() (string, error) {
	paths, err := s.runner.paths()
	if err != nil {
		return "", err
	}
	return filepath.Join(paths.stateDir, updateAutoStateName), nil
}

func pendingFinishedAutoJobs(paths runnerPaths) ([]finishedAutoJob, error) {
	entries, err := os.ReadDir(paths.jobsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	jobs := make([]finishedAutoJob, 0, len(entries))
	var scanErr error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		task := paths.job(entry.Name())
		item, err := readUpdateStatusFile(task.statusPath)
		if err != nil {
			scanErr = errors.Join(scanErr, fmt.Errorf("read update job %q: %w", entry.Name(), err))
			continue
		}
		if item.ID != entry.Name() {
			scanErr = errors.Join(scanErr, fmt.Errorf("update job directory %q contains id %q", entry.Name(), item.ID))
			continue
		}
		if item.Origin != OriginAuto || !isTerminalUpdateStatus(item.Status) {
			continue
		}
		handled, err := autoJobHandled(task.dir)
		if err != nil {
			scanErr = errors.Join(scanErr, fmt.Errorf("inspect update job %q notification marker: %w", item.ID, err))
			continue
		}
		if handled {
			continue
		}
		jobs = append(jobs, finishedAutoJob{
			status:      stateFromStatus(item, task.logPath),
			handledPath: filepath.Join(task.dir, updateHandledName),
		})
	}
	return jobs, scanErr
}

func autoJobHandled(dir string) (bool, error) {
	for _, name := range []string{updateHandledName, updateNotifiedName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
}

func markAutoJobHandled(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return f.Close()
}

func cleanupUpdateJobs(paths runnerPaths, currentID string) error {
	if currentID == "" {
		return nil
	}
	entries, err := os.ReadDir(paths.jobsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	var cleanupErr error
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == currentID {
			continue
		}
		task := paths.job(entry.Name())
		item, err := readUpdateStatusFile(task.statusPath)
		if err != nil || item.ID != entry.Name() || !isTerminalUpdateStatus(item.Status) {
			continue
		}
		switch item.Origin {
		case OriginManual:
		case OriginAuto:
			handled, err := autoJobHandled(task.dir)
			if err != nil || !handled {
				continue
			}
		default:
			continue
		}
		if err := os.RemoveAll(task.dir); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove update job %q: %w", item.ID, err))
		}
	}
	return cleanupErr
}

func isTerminalUpdateStatus(status Status) bool {
	return status == StatusCompleted || status == StatusFailed
}

func readAutoStateFile(path string) (autoState, error) {
	fields, err := readStatusFields(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return autoState{}, nil
		}
		return autoState{}, err
	}
	return autoState{
		LastAvailableKey: fields["last_available_key"],
		LastStartedID:    fields["last_started_id"],
		LastFinishedID:   fields["last_finished_id"],
	}, nil
}

func writeAutoStateFile(path string, state autoState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	var b strings.Builder
	writeStatusField(&b, "last_available_key", state.LastAvailableKey)
	writeStatusField(&b, "last_started_id", state.LastStartedID)
	writeStatusField(&b, "last_finished_id", state.LastFinishedID)
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
