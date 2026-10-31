# Security: authentication and authorisation

## Accounts

Users are **instance-wide**; a user can belong to several workspaces. An
instance admin (bootstrapped with `horizon admin create-user --instance-admin`)
manages workspaces, OIDC providers and SMTP.

## Authentication methods

| Method | Notes |
|--------|-------|
| Email + password | argon2id (`golang.org/x/crypto/argon2`), minimum length 12, breached-password check optional; rate-limited login; email verification |
| Magic link | One-time token emailed, valid 15 min, single use, bound to the requesting browser via a cookie nonce |
| OIDC / SSO | Generic OpenID Connect (authorization code + PKCE). Providers configured instance-wide or per workspace; optional email-domain allowlist and just-in-time provisioning into a workspace with a default role. A workspace can **enforce** SSO, which disables password and magic-link logins for its members |
| API tokens | Personal access tokens with scopes (`read`, `write`, `admin`) and optional expiry. Stored hashed (SHA-256); shown once |

Sessions: opaque random token in an `HttpOnly; Secure; SameSite=Lax` cookie,
stored hashed in `sessions`, sliding expiry (30 days idle). State-changing
requests from the SPA carry a CSRF token header (double submit). Logging out
revokes the session server-side; users can list and revoke their sessions.

## Authorisation

### Model

- A **permission** is a stable key, e.g. `issue.create`, `issue.transition`,
  `issue.delete`, `iteration.manage`, `project.config.manage`.
- A **role** is a named set of permissions, defined per workspace.
  ([ADR-0013](adr/0013-configurable-roles-and-permission-sets.md)).
- A **grant** gives a role to a user or a team, at **workspace** scope (applies
  to all projects) or **project** scope.
- Effective permissions on a project = union of all grants on that project and
  at workspace scope, for the user and their teams.
- Project **visibility**: `workspace` (every member may view) or `private`
  (only users with a grant on it).

Code checks **permission keys only**, never role names.

### Default roles

Seeded in every new workspace and editable afterwards, with one guard against
lock-out: the **Admin** role can't be deleted or lose `workspace.roles.manage`,
and its last workspace-scope grant can't be removed.

| Role | Intent |
|------|--------|
| Admin | Everything, including workspace configuration, roles, members, billing-free settings |
| Facilitator | Contributor + manage iterations, releases, milestones; rank the backlog; configure project boards and views |
| Contributor | Create, edit, transition, comment, link, attach, archive issues |
| Viewer | Read and comment |
| Guest | Read and comment on explicitly shared projects only; can't see the member list |

### Permission catalogue (v1)

| Area | Keys |
|------|------|
| Workspace | `workspace.manage`, `workspace.members.manage`, `workspace.roles.manage`, `workspace.config.manage`, `workspace.webhooks.manage`, `workspace.integrations.manage` |
| Project | `project.create`, `project.view`, `project.manage`, `project.config.manage` |
| Issue | `issue.view`, `issue.create`, `issue.edit`, `issue.transition`, `issue.assign`, `issue.estimate`, `issue.rank`, `issue.link`, `issue.move`, `issue.archive`, `issue.delete`, `issue.comment`, `issue.comment.moderate`, `issue.attach` |
| Planning | `iteration.manage`, `release.manage`, `milestone.manage` |
| Views | `view.share` |

Workflow transitions can further restrict who may perform them
([configuration.md](configuration.md#workflows)).

### Enforcement points

- REST: middleware resolves the workspace and project, and the handler declares
  the required key; the check runs before loading the aggregate.
- WebSocket: subscriptions are checked when made, and every pushed event is
  checked against the recipient's current permissions (cached per connection,
  invalidated by grant/role events).
- Webhooks and the inbox respect the target user's or workspace's visibility.
- Search and view queries add permission predicates to the generated SQL
  (only projects the user can view).

## Other measures

- Secrets at rest (OIDC client secrets, VCS tokens, webhook secrets) are
  encrypted with a key from `HORIZON_SECRET_KEY` (AES-GCM).
- Inbound forge webhooks are verified with each provider's signature scheme.
- Uploads: content type sniffed server-side, served with
  `Content-Disposition: attachment` except for safe image types; SVG is
  always served as an attachment.
- Strict Content-Security-Policy on the app page: `script-src 'self'` plus
  the hash of the import map (the only inline script), `style-src 'self'`,
  `frame-ancestors 'none'` ([frontend.md](frontend.md#layout-of-web)) — achievable because
  there's no third-party JS and no inline scripts.
- Audit: the `events` table records who did what. Hard deletes leave a
  tombstone naming the actor.
