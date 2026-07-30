package alert

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dash/internal/metrics"
	"dash/internal/model"
	alertstore "dash/internal/store/alert"
	kitlog "github.com/Ithildur/EiluneKit/logging"
)

func (s *Service) runEvalWorker(ctx context.Context, workerID int) error {
	for {
		serverID, snapshot, ok := s.store.NextDirtyServer(ctx)
		if !ok {
			return nil
		}

		err := s.processServer(ctx, serverID, snapshot)
		if err != nil {
			s.logger.Warn("process server alert reconcile failed", err,
				kitlog.Int("worker", workerID),
				kitlog.Int64("server_id", serverID),
			)
			if !waitEvalRetry(ctx) {
				s.store.FinishDirtyServer(serverID, true)
				return nil
			}
			s.store.FinishDirtyServer(serverID, true)
			continue
		}
		s.store.FinishDirtyServer(serverID, false)
	}
}

func waitEvalRetry(ctx context.Context) bool {
	timer := time.NewTimer(evalRetryDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (s *Service) processServer(ctx context.Context, serverID int64, snapshot *metrics.NodeView) error {
	compiled, cacheErr := s.cache.Refresh(ctx, false)
	if compiled == nil {
		if cacheErr != nil {
			return fmt.Errorf("refresh rule cache: %w", cacheErr)
		}
		return errors.New("alert rule cache returned no compiled rules")
	}
	if cacheErr != nil {
		s.logger.Warn("refresh rule cache failed during evaluation", cacheErr)
	}

	current, err := s.loadRuntimeState(ctx, serverID)
	if err != nil {
		return err
	}
	if snapshot == nil {
		var err error
		snapshot, err = s.front.FetchCurrentNode(ctx, serverID, s.staleAfterSec)
		if err != nil {
			return err
		}
	}
	mounts, err := s.store.RuleMountsForServer(ctx, serverID)
	if err != nil {
		return err
	}
	serverRules := compiled.ForMounts(mounts)

	now := time.Now().UTC()
	result := EvaluateServer(serverID, snapshot, serverRules, current, now)
	result.OpenTransitions = filterStartupOpens(result.OpenTransitions, now, s.openAfter)
	closingStateKeys := make(map[string]struct{}, len(result.CloseTransitions))
	for _, transition := range result.CloseTransitions {
		message := buildCloseMessage(transition, s.message)
		notifications, notifyErr := s.closeNotificationParams(ctx, transition, message)
		if notifyErr != nil {
			s.logNotificationTargetError(notifyErr, serverID, transition.StateKey, notifications)
			if errors.Is(notifyErr, errNotificationTargetsUnavailable) {
				continue
			}
		}
		outcome, err := s.store.WriteCloseTransition(ctx, alertstore.AlertCloseEventParams{
			EventID:        transition.EventID,
			RuleID:         transition.Rule.RuleID,
			RuleGeneration: transition.Rule.Generation,
			ObjectType:     model.ObjectTypeServer,
			ObjectID:       transition.ObjectID,
			ClosedAt:       transition.ClosedAt,
			CurrentValue:   transition.CurrentValue,
			CloseReason:    transition.CloseReason,
			Notifications:  notifications,
		})
		if err != nil {
			if errors.Is(err, alertstore.ErrAlertRuleVersionStale) {
				delete(result.Next, transition.StateKey)
				continue
			}
			s.logger.Warn("write close transition failed", err, kitlog.Int64("server_id", serverID), kitlog.String("state_key", transition.StateKey))
			continue
		}
		closingStateKeys[transition.StateKey] = struct{}{}
		if shouldDropRuntimeAfterClose(outcome) {
			delete(result.Next, transition.StateKey)
			if cooldownAfterClose(transition) && outcome.Status == alertstore.CloseStatusClosed {
				result.Next[transition.StateKey] = newCooldownState(transition.Rule, transition.ClosedAt, time.Now().UTC())
			}
		}
	}

	for _, transition := range result.OpenTransitions {
		ruleSnapshot, err := transition.Rule.snapshotJSON()
		if err != nil {
			return fmt.Errorf("prepare open transition: %w", err)
		}
		message := buildOpenMessage(transition, s.message)
		notifications, notifyErr := s.openNotificationParams(ctx, transition, message)
		if notifyErr != nil {
			s.logNotificationTargetError(notifyErr, serverID, transition.StateKey, notifications)
			if errors.Is(notifyErr, errNotificationTargetsUnavailable) {
				continue
			}
		}
		outcome, err := s.store.WriteOpenTransition(ctx, alertstore.AlertOpenEventParams{
			RuleID:             transition.Rule.RuleID,
			RuleGeneration:     transition.Rule.Generation,
			Builtin:            transition.Rule.Builtin,
			RuleSnapshot:       ruleSnapshot,
			ObjectType:         model.ObjectTypeServer,
			ObjectID:           transition.ObjectID,
			TriggeredAt:        transition.TriggeredAt,
			CurrentValue:       transition.CurrentValue,
			EffectiveThreshold: transition.EffectiveThreshold,
			Title:              message.Title,
			Message:            message.Body,
			Notifications:      notifications,
		})
		if err != nil {
			if errors.Is(err, alertstore.ErrAlertRuleVersionStale) {
				continue
			}
			s.logger.Warn("write open transition failed", err, kitlog.Int64("server_id", serverID), kitlog.String("state_key", transition.StateKey))
			continue
		}
		if outcome.EventID > 0 {
			applyOpenTransition(result.Next, transition, outcome.EventID)
		}
	}

	s.flushHeartbeats(ctx, current, result.Next, closingStateKeys, serverID)
	return saveRuntimeState(ctx, s.store, serverID, current, result.Next)
}

func (s *Service) logNotificationTargetError(err error, serverID int64, stateKey string, notifications []alertstore.AlertNotificationParams) {
	if errors.Is(err, errNotificationTargetsUnavailable) {
		s.logger.Warn("alert notification targets unavailable; deferring transition", err, kitlog.Int64("server_id", serverID), kitlog.String("state_key", stateKey))
		return
	}
	s.logger.Warn("load alert notification targets failed; using cached notification targets", err, kitlog.Int64("server_id", serverID), kitlog.String("state_key", stateKey))
}

func (s *Service) flushHeartbeats(ctx context.Context, current, next map[string]RuntimeState, closingStateKeys map[string]struct{}, serverID int64) {
	for key, state := range next {
		previous, ok := current[key]
		_, closing := closingStateKeys[key]
		if !ok || !shouldHeartbeatFiring(previous, state, firingHeartbeatInterval, closing) {
			continue
		}
		found, err := s.store.TouchOpenEvent(ctx, state.EventID, state.LastObservedAtTime(), state.CurrentValue, state.EffectiveThreshold)
		if err != nil {
			s.logger.Warn("touch firing alert failed", err, kitlog.Int64("server_id", serverID), kitlog.String("state_key", key))
			continue
		}
		if !found {
			delete(next, key)
			continue
		}
		state.LastDBHeartbeatAt = state.LastObservedAt
		next[key] = state
	}
}

func filterStartupOpens(transitions []OpenTransition, now, openAfter time.Time) []OpenTransition {
	if openAfter.IsZero() || !now.Before(openAfter) {
		return transitions
	}
	return transitions[:0]
}
