package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// release is a parsed "vMAJOR.MINOR.PATCH" (a pre-release suffix is
// ignored).
type release struct{ major, minor, patch int }

// parseRelease reads a release version; ok is false for a development
// build ("dev") or anything else.
func parseRelease(v string) (release, bool) {
	v = strings.TrimPrefix(v, "v")
	v, _, _ = strings.Cut(v, "-")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return release{}, false
	}
	var n [3]int
	for i, p := range parts {
		x, err := strconv.Atoi(p)
		if err != nil || x < 0 {
			return release{}, false
		}
		n[i] = x
	}
	return release{n[0], n[1], n[2]}, true
}

func (r release) compare(o release) int {
	switch {
	case r.major != o.major:
		return r.major - o.major
	case r.minor != o.minor:
		return r.minor - o.minor
	}
	return r.patch - o.patch
}

// CheckUpgrade decides whether binary may migrate a database last migrated
// by last: not backwards, and not from before minFrom (the oldest release
// this one upgrades from; "" for any). Development builds, and databases
// no release migrated yet, pass.
func CheckUpgrade(last, binary, minFrom string) error {
	l, okL := parseRelease(last)
	b, okB := parseRelease(binary)
	if !okL || !okB {
		return nil
	}
	if l.compare(b) > 0 {
		return fmt.Errorf("the database was migrated by %s, newer than this binary (%s): downgrades aren't supported, restore a backup instead", last, binary)
	}
	if m, ok := parseRelease(minFrom); ok && l.compare(m) < 0 {
		return fmt.Errorf("%s upgrades from %s or later, and the database is at %s: upgrade to %s first", binary, minFrom, last, minFrom)
	}
	return nil
}

// CheckRunnable decides whether binary may serve a database last migrated
// by last: the same major release, and at most one minor release behind
// (the old processes of a rolling upgrade, which migrations keep working).
// Development builds, and databases no release migrated yet, pass.
func CheckRunnable(last, binary string) error {
	l, okL := parseRelease(last)
	b, okB := parseRelease(binary)
	if !okL || !okB {
		return nil
	}
	if l.major != b.major || l.minor > b.minor+1 {
		return fmt.Errorf("the database was migrated by %s; this binary (%s) is too old to run on it: deploy %s or later", last, binary, fmt.Sprintf("v%d.%d.0", l.major, max(l.minor-1, 0)))
	}
	return nil
}

// LastRelease is the release that last migrated db ("" when none, or
// before the table existed).
func LastRelease(ctx context.Context, db *sql.DB) (string, error) {
	var v string
	err := db.QueryRowContext(ctx, `SELECT version FROM horizon_releases ORDER BY id DESC LIMIT 1`).Scan(&v)
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, sql.ErrNoRows), errors.As(err, &pgErr) && pgErr.Code == "42P01": // undefined_table
		return "", nil
	case err != nil:
		return "", err
	}
	return v, nil
}

// RecordRelease notes that version (built from commit) migrated db.
func RecordRelease(ctx context.Context, db *sql.DB, version, commit string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO horizon_releases (version, commit) VALUES ($1, $2)`, version, commit)
	return err
}
