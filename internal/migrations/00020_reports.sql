-- +goose Up
-- Report projections (docs/reports.md#how-reports-are-computed, ADR-0021),
-- kept by the reports worker from the events table. They hold ids,
-- outcomes, weights and timestamps only, never issue content, and have no
-- foreign keys: a tombstone removes an issue's rows, and a rebuild replays
-- the log.

-- The projector's work queue, as published_at is the relay's: events not
-- yet applied to the projections. A rebuild clears it.
ALTER TABLE events ADD COLUMN reported_at timestamptz;
CREATE INDEX events_unreported ON events (id) WHERE reported_at IS NULL;

-- Each stay of an issue in a status, with the status's category and that
-- category's outcome (left_at NULL: where it is now).
CREATE TABLE report_status_intervals (
    workspace_id uuid        NOT NULL,
    id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id   uuid        NOT NULL,
    issue_id     uuid        NOT NULL,
    status_id    uuid        NOT NULL,
    category_id  uuid,
    outcome      text        NOT NULL,
    entered_at   timestamptz NOT NULL,
    left_at      timestamptz,
    CONSTRAINT report_status_intervals_outcome CHECK (outcome IN ('unstarted', 'started', 'completed', 'abandoned'))
);
CREATE INDEX report_status_intervals_issue ON report_status_intervals (issue_id, entered_at);
CREATE INDEX report_status_intervals_project ON report_status_intervals (workspace_id, project_id, entered_at);
CREATE INDEX report_status_intervals_category ON report_status_intervals (category_id);

-- Per issue, for lead and cycle time: when it was created, first started,
-- and completed or abandoned (NULL while it isn't).
CREATE TABLE report_issue_milestones (
    workspace_id     uuid        NOT NULL,
    issue_id         uuid        PRIMARY KEY,
    project_id       uuid        NOT NULL,
    created_at       timestamptz NOT NULL,
    first_started_at timestamptz,
    completed_at     timestamptz,
    abandoned_at     timestamptz
);
CREATE INDEX report_issue_milestones_project ON report_issue_milestones (workspace_id, project_id, completed_at);

-- Each stay of an issue in an iteration (removed_at NULL: still in it).
-- Committed, added, removed and how each ended follow from the
-- iteration's started_at and completed_at.
CREATE TABLE report_iteration_scope (
    workspace_id uuid        NOT NULL,
    id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    iteration_id uuid        NOT NULL,
    issue_id     uuid        NOT NULL,
    added_at     timestamptz NOT NULL,
    removed_at   timestamptz
);
CREATE INDEX report_iteration_scope_iteration ON report_iteration_scope (iteration_id, added_at);
CREATE INDEX report_iteration_scope_issue ON report_iteration_scope (issue_id);

-- Each period an issue had an estimate weight (NULL: not estimated).
CREATE TABLE report_estimates (
    workspace_id uuid        NOT NULL,
    id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    issue_id     uuid        NOT NULL,
    weight       double precision,
    valid_from   timestamptz NOT NULL,
    valid_to     timestamptz
);
CREATE INDEX report_estimates_issue ON report_estimates (issue_id, valid_from);

-- +goose Down
DROP TABLE report_estimates;
DROP TABLE report_iteration_scope;
DROP TABLE report_issue_milestones;
DROP TABLE report_status_intervals;
DROP INDEX events_unreported;
ALTER TABLE events DROP COLUMN reported_at;
