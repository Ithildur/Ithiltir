package infra

import (
	"context"
	"fmt"

	"dash/internal/config"
	kitredis "github.com/Ithildur/EiluneKit/redis"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient builds a Redis client with sensible defaults.
func NewRedisClient(cfg config.RedisConfig) (*redis.Client, error) {
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

// PingRedis performs a lightweight health check against the Redis server.
func PingRedis(ctx context.Context, client *redis.Client) error {
	return kitredis.Ping(ctx, client)
}
