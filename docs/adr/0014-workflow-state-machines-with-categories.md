# 0014. Workflow state machines per issue type, statuses in categories

- **Status**: Proposed (the category mapping awaits confirmation)
- **Date**: 2026-10-27

## Context

The storm has "Issue Status Updated". Jira-style workflows (allowed
transitions, conditions, required fields) were chosen over Linear's free
transitions. Boards, reports, iteration completion and Git automation still
need to understand statuses across projects with different workflows.

## Decision

Each issue type is bound to a **workflow**: statuses plus allowed
**transitions** with conditions, validators and post-actions, configurable at
workspace level and overridable per project. Every status belongs to one fixed
**category**: `backlog`, `todo`, `in_progress`, `done`, `canceled`.

## Consequences

- Teams can enforce process; the server validates every transition.
- Cross-project features reason on categories, not on status names.
- Editing an in-use workflow needs a status migration mapping.
- More configuration UI to build than free transitions.
