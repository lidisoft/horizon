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

// Workspaces created before roles get the default roles, their first member
// the Admin role and the others Contributor, so nobody loses access in the
// upgrade.
func TestRolesBackfill(t *testing.T) {
	ctx := context.Background()
	pool := testenv.DB(t)
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	p, err := migrations.Provider(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.DownTo(ctx, 5); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, name, created_at, updated_at) VALUES
			('01900000-0000-7000-8000-000000000001', 'first@example.com', 'First', now(), now()),
			('01900000-0000-7000-8000-000000000002', 'second@example.com', 'Second', now(), now());
		INSERT INTO workspaces (id, slug, name, created_at, updated_at)
			VALUES ('01900000-0000-7000-8000-0000000000aa', 'old', 'Old', now(), now());
		INSERT INTO workspace_members (workspace_id, id, user_id, joined_at) VALUES
			('01900000-0000-7000-8000-0000000000aa', gen_random_uuid(), '01900000-0000-7000-8000-000000000001', now() - interval '1 day'),
			('01900000-0000-7000-8000-0000000000aa', gen_random_uuid(), '01900000-0000-7000-8000-000000000002', now());`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var roles, adminPerms int
	var admin, contributor string
	err = pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM roles),
		       (SELECT cardinality(permissions) FROM roles WHERE builtin = 'admin'),
		       (SELECT u.email FROM role_grants g JOIN roles r ON r.id = g.role_id JOIN users u ON u.id = g.user_id WHERE r.builtin = 'admin'),
		       (SELECT u.email FROM role_grants g JOIN roles r ON r.id = g.role_id JOIN users u ON u.id = g.user_id WHERE r.builtin = 'contributor')`).
		Scan(&roles, &adminPerms, &admin, &contributor)
	if err != nil {
		t.Fatal(err)
	}
	if roles != 5 || adminPerms != 28 || admin != "first@example.com" || contributor != "second@example.com" {
		t.Errorf("roles %d, admin permissions %d, admin %s, contributor %s", roles, adminPerms, admin, contributor)
	}
}

// Workspaces created before configuration get the defaults: categories, the
// Default workflow and the issue types.
func TestConfigurationBackfill(t *testing.T) {
	ctx := context.Background()
	pool := testenv.DB(t)
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	p, err := migrations.Provider(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.DownTo(ctx, 7); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, slug, name, created_at, updated_at)
		VALUES ('01900000-0000-7000-8000-0000000000bb', 'old', 'Old', now(), now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var cats, statuses, transitions, types, initials int
	var subParents int
	err = pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM status_categories),
		       (SELECT count(*) FROM statuses),
		       (SELECT count(*) FROM transitions),
		       (SELECT count(*) FROM issue_types),
		       (SELECT count(*) FROM statuses WHERE is_initial),
		       (SELECT cardinality(allowed_parent_type_ids) FROM issue_types WHERE name = 'Sub-task')`).
		Scan(&cats, &statuses, &transitions, &types, &initials, &subParents)
	if err != nil {
		t.Fatal(err)
	}
	if cats != 5 || statuses != 5 || transitions != 8 || types != 5 || initials != 1 || subParents != 3 {
		t.Errorf("backfill: %d categories, %d statuses, %d transitions, %d types, %d initial, sub-task parents %d",
			cats, statuses, transitions, types, initials, subParents)
	}
}

// Teal and orange, which LDS dropped, move to green and coral wherever a
// colour is stored, and projects can no longer take them.
func TestSwatchRemap(t *testing.T) {
	ctx := context.Background()
	pool := testenv.DB(t)
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	p, err := migrations.Provider(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.DownTo(ctx, 25); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO workspaces (id, slug, name, created_at, updated_at)
			VALUES ('01900000-0000-7000-8000-0000000000cc', 'old', 'Old', now(), now());
		INSERT INTO projects (workspace_id, id, key, name, colour, created_at, updated_at)
			VALUES ('01900000-0000-7000-8000-0000000000cc', gen_random_uuid(), 'OLD', 'Old', 'teal', now(), now());
		INSERT INTO labels (workspace_id, id, name, colour, created_at, updated_at)
			VALUES ('01900000-0000-7000-8000-0000000000cc', gen_random_uuid(), 'hot', 'orange', now(), now());
		INSERT INTO custom_fields (workspace_id, id, name, type, options, position, created_at, updated_at)
			VALUES ('01900000-0000-7000-8000-0000000000cc', gen_random_uuid(), 'Size', 'select',
				'{"options": [{"id": "a", "name": "S", "colour": "teal"}, {"id": "b", "name": "M", "colour": "blue"},
				  {"id": "c", "name": "L", "colour": "orange"}]}', 0, now(), now());`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var project, label, options string
	err = pool.QueryRow(ctx, `
		SELECT (SELECT colour FROM projects), (SELECT colour FROM labels),
		       (SELECT string_agg(o->>'colour', ',' ORDER BY o->>'id') FROM custom_fields, jsonb_array_elements(options->'options') o)`).
		Scan(&project, &label, &options)
	if err != nil {
		t.Fatal(err)
	}
	if project != "green" || label != "coral" || options != "green,blue,coral" {
		t.Errorf("project %s, label %s, options %s", project, label, options)
	}
	if _, err := pool.Exec(ctx, `UPDATE projects SET colour = 'orange'`); err == nil {
		t.Error("a project took orange")
	}
}
