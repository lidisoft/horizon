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
| `S3_PUBLIC_ENDPOINT` | Optional: where browsers reach the bucket, when it differs from `S3_ENDPOINT` (an internal address). Attachment URLs are presigned for it. `serve` adds a CORS rule for `BASE_URL` to the bucket; with a provider that manages CORS elsewhere, allow `PUT` and `GET` from that origin there |
| `SMTP_URL` | `smtp://user:pass@host:port`, plus `SMTP_FROM` |
| `BASE_URL` | Public URL; used in emails, OIDC redirects, webhooks |
| `SECRET_KEY` | Encrypts secrets at rest: 32 random bytes, base64 (`openssl rand -base64 32`). `SECRET_KEY_PREVIOUS` (comma-separated, same format) allows rotation: old keys decrypt, the current key re-encrypts |
| `SESSION_SECRET` | Signs CSRF tokens; at least 32 characters |
| `EVENTS_RETENTION` | JetStream retention, Go duration, default `168h` (7 days) |
| `LOG_LEVEL`, `LOG_FORMAT` | `info` and `json` by default; `LOG_FORMAT=console` for humans |
| `HTTP_ADDR`, `METRICS_ADDR` | Default `:8080`, `:9090` |
| `WEBHOOK_ALLOW_PRIVATE` | `true` lets webhooks reach loopback, private and link-local addresses (receivers on the instance's own network); default `false` |
| `FORGE_ALLOW_PRIVATE` | `true` lets Git forge API calls (pull request comments) reach loopback, private and link-local addresses: a self-hosted forge on the instance's network; default `false` |
| `OIDC_ALLOW_PRIVATE` | `true` lets OpenID Connect providers be reached on loopback, private and link-local addresses (a provider on the instance's network); default `false` |
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

On `SIGTERM` (or `SIGINT`):

- `serve` stops accepting connections and finishes in-flight requests within
  30 s. WebSockets close with code `1012` (service restart), so clients
  reconnect to another replica; new ones are refused (`503 shutting_down`).
- `worker` fetches no more events. The event in hand finishes and is
  acknowledged, within 25 s; one that fails or runs out of time is nak'ed at
  once, so another worker takes it. Periodic jobs stop; a webhook delivery
  or pull request comment cut short is retried when its lease ends.

Give processes at least 35 s between `SIGTERM` and `SIGKILL`.

## Upgrades

- Releases follow semantic versioning. The release notes state the minimum
  version an instance must be on before upgrading (no skipping across a
  minor version that contains a required data migration); the binary knows it
  too (`version.MinUpgradeFrom`).
- **Checked, not just documented.** `horizon_releases` records each release
  that ran `migrate up`. `migrate up` refuses to migrate a database last
  migrated by a release older than the minimum (upgrade through it first), or
  by a newer one (a downgrade). `serve` and `worker` refuse to start on a
  database migrated by a release more than one minor version ahead (the
  previous release keeps running during a rolling upgrade; older ones don't).
  `horizon migrate version` prints the applied and expected schema versions
  and the release that last migrated. Development builds (`dev`) skip the
  checks.
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

Restore order: object storage, then PostgreSQL, then `horizon admin
reconcile` (`--dry-run` first to see what it would do), then start `serve`
and `worker`. Reconcile:

- deletes objects no attachment references (older than an hour, so uploads
  in flight are spared), and lists attachments whose object is missing;
- empties the JetStream stream and deletes its consumers: the events after
  the restored point never happened for the database, so they must not reach
  webhooks, notifications or forges. Workers recreate their consumers and
  take what comes next; events the restored database hadn't published yet
  are published by the outbox relay. Report projections catch up from the
  database itself.

### Rotating SECRET_KEY

1. Set a new `SECRET_KEY` and move the old one to `SECRET_KEY_PREVIOUS`;
   roll `serve` and `worker` (old values still open, new ones use the new
   key).
2. Run `horizon admin rekey`: it reseals webhook secrets, Git connection
   tokens and secrets, and OIDC client secrets with the new key, and says
   how many.
3. Remove the old key from `SECRET_KEY_PREVIOUS`.

## Admin CLI

| Command | Does |
|---------|------|
| `horizon admin create-user` | Create a user (`--email`, `--name`, `--instance-admin`); prints a link to choose the password, or reads it with `--password-stdin` |
| `horizon admin reset-password` | Print a single-use reset link for a user, valid one hour |
| `horizon admin deactivate-user`, `reactivate-user` | Block or restore an account's sign-in instance-wide; deactivation revokes every session |
| `horizon admin create-workspace` | Create a workspace (`--slug`, `--name`, `--owner` email, `--time-zone`) with that account as first member |
| `horizon admin reconcile` | Post-restore consistency (see above); `--dry-run` reports only. Needs PostgreSQL, NATS and object storage |
| `horizon admin rekey` | Reseal stored secrets with the current `SECRET_KEY` (see [Rotating SECRET_KEY](#rotating-secret_key)); needs `SECRET_KEY`, `SECRET_KEY_PREVIOUS` and `SESSION_SECRET` |
| `horizon admin rebuild-reports` | Replay `events` into the report projections |

## Guarantees and limits

- Webhooks and emails are at-least-once.
- Realtime events are best-effort; clients detect gaps by version and refetch.
- No data leaves the instance except through configured SMTP, webhooks, Git
  forge integrations and OIDC.

## After v1

- Workspace export and import (moving a workspace between instances).
- Helm chart.
