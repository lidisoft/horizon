// Package migrations embeds the goose SQL migrations and runs them.
//
// Migrations are expand/contract (docs/operations.md#upgrades): each one must
// work with the code of the previous release.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var files embed.FS

// FS returns the embedded migration files.
func FS() fs.FS { return files }

// Provider returns a goose provider over the embedded migrations.
func Provider(db *sql.DB) (*goose.Provider, error) {
	p, err := goose.NewProvider(goose.DialectPostgres, db, files)
	if err != nil {
		return nil, fmt.Errorf("goose: %w", err)
	}
	return p, nil
}

// Expected returns the highest embedded version: the schema this binary needs.
// File names start with the version, as goose requires (00001_name.sql).
func Expected() (int64, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return 0, err
	}
	var max int64
	for _, e := range entries {
		v, err := goose.NumericComponent(e.Name())
		if err != nil {
			return 0, fmt.Errorf("migration %s: %w", e.Name(), err)
		}
		if v > max {
			max = v
		}
	}
	return max, nil
}

// Current returns the version applied to db (0 when none).
func Current(ctx context.Context, db *sql.DB) (int64, error) {
	p, err := Provider(db)
	if err != nil {
		return 0, err
	}
	return p.GetDBVersion(ctx)
}
