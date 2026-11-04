# Architecture Decision Records

Each significant decision gets one numbered file, following
[Michael Nygard's format](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions):
context, decision, consequences. ADRs are immutable once accepted: to change a
decision, write a new ADR that **supersedes** the old one and update the old
one's status line.

New ADR: copy [template.md](template.md) to `NNNN-short-title.md` with the next
number.

| # | Decision | Status |
|---|----------|--------|
| [0001](0001-record-architecture-decisions.md) | Record architecture decisions | Accepted |
| [0002](0002-go-backend-gin-pgx-sqlc-goose.md) | Go backend with Gin, pgx, sqlc and goose | Accepted |
| [0003](0003-postgresql-primary-store.md) | PostgreSQL as the primary store | Accepted |
| [0004](0004-state-tables-with-domain-event-log.md) | State tables with a domain event log, not event sourcing | Accepted |
| [0005](0005-nats-jetstream-backbone.md) | NATS JetStream as the messaging backbone | Accepted |
| [0006](0006-spa-native-js-no-dependencies.md) | SPA in native JavaScript with no third-party code | Accepted |
| [0007](0007-lidisoft-design-system.md) | LiDiSoft Design System for all UI | Accepted |
| [0008](0008-websockets-for-realtime.md) | WebSockets for realtime | Accepted |
| [0009](0009-instance-per-customer-multi-workspace.md) | One instance per customer, several workspaces per instance | Accepted |
| [0010](0010-projects-own-issues.md) | Projects own issues; teams are permission groups | Accepted |
| [0011](0011-workspace-defaults-project-overrides.md) | Workspace defaults, project overrides | Accepted |
| [0012](0012-hard-delete-with-event-scrubbing.md) | Hard delete with event scrubbing and a tombstone | Accepted |
| [0013](0013-configurable-roles-and-permission-sets.md) | Configurable roles built from permission sets | Accepted |
| [0014](0014-workflow-state-machines-with-categories.md) | Workflow state machines per issue type, statuses in configurable categories | Accepted |
| [0015](0015-block-editor-server-authoritative-collab.md) | Block editor with server-authoritative collaborative editing | Accepted |
| [0016](0016-s3-compatible-storage-only.md) | S3-compatible object storage only | Accepted |
| [0017](0017-english-only-i18n-ready.md) | English only, i18n-ready | Accepted |
| [0018](0018-standalone-spa-shell-ready.md) | Standalone SPA, ready for the LiDiSoft shell | Accepted |
| [0019](0019-task-runner.md) | Task instead of Make | Accepted |
| [0020](0020-postgresql-search.md) | Search with PostgreSQL full-text search and pg_trgm | Accepted |
| [0021](0021-report-projections.md) | Report projections fed by domain events | Accepted |
| [0022](0022-integration-tests-real-dependencies.md) | Integration tests against real dependencies | Accepted |
| [0023](0023-secrets-at-rest.md) | Secrets at rest sealed with SECRET_KEY | Accepted |
