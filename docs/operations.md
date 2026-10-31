# Operations: the instance contract

What anyone running a Horizon instance can rely on, and what they must
provide. Self-hosters read it as an install guide. For SaaS, the
provisioning, upgrade and backup automation for one instance per customer
lives in a **separate operations repository**, built against this contract.

## What an instance is

| Component | Required | Notes |
|-----------|----------|-------|
| `horizon` | Yes | One OCI image (`ghcr.io/lidisoft/horizon`), non-root, single static binary. Runs `serve`, `worker`, or both |
| PostgreSQL | Yes | Supported versions documented per release. Extensions: `pg_trgm`, `citext` |
| NATS with JetStream | Yes | Streams and consumers are created by Horizon at startup |
| S3-compatible storage | Yes | One bucket per instance (RustFS for self-hosting) |
| SMTP | Yes | For invitations, magic links, notifications |
| OIDC providers | No | Configured in the app |

Horizon keeps **no local state** on disk. Every `serve` and `worker` process is
disposable.

## Configuration

Environment variables only, prefixed `HORIZON_`. Startup fails fast, listing
every missing or invalid variable. `migrate` and `admin` need only `DATABASE_URL`
(`admin` also reads `BASE_URL` to print links); `serve` and `worker` need the
rest.

| Variable | Notes |
|----------|-------|
| `DATABASE_URL` | PostgreSQL DSN |
| `NATS_URL` | NATS URL, with credentials if any |
| `S3_ENDPOINT`, `S3_BUCKET`, `S3_REGION`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` | Object storage. `S3_ENDPOINT` is a URL (`https://` enables TLS); `S3_REGION` defaults to `us-east-1`. The bucket must exist |
| `SMTP_URL` | `smtp://user:pass@host:port`, plus `SMTP_FROM` |
| `BASE_URL` | Public URL; used in emails, OIDC redirects, webhooks |
| `SECRET_KEY` | Encrypts secrets at rest: 32 random bytes, base64 (`openssl rand -base64 32`). `SECRET_KEY_PREVIOUS` (comma-separated, same format) allows rotation: old keys decrypt, the current key re-encrypts |
| `SESSION_SECRET` | Signs CSRF tokens; at least 32 characters |
| `EVENTS_RETENTION` | JetStream retention, Go duration, default `168h` (7 days) |
| `LOG_LEVEL`, `LOG_FORMAT` | `info` and `json` by default; `LOG_FORMAT=console` for humans |
| `HTTP_ADDR`, `METRICS_ADDR` | Default `:8080`, `:9090` |
| `TRUSTED_PROXIES` | Comma-separated IPs or CIDR prefixes of reverse proxies allowed to set `X-Forwarded-For`; default none, so the client IP is the connection's. Set it behind a proxy, or sign-in throttling counts every user as the proxy's address |

## Lifecycle

### First start

```bash
horizon migrate up
horizon admin create-user --email admin@example.com --instance-admin
horizon serve --with-worker
```

### Health

| Endpoint | Meaning |
|----------|---------|
| `GET /healthz` | Process is alive |
| `GET /readyz` | PostgreSQL, NATS and S3 reachable, schema at the expected version. Use for load balancer and rollout gates |
| `GET /metrics` (on `METRICS_ADDR`) | Prometheus: requests, outbox lag, consumer lag, open sockets, job failures |

### Shutdown

On `SIGTERM`, `serve` stops accepting connections, closes WebSockets with code
`1012` (service restart) so clients reconnect elsewhere, and finishes
in-flight requests within 30 s. `worker` stops pulling and acknowledges or
naks in-flight messages.

## Upgrades

- Releases follow semantic versioning. The release notes state the minimum
  version an instance must be on before upgrading (no skipping across a
  minor version that contains a required data migration).
- **Migrations are expand/contract**: a release's migrations are compatible
  with the previous release's code. A rolling upgrade is therefore:
  1. `horizon migrate up` (with the new image), while old processes run;
  2. roll `serve` and `worker` to the new image.
  Destructive steps (dropping columns) land one release later.
- `horizon migrate status` reports pending migrations; `/readyz` fails if the
  schema is **older** than the code expects, not if it is newer.
- Downgrades aren't supported once a release's migrations have run; restore
  from backup instead.

## Backup and restore

What to back up:

| Data | How | Notes |
|------|-----|-------|
| PostgreSQL | `pg_dump` or continuous archiving (PITR) | The source of truth, including the event history |
| Object storage | Bucket versioning or replication | Attachments |
| NATS | Not needed | JetStream is a transport; the outbox republishes anything unpublished |
| Configuration | Environment / secret store | `SECRET_KEY` is required to read encrypted secrets — losing it means re-entering OIDC, VCS and webhook secrets |

Restore order: object storage, then PostgreSQL, then start Horizon. After a
restore, `horizon admin reconcile` removes object-storage files no database
row references, and resets JetStream consumers to the restored outbox
position.

## Admin CLI

| Command | Does |
|---------|------|
| `horizon admin create-user` | Create a user (`--email`, `--name`, `--instance-admin`); prints a link to choose the password, or reads it with `--password-stdin` |
| `horizon admin reset-password` | Print a single-use reset link for a user, valid one hour |
| `horizon admin deactivate-user`, `reactivate-user` | Block or restore an account's sign-in instance-wide; deactivation revokes every session |
| `horizon admin create-workspace` | Create a workspace (`--slug`, `--name`, `--owner` email, `--time-zone`) with that account as first member |
| `horizon admin reconcile` | Post-restore consistency (see above) |
| `horizon admin rekey` | Re-encrypt secrets with the current `SECRET_KEY` |
| `horizon admin rebuild-reports` | Replay `events` into the report projections |

## Guarantees and limits

- Webhooks and emails are at-least-once.
- Realtime events are best-effort; clients detect gaps by version and refetch.
- No data leaves the instance except through configured SMTP, webhooks, Git
  forge integrations and OIDC.

## After v1

- Workspace export and import (moving a workspace between instances).
- Helm chart.
