package traffic

import (
	"context"
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
	catchupChunk        = time.Hour
)

type Service struct {
	store          *trafficstore.Store
	location       *time.Location
	retention      time.Duration
	usageCatchupAt time.Time
	logger         *kitlog.Helper
}

func NewService(st *trafficstore.Store, loc *time.Location, retentionDays int) *Service {
	if loc == nil {
		loc = time.Local
	}
	if retentionDays <= 0 {
		retentionDays = config.DefaultRetentionDays
	}
	return &Service{
		store:     st,
		location:  loc,
		retention: time.Duration(retentionDays) * 24 * time.Hour,
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
	if err := s.materializeOnce(ctx); err != nil {
		s.logger.Warn("materialize traffic buckets failed", err)
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
	needsTimezoneSave := settings.BillingTimezone == ""
	settings = trafficstore.SettingsWithTimezone(settings, s.location)
	if needsTimezoneSave {
		if err := s.withWriteTimeout(ctx, func(c context.Context) error {
			return s.store.SetSettings(c, settings)
		}); err != nil {
			return err
		}
	}

	var first error
	if start, end, ok := s.nextUsageCatchup(now); ok {
		if err := s.withWriteTimeout(ctx, func(c context.Context) error {
			return s.store.BackfillTrafficMonthUsage(c, settings, s.location, start, end)
		}); err != nil && first == nil {
			first = err
		} else if err == nil {
			s.usageCatchupAt = end
		}
	}
	if settings.UsageMode == trafficstore.UsageBilling {
		if err := s.withWriteTimeout(ctx, func(c context.Context) error {
			return s.store.BackfillTraffic5m(c, time.Time{}, now)
		}); err != nil && first == nil {
			first = err
		}
		if err := s.withWriteTimeout(ctx, func(c context.Context) error {
			return s.store.BackfillTraffic5mMissing(c, s.retention, now)
		}); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (s *Service) nextUsageCatchup(now time.Time) (time.Time, time.Time, bool) {
	start := now.Add(-s.retention)
	if s.usageCatchupAt.IsZero() || s.usageCatchupAt.Before(start) || s.usageCatchupAt.After(now) {
		s.usageCatchupAt = start
	}
	end := s.usageCatchupAt.Add(catchupChunk)
	if end.After(now) {
		end = now
	}
	return s.usageCatchupAt, end, end.After(s.usageCatchupAt)
}

func (s *Service) snapshot(ctx context.Context) {
	if err := s.snapshotOnce(ctx); err != nil {
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
