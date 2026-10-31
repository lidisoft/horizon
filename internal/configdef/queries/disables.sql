-- name: Disables :many
SELECT definition_type, definition_id FROM config_disables WHERE project_id = $1;

-- name: Disable :exec
INSERT INTO config_disables (workspace_id, project_id, definition_type, definition_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT DO NOTHING;

-- name: Enable :exec
DELETE FROM config_disables WHERE project_id = $1 AND definition_type = $2 AND definition_id = $3;

-- name: ProjectsDisabling :many
-- Projects that switched a definition off, for usage counts.
SELECT project_id FROM config_disables WHERE definition_type = $1 AND definition_id = $2;
