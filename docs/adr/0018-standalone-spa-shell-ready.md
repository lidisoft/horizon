# 0018. Standalone SPA, ready for the LiDiSoft shell

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

`tocsin`'s UI is a set of Web Components mounted into a "LiDiSoft ecosystem
shell", which exists today only as a development harness. Horizon's Linear-like
feel depends on app-wide concerns: ⌘K palette, global shortcuts, one
WebSocket, a shared client cache and issue-key deep links.

## Decision

Horizon ships as a **standalone SPA** with its own thin frame (`hzn-app`).
Every route view is an `hzn-*` custom element configured by attributes and
receiving its services (`api`, `store`, `socket`) through a context object
rather than globals, so a shell adapter can mount the views later.

## Consequences

- Not blocked on a shell contract that doesn't exist yet.
- A second app frame in the ecosystem for now, which may diverge from
  `tocsin`'s.
- Embedding later means writing an adapter, not rewriting views.

## Alternatives considered

- **Micro-frontend now**: consistent with `tocsin`, but Horizon would have to
  design the shell contract first.
- **Both modes from day one**: most up-front design and testing.
