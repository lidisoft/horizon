-- +goose Up
-- Workspaces, members, invitations and teams (docs/data-model.md#identity-and-access).

CREATE TABLE workspaces (
    id         uuid PRIMARY KEY,
    slug       text NOT NULL UNIQUE,
    name       text NOT NULL,
    time_zone  text NOT NULL DEFAULT 'UTC',
    settings   jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    version    integer NOT NULL DEFAULT 1,
    CONSTRAINT workspaces_slug_shape CHECK (slug ~ '^[a-z][a-z0-9-]{1,39}$' AND slug !~ '--' AND slug !~ '-$'),
    CONSTRAINT workspaces_name_length CHECK (char_length(name) BETWEEN 1 AND 100)
);

-- One row per user who joined; pending people are in invitations.
CREATE TABLE workspace_members (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid NOT NULL,           -- aggregate id of the membership
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status       text NOT NULL DEFAULT 'active',
    joined_at    timestamptz NOT NULL,
    suspended_at timestamptz,
    version      integer NOT NULL DEFAULT 1,
    PRIMARY KEY (workspace_id, user_id),
    UNIQUE (id),
    CONSTRAINT workspace_members_status CHECK (status IN ('active', 'suspended')),
    CONSTRAINT workspace_members_suspended CHECK ((status = 'suspended') = (suspended_at IS NOT NULL))
);

CREATE INDEX workspace_members_user ON workspace_members (user_id);

CREATE TABLE teams (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    name         citext NOT NULL,
    description  text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    version      integer NOT NULL DEFAULT 1,
    UNIQUE (workspace_id, name),
    CONSTRAINT teams_name_length CHECK (char_length(name) BETWEEN 1 AND 60),
    CONSTRAINT teams_description_length CHECK (char_length(description) <= 300)
);

CREATE TABLE team_members (
    workspace_id uuid NOT NULL,
    team_id      uuid NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    user_id      uuid NOT NULL,
    added_at     timestamptz NOT NULL,
    PRIMARY KEY (team_id, user_id),
    -- Leaving the workspace leaves its teams.
    FOREIGN KEY (workspace_id, user_id) REFERENCES workspace_members (workspace_id, user_id) ON DELETE CASCADE
);

CREATE INDEX team_members_user ON team_members (workspace_id, user_id);

-- Invitations by email, valid 7 days; accepting one proves the address.
CREATE TABLE invitations (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    email        citext NOT NULL,
    token_hash   bytea NOT NULL UNIQUE,
    invited_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    team_ids     uuid[] NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL,
    sent_at      timestamptz NOT NULL,     -- last (re)send
    expires_at   timestamptz NOT NULL,
    accepted_at  timestamptz,
    accepted_by  uuid REFERENCES users (id) ON DELETE SET NULL,
    revoked_at   timestamptz,
    version      integer NOT NULL DEFAULT 1
);

-- At most one open invitation per email and workspace.
CREATE UNIQUE INDEX invitations_open ON invitations (workspace_id, email)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- +goose Down
DROP TABLE invitations;
DROP TABLE team_members;
DROP TABLE teams;
DROP TABLE workspace_members;
DROP TABLE workspaces;
