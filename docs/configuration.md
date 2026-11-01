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

All reads go through a single resolver (`configdef.Service.Resolve` in Go,
mirrored by the client cache), never directly from one scope. Changes to either scope emit
events, so clients invalidate their resolved view.

## Catalogue

| Item | Kind | Built-in default |
|------|------|------------------|
| Issue types | Definitions | Epic, Story, Task, Bug, Sub-task |
| Status categories | Definitions | Backlog, Todo (unstarted); In progress (started); Done (completed); Canceled (abandoned) |
| Workflows | Definitions | One simple workflow (Backlog → Todo → In progress → Done, Canceled from anywhere) |
| Type → workflow binding | Setting per type | All types use the default workflow |
| Custom fields | Definitions | None |
| Field → type binding | Setting per field | — |
| Labels | Definitions | None |
| Estimation scale | Setting | Fibonacci points (0, 1, 2, 3, 5, 8, 13, 21) |
| Iteration completion policy | Setting | `archive_done_rollover_rest` |
| Max active iterations | Setting | 1 |
| Default iteration length | Setting | 2 weeks (1–8) |
| Time zone | Setting | `UTC` until set at workspace creation |
| Working calendar | Setting | Monday–Friday, no holidays |
| Git merge transition | Setting | Off |
| Roles | Definitions (workspace only) | Admin, Facilitator, Contributor, Viewer, Guest |

## Issue types

| Property | Notes |
|----------|-------|
| name, icon, colour | Icon from the LDS Lucide sprite |
| workflow | Bound workflow |
| fields | Custom fields shown on this type, and whether each is required. Required is checked **when an issue is created**; to require a field at a later step, use a workflow validator on the transition |
| allowed parent types | Optional restriction (e.g. Sub-task must have a parent; Epic can't have one) |

Hierarchy is free by default: any issue can be the parent of any other in the
same workspace.

## Workflows

A workflow is a **state machine**, bound per issue type.

| Element | Properties |
|---------|------------|
| Status | name, colour, **category**, position |
| Initial status | Exactly one |
| Transition | from status (or *any*), to status, name, and optional rules |

Transition rules (evaluated server-side):

- **Conditions** — who may perform the transition (permission or role), and
  whether all children must be in a final category. Conditions on the
  person's relation to the issue ("only the assignee", "only the reporter")
  are not in v1.
- **Validators** — required fields (built-in or custom) on transition.
- **Post-actions** — set or clear a field (e.g. clear the assignee, set the
  resolution).

### Status categories

Categories group statuses so that features work across workflows that differ.
They are configurable like any other definition (workspace default, project
override).

| Property | Notes |
|----------|-------|
| name, colour, position | Position orders board columns and groupings |
| **outcome** | `unstarted` or `started` (together: *open*), or final: `completed` or `abandoned` |

- Several categories may be final. Finality is what iteration completion,
  "open issues" filters, the "all children closed" condition and archiving
  use.
- Velocity, burndown and release progress count only `completed`; `abandoned`
  work is closed but not counted as delivered.
- Cross-project features (workspace views, roadmap, reports) rely on the
  **outcome**, which is fixed, never on category names, because projects may
  define their own categories.
- Entering a `started` category for the first time marks when work began
  (cycle time, cumulative flow); see [reports.md](reports.md).
- A workflow's initial status must be in an `unstarted` or `started` category.

### Editing: drafts and publishing

Workflows are edited as a **draft**: statuses, transitions and rules can be
changed freely without affecting issues, and the draft can be discarded. One
draft exists per workflow at a time; it is shared by everyone allowed to edit
the workflow.

**Publishing** applies the draft in one operation:

- It lists the changes and, for every removed status that still holds issues,
  asks for a **migration mapping** to a status of the new version (a status
  with the same outcome is suggested). Publishing is refused until every
  such status is mapped.
- Affected issues are moved in the same transaction and each emits
  `IssueStatusUpdated`, so the move appears in their history and in reports.
- The workflow's `version` increases; `WorkflowPublished` is emitted.

Status categories are not edited in the workflow editor: they are managed on
their own settings page, and a status only picks one.

The draft is saved as it is edited, as a whole document guarded by a
`draft_version`: a save based on an older version is refused (409) and the
editor reloads the newer draft. Saving checks the draft's structure (unique
names, one initial status in an open category, transitions between its own
statuses, known roles and fields in rules); publishing checks it again.

Transition rules, as stored:

```json
{ "roles": ["<role id>"], "children_final": true,
  "validators": ["assignee", "estimate"],
  "post_actions": [{ "field": "assignee", "op": "clear" }, { "field": "assignee", "op": "set_actor" }] }
```

`roles` empty means anyone holding `issue.transition`. Validators and
post-actions name built-in fields (`assignee`, `estimate`, `priority`,
`due_date`, `start_date`, `iteration`, `release`, `description`, `labels`)
and, once they exist, custom fields. `set_actor` applies to `assignee` only.

#### Deleting a category in use

Deleting a status category that statuses use asks for a **replacement with
the same kind of outcome** (open or final). Its statuses move there in every
workflow of the workspace, drafts included, in one step. A workspace
category can only be replaced by a workspace one, since its statuses may be
in any project's workflows. A category holding a workflow's initial status
can't become final.

#### Workspace and project definitions

A project sees every workspace category, workflow and issue type it hasn't
switched off, plus its own. Switching off is refused while something
effective in the project depends on it (a category used by a workflow, a
workflow bound to an issue type). Copying an inherited **issue type** into a
project makes an editable copy and switches the original off there; copying
a **workflow** doesn't switch the original off, because issue types are moved
to the copy one by one. Changing an issue type's workflow asks where the
issues of each status go, as publishing does.

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

Fields are defined at workspace or project scope and attached per issue type,
in order, each optionally required. Values live in the issue's `fields` JSONB
column, keyed by field id, and are filterable, sortable and groupable in every
view.

- A field's **type is fixed** once created. Select options carry stable ids,
  so renaming an option keeps every value; removing one leaves values that
  show as unknown until changed.
- A workspace issue type takes workspace fields only; a project that wants
  its own fields on an inherited type copies the type (its fields come along).
- Deleting a field detaches it from every type and drops it from workflow
  rules (validators and post-actions name custom fields by id).

## Time zone and working calendar

**Time zone** (workspace setting, project override). Dates such as due dates,
start dates and iteration bounds are calendar dates without a time; the
project's time zone decides when a day starts and ends — when an iteration
starts or a due date passes — and how reports bucket days. Timestamps
(comments, activity) are shown in each user's own time zone.

**Working calendar** (workspace setting, project override):

| Property | Notes |
|----------|-------|
| working weekdays | Default Monday–Friday |
| non-working dates | Holidays, with a name; a project override replaces the workspace list or adds to it |

The timeline schedules in **working days**: durations and `precedes` lags count
working days only, auto-scheduling skips non-working days, and they are shaded
on the timeline and calendar. Individual availability (absences) is not
modelled in v1.

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

The estimate picker is a **segmented control** for scales of six values or
fewer, and a **combobox** for longer ones.

`fibonacci` and `linear` have fixed values; `tshirt` (XS 1, S 2, M 3, L 5,
XL 8 by default) and `custom` carry up to 20 labels with weights from 0 to
1000, which velocity counts. A project follows the workspace's scale unless
it overrides it; removing the override returns it to the workspace's.

## Labels

Labels are definitions like the others (workspace, project override).

- **Switching off** an inherited label in a project hides it from pickers and
  filters there; issues that carry it keep it, shown greyed.
- **Merge into…** relabels every issue carrying the label with another one,
  each change recorded in the issue's history, then deletes it.
- **Move to workspace** promotes a project label to the workspace.
