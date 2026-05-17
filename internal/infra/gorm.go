package infra

import (
	"context"
	"fmt"
	"log/slog"

	"dash/internal/config"
	"github.com/Ithildur/EiluneKit/contextutil"
	kitlog "github.com/Ithildur/EiluneKit/logging"
	kitgorm "github.com/Ithildur/EiluneKit/postgres/gorm"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newGormLogger() logger.Interface {
	level := gormLogLevel()
	return kitgorm.NewLogger(kitgorm.LogOptions{
		Logger:               SlogWithModule("gorm"),
		Level:                level,
		Disabled:             level != slog.LevelDebug,
		IgnoreRecordNotFound: level != slog.LevelDebug,
	})
}

// NewGORMTimescale creates a GORM DB backed by Timescale/PostgreSQL via pgx.
func NewGORMTimescale(ctx context.Context, cfg config.DatabaseConfig) (*gorm.DB, error) {
	if cfg.Driver != "" && cfg.Driver != "postgres" {
		return nil, fmt.Errorf("gorm requires postgres driver, got %s", cfg.Driver)
	}

	if d, specified, err := cfg.EffectiveConnMaxLifetime(); err != nil {
		return nil, fmt.Errorf("parse database.conn_max_lifetime: %w", err)
	} else if specified {
		return kitgorm.Connect(ctx, kitgorm.Config{
			Host:                 cfg.Host,
			Port:                 cfg.Port,
			User:                 cfg.User,
			Password:             cfg.Password,
			Database:             cfg.Name,
			SSLMode:              cfg.SSLMode,
			MaxOpenConns:         cfg.MaxOpenConns,
			MaxIdleConns:         cfg.MaxIdleConns,
			ConnMaxLifetime:      d,
			PreferSimpleProtocol: true,
			Logger:               newGormLogger(),
		})
	}
	return kitgorm.Connect(ctx, kitgorm.Config{
		Host:                 cfg.Host,
		Port:                 cfg.Port,
		User:                 cfg.User,
		Password:             cfg.Password,
		Database:             cfg.Name,
		SSLMode:              cfg.SSLMode,
		MaxOpenConns:         cfg.MaxOpenConns,
		MaxIdleConns:         cfg.MaxIdleConns,
		PreferSimpleProtocol: true,
		Logger:               newGormLogger(),
	})
}

// PingGORM runs a light ping using the underlying sql.DB.
func PingGORM(ctx context.Context, db *gorm.DB) error {
	return kitgorm.Ping(ctx, db)
}

// WithPGReadTimeout runs fn with the default PG read timeout.
func WithPGReadTimeout[T any](ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	return contextutil.WithTimeout(ctx, config.PGReadTimeout, fn)
}

// WithPGWriteTimeout runs fn with the default PG write timeout.
func WithPGWriteTimeout[T any](ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	return contextutil.WithTimeout(ctx, config.PGWriteTimeout, fn)
}

func gormLogLevel() slog.Level {
	level := LogLevel()
	if level == kitlog.LevelDebug {
		return slog.LevelDebug
	}
	return slog.LevelError
}
