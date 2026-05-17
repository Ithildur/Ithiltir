package cachekeys

const (
	// RedisKeyFrontNodeIDs stores node identifiers with cached frontend metrics.
	RedisKeyFrontNodeIDs = "front:v1:node:ids"
	// RedisKeyFrontNodeSnapshotPrefix prefixes per-node latest frontend snapshots.
	RedisKeyFrontNodeSnapshotPrefix = "front:v1:node:snapshot:"
	// RedisKeyFrontNodeSmartPrefix prefixes per-node latest SMART runtime details.
	RedisKeyFrontNodeSmartPrefix = "front:v1:node:smart:"
	// RedisKeyFrontNodeThermalPrefix prefixes per-node latest thermal runtime details.
	RedisKeyFrontNodeThermalPrefix = "front:v1:node:thermal:"
	// RedisKeyFrontMeta marks a complete published frontend snapshot index.
	RedisKeyFrontMeta = "front:v1:meta"
	// RedisKeyGuestVisibleIDs stores all guest-visible active server identifiers.
	RedisKeyGuestVisibleIDs = "server:v1:guest_visible:ids"
	// RedisKeyGuestVisibilityMeta marks a complete published guest visibility index.
	RedisKeyGuestVisibilityMeta = "server:v1:guest_visible:meta"
)
