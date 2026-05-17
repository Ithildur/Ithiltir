package infra

import (
	"context"
	"fmt"

	"dash/internal/config"
	kitpgx "github.com/Ithildur/EiluneKit/postgres/pgx"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewTimescalePool builds a pgx connection pool for TimescaleDB/PostgreSQL.
func NewTimescalePool(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	if cfg.Driver != "" && cfg.Driver != "postgres" {
		return nil, fmt.Errorf("timescale requires postgres driver, got %s", cfg.Driver)
	}

	if d, specified, err := cfg.EffectiveConnMaxLifetime(); err != nil {
		return nil, fmt.Errorf("parse database.conn_max_lifetime: %w", err)
	} else if specified {
		return kitpgx.NewPool(ctx, kitpgx.Config{
			Host:            cfg.Host,
			Port:            cfg.Port,
			User:            cfg.User,
			Password:        cfg.Password,
			Database:        cfg.Name,
			SSLMode:         cfg.SSLMode,
			MaxConns:        cfg.MaxOpenConns,
			MinConns:        cfg.MaxIdleConns,
			MaxConnLifetime: d,
		})
	}
	return kitpgx.NewPool(ctx, kitpgx.Config{
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		Database: cfg.Name,
		SSLMode:  cfg.SSLMode,
		MaxConns: cfg.MaxOpenConns,
		MinConns: cfg.MaxIdleConns,
	})
}

// PingTimescale performs a lightweight health check against the pool.
func PingTimescale(ctx context.Context, pool *pgxpool.Pool) error {
	return kitpgx.Ping(ctx, pool)
}
