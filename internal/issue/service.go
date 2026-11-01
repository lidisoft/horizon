package issue

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/document"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue/db"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Limits of issue content.
const (
	maxTitle       = 255
	maxDescription = 1 << 20 // plain text given for a description or comment
	maxLabels      = 50
	pageSize       = 100
	maxPageSize    = 500
)

// Service implements the issue commands and queries.
type Service struct {
	pool     *pgxpool.Pool
	access   *access.Service
	config   *configdef.Service
	projects *project.Service
	storage  Storage
	now      func() time.Time

	leaveIteration IterationHook
	docSteps       func(DocSteps)
}

// New returns a Service and plugs it into config as its Issues, so
// publishing workflows and merging labels move real issues.
func New(pool *pgxpool.Pool, acc *access.Service, config *configdef.Service, projects *project.Service, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	s := &Service{pool: pool, access: acc, config: config, projects: projects, now: func() time.Time { return now().UTC().Truncate(time.Microsecond) }}
	config.SetIssues(s)
	projects.OnDelete(s.deleteProject)
	return s
}

// Context is an issue as seen by the caller of a request: its project's
// context, and the issue as loaded (not locked).
type Context struct {
	project.Context
	Issue db.Issue
}

// Scope is the issue's access scope: its project.
func (ic Context) Scope() access.Scope { return ic.Context.Scope() }

func (ic Context) key() string { return keyOf(ic.Project.Key, ic.Issue.Number) }

// KeyOf formats an issue key: KeyOf("HZN", 42) is "HZN-42".
func KeyOf(projectKey string, n int64) string { return keyOf(projectKey, n) }

func keyOf(projectKey string, n int64) string {
	return projectKey + "-" + strconv.FormatInt(n, 10)
}

// Enter resolves an issue key (current or former project key) for the
// caller; issues of projects they can't see are not found.
func (s *Service) Enter(ctx context.Context, wc workspace.Context, key string) (Context, error) {
	projectKey, n, ok := ParseKey(key)
	if !ok {
		return Context{}, apperr.NotFound("issue_not_found")
	}
	pc, err := s.projects.Enter(ctx, wc, projectKey)
	if apperr.Is(err, "project_not_found") {
		return Context{}, apperr.NotFound("issue_not_found")
	}
	if err != nil {
		return Context{}, err
	}
	q := db.New(s.pool)
	i, err := q.IssueByNumber(ctx, db.IssueByNumberParams{WorkspaceID: wc.Workspace.ID, ProjectID: pc.Project.ID, Number: n})
	if errors.Is(err, pgx.ErrNoRows) {
		// A key the issue left behind when it moved: enter it by its
		// current key, in its current project.
		moved, err := q.IssueByAlias(ctx, db.IssueByAliasParams{WorkspaceID: wc.Workspace.ID, ProjectID: pc.Project.ID, Number: n})
		if errors.Is(err, pgx.ErrNoRows) {
			return Context{}, apperr.NotFound("issue_not_found")
		}
		if err != nil {
			return Context{}, err
		}
		return s.enterByID(ctx, wc, moved)
	}
	if err != nil {
		return Context{}, err
	}
	if err := access.Require(ctx, s.access, pc.Principal, pc.Scope(), access.IssueView); err != nil {
		return Context{}, apperr.NotFound("issue_not_found")
	}
	return Context{Context: pc, Issue: i}, nil
}

// Get returns the issue with its labels and description document.
func (s *Service) Get(ctx context.Context, ic Context) (Issue, error) {
	labels, err := s.labelsOf(ctx, s.pool, []uuid.UUID{ic.Issue.ID})
	if err != nil {
		return Issue{}, err
	}
	i := issueFrom(ic.Issue, ic.Project.Key, labels[ic.Issue.ID])
	doc, _, err := descriptionOf(ctx, s.pool, ic.Issue)
	if err != nil {
		return Issue{}, err
	}
	i.DescriptionDoc = docJSON(doc)
	if ic.Issue.ParentID.Valid {
		parent, key, ok, err := s.visibleIssue(ctx, ic, ic.Issue.ParentID.UUID)
		if err != nil {
			return Issue{}, err
		}
		if ok {
			i.Parent = &Ref{ID: parent.ID, Key: key, Name: parent.Title}
		}
	}
	return i, nil
}

func (s *Service) labelsOf(ctx context.Context, conn db.DBTX, ids []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	rows, err := db.New(conn).IssueLabels(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID][]uuid.UUID{}
	for _, r := range rows {
		out[r.IssueID] = append(out[r.IssueID], r.LabelID)
	}
	return out, nil
}

// Page is a page of issues; Next is the cursor of the following page.
type Page struct {
	Items []Issue `json:"items"`
	Next  string  `json:"next_cursor,omitempty"`
}

// List returns a project's issues in rank order, a page at a time (cursor
// is the last rank of the previous page).
func (s *Service) List(ctx context.Context, pc project.Context, cursor string, limit int, archived bool) (Page, error) {
	if err := access.Require(ctx, s.access, pc.Principal, pc.Scope(), access.IssueView); err != nil {
		return Page{}, err
	}
	if limit <= 0 {
		limit = pageSize
	}
	limit = min(limit, maxPageSize)
	rows, err := db.New(s.pool).ProjectIssues(ctx, db.ProjectIssuesParams{
		WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID, Archived: archived, After: cursor, PageSize: int32(limit + 1),
	})
	if err != nil {
		return Page{}, err
	}
	p := Page{Items: []Issue{}}
	if len(rows) > limit {
		rows = rows[:limit]
		p.Next = rows[limit-1].Rank
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	labels, err := s.labelsOf(ctx, s.pool, ids)
	if err != nil {
		return Page{}, err
	}
	for _, r := range rows {
		p.Items = append(p.Items, issueFrom(r, pc.Project.Key, labels[r.ID]))
	}
	return p, nil
}

// --- Creating ---------------------------------------------------------------------

// NewIssue is the input of Create.
type NewIssue struct {
	TypeID      uuid.UUID
	Title       string
	Description Text
	Priority    string
	AssigneeID  uuid.NullUUID
	Estimate    *string
	ParentKey   string
	Labels      []uuid.UUID
	StartDate   string
	DueDate     string
	Fields      map[string]json.RawMessage
}

// Create adds an issue to a project (issue.create): in the initial status of
// its type's workflow, at the end of the project's order. Required custom
// fields of the type must be set. Assigning on creation needs issue.assign,
// estimating issue.estimate.
func (s *Service) Create(ctx context.Context, pc project.Context, in NewIssue) (Issue, error) {
	if err := s.require(ctx, pc, access.IssueCreate); err != nil {
		return Issue{}, err
	}
	if pc.Project.ArchivedAt != nil {
		return Issue{}, apperr.Conflict("project_archived")
	}
	if in.AssigneeID.Valid {
		if err := s.require(ctx, pc, access.IssueAssign); err != nil {
			return Issue{}, err
		}
	}
	if in.Estimate != nil {
		if err := s.require(ctx, pc, access.IssueEstimate); err != nil {
			return Issue{}, err
		}
	}
	var created db.Issue
	var labels []uuid.UUID
	var createdDoc *document.Node
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		r, err := s.config.Resolve(ctx, tx, pc.Workspace.ID, uuid.NullUUID{UUID: pc.Project.ID, Valid: true})
		if err != nil {
			return err
		}
		var fields apperr.Fields
		it, ok := findType(r, in.TypeID)
		if !ok {
			fields.Add("type_id", "unknown")
			return fields.Err()
		}
		wf := r.Workflows[it.WorkflowID]
		initial := wf.Statuses[0]
		for _, st := range wf.Statuses {
			if st.Initial {
				initial = st
			}
		}
		next := db.Issue{
			WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID, TypeID: it.ID, StatusID: initial.ID,
			Priority: "none", Fields: json.RawMessage(`{}`), ReporterID: uuid.NullUUID{UUID: pc.Principal.User.ID, Valid: true},
		}
		if Started(r, initial.ID) {
			now := s.now()
			next.StartedAt = &now
		}
		next.Title = validTitle(&fields, in.Title)
		description := in.Description.parse(&fields, "description")
		if description != nil {
			next.Description = document.PlainText(description)
		}
		if in.Priority != "" {
			next.Priority = in.Priority
			if !slices.Contains(Priorities, in.Priority) {
				fields.Add("priority", "invalid")
			}
		}
		if in.AssigneeID.Valid {
			s.checkAssignee(ctx, tx, &fields, pc.Workspace.ID, in.AssigneeID.UUID)
			next.AssigneeID = in.AssigneeID
		}
		if in.Estimate != nil {
			next.Estimate = validEstimate(&fields, r, *in.Estimate)
		}
		var ok1, ok2 bool
		if next.StartDate, ok1 = parseDate(in.StartDate); !ok1 {
			fields.Add("start_date", "invalid")
		}
		if next.DueDate, ok2 = parseDate(in.DueDate); !ok2 {
			fields.Add("due_date", "invalid")
		}
		checkDates(&fields, next.StartDate, next.DueDate)
		values, err := s.validateFields(ctx, tx, &fields, r, it.ID, json.RawMessage(`{}`), in.Fields, true)
		if err != nil {
			return err
		}
		next.Fields = values
		labels = validLabels(&fields, r, in.Labels)
		if in.ParentKey != "" {
			parent, err := s.findParent(ctx, tx, pc.Context, in.ParentKey)
			if err != nil {
				fields.Add("parent", "unknown")
			} else {
				next.ParentID = uuid.NullUUID{UUID: parent.ID, Valid: true}
				checkParentRule(&fields, r, it, &parent)
			}
		} else {
			checkParentRule(&fields, r, it, nil)
		}
		if err := fields.Err(); err != nil {
			return err
		}

		n, err := q.NextIssueNumber(ctx, db.NextIssueNumberParams{WorkspaceID: pc.Workspace.ID, ID: pc.Project.ID})
		if err != nil {
			return err
		}
		last, err := q.LastRank(ctx, db.LastRankParams{WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID})
		if err != nil {
			return err
		}
		rank, err := Between(last, "")
		if err != nil {
			return err
		}
		created, err = q.CreateIssue(ctx, db.CreateIssueParams{
			WorkspaceID: next.WorkspaceID, ID: uuid.Must(uuid.NewV7()), ProjectID: next.ProjectID, Number: n, TypeID: next.TypeID,
			Title: next.Title, Description: next.Description, StatusID: next.StatusID, Priority: next.Priority, Rank: rank,
			Estimate: next.Estimate, AssigneeID: next.AssigneeID, ReporterID: next.ReporterID, ParentID: next.ParentID,
			StartDate: next.StartDate, DueDate: next.DueDate, Fields: next.Fields, StartedAt: next.StartedAt, CreatedAt: s.now(),
		})
		if err != nil {
			return err
		}
		for _, l := range labels {
			if err := q.AddIssueLabel(ctx, db.AddIssueLabelParams{WorkspaceID: pc.Workspace.ID, IssueID: created.ID, LabelID: l}); err != nil {
				return err
			}
		}
		// A new issue has no attachments yet: its description takes no images.
		if len(document.Attachments(description)) > 0 {
			return apperr.Invalid(apperr.Field{Field: "description_doc", Code: "unknown_attachment"})
		}
		if in.Description.IsSet() {
			if _, err := q.CreateDescription(ctx, db.CreateDescriptionParams{WorkspaceID: created.WorkspaceID, ID: uuid.Must(uuid.NewV7()),
				IssueID: created.ID, Content: docJSON(description), UpdatedAt: s.now()}); err != nil {
				return err
			}
		}
		createdDoc = description
		return eventlog.Append(ctx, tx, s.event(pc.Context, created, "IssueCreated", map[string]any{
			"project_id": created.ProjectID, "key": keyOf(pc.Project.Key, created.Number), "type_id": created.TypeID,
			"status_id": created.StatusID, "parent_id": created.ParentID, "assignee_id": created.AssigneeID,
		}))
	})
	if err != nil {
		return Issue{}, err
	}
	out := issueFrom(created, pc.Project.Key, labels)
	out.DescriptionDoc = docJSON(createdDoc)
	return out, nil
}

// --- Mutating ---------------------------------------------------------------------

// change is a domain event a command produces.
type change struct {
	typ     string
	payload map[string]any
}

// edit is what a command's function gets: the resolved configuration, the
// issue as it is (locked), and the copy to change.
type edit struct {
	tx     pgx.Tx
	r      *configdef.Resolved
	cur    db.Issue
	next   *db.Issue
	labels *[]uuid.UUID // nil: labels unchanged
	// description, when set, is the new description document (its text
	// already in next.Description).
	description *document.Node
}

// mutate runs one command on an issue: permission, lock, version check
// (If-Match, 0 skips it), archived check, then fn, then the save and the
// events — all in one transaction.
func (s *Service) mutate(ctx context.Context, ic Context, version int32, perm access.Permission, allowArchived bool,
	fn func(e *edit) ([]change, error)) (Issue, error) {
	if err := s.require(ctx, ic.Context, perm); err != nil {
		return Issue{}, err
	}
	var saved db.Issue
	var labels []uuid.UUID
	var described *DocSteps
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockIssue(ctx, db.LockIssueParams{WorkspaceID: ic.Workspace.ID, ID: ic.Issue.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("issue_not_found")
		}
		if err != nil {
			return err
		}
		if version != 0 && cur.Version != version {
			return apperr.Conflict("version_conflict")
		}
		if cur.ArchivedAt != nil && !allowArchived {
			return apperr.Conflict("issue_archived")
		}
		r, err := s.config.Resolve(ctx, tx, ic.Workspace.ID, uuid.NullUUID{UUID: cur.ProjectID, Valid: true})
		if err != nil {
			return err
		}
		next := cur
		e := &edit{tx: tx, r: r, cur: cur, next: &next}
		changes, err := fn(e)
		if err != nil {
			return err
		}
		if len(changes) == 0 {
			saved = cur
			cl, err := s.labelsOf(ctx, tx, []uuid.UUID{cur.ID})
			labels = cl[cur.ID]
			return err
		}
		saved, err = s.save(ctx, tx, next)
		if err != nil {
			return err
		}
		if e.description != nil {
			// One replace step, so whoever edits it receives it.
			if described, err = s.replaceDescription(ctx, tx, saved, e.description, uuid.NullUUID{UUID: ic.Principal.User.ID, Valid: true}); err != nil {
				return err
			}
		}
		if cur.IterationID.Valid && cur.IterationID != saved.IterationID {
			if err := s.leftIteration(ctx, tx, ic.Context.Context, cur, LeftCleared); err != nil {
				return err
			}
		}
		if e.labels != nil {
			if err := q.ClearIssueLabels(ctx, cur.ID); err != nil {
				return err
			}
			for _, l := range *e.labels {
				if err := q.AddIssueLabel(ctx, db.AddIssueLabelParams{WorkspaceID: cur.WorkspaceID, IssueID: cur.ID, LabelID: l}); err != nil {
					return err
				}
			}
			labels = *e.labels
		} else {
			cl, err := s.labelsOf(ctx, tx, []uuid.UUID{cur.ID})
			if err != nil {
				return err
			}
			labels = cl[cur.ID]
		}
		events := make([]eventlog.Event, len(changes))
		for i, c := range changes {
			events[i] = s.event(ic.Context.Context, saved, c.typ, c.payload)
		}
		return eventlog.Append(ctx, tx, events...)
	})
	if err != nil {
		return Issue{}, err
	}
	if described != nil {
		s.broadcast(*described)
	}
	return issueFrom(saved, ic.Project.Key, labels), nil
}

func (s *Service) save(ctx context.Context, tx pgx.Tx, i db.Issue) (db.Issue, error) {
	return db.New(tx).SaveIssue(ctx, db.SaveIssueParams{
		WorkspaceID: i.WorkspaceID, ID: i.ID, TypeID: i.TypeID, Title: i.Title, Description: i.Description, StatusID: i.StatusID,
		Priority: i.Priority, Rank: i.Rank, Estimate: i.Estimate, AssigneeID: i.AssigneeID, ParentID: i.ParentID,
		StartDate: i.StartDate, DueDate: i.DueDate, Fields: i.Fields, StartedAt: i.StartedAt, ResolvedAt: i.ResolvedAt,
		ArchivedAt: i.ArchivedAt, UpdatedAt: s.now(), IterationID: i.IterationID, ReleaseID: i.ReleaseID,
	})
}

func (s *Service) require(ctx context.Context, pc project.Context, perm access.Permission) error {
	return access.Require(ctx, s.access, pc.Principal, pc.Scope(), perm)
}

func (s *Service) event(wc workspace.Context, i db.Issue, typ string, payload map[string]any) eventlog.Event {
	if payload == nil {
		payload = map[string]any{}
	}
	return eventlog.Event{
		WorkspaceID: uuid.NullUUID{UUID: i.WorkspaceID, Valid: true}, AggregateType: "issue", AggregateID: i.ID,
		AggregateVersion: int64(i.Version), Type: typ,
		ActorID:    uuid.NullUUID{UUID: wc.Principal.User.ID, Valid: wc.Principal.User.ID != uuid.Nil},
		OccurredAt: s.now(), Payload: eventlog.Payload(payload),
	}
}

// --- Validation helpers -------------------------------------------------------------

func findType(r *configdef.Resolved, id uuid.UUID) (configdef.IssueType, bool) {
	for _, it := range r.IssueTypes {
		if it.ID == id {
			return it, true
		}
	}
	return configdef.IssueType{}, false
}

// outcomeOf returns the outcome of a status's category in r.
func outcomeOf(r *configdef.Resolved, statusID uuid.UUID) string {
	st, _, ok := r.Status(statusID)
	if !ok {
		return ""
	}
	return r.Categories[st.CategoryID].Outcome
}

// Started reports whether a status is in a started category.
func Started(r *configdef.Resolved, statusID uuid.UUID) bool {
	return outcomeOf(r, statusID) == configdef.OutcomeStarted
}

func validTitle(fields *apperr.Fields, title string) string {
	title = strings.TrimSpace(title)
	switch {
	case title == "":
		fields.Add("title", "required")
	case len([]rune(title)) > maxTitle:
		fields.Add("title", "too_long")
	}
	return title
}

// validEstimate accepts a value of the scale, or "" to clear.
func validEstimate(fields *apperr.Fields, r *configdef.Resolved, v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if r.Estimation.Kind == configdef.ScaleNone {
		fields.Add("estimate", "estimation_off")
		return nil
	}
	for _, sv := range r.Estimation.Values {
		if sv.Label == v {
			return &v
		}
	}
	fields.Add("estimate", "not_in_scale")
	return nil
}

func validLabels(fields *apperr.Fields, r *configdef.Resolved, ids []uuid.UUID) []uuid.UUID {
	if len(ids) > maxLabels {
		fields.Add("labels", "too_many")
	}
	out := []uuid.UUID{}
	for _, id := range ids {
		if _, ok := r.Labels[id]; !ok {
			fields.Add("labels", "unknown")
			continue
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

func checkDates(fields *apperr.Fields, start, due pgtype.Date) {
	if start.Valid && due.Valid && start.Time.After(due.Time) {
		fields.Add("due_date", "before_start")
	}
}

// checkAssignee requires an active member of the workspace.
func (s *Service) checkAssignee(ctx context.Context, tx pgx.Tx, fields *apperr.Fields, workspaceID, userID uuid.UUID) {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2 AND status = 'active')`,
		workspaceID, userID).Scan(&ok)
	if err != nil || !ok {
		fields.Add("assignee_id", "not_member")
	}
}

// findParent resolves a parent key visible to the caller, in the workspace.
func (s *Service) findParent(ctx context.Context, tx pgx.Tx, wc workspace.Context, key string) (db.Issue, error) {
	ic, err := s.Enter(ctx, wc, key)
	if err != nil {
		return db.Issue{}, err
	}
	return ic.Issue, nil
}

// checkParentRule applies an issue type's parent rule to a parent (nil:
// none).
func checkParentRule(fields *apperr.Fields, r *configdef.Resolved, it configdef.IssueType, parent *db.Issue) {
	switch {
	case parent == nil && it.ParentRule == configdef.ParentRequired:
		fields.Add("parent", "required")
	case parent != nil && it.ParentRule == configdef.ParentNone:
		fields.Add("parent", "not_allowed")
	case parent != nil && len(it.ParentTypeIDs) > 0 && !slices.Contains(it.ParentTypeIDs, parent.TypeID):
		fields.Add("parent", "type_not_allowed")
	}
}

// Children lists the issue's children the caller can see, in rank order.
func (s *Service) Children(ctx context.Context, ic Context) ([]Issue, error) {
	rows, err := db.New(s.pool).Children(ctx, db.ChildrenParams{WorkspaceID: ic.Workspace.ID, ParentID: uuid.NullUUID{UUID: ic.Issue.ID, Valid: true}})
	if err != nil {
		return nil, err
	}
	out := []Issue{}
	for _, r := range rows {
		child, key, ok, err := s.visibleIssue(ctx, ic, r.ID)
		if err != nil {
			return nil, err
		}
		if ok {
			labels, err := s.labelsOf(ctx, s.pool, []uuid.UUID{child.ID})
			if err != nil {
				return nil, err
			}
			i := issueFrom(child, "", labels[child.ID])
			i.Key = key
			out = append(out, i)
		}
	}
	return out, nil
}
