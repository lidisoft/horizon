-- +goose Up
-- Issues, their labels and links (docs/domain.md#issue, docs/data-model.md#work-items).
-- The description is plain text until the block editor (phase 5);
-- iterations and releases arrive with planning (phase 4).

CREATE TABLE issues (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    number       bigint NOT NULL,
    type_id      uuid NOT NULL REFERENCES issue_types (id),
    title        text NOT NULL,
    description  text NOT NULL DEFAULT '',
    status_id    uuid NOT NULL REFERENCES statuses (id),
    priority     text NOT NULL DEFAULT 'none',
    rank         text COLLATE "C" NOT NULL,
    estimate     text,                    -- a label of the scale, as estimated
    assignee_id  uuid REFERENCES users (id) ON DELETE SET NULL,
    reporter_id  uuid REFERENCES users (id) ON DELETE SET NULL,
    parent_id    uuid REFERENCES issues (id) ON DELETE SET NULL,
    iteration_id uuid,
    release_id   uuid,
    start_date   date,
    due_date     date,
    fields       jsonb NOT NULL DEFAULT '{}',
    started_at   timestamptz,             -- first entry into a started category
    resolved_at  timestamptz,             -- last entry into a final category
    archived_at  timestamptz,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    version      integer NOT NULL DEFAULT 1,
    UNIQUE (workspace_id, project_id, number),
    CONSTRAINT issues_title_length CHECK (char_length(title) BETWEEN 1 AND 255),
    CONSTRAINT issues_priority CHECK (priority IN ('none', 'urgent', 'high', 'medium', 'low')),
    CONSTRAINT issues_not_own_parent CHECK (parent_id <> id),
    CONSTRAINT issues_dates CHECK (start_date IS NULL OR due_date IS NULL OR start_date <= due_date)
);

CREATE INDEX issues_board ON issues (workspace_id, project_id, status_id, rank);
CREATE INDEX issues_rank ON issues (workspace_id, project_id, rank);
CREATE INDEX issues_parent ON issues (parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX issues_assignee ON issues (workspace_id, assignee_id) WHERE assignee_id IS NOT NULL;
CREATE INDEX issues_type ON issues (type_id);
CREATE INDEX issues_status ON issues (status_id);
CREATE INDEX issues_fields ON issues USING gin (fields jsonb_path_ops);
CREATE INDEX issues_title_trgm ON issues USING gin (title gin_trgm_ops);

CREATE TABLE issue_labels (
    workspace_id uuid NOT NULL,
    issue_id     uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    label_id     uuid NOT NULL REFERENCES labels (id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, label_id)
);

CREATE INDEX issue_labels_label ON issue_labels (label_id);

-- A link is stored once, from its source; the target sees the inverse type.
CREATE TABLE issue_links (
    workspace_id uuid NOT NULL,
    id           uuid PRIMARY KEY,
    source_id    uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    target_id    uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    type         text NOT NULL,
    lag_days     integer,
    created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL,
    UNIQUE (source_id, target_id, type),
    CONSTRAINT issue_links_type CHECK (type IN ('relates_to', 'blocks', 'duplicates', 'precedes')),
    CONSTRAINT issue_links_not_self CHECK (source_id <> target_id),
    CONSTRAINT issue_links_lag CHECK (lag_days IS NULL OR (type = 'precedes' AND lag_days BETWEEN -365 AND 365))
);

CREATE INDEX issue_links_target ON issue_links (target_id);

-- +goose Down
DROP TABLE issue_links;
DROP TABLE issue_labels;
DROP TABLE issues;
