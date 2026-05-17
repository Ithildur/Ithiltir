package db

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var embedded embed.FS

var Migrations = mustSub(embedded, "migrations")

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
