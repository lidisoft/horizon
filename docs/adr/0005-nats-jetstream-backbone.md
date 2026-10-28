# 0005. NATS JetStream as the messaging backbone

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

Events must reach WebSocket clients on every `serve` replica, and background
consumers (webhooks, email, git sync, notifications). `tocsin` already uses
NATS.

## Decision

Use **NATS with JetStream**. The outbox relay publishes each event to
`horizon.<workspace>.<aggregate_type>.<aggregate_id>.<event_type>`. Workers use
durable JetStream consumers; `serve` replicas use plain subscriptions for
fan-out. JetStream retention is bounded (configurable per instance, default 7 days); PostgreSQL holds
the long-term record.

## Consequences

- Every install needs a NATS server in addition to PostgreSQL and object
  storage.
- Consistent with `tocsin`; shared operational knowledge.
- Hard deletes must purge JetStream subjects too, which the subject scheme
  makes a single filtered purge.

## Alternatives considered

- **PostgreSQL only** (`LISTEN/NOTIFY` and a `SKIP LOCKED` job queue): fewer
  moving parts for self-hosting, but diverges from `tocsin` and scales
  fan-out less well.
- **Pluggable bus**: two code paths to test.
