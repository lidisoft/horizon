-- name: Providers :many
-- A workspace's providers, or the instance's (workspace_id NULL).
SELECT * FROM oidc_providers WHERE workspace_id IS NOT DISTINCT FROM sqlc.narg(workspace_id)::uuid ORDER BY created_at, id;

-- name: Provider :one
SELECT * FROM oidc_providers WHERE id = sqlc.arg(id);

-- name: ProviderBySlug :one
SELECT * FROM oidc_providers WHERE slug = sqlc.arg(slug);

-- name: LockProvider :one
SELECT * FROM oidc_providers WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: CreateProvider :one
INSERT INTO oidc_providers (id, workspace_id, slug, name, issuer, client_id, client_secret, email_domains, provision_role_id, enabled, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.narg(workspace_id), sqlc.arg(slug), sqlc.arg(name), sqlc.arg(issuer), sqlc.arg(client_id), sqlc.arg(client_secret),
        sqlc.arg(email_domains), sqlc.narg(provision_role_id), sqlc.arg(enabled), sqlc.arg(created_at), sqlc.arg(created_at))
RETURNING *;

-- name: UpdateProvider :one
UPDATE oidc_providers SET name = sqlc.arg(name), issuer = sqlc.arg(issuer), client_id = sqlc.arg(client_id), client_secret = sqlc.arg(client_secret),
    email_domains = sqlc.arg(email_domains), provision_role_id = sqlc.narg(provision_role_id), enabled = sqlc.arg(enabled),
    updated_at = sqlc.arg(updated_at), version = version + 1
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteProvider :exec
DELETE FROM oidc_providers WHERE id = sqlc.arg(id);

-- name: SignInProviders :many
-- The enabled instance-wide providers, for the sign-in page.
SELECT slug, name FROM oidc_providers WHERE workspace_id IS NULL AND enabled ORDER BY name;

-- name: ProviderForDomain :many
-- Enabled providers that allow an email domain.
SELECT slug, name FROM oidc_providers WHERE enabled AND sqlc.arg(domain)::text = ANY(email_domains) ORDER BY workspace_id NULLS LAST, name;

-- name: IdentityUser :one
SELECT user_id FROM user_identities WHERE provider_id = sqlc.arg(provider_id) AND subject = sqlc.arg(subject);

-- name: LinkIdentity :exec
INSERT INTO user_identities (provider_id, subject, user_id, created_at, last_login_at)
VALUES (sqlc.arg(provider_id), sqlc.arg(subject), sqlc.arg(user_id), sqlc.arg(at), sqlc.arg(at))
ON CONFLICT (provider_id, subject) DO UPDATE SET last_login_at = EXCLUDED.last_login_at;

-- name: Membership :one
SELECT status FROM workspace_members WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id);

-- name: SSOEnforcedFor :one
-- Whether the user is an active member of a workspace enforcing SSO.
SELECT EXISTS (
    SELECT 1 FROM workspace_members m JOIN workspaces w ON w.id = m.workspace_id
    WHERE m.user_id = sqlc.arg(user_id) AND m.status = 'active' AND w.sso_enforced
);

-- name: SSOEnforced :one
SELECT sso_enforced FROM workspaces WHERE id = sqlc.arg(id);

-- name: SetSSOEnforced :exec
UPDATE workspaces SET sso_enforced = sqlc.arg(enforced), updated_at = sqlc.arg(at)::timestamptz WHERE id = sqlc.arg(id);

-- name: EnforcedMembers :many
-- Active members of a workspace who aren't instance admins.
SELECT u.id FROM workspace_members m JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = sqlc.arg(workspace_id) AND m.status = 'active' AND NOT u.is_instance_admin;

-- name: SealedProviderSecrets :many
SELECT id, client_secret FROM oidc_providers ORDER BY id;

-- name: SetProviderSecret :exec
UPDATE oidc_providers SET client_secret = sqlc.arg(client_secret) WHERE id = sqlc.arg(id);
