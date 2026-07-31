package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	embeddedmigrations "dash/db"

	"github.com/pressly/goose/v3"
	gooselock "github.com/pressly/goose/v3/lock"
	"gorm.io/gorm"
)

const (
	advisoryLockID        int64 = 749153421
	stateMigrationVersion int64 = 11
)

var (
	ErrSchemaAhead  = errors.New("database schema is newer than this binary")
	ErrSchemaBehind = errors.New("database schema migrations are pending")
)

type Result struct {
	Total   int
	Applied int
	Skipped int
}

func Run(ctx context.Context, db *gorm.DB, notifyKeyPath string) (Result, error) {
	return run(ctx, db, notifyKeyPath, nil)
}

// RunTo advances a database only through target. It exists for historical
// migration fixtures; production migration should always call Run.
func RunTo(ctx context.Context, db *gorm.DB, notifyKeyPath string, target int64) (Result, error) {
	return run(ctx, db, notifyKeyPath, &target)
}

func run(ctx context.Context, db *gorm.DB, notifyKeyPath string, limit *int64) (Result, error) {
	provider, err := migrationProvider(db, notifyKeyPath)
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
	if limit != nil && (*limit < current || *limit > target) {
		return Result{Total: total}, fmt.Errorf(
			"invalid migration target %d for database=%d embedded=%d",
			*limit,
			current,
			target,
		)
	}

	var results []*goose.MigrationResult
	if limit == nil {
		results, err = provider.Up(ctx)
	} else {
		results, err = provider.UpTo(ctx, *limit)
	}
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
	provider, err := migrationProvider(db, "")
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

func migrationProvider(db *gorm.DB, notifyKeyPath string) (*goose.Provider, error) {
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
		goose.WithGoMigrations(stateMigration(notifyKeyPath)),
	)
	if err != nil {
		if errors.Is(err, goose.ErrNoMigrations) {
			return nil, errors.New("no embedded migration files found")
		}
		return nil, fmt.Errorf("create goose provider: %w", err)
	}
	return provider, nil
}

func stateMigration(keyPath string) *goose.Migration {
	return goose.NewGoMigration(
		stateMigrationVersion,
		&goose.GoFunc{
			RunTx: func(ctx context.Context, tx *sql.Tx) error {
				if strings.TrimSpace(keyPath) == "" {
					return errors.New("notification config key path is empty")
				}
				if _, err := tx.ExecContext(ctx, embeddedmigrations.NotificationTrafficStateSQL()); err != nil {
					return fmt.Errorf("run notification and traffic state SQL: %w", err)
				}
				return sealNotifyConfigs(ctx, tx, keyPath)
			},
		},
		nil,
	)
}

func schemaVersionError(kind error, current, target int64) error {
	return fmt.Errorf("%w: database=%d embedded=%d", kind, current, target)
}
