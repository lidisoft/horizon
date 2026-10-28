# Architecture

## System context

```
             ┌───────────────────────────── Instance ─────────────────────────────┐
 Browser ───▶│  horizon serve  (Gin: REST API, WebSocket, SPA, /lds/ assets)       │
 (SPA)  ◀──ws│        │                    ▲                                       │
             │        ▼ tx: state + event  │ NATS core (fan-out)                   │
             │   PostgreSQL ──outbox relay─▶ NATS JetStream ──▶ horizon worker      │
             │                                                   │ webhooks        │
             │   S3-compatible storage ◀── attachments           │ email (SMTP)    │
             │                                                   │ git sync        │
             └───────────────────────────────────────────────────┴─────────────────┘
```

External systems: SMTP server, OIDC identity providers, Git forges (GitHub,
GitLab, Forgejo), webhook receivers.

## Processes

One binary, `horizon`, with subcommands (urfave/cli, as in `tocsin`):

| Command | Role |
|---------|------|
| `horizon serve` | HTTP API, WebSockets, SPA and LDS assets. Stateless, horizontally scalable |
| `horizon worker` | Consumes JetStream: outbox relay, webhooks, notifications, email, git sync, search indexing, purges |
| `horizon migrate up\|down\|status` | goose migrations, embedded |
| `horizon admin …` | Bootstrap instance admin, create workspace, reset password |

`serve` and `worker` can run in the same process for small installs
(`horizon serve --with-worker`).

## Backend stack

| Concern | Choice |
|---------|--------|
| Language | Go (same version line as `tocsin`) |
| HTTP | Gin |
| Database driver | pgx v5 |
| Queries | sqlc (SQL in `internal/<module>/queries/*.sql`) |
| Migrations | goose, embedded in the binary |
| Messaging | NATS + JetStream |
| Logging | zerolog |
| CLI | urfave/cli v3 |
| API docs | swag → OpenAPI, served at `/swagger/` |
| Object storage | S3 API (RustFS in development and self-hosted) |
| Design system | `github.com/lidisoft/design-system`, embedded and served at `/lds/` |

## Module layout

Modules follow the domain, not technical layers. Each owns its handlers,
service, queries and events.

```
cmd/horizon/              entry point
internal/
  cli/                    command wiring
  config/                 process config (env) AND the workspace→project resolver
  server/                 Gin setup, middleware, routing, SPA + LDS serving
  platform/               pg pool, tx helper, NATS, S3, mailer, clock, ids
  eventlog/               event append, outbox relay, scrubbing, subjects
  identity/               users, sessions, passwords, magic links, OIDC, tokens
  access/                 roles, permission sets, grants, checks
  workspace/              workspaces, members, teams
  project/                projects, keys, sequences
  configdef/              issue types, workflows, fields, scales, labels
  issue/                  issues, links, hierarchy, ranking, comments
  planning/               iterations, releases, milestones
  view/                   saved views, filter AST → SQL compiler
  document/               block documents, collaborative steps
  realtime/               WebSocket hub, subscriptions, presence
  attachment/             uploads, S3
  notify/                 inbox, subscriptions, email
  webhook/                endpoints, deliveries, signing
  vcs/                    git forge integrations
  search/                 PostgreSQL full-text search
  i18n/                   message catalogs (server-side strings: emails)
  migrations/             goose SQL files
  docs/                   generated OpenAPI
web/                      the SPA (see frontend.md), embedded via go:embed
```

## Write path

Every command follows the same steps, inside **one PostgreSQL transaction**:

1. Authenticate; resolve workspace and project; check the permission key.
2. Load the aggregate `FOR UPDATE` (or check `version` for optimistic writes).
3. Validate invariants against resolved configuration.
4. Write the new state; increment the aggregate's `version`.
5. Append one or more domain events to `events` (which is also the outbox).
6. Commit.

After commit, `pg_notify('events', id)` nudges the outbox relay (it also polls,
so a missed notification is only latency).

## Domain events and the outbox

- `events` holds every event: `id` (UUIDv7, time-ordered), `workspace_id`,
  `aggregate_type`, `aggregate_id`, `aggregate_version`, `type`, `actor_id`,
  `occurred_at`, `payload` (JSONB), `published_at`.
- The **outbox relay** (in `worker`) reads unpublished rows in id order using
  `FOR UPDATE SKIP LOCKED`, publishes to JetStream, then sets `published_at`.
  Publishing is at-least-once; consumers dedupe by event id (JetStream's
  `Nats-Msg-Id` header).
- **Subject scheme**:
  `horizon.<workspace_id>.<aggregate_type>.<aggregate_id>.<event_type>`.
  Including the aggregate id makes hard-delete scrubbing a single
  subject-filtered purge.
- The `events` table is the **activity history** shown in the UI. JetStream
  retention is bounded (configurable, default 30 days) because PostgreSQL is
  the long-term record.

## Read path

- REST reads go straight to PostgreSQL through sqlc queries.
- Views compile their filter tree to parameterised SQL (`internal/view`);
  custom fields use JSONB operators backed by GIN indexes.
- Full-text search uses PostgreSQL `tsvector` columns on issues and comments,
  updated in the write transaction.

## Realtime fan-out

Each `serve` replica subscribes to `horizon.>` on NATS core (not durable) and
pushes matching events to its connected sockets, after a permission check.
Details in [realtime.md](realtime.md).

## Workers

JetStream durable consumers, one per concern, each idempotent:

| Consumer | Does |
|----------|------|
| `notify` | Computes recipients (assignee, subscribers, mentions), writes inbox items, queues email |
| `email` | Sends via SMTP, with digests and retries |
| `webhook` | Matches subscriptions, POSTs signed payloads (HMAC-SHA256), retries with backoff, logs deliveries |
| `vcs` | Processes forge webhooks: links branches/PRs/commits, applies merge transitions |
| `policy` | Runs cross-aggregate policies (e.g. iteration completion) as new commands |
| `purge` | Scrubs JetStream subjects and S3 objects after hard deletes |

## Object storage

Attachments go to S3-compatible storage only. Uploads use presigned PUT URLs.
Downloads go through a Horizon endpoint that checks permissions and redirects
to a short-lived presigned GET. Keys: `ws/<workspace_id>/att/<attachment_id>`.

## Configuration of the process

Environment variables, prefixed `HORIZON_` (as in `tocsin`):
`DATABASE_URL`, `NATS_URL`, `S3_ENDPOINT`/`S3_BUCKET`/`S3_ACCESS_KEY`/
`S3_SECRET_KEY`, `SMTP_URL`, `BASE_URL`, `SESSION_SECRET`, `LOG_LEVEL`.

## Development environment

Mirrors `tocsin` and LDS: `docker compose` with PostgreSQL, NATS (JetStream
on), RustFS and Mailpit, plus the app under Air for hot reload.

Tasks run with [Task](https://taskfile.dev) (`Taskfile.yml`), not Make — see
[ADR-0019](adr/0019-task-runner.md). `task --list` is the reference:

| Task | Does |
|------|------|
| `task up` / `task down` / `task logs` | Docker Compose environment |
| `task test` | `go test ./...` |
| `task check` | fmt, vet, tests, generated-code freshness — run before finishing |
| `task migrate:up` / `migrate:down` / `migrate:status` | goose via `horizon migrate` |
| `task migrate:create -- <name>` | New numbered migration in `internal/migrations/` |
| `task gen` | All code generation (`gen:sqlc`, `gen:swagger`) |

## Observability

zerolog JSON logs with request id and workspace id; `/healthz` (liveness) and
`/readyz` (database and NATS); Prometheus metrics at `/metrics` (requests,
outbox lag, consumer lag, socket count).
