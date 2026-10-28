# 0022. Integration tests against real dependencies

- **Status**: Accepted
- **Date**: 2026-10-28

## Context

Most of Horizon's correctness lives at the boundaries: SQL written for sqlc,
JSONB filters, `SKIP LOCKED` outbox relaying, JetStream consumers, presigned S3
URLs, permission predicates. Mocks of these would test our assumptions about
them, not their behaviour.

## Decision

Integration tests run against **real PostgreSQL, NATS (JetStream) and
RustFS**, started by **testcontainers-go**. Containers are shared per test
package; every test gets its own database cloned from a migrated template.
Pure domain logic keeps plain unit tests.

## Consequences

- Tests catch SQL, migration and messaging bugs that mocks would hide.
- Docker is required to run the full suite locally and in CI;
  `task test:unit` runs without it.
- The first run in a package pays container start-up (a few seconds).

## Alternatives considered

- **Compose stack only**: no extra Go dependency, but tests depend on a stack
  being up and share state.
- **Fakes behind interfaces**: fast, but diverge from real behaviour.
