-- +goose Up
-- Comments (plain text until the block editor, phase 5) and attachments
-- (docs/data-model.md#work-items, docs/architecture.md#object-storage).

CREATE TABLE comments (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    issue_id     uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    author_id    uuid REFERENCES users (id) ON DELETE SET NULL,
    body         text NOT NULL,
    created_at   timestamptz NOT NULL,
    edited_at    timestamptz,
    version      integer NOT NULL DEFAULT 1,
    CONSTRAINT comments_body_length CHECK (char_length(body) BETWEEN 1 AND 20000)
);

CREATE INDEX comments_issue ON comments (issue_id, id);

-- An attachment is pending from the presigned upload until the client
-- completes it; pending ones that are never completed expire.
CREATE TABLE attachments (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    issue_id     uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    uploader_id  uuid REFERENCES users (id) ON DELETE SET NULL,
    filename     text NOT NULL,
    content_type text NOT NULL,
    size         bigint NOT NULL,
    object_key   text NOT NULL UNIQUE,
    state        text NOT NULL DEFAULT 'pending',
    created_at   timestamptz NOT NULL,
    completed_at timestamptz,
    CONSTRAINT attachments_state CHECK (state IN ('pending', 'ready')),
    CONSTRAINT attachments_filename_length CHECK (char_length(filename) BETWEEN 1 AND 255)
);

CREATE INDEX attachments_issue ON attachments (issue_id, id);
CREATE INDEX attachments_pending ON attachments (created_at) WHERE state = 'pending';

-- +goose Down
DROP TABLE attachments;
DROP TABLE comments;
