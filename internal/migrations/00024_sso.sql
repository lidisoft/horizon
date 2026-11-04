-- +goose Up
-- Magic links, OpenID Connect providers and enforced SSO
-- (docs/security.md#authentication-methods).

-- A sign-in link: the token's hash, and the hash of the nonce in the
-- requesting browser's cookie, so the link only works there.
CREATE TABLE magic_links (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    nonce_hash bytea NOT NULL,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz
);

CREATE INDEX magic_links_expires ON magic_links (expires_at);

-- How a session was opened: password, magic_link or oidc. Enforcing SSO
-- closes the others.
ALTER TABLE sessions ADD COLUMN auth_method text NOT NULL DEFAULT 'password';

-- An OpenID Connect provider: instance-wide (workspace_id NULL, managed by
-- instance admins) or a workspace's. slug names it in URLs.
CREATE TABLE oidc_providers (
    id                uuid PRIMARY KEY,
    workspace_id      uuid REFERENCES workspaces (id) ON DELETE CASCADE,
    slug              text NOT NULL UNIQUE,
    name              text NOT NULL,
    issuer            text NOT NULL,
    client_id         text NOT NULL,
    client_secret     text NOT NULL,              -- sealed with SECRET_KEY
    email_domains     text[] NOT NULL DEFAULT '{}', -- allowed email domains; empty: any
    provision_role_id uuid REFERENCES roles (id) ON DELETE SET NULL, -- just-in-time: join the workspace with this role
    enabled           boolean NOT NULL DEFAULT true,
    created_at        timestamptz NOT NULL,
    updated_at        timestamptz NOT NULL,
    version           integer NOT NULL DEFAULT 1,
    CONSTRAINT oidc_providers_slug CHECK (slug ~ '^[a-z0-9][a-z0-9-]{0,39}$'),
    CONSTRAINT oidc_providers_name_length CHECK (char_length(name) BETWEEN 1 AND 100)
);

-- An account's identity at a provider: the provider's subject, so a changed
-- email at the provider still finds the account.
CREATE TABLE user_identities (
    provider_id   uuid NOT NULL REFERENCES oidc_providers (id) ON DELETE CASCADE,
    subject       text NOT NULL,
    user_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at    timestamptz NOT NULL,
    last_login_at timestamptz NOT NULL,
    PRIMARY KEY (provider_id, subject)
);

CREATE INDEX user_identities_user ON user_identities (user_id);

-- A workspace that enforces SSO lets its members sign in with its OIDC
-- providers only (instance admins keep their password, to recover).
ALTER TABLE workspaces ADD COLUMN sso_enforced boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE workspaces DROP COLUMN sso_enforced;
DROP TABLE user_identities;
DROP TABLE oidc_providers;
ALTER TABLE sessions DROP COLUMN auth_method;
DROP TABLE magic_links;
