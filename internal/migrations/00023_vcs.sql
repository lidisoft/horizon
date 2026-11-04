-- +goose Up
-- Git forge integration (docs/integrations.md): connections to GitHub,
-- GitLab and Forgejo, the branches, pull requests and commits linked to
-- issues, and the one comment Horizon keeps on each linked pull request.

CREATE TABLE vcs_connections (
    workspace_id   uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id             uuid PRIMARY KEY,
    provider       text NOT NULL,
    name           text NOT NULL,
    base_url       text NOT NULL,   -- the forge's web address
    webhook_secret text NOT NULL,   -- sealed; checks inbound deliveries
    token          text,            -- sealed; for comments on pull requests (none: no comments)
    created_by     uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at     timestamptz NOT NULL,
    updated_at     timestamptz NOT NULL,
    last_event_at  timestamptz,
    version        integer NOT NULL DEFAULT 1,
    CONSTRAINT vcs_connections_provider CHECK (provider IN ('github', 'gitlab', 'forgejo')),
    CONSTRAINT vcs_connections_name_length CHECK (char_length(name) BETWEEN 1 AND 100)
);

CREATE INDEX vcs_connections_workspace ON vcs_connections (workspace_id);

-- A link: an issue named by a branch, a pull request or a commit of a
-- repository. external_id is the branch name, the pull request number, or
-- the commit hash.
CREATE TABLE vcs_links (
    workspace_id  uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id            uuid PRIMARY KEY,
    issue_id      uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    connection_id uuid NOT NULL REFERENCES vcs_connections (id) ON DELETE CASCADE,
    kind          text NOT NULL,
    repository    text NOT NULL,
    external_id   text NOT NULL,
    url           text NOT NULL,
    title         text NOT NULL DEFAULT '',
    state         text NOT NULL DEFAULT '',  -- branches: open, deleted; pull requests: open, merged, closed
    author        text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL,
    updated_at    timestamptz NOT NULL,
    UNIQUE (issue_id, connection_id, kind, repository, external_id),
    CONSTRAINT vcs_links_kind CHECK (kind IN ('branch', 'pull_request', 'commit'))
);

CREATE INDEX vcs_links_ref ON vcs_links (connection_id, kind, repository, external_id);

-- The comment on a pull request listing its issues: written by the vcs job
-- while dirty, then edited in place.
CREATE TABLE vcs_comments (
    workspace_id    uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    connection_id   uuid NOT NULL REFERENCES vcs_connections (id) ON DELETE CASCADE,
    repository      text NOT NULL,
    number          text NOT NULL,
    forge_id        text,            -- the forge's comment id, once posted
    project_ref     text NOT NULL DEFAULT '', -- GitLab's numeric project id
    body            text NOT NULL DEFAULT '',  -- as last written
    dirty           boolean NOT NULL DEFAULT true,
    attempts        integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL,
    last_error      text NOT NULL DEFAULT '',
    updated_at      timestamptz NOT NULL,
    PRIMARY KEY (connection_id, repository, number)
);

CREATE INDEX vcs_comments_due ON vcs_comments (next_attempt_at) WHERE dirty;

-- +goose Down
DROP TABLE vcs_comments;
DROP TABLE vcs_links;
DROP TABLE vcs_connections;
