-- +goose Up
-- Releases (a project's versions issues are targeted to) and milestones
-- (dated points, of a project or of the whole workspace)
-- (docs/domain.md#release, docs/domain.md#milestone).

CREATE TABLE releases (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name         text NOT NULL,
    description  text NOT NULL DEFAULT '',
    state        text NOT NULL DEFAULT 'unreleased',
    start_date   date,
    release_date date,
    released_at  timestamptz,
    archived_at  timestamptz,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    version      integer NOT NULL DEFAULT 1,
    CONSTRAINT releases_state CHECK (state IN ('unreleased', 'released', 'archived')),
    CONSTRAINT releases_name_length CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT releases_description_length CHECK (char_length(description) <= 2000),
    CONSTRAINT releases_dates CHECK (start_date IS NULL OR release_date IS NULL OR start_date <= release_date)
);

CREATE UNIQUE INDEX releases_name ON releases (project_id, lower(name));

ALTER TABLE issues ADD CONSTRAINT issues_release FOREIGN KEY (release_id) REFERENCES releases (id) ON DELETE SET NULL;
CREATE INDEX issues_release ON issues (release_id) WHERE release_id IS NOT NULL;

CREATE TABLE milestones (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    project_id   uuid REFERENCES projects (id) ON DELETE CASCADE, -- NULL: workspace-level
    name         text NOT NULL,
    description  text NOT NULL DEFAULT '',
    date         date NOT NULL,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    version      integer NOT NULL DEFAULT 1,
    CONSTRAINT milestones_name_length CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT milestones_description_length CHECK (char_length(description) <= 2000)
);

CREATE INDEX milestones_workspace ON milestones (workspace_id, date);

CREATE TABLE milestone_issues (
    workspace_id uuid NOT NULL,
    milestone_id uuid NOT NULL REFERENCES milestones (id) ON DELETE CASCADE,
    issue_id     uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    PRIMARY KEY (milestone_id, issue_id)
);

CREATE INDEX milestone_issues_issue ON milestone_issues (issue_id);

-- +goose Down
DROP TABLE milestone_issues;
DROP TABLE milestones;
DROP INDEX issues_release;
ALTER TABLE issues DROP CONSTRAINT issues_release;
DROP TABLE releases;
