package infra

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dash/internal/config"
	appversion "dash/internal/version"
	kitredis "github.com/Ithildur/EiluneKit/redis"

	"github.com/redis/go-redis/v9"
)

const (
	redisMinVersion         = "6.2.0"
	RedisRecommendedVersion = "8.2.3"
)

// NewRedisClient builds a Redis client with sensible defaults.
func NewRedisClient(cfg config.RedisConfig) (*redis.Client, error) {
	if err := validateRedisPool(cfg); err != nil {
		return nil, err
	}
	dialTimeout, err := cfg.EffectiveDialTimeout()
	if err != nil {
		return nil, fmt.Errorf("parse redis.dial_timeout: %w", err)
	}
	readTimeout, err := cfg.EffectiveReadTimeout()
	if err != nil {
		return nil, fmt.Errorf("parse redis.read_timeout: %w", err)
	}
	writeTimeout, err := cfg.EffectiveWriteTimeout()
	if err != nil {
		return nil, fmt.Errorf("parse redis.write_timeout: %w", err)
	}

	return kitredis.NewClient(kitredis.Config{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	})
}

func validateRedisPool(cfg config.RedisConfig) error {
	if cfg.PoolSize < 0 {
		return fmt.Errorf("redis.pool_size must be >= 0")
	}
	if cfg.MinIdleConns < 0 {
		return fmt.Errorf("redis.min_idle_conns must be >= 0")
	}
	if cfg.PoolSize == 0 && cfg.MinIdleConns > 0 {
		return fmt.Errorf("redis.pool_size must be positive when redis.min_idle_conns is positive")
	}
	if cfg.PoolSize > 0 && cfg.MinIdleConns > cfg.PoolSize {
		return fmt.Errorf("redis.min_idle_conns must be <= redis.pool_size")
	}
	return nil
}

// CheckRedis verifies connectivity and the required server version. Each
// command receives a fresh timeout so INFO does not inherit time already spent
// dialing or pinging.
func CheckRedis(ctx context.Context, client *redis.Client, operationTimeout time.Duration) (string, error) {
	if operationTimeout <= 0 {
		return "", fmt.Errorf("redis check timeout must be positive")
	}
	pingCtx, pingCancel := context.WithTimeout(ctx, operationTimeout)
	err := kitredis.Ping(pingCtx, client)
	pingCancel()
	if err != nil {
		return "", fmt.Errorf("ping redis: %w", err)
	}
	infoCtx, infoCancel := context.WithTimeout(ctx, operationTimeout)
	info, err := client.Info(infoCtx, "server").Result()
	infoCancel()
	if err != nil {
		return "", fmt.Errorf("read redis server info: %w", err)
	}
	serverVersion, ok := redisInfoValue(info, "redis_version")
	if !ok {
		return "", fmt.Errorf("redis server info is missing redis_version")
	}
	cmp, err := appversion.Compare(serverVersion, redisMinVersion)
	if err != nil {
		return "", fmt.Errorf("invalid redis version %q: %w", serverVersion, err)
	}
	if cmp < 0 {
		return "", fmt.Errorf("redis version %s is below required %s", serverVersion, redisMinVersion)
	}
	return serverVersion, nil
}

func redisInfoValue(info, key string) (string, bool) {
	for line := range strings.SplitSeq(info, "\n") {
		name, value, ok := strings.Cut(strings.TrimSuffix(line, "\r"), ":")
		if ok && name == key {
			value = strings.TrimSpace(value)
			return value, value != ""
		}
	}
	return "", false
}
