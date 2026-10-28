# Roadmap

Ordered by what unblocks the most. All of phases 1–7 make up **v1** (see
[vision.md](vision.md#v1-scope)); the phases sequence it so that something
usable exists early.

## Phase 0 — Design

- [x] Event storm (Issue and Iteration aggregates)
- [x] Documentation set and ADRs
- [ ] Review open questions (below) with the team
- [ ] Wireframes for list, issue detail, board, backlog

## Phase 1 — Skeleton

- [ ] `cmd/horizon` with `serve`, `worker`, `migrate`, `admin`
- [ ] Docker Compose: PostgreSQL, NATS (JetStream), RustFS, Mailpit, Air
- [ ] Config, logging, health and readiness, metrics
- [ ] sqlc + goose wiring; `Taskfile.yml` covering the same commands as `tocsin`'s Makefile
- [ ] Event log: append in transaction, outbox relay, subject scheme
- [ ] LDS embedded at `/lds/`; `web/` embedded with hashed import map
- [ ] SPA core: `api`, `store`, `router`, `i18n`, `shortcuts`; `hzn-app` frame
- [ ] In-browser test runner and chromedp harness in CI

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
      rank, parent/child, links, archive, hard delete with scrubbing
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
- [ ] Timeline / Gantt with dependencies, auto-scheduling, milestones
- [ ] Calendar view
- [ ] Workspace roadmap across projects

## Phase 7 — Notifications and integrations

- [ ] Subscriptions, inbox, email notifications and digests
- [ ] Outbound webhooks with signing, retries, delivery log
- [ ] Git integration: GitHub, GitLab, Forgejo
- [ ] Magic link login, OIDC / SSO, enforced SSO

## After v1

- Time tracking and work logs
- Importers (Jira, Linear, CSV)
- Sub-projects
- Text query language over the filter AST
- Formula and relation fields
- LiDiSoft shell adapter
- Additional languages
- Offline editing

## Open questions

Assumptions made while writing these docs that still need a decision:

1. **Status categories** — workflows are full state machines; the docs also
   map every status to a fixed category (backlog, todo, in progress, done,
   canceled) so boards, reports and iteration completion work across
   projects. Confirm.
2. **Commands over WebSocket** — the docs keep domain commands on REST and use
   the socket for pushes, presence and editor steps. Confirm, or move commands
   to the socket.
3. **Search** — PostgreSQL full-text search is assumed sufficient; no external
   engine.
4. **Children of a deleted issue** — re-parented to the deleted issue's parent
   (assumed), rather than deleted too.
5. **JetStream retention** — 30 days assumed, with PostgreSQL as the long-term
   history.
6. **Guests** — external collaborators are modelled as a Guest role; is
   anything more (e.g. a portal) needed?
7. **SaaS operations** — provisioning, upgrades and backups of
   one-instance-per-customer are not covered yet.
8. **Licensing vs SaaS** — the repository is GPL-3.0; confirm that is
   intended (AGPL would also cover network use).
