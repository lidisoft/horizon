# 0004. State tables with a domain event log, not event sourcing

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

The initial analysis was an event storm, which naturally suggests event
sourcing. The product needs activity history, realtime updates, webhooks and
notifications — all fed by events — but also flexible querying of current
state (views with arbitrary filters) and true deletion
([0012](0012-hard-delete-with-event-scrubbing.md)), which pure event sourcing
makes hard.

## Decision

Aggregates are stored as ordinary rows (the source of truth for current
state). Every state change appends one or more **domain events** to an
`events` table **in the same transaction**. That table is also the
**transactional outbox**: a relay publishes events to NATS JetStream
([0005](0005-nats-jetstream-backbone.md)).

## Consequences

- Querying current state is plain SQL.
- History, realtime, webhooks and notifications share one event stream.
- Events aren't used to rebuild state, so a bug that forgets to emit an event
  isn't caught by replay. Every command handler test asserts the events
  emitted.
- The event log can be edited (scrubbed), which event sourcing would forbid.

## Alternatives considered

- **Full event sourcing**: history for free, but projections for every view,
  and erasure would mean crypto-shredding or rewriting streams.
- **Plain CRUD**: no reliable source for activity history or integrations.
