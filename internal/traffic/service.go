package traffic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dash/internal/infra"
	trafficstore "dash/internal/store/traffic"
	kitlog "github.com/Ithildur/EiluneKit/logging"
)

const (
	materializeInterval    = 5 * time.Minute
	snapshotInterval       = time.Hour
	materializeSettle      = 30 * time.Second
	materializeStepTimeout = 30 * time.Second
	materializeMaxSteps    = 12

	usageRepairMaxSteps = 120
	usageRepairBudget   = 30 * time.Second
)

type Service struct {
	store            *trafficstore.Store
	location         *time.Location
	sourceRetention  time.Duration
	trafficRetention time.Duration
	gate             *writeGate
	logger           *kitlog.Helper
}

type Runtime struct {
	service *Service
	rebuild *RebuildRunner
}

func NewRuntime(ctx context.Context, st *trafficstore.Store, loc *time.Location, sourceRetentionDays, trafficRetentionDays int) (*Runtime, error) {
	if ctx == nil {
		return nil, fmt.Errorf("traffic runtime context is nil")
	}
	if st == nil {
		return nil, fmt.Errorf("traffic store is nil")
	}
	if loc == nil {
		return nil, fmt.Errorf("traffic location is nil")
	}
	if sourceRetentionDays <= 0 {
		return nil, fmt.Errorf("traffic source retention days must be positive")
	}
	if trafficRetentionDays <= 0 {
		return nil, fmt.Errorf("traffic retention days must be positive")
	}

	gate := newWriteGate()
	sourceRetention := trafficRetention(sourceRetentionDays)
	trafficRetention := trafficRetention(trafficRetentionDays)
	runtime := &Runtime{
		service: newService(st, loc, sourceRetention, trafficRetention, gate),
		rebuild: newRebuildRunner(ctx, st, gate, minDuration(sourceRetention, trafficRetention)),
	}
	return runtime, nil
}

func (r *Runtime) RebuildRunner() *RebuildRunner {
	return r.rebuild
}

func (r *Runtime) Run(ctx context.Context) error {
	if r == nil || r.service == nil {
		return fmt.Errorf("traffic runtime is not initialized")
	}
	if ctx == nil {
		return fmt.Errorf("traffic runtime context is nil")
	}
	return r.service.Run(ctx)
}

func (r *Runtime) Stop() {
	if r == nil || r.rebuild == nil {
		return
	}
	r.rebuild.Stop()
}

func trafficRetention(days int) time.Duration {
	return time.Duration(days) * 24 * time.Hour
}

func newService(st *trafficstore.Store, loc *time.Location, sourceRetention, trafficRetention time.Duration, gate *writeGate) *Service {
	return &Service{
		store:            st,
		location:         loc,
		sourceRetention:  sourceRetention,
		trafficRetention: trafficRetention,
		gate:             gate,
		logger:           infra.WithModule("traffic"),
	}
}

func (s *Service) Run(ctx context.Context) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("traffic service is not initialized")
	}

	s.materialize(ctx)
	s.snapshot(ctx)
	materializeTicker := time.NewTicker(materializeInterval)
	defer materializeTicker.Stop()
	snapshotTicker := time.NewTicker(snapshotInterval)
	defer snapshotTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-materializeTicker.C:
			s.materialize(ctx)
		case <-snapshotTicker.C:
			s.snapshot(ctx)
		}
	}
}

func (s *Service) materialize(ctx context.Context) {
	if err := s.materializeOnce(ctx); err != nil {
		s.logger.Warn("materialize traffic failed", err)
	}
}

func (s *Service) materializeOnce(ctx context.Context) error {
	now := time.Now().In(s.location)
	settings, err := infra.WithPGReadTimeout(ctx, func(c context.Context) (trafficstore.Settings, error) {
		return s.store.GetSettings(c)
	})
	if err != nil {
		return err
	}
	target := now.Add(-materializeSettle)
	usageFloor := target.Add(-s.sourceRetention)

	var errs error
	if err := s.materializeUsage(ctx, target, usageFloor); err != nil {
		errs = errors.Join(errs, fmt.Errorf("materialize traffic usage: %w", err))
	}
	if err := s.materializeUsageRepairs(ctx, target, usageFloor); err != nil {
		errs = errors.Join(errs, fmt.Errorf("repair traffic usage: %w", err))
	}
	if settings.UsageMode == trafficstore.UsageBilling {
		factsFloor := target.Add(-minDuration(s.sourceRetention, s.trafficRetention))
		if err := s.materializeFacts(ctx, target, factsFloor); err != nil {
			errs = errors.Join(errs, fmt.Errorf("materialize traffic facts: %w", err))
		}
	}
	return errs
}

func (s *Service) materializeUsage(ctx context.Context, target, sourceFloor time.Time) error {
	for range materializeMaxSteps {
		hasMore, err := s.usageStep(ctx, target, sourceFloor)
		if err != nil {
			return err
		}
		if !hasMore {
			return nil
		}
	}
	return nil
}

func (s *Service) materializeUsageRepairs(ctx context.Context, target, sourceFloor time.Time) error {
	repairCtx, cancel := context.WithTimeout(ctx, usageRepairBudget)
	defer cancel()

	for range usageRepairMaxSteps {
		if repairCtx.Err() != nil {
			return nil
		}
		hasMore, err := s.usageRepairStep(repairCtx, target, sourceFloor)
		if err != nil {
			return err
		}
		if !hasMore {
			return nil
		}
	}
	return nil
}

func (s *Service) materializeFacts(ctx context.Context, target, sourceFloor time.Time) error {
	for range materializeMaxSteps {
		hasMore, err := s.factsStep(ctx, target, sourceFloor)
		if err != nil {
			return err
		}
		if !hasMore {
			return nil
		}
	}
	return nil
}

func (s *Service) usageStep(ctx context.Context, target, sourceFloor time.Time) (bool, error) {
	var hasMore bool
	err := s.gate.with(ctx, func(c context.Context) error {
		var err error
		hasMore, err = withMaterializeStepTimeout(c, func(writeCtx context.Context) (bool, error) {
			return s.store.MaterializeTrafficMonthUsage(writeCtx, s.location, target, sourceFloor)
		})
		return err
	})
	return hasMore, err
}

func (s *Service) usageRepairStep(ctx context.Context, target, sourceFloor time.Time) (bool, error) {
	var hasMore bool
	err := s.gate.with(ctx, func(c context.Context) error {
		var err error
		hasMore, err = s.store.MaterializeTrafficMonthUsageRepair(c, s.location, target, sourceFloor)
		return err
	})
	return hasMore, err
}

func (s *Service) factsStep(ctx context.Context, target, sourceFloor time.Time) (bool, error) {
	var hasMore bool
	err := s.gate.with(ctx, func(c context.Context) error {
		var err error
		hasMore, err = withMaterializeStepTimeout(c, func(writeCtx context.Context) (bool, error) {
			return s.store.MaterializeTraffic5m(writeCtx, target, sourceFloor)
		})
		return err
	})
	return hasMore, err
}

func (s *Service) snapshot(ctx context.Context) {
	if err := s.gate.with(ctx, s.snapshotOnce); err != nil {
		s.logger.Warn("refresh traffic monthly snapshots failed", err)
	}
}

func (s *Service) snapshotOnce(ctx context.Context) error {
	now := time.Now().In(s.location)
	_, err := infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
		settings, err := s.store.GetSettings(c)
		if err != nil {
			return struct{}{}, err
		}
		settings, err = trafficstore.SettingsWithTimezone(settings, s.location)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, s.store.RefreshTrafficMonthlySnapshots(c, settings, s.location, now, s.trafficRetention)
	})
	return err
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func withMaterializeStepTimeout(ctx context.Context, fn func(context.Context) (bool, error)) (bool, error) {
	stepCtx, cancel := context.WithTimeout(ctx, materializeStepTimeout)
	defer cancel()
	return fn(stepCtx)
}
