// Package testenv starts the real dependencies integration tests run against
// (ADR-0022): PostgreSQL, NATS with JetStream and headless Chrome, in
// containers shared by every test of a package.
//
// Each test gets its own database, cloned from a template that has every
// migration applied, and its own JetStream stream and subject prefix, so
// tests run in parallel without cleanup.
//
//	func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres|testenv.NATS)) }
//
//	func TestSomething(t *testing.T) {
//		pool := testenv.DB(t)
//		...
//	}
//
// With -short (task test:unit) no container starts and these helpers skip
// the calling test.
package testenv

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"
	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/migrations"
)

// Service selects the containers a package needs.
type Service uint8

// Services.
const (
	Postgres Service = 1 << iota
	NATS
	// S3 is RustFS object storage (S3); each test gets its own bucket.
	S3Storage
	// Chrome is headless Chrome for browser tests (Browser, Serve). Set
	// HORIZON_TEST_CHROME to a local binary to skip the container.
	Chrome
	// Forgejo is a Git forge (Forgejo), with an admin and a token (Forge).
	Forgejo
	// Dex is an OpenID Connect provider (OIDC).
	Dex
)

// Images match docker-compose.yml.
const (
	postgresImage = "postgres:17-alpine"
	natsImage     = "nats:2-alpine"
	templateDB    = "horizon_template"
)

type env struct {
	adminURL string // superuser connection to the postgres database
	dbURL    func(name string) string
	admin    *pgxpool.Pool
	createMu sync.Mutex // CREATE DATABASE … TEMPLATE needs the template idle

	natsURL string

	s3Endpoint *url.URL

	browser *browserEnv
	forge   *ForgeEnv
	dex     *OIDCEnv
}

var current *env

// Main starts the requested services, runs the package's tests and stops the
// containers. Use it as the body of TestMain.
func Main(m *testing.M, services Service) int {
	flag.Parse()
	if testing.Short() {
		return m.Run()
	}

	ctx := context.Background()
	e := &env{}
	var containers []testcontainers.Container
	defer func() {
		if e.admin != nil {
			e.admin.Close()
		}
		if e.browser != nil {
			e.browser.cancel()
		}
		for _, c := range containers {
			_ = testcontainers.TerminateContainer(c)
		}
	}()

	if services&Postgres != 0 {
		c, err := tcpostgres.Run(ctx, postgresImage,
			tcpostgres.WithDatabase("postgres"),
			tcpostgres.WithUsername("horizon"),
			tcpostgres.WithPassword("horizon"),
			tcpostgres.BasicWaitStrategies(),
		)
		if c != nil {
			containers = append(containers, c)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "testenv: postgres:", err)
			return 1
		}
		if err := e.setupPostgres(ctx, c); err != nil {
			fmt.Fprintln(os.Stderr, "testenv: postgres:", err)
			return 1
		}
	}

	if services&NATS != 0 {
		c, err := tcnats.Run(ctx, natsImage)
		if c != nil {
			containers = append(containers, c)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "testenv: nats:", err)
			return 1
		}
		if e.natsURL, err = c.ConnectionString(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "testenv: nats:", err)
			return 1
		}
	}

	if services&S3Storage != 0 {
		c, err := e.startS3(ctx)
		if c != nil {
			containers = append(containers, c)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "testenv: s3:", err)
			return 1
		}
	}

	if services&Chrome != 0 {
		cs, err := e.startChrome(ctx)
		containers = append(containers, cs...)
		if err != nil {
			fmt.Fprintln(os.Stderr, "testenv: chrome:", err)
			return 1
		}
	}

	if services&Forgejo != 0 {
		cs, err := e.startForgejo(ctx)
		containers = append(containers, cs...)
		if err != nil {
			fmt.Fprintln(os.Stderr, "testenv: forgejo:", err)
			return 1
		}
	}

	if services&Dex != 0 {
		cs, err := e.startDex(ctx)
		containers = append(containers, cs...)
		if err != nil {
			fmt.Fprintln(os.Stderr, "testenv: dex:", err)
			return 1
		}
	}

	current = e
	return m.Run()
}

func (e *env) setupPostgres(ctx context.Context, c *tcpostgres.PostgresContainer) error {
	var err error
	if e.adminURL, err = c.ConnectionString(ctx, "sslmode=disable"); err != nil {
		return err
	}
	host, err := c.Host(ctx)
	if err != nil {
		return err
	}
	port, err := c.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return err
	}
	e.dbURL = func(name string) string {
		return fmt.Sprintf("postgres://horizon:horizon@%s:%s/%s?sslmode=disable", host, port.Port(), name)
	}
	if e.admin, err = pgxpool.New(ctx, e.adminURL); err != nil {
		return err
	}
	if _, err := e.admin.Exec(ctx, "CREATE DATABASE "+templateDB); err != nil {
		return fmt.Errorf("create template: %w", err)
	}

	db, err := sql.Open("pgx", e.dbURL(templateDB))
	if err != nil {
		return err
	}
	defer db.Close() // the template must have no connections when cloned
	p, err := migrations.Provider(db)
	if err != nil {
		return err
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("migrate template: %w", err)
	}
	return nil
}

func randomName(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func require(t testing.TB, ok bool, service string) {
	t.Helper()
	if current == nil || !ok {
		if testing.Short() {
			t.Skip("integration test: needs " + service + " (skipped with -short)")
		}
		t.Fatalf("testenv: %s not started; add it to testenv.Main in TestMain", service)
	}
}

// DB returns a pool on a fresh, fully migrated database dropped after the test.
func DB(t testing.TB) *pgxpool.Pool {
	t.Helper()
	require(t, current != nil && current.admin != nil, "PostgreSQL")
	e := current
	ctx := context.Background()
	name := randomName("t_")

	e.createMu.Lock()
	_, err := e.admin.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, templateDB))
	e.createMu.Unlock()
	if err != nil {
		t.Fatalf("testenv: create database: %v", err)
	}

	pool, err := pgxpool.New(ctx, e.dbURL(name))
	if err != nil {
		t.Fatalf("testenv: connect: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, err := e.admin.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name)); err != nil {
			t.Logf("testenv: drop database %s: %v", name, err)
		}
	})
	return pool
}

// Bus is a test's slice of NATS: its own connection, stream and subject prefix.
type Bus struct {
	Conn      *natsgo.Conn
	JetStream jetstream.JetStream
	Stream    eventlog.Stream
}

// JetStream returns a connection and a stream definition unique to the test.
// The stream is deleted after the test.
func JetStream(t testing.TB) Bus {
	t.Helper()
	require(t, current != nil && current.natsURL != "", "NATS")
	nc, err := natsgo.Connect(current.natsURL)
	if err != nil {
		t.Fatalf("testenv: nats: %v", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("testenv: jetstream: %v", err)
	}
	name := randomName("t")
	bus := Bus{
		Conn:      nc,
		JetStream: js,
		Stream:    eventlog.Stream{Name: "T_" + name, Prefix: name, Retention: time.Hour},
	}
	t.Cleanup(func() {
		_ = js.DeleteStream(context.Background(), bus.Stream.Name)
		nc.Close()
	})
	return bus
}
