-- +goose Up
-- The version of a description its last IssueDescribed event covers: a
-- session that goes quiet, or whose last editor leaves, is described once
-- (docs/realtime.md#domain-events-for-documents).
ALTER TABLE documents ADD COLUMN described_version bigint NOT NULL DEFAULT 0;
UPDATE documents SET described_version = version;
CREATE INDEX documents_undescribed ON documents (updated_at) WHERE version > described_version;

-- +goose Down
DROP INDEX documents_undescribed;
ALTER TABLE documents DROP COLUMN described_version;
