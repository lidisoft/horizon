# Configuration

Most of Horizon's behaviour is configurable. Everything configurable follows
**one rule**: it is defined at **workspace** level, and a **project** may
override it. See [ADR-0011](adr/0011-workspace-defaults-project-overrides.md).

## Resolution

```
effective(project, key) = project override  if present
                          workspace value    otherwise
                          built-in default   if neither
```

Two kinds of configurable things:

| Kind | Examples | Override semantics |
|------|----------|--------------------|
| **Settings** (scalar values) | iteration completion policy, estimation scale, max active iterations | Project value replaces the workspace value |
| **Definitions** (collections) | issue types, workflows, custom fields, labels | A project **inherits** workspace definitions, can **disable** inherited ones, and can **add** its own. Inherited definitions can't be edited from a project — the project copies one and disables the original instead |

All reads go through a single resolver (`internal/config` in Go, mirrored by
the client cache), never directly from one scope. Changes to either scope emit
events, so clients invalidate their resolved view.

## Catalogue

| Item | Kind | Built-in default |
|------|------|------------------|
| Issue types | Definitions | Epic, Story, Task, Bug, Sub-task |
| Workflows | Definitions | One simple workflow (Backlog → Todo → In progress → Done, Canceled from anywhere) |
| Type → workflow binding | Setting per type | All types use the default workflow |
| Custom fields | Definitions | None |
| Field → type binding | Setting per field | — |
| Labels | Definitions | None |
| Estimation scale | Setting | Fibonacci points (0, 1, 2, 3, 5, 8, 13, 21) |
| Iteration completion policy | Setting | `archive_done_rollover_rest` |
| Max active iterations | Setting | 1 |
| Default iteration length | Setting | 2 weeks |
| Git merge transition | Setting | Off |
| Roles | Definitions (workspace only) | Admin, Facilitator, Contributor, Viewer, Guest |

## Issue types

| Property | Notes |
|----------|-------|
| name, icon, colour | Icon from the LDS Lucide sprite |
| workflow | Bound workflow |
| fields | Custom fields shown on this type, and whether each is required |
| allowed parent types | Optional restriction (e.g. Sub-task must have a parent; Epic can't have one) |

Hierarchy is free by default: any issue can be the parent of any other in the
same workspace.

## Workflows

A workflow is a **state machine**, bound per issue type.

| Element | Properties |
|---------|------------|
| Status | name, colour, **category** (`backlog`, `todo`, `in_progress`, `done`, `canceled`), position |
| Initial status | Exactly one |
| Transition | from status (or *any*), to status, name, and optional rules |

Transition rules (evaluated server-side):

- **Conditions** — who may perform the transition (permission or role), and
  whether all children must be in `done`/`canceled`.
- **Validators** — required fields (built-in or custom) on transition.
- **Post-actions** — set or clear a field (e.g. clear the assignee, set the
  resolution).

The fixed **status categories** let boards, filters ("open issues"), reports,
iteration completion and the Git integration work across projects whose
workflows differ.

Editing a workflow that is in use requires a **migration mapping** for any
removed status; affected issues are moved in the same operation and emit
`IssueStatusUpdated`.

## Custom fields

| Type | Stored as | Notes |
|------|-----------|-------|
| text | string | Single line |
| number | number | Optional unit and precision |
| select | option id | Options have a name and colour |
| multi_select | option id[] | |
| date | ISO date | |
| user | user id | |
| url | string | |
| checkbox | boolean | |

Fields are defined at workspace or project scope and attached per issue type.
Values live in the issue's `fields` JSONB column, keyed by field id, and are
filterable, sortable and groupable in every view.

## Estimation scales

| Scale | Values |
|-------|--------|
| `fibonacci` | 0, 1, 2, 3, 5, 8, 13, 21 |
| `linear` | 0, 1, 2, 3, 4, 5, 6, 7, 8 |
| `tshirt` | XS, S, M, L, XL (mapped to numbers for velocity) |
| `custom` | Ordered list of labels with numeric weights |
| `none` | Estimation hidden |

Changing a project's scale doesn't rewrite existing estimates; values outside
the new scale are shown as-is and flagged.
