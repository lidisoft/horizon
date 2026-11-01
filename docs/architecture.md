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
| Language | Go 1.26 (the `go` line in `go.mod`) |
| HTTP | Gin |
| Database driver | pgx v5 |
| Queries | sqlc (SQL in `internal/<module>/queries/*.sql`, generated code in `internal/<module>/db`; one entry per module in `sqlc.yaml`) |
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
  config/                 process config (env)
  server/                 Gin setup, middleware, routing, SPA + LDS serving
  platform/               pg pool, tx helper, NATS, S3, mailer, clock, ids
  eventlog/               event append, outbox relay, scrubbing, subjects
  identity/               users, sessions, passwords, magic links, OIDC, tokens
  access/                 roles, permission sets, grants, checks
  workspace/              workspaces, members, teams
  project/                projects, keys, sequences
  configdef/              status categories, workflows, issue types, fields, scales, labels; the workspace→project resolver
  issue/                  issues, links, hierarchy, ranking, moves, deletes, comments, attachments, activity
  planning/               iterations, releases, milestones
  view/                   saved views, filter AST → SQL compiler
  document/               block documents, collaborative steps
  realtime/               WebSocket hub, subscriptions, presence
  notify/                 inbox, subscriptions, email
  webhook/                endpoints, deliveries, signing
  vcs/                    git forge integrations
  search/                 PostgreSQL full-text search
  report/                 report projections, report queries, project overview
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

When a transaction that appended events commits, a statement-level trigger on
`events` notifies channel `events`, which wakes the outbox relay. The relay also
polls every second, so a missed notification only costs latency.

Commands write events through `eventlog.Append(ctx, tx, …)` inside the same
`postgres.InTx` call as their state change.

## Domain events and the outbox

- `events` holds every event: `id` (UUIDv7, time-ordered), `workspace_id`
  (null for instance-level events such as users and workspaces themselves),
  `aggregate_type`, `aggregate_id`, `aggregate_version`, `type`, `actor_id`,
  `occurred_at`, `payload` (JSONB), `published_at`.
- **Names**: `aggregate_type` is lower snake case (`issue`,
  `status_category`); `type` is the event storm's name in PascalCase
  (`IssueStatusUpdated`). Both are valid subject tokens. The public name used
  by the API, WebSockets and webhooks derives from them: `issue.status_updated`
  ([api.md](api.md#domain-event-format)). Avoid acronyms in event names
  (`IssueUrlChanged`, not `IssueURLChanged`).
- **Payloads** carry ids and changed values only, never another aggregate's
  content, so they survive its deletion ([ADR-0012](adr/0012-hard-delete-with-event-scrubbing.md)).
- The **outbox relay** (in `worker`) claims unpublished rows in id order using
  `FOR UPDATE SKIP LOCKED`, publishes them to JetStream, and sets
  `published_at` in the same transaction. Several relays can run at once.
  Publishing is at-least-once: a failed batch is retried whole, JetStream drops
  copies within its 2-minute duplicate window (`Nats-Msg-Id` = event id), and
  consumers dedupe by event id after that. Order is by id across relays only
  roughly; consumers order one aggregate's events by `aggregate_version`.
- **Stream**: `HORIZON_EVENTS`, subjects `horizon.*.*.*.*`, file storage,
  `MaxAge` = `HORIZON_EVENTS_RETENTION`, created or updated by the worker at
  startup.
- **Subject scheme**:
  `horizon.<workspace_id>.<aggregate_type>.<aggregate_id>.<event_type>`, with
  `instance` in place of the workspace id for instance-level events.
  Including the aggregate id makes hard-delete scrubbing a single
  subject-filtered purge.
- **Message body**: the stored row as JSON (`id`, `workspace_id`,
  `aggregate_type`, `aggregate_id`, `aggregate_version`, `type`, `actor_id`,
  `occurred_at`, `payload`), ids only. Consumers enrich it (workspace slug,
  issue key, actor name) where it leaves the system.
- The `events` table is the **activity history** shown in the UI. JetStream
  retention is bounded (`HORIZON_EVENTS_RETENTION`, default 7 days) because PostgreSQL is
  the long-term record.

## Read path

- REST reads go straight to PostgreSQL through sqlc queries.
- Views compile their filter tree to parameterised SQL (`internal/view`);
  custom fields use JSONB operators backed by GIN indexes.
- Search uses PostgreSQL only ([ADR-0020](adr/0020-postgresql-search.md)):
  `tsvector` columns for word search on issues and comments, and `pg_trgm`
  trigram indexes on titles and keys for typo-tolerant, prefix matching in ⌘K.
  The `tsvector` columns are updated in the write transaction.

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
| `purge` | Scrubs JetStream subjects and S3 objects after hard deletes (a periodic job over the `purges` queue; it keeps tombstones) |
| `reports` | Maintains report projections ([reports.md](reports.md), [ADR-0021](adr/0021-report-projections.md)) |

## Object storage

Attachments go to S3-compatible storage only. Uploads use presigned PUT URLs:
the client announces the file (name, size, type), PUTs the bytes, then
completes the upload; Horizon checks the stored size and sniffs the content
type. Downloads go through a Horizon endpoint that checks permissions and
redirects to a short-lived presigned GET. Keys:
`ws/<workspace_id>/att/<attachment_id>`.

Browsers reach the bucket at `S3_PUBLIC_ENDPOINT` (default `S3_ENDPOINT`):
URLs are presigned for it, the page's CSP allows it in `connect-src` and
`img-src`, and `serve` adds a CORS rule for `BASE_URL`'s origin to the bucket
at startup (keeping the bucket's other rules). Uploads never completed expire
after a day (`upload-expiry` job); deleted attachments' objects go through the
`purge` job.

## Configuration of the process

Environment variables, prefixed `HORIZON_` (as in `tocsin`):
`DATABASE_URL`, `NATS_URL`, `S3_ENDPOINT`/`S3_BUCKET`/`S3_ACCESS_KEY`/
`S3_SECRET_KEY`, `SMTP_URL`, `BASE_URL`, `SESSION_SECRET`, `LOG_LEVEL`,
`EVENTS_RETENTION` (JetStream, default `168h`). The full operator contract is
in [operations.md](operations.md).

## Development environment

Mirrors `tocsin` and LDS: `docker compose` with PostgreSQL, NATS (JetStream
on), RustFS and Mailpit, plus the app under Air for hot reload.

Tasks run with [Task](https://taskfile.dev) (`Taskfile.yml`), not Make — see
[ADR-0019](adr/0019-task-runner.md). `task --list` is the reference:

| Task | Does |
|------|------|
| `task up` / `task down` / `task logs` | Docker Compose environment |
| `task test` | All tests, including integration tests with testcontainers ([development.md](development.md#tests)) |
| `task test:unit` | Unit tests only, no Docker |
| `task check` | fmt, vet, tests, generated-code freshness — run before finishing |
| `task migrate:up` / `migrate:down` / `migrate:status` | goose via `horizon migrate` |
| `task migrate:create -- <name>` | New numbered migration in `internal/migrations/` |
| `task gen` | All code generation (`gen:sqlc`, `gen:swagger`) |

## Observability

zerolog JSON logs with request id and workspace id; `/healthz` (liveness) and
`/readyz` (database and NATS); Prometheus metrics at `/metrics` (requests,
outbox lag, consumer lag, socket count).
