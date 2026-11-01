-- name: CreateComment :one
INSERT INTO comments (workspace_id, id, issue_id, author_id, body, body_doc, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: IssueComments :many
-- An issue's comments, oldest first, after cursor (a comment id).
SELECT c.*, coalesce(u.name, '')::text AS author_name FROM comments c LEFT JOIN users u ON u.id = c.author_id
WHERE c.issue_id = sqlc.arg(issue_id) AND c.id > sqlc.arg(after)
ORDER BY c.id
LIMIT sqlc.arg(max_rows);

-- name: LockComment :one
SELECT * FROM comments WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: EditComment :one
UPDATE comments SET body = $3, body_doc = $4, edited_at = $5, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: DeleteComment :exec
DELETE FROM comments WHERE workspace_id = $1 AND id = $2;

-- name: CreateAttachment :one
INSERT INTO attachments (workspace_id, id, issue_id, uploader_id, filename, content_type, size, object_key, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: LockAttachment :one
SELECT * FROM attachments WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: AttachmentByID :one
SELECT * FROM attachments WHERE workspace_id = $1 AND id = $2;

-- name: CompleteAttachment :one
UPDATE attachments SET state = 'ready', content_type = $3, completed_at = $4
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: IssueAttachments :many
SELECT a.*, coalesce(u.name, '')::text AS uploader_name FROM attachments a LEFT JOIN users u ON u.id = a.uploader_id
WHERE a.issue_id = $1 AND a.state = 'ready'
ORDER BY a.id;

-- name: DeleteAttachment :exec
DELETE FROM attachments WHERE workspace_id = $1 AND id = $2;

-- name: AttachmentKeys :many
-- Object keys of issues' attachments, pending ones included, for purges.
SELECT object_key FROM attachments WHERE issue_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ExpiredUploads :many
DELETE FROM attachments WHERE state = 'pending' AND created_at < $1
RETURNING workspace_id, id, object_key;

-- name: ReadyAttachmentIDs :many
-- Which of the ids are ready attachments of the issue, for images.
SELECT id FROM attachments WHERE issue_id = $1 AND state = 'ready' AND id = ANY(sqlc.arg(ids)::uuid[]);
