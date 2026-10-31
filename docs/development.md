# Development

How to work on Horizon day to day: environment, tasks, tests, CI and
conventions. The environment itself is described in
[architecture.md](architecture.md#development-environment).

## Prerequisites

Go (version from `go.mod`), [Task](https://taskfile.dev) and Docker (Compose
and testcontainers). Browser tests run headless Chrome in a container; set
`HORIZON_TEST_CHROME` to a local Chrome or Chromium binary to use it instead. Code generators
(sqlc and swag) are Go tool dependencies run with `go tool`, so there is
nothing else to install; sqlc needs cgo, so a C compiler (gcc) must be present.

## Development stack

`task up` builds and starts the stack: PostgreSQL, NATS (JetStream), RustFS,
Mailpit, a one-shot `migrate`, a one-shot bucket creation, and the app under
Air (`serve --with-worker`, rebuilt on change). Host ports avoid `tocsin`'s so
both stacks can run at once; change them in `.env` (see `.env.example`).

| Service | Host port | Notes |
|---------|-----------|-------|
| App | 8090 | API, `/healthz`, `/readyz`, `/swagger/` |
| Metrics | 9091 | `/metrics` |
| PostgreSQL | 5433 | `horizon` / `horizon`, database `horizon` |
| NATS | 4223, monitoring 8223 | JetStream on |
| RustFS | 9100, console 9101 | `horizon` / `horizon-dev-secret`, bucket `horizon` |
| Mailpit | 8025 (UI), 1025 (SMTP) | Catches every email |

Host-side tasks (`task migrate:*`) reach PostgreSQL on 5433; override with
`HORIZON_DATABASE_URL`.

## Tests

| Layer | What | How |
|-------|------|-----|
| Unit | Pure domain logic: invariants, workflow rules, configuration resolution, filter compiler, rank keys, document steps | Plain `go test`, no I/O |
| Integration | Command handlers, queries, outbox relay, workers, permissions | Real PostgreSQL, NATS (JetStream) and RustFS started by **testcontainers-go** ([ADR-0022](adr/0022-integration-tests-real-dependencies.md)) |
| Frontend units | `store`, `router`, `api`, `i18n`, `shortcuts`, later `rank`, editor steps and filter evaluation | In-browser runner (`web/test/`), run headless by `chromedp` in `web/browser_test.go` ([frontend.md](frontend.md#testing)) |
| End to end | Key user flows through the SPA | `chromedp` against the real server engine and page (`TestAppShell`) |

Rules:

- One container set per test package (started in `TestMain`), not per test:
  `os.Exit(testenv.Main(m, testenv.Postgres|testenv.NATS))`
  (`internal/testenv`).
- Each test gets its **own database**, created from a template database that
  has all migrations applied (`CREATE DATABASE … TEMPLATE …`), so tests run in
  parallel without cleanup.
  `testenv.DB(t)` returns a pool on a fresh copy, dropped after the test.
- NATS subjects and S3 key prefixes are namespaced per test:
  `testenv.JetStream(t)` gives each test its own stream name and subject
  prefix.
- Integration tests skip themselves under `-short`, which is how
  `task test:unit` avoids Docker.
- Browser tests use `testenv.Chrome`: `testenv.Browser(t)` opens a tab and
  `testenv.Serve(t, handler)` returns a URL that tab can reach. On Linux the
  Chrome container shares the host network, with DevTools and test servers on
  127.0.0.1 only; on Docker Desktop it reaches them through
  `host.docker.internal`.
- Browser tests fail on any uncaught exception, console error or CSP
  violation, except entries in `knownIssues` (each names its upstream fix).
- **Command tests assert both the new state and the emitted events**
  ([ADR-0004](adr/0004-state-tables-with-domain-event-log.md)).
- Go and JS implementations of shared algorithms (document steps, rank keys,
  filter evaluation) run the same JSON test vectors from `testdata/`.
- `task test` runs everything; `task test:unit` skips containers.

## CI

GitHub Actions:

| Workflow | Trigger | Does |
|----------|---------|------|
| `check` | Pull requests, pushes to `main` | `task check`: gofmt, vet, generated code freshness (sqlc, swagger), unit and integration tests, headless browser tests |
| `release` | Tags `v*` | `task check`, then build and push the OCI image to `ghcr.io/lidisoft/horizon`, and publish a GitHub release with notes generated from conventional commits |

## Branches, commits and pull requests

- **Branches**: `dev/<user>/<ticket>-<slug>`, e.g.
  `dev/rli/lds-22-event-storm`.
- **Tickets**: tracked in the current LiDiSoft tracker (`LDS-nn`) until Horizon
  can host its own work; then tickets move to Horizon itself (`HZN-nn`).
- **Commits**: [Conventional Commits](https://www.conventionalcommits.org),
  as in LDS: `type(scope): summary`. Types: `feat`, `fix`, `docs`, `refactor`,
  `test`, `chore`, `ci`, `perf`. Scopes are Go modules or web areas
  (`issue`, `planning`, `web/board`, `realtime`, …).
- **Pull requests** target `main`, need a green `check`, and are
  squash-merged with a conventional title.
- An architectural change comes with an ADR in `docs/adr/`.
