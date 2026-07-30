package alert

import (
	"context"
	"fmt"
	"time"

	"dash/internal/infra"
	"dash/internal/metrics"
	alertstore "dash/internal/store/alert"
	"dash/internal/store/frontcache"
	kitlog "github.com/Ithildur/EiluneKit/logging"
	"golang.org/x/sync/errgroup"
)

const (
	defaultEvalWorkers       = 4
	ruleCacheMinRefresh      = 5 * time.Second
	controlPollInterval      = 1 * time.Second
	notificationPollInterval = 1 * time.Second
	fullReconcileInterval    = 1 * time.Minute
	startupAlertGrace        = 1 * time.Minute
	evalRetryDelay           = 5 * time.Second
	notificationRetryLimit   = 7
	firingHeartbeatInterval  = 1 * time.Minute
)

type Service struct {
	store         *alertstore.Store
	front         *frontcache.Store
	cache         *RuleCache
	notify        *notifyCache
	logger        *kitlog.Helper
	evalWorkers   int
	message       MessageConfig
	openAfter     time.Time
	staleAfterSec int
}

func NewService(st *alertstore.Store, front *frontcache.Store, message MessageConfig, offlineThreshold time.Duration) (*Service, error) {
	if st == nil {
		return nil, fmt.Errorf("alert store is nil")
	}
	if front == nil {
		return nil, fmt.Errorf("front cache is nil")
	}
	if message.Location == nil {
		return nil, fmt.Errorf("alert message location is nil")
	}
	if offlineThreshold <= 0 {
		return nil, fmt.Errorf("alert offline threshold must be positive")
	}
	return &Service{
		store:         st,
		front:         front,
		cache:         NewRuleCache(st, ruleCacheMinRefresh),
		notify:        newNotifyCache(st, ruleCacheMinRefresh),
		logger:        infra.WithModule("alert"),
		evalWorkers:   defaultEvalWorkers,
		message:       messageConfig([]MessageConfig{message}),
		openAfter:     time.Now().UTC().Add(startupAlertGrace),
		staleAfterSec: metrics.DurationSecondsCeil(offlineThreshold),
	}, nil
}

func (s *Service) Run(ctx context.Context) error {
	if s == nil || s.store == nil || s.front == nil || s.cache == nil || s.notify == nil {
		return fmt.Errorf("alert service is not initialized")
	}
	if ctx == nil {
		return fmt.Errorf("alert service context is nil")
	}
	if _, err := s.cache.Refresh(ctx, true); err != nil {
		return fmt.Errorf("refresh alert rule cache: %w", err)
	}
	if err := s.rebuildRuntimeFromOpenEvents(ctx); err != nil {
		return fmt.Errorf("rebuild alert runtime from open events: %w", err)
	}
	if err := s.store.EnqueueFullReconcileTask(ctx, "full_reconcile:global"); err != nil {
		return fmt.Errorf("enqueue full alert reconcile: %w", err)
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error { return s.runControlLoop(groupCtx) })
	group.Go(func() error { return s.runNotificationLoop(groupCtx) })
	group.Go(func() error { return s.runFullReconcileTicker(groupCtx) })
	for i := 0; i < s.evalWorkers; i++ {
		workerID := i
		group.Go(func() error { return s.runEvalWorker(groupCtx, workerID) })
	}
	return group.Wait()
}

func controlTaskRetryDelay(attempt int32) time.Duration {
	seconds := 1 << minInt(int(attempt), 6)
	return time.Duration(seconds) * time.Second
}

func notificationRetryDelay(attempt int32) time.Duration {
	shift := max(int(attempt)-1, 0)
	return 5 * time.Second * time.Duration(1<<minInt(shift, 6))
}

func notificationBlockedDelay(blockedCount int32) time.Duration {
	switch blockedCount {
	case 0:
		return 5 * time.Minute
	case 1:
		return 15 * time.Minute
	case 2:
		return 30 * time.Minute
	default:
		return time.Hour
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func shouldDropRuntimeAfterClose(result alertstore.AlertCloseEventResult) bool {
	return result.Status == alertstore.CloseStatusClosed || result.Status == alertstore.CloseStatusNotFound
}

func mergeServerIDSet(dst, src map[int64]struct{}) {
	for serverID := range src {
		if serverID > 0 {
			dst[serverID] = struct{}{}
		}
	}
}
