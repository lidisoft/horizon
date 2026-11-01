-- +goose Up
-- Saved views (docs/domain.md#view): a query, a layout and display
-- options, personal (owner_id) or shared in a project or the workspace.
CREATE TABLE views (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    project_id   uuid REFERENCES projects (id) ON DELETE CASCADE,
    owner_id     uuid REFERENCES users (id) ON DELETE CASCADE, -- NULL: shared
    name         text NOT NULL,
    layout       text NOT NULL,
    query        jsonb NOT NULL DEFAULT '{}',
    display      jsonb NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    version      integer NOT NULL DEFAULT 1,
    CONSTRAINT views_name_length CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT views_layout CHECK (layout IN ('list', 'board', 'table', 'timeline', 'calendar'))
);

CREATE INDEX views_scope ON views (workspace_id, project_id);

-- +goose Down
DROP TABLE views;
