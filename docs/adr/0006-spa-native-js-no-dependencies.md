# 0006. SPA in native JavaScript with no third-party code

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

LiDiSoft's toolchain is Go; LDS forbids npm, Node and bundlers. A Linear-like
experience (instant navigation, optimistic updates, keyboard layer, realtime
cache) needs a client-side application, not server-rendered pages.

## Decision

The frontend is a **single-page app** written in plain JavaScript ES modules,
served as-is by the Go binary, over the public JSON API. **No npm, no build
step and no third-party JavaScript at all** — not even vendored modules.

## Consequences

- No framework: the store, router, keyed rendering, keyboard layer, block
  editor and collaborative editing algorithm are written in-house.
- No TypeScript or linters from the npm ecosystem; tests run in a small
  in-browser runner driven by Go (`chromedp`).
- A strict CSP (`script-src 'self'`) is straightforward.
- Supply-chain risk on the client is nil; maintenance cost is ours.

## Alternatives considered

- **Server-rendered pages with JS islands**: simpler, but not the instant,
  keyboard-first feel the product targets.
- **Vendored ES modules** (e.g. Yjs for collaboration): faster to build the
  editor, rejected to keep the client dependency-free.
