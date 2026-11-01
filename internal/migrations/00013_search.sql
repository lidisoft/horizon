-- +goose Up
-- Full-text search on issues and comments (ADR-0020): GIN expression
-- indexes, kept current by every write in its transaction, without a
-- tsvector column in the rows every read returns. Queries must use the
-- same expressions (internal/search). The 'simple' configuration doesn't
-- stem: it suits mixed-language workspaces and identifiers.

CREATE INDEX issues_search ON issues USING gin (
    (setweight(to_tsvector('simple', title), 'A') || setweight(to_tsvector('simple', description), 'B')));
CREATE INDEX comments_search ON comments USING gin ((to_tsvector('simple', body)));

-- +goose Down
DROP INDEX comments_search;
DROP INDEX issues_search;
