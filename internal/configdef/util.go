package configdef

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/platform/postgres"
)

func inTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	return postgres.InTx(ctx, pool, fn)
}

// notFound maps a missing row to code, passing other errors through.
func notFound(err error, code string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound(code)
	}
	return err
}

func conflict(code string) error { return apperr.Conflict(code) }

// name validates a definition name of at most maxLen runes.
func name(fields *apperr.Fields, field, v string, maxLen int) string {
	v = strings.TrimSpace(v)
	switch {
	case v == "":
		fields.Add(field, "required")
	case len([]rune(v)) > maxLen:
		fields.Add(field, "too_long")
	}
	return v
}

func colour(fields *apperr.Fields, v string) {
	if !slices.Contains(Colours, v) {
		fields.Add("colour", "invalid")
	}
}
