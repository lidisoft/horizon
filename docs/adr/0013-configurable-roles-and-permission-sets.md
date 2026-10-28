# 0013. Configurable roles built from permission sets

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

The storm names actors (Contributors, Facilitators). Customers coming from Jira
and OpenProject expect to define their own roles.

## Decision

Permissions are fixed keys. **Roles** are workspace-defined sets of
permissions, granted to users or teams at workspace or project scope. Default
roles (Admin, Facilitator, Contributor, Viewer, Guest) are seeded and
editable. Code checks permission keys only.

## Consequences

- Flexible for customers; a permission-management UI is needed in v1.
- Permission checks must be cheap: effective permissions are cached per
  request/connection and invalidated by grant and role events.

See [security.md](../security.md).
