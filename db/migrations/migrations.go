package migrations

import (
	"embed"
	"io/fs"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql */*.sql
var files embed.FS

// Files returns the migration filesystem. Goose only discovers SQL files at
// its root; version subdirectories hold SQL owned by registered Go migrations.
func Files() fs.FS {
	return files
}

// Go returns migrations that require application code in addition to SQL.
func Go(notifyKeyPath string) []*goose.Migration {
	return []*goose.Migration{
		notificationAndTrafficState(notifyKeyPath),
		metricsHistory(),
	}
}

func readSQL(name string) string {
	body, err := files.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(body)
}
