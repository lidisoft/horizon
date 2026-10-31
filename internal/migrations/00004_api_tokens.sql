-- +goose Up
-- Personal access tokens (docs/security.md#authentication-methods). The
-- secret is shown once; only its SHA-256 hash is stored.

CREATE TABLE api_tokens (
    id           uuid PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name         text NOT NULL,
    token_hash   bytea NOT NULL UNIQUE,
    hint         text NOT NULL,          -- the token's first characters, to tell tokens apart
    scope        text NOT NULL,          -- read, write or admin; each includes the previous
    created_at   timestamptz NOT NULL,
    last_used_at timestamptz,
    expires_at   timestamptz,            -- null: never
    CONSTRAINT api_tokens_name_length CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT api_tokens_scope CHECK (scope IN ('read', 'write', 'admin'))
);

CREATE INDEX api_tokens_user ON api_tokens (user_id, created_at DESC);

-- +goose Down
DROP TABLE api_tokens;
