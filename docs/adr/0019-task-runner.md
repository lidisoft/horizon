# 0019. Task instead of Make

- **Status**: Accepted
- **Date**: 2026-10-28

## Context

`tocsin` and LDS use a `Makefile` as their command entry point. Make's syntax
(tabs, `$$` escaping, shell-per-line, `.PHONY`) is a poor fit for what is
really a list of developer commands, and it behaves differently across
platforms.

## Decision

Horizon uses [Task](https://taskfile.dev) with a `Taskfile.yml` at the repository
root. Related tasks are namespaced (`migrate:up`, `gen:sqlc`). Every task has a
`desc`, so `task --list` documents them. Tasks that generate files declare
`sources`/`generates` so they are skipped when up to date.

## Consequences

- Contributors need the `task` binary (single static Go binary; also
  installable with `go install github.com/go-task/task/v3/cmd/task@latest`).
  CI installs it too.
- Commands differ from `tocsin` and LDS (`task up` rather than `make up`);
  names are kept the same where possible to limit the gap.
- YAML, variables and cross-platform behaviour come for free; no Makefile is
  kept alongside.

## Alternatives considered

- **Make**: consistent with the other LiDiSoft repositories, but harder to
  read and maintain.
- **Shell scripts in `scripts/`**: no dependency, but no discovery, no
  dependency graph and no up-to-date checks.
