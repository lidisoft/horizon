# API

One public REST API serves the SPA, the API tokens and integrations. Realtime
traffic uses the WebSocket described in [realtime.md](realtime.md).

## Conventions

- Base path: `/api/v1`. Breaking changes need a new version.
- JSON everywhere; field names in `snake_case`; times in RFC 3339 UTC.
- Authentication: session cookie (SPA, with the `X-CSRF-Token` header on
  state-changing requests) or `Authorization: Bearer <api token>`
  ([security.md](security.md#authentication-methods)). Anonymous requests to
  protected endpoints get `401 unauthenticated`; a missing or stale CSRF
  token gets `403 csrf_invalid`; a token whose scope is too narrow gets
  `403 insufficient_scope`, and credential endpoints called with a token
  `403 session_required`.
- API responses are `Cache-Control: no-store`.
- Workspace in the path: `/api/v1/w/{workspace}/…` (slug).
- Issues are addressable by key: `/api/v1/w/{ws}/issues/HZN-42`.
- **Commands are explicit** where the storm names an intent, rather than a
  generic PATCH, so each maps to its domain event:
  `POST …/issues/{key}/transition`, `…/assign`, `…/estimate`, `…/rank`,
  `…/links`, `…/archive`. `PATCH` covers plain field edits (→ `IssueUpdated`).
- **Optimistic concurrency**: responses carry `ETag: "<version>"`; mutations
  accept `If-Match`. A stale version returns `409 Conflict` with the current
  representation.
- **Idempotency**: mutations accept an `Idempotency-Key` header; the SPA
  always sends one so retries after network errors are safe.
- **Pagination**: cursor based — `?limit=50&cursor=…`, response
  `{ "items": [...], "next_cursor": "…" }`.
- **Errors**: RFC 9457 problem details
  `{ "type", "title", "status", "detail", "errors": [{ "field", "code" }] }`.
  `type` is a stable snake_case code (`invalid_credentials`,
  `version_conflict`) translated client-side as `error.<code>`; invalid input
  is `422 invalid_input` with one `{ field, code }` per problem
  (`error.<field>.<code>`). Unknown JSON fields are a `400 bad_request`.
  `429` responses carry `Retry-After`.
- OpenAPI is generated with swag and served at `/swagger/`.

## Resource map (v1)

```
/auth/login | /auth/logout | /auth/password/forgot | /auth/password/reset
/auth/magic-link | /auth/oidc/{provider}/…
/me                                        GET (user + CSRF token) | PATCH profile
/me/password | /me/sessions | /me/sessions/{id} | /me/sessions/revoke-others
/me/tokens | /me/tokens/{id}                GET, POST (secret shown once) | DELETE; session only
/me/inbox
/invitations/lookup | /invitations/accept  token in the body, never the URL
/w                                         workspaces of the current user (?all=true: every one, instance admins); POST creates (instance admins)
/w/{ws}                                    GET (with the caller's workspace permissions in "can") | PATCH name, slug, time zone
  /members  /members/{user}                PATCH status (active, suspended) | DELETE
  /invitations  /invitations/{id}/resend   POST {emails[], team_ids[]} | DELETE revokes
  /teams  /teams/{id}  /teams/{id}/members/{user}
  /roles  /roles/{id}                      GET (with the permission catalogue) | POST | PATCH | DELETE (unused roles; never Admin)
  /grants  /grants/{id}                    GET | PUT {role_id, user_id or team_id, project_id?} sets a subject's role in a scope | DELETE
  /config/categories  /config/categories/{id}   ?replacement= on DELETE when in use; /config/categories/order
  /config/issue-types  /config/issue-types/{id}  PATCH may carry status_mapping (workflow change); /config/issue-types/order
  /config/workflows  /config/workflows/{id}      POST copies {name, copy_from}
  /config/workflows/{id}/draft             PUT {version, draft} | DELETE (discard)
  /config/workflows/{id}/publish           POST {version, status_mapping}; 409 unmapped_statuses lists them
  /config/fields  /config/fields/{id}       type fixed after creation; /config/issue-types/{id}/fields GET | PUT {fields: [{field_id, required}]}
  /config/labels  /config/labels/{id}       /merge {into}; projects: /move-to-workspace
  /config/settings/estimation              GET | PUT {scale}; at project scope {scale: null} follows the workspace
  /config/settings/iterations              GET | PUT {completion_policy, max_active_iterations, iteration_length_weeks}: absent keys unchanged, null removes the value
  /config/settings/calendar                GET | PUT {time_zone (projects), working_days ([1–7], Monday 1), holidays ([{date, name}]), holidays_mode (projects: add | replace)}: same rules
  /projects                                 GET (visible ones) | POST (project.create)
  /projects/{key}                          GET (current or former key; "can" lists the caller's permissions) | PATCH | /archive | /unarchive
                                           DELETE ?confirm={key} (archived projects only)
    /people                                active members who see the project (assignee pickers)
    /config/resolved                       the configuration its issues use, in one read (switched-off labels flagged), with its working calendar
    /config/…                              project overrides (same shape), plus …/{id}/disable|enable and issue-types/{id}/copy
    /iterations                            GET (stats, next suggestion, settings, velocity; ?archived=true) | POST (iteration.manage)
    /iterations/{id}                       GET | PATCH (If-Match) | DELETE (not while active)
    /iterations/{id}/issues                POST {add: [keys], remove: [keys]} — load
    /iterations/{id}/start|complete|archive  POST (If-Match optional); complete takes {rollover_to}
    /releases                              GET (progress; ?archived=true) | POST (release.manage)
    /releases/{id}                         GET | PATCH (If-Match) | DELETE; /release  /archive  POST
    /milestones                            GET the project's and the workspace's
  /projects/{key}/issues                   GET in rank order (?cursor, ?limit, ?archived=true)
  /issues                                  POST {project, type_id, title, …}; filter via view query (POST /issues/query)
  /issues/{key}                            GET (former keys resolve; "parent" names a visible parent) | PATCH (If-Match: version) | DELETE ?children=
    /transitions                           GET those available from the current status ("blocked" when not allowed)
    /transition  /assign  /estimate  /prioritize  /rank  /archive  /unarchive   POST (If-Match optional)
    /release                               POST {release_id | null} (issue.edit)
    /links  /links/{id}                    GET | POST {type, target, lag_days?} | DELETE; seen from the issue (blocks / blocked_by …)
    /children  /move  /move/preview
    /comments  /comments/{id}               GET (oldest first) | POST {body} | PATCH (author) | DELETE (author, or issue.comment.moderate)
    /attachments                           GET | POST {filename, content_type, size} → presigned upload_url
    /attachments/{id}/complete  /download  POST after the PUT | GET redirects to a presigned URL; DELETE /attachments/{id}
    /activity                              GET the issue's events, newest first, in the domain event format
    /subscription
  /search                                  GET ?q=&limit=&archived=&project= — issues by key, words (prefixes) in titles,
                                           descriptions and comments, and near titles; snippets mark words with U+0002/U+0003
  /views
  /milestones  /milestones/{id}            GET (visible ones, with progress) | POST {project?, name, date, description} | PATCH | DELETE
    /issues                                GET linked keys | POST {add: [keys], remove: [keys]} (milestone.manage)
  /reports/{report}                        report data (see reports.md)
  /projects/{key}/overview                 project overview blocks
  /webhooks  /webhooks/{id}/deliveries
  /integrations/vcs
/ws                                        WebSocket (realtime.md)
/hooks/vcs/{provider}                      inbound forge webhooks
```

## Deleting issues

`DELETE /w/{ws}/issues/{key}?children=reparent|cascade`. The `children`
parameter is required when the issue has children (`422` with code
`children_choice_required` otherwise). `cascade` fails as a whole with `403`
if the caller lacks `issue.delete` on any descendant.

## Moving issues

`POST /w/{ws}/issues/{key}/move` with the target project and the mapping:

```json
{
  "project": "OPS",
  "children": "move",
  "mapping": {
    "types":    { "<source type id>": "<target type id>" },
    "statuses": { "<source status id>": "<target status id>" },
    "fields":   { "<source field id>": "<target field id> | null" }
  }
}
```

Without a mapping, a type, status, field or label matches the same
definition when the target project sees it, else one of the same name
(field: same name and type; select options by name). Types and statuses
that match nothing must be mapped (`422 mapping_required`, one
`mapping.types.<id>` or `mapping.statuses.<id>` per gap); field values and
labels that match nothing are dropped, and so are estimates outside the
target scale.

`GET …/move/preview?project=OPS&children=move` returns the matches (`match:
null` for gaps), the number of issues that move and the estimates that
would be cleared. `children` (`move` or `leave`) is required when the issue
has children. Moving needs `issue.move` on what moves and `issue.create` on
the target. The response contains the new key; requests for the old key
resolve to it. Iteration and release are cleared; links, comments and
history stay.

## Descriptions and comments

Descriptions and comments are block documents (`description_doc`,
`body_doc`: the JSON tree of [realtime.md](realtime.md#document-model)).
`description` and `body` are their plain-text projection: blocks on lines,
list items marked (`- `, `1. `, `[x] `), issue references as keys. Writes
take either: the document, checked against the schema (`422
description_doc: invalid`, `too_long` over 1 MiB, `unknown_attachment` for
an image that isn't a ready attachment of the issue), or plain text, one
paragraph per line. Lists of issues leave `description_doc` out.

## Planning iterations

An iteration is planned (`POST …/iterations`, a name, optional dates),
scheduled once it has both dates, then started, completed and archived
(`docs/domain.md#iteration`). Loading moves issues of the project in and
out: `POST …/iterations/{id}/issues {"add": ["HZN-41"], "remove": ["HZN-38"]}`.
An issue added from another open iteration leaves that one. Issues carry
`iteration_id`; an issue's iteration changes only through loading (and
with it, through deletes, moves, a deleted iteration, a workflow
post-action or the completion policy).

`POST …/complete` takes `{"rollover_to": "<iteration id>"}` (or nothing,
for the backlog) and answers at once with the policy in effect recorded
under `completion`; the policy worker applies it to the issues shortly
after, and their events arrive over the socket.

## Querying issues

`POST /w/{ws}/issues/query` takes the same JSON a saved view stores:

```json
{
  "filter": {
    "and": [
      { "field": "project", "op": "in", "value": ["HZN"] },
      { "field": "status.outcome", "op": "eq", "value": "open" },
      { "or": [
        { "field": "assignee", "op": "eq", "value": "@me" },
        { "field": "cf:01J…", "op": "gte", "value": 3 }
      ]}
    ]
  },
  "group_by": "status",
  "sort": [{ "field": "rank", "dir": "asc" }],
  "limit": 200
}
```

The request adds `projects` (keys to look in; none: every project the
caller sees issues of) and `cursor` (the `next_cursor` of the previous
page); the response has `items`, `total` and `next_cursor`. Archived
issues count only with `"archived": true`.

Fields: `project` (key or id), `type`, `status`, `status.category`,
`status.outcome` (`unstarted`, `started`, `completed`, `abandoned`, or the
shorthands `open` = unstarted or started and `closed`; use it for
cross-project queries), `priority`, `assignee`, `reporter`, `parent` (key
or id), `iteration`, `release`, `labels`, `estimate`, `title`, `text`
(words in the title and description, as prefixes), `start_date`,
`due_date`, `created`, `updated`, `resolved`, `started`, `archived`, and
`cf:<field id>` for custom fields (compared by their type).
Operators: `eq`, `neq`, `in`, `nin`, `lt`, `lte`, `gt`, `gte`, `contains`
(text; for labels and multi-selects: carries all), `is_empty`,
`is_not_empty`, `within` (between today and a relative date such as `-7d`,
`+2w`, `1m`). Placeholders: `@me`, `@current_iteration`, `@today`.
Sort keys: `rank`, `key`, `title`, `created`, `updated`, `resolved`,
`start_date`, `due_date`, `priority`, `status` (category order), `estimate`
and custom fields. Every value is a bind parameter (`internal/query`); a
query that doesn't compile answers 422 with the place (`filter.and[1].op:
bad_op`). At most 50 conditions, 6 levels deep, 500 issues a page.

## Rescheduling

`POST /w/{ws}/issues/{key}/schedule {start_date, due_date, confirm}` (with
`If-Match`) answers the plan for the issue's new dates: `issue` (from, to)
and `moves`, the issues it precedes, directly or not, that would start too
early: each with `from`, `to`, `shift` (working days later), `after` (the
issue pushing it) and `blocked` (`forbidden`, `archived`) when it can't
move. Successors keep their length in working days of their own project's
calendar and never move earlier; a lag counts working days too. Without
`confirm` nothing changes. With it the plan is applied: the issue, then
each successor at the `version` it was planned from, each `result` being
`moved` or the error code that stopped it. `GET /w/{ws}/projects/{key}/dependencies`
lists the `precedes` links touching the project's issues whose ends the
caller sees.

## Saved views

A view stores a query (as above, without `projects` and `cursor`), a
layout (`list`, `board`, `table`, `timeline`, `calendar`) and `display`
options the client owns (at most 16 KB of JSON; the board keeps `lanes` and
`wip`, its WIP limits by column name).

| Method | Path | Notes |
|--------|------|-------|
| `GET` | `/w/{ws}/views[?project=KEY]` | The workspace's shared views, those of visible projects, and the caller's own; with `project`, the workspace's and that project's |
| `POST` | `/w/{ws}/views` | `{ name, layout, project?, shared?, query?, display? }`. Personal by default; `shared` needs `view.share` on the project (or the workspace without one) |
| `GET` | `/w/{ws}/views/{id}` | 404 for someone else's personal view or a view of a project the caller doesn't see |
| `PATCH` | `/w/{ws}/views/{id}` | Fields left out stay; `If-Match`. Its owner edits a personal view, `view.share` a shared one; sharing or unsharing needs `view.share` |
| `DELETE` | `/w/{ws}/views/{id}` | Same rule; a `ViewDeleted` tombstone remains |

The query is compiled when it is saved: one that wouldn't run answers 422
naming the place (`query.filter.field: unknown_field`). Each view answers
`can_edit`. Events of personal views are pushed to their owner only.

## Domain event format

Used for the activity feed, WebSocket pushes and webhook payloads:

```json
{
  "id": "01J…",
  "type": "issue.status_updated",
  "workspace": "acme",
  "aggregate": { "type": "issue", "id": "01J…", "key": "HZN-42", "version": 7 },
  "actor": { "id": "01J…", "name": "…" },
  "occurred_at": "2026-10-27T09:12:00Z",
  "payload": { "from": { "id": "…", "name": "Todo" }, "to": { "id": "…", "name": "In progress" } }
}
```

Event type names are `<aggregate>.<event>` in snake case: `issue.created`,
`issue.described`, `iteration.loaded`, …

## Webhooks

- Configured per workspace or project, filtered by event type.
- `POST` with the event JSON; headers `Horizon-Event`, `Horizon-Delivery`,
  `Horizon-Signature: sha256=<hmac of body with the secret>`.
- Retries with exponential backoff for 24 h; deliveries visible in the UI with
  a redeliver action.
- Tombstones are delivered (`issue.deleted` with id and key only).
