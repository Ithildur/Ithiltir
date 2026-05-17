package node

import "errors"

// Sentinel errors for store operations.
var (
	ErrDuplicateSecret       = errors.New("duplicate_secret")
	ErrFrontCacheUpdate      = errors.New("front_cache_update_failed")
	ErrServerMetaCacheUpdate = errors.New("server_meta_cache_update_failed")
	ErrInvalidGroupIDs       = errors.New("invalid_group_ids")
	ErrInvalidSecret         = errors.New("invalid_secret")
)
