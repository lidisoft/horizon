# 0020. Search with PostgreSQL full-text search and pg_trgm

- **Status**: Accepted
- **Date**: 2026-10-28

## Context

Users search issues and comments, and the ⌘K palette needs fast,
typo-tolerant matching on titles and issue keys. Each instance should stay
small to operate ([0009](0009-instance-per-customer-multi-workspace.md)).

## Decision

Search runs on PostgreSQL only: `tsvector` columns (updated in the write
transaction) for word search on issues and comments, and `pg_trgm` trigram
indexes on titles and keys for fuzzy and prefix matching. No external search
engine.

## Consequences

- No extra service, no indexing lag, permissions applied in the same SQL.
- Relevance is weaker than a dedicated engine; language stemming is per
  workspace configuration (`simple` by default).
- The `pg_trgm` extension must be available (it ships with PostgreSQL contrib).

## Alternatives considered

- **Meilisearch / Typesense / OpenSearch**: better relevance, one more service
  per instance, fed asynchronously from the event stream.
- **Pluggable engine**: an abstraction with a single implementation; can be
  introduced if a real need appears.
