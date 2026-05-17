package config

import "time"

const DefaultNodeOfflineThreshold = 14 * time.Second
const DefaultRetentionDays = 45
const DefaultTrafficRetentionDays = 45

const (
	defaultRedisDialTimeout  = 5 * time.Second
	defaultRedisReadTimeout  = 3 * time.Second
	defaultRedisWriteTimeout = 3 * time.Second
)

// Database timeouts.
const (
	PGReadTimeout  = 3 * time.Second
	PGWriteTimeout = 5 * time.Second
)

// Redis operation timeouts.
const (
	RedisWriteTimeout = 500 * time.Millisecond
	RedisFetchTimeout = 500 * time.Millisecond
)

// Front API pagination.
const (
	FrontDefaultLimit = 200
	FrontMaxLimit     = 1000
)

// Node metrics ingest settings.
const (
	NodeMaxMetricsBodySize = 1 << 20 // 1MB limit to avoid abuse
	NodeRateLimitRequests  = 600
	NodeRateLimitWindow    = time.Minute
)

// HTTP server timeouts.
const (
	HTTPReadTimeout  = 15 * time.Second
	HTTPWriteTimeout = 15 * time.Second
	HTTPIdleTimeout  = 60 * time.Second
)
