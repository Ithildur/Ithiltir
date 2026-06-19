package dashupdate

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dash/internal/infra"
	"dash/internal/lang"
	"dash/internal/model"
	"dash/internal/notify"
	alertstore "dash/internal/store/alert"
	systemstore "dash/internal/store/system"
	kitlog "github.com/Ithildur/EiluneKit/logging"
)

const (
	autoCheckInterval   = 6 * time.Hour
	autoStatusInterval  = 30 * time.Second
	autoStartupDelay    = 30 * time.Second
	autoNotifyTimeout   = 15 * time.Second
	updateAutoStateName = "auto.env"
)

type Service struct {
	system   *systemstore.Store
	alert    *alertstore.Store
	runner   *Runner
	language string
	logger   *kitlog.Helper
}

type autoPolicy struct {
	Channel systemstore.DashUpdateChannel
	Mode    systemstore.DashUpdateMode
}

type autoState struct {
	LastAvailableKey string
	LastStartedID    string
	LastFinishedID   string
}

func NewService(system *systemstore.Store, alert *alertstore.Store, runner *Runner, language string) *Service {
	if runner == nil {
		runner = NewRunner()
	}
	return &Service{
		system:   system,
		alert:    alert,
		runner:   runner,
		language: language,
		logger:   infra.WithModule("dash-update"),
	}
}

func (s *Service) Run(ctx context.Context) error {
	if s == nil || s.system == nil || s.alert == nil || s.runner == nil {
		return fmt.Errorf("dash update service is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
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

	state, err := readAutoStateFile(s.autoStatePath())
	if err != nil {
		return err
	}
	availableKey := fmt.Sprintf("%s:%s", check.TargetChannel, check.LatestVersion)
	if policy.Mode == systemstore.DashUpdateModeNotify {
		if state.LastAvailableKey == availableKey {
			return nil
		}
		if err := s.send(ctx, s.availableMessage(check)); err != nil {
			return err
		}
		state.LastAvailableKey = availableKey
		return writeAutoStateFile(s.autoStatePath(), state)
	}

	if policy.Mode != systemstore.DashUpdateModeAuto {
		return nil
	}

	if err := s.send(ctx, s.startingMessage(check)); err != nil {
		s.logger.Warn("send dash update starting notification failed", err)
	}
	next, err := s.runner.Start(ctx, RunInput{
		Action:  ActionUpdate,
		Channel: Channel(policy.Channel),
		Lang:    s.updateLang(),
	})
	if err != nil {
		if errors.Is(err, ErrRunning) {
			return nil
		}
		notifyErr := s.send(ctx, s.startFailedMessage(check, err))
		if notifyErr != nil {
			return errors.Join(err, notifyErr)
		}
		return err
	}
	if next.ID != "" {
		state.LastAvailableKey = availableKey
		state.LastStartedID = next.ID
		if err := writeAutoStateFile(s.autoStatePath(), state); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) notifyFinishedAutoUpdate(ctx context.Context) {
	state, err := readAutoStateFile(s.autoStatePath())
	if err != nil {
		s.logger.Warn("read dash update auto state failed", err)
		return
	}
	if state.LastStartedID == "" || state.LastFinishedID == state.LastStartedID {
		return
	}

	status := s.runner.Status(ctx)
	if status.ID != state.LastStartedID {
		return
	}
	switch status.Status {
	case StatusCompleted, StatusFailed:
	default:
		return
	}

	if err := s.send(ctx, s.finishedMessage(status)); err != nil {
		s.logger.Warn("send dash update finish notification failed", err)
		return
	}
	state.LastFinishedID = state.LastStartedID
	if err := writeAutoStateFile(s.autoStatePath(), state); err != nil {
		s.logger.Warn("write dash update auto state failed", err)
	}
}

func (s *Service) loadPolicy(ctx context.Context) (autoPolicy, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (autoPolicy, error) {
		channel, err := s.system.GetDashUpdateChannel(c)
		if err != nil {
			return autoPolicy{}, err
		}
		mode, err := s.system.GetDashUpdateMode(c)
		if err != nil {
			return autoPolicy{}, err
		}
		return autoPolicy{
			Channel: channel,
			Mode:    mode,
		}, nil
	})
}

func (s *Service) send(ctx context.Context, msg notify.Message) error {
	channels, err := infra.WithPGReadTimeout(ctx, func(c context.Context) ([]model.NotifyChannel, error) {
		return s.alert.ListDefaultNotifyChannels(c)
	})
	if err != nil {
		return fmt.Errorf("load dash update notify channels: %w", err)
	}
	if len(channels) == 0 {
		return nil
	}

	sendCtx, cancel := context.WithTimeout(ctx, autoNotifyTimeout)
	defer cancel()
	var errs []error
	for _, channel := range channels {
		item := channel
		if err := notify.Send(sendCtx, &item, msg); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *Service) availableMessage(check Check) notify.Message {
	if s.isEnglish() {
		return notify.Message{
			Title: "Dash update available",
			Body: strings.Join([]string{
				"Current: " + check.CurrentVersion,
				"Latest: " + check.LatestVersion,
				"Channel: " + string(check.TargetChannel),
			}, "\n"),
			Metadata: updateMessageMetadata("available", check),
		}
	}
	return notify.Message{
		Title: "Dash 有可用更新",
		Body: strings.Join([]string{
			"当前版本: " + check.CurrentVersion,
			"最新版本: " + check.LatestVersion,
			"更新通道: " + string(check.TargetChannel),
		}, "\n"),
		Metadata: updateMessageMetadata("available", check),
	}
}

func (s *Service) startingMessage(check Check) notify.Message {
	if s.isEnglish() {
		return notify.Message{
			Title: "Dash auto update starting",
			Body: strings.Join([]string{
				"Current: " + check.CurrentVersion,
				"Target: " + check.LatestVersion,
				"Channel: " + string(check.TargetChannel),
			}, "\n"),
			Metadata: updateMessageMetadata("auto_starting", check),
		}
	}
	return notify.Message{
		Title: "Dash 自动更新开始",
		Body: strings.Join([]string{
			"当前版本: " + check.CurrentVersion,
			"目标版本: " + check.LatestVersion,
			"更新通道: " + string(check.TargetChannel),
		}, "\n"),
		Metadata: updateMessageMetadata("auto_starting", check),
	}
}

func (s *Service) startFailedMessage(check Check, startErr error) notify.Message {
	body := updateStartFailedBody(check, startErr, s.isEnglish())
	title := "Dash auto update failed to start"
	if !s.isEnglish() {
		title = "Dash 自动更新启动失败"
	}
	return notify.Message{
		Title:    title,
		Body:     body,
		Metadata: updateMessageMetadata("auto_start_failed", check),
	}
}

func (s *Service) finishedMessage(status State) notify.Message {
	if status.Status == StatusCompleted {
		if s.isEnglish() {
			return notify.Message{
				Title:    "Dash auto update completed",
				Body:     finishedBody(status, true),
				Metadata: statusMessageMetadata("auto_completed", status),
			}
		}
		return notify.Message{
			Title:    "Dash 自动更新完成",
			Body:     finishedBody(status, false),
			Metadata: statusMessageMetadata("auto_completed", status),
		}
	}
	if s.isEnglish() {
		return notify.Message{
			Title:    "Dash auto update failed",
			Body:     finishedBody(status, true),
			Metadata: statusMessageMetadata("auto_failed", status),
		}
	}
	return notify.Message{
		Title:    "Dash 自动更新失败",
		Body:     finishedBody(status, false),
		Metadata: statusMessageMetadata("auto_failed", status),
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
		"kind":    "dash_update",
		"event":   event,
		"job_id":  status.ID,
		"status":  string(status.Status),
		"action":  string(status.Action),
		"channel": string(status.Channel),
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

func (s *Service) autoStatePath() string {
	return filepath.Join(s.runner.stateDir(), updateAutoStateName)
}

func readAutoStateFile(path string) (autoState, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return autoState{}, nil
	}
	if err != nil {
		return autoState{}, err
	}
	defer f.Close()

	fields := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return autoState{}, err
	}
	return autoState{
		LastAvailableKey: fields["last_available_key"],
		LastStartedID:    fields["last_started_id"],
		LastFinishedID:   fields["last_finished_id"],
	}, nil
}

func writeAutoStateFile(path string, state autoState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	var b strings.Builder
	writeStatusField(&b, "last_available_key", state.LastAvailableKey)
	writeStatusField(&b, "last_started_id", state.LastStartedID)
	writeStatusField(&b, "last_finished_id", state.LastFinishedID)
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
