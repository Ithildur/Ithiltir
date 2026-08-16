package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	embeddedmigrations "dash/db"

	kitmigration "github.com/Ithildur/EiluneKit/postgres/migration"
	"github.com/pressly/goose/v3"
)

const (
	advisoryLockID        int64 = 749153421
	stateMigrationVersion int64 = 11
)

// New returns this application's Kit migration configuration.
func New(db *sql.DB, notifyKeyPath string) kitmigration.Config {
	return kitmigration.Config{
		DB:           db,
		Migrations:   embeddedmigrations.Migrations,
		LockID:       advisoryLockID,
		GoMigrations: []*goose.Migration{stateMigration(notifyKeyPath)},
	}
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
