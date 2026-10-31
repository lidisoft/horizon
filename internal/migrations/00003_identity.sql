-- +goose Up
-- Users, password sessions and password resets (docs/security.md,
-- docs/data-model.md#identity-and-access). Users are instance-wide.

CREATE TABLE users (
    id                uuid PRIMARY KEY,
    email             citext NOT NULL UNIQUE,
    name              text NOT NULL,
    password_hash     text,                  -- argon2id PHC string; null: no password set
    locale            text NOT NULL DEFAULT 'en',
    timezone          text,                  -- IANA name; null: the browser's
    is_instance_admin boolean NOT NULL DEFAULT false,
    deactivated_at    timestamptz,           -- instance-wide: no sign-in, no sessions
    created_at        timestamptz NOT NULL,
    updated_at        timestamptz NOT NULL,
    version           integer NOT NULL DEFAULT 1,
    CONSTRAINT users_email_shape CHECK (email ~ '^[^@\s]+@[^@\s]+$'),
    CONSTRAINT users_name_length CHECK (char_length(name) BETWEEN 1 AND 200)
);

-- Tokens are stored as SHA-256 hashes; the raw value only ever lives in the
-- browser's cookie.
CREATE TABLE sessions (
    id           uuid PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   bytea NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    expires_at   timestamptz NOT NULL,   -- sliding: last_seen_at + idle timeout
    user_agent   text NOT NULL DEFAULT '',
    ip           inet
);

CREATE INDEX sessions_user ON sessions (user_id, last_seen_at DESC);
CREATE INDEX sessions_expires ON sessions (expires_at);

CREATE TABLE password_resets (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz
);

CREATE INDEX password_resets_user ON password_resets (user_id);

-- Failed sign-ins per key ("email:…", "ip:…") within a window; see
-- identity.Throttle.
CREATE TABLE auth_throttle (
    key          text PRIMARY KEY,
    failures     integer NOT NULL,
    window_start timestamptz NOT NULL
);

-- +goose Down
DROP TABLE auth_throttle;
DROP TABLE password_resets;
DROP TABLE sessions;
DROP TABLE users;
