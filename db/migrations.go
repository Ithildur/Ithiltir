package db

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql migrationdata/*.sql
var embedded embed.FS

var Migrations = mustSub(embedded, "migrations")
var notificationTrafficStateSQL = mustRead(embedded, "migrationdata/0011_notification_and_traffic_state.sql")

// NotificationTrafficStateSQL is the SQL body of Go migration 11. Keeping it
// outside Migrations prevents SQL-only runners from recording version 11
// without completing the Go-owned notification-config encryption step.
func NotificationTrafficStateSQL() string {
	return notificationTrafficStateSQL
}

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

func mustRead(fsys fs.FS, name string) string {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		panic(err)
	}
	return string(data)
}
