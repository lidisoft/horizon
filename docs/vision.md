# Vision and scope

## What Horizon is

A project management tool for software teams that should feel as fast and
uncluttered as **Linear** — dense lists, instant navigation, keyboard first,
live updates — while offering the configurability teams reach for **Jira**,
**OpenProject** and **Focalboard** to get: their own issue types and workflows,
custom fields, sprints, releases, tables and Gantt charts.

| Borrowed from | What |
|---------------|------|
| Linear | Look and feel, ⌘K command palette, keyboard shortcuts, inbox, realtime, filter chips, saved views |
| Jira | Projects own issues, configurable issue types, workflow state machines, sprints with backlog planning, releases |
| OpenProject | Gantt with scheduling dependencies, milestones, configurable roles and permissions |
| Focalboard | Typed custom fields shown as table columns with inline editing |

## Principles

1. **Fast by default.** Optimistic UI, a client-side cache kept fresh by
   WebSockets, no full page loads.
2. **Configurable, with good defaults.** Everything configurable is defined at
   workspace level and may be overridden per project. A fresh workspace works
   with no setup.
3. **One API.** The SPA uses the same public REST API as integrations.
4. **Everything is traceable.** Every change is a domain event; activity feeds,
   webhooks, notifications and realtime updates are all fed from them.
5. **Few moving parts in the frontend.** Plain JavaScript and LDS — no
   framework, no build, no third-party code.

## Deployment model

- One codebase. The deployment unit is an **instance**: the Go binary plus
  PostgreSQL, NATS and S3-compatible storage.
- An instance hosts **several workspaces**. Users are instance-wide and can
  belong to several workspaces.
- **Self-hosted**: the customer runs one instance.
- **SaaS**: LiDiSoft runs one instance per paying customer. There is no shared
  multi-tenant database; isolation is per instance.

## v1 scope

### Organisation
- Workspaces, projects (with a key, e.g. `HZN`), teams (groups of users)
- Configurable roles built from permission sets, at workspace and project level

### Issues
- Configurable issue types (workspace-wide, overridable per project)
- Free parent/child hierarchy
- A workflow (state machine) per issue type; statuses grouped into configurable
  categories, each with an outcome (unstarted, started, completed, abandoned)
- Typed custom fields
- Priority levels plus manual ranking
- Estimates on a configurable scale (points, T-shirt sizes or none)
- Links: relates to, blocks, duplicates, precedes/follows (scheduling)
- Assignee, labels, due date, start date
- Rich descriptions and comments in a block editor with realtime
  collaborative editing
- Attachments
- Activity history
- Archive, and hard delete

### Planning
- Iterations (Scrum sprints) per project: plan, schedule, load, start, complete
- Configurable completion behaviour (archive all, archive done and roll over the
  rest, or nothing)
- Releases (versions) per project
- Milestones, and a workspace-level roadmap across projects

### Views
- List (grouped, dense, Linear-style)
- Board (kanban)
- Backlog with sprint planning
- Table with custom-field columns and inline editing
- Timeline / Gantt with dependencies and auto-scheduling
- Calendar
- Filter builder, grouping, sorting; personal and shared saved views

### Reports
- Sprint: burndown, burnup, sprint report, velocity
- Flow: cumulative flow, cycle and lead time, throughput
- Release and milestone progress
- Workload: distribution, created vs resolved
- Any report scoped by a view filter, measured in issue count or estimate,
  available through the API and shareable by link
- A fixed project overview page

### Collaboration and integration
- Realtime updates and presence over WebSockets
- In-app inbox and email notifications, subscriptions, @mentions
- Public REST API, personal access tokens, outbound webhooks
- Git integration (GitHub, GitLab, Forgejo): link branches, PRs and commits by
  issue key; optional status transitions on merge
- Authentication: email + password, magic link, OIDC/SSO

### Platform
- English UI, with every string in a message catalog (i18n-ready)
- Full-text and fuzzy search across issues and comments (PostgreSQL FTS and
  `pg_trgm`)

## Out of scope for v1

Listed so nobody builds them by accident. Several are on the
[roadmap](roadmap.md).

- Time tracking and work logs
- Sub-projects or project hierarchies
- Importers (Jira, Linear, …)
- Formula and relation custom fields
- Embedding in the LiDiSoft ecosystem shell (the SPA is designed to allow it
  later; see [ADR-0018](adr/0018-standalone-spa-shell-ready.md))
- Text query language (JQL-like)
- Configurable dashboards, forecasts, CSV and image exports of reports
- Offline editing
- Native mobile apps, and phone-sized layouts: v1 is desktop only (minimum
  width about 1024 px)
- Additional UI languages
