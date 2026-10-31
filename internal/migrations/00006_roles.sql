-- +goose Up
-- Roles, permission sets and grants (ADR-0013, docs/security.md#authorisation).

CREATE TABLE roles (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    name         citext NOT NULL,
    description  text NOT NULL DEFAULT '',
    builtin      text,                    -- admin, facilitator, contributor, viewer, guest; null: custom
    permissions  text[] NOT NULL DEFAULT '{}',
    position     integer NOT NULL,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    version      integer NOT NULL DEFAULT 1,
    UNIQUE (workspace_id, name),
    UNIQUE (workspace_id, builtin),
    CONSTRAINT roles_name_length CHECK (char_length(name) BETWEEN 1 AND 40),
    CONSTRAINT roles_builtin CHECK (builtin IN ('admin', 'facilitator', 'contributor', 'viewer', 'guest'))
);

-- A grant gives a role to one member or one team, on the whole workspace
-- (project_id null) or on one project. One role per subject and scope.
CREATE TABLE role_grants (
    workspace_id uuid NOT NULL,
    id           uuid PRIMARY KEY,
    role_id      uuid NOT NULL REFERENCES roles (id) ON DELETE RESTRICT,
    user_id      uuid,
    team_id      uuid REFERENCES teams (id) ON DELETE CASCADE,
    project_id   uuid,                    -- foreign key added with projects
    granted_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL,
    version      integer NOT NULL DEFAULT 1,
    -- Leaving the workspace removes a member's grants; suspension keeps them.
    FOREIGN KEY (workspace_id, user_id) REFERENCES workspace_members (workspace_id, user_id) ON DELETE CASCADE,
    CONSTRAINT role_grants_one_subject CHECK ((user_id IS NULL) <> (team_id IS NULL))
);

CREATE UNIQUE INDEX role_grants_subject_scope ON role_grants
    (workspace_id, coalesce(user_id, team_id), coalesce(project_id, '00000000-0000-0000-0000-000000000000'));
CREATE INDEX role_grants_role ON role_grants (role_id);

-- Invitations name the workspace role the newcomer gets.
ALTER TABLE invitations ADD COLUMN role_id uuid REFERENCES roles (id) ON DELETE SET NULL;

-- Existing workspaces get the default roles (the same sets as
-- internal/access/defaults.go at this version); their first member gets the
-- Admin role and every other member Contributor, the role invitations give.
INSERT INTO roles (workspace_id, id, name, description, builtin, permissions, position, created_at, updated_at)
SELECT w.id, gen_random_uuid(), d.name, d.description, d.builtin, d.permissions, d.position, now(), now()
FROM workspaces w
CROSS JOIN (VALUES
    ('Admin', 'Everything, including workspace configuration, roles and members', 'admin', 0, ARRAY[
        'workspace.manage', 'workspace.members.manage', 'workspace.roles.manage', 'workspace.config.manage',
        'workspace.webhooks.manage', 'workspace.integrations.manage',
        'project.create', 'project.view', 'project.manage', 'project.config.manage',
        'issue.view', 'issue.create', 'issue.edit', 'issue.transition', 'issue.assign', 'issue.estimate',
        'issue.rank', 'issue.link', 'issue.move', 'issue.archive', 'issue.delete', 'issue.comment',
        'issue.comment.moderate', 'issue.attach',
        'iteration.manage', 'release.manage', 'milestone.manage', 'view.share']),
    ('Facilitator', 'Contributor, plus planning, ranking and project boards', 'facilitator', 1, ARRAY[
        'project.view', 'project.config.manage',
        'issue.view', 'issue.create', 'issue.edit', 'issue.transition', 'issue.assign', 'issue.estimate',
        'issue.rank', 'issue.link', 'issue.move', 'issue.archive', 'issue.comment', 'issue.comment.moderate',
        'issue.attach',
        'iteration.manage', 'release.manage', 'milestone.manage', 'view.share']),
    ('Contributor', 'Create, edit, transition, comment on and archive issues', 'contributor', 2, ARRAY[
        'project.view',
        'issue.view', 'issue.create', 'issue.edit', 'issue.transition', 'issue.assign', 'issue.estimate',
        'issue.link', 'issue.archive', 'issue.comment', 'issue.attach']),
    ('Viewer', 'Read and comment', 'viewer', 3, ARRAY['project.view', 'issue.view', 'issue.comment']),
    ('Guest', 'Read and comment on the projects shared with them; no member list', 'guest', 4,
        ARRAY['project.view', 'issue.view', 'issue.comment'])
) AS d (name, description, builtin, position, permissions);

INSERT INTO role_grants (workspace_id, id, role_id, user_id, created_at)
SELECT DISTINCT ON (m.workspace_id) m.workspace_id, gen_random_uuid(), r.id, m.user_id, now()
FROM workspace_members m
JOIN roles r ON r.workspace_id = m.workspace_id AND r.builtin = 'admin'
ORDER BY m.workspace_id, m.joined_at;

INSERT INTO role_grants (workspace_id, id, role_id, user_id, created_at)
SELECT m.workspace_id, gen_random_uuid(), r.id, m.user_id, now()
FROM workspace_members m
JOIN roles r ON r.workspace_id = m.workspace_id AND r.builtin = 'contributor'
WHERE NOT EXISTS (SELECT 1 FROM role_grants g WHERE g.workspace_id = m.workspace_id AND g.user_id = m.user_id);

-- +goose Down
ALTER TABLE invitations DROP COLUMN role_id;
DROP TABLE role_grants;
DROP TABLE roles;
