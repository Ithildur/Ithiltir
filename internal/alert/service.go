package alert

import (
	"context"
	"fmt"
	"time"

	"dash/internal/infra"
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
	evalLeaseTTL             = 30 * time.Second
	controlTaskLeaseTTL      = 30 * time.Second
	notificationLeaseTTL     = 30 * time.Second
	dirtyWakeTimeout         = 5 * time.Second
	firingHeartbeatInterval  = 1 * time.Minute
)

type Service struct {
	store       *alertstore.Store
	front       *frontcache.Store
	cache       *RuleCache
	notify      *notifyCache
	logger      *kitlog.Helper
	evalWorkers int
	message     MessageConfig
	openAfter   time.Time
}

type ServiceOption func(*Service)

func WithMessageConfig(cfg MessageConfig) ServiceOption {
	return func(s *Service) {
		s.message = messageConfig([]MessageConfig{cfg})
	}
}

func NewService(st *alertstore.Store, front *frontcache.Store, opts ...ServiceOption) *Service {
	s := &Service{
		store:       st,
		front:       front,
		cache:       NewRuleCache(st, ruleCacheMinRefresh),
		notify:      newNotifyCache(st, ruleCacheMinRefresh),
		logger:      infra.WithModule("alert"),
		evalWorkers: defaultEvalWorkers,
		message:     messageConfig(nil),
		openAfter:   time.Now().UTC().Add(startupAlertGrace),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

func (s *Service) Run(ctx context.Context) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("alert service is not initialized")
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
