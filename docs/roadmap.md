# Roadmap

Ordered by what unblocks the most. All of phases 1–8 make up **v1** (see
[vision.md](vision.md#v1-scope)); the phases sequence it so that something
usable exists early.

## Phase 0 — Design

- [x] Event storm (Issue and Iteration aggregates)
- [x] Documentation set and ADRs
- [x] Review open questions (below) with the team
- [ ] Wireframes for list, issue detail, board, backlog

## Phase 1 — Skeleton

- [ ] `cmd/horizon` with `serve`, `worker`, `migrate`, `admin`
- [ ] Docker Compose: PostgreSQL, NATS (JetStream), RustFS, Mailpit, Air
- [ ] Config, logging, health and readiness, metrics
- [ ] sqlc + goose wiring; `Taskfile.yml` covering the same commands as `tocsin`'s Makefile
- [ ] Event log: append in transaction, outbox relay, subject scheme
- [ ] LDS embedded at `/lds/`; `web/` embedded with hashed import map
- [ ] SPA core: `api`, `store`, `router`, `i18n`, `shortcuts`; `hzn-app` frame
- [ ] In-browser test runner and chromedp harness
- [ ] testcontainers harness with template databases
- [ ] GitHub Actions: `check` and `release` workflows

## Phase 2 — Identity and organisation

- [ ] Users, email + password, sessions, CSRF
- [ ] API tokens
- [ ] Workspaces, members, invitations, teams
- [ ] Roles, permission sets, grants; enforcement middleware
- [ ] Projects with keys and aliases

## Phase 3 — Issues core

- [ ] Configuration resolver; issue types, workflows (state machine, rules),
      custom fields, labels, estimation scales — with defaults
- [ ] Issues: create, field edits, transition, assign, estimate, prioritise,
      rank, parent/child, links, move between projects, archive, hard delete
      with scrubbing
- [ ] Comments (plain text first), attachments via S3
- [ ] Activity feed from `events`
- [ ] WebSocket pushes for domain events (no collab yet)
- [ ] List view, issue detail, ⌘K palette, shortcuts
- [ ] Full-text search

## Phase 4 — Planning

- [ ] Iterations: plan, schedule, load, start, complete with policy, archive
- [ ] Backlog view with sprint planning
- [ ] Board view
- [ ] Releases, milestones

## Phase 5 — Block editor and collaboration

- [ ] Shared schema and step test corpus (Go + JS)
- [ ] Single-user block editor replacing plain-text descriptions and comments
- [ ] Server-authoritative step protocol, rebase, snapshots
- [ ] Presence, remote cursors
- [ ] `IssueDescribed` session events

## Phase 6 — Views

- [ ] Filter AST → SQL compiler, filter bar, saved views (personal / shared)
- [ ] Table view with inline editing
- [ ] Working calendars (weekdays, holidays) and time zone settings
- [ ] Timeline / Gantt with dependencies, auto-scheduling in working days, milestones
- [ ] Calendar view
- [ ] Workspace roadmap across projects

## Phase 7 — Reports

- [ ] `reports` worker and projections, `admin rebuild-reports`
- [ ] SVG chart components
- [ ] Sprint reports, flow reports, release and milestone progress, workload
- [ ] Project overview page

## Phase 8 — Notifications and integrations

- [ ] Subscriptions, inbox, email notifications and digests
- [ ] Outbound webhooks with signing, retries, delivery log
- [ ] Git integration: GitHub, GitLab, Forgejo
- [ ] Magic link login, OIDC / SSO, enforced SSO
- [ ] Operator contract complete: graceful shutdown, `admin reconcile`,
      `admin rekey`, upgrade checks ([operations.md](operations.md))

## After v1

- Time tracking and work logs
- Importers (Jira, Linear, CSV)
- Sub-projects
- Text query language over the filter AST
- Formula and relation fields
- LiDiSoft shell adapter
- Additional languages
- Offline editing
- Public intake forms per project; customer portal for external users
- Workspace export / import between instances; Helm chart
- Configurable dashboards; release forecasts; CSV and PNG/SVG report exports
- Phone-sized layouts
- Individual availability (absences) in scheduling
- Move Horizon's own tickets into Horizon (dogfooding)

## Open questions

None at the moment.

### Resolved

| Question | Decision |
|----------|----------|
| Status categories | Configurable, each with an outcome (`unstarted`, `started`, `completed`, `abandoned`) — [ADR-0014](adr/0014-workflow-state-machines-with-categories.md) |
| Commands over WebSocket | No: REST only — [ADR-0008](adr/0008-websockets-for-realtime.md) |
| Search | PostgreSQL FTS + `pg_trgm` — [ADR-0020](adr/0020-postgresql-search.md) |
| Children of a deleted issue | The user chooses re-parent or cascade at delete time — [domain.md](domain.md#deletion) |
| JetStream retention | Configurable per instance, default 7 days — [operations.md](operations.md#configuration) |
| Guests | Guest role only in v1; intake forms and a portal after v1 |
| SaaS operations | Out of this repository; Horizon defines the instance contract — [operations.md](operations.md) |
| License | GPL-3.0 kept |
| Reports | Sprint, flow, release/milestone and workload reports; fixed project overview; event-fed projections — [reports.md](reports.md), [ADR-0021](adr/0021-report-projections.md) |
| Moving an issue between projects | Mapping dialog with auto-match by name; old key becomes an alias; user chooses whether children move — [domain.md](domain.md#issue) |
| Time zones | Workspace time zone, project override; timestamps in the user's zone — [configuration.md](configuration.md#time-zone-and-working-calendar) |
| Gantt calendars | Working days: configurable weekdays and holidays, workspace default, project override — [configuration.md](configuration.md#time-zone-and-working-calendar) |
| Responsive / mobile web | Desktop only in v1 — [frontend.md](frontend.md#look-and-feel) |
| Backend testing | Real dependencies via testcontainers — [ADR-0022](adr/0022-integration-tests-real-dependencies.md) |
| CI | GitHub Actions: PR checks, release on tag — [development.md](development.md#ci) |
| Branch and commit conventions | `dev/<user>/<ticket>-<slug>`, conventional commits, squash merge; tickets move to Horizon once it can host them — [development.md](development.md#branches-commits-and-pull-requests) |
