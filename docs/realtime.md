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
| `presence` | `{ "ws": "acme", "issue": "HZN-42", "state": "viewing" \| "editing" \| "gone" }` | Announce presence (`issue.view`); repeated every 25 s |
| `doc.join` | `{ "ws": "acme", "issue": "HZN-42", "version": n }` | Start editing a description (`issue.view`); `version` (optional) is the one the client holds |
| `doc.steps` | `{ "ws", "issue", "version": n, "client": "<id>", "steps": [...] }` | Submit a batch of steps made against `version` (`issue.edit`) |
| `doc.leave` | `{ "ws", "issue" }` | Stop editing (closing the socket does too) |
| `doc.cursor` | `{ "ws", "issue", "version": n, "anchor": p, "head": p }` | Share the selection (positions in the document at `version`), in a description the socket joined |
| `ping` | — | Keepalive (every 25 s) |

### Server → client

| `t` | `d` |
|-----|-----|
| `ack` | `{ "topics": [...accepted], "denied": [...] }`, answering `sub` and `unsub` by `id` |
| `error` | `{ "code": "bad_message" \| "unknown_type" }` |
| `event` | A domain event (see [api.md](api.md#domain-event-format)) |
| `presence` | `{ issue, users: [{ id, name, state }] }`: who views or edits an issue the socket announced itself on, sent as it changes (a person editing in any tab shows as editing) |
| `doc.joined` | `{ issue, version, steps: [batches since the client's version], doc? }`: `doc` (the whole document) when the client named no version or the batches since are gone |
| `doc.steps` | An accepted batch, to everyone editing: `{ issue, version (it applied to), steps, client, user }` |
| `doc.ack` | The sender's batch is in: `{ issue, version (after it) }` |
| `doc.reject` | The batch was made against an old version: `{ issue, version, steps: [the batches since] }`; the client rebases and sends again |
| `doc.error` | `{ issue, code }`: `invalid_steps`, `too_large`, `resync`, `forbidden`, `issue_archived`; the client starts over from the server's document |
| `doc.cursor` | Another socket's selection: `{ issue, conn, user, name, version, anchor, head }`, or `gone: true` when it leaves; the client maps it from `version` to its document |
| `resync` | The client missed too much; refetch the listed topics |
| `pong` | — |

Presence and the `doc.*` messages came with the block editor (phase 5).

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
  `horizon.presence.<workspace id>`, expiring after 60 s without a heartbeat;
  a socket closing announces itself gone. Selections travel on
  `horizon.cursors.<document id>`; editors draw others' carets and
  selections over the text, named, in one of five colours per person.

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
    { "type": "text", "text": " for details", "marks": [{ "type": "italic" }] }
  ]},
  { "type": "task_list", "content": [ … ] }
]}
```

v1 node types: paragraph, heading (1–3), bullet list, ordered list, task list,
quote, code block, divider, image (attachment), table (simple), callout.
Inline: text with marks (bold, italic, strike, code, link), mention,
issue reference, emoji.

Positions are integer offsets into a flattened token sequence, as in
ProseMirror: a text character counts its UTF-16 code units (as JavaScript
strings do; Go counts the same way), a leaf node 1, and other nodes their
content plus 2 for their boundaries. No position may fall inside a
surrogate pair. The schema is defined **once** as JSON
(`internal/document/schema.json`); Go embeds it and `task gen:schema`
writes it as `web/app/editor/schema.js`. Content expressions are
space-separated terms (a node or group name, with `?`, `*` or `+`);
attributes have defaults and allowed values or a kind (`count`, `string`,
`uuid`, `issue_key`, `url`: http, https or mailto only). Documents are at
most 1 MiB serialised and 12 levels deep.

### Steps

| Step | Effect |
|------|--------|
| `replace` | Replace range `[from, to)` with a slice |
| `replace_around` | Wrap or unwrap a range (lists, quotes) |
| `add_mark` / `remove_mark` | Toggle a mark on a range |
| `set_attrs` | Change a node's attributes (heading level, task checked) |

Every step can be **applied** and **mapped** through other steps (position
mapping). Both operations are implemented in Go (`internal/document`) and
in JS (`web/app/editor/model.js`, `transform.js`), following ProseMirror's
model and transform modules, and checked against a shared corpus of test
vectors (`internal/document/testdata/*.json`: apply and invert, mapping,
rebase, schema), so the two implementations can't drift. Steps come from
clients, so the server checks the whole document after applying them (a
slice may carry a node whose own content is invalid) and the step parser
is fuzzed (`FuzzApply`). Mark steps invert exactly only over uniformly
marked text, as in ProseMirror: the editor emits mark steps over precise
ranges.

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
   (default 50) and when the last editor leaves, and the issue's plain
   text (`issues.description`, search) with it. Reading a description
   replays the batches since the snapshot; batches more than 1,000 steps
   behind the snapshot are dropped (a client further behind joins from the
   whole document).

Batches are stored in `document_steps`, keyed by the version they applied
to, and travel between replicas on NATS core (`<prefix>.docs.<document
id>`). A description written whole through the API (`PATCH`, workflow
post-actions) becomes one `replace` step of the whole content, so editors
receive it too. The client (`web/app/editor/collab.js`) sends its
unconfirmed steps in batches 50 ms apart, one batch in flight; a batch from
someone else rebases them (`rebase`), and the selection and undo history
are mapped through what changed.

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
