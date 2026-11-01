// Package project owns projects: keys and their aliases, visibility, lead,
// archiving, and the issue sequence (docs/domain.md#project).
package project

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/project/db"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Visibility values.
const (
	VisibilityWorkspace = "workspace" // every member with project.view
	VisibilityPrivate   = "private"   // only people and teams with a grant on it
)

// Colours a project can take; the UI maps them to design tokens.
var Colours = []string{"blue", "green", "teal", "purple", "pink", "coral", "orange", "yellow", "grey"}

// keyRe: 2–10 upper-case letters or digits, starting with a letter.
var keyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)

// Project is the public view of a project.
type Project struct {
	ID          uuid.UUID     `json:"id"`
	Key         string        `json:"key" example:"HZN"`
	Name        string        `json:"name" example:"Horizon"`
	Description string        `json:"description"`
	Colour      string        `json:"colour" example:"blue"`
	LeadID      uuid.NullUUID `json:"lead_id" swaggertype:"string"`
	Visibility  string        `json:"visibility" enums:"workspace,private"`
	Aliases     []string      `json:"aliases"`
	ArchivedAt  *time.Time    `json:"archived_at"`
	CreatedAt   time.Time     `json:"created_at"`
	Version     int32         `json:"version"`
}

func projectFrom(p db.Project, aliases []string) Project {
	if aliases == nil {
		aliases = []string{}
	}
	return Project{
		ID: p.ID, Key: p.Key, Name: p.Name, Description: p.Description, Colour: p.Colour, LeadID: p.LeadID,
		Visibility: p.Visibility, Aliases: aliases, ArchivedAt: p.ArchivedAt, CreatedAt: p.CreatedAt, Version: p.Version,
	}
}

// Service implements the project commands and queries.
type Service struct {
	pool     *pgxpool.Pool
	access   *access.Service
	now      func() time.Time
	onDelete []DeleteHook
}

// DeleteHook removes, in tx, what another module keeps for a project being
// deleted, and scrubs its events (eventlog.Scrub). Rows that reference the
// project cascade anyway; hooks exist for the history.
type DeleteHook func(ctx context.Context, tx pgx.Tx, pc Context) error

// OnDelete registers a hook run by Delete.
func (s *Service) OnDelete(h DeleteHook) { s.onDelete = append(s.onDelete, h) }

// New returns a Service; now defaults to time.Now.
func New(pool *pgxpool.Pool, acc *access.Service, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{pool: pool, access: acc, now: func() time.Time { return now().UTC().Truncate(time.Microsecond) }}
}

// Context is a project as seen by the caller of a request.
type Context struct {
	workspace.Context
	Project Project
}

// Scope is the project's access scope.
func (pc Context) Scope() access.Scope {
	return access.Scope{WorkspaceID: pc.Workspace.ID, ProjectID: uuid.NullUUID{UUID: pc.Project.ID, Valid: true}}
}

// Require checks a permission on the project.
func (s *Service) Require(ctx context.Context, pc Context, perm access.Permission) error {
	return access.Require(ctx, s.access, pc.Principal, pc.Scope(), perm)
}

// visible decides whether the caller sees a project: project.view on it,
// and for a private project a grant on it, or project.manage across the
// workspace (admins administer private projects too).
type visible struct {
	s          *Service
	wc         workspace.Context
	wsPerms    []access.Permission
	privateOK  map[uuid.UUID]bool
	instanceOK bool
}

func (s *Service) visibility(ctx context.Context, wc workspace.Context) (*visible, error) {
	v := &visible{s: s, wc: wc, instanceOK: wc.Principal.User.IsInstanceAdmin, privateOK: map[uuid.UUID]bool{}}
	if v.instanceOK {
		return v, nil
	}
	var err error
	if v.wsPerms, err = s.access.Permissions(ctx, wc.Principal, wc.Scope()); err != nil {
		return nil, err
	}
	ids, err := db.New(s.pool).ProjectGrantProjects(ctx, db.ProjectGrantProjectsParams{WorkspaceID: wc.Workspace.ID, UserID: wc.Principal.User.ID})
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		v.privateOK[id] = true
	}
	return v, nil
}

func (v *visible) sees(ctx context.Context, p db.Project) (bool, error) {
	if v.instanceOK {
		return true, nil
	}
	granted := v.privateOK[p.ID]
	if p.Visibility == VisibilityPrivate && !granted && !slices.Contains(v.wsPerms, access.ProjectManage) {
		return false, nil
	}
	if slices.Contains(v.wsPerms, access.ProjectView) {
		return true, nil
	}
	if !granted {
		return false, nil
	}
	return v.s.access.Can(ctx, v.wc.Principal, access.Scope{WorkspaceID: p.WorkspaceID, ProjectID: uuid.NullUUID{UUID: p.ID, Valid: true}}, access.ProjectView)
}

// List returns the projects the caller sees, active first.
func (s *Service) List(ctx context.Context, wc workspace.Context) ([]Project, error) {
	v, err := s.visibility(ctx, wc)
	if err != nil {
		return nil, err
	}
	q := db.New(s.pool)
	rows, err := q.ListProjects(ctx, wc.Workspace.ID)
	if err != nil {
		return nil, err
	}
	aliases, err := s.aliases(ctx, q, wc.Workspace.ID)
	if err != nil {
		return nil, err
	}
	out := []Project{}
	for _, r := range rows {
		ok, err := v.sees(ctx, r)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, projectFrom(r, aliases[r.ID]))
		}
	}
	return out, nil
}

func (s *Service) aliases(ctx context.Context, q *db.Queries, workspaceID uuid.UUID) (map[uuid.UUID][]string, error) {
	rows, err := q.ProjectAliases(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID][]string{}
	for _, r := range rows {
		out[r.ProjectID] = append(out[r.ProjectID], r.OldKey)
	}
	return out, nil
}

// Enter resolves a project by its key or a former one. Projects the caller
// can't see are not found.
func (s *Service) Enter(ctx context.Context, wc workspace.Context, key string) (Context, error) {
	q := db.New(s.pool)
	p, err := q.ProjectByKey(ctx, db.ProjectByKeyParams{WorkspaceID: wc.Workspace.ID, Key: strings.ToUpper(key)})
	if errors.Is(err, pgx.ErrNoRows) {
		return Context{}, apperr.NotFound("project_not_found")
	}
	if err != nil {
		return Context{}, err
	}
	v, err := s.visibility(ctx, wc)
	if err != nil {
		return Context{}, err
	}
	if ok, err := v.sees(ctx, p); err != nil || !ok {
		if err == nil {
			err = apperr.NotFound("project_not_found")
		}
		return Context{}, err
	}
	aliases, err := s.aliases(ctx, q, wc.Workspace.ID)
	if err != nil {
		return Context{}, err
	}
	return Context{Context: wc, Project: projectFrom(p, aliases[p.ID])}, nil
}

// Input is the input of Create and Update; nil fields are left unchanged on
// update. Empty LeadID clears the lead.
type Input struct {
	Key         *string
	Name        *string
	Description *string
	Colour      *string
	LeadID      *uuid.UUID
	Visibility  *string
}

type values struct {
	key, name, description, colour, visibility string
	lead                                       uuid.NullUUID
}

func (s *Service) validate(ctx context.Context, q *db.Queries, workspaceID uuid.UUID, in Input, v values, creating bool) (values, []string, error) {
	var fields apperr.Fields
	var changed []string
	if in.Key != nil || creating {
		key := ""
		if in.Key != nil {
			key = strings.ToUpper(strings.TrimSpace(*in.Key))
		}
		if !keyRe.MatchString(key) {
			fields.Add("key", "invalid")
		} else if key != v.key {
			v.key = key
			changed = append(changed, "key")
		}
	}
	if in.Name != nil || creating {
		name := ""
		if in.Name != nil {
			name = strings.TrimSpace(*in.Name)
		}
		switch {
		case name == "":
			fields.Add("name", "required")
		case len([]rune(name)) > 80:
			fields.Add("name", "too_long")
		case name != v.name:
			v.name = name
			changed = append(changed, "name")
		}
	}
	if in.Description != nil {
		d := strings.TrimSpace(*in.Description)
		if len([]rune(d)) > 2000 {
			fields.Add("description", "too_long")
		} else if d != v.description {
			v.description = d
			changed = append(changed, "description")
		}
	}
	if in.Colour != nil && *in.Colour != v.colour {
		if !slices.Contains(Colours, *in.Colour) {
			fields.Add("colour", "invalid")
		}
		v.colour = *in.Colour
		changed = append(changed, "colour")
	}
	if in.Visibility != nil && *in.Visibility != v.visibility {
		if *in.Visibility != VisibilityWorkspace && *in.Visibility != VisibilityPrivate {
			fields.Add("visibility", "invalid")
		}
		v.visibility = *in.Visibility
		changed = append(changed, "visibility")
	}
	if in.LeadID != nil {
		lead := uuid.NullUUID{UUID: *in.LeadID, Valid: *in.LeadID != uuid.Nil}
		if lead != v.lead {
			if lead.Valid {
				ok, err := q.IsMember(ctx, db.IsMemberParams{WorkspaceID: workspaceID, UserID: lead.UUID})
				if err != nil {
					return v, nil, err
				}
				if !ok {
					fields.Add("lead_id", "not_member")
				}
			}
			v.lead = lead
			changed = append(changed, "lead")
		}
	}
	return v, changed, fields.Err()
}

// Create adds a project (project.create). The creator becomes its lead
// unless one is given.
func (s *Service) Create(ctx context.Context, wc workspace.Context, in Input) (Project, error) {
	if err := access.Require(ctx, s.access, wc.Principal, wc.Scope(), access.ProjectCreate); err != nil {
		return Project{}, err
	}
	var created db.Project
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		if in.LeadID == nil && wc.Member {
			id := wc.Principal.User.ID
			in.LeadID = &id
		}
		v, _, err := s.validate(ctx, q, wc.Workspace.ID, in, values{colour: "blue", visibility: VisibilityWorkspace}, true)
		if err != nil {
			return err
		}
		if err := s.claimKey(ctx, q, wc.Workspace.ID, uuid.Nil, v.key); err != nil {
			return err
		}
		created, err = q.CreateProject(ctx, db.CreateProjectParams{
			WorkspaceID: wc.Workspace.ID, ID: uuid.Must(uuid.NewV7()), Key: v.key, Name: v.name, Description: v.description,
			Colour: v.colour, LeadID: v.lead, Visibility: v.visibility, CreatedAt: s.now(),
		})
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(wc, created.ID, int64(created.Version), "ProjectCreated",
			map[string]any{"key": created.Key, "visibility": created.Visibility}))
	})
	if err != nil {
		return Project{}, err
	}
	return projectFrom(created, nil), nil
}

// claimKey locks the workspace's key namespace and refuses a key used by
// another project, now or formerly.
func (s *Service) claimKey(ctx context.Context, q *db.Queries, workspaceID, projectID uuid.UUID, key string) error {
	if err := q.LockWorkspaceKeys(ctx, workspaceID); err != nil {
		return err
	}
	taken, err := q.KeyTaken(ctx, db.KeyTakenParams{WorkspaceID: workspaceID, Key: key, OtherThan: projectID})
	if err != nil {
		return err
	}
	if taken {
		return apperr.Invalid(apperr.Field{Field: "key", Code: "taken"})
	}
	return nil
}

// Update edits the project (project.manage). A new key keeps the old one as
// an alias; taking a former key back drops that alias.
func (s *Service) Update(ctx context.Context, pc Context, version int32, in Input) (Project, error) {
	if err := s.Require(ctx, pc, access.ProjectManage); err != nil {
		return Project{}, err
	}
	var updated db.Project
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockProject(ctx, db.LockProjectParams{WorkspaceID: pc.Workspace.ID, ID: pc.Project.ID})
		if err != nil {
			return err
		}
		if version != 0 && cur.Version != version {
			return apperr.Conflict("version_conflict")
		}
		v, changed, err := s.validate(ctx, q, cur.WorkspaceID, in, values{
			key: cur.Key, name: cur.Name, description: cur.Description, colour: cur.Colour, visibility: cur.Visibility, lead: cur.LeadID,
		}, false)
		if err != nil {
			return err
		}
		if len(changed) == 0 {
			updated = cur
			return nil
		}
		if v.key != cur.Key {
			if err := s.claimKey(ctx, q, cur.WorkspaceID, cur.ID, v.key); err != nil {
				return err
			}
			if err := q.DropKeyAlias(ctx, db.DropKeyAliasParams{WorkspaceID: cur.WorkspaceID, ProjectID: cur.ID, OldKey: v.key}); err != nil {
				return err
			}
			if err := q.AddKeyAlias(ctx, db.AddKeyAliasParams{WorkspaceID: cur.WorkspaceID, ProjectID: cur.ID, OldKey: cur.Key, CreatedAt: s.now()}); err != nil {
				return err
			}
		}
		updated, err = q.UpdateProject(ctx, db.UpdateProjectParams{
			WorkspaceID: cur.WorkspaceID, ID: cur.ID, Key: v.key, Name: v.name, Description: v.description,
			Colour: v.colour, LeadID: v.lead, Visibility: v.visibility, UpdatedAt: s.now(),
		})
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "key", Code: "taken"})
		}
		if err != nil {
			return err
		}
		payload := map[string]any{"fields": changed}
		if v.key != cur.Key {
			payload["key"], payload["old_key"] = v.key, cur.Key
		}
		return eventlog.Append(ctx, tx, s.event(pc.Context, cur.ID, int64(updated.Version), "ProjectUpdated", payload))
	})
	if err != nil {
		return Project{}, err
	}
	aliases, err := s.aliases(ctx, db.New(s.pool), pc.Workspace.ID)
	if err != nil {
		return Project{}, err
	}
	return projectFrom(updated, aliases[updated.ID]), nil
}

// SetArchived archives (read-only, out of the sidebar) or restores a
// project (project.manage).
func (s *Service) SetArchived(ctx context.Context, pc Context, archived bool) (Project, error) {
	if err := s.Require(ctx, pc, access.ProjectManage); err != nil {
		return Project{}, err
	}
	var updated db.Project
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockProject(ctx, db.LockProjectParams{WorkspaceID: pc.Workspace.ID, ID: pc.Project.ID})
		if err != nil {
			return err
		}
		if (cur.ArchivedAt != nil) == archived {
			updated = cur
			return nil
		}
		var at *time.Time
		typ := "ProjectUnarchived"
		if archived {
			now := s.now()
			at, typ = &now, "ProjectArchived"
		}
		updated, err = q.SetProjectArchived(ctx, db.SetProjectArchivedParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID, ArchivedAt: at, UpdatedAt: s.now()})
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(pc.Context, cur.ID, int64(updated.Version), typ, nil))
	})
	if err != nil {
		return Project{}, err
	}
	return projectFrom(updated, pc.Project.Aliases), nil
}

// Delete hard-deletes an archived project (project.manage) with its issues,
// configuration overrides and grants; confirm must be its current key. One
// tombstone remains: ProjectDeleted (docs/domain.md#deletion).
func (s *Service) Delete(ctx context.Context, pc Context, confirm string) error {
	if err := s.Require(ctx, pc, access.ProjectManage); err != nil {
		return err
	}
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockProject(ctx, db.LockProjectParams{WorkspaceID: pc.Workspace.ID, ID: pc.Project.ID})
		if err != nil {
			return err
		}
		if cur.ArchivedAt == nil {
			return apperr.Conflict("project_not_archived")
		}
		if confirm != cur.Key {
			return apperr.Invalid(apperr.Field{Field: "confirm", Code: "mismatch"})
		}
		for _, h := range s.onDelete {
			if err := h(ctx, tx, pc); err != nil {
				return err
			}
		}
		if err := s.access.ScrubProjectGrants(ctx, tx, cur.WorkspaceID, cur.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM projects WHERE workspace_id = $1 AND id = $2`, cur.WorkspaceID, cur.ID); err != nil {
			return err
		}
		return eventlog.Tombstone(ctx, tx, s.event(pc.Context, cur.ID, int64(cur.Version)+1, "ProjectDeleted", map[string]any{"key": cur.Key}))
	})
}

func (s *Service) event(wc workspace.Context, id uuid.UUID, version int64, typ string, payload map[string]any) eventlog.Event {
	if payload == nil {
		payload = map[string]any{}
	}
	return eventlog.Event{
		WorkspaceID:   uuid.NullUUID{UUID: wc.Workspace.ID, Valid: true},
		AggregateType: "project", AggregateID: id, AggregateVersion: version, Type: typ,
		ActorID:    uuid.NullUUID{UUID: wc.Principal.User.ID, Valid: wc.Principal.User.ID != uuid.Nil},
		OccurredAt: s.now(), Payload: eventlog.Payload(payload),
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
