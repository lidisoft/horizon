package migrations_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/lidisoft/horizon/internal/migrations"
	"github.com/lidisoft/horizon/internal/testenv"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres)) }

// Every migration must roll back and re-apply cleanly.
func TestDownAndUpAgain(t *testing.T) {
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(testenv.DB(t))
	defer db.Close()

	want, err := migrations.Expected()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := migrations.Current(ctx, db); err != nil || got != want {
		t.Fatalf("template at %d (%v), want %d", got, err, want)
	}

	p, err := migrations.Provider(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.DownTo(ctx, 0); err != nil {
		t.Fatalf("down: %v", err)
	}
	if got, _ := migrations.Current(ctx, db); got != 0 {
		t.Fatalf("after down: version %d", got)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
	if got, _ := migrations.Current(ctx, db); got != want {
		t.Fatalf("after up: version %d", got)
	}
}
