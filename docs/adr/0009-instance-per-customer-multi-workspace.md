# 0009. One instance per customer, several workspaces per instance

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

Horizon must be self-hostable and offered as SaaS.

## Decision

The deployment unit is an **instance**. An instance hosts several
**workspaces**; users are instance-wide. Self-hosted customers run one
instance. In SaaS, LiDiSoft runs **one instance per paying customer**. There
is no cross-customer multi-tenancy inside a database.

## Consequences

- Strong isolation between customers; per-customer upgrades, backups and data
  location are possible.
- Every table is still scoped by `workspace_id`, and every query filters by
  it.
- SaaS needs automation to provision and operate many instances (not yet
  designed).
