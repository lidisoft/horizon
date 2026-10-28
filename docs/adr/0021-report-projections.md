# 0021. Report projections fed by domain events

- **Status**: Accepted
- **Date**: 2026-10-28

## Context

v1 includes historical reports — burndown, velocity, cumulative flow, cycle
time — which need past states of issues, while current state lives in plain
tables ([0004](0004-state-tables-with-domain-event-log.md)). Every change is
already recorded in the `events` table and published to JetStream.

## Decision

A `reports` worker consumes domain events and maintains dedicated projection
tables (status intervals, iteration scope, daily totals). Reports query those
tables joined with current issue rows. Projections are rebuildable at any time
by replaying the PostgreSQL `events` table.

## Consequences

- Report queries are fast and independent of event payload shapes.
- Reports lag writes by the worker's latency (normally under a second).
- Event payloads become a contract for the projections: changing an event's
  shape needs a projection update and, if needed, a rebuild.
- Hard deletes stay consistent: the tombstone removes projection rows, and a
  rebuild from the scrubbed log produces the same result.

## Alternatives considered

- **Reconstructing history from `events` per request**: no extra tables, but
  slow on large projects and coupled to payload shapes in every query.
- **Nightly snapshots**: simple, but day-granular and empty before the first
  snapshot.
