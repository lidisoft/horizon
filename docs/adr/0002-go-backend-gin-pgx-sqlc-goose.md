# 0002. Go backend with Gin, pgx, sqlc and goose

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

All LiDiSoft services are Go. `tocsin` uses Gin, pgx, goose, zerolog,
urfave/cli and swag. Sharing a stack lowers the cost of moving between
projects.

## Decision

Go backend using **Gin** (HTTP), **pgx v5** (driver), **sqlc** (typed queries
generated from SQL), **goose** (migrations embedded in the binary),
**zerolog**, **urfave/cli v3** and **swag** for OpenAPI — the same libraries as
`tocsin`, plus sqlc.

## Consequences

- SQL is written by hand and type-checked by sqlc; dynamic queries (view
  filters) are the exception and are built by a dedicated, tested compiler.
- Gin ties handlers to `*gin.Context`; keep domain services independent of
  it so they're testable without HTTP.

## Alternatives considered

- **stdlib `net/http`** with Go 1.22 routing: fewer dependencies, but diverges
  from `tocsin`.
- **Hand-written repositories** without sqlc: more boilerplate, no
  compile-time checking of queries.
