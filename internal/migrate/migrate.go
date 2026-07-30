package migrate

import (
	"context"
	"errors"
	"fmt"

	embeddedmigrations "dash/db"
	"github.com/pressly/goose/v3"
	gooselock "github.com/pressly/goose/v3/lock"
	"gorm.io/gorm"
)

const advisoryLockID int64 = 749153421

var (
	ErrSchemaAhead  = errors.New("database schema is newer than this binary")
	ErrSchemaBehind = errors.New("database schema migrations are pending")
)

type Result struct {
	Total   int
	Applied int
	Skipped int
}

func Run(ctx context.Context, db *gorm.DB) (Result, error) {
	provider, err := migrationProvider(db)
	if err != nil {
		return Result{}, err
	}
	total := len(provider.ListSources())
	current, target, err := provider.GetVersions(ctx)
	if err != nil {
		return Result{Total: total}, fmt.Errorf("read schema versions: %w", err)
	}
	if current > target {
		return Result{Total: total}, schemaVersionError(ErrSchemaAhead, current, target)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return Result{Total: total}, fmt.Errorf("run goose migrations: %w", err)
	}

	return Result{
		Total:   total,
		Applied: len(results),
		Skipped: total - len(results),
	}, nil
}

// CheckVersion requires the database schema to match this binary exactly.
// Schema upgrades belong to the explicit migrate command; application startup
// must not run against either older or newer persisted data.
func CheckVersion(ctx context.Context, db *gorm.DB) error {
	provider, err := migrationProvider(db)
	if err != nil {
		return err
	}
	current, target, err := provider.GetVersions(ctx)
	if err != nil {
		return fmt.Errorf("read schema versions: %w", err)
	}
	if current > target {
		return schemaVersionError(ErrSchemaAhead, current, target)
	}
	if current < target {
		return schemaVersionError(ErrSchemaBehind, current, target)
	}
	pending, err := provider.HasPending(ctx)
	if err != nil {
		return fmt.Errorf("inspect pending migrations: %w", err)
	}
	if pending {
		return schemaVersionError(ErrSchemaBehind, current, target)
	}
	return nil
}

func migrationProvider(db *gorm.DB) (*goose.Provider, error) {
	if db == nil {
		return nil, errors.New("db is nil")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("extract sql.DB: %w", err)
	}
	locker, err := gooselock.NewPostgresSessionLocker(gooselock.WithLockID(advisoryLockID))
	if err != nil {
		return nil, fmt.Errorf("create migration locker: %w", err)
	}
	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		sqlDB,
		embeddedmigrations.Migrations,
		goose.WithSessionLocker(locker),
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		if errors.Is(err, goose.ErrNoMigrations) {
			return nil, errors.New("no embedded migration files found")
		}
		return nil, fmt.Errorf("create goose provider: %w", err)
	}
	return provider, nil
}

func schemaVersionError(kind error, current, target int64) error {
	return fmt.Errorf("%w: database=%d embedded=%d", kind, current, target)
}
