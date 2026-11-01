// Package search finds issues by key, words and fuzzy titles with
// PostgreSQL only (ADR-0020): full-text search over titles, descriptions
// and comments, trigram similarity on titles, and the caller's
// permissions in the same SQL (docs/security.md).
package search

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/search/db"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Snippet markers around matched words: control characters, which issue
// text doesn't contain, so clients can highlight safely after escaping.
const (
	MarkStart = "\x02"
	MarkStop  = "\x03"
)

const headlineOptions = "StartSel=" + MarkStart + ", StopSel=" + MarkStop + ", MaxWords=24, MinWords=10, MaxFragments=1"

// Limits of a search.
const (
	DefaultLimit = 20
	MaxLimit     = 100
	maxQuery     = 200
)

var issueKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{1,9}-[1-9][0-9]*$`)

// Service searches a workspace's issues.
type Service struct {
	pool     *pgxpool.Pool
	access   *access.Service
	projects *project.Service
	issues   *issue.Service
}

// New returns a Service.
func New(pool *pgxpool.Pool, acc *access.Service, projects *project.Service, issues *issue.Service) *Service {
	return &Service{pool: pool, access: acc, projects: projects, issues: issues}
}

// Status is an issue's status, named.
type Status struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name" example:"In progress"`
	Outcome string    `json:"outcome" enums:"unstarted,started,completed,abandoned"`
}

// Result is an issue that matches, and where.
type Result struct {
	ID         uuid.UUID     `json:"id"`
	Key        string        `json:"key" example:"HZN-42"`
	Title      string        `json:"title"`
	ProjectID  uuid.UUID     `json:"project_id"`
	Status     Status        `json:"status"`
	TypeID     uuid.UUID     `json:"type_id"`
	Priority   string        `json:"priority"`
	AssigneeID uuid.NullUUID `json:"assignee_id" swaggertype:"string"`
	Archived   bool          `json:"archived"`
	// Match is where the query matched: the key, the title, the description
	// or a comment.
	Match string `json:"match" enums:"key,title,description,comment"`
	// Snippet is the matching text of the description or comment, matched
	// words between U+0002 and U+0003.
	Snippet   string        `json:"snippet,omitempty"`
	CommentID uuid.NullUUID `json:"comment_id,omitempty" swaggertype:"string"`
	score     float64
}

// Results are the matches, best first.
type Results struct {
	Query string   `json:"query"`
	Items []Result `json:"items"`
}

// Options narrow a search.
type Options struct {
	Limit    int
	Archived bool   // include archived issues
	Project  string // only this project (key)
}

// Search returns the issues of visible projects matching q: an exact key
// first, then word matches (each word as a prefix, so typing works) in
// titles, descriptions and comments, and titles close to q despite typos.
func (s *Service) Search(ctx context.Context, wc workspace.Context, q string, o Options) (Results, error) {
	q = strings.TrimSpace(q)
	if r := []rune(q); len(r) > maxQuery {
		q = string(r[:maxQuery])
	}
	out := Results{Query: q, Items: []Result{}}
	if q == "" {
		return out, nil
	}
	if o.Limit <= 0 || o.Limit > MaxLimit {
		o.Limit = DefaultLimit
	}
	projects, err := s.visibleProjects(ctx, wc, o.Project)
	if err != nil || len(projects) == 0 {
		return out, err
	}
	keys := map[uuid.UUID]string{}
	ids := make([]uuid.UUID, 0, len(projects))
	for _, p := range projects {
		keys[p.ID] = p.Key
		ids = append(ids, p.ID)
	}
	seen := map[uuid.UUID]bool{}

	// An issue key: that issue first (former keys resolve).
	if issueKey.MatchString(q) {
		if ic, err := s.issues.Enter(ctx, wc, q); err == nil && slices.Contains(ids, ic.Issue.ProjectID) && (o.Archived || ic.Issue.ArchivedAt == nil) {
			i, err := s.issues.Get(ctx, ic)
			if err != nil {
				return out, err
			}
			st, err := s.status(ctx, i.StatusID)
			if err != nil {
				return out, err
			}
			out.Items = append(out.Items, Result{ID: i.ID, Key: i.Key, Title: i.Title, ProjectID: i.ProjectID, Status: st, TypeID: i.TypeID,
				Priority: i.Priority, AssigneeID: i.AssigneeID, Archived: i.ArchivedAt != nil, Match: "key"})
			seen[i.ID] = true
		}
	}

	tsq := tsquery(q)
	if tsq == "" {
		return out, nil
	}
	query := db.New(s.pool)
	issues, err := query.SearchIssues(ctx, db.SearchIssuesParams{Tsquery: tsq, Raw: q, HeadlineOptions: headlineOptions,
		WorkspaceID: wc.Workspace.ID, ProjectIds: ids, Archived: o.Archived, MaxRows: int32(o.Limit)})
	if err != nil {
		return out, err
	}
	for _, r := range issues {
		if seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		res := Result{ID: r.ID, Key: issue.KeyOf(keys[r.ProjectID], r.Number), Title: r.Title, ProjectID: r.ProjectID,
			Status: Status{ID: r.StatusID, Name: r.StatusName, Outcome: r.StatusOutcome}, TypeID: r.TypeID, Priority: r.Priority,
			AssigneeID: r.AssigneeID, Archived: r.ArchivedAt != nil, Match: "title", score: r.Score}
		if strings.Contains(r.Headline, MarkStart) {
			res.Snippet = r.Headline
			if !r.InTitle {
				res.Match = "description"
			}
		}
		out.Items = append(out.Items, res)
	}
	comments, err := query.SearchComments(ctx, db.SearchCommentsParams{Tsquery: tsq, HeadlineOptions: headlineOptions,
		WorkspaceID: wc.Workspace.ID, ProjectIds: ids, Archived: o.Archived, MaxRows: int32(o.Limit)})
	if err != nil {
		return out, err
	}
	var extra []Result
	for _, r := range comments {
		if seen[r.IssueID] {
			continue
		}
		seen[r.IssueID] = true
		extra = append(extra, Result{ID: r.IssueID, Key: issue.KeyOf(keys[r.ProjectID], r.Number), Title: r.Title, ProjectID: r.ProjectID,
			Status: Status{ID: r.StatusID, Name: r.StatusName, Outcome: r.StatusOutcome}, TypeID: r.TypeID, Priority: r.Priority,
			AssigneeID: r.AssigneeID, Archived: r.ArchivedAt != nil, Match: "comment", Snippet: r.Headline,
			CommentID: uuid.NullUUID{UUID: r.CommentID, Valid: true}, score: r.Score})
	}
	slices.SortStableFunc(extra, func(a, b Result) int {
		switch {
		case a.score > b.score:
			return -1
		case a.score < b.score:
			return 1
		}
		return 0
	})
	out.Items = append(out.Items, extra...)
	if len(out.Items) > o.Limit {
		out.Items = out.Items[:o.Limit]
	}
	return out, nil
}

// visibleProjects are the projects whose issues the caller may see.
func (s *Service) visibleProjects(ctx context.Context, wc workspace.Context, only string) ([]project.Project, error) {
	all, err := s.projects.List(ctx, wc)
	if err != nil {
		return nil, err
	}
	var out []project.Project
	for _, p := range all {
		if only != "" && !strings.EqualFold(p.Key, only) && !slices.ContainsFunc(p.Aliases, func(a string) bool { return strings.EqualFold(a, only) }) {
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

func (s *Service) status(ctx context.Context, id uuid.UUID) (Status, error) {
	st := Status{ID: id}
	err := s.pool.QueryRow(ctx, `SELECT s.name, sc.outcome FROM statuses s JOIN status_categories sc ON sc.id = s.category_id WHERE s.id = $1`, id).
		Scan(&st.Name, &st.Outcome)
	return st, err
}

// tsquery makes a prefix query of q's words, all required:
// "outbox rel" → "outbox:* & rel:*". Operators and quotes are dropped.
func tsquery(q string) string {
	words := strings.FieldsFunc(strings.ToLower(q), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	if len(words) > 12 {
		words = words[:12]
	}
	for i, w := range words {
		words[i] = w + ":*"
	}
	return strings.Join(words, " & ")
}
