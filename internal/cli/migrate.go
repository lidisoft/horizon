package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"text/tabwriter"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"
	"github.com/pressly/goose/v3"
	"github.com/urfave/cli/v3"

	"github.com/lidisoft/horizon/internal/config"
	"github.com/lidisoft/horizon/internal/logging"
	"github.com/lidisoft/horizon/internal/migrations"
	"github.com/lidisoft/horizon/internal/version"
)

func migrateCommand() *cli.Command {
	return &cli.Command{
		Name:  "migrate",
		Usage: "Manage the database schema (embedded goose migrations)",
		Commands: []*cli.Command{
			{Name: "up", Usage: "Apply every pending migration", Action: migrate(migrateUp)},
			{Name: "down", Usage: "Roll back the most recent migration", Action: migrate(migrateDown)},
			{Name: "status", Usage: "List migrations and whether they are applied", Action: migrate(migrateStatus)},
			{Name: "version", Usage: "Print the applied and expected versions", Action: migrate(migrateVersion)},
		},
	}
}

type migrateFunc func(ctx context.Context, p *goose.Provider, db *sql.DB) error

func migrate(fn migrateFunc) cli.ActionFunc {
	return func(ctx context.Context, _ *cli.Command) error {
		cfg, err := config.FromEnv(config.NeedMigrate)
		if err != nil {
			return err
		}
		log := logging.New(cfg.LogLevel, cfg.LogFormat, "migrate")

		db, err := sql.Open("pgx", cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()
		if err := db.PingContext(ctx); err != nil {
			return fmt.Errorf("ping database: %w", err)
		}

		p, err := migrations.Provider(db)
		if err != nil {
			return err
		}
		if err := fn(ctx, p, db); err != nil {
			return err
		}
		log.Debug().Msg("done")
		return nil
	}
}

// migrateUp applies pending migrations once the upgrade path is allowed,
// and records this release as the last to migrate the database.
func migrateUp(ctx context.Context, p *goose.Provider, db *sql.DB) error {
	last, err := migrations.LastRelease(ctx, db)
	if err != nil {
		return err
	}
	if err := migrations.CheckUpgrade(last, version.Version, version.MinUpgradeFrom); err != nil {
		return err
	}
	results, err := p.Up(ctx)
	for _, r := range results {
		fmt.Printf("applied %s (%s)\n", r.Source.Path, r.Duration.Round(1e6))
	}
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	if len(results) == 0 {
		fmt.Println("schema is up to date")
	}
	if last != version.Version {
		if err := migrations.RecordRelease(ctx, db, version.Version, version.Commit); err != nil {
			return err
		}
	}
	return nil
}

func migrateDown(ctx context.Context, p *goose.Provider, _ *sql.DB) error {
	r, err := p.Down(ctx)
	if err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	fmt.Printf("rolled back %s\n", r.Source.Path)
	return nil
}

func migrateStatus(ctx context.Context, p *goose.Provider, _ *sql.DB) error {
	statuses, err := p.Status(ctx)
	if err != nil {
		return fmt.Errorf("migrate status: %w", err)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "VERSION\tSTATE\tAPPLIED AT\tFILE")
	for _, s := range statuses {
		applied := "-"
		if !s.AppliedAt.IsZero() {
			applied = s.AppliedAt.Format("2006-01-02 15:04:05")
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", s.Source.Version, s.State, applied, s.Source.Path)
	}
	return w.Flush()
}

func migrateVersion(ctx context.Context, _ *goose.Provider, db *sql.DB) error {
	got, err := migrations.Current(ctx, db)
	if err != nil {
		return err
	}
	want, err := migrations.Expected()
	if err != nil {
		return err
	}
	last, err := migrations.LastRelease(ctx, db)
	if err != nil {
		return err
	}
	if last == "" {
		last = "none recorded"
	}
	fmt.Printf("applied %d, expected %d; last migrated by %s; this binary %s\n", got, want, last, version.String())
	return nil
}
