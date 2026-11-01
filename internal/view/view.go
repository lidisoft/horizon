// Package view owns saved views (docs/domain.md#view): a query
// (internal/query), a layout and display options, personal or shared in a
// project or the workspace.
package view

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/query"
	"github.com/lidisoft/horizon/internal/view/db"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Layouts a view shows its issues in.
var Layouts = []string{"list", "board", "table", "timeline", "calendar"}

const maxDisplay = 16 << 10

// View is a saved view.
type View struct {
	ID        uuid.UUID     `json:"id"`
	ProjectID uuid.NullUUID `json:"project_id" swaggertype:"string"`
	// Shared views are seen by everyone who sees their scope; others by
	// their owner only.
	Shared bool        `json:"shared"`
	Name   string      `json:"name" example:"Open bugs"`
	Layout string      `json:"layout" enums:"list,board,table,timeline,calendar"`
	Query  query.Query `json:"query"`
	// Display holds the layout's options: columns, grouping, swimlanes,
	// board WIP limits…, as the client stores them.
	Display   json.RawMessage `json:"display" swaggertype:"object"`
	CanEdit   bool            `json:"can_edit"`
	CreatedAt time.Time       `json:"created_at"`
	Version   int32           `json:"version"`
}

// Service implements the view commands and queries.
type Service struct {
	pool     *pgxpool.Pool
	access   *access.Service
	projects *project.Service
	now      func() time.Time
}

// New returns a Service.
func New(pool *pgxpool.Pool, acc *access.Service, projects *project.Service) *Service {
	return &Service{pool: pool, access: acc, projects: projects, now: func() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }}
}

func (s *Service) viewFrom(ctx context.Context, wc workspace.Context, v db.View) View {
	out := View{ID: v.ID, ProjectID: v.ProjectID, Shared: !v.OwnerID.Valid, Name: v.Name, Layout: v.Layout, Display: v.Display,
		CreatedAt: v.CreatedAt, Version: v.Version}
	_ = json.Unmarshal(v.Query, &out.Query)
	out.CanEdit = s.canEdit(ctx, wc, v) == nil
	return out
}

func scopeOf(wc workspace.Context, projectID uuid.NullUUID) access.Scope {
	return access.Scope{WorkspaceID: wc.Workspace.ID, ProjectID: projectID}
}

// canEdit: the owner edits a personal view; view.share on its scope a shared one.
func (s *Service) canEdit(ctx context.Context, wc workspace.Context, v db.View) error {
	if v.OwnerID.Valid {
		if v.OwnerID.UUID != wc.Principal.User.ID {
			return apperr.Forbidden("forbidden")
		}
		return nil
	}
	return access.Require(ctx, s.access, wc.Principal, scopeOf(wc, v.ProjectID), access.ViewShare)
}

// visibleProjects are the ids of the projects whose views the caller sees.
func (s *Service) visibleProjects(ctx context.Context, wc workspace.Context) (map[uuid.UUID]project.Project, error) {
	all, err := s.projects.List(ctx, wc)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]project.Project{}
	for _, p := range all {
		out[p.ID] = p
	}
	return out, nil
}

// List returns the views the caller sees: shared ones of the workspace and
// of visible projects, and their own; with projectKey, the workspace's
// and that project's.
func (s *Service) List(ctx context.Context, wc workspace.Context, projectKey string) ([]View, error) {
	visible, err := s.visibleProjects(ctx, wc)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(visible))
	for id, p := range visible {
		if projectKey == "" || strings.EqualFold(p.Key, projectKey) || slices.ContainsFunc(p.Aliases, func(a string) bool { return strings.EqualFold(a, projectKey) }) {
			ids = append(ids, id)
		}
	}
	rows, err := db.New(s.pool).Views(ctx, db.ViewsParams{WorkspaceID: wc.Workspace.ID, ProjectIds: ids, OwnerID: uuid.NullUUID{UUID: wc.Principal.User.ID, Valid: true}})
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(rows))
	for _, r := range rows {
		out = append(out, s.viewFrom(ctx, wc, r))
	}
	return out, nil
}

// load returns a view the caller sees.
func (s *Service) load(ctx context.Context, q *db.Queries, wc workspace.Context, id uuid.UUID, lock bool) (db.View, error) {
	var v db.View
	var err error
	if lock {
		v, err = q.LockView(ctx, db.LockViewParams{WorkspaceID: wc.Workspace.ID, ID: id})
	} else {
		v, err = q.View(ctx, db.ViewParams{WorkspaceID: wc.Workspace.ID, ID: id})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return v, apperr.NotFound("view_not_found")
	}
	if err != nil {
		return v, err
	}
	if v.OwnerID.Valid && v.OwnerID.UUID != wc.Principal.User.ID {
		return v, apperr.NotFound("view_not_found")
	}
	if v.ProjectID.Valid {
		visible, err := s.visibleProjects(ctx, wc)
		if err != nil {
			return v, err
		}
		if _, ok := visible[v.ProjectID.UUID]; !ok {
			return v, apperr.NotFound("view_not_found")
		}
	}
	return v, nil
}

// Get returns one view.
func (s *Service) Get(ctx context.Context, wc workspace.Context, id uuid.UUID) (View, error) {
	v, err := s.load(ctx, db.New(s.pool), wc, id, false)
	if err != nil {
		return View{}, err
	}
	return s.viewFrom(ctx, wc, v), nil
}

// Input creates a view or, with nil fields left alone, changes one.
// Project (a key) applies on creation only.
type Input struct {
	Project string
	Name    *string
	Layout  *string
	Query   *query.Query
	Display json.RawMessage
	Shared  *bool
}

// check validates what is set in in.
func (s *Service) check(ctx context.Context, wc workspace.Context, in Input) error {
	var fields apperr.Fields
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" || len([]rune(n)) > 100 {
			fields.Add("name", "invalid")
		}
		*in.Name = n
	}
	if in.Layout != nil && !slices.Contains(Layouts, *in.Layout) {
		fields.Add("layout", "invalid")
	}
	if in.Display != nil {
		var obj map[string]json.RawMessage
		if len(in.Display) > maxDisplay || json.Unmarshal(in.Display, &obj) != nil {
			fields.Add("display", "invalid")
		}
	}
	if in.Query != nil {
		if err := s.checkQuery(ctx, wc, *in.Query); err != nil {
			var qe *query.Error
			if errors.As(err, &qe) {
				fields.Add("query."+qe.Path, qe.Code)
			} else {
				return err
			}
		}
	}
	return fields.Err()
}

// checkQuery compiles the query against the workspace's fields and
// projects, so a saved view always runs.
func (s *Service) checkQuery(ctx context.Context, wc workspace.Context, q query.Query) error {
	types := map[uuid.UUID]string{}
	rows, err := s.pool.Query(ctx, `SELECT id, type FROM custom_fields WHERE workspace_id = $1`, wc.Workspace.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id uuid.UUID
		var t string
		if err := rows.Scan(&id, &t); err != nil {
			rows.Close()
			return err
		}
		types[id] = t
	}
	rows.Close()
	projects := map[string]uuid.UUID{}
	visible, err := s.visibleProjects(ctx, wc)
	if err != nil {
		return err
	}
	for id, p := range visible {
		projects[p.Key] = id
		for _, a := range p.Aliases {
			projects[a] = id
		}
	}
	_, err = query.Compile(q, query.Context{
		Today: s.now(), Projects: projects,
		CustomFields: func(id uuid.UUID) (string, bool) { t, ok := types[id]; return t, ok },
		IssueID:      func(string) (uuid.UUID, bool) { return uuid.Nil, true },
	}, 1)
	return err
}

func (s *Service) event(wc workspace.Context, v db.View, typ string) eventlog.Event {
	payload := map[string]any{"name": v.Name}
	if v.ProjectID.Valid {
		payload["project_id"] = v.ProjectID.UUID
	}
	if v.OwnerID.Valid {
		payload["owner_id"] = v.OwnerID.UUID // pushed to its owner only
	}
	return eventlog.Event{
		WorkspaceID: uuid.NullUUID{UUID: wc.Workspace.ID, Valid: true}, AggregateType: "view", AggregateID: v.ID,
		AggregateVersion: int64(v.Version), Type: typ, ActorID: uuid.NullUUID{UUID: wc.Principal.User.ID, Valid: true},
		OccurredAt: s.now(), Payload: eventlog.Payload(payload),
	}
}

// Create saves a view: personal (seeing the scope is enough) or shared
// (view.share on the project, or the workspace).
func (s *Service) Create(ctx context.Context, wc workspace.Context, in Input) (View, error) {
	if in.Name == nil || in.Layout == nil {
		return View{}, apperr.Invalid(apperr.Field{Field: "name", Code: "required"})
	}
	if in.Query == nil {
		in.Query = &query.Query{}
	}
	if in.Display == nil {
		in.Display = json.RawMessage(`{}`)
	}
	var projectID uuid.NullUUID
	if in.Project != "" {
		pc, err := s.projects.Enter(ctx, wc, in.Project)
		if err != nil {
			return View{}, apperr.Invalid(apperr.Field{Field: "project", Code: "unknown"})
		}
		projectID = uuid.NullUUID{UUID: pc.Project.ID, Valid: true}
	}
	shared := in.Shared != nil && *in.Shared
	owner := uuid.NullUUID{UUID: wc.Principal.User.ID, Valid: true}
	if shared {
		if err := access.Require(ctx, s.access, wc.Principal, scopeOf(wc, projectID), access.ViewShare); err != nil {
			return View{}, err
		}
		owner = uuid.NullUUID{}
	}
	if err := s.check(ctx, wc, in); err != nil {
		return View{}, err
	}
	raw, _ := json.Marshal(in.Query)
	var created db.View
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		created, err = db.New(tx).CreateView(ctx, db.CreateViewParams{WorkspaceID: wc.Workspace.ID, ID: uuid.Must(uuid.NewV7()), ProjectID: projectID,
			OwnerID: owner, Name: *in.Name, Layout: *in.Layout, Query: raw, Display: in.Display, CreatedAt: s.now()})
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(wc, created, "ViewCreated"))
	})
	if err != nil {
		return View{}, err
	}
	return s.viewFrom(ctx, wc, created), nil
}

// Update changes a view the caller may edit; sharing a personal view (or
// taking a shared one back) needs view.share.
func (s *Service) Update(ctx context.Context, wc workspace.Context, id uuid.UUID, version int32, in Input) (View, error) {
	if err := s.check(ctx, wc, in); err != nil {
		return View{}, err
	}
	var saved db.View
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := s.load(ctx, q, wc, id, true)
		if err != nil {
			return err
		}
		if err := s.canEdit(ctx, wc, cur); err != nil {
			return err
		}
		if version != 0 && cur.Version != version {
			return apperr.Conflict("version_conflict")
		}
		next := cur
		if in.Name != nil {
			next.Name = *in.Name
		}
		if in.Layout != nil {
			next.Layout = *in.Layout
		}
		if in.Query != nil {
			next.Query, _ = json.Marshal(in.Query)
		}
		if in.Display != nil {
			next.Display = in.Display
		}
		if in.Shared != nil && *in.Shared == cur.OwnerID.Valid {
			if err := access.Require(ctx, s.access, wc.Principal, scopeOf(wc, cur.ProjectID), access.ViewShare); err != nil {
				return err
			}
			if *in.Shared {
				next.OwnerID = uuid.NullUUID{}
			} else {
				next.OwnerID = uuid.NullUUID{UUID: wc.Principal.User.ID, Valid: true}
			}
		}
		saved, err = q.SaveView(ctx, db.SaveViewParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID, Name: next.Name, Layout: next.Layout,
			Query: next.Query, Display: next.Display, OwnerID: next.OwnerID, UpdatedAt: s.now()})
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(wc, saved, "ViewUpdated"))
	})
	if err != nil {
		return View{}, err
	}
	return s.viewFrom(ctx, wc, saved), nil
}

// Delete removes a view the caller may edit; a ViewDeleted tombstone remains.
func (s *Service) Delete(ctx context.Context, wc workspace.Context, id uuid.UUID) error {
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := s.load(ctx, q, wc, id, true)
		if err != nil {
			return err
		}
		if err := s.canEdit(ctx, wc, cur); err != nil {
			return err
		}
		if err := q.DeleteView(ctx, db.DeleteViewParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID}); err != nil {
			return err
		}
		cur.Version++
		return eventlog.Tombstone(ctx, tx, s.event(wc, cur, "ViewDeleted"))
	})
}
