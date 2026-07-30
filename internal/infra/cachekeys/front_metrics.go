package cachekeys

const redisKeyPrefix = "ithiltir:dash:"

const (
	// RedisKeyFrontNodeIDs stores node identifiers with cached frontend metrics.
	RedisKeyFrontNodeIDs = redisKeyPrefix + "front:v2:node:ids"
	// RedisKeyFrontNodeSnapshotPrefix prefixes per-node latest runtime snapshots.
	RedisKeyFrontNodeSnapshotPrefix = redisKeyPrefix + "front:v2:node:runtime:"
	// RedisKeyFrontNodeMetadataPrefix prefixes PostgreSQL-derived frontend metadata.
	RedisKeyFrontNodeMetadataPrefix = redisKeyPrefix + "front:v2:node:meta:"
	// RedisKeyFrontNodeSmartPrefix prefixes per-node latest SMART runtime details.
	RedisKeyFrontNodeSmartPrefix = redisKeyPrefix + "front:v2:node:smart:"
	// RedisKeyFrontNodeThermalPrefix prefixes per-node latest thermal runtime details.
	RedisKeyFrontNodeThermalPrefix = redisKeyPrefix + "front:v2:node:thermal:"
	// RedisKeyFrontMeta marks a complete published frontend node catalog.
	RedisKeyFrontMeta = redisKeyPrefix + "front:v2:node:catalog"
	// RedisKeyGuestVisibleIDs stores all guest-visible active server identifiers.
	RedisKeyGuestVisibleIDs = redisKeyPrefix + "front:v2:guest:ids"
	// RedisKeyGuestVisibilityMeta marks a complete published guest visibility index.
	RedisKeyGuestVisibilityMeta = redisKeyPrefix + "front:v2:guest:catalog"
)
