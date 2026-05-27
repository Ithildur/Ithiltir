package traffic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dash/internal/config"
	"dash/internal/infra"
	trafficstore "dash/internal/store/traffic"
	kitlog "github.com/Ithildur/EiluneKit/logging"
)

const (
	materializeInterval = 5 * time.Minute
	snapshotInterval    = time.Hour
)

type Service struct {
	store     *trafficstore.Store
	location  *time.Location
	retention time.Duration
	gate      *writeGate
	logger    *kitlog.Helper
}

type Runtime struct {
	service *Service
	rebuild *RebuildRunner
}

func NewRuntime(ctx context.Context, st *trafficstore.Store, loc *time.Location, retentionDays int) *Runtime {
	gate := newWriteGate()
	retention := trafficRetention(retentionDays)
	return &Runtime{
		service: newService(st, loc, retention, gate),
		rebuild: newRebuildRunner(ctx, st, gate, retention),
	}
}

func (r *Runtime) RebuildRunner() *RebuildRunner {
	if r == nil {
		return nil
	}
	return r.rebuild
}

func (r *Runtime) Run(ctx context.Context) error {
	if r == nil || r.service == nil {
		return fmt.Errorf("traffic runtime is not initialized")
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
	if days <= 0 {
		days = config.DefaultTrafficRetentionDays
	}
	return time.Duration(days) * 24 * time.Hour
}

func newService(st *trafficstore.Store, loc *time.Location, retention time.Duration, gate *writeGate) *Service {
	if loc == nil {
		loc = time.Local
	}
	if retention <= 0 {
		retention = trafficRetention(config.DefaultTrafficRetentionDays)
	}
	return &Service{
		store:     st,
		location:  loc,
		retention: retention,
		gate:      writeGateOrNew(gate),
		logger:    infra.WithModule("traffic"),
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
	if err := s.gate.with(ctx, s.materializeOnce); err != nil {
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
	settings = trafficstore.SettingsWithTimezone(settings, s.location)

	var errs error
	if settings.UsageMode == trafficstore.UsageBilling {
		if err := s.withWriteTimeout(ctx, func(c context.Context) error {
			return s.store.BackfillTraffic5m(c, time.Time{}, now)
		}); err != nil {
			errs = errors.Join(errs, fmt.Errorf("backfill traffic 5m: %w", err))
		}
	}

	if err := s.withWriteTimeout(ctx, func(c context.Context) error {
		return s.store.BackfillTrafficMonthUsage(c, settings, s.location, time.Time{}, now)
	}); err != nil {
		errs = errors.Join(errs, fmt.Errorf("backfill traffic month usage: %w", err))
	}
	return errs
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
		settings = trafficstore.SettingsWithTimezone(settings, s.location)
		return struct{}{}, s.store.RefreshTrafficMonthlySnapshots(c, settings, s.location, now, s.retention)
	})
	return err
}

func (s *Service) withWriteTimeout(ctx context.Context, fn func(context.Context) error) error {
	_, err := infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
		return struct{}{}, fn(c)
	})
	return err
}
