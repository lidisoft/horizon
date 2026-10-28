# 0012. Hard delete with event scrubbing and a tombstone

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

The event storm has both Delete and Archive. Deleted content must be truly
erased (privacy, mistakes, leaked secrets in descriptions), while clients and
integrations must still learn that the item disappeared.

## Decision

**Archive** is the reversible, read-only option. **Delete** is a hard delete:
the row, everything it owns, its attachments, and **all its stored events**
(PostgreSQL and JetStream) are removed. A single **tombstone** event remains,
with id, key, actor and time only.

## Consequences

- The event log isn't append-only; activity history of deleted items is gone.
- Events from other aggregates may reference the deleted id but must never
  embed its content — payloads reference by id.
- Delete is guarded by its own permission (`issue.delete`) and a confirmation
  in the UI.
- Purging JetStream and S3 is asynchronous (the `purge` worker).

## Alternatives considered

- **Soft delete with retention**: restorable, but not true erasure.
- **No tombstone**: clients and webhooks couldn't reliably learn about deletes.
