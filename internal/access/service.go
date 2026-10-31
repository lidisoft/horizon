package access

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lidisoft/horizon/internal/access/db"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/platform/postgres"
)

// Service manages roles and grants and answers permission questions.
type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// New returns a Service; now defaults to time.Now.
func New(pool *pgxpool.Pool, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{pool: pool, now: func() time.Time { return now().UTC().Truncate(time.Microsecond) }}
}

// --- Checking -------------------------------------------------------------------

// Can implements Checker: the union of the user's grants and their teams',
// at workspace scope and on scope's project. Instance admins hold every
// permission everywhere: they recover workspaces whose admins left.
func (s *Service) Can(ctx context.Context, p identity.Principal, scope Scope, perm Permission) (bool, error) {
	if p.User.IsInstanceAdmin {
		return true, nil
	}
	perms, err := s.permissions(ctx, p, scope)
	if err != nil {
		return false, err
	}
	return slices.Contains(perms, string(perm)), nil
}

// Permissions lists what p holds in scope, sorted; every permission for an
// instance admin. Token scopes are not applied: see Require.
func (s *Service) Permissions(ctx context.Context, p identity.Principal, scope Scope) ([]Permission, error) {
	if p.User.IsInstanceAdmin {
		return all(), nil
	}
	perms, err := s.permissions(ctx, p, scope)
	if err != nil {
		return nil, err
	}
	out := make([]Permission, 0, len(perms))
	for _, a := range Catalogue {
		for _, k := range a.Permissions {
			if slices.Contains(perms, string(k)) {
				out = append(out, k)
			}
		}
	}
	return out, nil
}

func (s *Service) permissions(ctx context.Context, p identity.Principal, scope Scope) ([]string, error) {
	return db.New(s.pool).Permissions(ctx, db.PermissionsParams{UserID: p.User.ID, WorkspaceID: scope.WorkspaceID, ProjectID: scope.ProjectID})
}

// SeesWorkspace reports whether p may see the workspace-wide lists (members,
// teams): instance admins and holders of a workspace-scope grant. Guests,
// granted on projects only, can't.
func (s *Service) SeesWorkspace(ctx context.Context, p identity.Principal, workspaceID uuid.UUID) (bool, error) {
	if p.User.IsInstanceAdmin {
		return true, nil
	}
	return db.New(s.pool).HasWorkspaceGrant(ctx, db.HasWorkspaceGrantParams{WorkspaceID: workspaceID, UserID: p.User.ID})
}

// --- Roles ----------------------------------------------------------------------

// Role is a named permission set of a workspace.
type Role struct {
	ID          uuid.UUID    `json:"id"`
	Name        string       `json:"name" example:"Contributor"`
	Description string       `json:"description"`
	Builtin     string       `json:"builtin,omitempty" enums:"admin,facilitator,contributor,viewer,guest"`
	Permissions []Permission `json:"permissions"`
	People      int64        `json:"people"`
	Teams       int64        `json:"teams"`
	Version     int32        `json:"version"`
}

func roleFrom(r db.Role) Role {
	perms := make([]Permission, len(r.Permissions))
	for i, p := range r.Permissions {
		perms[i] = Permission(p)
	}
	b := ""
	if r.Builtin != nil {
		b = *r.Builtin
	}
	return Role{ID: r.ID, Name: r.Name, Description: r.Description, Builtin: b, Permissions: perms, Version: r.Version}
}

// SeedDefaults creates the default roles of a new workspace inside tx and
// returns them by builtin key.
func (s *Service) SeedDefaults(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, actor uuid.NullUUID) (map[string]uuid.UUID, error) {
	q := db.New(tx)
	ids := map[string]uuid.UUID{}
	for i, d := range defaultRoles {
		builtin := d.builtin
		r, err := q.CreateRole(ctx, db.CreateRoleParams{
			WorkspaceID: workspaceID, ID: uuid.Must(uuid.NewV7()), Name: d.name, Description: d.description,
			Builtin: &builtin, Permissions: keys(d.permissions), Position: int32(i), CreatedAt: s.now(),
		})
		if err != nil {
			return nil, err
		}
		if err := eventlog.Append(ctx, tx, s.event(workspaceID, actor, "role", r.ID, int64(r.Version), "RoleCreated",
			map[string]any{"builtin": builtin, "permissions": r.Permissions})); err != nil {
			return nil, err
		}
		ids[builtin] = r.ID
	}
	return ids, nil
}

// BuiltinRole returns the id of a built-in role, inside tx.
func (s *Service) BuiltinRole(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, builtin string) (uuid.UUID, error) {
	r, err := db.New(tx).RoleByBuiltin(ctx, db.RoleByBuiltinParams{WorkspaceID: workspaceID, Builtin: &builtin})
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, apperr.NotFound("role_not_found")
	}
	return r.ID, err
}

// Roles lists the workspace's roles with how many people and teams hold
// each. Every member who sees the workspace may read them.
func (s *Service) Roles(ctx context.Context, workspaceID uuid.UUID) ([]Role, error) {
	q := db.New(s.pool)
	rows, err := q.ListRoles(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	usage, err := q.RoleUsage(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]Role, len(rows))
	for i, r := range rows {
		out[i] = roleFrom(r)
		for _, u := range usage {
			if u.RoleID == r.ID {
				out[i].People, out[i].Teams = u.People, u.Teams
			}
		}
	}
	return out, nil
}

// RoleInput is the input of CreateRole and UpdateRole; nil fields are left
// unchanged on update.
type RoleInput struct {
	Name        *string
	Description *string
	Permissions *[]Permission
}

func validateRole(in RoleInput, creating bool) (name, desc string, perms []string, fields apperr.Fields) {
	if in.Name != nil || creating {
		if in.Name != nil {
			name = strings.TrimSpace(*in.Name)
		}
		switch {
		case name == "":
			fields.Add("name", "required")
		case len([]rune(name)) > 40:
			fields.Add("name", "too_long")
		}
	}
	if in.Description != nil {
		desc = strings.TrimSpace(*in.Description)
		if len([]rune(desc)) > 200 {
			fields.Add("description", "too_long")
		}
	}
	if in.Permissions != nil {
		seen := map[Permission]bool{}
		perms = []string{}
		for _, p := range *in.Permissions {
			if !Known(p) {
				fields.Add("permissions", "unknown")
				break
			}
			if !seen[p] {
				seen[p] = true
				perms = append(perms, string(p))
			}
		}
	}
	return name, desc, perms, fields
}

// CreateRole adds a custom role (workspace.roles.manage, checked by the
// caller through Require).
func (s *Service) CreateRole(ctx context.Context, workspaceID uuid.UUID, actor uuid.NullUUID, in RoleInput) (Role, error) {
	name, desc, perms, fields := validateRole(in, true)
	if err := fields.Err(); err != nil {
		return Role{}, err
	}
	if perms == nil {
		perms = []string{}
	}
	var r db.Role
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		pos, err := q.NextRolePosition(ctx, workspaceID)
		if err != nil {
			return err
		}
		r, err = q.CreateRole(ctx, db.CreateRoleParams{
			WorkspaceID: workspaceID, ID: uuid.Must(uuid.NewV7()), Name: name, Description: desc,
			Permissions: perms, Position: pos, CreatedAt: s.now(),
		})
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(workspaceID, actor, "role", r.ID, int64(r.Version), "RoleCreated",
			map[string]any{"permissions": r.Permissions}))
	})
	if err != nil {
		return Role{}, err
	}
	return roleFrom(r), nil
}

// UpdateRole renames a role or replaces its permissions. The Admin role
// keeps workspace.roles.manage, so a workspace can't lock itself out.
func (s *Service) UpdateRole(ctx context.Context, workspaceID uuid.UUID, actor uuid.NullUUID, id uuid.UUID, version int32, in RoleInput) (Role, error) {
	name, desc, perms, fields := validateRole(in, false)
	if err := fields.Err(); err != nil {
		return Role{}, err
	}
	var r db.Role
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := s.lockRole(ctx, q, workspaceID, id)
		if err != nil {
			return err
		}
		if version != 0 && cur.Version != version {
			return apperr.Conflict("version_conflict")
		}
		p := db.UpdateRoleParams{WorkspaceID: workspaceID, ID: id, Name: cur.Name, Description: cur.Description, Permissions: cur.Permissions, UpdatedAt: s.now()}
		var changed []string
		if in.Name != nil && name != cur.Name {
			p.Name = name
			changed = append(changed, "name")
		}
		if in.Description != nil && desc != cur.Description {
			p.Description = desc
			changed = append(changed, "description")
		}
		if in.Permissions != nil && !sameSet(perms, cur.Permissions) {
			if cur.Builtin != nil && *cur.Builtin == BuiltinAdmin && !slices.Contains(perms, string(WorkspaceRolesManage)) {
				return apperr.Conflict("admin_keeps_roles_manage")
			}
			p.Permissions = perms
			changed = append(changed, "permissions")
		}
		if len(changed) == 0 {
			r = cur
			return nil
		}
		r, err = q.UpdateRole(ctx, p)
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(workspaceID, actor, "role", r.ID, int64(r.Version), "RoleUpdated",
			map[string]any{"fields": changed, "permissions": r.Permissions}))
	})
	if err != nil {
		return Role{}, err
	}
	return roleFrom(r), nil
}

// DeleteRole removes a role nobody holds. The Admin role can't be deleted.
func (s *Service) DeleteRole(ctx context.Context, workspaceID uuid.UUID, actor uuid.NullUUID, id uuid.UUID) error {
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		r, err := s.lockRole(ctx, q, workspaceID, id)
		if err != nil {
			return err
		}
		if r.Builtin != nil && *r.Builtin == BuiltinAdmin {
			return apperr.Conflict("admin_role_required")
		}
		n, err := q.CountRoleGrants(ctx, id)
		if err != nil {
			return err
		}
		if n > 0 {
			return apperr.Conflict("role_in_use")
		}
		if err := q.DeleteRole(ctx, db.DeleteRoleParams{WorkspaceID: workspaceID, ID: id}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(workspaceID, actor, "role", id, int64(r.Version)+1, "RoleDeleted", nil))
	})
}

func (s *Service) lockRole(ctx context.Context, q *db.Queries, workspaceID, id uuid.UUID) (db.Role, error) {
	r, err := q.LockRole(ctx, db.LockRoleParams{WorkspaceID: workspaceID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Role{}, apperr.NotFound("role_not_found")
	}
	return r, err
}

// --- Grants ---------------------------------------------------------------------

// Grant gives a role to a member or a team, on the workspace or a project.
type Grant struct {
	ID           uuid.UUID     `json:"id"`
	RoleID       uuid.UUID     `json:"role_id"`
	RoleName     string        `json:"role_name"`
	UserID       uuid.NullUUID `json:"user_id" swaggertype:"string"`
	TeamID       uuid.NullUUID `json:"team_id" swaggertype:"string"`
	ProjectID    uuid.NullUUID `json:"project_id" swaggertype:"string"`
	SubjectName  string        `json:"subject_name"`
	SubjectEmail string        `json:"subject_email,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
}

// Grants lists every grant of the workspace, workspace scope first.
func (s *Service) Grants(ctx context.Context, workspaceID uuid.UUID) ([]Grant, error) {
	rows, err := db.New(s.pool).ListGrants(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]Grant, len(rows))
	for i, r := range rows {
		out[i] = Grant{
			ID: r.ID, RoleID: r.RoleID, RoleName: r.RoleName, UserID: r.UserID, TeamID: r.TeamID, ProjectID: r.ProjectID,
			SubjectName: r.SubjectName, SubjectEmail: r.SubjectEmail, CreatedAt: r.CreatedAt,
		}
	}
	return out, nil
}

// GrantInput sets the role of one subject (UserID or TeamID) in a scope.
type GrantInput struct {
	RoleID    uuid.UUID
	UserID    uuid.NullUUID
	TeamID    uuid.NullUUID
	ProjectID uuid.NullUUID
}

// SetGrant gives the subject the role in the scope, replacing their previous
// role there.
func (s *Service) SetGrant(ctx context.Context, workspaceID uuid.UUID, actor uuid.NullUUID, in GrantInput) (Grant, error) {
	var g Grant
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		g, err = s.SetGrantInTx(ctx, tx, workspaceID, actor, in)
		return err
	})
	return g, err
}

// SetGrantInTx is SetGrant inside the caller's transaction (creating a
// workspace, accepting an invitation).
func (s *Service) SetGrantInTx(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, actor uuid.NullUUID, in GrantInput) (Grant, error) {
	if in.UserID.Valid == in.TeamID.Valid {
		return Grant{}, apperr.Invalid(apperr.Field{Field: "subject", Code: "one_of_user_team"})
	}
	q := db.New(tx)
	role, err := q.RoleInWorkspace(ctx, db.RoleInWorkspaceParams{WorkspaceID: workspaceID, ID: in.RoleID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Grant{}, apperr.Invalid(apperr.Field{Field: "role_id", Code: "unknown"})
	}
	if err != nil {
		return Grant{}, err
	}
	// Taking Admin away from the last workspace-scope holder would lock the
	// workspace: an existing Admin grant changing role counts.
	if err := s.guardLastAdmin(ctx, q, workspaceID, in, role); err != nil {
		return Grant{}, err
	}
	row, err := q.UpsertGrant(ctx, db.UpsertGrantParams{
		WorkspaceID: workspaceID, ID: uuid.Must(uuid.NewV7()), RoleID: in.RoleID, UserID: in.UserID, TeamID: in.TeamID,
		ProjectID: in.ProjectID, GrantedBy: actor, CreatedAt: s.now(),
	})
	if isForeignKeyViolation(err) {
		return Grant{}, apperr.Invalid(apperr.Field{Field: "subject", Code: "unknown"})
	}
	if err != nil {
		return Grant{}, err
	}
	err = eventlog.Append(ctx, tx, s.event(workspaceID, actor, "role_grant", row.ID, int64(row.Version), "RoleGrantSet",
		map[string]any{"role_id": row.RoleID, "user_id": row.UserID, "team_id": row.TeamID, "project_id": row.ProjectID}))
	return Grant{ID: row.ID, RoleID: row.RoleID, RoleName: role.Name, UserID: row.UserID, TeamID: row.TeamID, ProjectID: row.ProjectID, CreatedAt: row.CreatedAt}, err
}

// guardLastAdmin refuses to replace the last workspace-scope Admin grant.
func (s *Service) guardLastAdmin(ctx context.Context, q *db.Queries, workspaceID uuid.UUID, in GrantInput, newRole db.Role) error {
	if in.ProjectID.Valid || (newRole.Builtin != nil && *newRole.Builtin == BuiltinAdmin) {
		return nil
	}
	grants, err := q.ListGrants(ctx, workspaceID)
	if err != nil {
		return err
	}
	admin, err := q.RoleByBuiltin(ctx, db.RoleByBuiltinParams{WorkspaceID: workspaceID, Builtin: ptr(BuiltinAdmin)})
	if err != nil {
		return err
	}
	for _, g := range grants {
		if g.RoleID == admin.ID && !g.ProjectID.Valid && g.UserID == in.UserID && g.TeamID == in.TeamID {
			return s.refuseIfLastAdmin(ctx, q, workspaceID)
		}
	}
	return nil
}

func (s *Service) refuseIfLastAdmin(ctx context.Context, q *db.Queries, workspaceID uuid.UUID) error {
	n, err := q.CountAdminWorkspaceGrants(ctx, workspaceID)
	if err != nil {
		return err
	}
	if n <= 1 {
		return apperr.Conflict("last_admin")
	}
	return nil
}

// RemoveGrant takes a role away. The last workspace-scope Admin grant stays.
func (s *Service) RemoveGrant(ctx context.Context, workspaceID uuid.UUID, actor uuid.NullUUID, id uuid.UUID) error {
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		g, err := q.LockGrant(ctx, db.LockGrantParams{WorkspaceID: workspaceID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("grant_not_found")
		}
		if err != nil {
			return err
		}
		role, err := q.RoleInWorkspace(ctx, db.RoleInWorkspaceParams{WorkspaceID: workspaceID, ID: g.RoleID})
		if err != nil {
			return err
		}
		if !g.ProjectID.Valid && role.Builtin != nil && *role.Builtin == BuiltinAdmin {
			if err := s.refuseIfLastAdmin(ctx, q, workspaceID); err != nil {
				return err
			}
		}
		if err := q.DeleteGrant(ctx, db.DeleteGrantParams{WorkspaceID: workspaceID, ID: id}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(workspaceID, actor, "role_grant", id, int64(g.Version)+1, "RoleGrantRemoved",
			map[string]any{"role_id": g.RoleID, "user_id": g.UserID, "team_id": g.TeamID, "project_id": g.ProjectID}))
	})
}

// --- Helpers --------------------------------------------------------------------

func (s *Service) event(workspaceID uuid.UUID, actor uuid.NullUUID, aggregate string, id uuid.UUID, version int64, typ string, payload map[string]any) eventlog.Event {
	if payload == nil {
		payload = map[string]any{}
	}
	return eventlog.Event{
		WorkspaceID: uuid.NullUUID{UUID: workspaceID, Valid: true}, AggregateType: aggregate, AggregateID: id,
		AggregateVersion: version, Type: typ, ActorID: actor, OccurredAt: s.now(), Payload: eventlog.Payload(payload),
	}
}

func keys(perms []Permission) []string {
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = string(p)
	}
	return out
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, x := range a {
		if !slices.Contains(b, x) {
			return false
		}
	}
	return true
}

func ptr[T any](v T) *T { return &v }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
