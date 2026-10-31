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
  /projects                                 GET (visible ones) | POST (project.create)
  /projects/{key}                          GET (current or former key; "can" lists the caller's permissions) | PATCH | /archive | /unarchive
    /config/…                              project overrides (same shape), plus …/{id}/disable|enable and issue-types/{id}/copy
    /iterations  /iterations/{id}/start|complete|archive
    /releases  /milestones
  /issues                                  filter via view query (POST /issues/query)
  /issues/{key}
    /transition  /assign  /estimate  /prioritize  /rank  /move  /archive  /unarchive
    /links  /children  /comments  /attachments  /activity  /subscription
  /views  /milestones  /search
  /reports/{report}                        report data (see reports.md)
  /projects/{key}/overview                 project overview blocks
  /webhooks  /webhooks/{id}/deliveries
  /integrations/vcs
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

`GET …/move/preview?project=OPS` returns the automatic matches and what is
left to map. `children` (`move` or `leave`) is required when the issue has
children. The response contains the new key; requests for the old key resolve
to it.

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

Fields: built-ins, `status.category`, `status.outcome` (`unstarted`,
`started`, `completed`, `abandoned`, or the shorthand `open` = unstarted or
started; use it for cross-project queries), `cf:<field id>` for custom
fields.
Operators: `eq`, `neq`, `in`, `nin`, `lt`, `lte`, `gt`, `gte`, `contains`,
`is_empty`, `is_not_empty`, `within` (relative dates such as `-7d`).
Placeholders: `@me`, `@current_iteration`, `@today`.

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
