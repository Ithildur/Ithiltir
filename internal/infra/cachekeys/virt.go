package cachekeys

// RedisKeyVirtSnapshotPrefix prefixes per-host VM snapshots with a ten-minute TTL.
const RedisKeyVirtSnapshotPrefix = redisKeyPrefix + "virt:v1:node:"
