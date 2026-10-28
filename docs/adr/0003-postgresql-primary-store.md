# 0003. PostgreSQL as the primary store

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

Horizon is a multi-user server application with relational data, flexible
custom fields, full-text search and an event log.

## Decision

PostgreSQL is the only database. JSONB holds custom field values, event
payloads, settings and documents; `tsvector` and `pg_trgm` provide search
([0020](0020-postgresql-search.md)).

## Consequences

- One database to operate, back up and reason about.
- No SQLite: a self-hosted install always needs a PostgreSQL server.
- PostgreSQL-specific features (JSONB, GIN, `SKIP LOCKED`, `LISTEN/NOTIFY`)
  are used freely.

## Alternatives considered

- **SQLite**: zero-ops self-hosting, but weak concurrency for a realtime,
  multi-user tool.
- **Both**: a portability layer would cost on every query.
