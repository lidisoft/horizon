# 0014. Workflow state machines per issue type, statuses in configurable categories

- **Status**: Accepted
- **Date**: 2026-10-28

## Context

The storm has "Issue Status Updated". Jira-style workflows (allowed
transitions, conditions, required fields) were chosen over Linear's free
transitions. Boards, reports, iteration completion and Git automation still
need to know which statuses mean "closed" across projects with different
workflows, and teams want to choose how statuses are grouped.

## Decision

Each issue type is bound to a **workflow**: statuses plus allowed
**transitions** with conditions, validators and post-actions, configurable at
workspace level and overridable per project.

Every status belongs to a **category**. Categories are configurable too
(workspace default, project override) and each has a fixed **outcome**:
`unstarted` or `started` (together *open*), or final — `completed` or
`abandoned`. Several categories may share an outcome. Built-in defaults:
Backlog, Todo (unstarted), In progress (started), Done (completed), Canceled
(abandoned).

## Consequences

- Teams can enforce process; the server validates every transition.
- Grouping is per-team, but cross-project features (workspace views,
  roadmap, reports, iteration completion, Git automation) reason on the
  **outcome**, which never varies.
- The `started` outcome gives reports a start point for cycle time and
  cumulative flow without per-report configuration.
- Velocity and burndown count `completed` only; `abandoned` closes work without
  counting it as delivered.
- Editing an in-use workflow or deleting a category needs a migration mapping.
- More configuration UI to build than free transitions or fixed categories.

## Alternatives considered

- **Fixed categories** (Linear's five, Jira's three): simplest cross-project
  semantics, but teams can't shape their own grouping.
- **No categories**, marking final statuses per workflow: every cross-project
  feature would need per-workflow configuration.
