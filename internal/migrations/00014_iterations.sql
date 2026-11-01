-- +goose Up
-- Iterations: time-boxed sprints of a project (docs/domain.md#iteration).
-- An issue sits in at most one, through issues.iteration_id.

CREATE TABLE iterations (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name         text NOT NULL,
    goal         text NOT NULL DEFAULT '',
    state        text NOT NULL DEFAULT 'planned',
    start_date   date,
    end_date     date,
    started_at   timestamptz,
    completed_at timestamptz,
    archived_at  timestamptz,
    -- At completion: the policy applied and where unfinished issues go
    -- ({"policy": "…", "rollover_to": id or null}).
    completion   jsonb,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    version      integer NOT NULL DEFAULT 1,
    CONSTRAINT iterations_state CHECK (state IN ('planned', 'scheduled', 'active', 'completed', 'archived')),
    CONSTRAINT iterations_name_length CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT iterations_goal_length CHECK (char_length(goal) <= 1000),
    CONSTRAINT iterations_dates CHECK (start_date IS NULL OR end_date IS NULL OR start_date <= end_date),
    CONSTRAINT iterations_dated CHECK (state = 'planned' OR (start_date IS NOT NULL AND end_date IS NOT NULL))
);

CREATE INDEX iterations_project ON iterations (workspace_id, project_id, state);

ALTER TABLE issues ADD CONSTRAINT issues_iteration FOREIGN KEY (iteration_id) REFERENCES iterations (id) ON DELETE SET NULL;
CREATE INDEX issues_iteration ON issues (iteration_id) WHERE iteration_id IS NOT NULL;

-- +goose Down
DROP INDEX issues_iteration;
ALTER TABLE issues DROP CONSTRAINT issues_iteration;
DROP TABLE iterations;
