# 0015. Block editor with server-authoritative collaborative editing

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

Descriptions and comments should use a Notion/Linear-like block editor, with
realtime collaborative editing. No third-party JavaScript is allowed
([0006](0006-spa-native-js-no-dependencies.md)), so Yjs, ProseMirror and
similar are unavailable.

## Decision

Store documents as a JSON block tree. Implement a hand-written
`contenteditable` editor whose edits are **steps** (replace, replace-around,
add/remove mark, set attrs). Collaboration uses a **server-authoritative**
protocol: clients send steps against a version; the Go server accepts them
only if the version is current (a unique constraint serialises replicas) and
broadcasts them; stale clients rebase locally. Step application and position
mapping are implemented in both Go and JS, verified against a shared test
corpus.

## Consequences

- Correct concurrent editing without a CRDT; the server never transforms.
- No offline editing.
- The editor is the largest and riskiest frontend component; it gets its own
  milestone.
- The schema is defined once (JSON) and shared by Go and JS.

## Alternatives considered

- **Hand-written CRDT** (RGA/Fugue) in both languages: supports offline and
  peer merges, but much more code and metadata overhead.
- **Markdown text** with presence and conflict warnings: far cheaper, rejected
  for UX.
