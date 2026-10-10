package alert

import (
	"context"
	"fmt"
	"sync"
	"time"

	"dash/internal/infra"
	"dash/internal/lang"
	"dash/internal/metrics"
	alertstore "dash/internal/store/alert"
	"dash/internal/store/frontcache"
	kitlog "github.com/Ithildur/EiluneKit/logging"
	"golang.org/x/sync/errgroup"
)

const (
	evalWorkers              = 4
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
	message       MessageConfig
	openAfter     time.Time
	staleAfterSec int
	runtimeMu     sync.Mutex
	runtime       map[int64]map[string]RuntimeState
}

func NewService(st *alertstore.Store, front *frontcache.Store, message MessageConfig, offlineThreshold time.Duration) *Service {
	message.Language = lang.Normalize(message.Language)
	return &Service{
		store:         st,
		front:         front,
		cache:         NewRuleCache(st, ruleCacheMinRefresh),
		notify:        newNotifyCache(st, ruleCacheMinRefresh),
		logger:        infra.WithModule("alert"),
		message:       message,
		openAfter:     time.Now().UTC().Add(startupAlertGrace),
		staleAfterSec: metrics.DurationSecondsCeil(offlineThreshold),
		runtime:       make(map[int64]map[string]RuntimeState),
	}
}

func (s *Service) Run(ctx context.Context) error {
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
	for workerID := range evalWorkers {
		group.Go(func() error { return s.runEvalWorker(groupCtx, workerID) })
	}
	return group.Wait()
}

func controlTaskRetryDelay(attempt int32) time.Duration {
	seconds := 1 << min(int(attempt), 6)
	return time.Duration(seconds) * time.Second
}

func notificationRetryDelay(attempt int32) time.Duration {
	shift := max(int(attempt)-1, 0)
	return 5 * time.Second * time.Duration(1<<min(shift, 6))
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
