-- +goose Up
-- Status categories, workflows and issue types (docs/configuration.md):
-- definitions live at workspace scope (project_id null) or project scope; a
-- project inherits the workspace's, can switch them off, and add its own.

CREATE TABLE status_categories (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    project_id   uuid REFERENCES projects (id) ON DELETE CASCADE,
    name         text NOT NULL,
    colour       text NOT NULL DEFAULT 'grey',
    outcome      text NOT NULL,
    position     integer NOT NULL,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    version      integer NOT NULL DEFAULT 1,
    CONSTRAINT status_categories_outcome CHECK (outcome IN ('unstarted', 'started', 'completed', 'abandoned')),
    CONSTRAINT status_categories_name_length CHECK (char_length(name) BETWEEN 1 AND 40)
);

CREATE UNIQUE INDEX status_categories_name ON status_categories
    (workspace_id, coalesce(project_id, '00000000-0000-0000-0000-000000000000'), lower(name));

CREATE TABLE workflows (
    workspace_id     uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id               uuid PRIMARY KEY,
    project_id       uuid REFERENCES projects (id) ON DELETE CASCADE,
    name             text NOT NULL,
    copied_from      uuid REFERENCES workflows (id) ON DELETE SET NULL,
    draft            jsonb,                   -- statuses and transitions being edited; null: none
    draft_updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
    draft_updated_at timestamptz,
    draft_version    integer NOT NULL DEFAULT 0,  -- optimistic concurrency of the shared draft
    published_at     timestamptz NOT NULL,
    created_at       timestamptz NOT NULL,
    updated_at       timestamptz NOT NULL,
    version          integer NOT NULL DEFAULT 1,
    CONSTRAINT workflows_name_length CHECK (char_length(name) BETWEEN 1 AND 60)
);

CREATE UNIQUE INDEX workflows_name ON workflows
    (workspace_id, coalesce(project_id, '00000000-0000-0000-0000-000000000000'), lower(name));

-- The published statuses and transitions of each workflow.
CREATE TABLE statuses (
    workflow_id uuid NOT NULL REFERENCES workflows (id) ON DELETE CASCADE,
    id          uuid PRIMARY KEY,
    name        text NOT NULL,
    category_id uuid NOT NULL REFERENCES status_categories (id),
    position    integer NOT NULL,
    is_initial  boolean NOT NULL DEFAULT false
);

CREATE INDEX statuses_workflow ON statuses (workflow_id, position);
CREATE INDEX statuses_category ON statuses (category_id);

CREATE TABLE transitions (
    workflow_id    uuid NOT NULL REFERENCES workflows (id) ON DELETE CASCADE,
    id             uuid PRIMARY KEY,
    from_status_id uuid REFERENCES statuses (id) ON DELETE CASCADE,  -- null: from any status
    to_status_id   uuid NOT NULL REFERENCES statuses (id) ON DELETE CASCADE,
    name           text NOT NULL,
    rules          jsonb NOT NULL DEFAULT '{}'
);

CREATE INDEX transitions_workflow ON transitions (workflow_id);

CREATE TABLE issue_types (
    workspace_id            uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id                      uuid PRIMARY KEY,
    project_id              uuid REFERENCES projects (id) ON DELETE CASCADE,
    name                    text NOT NULL,
    icon                    text NOT NULL,
    colour                  text NOT NULL DEFAULT 'blue',
    workflow_id             uuid NOT NULL REFERENCES workflows (id),
    parent_rule             text NOT NULL DEFAULT 'any',  -- any, none, required
    allowed_parent_type_ids uuid[] NOT NULL DEFAULT '{}', -- empty: any type
    position                integer NOT NULL,
    copied_from             uuid REFERENCES issue_types (id) ON DELETE SET NULL,
    created_at              timestamptz NOT NULL,
    updated_at              timestamptz NOT NULL,
    version                 integer NOT NULL DEFAULT 1,
    CONSTRAINT issue_types_parent_rule CHECK (parent_rule IN ('any', 'none', 'required')),
    CONSTRAINT issue_types_name_length CHECK (char_length(name) BETWEEN 1 AND 40)
);

CREATE UNIQUE INDEX issue_types_name ON issue_types
    (workspace_id, coalesce(project_id, '00000000-0000-0000-0000-000000000000'), lower(name));

-- Inherited definitions a project switched off.
CREATE TABLE config_disables (
    workspace_id    uuid NOT NULL,
    project_id      uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    definition_type text NOT NULL,
    definition_id   uuid NOT NULL,
    PRIMARY KEY (project_id, definition_type, definition_id),
    CONSTRAINT config_disables_type CHECK (definition_type IN ('status_category', 'workflow', 'issue_type', 'custom_field', 'label'))
);

-- Existing workspaces get the built-in defaults (internal/configdef/defaults.go
-- at this version).
-- +goose StatementBegin
DO $$
DECLARE
    w record;
    c_backlog uuid; c_todo uuid; c_progress uuid; c_done uuid; c_canceled uuid;
    wf uuid; s_backlog uuid; s_todo uuid; s_progress uuid; s_done uuid; s_canceled uuid;
    t_epic uuid; t_story uuid; t_task uuid; t_bug uuid;
BEGIN
    FOR w IN SELECT id FROM workspaces LOOP
        c_backlog := gen_random_uuid(); c_todo := gen_random_uuid(); c_progress := gen_random_uuid();
        c_done := gen_random_uuid(); c_canceled := gen_random_uuid();
        INSERT INTO status_categories (workspace_id, id, name, colour, outcome, position, created_at, updated_at) VALUES
            (w.id, c_backlog, 'Backlog', 'grey', 'unstarted', 0, now(), now()),
            (w.id, c_todo, 'Todo', 'grey', 'unstarted', 1, now(), now()),
            (w.id, c_progress, 'In progress', 'blue', 'started', 2, now(), now()),
            (w.id, c_done, 'Done', 'green', 'completed', 3, now(), now()),
            (w.id, c_canceled, 'Canceled', 'grey', 'abandoned', 4, now(), now());

        wf := gen_random_uuid();
        INSERT INTO workflows (workspace_id, id, name, published_at, created_at, updated_at)
            VALUES (w.id, wf, 'Default', now(), now(), now());
        s_backlog := gen_random_uuid(); s_todo := gen_random_uuid(); s_progress := gen_random_uuid();
        s_done := gen_random_uuid(); s_canceled := gen_random_uuid();
        INSERT INTO statuses (workflow_id, id, name, category_id, position, is_initial) VALUES
            (wf, s_backlog, 'Backlog', c_backlog, 0, true),
            (wf, s_todo, 'Todo', c_todo, 1, false),
            (wf, s_progress, 'In progress', c_progress, 2, false),
            (wf, s_done, 'Done', c_done, 3, false),
            (wf, s_canceled, 'Canceled', c_canceled, 4, false);
        INSERT INTO transitions (workflow_id, id, from_status_id, to_status_id, name) VALUES
            (wf, gen_random_uuid(), s_backlog, s_todo, 'Plan'),
            (wf, gen_random_uuid(), s_todo, s_backlog, 'Unplan'),
            (wf, gen_random_uuid(), s_todo, s_progress, 'Start'),
            (wf, gen_random_uuid(), s_progress, s_todo, 'Stop'),
            (wf, gen_random_uuid(), s_progress, s_done, 'Finish'),
            (wf, gen_random_uuid(), s_done, s_progress, 'Reopen'),
            (wf, gen_random_uuid(), NULL, s_canceled, 'Cancel'),
            (wf, gen_random_uuid(), s_canceled, s_backlog, 'Restore');

        t_epic := gen_random_uuid(); t_story := gen_random_uuid(); t_task := gen_random_uuid(); t_bug := gen_random_uuid();
        INSERT INTO issue_types (workspace_id, id, name, icon, colour, workflow_id, parent_rule, allowed_parent_type_ids, position, created_at, updated_at) VALUES
            (w.id, t_epic, 'Epic', 'layers', 'purple', wf, 'none', '{}', 0, now(), now()),
            (w.id, t_story, 'Story', 'bookmark', 'green', wf, 'any', '{}', 1, now(), now()),
            (w.id, t_task, 'Task', 'check', 'blue', wf, 'any', '{}', 2, now(), now()),
            (w.id, t_bug, 'Bug', 'alert-circle', 'coral', wf, 'any', '{}', 3, now(), now()),
            (w.id, gen_random_uuid(), 'Sub-task', 'corner-down-right', 'teal', wf, 'required', ARRAY[t_story, t_task, t_bug], 4, now(), now());
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE config_disables;
DROP TABLE issue_types;
DROP TABLE transitions;
DROP TABLE statuses;
DROP TABLE workflows;
DROP TABLE status_categories;
