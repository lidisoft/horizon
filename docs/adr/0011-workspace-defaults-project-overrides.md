# 0011. Workspace defaults, project overrides

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

Issue types, workflows, custom fields, estimation scales and policies must be
configurable "globally and/or per project".

## Decision

Every configurable item is defined at **workspace** level and may be
overridden at **project** level, resolved through one resolver. Scalar
settings are replaced; collections (types, fields, …) are inherited, can be
disabled, and extended. Inherited definitions aren't edited from a project.

## Consequences

- One mental model and one code path for all configuration.
- Every read of a configurable value must go through the resolver (server and
  client).
- Built-in defaults make a fresh workspace usable without setup.

See [configuration.md](../configuration.md).
