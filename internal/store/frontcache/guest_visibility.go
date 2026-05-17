package frontcache

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type GuestVisibilityOptions struct {
	CacheTimeout time.Duration
	BuildTimeout time.Duration
}

var errMissingDB = errors.New("frontcache: db is nil")

func (s *Store) loadGuestVisibleIDs(ctx context.Context, ids []int64) (map[int64]struct{}, bool, error) {
	allowed, ok, err := s.backend.loadGuestVisibleIDs(ctx, ids)
	if errors.Is(err, errCorruptGuestVisibility) {
		if clearErr := s.backend.clearGuestVisibilityMeta(ctx); clearErr != nil {
			return nil, false, errors.Join(err, clearErr)
		}
		return nil, false, nil
	}
	return allowed, ok, err
}

func (s *Store) replaceGuestVisibleIDs(ctx context.Context, allowed map[int64]struct{}) error {
	return s.backend.replaceGuestVisibleIDs(ctx, allowed)
}

func (s *Store) EnsureGuestVisibleIDs(ctx context.Context, ids []int64, opts GuestVisibilityOptions) (map[int64]struct{}, error) {
	if len(ids) == 0 {
		return map[int64]struct{}{}, nil
	}

	cacheCtx, cacheCancel := context.WithTimeout(ctx, opts.CacheTimeout)
	allowed, ok, err := s.loadGuestVisibleIDs(cacheCtx, ids)
	cacheCancel()
	if err != nil {
		return nil, err
	}
	if ok {
		return allowed, nil
	}

	allowed, err = singleflightDo(ctx, &s.guestVisibilitySF, "rebuild", func(rebuildCtx context.Context) (map[int64]struct{}, error) {
		return s.rebuildGuestVisibility(rebuildCtx, opts.BuildTimeout, opts.CacheTimeout)
	})
	if err != nil {
		return nil, err
	}

	return filterAllowedIDs(ids, allowed), nil
}

func (s *Store) EnsureGuestVisible(ctx context.Context, id int64, opts GuestVisibilityOptions) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	allowed, err := s.EnsureGuestVisibleIDs(ctx, []int64{id}, opts)
	if err != nil {
		return false, err
	}
	_, ok := allowed[id]
	return ok, nil
}

func (s *Store) ClearGuestVisibilityMeta(ctx context.Context) error {
	return s.backend.clearGuestVisibilityMeta(ctx)
}

func (s *Store) rebuildGuestVisibility(ctx context.Context, dbTimeout, cacheTimeout time.Duration) (map[int64]struct{}, error) {
	dbCtx, dbCancel := context.WithTimeout(ctx, dbTimeout)
	defer dbCancel()

	allowed, err := s.fetchGuestVisibleIDs(dbCtx)
	if err != nil {
		return nil, fmt.Errorf("fetch guest visible ids: %w", err)
	}

	cacheCtx, cacheCancel := context.WithTimeout(ctx, cacheTimeout)
	publishErr := s.replaceGuestVisibleIDs(cacheCtx, allowed)
	cacheCancel()
	if publishErr != nil {
		return nil, fmt.Errorf("publish guest visible ids: %w", publishErr)
	}
	return allowed, nil
}

func (s *Store) fetchGuestVisibleIDs(ctx context.Context) (map[int64]struct{}, error) {
	if s == nil || s.db == nil {
		return nil, errMissingDB
	}
	var allowed []struct {
		ID int64
	}
	if err := s.db.WithContext(ctx).
		Table("servers").
		Select("id").
		Where("is_guest_visible = ?", true).
		Where("is_deleted = ?", false).
		Find(&allowed).Error; err != nil {
		return nil, err
	}
	out := make(map[int64]struct{}, len(allowed))
	for _, srv := range allowed {
		out[srv.ID] = struct{}{}
	}
	return out, nil
}

func filterAllowedIDs(ids []int64, allowed map[int64]struct{}) map[int64]struct{} {
	out := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := allowed[id]; ok {
			out[id] = struct{}{}
		}
	}
	return out
}
