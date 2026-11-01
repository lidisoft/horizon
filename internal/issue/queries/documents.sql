-- name: DescriptionDocument :one
SELECT * FROM documents WHERE issue_id = $1;

-- name: SaveDescription :one
-- Writes an issue's description document whole (a new version).
INSERT INTO documents (workspace_id, id, issue_id, version, content, updated_at)
VALUES ($1, $2, $3, 1, $4, $5)
ON CONFLICT (issue_id) DO UPDATE SET content = EXCLUDED.content, version = documents.version + 1, updated_at = EXCLUDED.updated_at
RETURNING *;
