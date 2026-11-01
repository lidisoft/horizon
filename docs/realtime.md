# Realtime and collaborative editing

## WebSocket connection

One socket per browser tab: `GET /api/v1/ws` (session cookie; API tokens may
use a `token` query parameter). The server is the Gin app; any `serve`
replica can accept a socket.

Commands that change domain state still go through REST. The socket carries
**pushes**, **presence** and **document steps** — the traffic that needs low
latency or ordering. See [ADR-0008](adr/0008-websockets-for-realtime.md).

### Envelope

```json
{ "t": "<message type>", "id": "<client correlation id, optional>", "d": { … } }
```

### Client → server

| `t` | `d` | Purpose |
|-----|-----|---------|
| `sub` | `{ "topics": ["ws:acme", "project:acme/HZN", "issue:acme/HZN-42"], "since": "<event id>" }` | Subscribe; `since` (optional) replays what was missed |
| `unsub` | `{ "topics": [...] }` | Unsubscribe |
| `presence` | `{ "issue": "HZN-42", "state": "viewing" \| "editing" }` | Announce presence |
| `doc.join` | `{ "doc": "<id>", "version": n }` | Start editing; server replies with missed steps |
| `doc.steps` | `{ "doc": "<id>", "version": n, "client": "<id>", "steps": [...] }` | Submit steps |
| `doc.cursor` | `{ "doc": "<id>", "anchor": p, "head": p }` | Share selection |
| `ping` | — | Keepalive (every 25 s) |

### Server → client

| `t` | `d` |
|-----|-----|
| `ack` | `{ "topics": [...accepted], "denied": [...] }`, answering `sub` and `unsub` by `id` |
| `error` | `{ "code": "bad_message" \| "unknown_type" }` |
| `event` | A domain event (see [api.md](api.md#domain-event-format)) |
| `presence` | Users viewing/editing a subscribed issue |
| `doc.steps` | Accepted steps with the new version and the author client id |
| `doc.reject` | Steps rejected as stale; includes the steps the client is missing |
| `doc.cursor` | Other users' selections |
| `resync` | The client missed too much; refetch the listed topics |
| `pong` | — |

Presence and the `doc.*` messages arrive with the block editor (phase 5).

### Topics

| Topic | Events | Checked when subscribing |
|-------|--------|--------------------------|
| `ws:<slug>` | Everything in the workspace the user may see | Membership |
| `project:<slug>/<KEY>` | The project's issues, the project itself, its configuration overrides | The project is visible (former keys resolve) |
| `issue:<slug>/<KEY-n>` | The issue's events | `issue.view` on its project (former keys resolve) |

A topic that doesn't exist or isn't visible is `denied`, as REST would answer
404. Each event is checked again against the recipient's current access
before it is sent: a project's events need its visibility (and `issue.view`
for issue events); workspace-wide events (members, roles, configuration) need
a workspace grant, so guests don't receive them. Events that change who sees
a project (a visibility change, a deletion) go to every member, so sidebars
update. Pushed issue events name statuses, types, labels, fields and people
like the activity feed, but leave other issues as ids, as recipients differ.

Sockets close after an hour (`1001 reconnect`); the client reconnects, which
authenticates again. A socket too slow to take its events is closed the same
way. The client pings every 25 s; 75 s of silence closes the socket.

### Fan-out

```
commit ─▶ outbox relay ─▶ JetStream ─▶ (NATS core subject horizon.>)
                                              │
                         each serve replica ◀─┘ ─▶ filter by subscription
                                                   ─▶ permission check ─▶ socket
```

- Every event carries `aggregate.version`. A client that sees a gap refetches
  that aggregate.
- On reconnect, the client sends the last event id it received as `since`;
  the server replays the workspace's later events from PostgreSQL `events`
  (filtered like live ones) if that id is under 24 h old and at most 500
  events follow, otherwise it sends `resync` with the topics to refetch.
  Replayed events can repeat live ones: clients drop events by id.
- Presence lives in memory per replica and is shared through NATS core subject
  `horizon.presence.<ws>`, expiring after 60 s without a heartbeat.

## Collaborative editing

Descriptions and comments are **block documents** edited in a hand-written
block editor ([frontend.md](frontend.md#block-editor)). Several users can
edit the same document at once using a **server-authoritative step protocol**,
in the style of ProseMirror's collab module. See
[ADR-0015](adr/0015-block-editor-server-authoritative-collab.md).

### Document model

A JSON tree:

```json
{ "type": "doc", "content": [
  { "type": "heading", "attrs": { "level": 2 }, "content": [{ "type": "text", "text": "Context" }] },
  { "type": "paragraph", "content": [
    { "type": "text", "text": "See " },
    { "type": "issue_ref", "attrs": { "key": "HZN-12" } },
    { "type": "text", "text": " for details", "marks": [{ "type": "em" }] }
  ]},
  { "type": "task_list", "content": [ … ] }
]}
```

v1 node types: paragraph, heading (1–3), bullet list, ordered list, task list,
quote, code block, divider, image (attachment), table (simple), callout.
Inline: text with marks (bold, italic, strike, code, link), mention,
issue reference, emoji.

Positions are integer offsets into a flattened token sequence, as in
ProseMirror. The schema and the step types are defined **once** as JSON
(`internal/document/schema.json`) and loaded by both Go and JS.

### Steps

| Step | Effect |
|------|--------|
| `replace` | Replace range `[from, to)` with a slice |
| `replace_around` | Wrap or unwrap a range (lists, quotes) |
| `add_mark` / `remove_mark` | Toggle a mark on a range |
| `set_attrs` | Change a node's attributes (heading level, task checked) |

Every step can be **applied** and **mapped** through other steps (position
mapping). Both operations are implemented in Go and in JS and checked against
a shared corpus of test vectors (`internal/document/testdata/*.json`), so the
two implementations can't drift.

### Protocol

1. Client joins at version `v`, holding unconfirmed local steps.
2. Client sends `doc.steps` with base version `v`.
3. Server, in a transaction: if the document's version is `v`, apply and
   validate the steps against the schema, insert into `document_steps`
   (unique on `(document_id, version)` — this serialises concurrent replicas),
   bump the version, and broadcast via NATS. Otherwise reply `doc.reject`
   with steps since `v`.
4. On reject, the client **rebases**: maps its unconfirmed steps through the
   missing steps, reapplies them, and resends.
5. The server writes a full snapshot to `documents.content` every N steps
   (default 50) and when the last editor leaves.

The server never transforms steps; only clients rebase. This keeps the server
simple and makes the shared test corpus the main correctness tool.

### Domain events for documents

Keystrokes don't produce domain events. An `IssueDescribed` event (with a
summary diff and the new version) is emitted when an editing session goes
quiet (30 s without steps) or the last editor leaves. Notifications and
webhooks react to that event, not to steps.

### Offline and limits

No offline editing in v1: unconfirmed steps are kept in memory, and the editor
turns read-only after 30 s without a connection. Maximum document size
1 MiB serialised.
