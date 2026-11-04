# Security: authentication and authorisation

## Accounts

Users are **instance-wide**; a user can belong to several workspaces. An
instance admin (bootstrapped with `horizon admin create-user --instance-admin`)
manages workspaces, OIDC providers and SMTP.

There is **no public sign-up**. Accounts are created by an instance admin
(`horizon admin create-user`) or by accepting a workspace invitation, which
proves the email address. Without a password, an account gets a single-use
link to choose one.

- **Deactivating** an account (instance admin, `horizon admin
  deactivate-user`) blocks sign-in instance-wide and revokes every session and
  token. Its issues and history stay.
- **Suspending** a member is per workspace: it removes access to that
  workspace only, keeping grants and history, and closes live connections to
  it. The account and its other workspaces are untouched.
- **Invitations** expire after 7 days and can be resent (a new link; the old
  one stops working). Inviting an email with a pending invitation renews it.
  The link carries the token in its fragment, and the API takes it in request
  bodies, so it never reaches server logs. Accepting signed in needs the
  invited email; signed out, it creates the account (the email is proven) or,
  if one exists, asks to sign in first.
- Only **instance admins create workspaces**; the creator joins as first
  member. People who aren't active members of a workspace get `404
  workspace_not_found` for it, the same as for one that doesn't exist.

## Authentication methods

| Method | Notes |
|--------|-------|
| Email + password | argon2id (`golang.org/x/crypto/argon2`, OWASP baseline: 19 MiB, 2 passes; parameters stored with each hash), 12 to 256 characters. Sign-in failures are throttled per email (10) and per client IP (100) in 15-minute windows shared by every replica, and all answer `invalid_credentials`, taking the same time whether or not the account exists. Password reset links are single use, valid one hour, carry the token in the URL fragment (never logged), and sign out every session. A breached-password check may come later |
| Magic link | One-time token emailed, valid 15 min, single use, bound to the requesting browser via a cookie nonce |
| OIDC / SSO | Generic OpenID Connect (authorization code + PKCE). Providers configured instance-wide or per workspace; optional email-domain allowlist and just-in-time provisioning into a workspace with a default role. A workspace can **enforce** SSO, which disables password and magic-link logins for its members |
| API tokens | Personal access tokens, `Authorization: Bearer hzn_…` (the prefix lets secret scanners spot leaks). One scope each, cumulative: `read` (safe methods only), `write` (changes within the user's permissions), `admin` (also the `workspace.*` permissions and instance administration; sessions always have it). Optional expiry; at most 50 per user. Stored hashed (SHA-256) with a short hint to tell them apart; shown once. Managing credentials (password, sessions, tokens) needs a browser session: a token can't mint tokens. Deactivating the account deletes its tokens; changing the password keeps them |

Sessions: opaque random token (256 bits) in an `HttpOnly; SameSite=Lax`
cookie, stored hashed (SHA-256) in `sessions`, sliding expiry (30 days idle).
Over HTTPS (`BASE_URL` with `https://`) the cookie is `Secure` and named
`__Host-horizon_session`, which pins it to the exact origin; plain HTTP is for
development only.

CSRF: state-changing requests made with the session cookie carry
`X-CSRF-Token`, an HMAC of the session token under `SESSION_SECRET`. The SPA
gets it from `POST /auth/login` and `GET /me`; a cookie planted by a sibling
origin can't produce it. Requests a browser marks `Sec-Fetch-Site: cross-site`
are refused before any handler runs. API tokens (Bearer) need no CSRF token.
An `Authorization` header takes precedence over the cookie; an invalid one is
rejected (`401`, `WWW-Authenticate: Bearer`) rather than ignored.

Changing the password signs out the user's other sessions. Logging out revokes
the session server-side; users can list and revoke their sessions.

The client IP (session list, throttling) is the connection's address unless
the request comes through a proxy listed in `HORIZON_TRUSTED_PROXIES`.

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
- Project **visibility**: `workspace` (every member holding `project.view`
  may view) or `private` (only users with a grant on it, directly or through
  a team, plus holders of `project.manage` on the whole workspace, so admins
  can administer it). Projects the caller can't see are `404
  project_not_found`.

Code checks **permission keys only**, never role names, through
`internal/access`:

- A subject (member or team) holds **one role per scope**: one on the whole
  workspace and one per project. Setting another replaces it.
- Effective permissions are the union of the user's own grants and their
  teams', at workspace scope and on the project. Suspended members hold none;
  their grants come back with them. Leaving the workspace or deleting a team
  removes its grants.
- **Instance admins hold every permission** in every workspace: they are the
  recovery path when a workspace's admins are gone.
- The workspace-wide lists (members, teams, roles, grants) need a
  workspace-scope grant, so guests (granted on projects only) don't see them.
- API tokens need the `admin` scope for `workspace.*` permissions.
- Inviting gives the new member Contributor; choosing another role needs
  `workspace.roles.manage`, so inviting can't hand out more than one holds.
- A new workspace gets the default roles and its creator the Admin role.
  Workspaces created before roles existed were backfilled: the first member
  became Admin and the others Contributor (migration 00006).

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

- REST: middleware resolves the workspace and project, and the route declares
  the required key (`access.Service.Need(key)`); the check runs before loading
  the aggregate. Services check again, so no route depends on its middleware
  alone.
- WebSocket: subscriptions are checked when made, and every pushed event is
  checked against the recipient's current permissions (cached per connection
  for 30 s, dropped on any workspace event that isn't an issue's: roles,
  grants, members, projects). The upgrade checks `Origin` against the request
  host and `BASE_URL`; sockets close after an hour to authenticate again.
- Webhooks and the inbox respect the target user's or workspace's visibility.
- Search and view queries add permission predicates to the generated SQL
  (only projects the user can view).

## Other measures

- Secrets at rest (OIDC client secrets, VCS tokens, webhook secrets) are
  sealed with `SECRET_KEY` (AES-256-GCM, bound to their row), rotatable with
  `SECRET_KEY_PREVIOUS` and `admin rekey` ([ADR-0023](adr/0023-secrets-at-rest.md)).
- Webhook deliveries can't reach the instance's own networks: the address
  actually dialled is checked (loopback, private, link-local refused) unless
  `WEBHOOK_ALLOW_PRIVATE=true`; redirects aren't followed.
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
