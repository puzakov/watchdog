// Package migrations handles database schema migrations using the
// github.com/pressly/goose library with embedded SQL migration files.
package migrations

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var fs embed.FS

// Up applies all pending SQL migrations using goose.
func Up(db *sql.DB) error {
	goose.SetBaseFS(fs)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, ".")
}
