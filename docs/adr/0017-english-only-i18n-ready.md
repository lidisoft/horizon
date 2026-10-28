# 0017. English only, i18n-ready

- **Status**: Accepted
- **Date**: 2026-10-27

## Decision

Ship English only in v1, but route every user-visible string (SPA and emails)
through message catalogs, and format dates and numbers with `Intl` and the
user's locale and time zone. API errors carry stable codes that the client
translates.

## Consequences

Adding a language is a catalog file, not a code change. A small discipline
cost on every UI change.
