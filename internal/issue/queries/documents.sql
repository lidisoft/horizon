-- name: DescriptionDocument :one
SELECT * FROM documents WHERE issue_id = $1;

-- name: CreateDescription :one
INSERT INTO documents (workspace_id, id, issue_id, version, snapshot_version, content, updated_at)
VALUES ($1, $2, $3, 0, 0, $4, $5)
ON CONFLICT (issue_id) DO NOTHING
RETURNING *;

-- name: LockDescription :one
SELECT * FROM documents WHERE issue_id = $1 FOR UPDATE;

-- name: StepsSince :many
-- The batches applied since version (each against the version it names).
SELECT * FROM document_steps WHERE document_id = $1 AND version >= $2 ORDER BY version;

-- name: AddSteps :exec
INSERT INTO document_steps (document_id, version, steps, client_id, user_id, created_at) VALUES ($1, $2, $3, $4, $5, $6);

-- name: SetDocumentVersion :exec
UPDATE documents SET version = $2, updated_at = $3 WHERE id = $1;

-- name: Snapshot :exec
-- The content at version; batches older than the retained window go.
UPDATE documents SET content = $2, snapshot_version = $3, updated_at = $4 WHERE id = $1;

-- name: CompactSteps :exec
DELETE FROM document_steps WHERE document_id = $1 AND version < $2;
