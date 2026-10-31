# Horizon

Horizon is LiDiSoft's project management tool. It aims for the speed and
keyboard-first feel of Linear, with the planning depth of Jira, OpenProject and
Focalboard: configurable issue types and workflows, custom fields, sprints,
releases, boards, tables, Gantt timelines and calendars.

- **Backend**: Go (Gin, pgx, sqlc, goose), PostgreSQL, NATS JetStream,
  S3-compatible object storage.
- **Frontend**: a single-page app in plain JavaScript (ES modules, no build
  step, no third-party JS) built on the
  [LiDiSoft Design System](https://github.com/lidisoft/design-system) (LDS).
- **Deployment**: one instance hosts several workspaces. Self-hosted, or as
  SaaS with one instance per customer.

## Status

Phase 2, identity and organisation, is done; phase 3, the issues core, is next ([roadmap](docs/roadmap.md)). The
documentation below is the specification.

## Getting started

With Docker installed, and Go and [Task](https://taskfile.dev) installed directly or by
[mise](https://mise.jdx.dev) from `mise.toml`:

```bash
mise install                  # optional: the pinned Go and Task
task up                       # the whole stack; the app answers on http://localhost:8090
curl localhost:8090/readyz    # PostgreSQL, NATS, S3 and schema checks
task admin -- create-user --email you@example.com --instance-admin   # prints a link to set your password
task check                    # what CI runs
```

[docs/development.md](docs/development.md) has the ports and every task.

## Documentation

| Document | Read it when |
|----------|--------------|
| [docs/vision.md](docs/vision.md) | You want to know what Horizon is and what v1 contains |
| [docs/domain.md](docs/domain.md) | You need the domain model, event storm, aggregates and policies |
| [docs/configuration.md](docs/configuration.md) | You touch anything configurable (types, workflows, fields, policies) |
| [docs/architecture.md](docs/architecture.md) | You want to know how the system is put together |
| [docs/data-model.md](docs/data-model.md) | You are writing a migration or a query |
| [docs/api.md](docs/api.md) | You are adding or consuming an endpoint, event or webhook |
| [docs/reports.md](docs/reports.md) | You work on reports or the project overview |
| [docs/realtime.md](docs/realtime.md) | You work on WebSockets, presence or collaborative editing |
| [docs/frontend.md](docs/frontend.md) | You work on the SPA |
| [docs/security.md](docs/security.md) | You touch authentication, roles or permissions |
| [docs/operations.md](docs/operations.md) | You run an instance, or automate running many |
| [docs/development.md](docs/development.md) | You write code: tests, CI, branches and commits |
| [docs/roadmap.md](docs/roadmap.md) | You are picking up the next chunk of work |
| [docs/adr/](docs/adr/README.md) | You are wondering *why* something is the way it is |

## License

GPL-3.0 — see [COPYING](COPYING).
