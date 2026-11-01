package report

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/calendar"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/planning"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/query"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Metrics a report measures in (docs/reports.md#common-rules).
const (
	MetricCount    = "count"
	MetricEstimate = "estimate"
)

// Why a report measures in count though estimate was asked.
const (
	FallbackNoEstimates = "no_estimates" // a project's scale is none
	FallbackMixedScales = "mixed_scales" // the projects' scales differ
)

// maxDays bounds a daily series.
const maxDays = 366

// Service answers the reports.
type Service struct {
	pool     *pgxpool.Pool
	access   *access.Service
	config   *configdef.Service
	projects *project.Service
	issues   *issue.Service
	plans    *planning.Service
	now      func() time.Time
}

// New returns a Service.
func New(pool *pgxpool.Pool, acc *access.Service, config *configdef.Service, projects *project.Service, issues *issue.Service,
	plans *planning.Service, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{pool: pool, access: acc, config: config, projects: projects, issues: issues, plans: plans, now: now}
}

// Request is a report's scope and options: a query over projects, or one
// iteration, release or milestone. Its URL encoding is the report's
// shareable link (docs/reports.md#api).
type Request struct {
	Filter   *query.Node `json:"filter,omitempty"`
	Archived bool        `json:"archived,omitempty"`
	Projects []string    `json:"projects,omitempty" example:"HZN"`
	// Iteration, Release and Milestone scope sprint and progress reports.
	Iteration uuid.NullUUID `json:"iteration,omitempty" swaggertype:"string"`
	Release   uuid.NullUUID `json:"release,omitempty" swaggertype:"string"`
	Milestone uuid.NullUUID `json:"milestone,omitempty" swaggertype:"string"`
	// Metric is count (default) or estimate.
	Metric string `json:"metric,omitempty" enums:"count,estimate"`
	// From and To bound time series (dates, inclusive); defaults per report.
	From string `json:"from,omitempty" example:"2026-10-01"`
	To   string `json:"to,omitempty" example:"2026-11-30"`
	// Last is how many iterations velocity shows (default 7, at most 20).
	Last int `json:"last,omitempty"`
	// Group is what distribution groups by: assignee, type, priority,
	// label, project, or cf:<select field id>.
	Group string `json:"group,omitempty" example:"assignee"`
	// Period buckets throughput and created vs resolved: week (default) or month.
	Period string `json:"period,omitempty" enums:"week,month"`
	// By is what cumulative flow stacks: outcome (default) or category
	// (one project only).
	By string `json:"by,omitempty" enums:"outcome,category"`
}

// Meta says how a report was measured.
type Meta struct {
	Metric string `json:"metric" enums:"count,estimate"`
	// Fallback says why it is in count though estimate was asked.
	Fallback string `json:"metric_fallback,omitempty" enums:"no_estimates,mixed_scales"`
	TimeZone string `json:"time_zone" example:"Europe/Paris"`
}

// target is what a report covers once resolved.
type target struct {
	wc       workspace.Context
	projects map[uuid.UUID]string // key by id: the projects covered
	issues   []uuid.UUID
	cal      calendar.Calendar
	meta     Meta
}

// projectTarget is one project the caller sees issues of.
func (s *Service) projectContext(ctx context.Context, wc workspace.Context, projectID uuid.UUID) (project.Context, error) {
	var key string
	if err := s.pool.QueryRow(ctx, `SELECT key FROM projects WHERE workspace_id = $1 AND id = $2`, wc.Workspace.ID, projectID).Scan(&key); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return project.Context{}, apperr.NotFound("project_not_found")
		}
		return project.Context{}, err
	}
	pc, err := s.projects.Enter(ctx, wc, key)
	if err != nil {
		return pc, err
	}
	return pc, access.Require(ctx, s.access, pc.Principal, pc.Scope(), access.IssueView)
}

// measure settles the metric and time zone of a target: one project's
// calendar, else the workspace's time zone.
func (s *Service) measure(ctx context.Context, wc workspace.Context, r Request, projects map[uuid.UUID]string, issues []uuid.UUID) (*target, error) {
	t := &target{wc: wc, projects: projects, issues: issues, cal: calendar.New(wc.Workspace.TimeZone, nil, nil)}
	t.meta = Meta{Metric: MetricCount, TimeZone: t.cal.Location.String()}
	if len(projects) == 1 {
		for id := range projects {
			c, err := s.config.CalendarSettings(ctx, s.pool, configdef.Target{Principal: wc.Principal, WorkspaceID: wc.Workspace.ID, ProjectID: uuid.NullUUID{UUID: id, Valid: true}})
			if err != nil {
				return nil, err
			}
			t.cal = c.Calendar()
			t.meta.TimeZone = c.TimeZone.Value
		}
	}
	switch r.Metric {
	case "", MetricCount:
	case MetricEstimate:
		var first *configdef.Scale
		for id := range projects {
			e, err := s.config.Estimation(ctx, configdef.Target{Principal: wc.Principal, WorkspaceID: wc.Workspace.ID, ProjectID: uuid.NullUUID{UUID: id, Valid: true}})
			if err != nil {
				return nil, err
			}
			switch {
			case e.Kind == "none":
				t.meta.Fallback = FallbackNoEstimates
			case first == nil:
				first = &e.Scale
			case !sameScale(*first, e.Scale):
				t.meta.Fallback = FallbackMixedScales
			}
		}
		if t.meta.Fallback == "" {
			t.meta.Metric = MetricEstimate
		}
	default:
		return nil, apperr.Invalid(apperr.Field{Field: "metric", Code: "invalid"})
	}
	return t, nil
}

func sameScale(a, b configdef.Scale) bool {
	ja, _ := json.Marshal(a.Values)
	jb, _ := json.Marshal(b.Values)
	return a.Kind == b.Kind && string(ja) == string(jb)
}

// --- History ------------------------------------------------------------------------

type stay struct {
	outcome  string
	category uuid.NullUUID
	status   uuid.UUID
	from, to time.Time // to zero: still there
}

type weighted struct {
	weight   float64
	from, to time.Time
}

type membership struct {
	issue    uuid.UUID
	from, to time.Time
}

// history is the projections of a set of issues, to ask about any instant.
type history struct {
	stays   map[uuid.UUID][]stay
	weights map[uuid.UUID][]weighted
	created map[uuid.UUID]time.Time
}

func covers(from, to, at time.Time) bool {
	return !from.After(at) && (to.IsZero() || to.After(at))
}

// outcome is the issue's outcome at, "" before it existed.
func (h *history) outcome(id uuid.UUID, at time.Time) string {
	if st, ok := h.stay(id, at); ok {
		return st.outcome
	}
	return ""
}

func (h *history) stay(id uuid.UUID, at time.Time) (stay, bool) {
	ss := h.stays[id]
	for k := len(ss) - 1; k >= 0; k-- {
		if covers(ss[k].from, ss[k].to, at) {
			return ss[k], true
		}
	}
	return stay{}, false
}

// weight is the issue's estimate weight at (0 without one).
func (h *history) weight(id uuid.UUID, at time.Time) float64 {
	for _, w := range h.weights[id] {
		if covers(w.from, w.to, at) {
			return w.weight
		}
	}
	return 0
}

// measure is what an issue counts for at, in the target's metric.
func (h *history) measure(t *target, id uuid.UUID, at time.Time) float64 {
	if t.meta.Metric == MetricEstimate {
		return h.weight(id, at)
	}
	return 1
}

// load reads the projections of the issues.
func (s *Service) history(ctx context.Context, issues []uuid.UUID) (*history, error) {
	h := &history{stays: map[uuid.UUID][]stay{}, weights: map[uuid.UUID][]weighted{}, created: map[uuid.UUID]time.Time{}}
	rows, err := s.pool.Query(ctx, `SELECT issue_id, outcome, category_id, status_id, entered_at, left_at FROM report_status_intervals
		WHERE issue_id = ANY($1::uuid[]) ORDER BY issue_id, entered_at, id`, issues)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uuid.UUID
		var st stay
		var to *time.Time
		if err := rows.Scan(&id, &st.outcome, &st.category, &st.status, &st.from, &to); err != nil {
			rows.Close()
			return nil, err
		}
		if to != nil {
			st.to = *to
		}
		h.stays[id] = append(h.stays[id], st)
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT issue_id, coalesce(weight, 0), valid_from, valid_to FROM report_estimates
		WHERE issue_id = ANY($1::uuid[]) ORDER BY issue_id, valid_from, id`, issues)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uuid.UUID
		var w weighted
		var to *time.Time
		if err := rows.Scan(&id, &w.weight, &w.from, &to); err != nil {
			rows.Close()
			return nil, err
		}
		if to != nil {
			w.to = *to
		}
		h.weights[id] = append(h.weights[id], w)
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT issue_id, created_at FROM report_issue_milestones WHERE issue_id = ANY($1::uuid[])`, issues)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var at time.Time
		if err := rows.Scan(&id, &at); err != nil {
			return nil, err
		}
		h.created[id] = at
	}
	return h, rows.Err()
}

// --- Days ---------------------------------------------------------------------------

// day is a calendar day of a series and the instant it is read at: its
// end, or now for today; future days aren't read.
type day struct {
	Date   string
	at     time.Time
	future bool
}

// days lists the days from to to in the calendar, each with its reading
// instant; at most maxDays (the last ones).
func (s *Service) days(cal calendar.Calendar, from, to time.Time) []day {
	if to.Before(from) {
		return nil
	}
	if n := int(to.Sub(from).Hours()/24) + 1; n > maxDays {
		from = to.AddDate(0, 0, 1-maxDays)
	}
	now := s.now()
	today := cal.Today(now)
	var out []day
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		y, m, dd := d.Date()
		end := time.Date(y, m, dd+1, 0, 0, 0, 0, cal.Location)
		dy := day{Date: d.Format(time.DateOnly), at: end}
		switch {
		case d.After(today):
			dy.future = true
		case d.Equal(today):
			dy.at = now
		}
		out = append(out, dy)
	}
	return out
}

// parseRange reads r.From and r.To, with defaults.
func parseRange(r Request, defFrom, defTo time.Time) (time.Time, time.Time, error) {
	from, to := defFrom, defTo
	var fields apperr.Fields
	if r.From != "" {
		t, err := time.Parse(time.DateOnly, r.From)
		if err != nil {
			fields.Add("from", "invalid")
		}
		from = t
	}
	if r.To != "" {
		t, err := time.Parse(time.DateOnly, r.To)
		if err != nil {
			fields.Add("to", "invalid")
		}
		to = t
	}
	if err := fields.Err(); err != nil {
		return from, to, err
	}
	if to.Before(from) {
		return from, to, apperr.Invalid(apperr.Field{Field: "to", Code: "before_from"})
	}
	return from, to, nil
}

// num is a series value; nil for days to come.
func num(v float64, future bool) *float64 {
	if future {
		return nil
	}
	return &v
}

// IssueRef names an issue in a report.
type IssueRef struct {
	ID    uuid.UUID `json:"id"`
	Key   string    `json:"key" example:"HZN-7"`
	Title string    `json:"title"`
}

// refs names the issues that still exist.
func (s *Service) refs(ctx context.Context, wsID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]IssueRef, error) {
	rows, err := s.pool.Query(ctx, `SELECT i.id, p.key || '-' || i.number, i.title FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE i.workspace_id = $1 AND i.id = ANY($2::uuid[])`, wsID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]IssueRef{}
	for rows.Next() {
		var r IssueRef
		if err := rows.Scan(&r.ID, &r.Key, &r.Title); err != nil {
			return nil, err
		}
		out[r.ID] = r
	}
	return out, rows.Err()
}
