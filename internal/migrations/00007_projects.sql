-- +goose Up
-- Projects with keys and key aliases (docs/domain.md#project).

CREATE TABLE projects (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    key          text NOT NULL,
    name         text NOT NULL,
    description  text NOT NULL DEFAULT '',
    colour       text NOT NULL DEFAULT 'blue',
    lead_id      uuid REFERENCES users (id) ON DELETE SET NULL,
    visibility   text NOT NULL DEFAULT 'workspace',
    settings     jsonb NOT NULL DEFAULT '{}',   -- configuration overrides (phase 3)
    issue_seq    bigint NOT NULL DEFAULT 0,     -- last issue number used
    archived_at  timestamptz,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    version      integer NOT NULL DEFAULT 1,
    UNIQUE (workspace_id, key),
    CONSTRAINT projects_key_shape CHECK (key ~ '^[A-Z][A-Z0-9]{1,9}$'),
    CONSTRAINT projects_name_length CHECK (char_length(name) BETWEEN 1 AND 80),
    CONSTRAINT projects_description_length CHECK (char_length(description) <= 2000),
    CONSTRAINT projects_visibility CHECK (visibility IN ('workspace', 'private')),
    CONSTRAINT projects_colour CHECK (colour IN ('blue', 'green', 'teal', 'purple', 'pink', 'coral', 'orange', 'yellow', 'grey'))
);

-- Previous keys, so OLD-12 still resolves after a rename.
CREATE TABLE project_key_aliases (
    workspace_id uuid NOT NULL,
    project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    old_key      text NOT NULL,
    created_at   timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, old_key)
);

CREATE INDEX project_key_aliases_project ON project_key_aliases (project_id);

-- Grants on a project go with it.
ALTER TABLE role_grants
    ADD CONSTRAINT role_grants_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE role_grants DROP CONSTRAINT role_grants_project;
DROP TABLE project_key_aliases;
DROP TABLE projects;
