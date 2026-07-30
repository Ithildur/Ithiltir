package alert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"dash/internal/model"
	alertstore "dash/internal/store/alert"
	kitlog "github.com/Ithildur/EiluneKit/logging"
)

var errInvalidControlTask = errors.New("invalid alert control task")

func (s *Service) runControlLoop(ctx context.Context) error {
	ticker := time.NewTicker(controlPollInterval)
	defer ticker.Stop()

	for {
		processed, err := s.processControlTasks(ctx)
		if err != nil {
			s.logger.Warn("process control task failed", err)
			ticker.Reset(controlPollInterval)
		} else if processed {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (s *Service) processControlTasks(ctx context.Context) (bool, error) {
	processed := false
	for {
		now := time.Now().UTC()
		task, err := s.store.TakeNextControlTask(ctx, now)
		if err != nil {
			return processed, err
		}
		if task == nil {
			return processed, nil
		}
		processed = true

		if err := s.controlTask(ctx, task); err != nil {
			if errors.Is(err, errInvalidControlTask) {
				if failErr := s.store.FailControlTask(ctx, task.ID, err.Error()); failErr != nil {
					return processed, fmt.Errorf("fail invalid control task %d: %w", task.ID, failErr)
				}
				s.logger.Warn("control task permanently failed", err, kitlog.Int64("task_id", task.ID), kitlog.String("task_type", task.TaskType))
				continue
			}
			next := now.Add(controlTaskRetryDelay(task.AttemptCount))
			retryErr := s.store.RetryControlTask(ctx, task.ID, next, err.Error())
			if retryErr != nil {
				return processed, fmt.Errorf("retry control task %d: %w", task.ID, retryErr)
			}
			s.logger.Warn("control task failed", err, kitlog.Int64("task_id", task.ID), kitlog.String("task_type", task.TaskType))
			continue
		}
		if err := s.store.CompleteControlTask(ctx, task.ID); err != nil {
			return processed, fmt.Errorf("complete control task %d: %w", task.ID, err)
		}
	}
}

func (s *Service) controlTask(ctx context.Context, task *model.AlertControlTask) error {
	if task == nil {
		return fmt.Errorf("%w: nil task", errInvalidControlTask)
	}
	compiled, err := s.cache.Refresh(ctx, true)
	if err != nil {
		return err
	}

	affected := make(map[int64]struct{})
	switch task.TaskType {
	case alertstore.ControlTaskRuleChange:
		var payload alertstore.RuleChangePayload
		if err := json.Unmarshal(task.Payload, &payload); err != nil {
			return fmt.Errorf("%w: decode rule_change payload: %w", errInvalidControlTask, err)
		}
		if payload.RuleID == 0 {
			return fmt.Errorf("%w: rule_change rule_id is required", errInvalidControlTask)
		}
		if payload.OldGeneration > 0 && payload.CloseReason != "" {
			closed, err := s.closeGeneration(ctx, payload.RuleID, payload.OldGeneration, payload.CloseReason)
			if err != nil {
				return err
			}
			mergeServerIDSet(affected, closed)
		}
		closed, err := s.closeInvalid(ctx, compiled.Invalid)
		if err != nil {
			return err
		}
		mergeServerIDSet(affected, closed)
		s.markServersDirty(affected)
		return s.enqueueTargets(ctx, false)
	case alertstore.ControlTaskFullReconcile:
		closed, err := s.closeInvalid(ctx, compiled.Invalid)
		if err != nil {
			return err
		}
		mergeServerIDSet(affected, closed)
		closed, err = s.closeDeletedServers(ctx)
		if err != nil {
			return err
		}
		mergeServerIDSet(affected, closed)
		if err := s.restoreOpenRuntime(ctx); err != nil {
			return err
		}
		s.markServersDirty(affected)
		return s.enqueueTargets(ctx, true)
	default:
		return fmt.Errorf("%w: unknown task type %q", errInvalidControlTask, task.TaskType)
	}
}

func (s *Service) runFullReconcileTicker(ctx context.Context) error {
	ticker := time.NewTicker(fullReconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.enqueueReconcile(ctx); err != nil {
				s.logger.Warn("enqueue periodic full reconcile failed", err)
			}
		}
	}
}

func (s *Service) enqueueReconcile(ctx context.Context) error {
	return s.store.EnqueueFullReconcileTask(ctx, "full_reconcile:global")
}

func (s *Service) closeGeneration(ctx context.Context, ruleID, generation int64, reason string) (map[int64]struct{}, error) {
	events, err := s.store.ListOpenEventsByRule(ctx, ruleID, generation)
	if err != nil {
		return nil, err
	}
	affected := make(map[int64]struct{})
	for _, event := range events {
		if event.ObjectType == model.ObjectTypeServer && event.ObjectID > 0 {
			affected[event.ObjectID] = struct{}{}
		}
		closedAt := time.Now().UTC()
		if _, err := s.store.WriteCloseTransition(ctx, alertstore.AlertCloseEventParams{
			EventID:     event.ID,
			ClosedAt:    closedAt,
			CloseReason: reason,
		}); err != nil {
			return nil, err
		}
	}
	return affected, nil
}

func (s *Service) closeInvalid(ctx context.Context, invalid []InvalidRule) (map[int64]struct{}, error) {
	affected := make(map[int64]struct{})
	for _, item := range invalid {
		closed, err := s.closeGeneration(ctx, item.RuleID, item.Generation, "rule_invalid")
		if err != nil {
			return nil, err
		}
		mergeServerIDSet(affected, closed)
	}
	return affected, nil
}

func (s *Service) closeDeletedServers(ctx context.Context) (map[int64]struct{}, error) {
	events, err := s.store.ListOpenEventsForDeletedServers(ctx)
	if err != nil {
		return nil, err
	}
	affected := make(map[int64]struct{})
	drops := make(map[int64]map[string]struct{})
	for _, event := range events {
		if event.ObjectID <= 0 {
			continue
		}
		outcome, err := s.store.WriteCloseTransition(ctx, alertstore.AlertCloseEventParams{
			EventID:     event.ID,
			ClosedAt:    time.Now().UTC(),
			CloseReason: "server_deleted",
		})
		if err != nil {
			return nil, err
		}
		if shouldDropRuntimeAfterClose(outcome) {
			affected[event.ObjectID] = struct{}{}
			keys := drops[event.ObjectID]
			if keys == nil {
				keys = make(map[string]struct{})
				drops[event.ObjectID] = keys
			}
			keys[ruleStateKey(event.RuleID, event.RuleGeneration)] = struct{}{}
		}
	}
	return affected, s.dropRuntimeKeys(ctx, drops)
}

func (s *Service) dropRuntimeKeys(ctx context.Context, drops map[int64]map[string]struct{}) error {
	for serverID, keys := range drops {
		if serverID <= 0 || len(keys) == 0 {
			continue
		}
		current, err := s.loadRuntimeState(ctx, serverID)
		if err != nil {
			return err
		}
		next := make(map[string]RuntimeState, len(current))
		changed := false
		for key, state := range current {
			if _, drop := keys[key]; drop {
				changed = true
				continue
			}
			next[key] = state
		}
		if !changed {
			continue
		}
		if err := saveRuntimeState(ctx, s.store, serverID, current, next); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) enqueueTargets(ctx context.Context, includeOpenEvents bool) error {
	targets := make(map[int64]struct{})

	currentIDs, err := s.front.ListCurrentNodeIDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range currentIDs {
		targets[id] = struct{}{}
	}

	runtimeIDs, err := s.store.ListAlertRuntimeServerIDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range runtimeIDs {
		if id > 0 {
			targets[id] = struct{}{}
		}
	}

	if includeOpenEvents {
		openIDs, err := s.store.ListOpenObjectIDs(ctx, model.ObjectTypeServer)
		if err != nil {
			return err
		}
		for _, id := range openIDs {
			if id > 0 {
				targets[id] = struct{}{}
			}
		}
	}

	for serverID := range targets {
		s.store.MarkServerDirty(serverID)
	}
	return nil
}

func (s *Service) markServersDirty(ids map[int64]struct{}) {
	for serverID := range ids {
		s.store.MarkServerDirty(serverID)
	}
}
