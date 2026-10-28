# Agent guide

Orientation for AI assistants working in this repository. Read
[docs/domain.md](docs/domain.md) before touching business logic, and the
[ADR log](docs/adr/README.md) before changing anything architectural.

## What this is

Horizon, LiDiSoft's project management tool: a Go backend serving a JSON API,
WebSockets and a plain-JavaScript SPA styled with the LiDiSoft Design System
(`github.com/lidisoft/design-system`, checked out at `../design-system`).

## Commands

The task runner is [Task](https://taskfile.dev), not Make
([ADR-0019](docs/adr/0019-task-runner.md)). `task --list` shows everything.

```bash
task up                       # dev environment under Docker + Air
task test                     # go test ./...
task check                    # fmt + vet + test + generated-code freshness — run before finishing
task migrate:up               # apply migrations
task migrate:create -- name   # new migration
task gen                      # sqlc + swagger
```

## Rules that are not negotiable

1. **No npm, no Node, no bundler, no third-party JavaScript.** The SPA is
   hand-written ES modules served as-is. See [ADR-0006](docs/adr/0006-spa-native-js-no-dependencies.md).
2. **Every state change emits a domain event in the same transaction.** Write
   the state row and the event row together; never publish to NATS directly
   from a handler. See [ADR-0004](docs/adr/0004-state-tables-with-domain-event-log.md).
3. **Configuration resolves workspace → project.** Never read a configurable
   value from a single scope; go through the resolver. See
   [docs/configuration.md](docs/configuration.md).
4. **Permissions are checked by permission key, never by role name.** Roles are
   user-defined. See [docs/security.md](docs/security.md).
5. **Hard delete means hard delete.** A delete removes the row, its events and
   its derived data, leaving only a tombstone. See
   [ADR-0012](docs/adr/0012-hard-delete-with-event-scrubbing.md).
6. **UI uses LDS components and semantic tokens only.** No hex values, no core
   tokens in Horizon CSS. Generic components belong upstream in LDS;
   Horizon-specific ones are `hzn-*` (see [docs/frontend.md](docs/frontend.md)).
7. **Every user-visible string goes through the message catalog**, even though
   only English ships.
8. **Record decisions.** A change that reverses or adds an architectural
   decision comes with an ADR.

## Where to pick up work

[docs/roadmap.md](docs/roadmap.md).
