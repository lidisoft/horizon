# Development

How to work on Horizon day to day: environment, tasks, tests, CI and
conventions. The environment itself is described in
[architecture.md](architecture.md#development-environment).

## Prerequisites

Go (version from `go.mod`), [Task](https://taskfile.dev), Docker (Compose and
testcontainers), and a Chromium for headless browser tests. Code generators
(swag, and sqlc later) are Go tool dependencies run with `go tool`, so there is
nothing else to install.

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
| Frontend units | `store`, `rank`, editor steps and mapping, filter evaluation | In-browser runner `web/test/index.html`, run headless by `chromedp` ([frontend.md](frontend.md#testing)) |
| End to end | Key user flows through the SPA | `chromedp` against a real `horizon serve` |

Rules:

- One container set per test package (started in `TestMain`), not per test.
- Each test gets its **own database**, created from a template database that
  has all migrations applied (`CREATE DATABASE … TEMPLATE …`), so tests run in
  parallel without cleanup.
- NATS subjects and S3 key prefixes are namespaced per test.
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
