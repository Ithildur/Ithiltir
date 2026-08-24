package migrate

import (
	"database/sql"

	"dash/db/migrations"

	kitmigration "github.com/Ithildur/EiluneKit/postgres/migration"
)

const advisoryLockID int64 = 749153421

// New returns this application's Kit migration configuration.
func New(db *sql.DB, notifyKeyPath string) kitmigration.Config {
	return kitmigration.Config{
		DB:           db,
		Migrations:   migrations.Files(),
		LockID:       advisoryLockID,
		GoMigrations: migrations.Go(notifyKeyPath),
	}
}
