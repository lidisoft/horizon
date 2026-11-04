# 0025. Single sign-on with OpenID Connect

- **Status**: Accepted
- **Date**: 2026-11-04

## Context

Organisations want their people to sign in with their identity provider
(Okta, Entra ID, Google Workspace, Keycloak…), to create accounts as people
arrive, and to make it the only way in for a workspace. Horizon's instances
host several workspaces, each possibly with its own provider. The ID token
checks are security-critical.

## Decision

- **OpenID Connect only** (no SAML), authorization code flow with PKCE and
  a nonce, through `github.com/coreos/go-oidc/v3` (discovery, keys, ID token
  verification) and `golang.org/x/oauth2` (the code exchange).
- **Providers instance-wide or per workspace.** Instance providers show on
  the sign-in page; a workspace's are found by email domain, so the sign-in
  page never lists workspaces.
- **Accounts** are found by the provider's subject (stored on first
  sign-in), else by a verified email. A workspace's provider may provision
  accounts and memberships with a role chosen by the workspace.
- **Enforced SSO** is per workspace and applies to its members across the
  instance: no password, no magic link; their other sessions close when it
  is turned on. Instance admins are exempt, to recover.
- The sign-in state (state, nonce, PKCE verifier, return path) lives in an
  HMAC-signed cookie, not in the database.
- Tests run a real Dex ([ADR-0022](0022-integration-tests-real-dependencies.md)).

## Consequences

- Two small, well-reviewed dependencies for the cryptographic part.
- A person in several workspaces is held to SSO by any one of them that
  enforces it; any configured provider satisfies it.
- No SCIM: deprovisioning happens in the provider (no more sign-ins) and in
  Horizon (suspend or deactivate).

## Alternatives considered

- **Hand-written JWT and JWKS handling**: no dependency, but security code
  of our own to maintain.
- **SAML**: still common in enterprises, but heavy (XML signatures); OIDC
  is offered by every provider Horizon targets.
- **Per-workspace enforcement limited to that workspace's own provider**:
  stricter, but awkward for people in several workspaces.
