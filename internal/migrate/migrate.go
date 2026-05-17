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

type Result struct {
	Total   int
	Applied int
	Skipped int
}

func Run(ctx context.Context, db *gorm.DB) (Result, error) {
	if db == nil {
		return Result{}, errors.New("db is nil")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return Result{}, fmt.Errorf("extract sql.DB: %w", err)
	}

	locker, err := gooselock.NewPostgresSessionLocker(gooselock.WithLockID(advisoryLockID))
	if err != nil {
		return Result{}, fmt.Errorf("create migration locker: %w", err)
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
			return Result{}, errors.New("no embedded migration files found")
		}
		return Result{}, fmt.Errorf("create goose provider: %w", err)
	}

	total := len(provider.ListSources())
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
