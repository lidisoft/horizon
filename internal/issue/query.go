package issue

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/issue/db"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/query"
	"github.com/lidisoft/horizon/internal/workspace"
)

// QueryResult is a page of issues matching a query.
type QueryResult struct {
	Items []Issue `json:"items"`
	// Total is the number of issues matching, all pages.
	Total int `json:"total"`
	// Next is the cursor of the following page, if any.
	Next string `json:"next_cursor,omitempty"`
}

// Query runs a query (docs/api.md#querying-issues) over the projects the
// caller may see issues of; projects names those to look in (keys; none:
// all). cursor is the next_cursor of the previous page.
func (s *Service) Query(ctx context.Context, wc workspace.Context, q query.Query, projects []string, cursor string) (QueryResult, error) {
	return s.query(ctx, wc, q, projects, cursor, "TRUE")
}

// compiled is a query ready to run: its FROM … WHERE over issues i (with
// their status s and category sc), and the arguments.
type compiled struct {
	from    string
	orderBy string
	args    []any
	keys    map[uuid.UUID]string // project keys by id, of the projects looked in
}

// compile prepares q over the visible projects named, with a further
// condition on i (trusted SQL, no arguments); ok is false when no project
// is visible.
func (s *Service) compile(ctx context.Context, wc workspace.Context, q query.Query, projects []string, also string) (compiled, bool, error) {
	visible, err := s.queryProjects(ctx, wc, projects)
	if err != nil || len(visible) == 0 {
		return compiled{}, false, err
	}
	keys := map[uuid.UUID]string{}
	byKey := map[string]uuid.UUID{}
	ids := make([]uuid.UUID, 0, len(visible))
	for _, p := range visible {
		keys[p.ID] = p.Key
		byKey[p.Key] = p.ID
		for _, a := range p.Aliases {
			byKey[a] = p.ID
		}
		ids = append(ids, p.ID)
	}
	qc, err := s.queryContext(ctx, wc, ids, byKey)
	if err != nil {
		return compiled{}, false, err
	}
	c, err := query.Compile(q, qc, 3)
	if err != nil {
		var qe *query.Error
		if errors.As(err, &qe) {
			return compiled{}, false, apperr.Invalid(apperr.Field{Field: qe.Path, Code: qe.Code})
		}
		return compiled{}, false, err
	}
	return compiled{
		from: `FROM issues i JOIN statuses s ON s.id = i.status_id JOIN status_categories sc ON sc.id = s.category_id
		WHERE i.workspace_id = $1 AND i.project_id = ANY($2::uuid[]) AND (` + c.Where + `) AND (` + also + `)`,
		orderBy: c.OrderBy,
		args:    append([]any{wc.Workspace.ID, ids}, c.Args...),
		keys:    keys,
	}, true, nil
}

// VisibleProjects are the projects the caller sees issues of (keys by id).
func (s *Service) VisibleProjects(ctx context.Context, wc workspace.Context) (map[uuid.UUID]string, error) {
	visible, err := s.queryProjects(ctx, wc, nil)
	out := make(map[uuid.UUID]string, len(visible))
	for _, p := range visible {
		out[p.ID] = p.Key
	}
	return out, err
}

// MaxScope bounds the issues a report covers.
const MaxScope = 20000

// Scope is the issues a query matches now, for reports: their ids, and
// the projects looked in (key by id).
type Scope struct {
	IDs      []uuid.UUID
	Projects map[uuid.UUID]string
}

// Scope runs q without paging (archived issues only with q.Archived), up
// to MaxScope issues (422 scope_too_large beyond).
func (s *Service) Scope(ctx context.Context, wc workspace.Context, q query.Query, projects []string) (Scope, error) {
	c, ok, err := s.compile(ctx, wc, q, projects, "TRUE")
	if err != nil || !ok {
		return Scope{Projects: map[uuid.UUID]string{}}, err
	}
	rows, err := s.pool.Query(ctx, "SELECT i.id "+c.from+" LIMIT "+strconv.Itoa(MaxScope+1), c.args...)
	if err != nil {
		return Scope{}, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return Scope{}, err
	}
	if len(ids) > MaxScope {
		return Scope{}, apperr.Invalid(apperr.Field{Field: "filter", Code: "scope_too_large"})
	}
	return Scope{IDs: ids, Projects: c.keys}, nil
}

// query runs q with a further condition on i (trusted SQL, no arguments).
func (s *Service) query(ctx context.Context, wc workspace.Context, q query.Query, projects []string, cursor, also string) (QueryResult, error) {
	out := QueryResult{Items: []Issue{}}
	c, ok, err := s.compile(ctx, wc, q, projects, also)
	if err != nil || !ok {
		return out, err
	}
	keys := c.keys
	limit := q.Limit
	if limit <= 0 {
		limit = query.DefaultLimit
	}
	limit = min(limit, query.MaxLimit)
	offset := 0
	if cursor != "" {
		if offset, err = strconv.Atoi(cursor); err != nil || offset < 0 {
			return QueryResult{}, apperr.Invalid(apperr.Field{Field: "cursor", Code: "invalid"})
		}
	}
	from, args := c.from, c.args
	if err := s.pool.QueryRow(ctx, "SELECT count(*) "+from, args...).Scan(&out.Total); err != nil {
		return QueryResult{}, err
	}
	rows, err := s.pool.Query(ctx, "SELECT i.id "+from+" ORDER BY "+c.orderBy+
		" LIMIT "+strconv.Itoa(limit)+" OFFSET "+strconv.Itoa(offset), args...)
	if err != nil {
		return QueryResult{}, err
	}
	order, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return QueryResult{}, err
	}
	issues, err := db.New(s.pool).IssuesByIDs(ctx, db.IssuesByIDsParams{WorkspaceID: wc.Workspace.ID, Ids: order})
	if err != nil {
		return QueryResult{}, err
	}
	labels, err := s.labelsOf(ctx, s.pool, order)
	if err != nil {
		return QueryResult{}, err
	}
	byID := make(map[uuid.UUID]db.Issue, len(issues))
	for _, i := range issues {
		byID[i.ID] = i
	}
	for _, id := range order {
		if i, ok := byID[id]; ok {
			out.Items = append(out.Items, issueFrom(i, keys[i.ProjectID], labels[id]))
		}
	}
	if offset+len(order) < out.Total {
		out.Next = strconv.Itoa(offset + len(order))
	}
	return out, nil
}

// queryProjects are the projects a query looks in: those named (or all)
// that the caller sees issues of.
func (s *Service) queryProjects(ctx context.Context, wc workspace.Context, only []string) ([]project.Project, error) {
	all, err := s.projects.List(ctx, wc)
	if err != nil {
		return nil, err
	}
	var out []project.Project
	for _, p := range all {
		if len(only) > 0 && !named(p, only) {
			continue
		}
		ok, err := s.access.Can(ctx, wc.Principal, access.Scope{WorkspaceID: wc.Workspace.ID, ProjectID: uuid.NullUUID{UUID: p.ID, Valid: true}}, access.IssueView)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func named(p project.Project, keys []string) bool {
	for _, k := range keys {
		if strings.EqualFold(k, p.Key) {
			return true
		}
		for _, a := range p.Aliases {
			if strings.EqualFold(k, a) {
				return true
			}
		}
	}
	return false
}

// queryContext gathers what a query's values and placeholders resolve against.
func (s *Service) queryContext(ctx context.Context, wc workspace.Context, projectIDs []uuid.UUID, byKey map[string]uuid.UUID) (query.Context, error) {
	types := map[uuid.UUID]string{}
	rows, err := s.pool.Query(ctx, `SELECT id, type FROM custom_fields WHERE workspace_id = $1`, wc.Workspace.ID)
	if err != nil {
		return query.Context{}, err
	}
	for rows.Next() {
		var id uuid.UUID
		var t string
		if err := rows.Scan(&id, &t); err != nil {
			rows.Close()
			return query.Context{}, err
		}
		types[id] = t
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT id FROM iterations WHERE workspace_id = $1 AND project_id = ANY($2::uuid[]) AND state = 'active'`, wc.Workspace.ID, projectIDs)
	if err != nil {
		return query.Context{}, err
	}
	current, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return query.Context{}, err
	}
	loc, err := time.LoadLocation(wc.Workspace.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	y, m, d := s.now().In(loc).Date()
	return query.Context{
		Me:                wc.Principal.User.ID,
		Today:             time.Date(y, m, d, 0, 0, 0, 0, time.UTC),
		CurrentIterations: current,
		Projects:          byKey,
		CustomFields: func(id uuid.UUID) (string, bool) {
			t, ok := types[id]
			return t, ok
		},
		IssueID: func(key string) (uuid.UUID, bool) {
			ic, err := s.Enter(ctx, wc, key)
			return ic.Issue.ID, err == nil
		},
	}, nil
}
