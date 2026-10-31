package configdef

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/configdef/db"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
)

// Issues is what configuration needs from the issue module: counting and
// moving issues when statuses disappear or a type changes workflow. Until
// issues exist, NoIssues stands in.
type Issues interface {
	// CountByStatus returns the number of issues in each of the statuses
	// that hold any.
	CountByStatus(ctx context.Context, tx pgx.Tx, statusIDs []uuid.UUID) (map[uuid.UUID]int64, error)
	// CountByType returns the number of issues of an issue type.
	CountByType(ctx context.Context, tx pgx.Tx, typeID uuid.UUID) (int64, error)
	// MoveStatuses moves issues between statuses (from → to), each
	// recording the change in its history.
	MoveStatuses(ctx context.Context, tx pgx.Tx, actor uuid.NullUUID, mapping map[uuid.UUID]uuid.UUID) error
	// CountTypeByStatus returns, for one issue type, the number of issues
	// in each status that holds any.
	CountTypeByStatus(ctx context.Context, tx pgx.Tx, typeID uuid.UUID) (map[uuid.UUID]int64, error)
	// MoveTypeStatuses moves the issues of one type between statuses, when
	// the type changes workflow.
	MoveTypeStatuses(ctx context.Context, tx pgx.Tx, actor uuid.NullUUID, typeID uuid.UUID, mapping map[uuid.UUID]uuid.UUID) error
	// CountByLabel returns the number of issues carrying each label that
	// any carries.
	CountByLabel(ctx context.Context, conn db.DBTX, workspaceID uuid.UUID) (map[uuid.UUID]int64, error)
	// Relabel replaces label from with into on every issue carrying it,
	// each recording the change in its history.
	Relabel(ctx context.Context, tx pgx.Tx, actor uuid.NullUUID, from, into uuid.UUID) error
	// ScrubField removes a deleted custom field's values from issues.
	ScrubField(ctx context.Context, tx pgx.Tx, workspaceID, fieldID uuid.UUID) error
}

// NoIssues reports no issues anywhere.
type NoIssues struct{}

// CountByStatus implements Issues.
func (NoIssues) CountByStatus(context.Context, pgx.Tx, []uuid.UUID) (map[uuid.UUID]int64, error) {
	return map[uuid.UUID]int64{}, nil
}

// CountByType implements Issues.
func (NoIssues) CountByType(context.Context, pgx.Tx, uuid.UUID) (int64, error) { return 0, nil }

// MoveStatuses implements Issues.
func (NoIssues) MoveStatuses(context.Context, pgx.Tx, uuid.NullUUID, map[uuid.UUID]uuid.UUID) error {
	return nil
}

// CountTypeByStatus implements Issues.
func (NoIssues) CountTypeByStatus(context.Context, pgx.Tx, uuid.UUID) (map[uuid.UUID]int64, error) {
	return map[uuid.UUID]int64{}, nil
}

// MoveTypeStatuses implements Issues.
func (NoIssues) MoveTypeStatuses(context.Context, pgx.Tx, uuid.NullUUID, uuid.UUID, map[uuid.UUID]uuid.UUID) error {
	return nil
}

// CountByLabel implements Issues.
func (NoIssues) CountByLabel(context.Context, db.DBTX, uuid.UUID) (map[uuid.UUID]int64, error) {
	return map[uuid.UUID]int64{}, nil
}

// Relabel implements Issues.
func (NoIssues) Relabel(context.Context, pgx.Tx, uuid.NullUUID, uuid.UUID, uuid.UUID) error {
	return nil
}

// ScrubField implements Issues.
func (NoIssues) ScrubField(context.Context, pgx.Tx, uuid.UUID, uuid.UUID) error { return nil }

// Service manages the definitions.
type Service struct {
	pool   *pgxpool.Pool
	access *access.Service
	issues Issues
	now    func() time.Time
}

// New returns a Service. issues may be nil (NoIssues); now defaults to
// time.Now.
func New(pool *pgxpool.Pool, acc *access.Service, issues Issues, now func() time.Time) *Service {
	if issues == nil {
		issues = NoIssues{}
	}
	if now == nil {
		now = time.Now
	}
	return &Service{pool: pool, access: acc, issues: issues, now: func() time.Time { return now().UTC().Truncate(time.Microsecond) }}
}

// SetIssues plugs the issue module in once it exists (it depends on this
// package, so it can't be passed to New).
func (s *Service) SetIssues(issues Issues) { s.issues = issues }

// Target is where a request reads or edits definitions: a workspace, or a
// project of it.
type Target struct {
	Principal   identity.Principal
	WorkspaceID uuid.UUID
	ProjectID   uuid.NullUUID
}

func (t Target) scope() access.Scope {
	return access.Scope{WorkspaceID: t.WorkspaceID, ProjectID: t.ProjectID}
}

// requireManage checks the permission to edit definitions of the target's
// scope: workspace.config.manage, or project.config.manage on the project.
func (s *Service) requireManage(ctx context.Context, t Target) error {
	perm := access.WorkspaceConfigManage
	if t.ProjectID.Valid {
		perm = access.ProjectConfigManage
	}
	return access.Require(ctx, s.access, t.Principal, t.scope(), perm)
}

// CanManage reports whether the caller may edit the target's definitions,
// for the UI.
func (s *Service) CanManage(ctx context.Context, t Target) bool {
	return s.requireManage(ctx, t) == nil
}

// sameScope reports whether a definition belongs to the target's own scope
// (editable there), as opposed to being inherited.
func sameScope(t Target, project uuid.NullUUID) bool { return project == t.ProjectID }

// disabled returns the definitions a project switched off, by type.
func disabled(ctx context.Context, q *db.Queries, t Target) (map[string]map[uuid.UUID]bool, error) {
	out := map[string]map[uuid.UUID]bool{TypeCategory: {}, TypeWorkflow: {}, TypeIssueType: {}, TypeCustomField: {}, TypeLabel: {}}
	if !t.ProjectID.Valid {
		return out, nil
	}
	rows, err := q.Disables(ctx, t.ProjectID.UUID)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if out[r.DefinitionType] == nil {
			out[r.DefinitionType] = map[uuid.UUID]bool{}
		}
		out[r.DefinitionType][r.DefinitionID] = true
	}
	return out, nil
}

func origin(t Target, project uuid.NullUUID, off map[uuid.UUID]bool, id uuid.UUID) Origin {
	inherited := t.ProjectID.Valid && !project.Valid
	return Origin{Inherited: inherited, Disabled: inherited && off[id]}
}

// Resolved is the effective configuration of a scope: what issues there
// follow. Switched-off inherited definitions are left out.
type Resolved struct {
	Categories map[uuid.UUID]Category
	Workflows  map[uuid.UUID]Workflow
	IssueTypes []IssueType
	Fields     map[uuid.UUID]Field
	// TypeFields lists the fields of each issue type, in order.
	TypeFields map[uuid.UUID][]TypeField
	Labels     map[uuid.UUID]Label
	Estimation Estimation
}

// Status finds a status among the resolved workflows.
func (r *Resolved) Status(id uuid.UUID) (Status, Workflow, bool) {
	for _, w := range r.Workflows {
		for _, st := range w.Statuses {
			if st.ID == id {
				return st, w, true
			}
		}
	}
	return Status{}, Workflow{}, false
}

// Resolve returns the effective configuration of a workspace, or of one of
// its projects (docs/configuration.md#resolution). It reads through conn,
// so commands can resolve inside their transaction.
func (s *Service) Resolve(ctx context.Context, conn db.DBTX, workspaceID uuid.UUID, projectID uuid.NullUUID) (*Resolved, error) {
	q := db.New(conn)
	t := Target{WorkspaceID: workspaceID, ProjectID: projectID}
	off, err := disabled(ctx, q, t)
	if err != nil {
		return nil, err
	}
	r := &Resolved{Categories: map[uuid.UUID]Category{}, Workflows: map[uuid.UUID]Workflow{}}
	cats, err := q.Categories(ctx, db.CategoriesParams{WorkspaceID: workspaceID, ProjectID: projectID})
	if err != nil {
		return nil, err
	}
	for _, c := range cats {
		if !off[TypeCategory][c.ID] {
			r.Categories[c.ID] = categoryFrom(c)
		}
	}
	wfs, err := s.workflows(ctx, q, t, off)
	if err != nil {
		return nil, err
	}
	for _, w := range wfs {
		if !w.Disabled {
			r.Workflows[w.ID] = w
		}
	}
	types, err := q.IssueTypes(ctx, db.IssueTypesParams{WorkspaceID: workspaceID, ProjectID: projectID})
	if err != nil {
		return nil, err
	}
	for _, it := range types {
		if !off[TypeIssueType][it.ID] {
			r.IssueTypes = append(r.IssueTypes, issueTypeFrom(it))
		}
	}
	fields, attached, err := s.fields(ctx, q, t, off)
	if err != nil {
		return nil, err
	}
	r.Fields = map[uuid.UUID]Field{}
	for _, f := range fields {
		if !f.Disabled {
			r.Fields[f.ID] = f
		}
	}
	r.TypeFields = map[uuid.UUID][]TypeField{}
	for _, a := range attached {
		if _, ok := r.Fields[a.FieldID]; ok {
			r.TypeFields[a.IssueTypeID] = append(r.TypeFields[a.IssueTypeID], TypeField{FieldID: a.FieldID, Required: a.Required})
		}
	}
	labels, err := q.Labels(ctx, db.LabelsParams{WorkspaceID: workspaceID, ProjectID: projectID})
	if err != nil {
		return nil, err
	}
	r.Labels = map[uuid.UUID]Label{}
	for _, l := range labels {
		if !off[TypeLabel][l.ID] {
			r.Labels[l.ID] = labelFrom(l)
		}
	}
	if r.Estimation, err = estimation(ctx, q, t); err != nil {
		return nil, err
	}
	return r, nil
}

// Disable switches an inherited definition off in the target project, or
// back on.
func (s *Service) SetDisabled(ctx context.Context, t Target, defType string, id uuid.UUID, off bool) error {
	if !t.ProjectID.Valid {
		return errors.New("configdef: switching definitions off needs a project")
	}
	if err := s.requireManage(ctx, t); err != nil {
		return err
	}
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		if err := s.checkInherited(ctx, q, t, defType, id); err != nil {
			return err
		}
		if !off {
			return q.Enable(ctx, db.EnableParams{ProjectID: t.ProjectID.UUID, DefinitionType: defType, DefinitionID: id})
		}
		if err := s.checkCanDisable(ctx, tx, t, defType, id); err != nil {
			return err
		}
		if err := q.Disable(ctx, db.DisableParams{WorkspaceID: t.WorkspaceID, ProjectID: t.ProjectID.UUID, DefinitionType: defType, DefinitionID: id}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, "project", t.ProjectID.UUID, 0, "ProjectDefinitionDisabled",
			map[string]any{"definition_type": defType, "definition_id": id}))
	})
}

// checkInherited refuses switching off what isn't a workspace definition.
func (s *Service) checkInherited(ctx context.Context, q *db.Queries, t Target, defType string, id uuid.UUID) error {
	var project uuid.NullUUID
	switch defType {
	case TypeCategory:
		c, err := q.LockCategory(ctx, db.LockCategoryParams{WorkspaceID: t.WorkspaceID, ID: id})
		if err != nil {
			return notFound(err, "category_not_found")
		}
		project = c.ProjectID
	case TypeWorkflow:
		w, err := q.Workflow(ctx, db.WorkflowParams{WorkspaceID: t.WorkspaceID, ID: id})
		if err != nil {
			return notFound(err, "workflow_not_found")
		}
		project = w.ProjectID
	case TypeIssueType:
		it, err := q.LockIssueType(ctx, db.LockIssueTypeParams{WorkspaceID: t.WorkspaceID, ID: id})
		if err != nil {
			return notFound(err, "issue_type_not_found")
		}
		project = it.ProjectID
	case TypeCustomField:
		f, err := q.LockField(ctx, db.LockFieldParams{WorkspaceID: t.WorkspaceID, ID: id})
		if err != nil {
			return notFound(err, "field_not_found")
		}
		project = f.ProjectID
	case TypeLabel:
		l, err := q.LockLabel(ctx, db.LockLabelParams{WorkspaceID: t.WorkspaceID, ID: id})
		if err != nil {
			return notFound(err, "label_not_found")
		}
		project = l.ProjectID
	default:
		return errors.New("configdef: unknown definition type " + defType)
	}
	if project.Valid {
		return conflict("not_inherited")
	}
	return nil
}

// checkCanDisable keeps the project's configuration consistent: a category
// can't go while an effective workflow uses it, nor a workflow while an
// effective issue type is bound to it.
func (s *Service) checkCanDisable(ctx context.Context, tx pgx.Tx, t Target, defType string, id uuid.UUID) error {
	r, err := s.Resolve(ctx, tx, t.WorkspaceID, t.ProjectID)
	if err != nil {
		return err
	}
	switch defType {
	case TypeCategory:
		for _, w := range r.Workflows {
			for _, st := range w.Statuses {
				if st.CategoryID == id {
					return conflict("category_in_use")
				}
			}
		}
	case TypeWorkflow:
		for _, it := range r.IssueTypes {
			if it.WorkflowID == id {
				return conflict("workflow_in_use")
			}
		}
	}
	return nil
}

func (s *Service) event(t Target, aggregate string, id uuid.UUID, version int64, typ string, payload map[string]any) eventlog.Event {
	if payload == nil {
		payload = map[string]any{}
	}
	if t.ProjectID.Valid {
		payload["project_id"] = t.ProjectID.UUID
	}
	if version < 1 {
		version = 1
	}
	return eventlog.Event{
		WorkspaceID: uuid.NullUUID{UUID: t.WorkspaceID, Valid: true}, AggregateType: aggregate, AggregateID: id,
		AggregateVersion: version, Type: typ, ActorID: actor(t), OccurredAt: s.now(), Payload: eventlog.Payload(payload),
	}
}

func actor(t Target) uuid.NullUUID {
	return uuid.NullUUID{UUID: t.Principal.User.ID, Valid: t.Principal.User.ID != uuid.Nil}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
