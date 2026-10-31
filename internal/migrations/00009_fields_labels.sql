-- +goose Up
-- Custom fields, their attachment to issue types, and labels
-- (docs/configuration.md#custom-fields, #labels). Estimation scales are a
-- setting, kept in workspaces.settings and projects.settings.

CREATE TABLE custom_fields (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    project_id   uuid REFERENCES projects (id) ON DELETE CASCADE,
    name         text NOT NULL,
    type         text NOT NULL,
    description  text NOT NULL DEFAULT '',
    options      jsonb NOT NULL DEFAULT '{}',  -- select options; number unit and precision
    position     integer NOT NULL,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    version      integer NOT NULL DEFAULT 1,
    CONSTRAINT custom_fields_type CHECK (type IN ('text', 'number', 'select', 'multi_select', 'date', 'user', 'url', 'checkbox')),
    CONSTRAINT custom_fields_name_length CHECK (char_length(name) BETWEEN 1 AND 60)
);

CREATE UNIQUE INDEX custom_fields_name ON custom_fields
    (workspace_id, coalesce(project_id, '00000000-0000-0000-0000-000000000000'), lower(name));

CREATE TABLE issue_type_fields (
    workspace_id  uuid NOT NULL,
    issue_type_id uuid NOT NULL REFERENCES issue_types (id) ON DELETE CASCADE,
    field_id      uuid NOT NULL REFERENCES custom_fields (id) ON DELETE CASCADE,
    required      boolean NOT NULL DEFAULT false,
    position      integer NOT NULL,
    PRIMARY KEY (issue_type_id, field_id)
);

CREATE INDEX issue_type_fields_field ON issue_type_fields (field_id);

CREATE TABLE labels (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    project_id   uuid REFERENCES projects (id) ON DELETE CASCADE,
    name         text NOT NULL,
    colour       text NOT NULL DEFAULT 'grey',
    description  text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    version      integer NOT NULL DEFAULT 1,
    CONSTRAINT labels_name_length CHECK (char_length(name) BETWEEN 1 AND 40),
    CONSTRAINT labels_description_length CHECK (char_length(description) <= 200)
);

CREATE UNIQUE INDEX labels_name ON labels
    (workspace_id, coalesce(project_id, '00000000-0000-0000-0000-000000000000'), lower(name));

-- +goose Down
DROP TABLE labels;
DROP TABLE issue_type_fields;
DROP TABLE custom_fields;
