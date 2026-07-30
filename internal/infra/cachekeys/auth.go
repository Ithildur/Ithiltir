package cachekeys

const (
	// RedisKeyAuthTokenPrefix remains unchanged so sessions survive upgrades.
	RedisKeyAuthTokenPrefix = "auth:jwt:"
)
