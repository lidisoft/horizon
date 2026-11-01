package report

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/calendar"
	"github.com/lidisoft/horizon/internal/query"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Flow and workload reports (docs/reports.md#flow-reports,
// #workload-and-distribution), over the issues a filter matches now.

// Outcomes in their stacking order (docs/reports.md#charts).
var Outcomes = []string{"completed", "started", "unstarted", "abandoned"}

// filtered resolves a query-scoped report: the issues it matches now.
func (s *Service) filtered(ctx context.Context, wc workspace.Context, r Request) (*target, *history, error) {
	sc, err := s.issues.Scope(ctx, wc, query.Query{Filter: r.Filter, Archived: r.Archived}, r.Projects)
	if err != nil {
		return nil, nil, err
	}
	t, err := s.measure(ctx, wc, r, sc.Projects, sc.IDs)
	if err != nil {
		return nil, nil, err
	}
	h, err := s.history(ctx, sc.IDs)
	return t, h, err
}

// Series is a named row of a chart.
type Series struct {
	Key    string     `json:"key" example:"completed"`
	Name   string     `json:"name"`
	Values []*float64 `json:"values"`
}

// Flow is a cumulative flow diagram.
type Flow struct {
	Meta
	Days []string `json:"days"`
	// Series stack completed, started, unstarted, abandoned (or the
	// project's categories in that order of outcomes).
	Series []Series `json:"series"`
}

// CumulativeFlow counts the issues per outcome (r.By = category: per
// category, one project only) each day, from r.From (default 30 days ago)
// to r.To (default today).
func (s *Service) CumulativeFlow(ctx context.Context, wc workspace.Context, r Request) (Flow, error) {
	t, h, err := s.filtered(ctx, wc, r)
	if err != nil {
		return Flow{}, err
	}
	today := t.cal.Today(s.now())
	from, to, err := parseRange(r, today.AddDate(0, 0, -29), today)
	if err != nil {
		return Flow{}, err
	}
	out := Flow{Meta: t.meta, Days: []string{}, Series: []Series{}}
	type column struct{ key, name string }
	var columns []column
	byCategory := r.By == "category"
	switch {
	case byCategory && len(t.projects) != 1:
		return Flow{}, apperr.Invalid(apperr.Field{Field: "by", Code: "one_project"})
	case byCategory:
		cats, err := s.categories(ctx, wc, t)
		if err != nil {
			return Flow{}, err
		}
		for _, c := range cats {
			columns = append(columns, column{c.id.String(), c.name})
		}
	case r.By == "" || r.By == "outcome":
		for _, o := range Outcomes {
			columns = append(columns, column{o, o})
		}
	default:
		return Flow{}, apperr.Invalid(apperr.Field{Field: "by", Code: "invalid"})
	}
	index := map[string]int{}
	for k, c := range columns {
		index[c.key] = k
		out.Series = append(out.Series, Series{Key: c.key, Name: c.name, Values: []*float64{}})
	}
	for _, d := range s.days(t.cal, from, to) {
		sums := make([]float64, len(columns))
		if !d.future {
			for _, id := range t.issues {
				st, ok := h.stay(id, d.at)
				if !ok {
					continue
				}
				key := st.outcome
				if byCategory {
					key = st.category.UUID.String()
				}
				if k, ok := index[key]; ok {
					sums[k] += h.measure(t, id, d.at)
				}
			}
		}
		out.Days = append(out.Days, d.Date)
		for k := range columns {
			out.Series[k].Values = append(out.Series[k].Values, num(sums[k], d.future))
		}
	}
	return out, nil
}

type category struct {
	id   uuid.UUID
	name string
}

// categories are the project's status categories, by outcome then position.
func (s *Service) categories(ctx context.Context, wc workspace.Context, t *target) ([]category, error) {
	var projectID uuid.UUID
	for id := range t.projects {
		projectID = id
	}
	r, err := s.config.Resolve(ctx, s.pool, wc.Workspace.ID, uuid.NullUUID{UUID: projectID, Valid: true})
	if err != nil {
		return nil, err
	}
	var out []category
	for _, o := range Outcomes {
		var cs []category
		var pos []int32
		for _, c := range r.Categories {
			if c.Outcome == o {
				cs = append(cs, category{c.ID, c.Name})
				pos = append(pos, c.Position)
			}
		}
		idx := make([]int, len(cs))
		for k := range idx {
			idx[k] = k
		}
		slices.SortFunc(idx, func(a, b int) int { return int(pos[a] - pos[b]) })
		for _, k := range idx {
			out = append(out, cs[k])
		}
	}
	return out, nil
}

// CyclePoint is a completed issue.
type CyclePoint struct {
	IssueRef
	Completed string `json:"completed" example:"2026-11-05"`
	// CycleDays is started → completed (null when never started), LeadDays
	// created → completed, in days.
	CycleDays *float64 `json:"cycle_days"`
	LeadDays  float64  `json:"lead_days"`
}

// Percentiles of a distribution.
type Percentiles struct {
	P50 *float64 `json:"p50"`
	P85 *float64 `json:"p85"`
	P95 *float64 `json:"p95"`
}

// CycleTime is the cycle and lead time of the issues completed.
type CycleTime struct {
	Meta
	Items []CyclePoint `json:"items"`
	Cycle Percentiles  `json:"cycle"`
	Lead  Percentiles  `json:"lead"`
}

// CycleTime reports the issues completed from r.From (default 90 days
// ago) to r.To (default today).
func (s *Service) CycleTime(ctx context.Context, wc workspace.Context, r Request) (CycleTime, error) {
	t, _, err := s.filtered(ctx, wc, r)
	if err != nil {
		return CycleTime{}, err
	}
	today := t.cal.Today(s.now())
	from, to, err := parseRange(r, today.AddDate(0, 0, -89), today)
	if err != nil {
		return CycleTime{}, err
	}
	start, end := startOf(t.cal, from), startOf(t.cal, to.AddDate(0, 0, 1))
	rows, err := s.pool.Query(ctx, `SELECT issue_id, created_at, first_started_at, completed_at FROM report_issue_milestones
		WHERE issue_id = ANY($1::uuid[]) AND completed_at >= $2 AND completed_at < $3 ORDER BY completed_at`, t.issues, start, end)
	if err != nil {
		return CycleTime{}, err
	}
	type done struct {
		id                 uuid.UUID
		created, completed time.Time
		started            *time.Time
	}
	var all []done
	for rows.Next() {
		var d done
		if err := rows.Scan(&d.id, &d.created, &d.started, &d.completed); err != nil {
			rows.Close()
			return CycleTime{}, err
		}
		all = append(all, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return CycleTime{}, err
	}
	ids := make([]uuid.UUID, len(all))
	for k, d := range all {
		ids[k] = d.id
	}
	refs, err := s.refs(ctx, wc.Workspace.ID, ids)
	if err != nil {
		return CycleTime{}, err
	}
	out := CycleTime{Meta: t.meta, Items: []CyclePoint{}}
	var cycles, leads []float64
	days := func(a, b time.Time) float64 { return float64(int(b.Sub(a).Hours()/24*10+0.5)) / 10 } // a tenth of a day
	for _, d := range all {
		p := CyclePoint{IssueRef: refs[d.id], Completed: d.completed.In(t.cal.Location).Format(time.DateOnly), LeadDays: days(d.created, d.completed)}
		if d.started != nil {
			c := days(*d.started, d.completed)
			p.CycleDays = &c
			cycles = append(cycles, c)
		}
		leads = append(leads, p.LeadDays)
		out.Items = append(out.Items, p)
	}
	out.Cycle, out.Lead = percentiles(cycles), percentiles(leads)
	return out, nil
}

func percentiles(v []float64) Percentiles {
	slices.Sort(v)
	return Percentiles{P50: percentile(v, 0.5), P85: percentile(v, 0.85), P95: percentile(v, 0.95)}
}

// percentile of sorted values (nearest rank), nil when empty.
func percentile(sorted []float64, p float64) *float64 {
	if len(sorted) == 0 {
		return nil
	}
	k := int(p*float64(len(sorted))+0.999999) - 1
	k = min(max(k, 0), len(sorted)-1)
	v := sorted[k]
	return &v
}

// period is a bucket of a periodic report.
type period struct {
	label    string // its first day
	from, to time.Time
	future   bool
}

// periods splits from..to into weeks (Monday first) or months, in the
// calendar's time zone.
func (s *Service) periods(t *target, r Request, weeksBack int) ([]period, error) {
	today := t.cal.Today(s.now())
	monthly := r.Period == "month"
	if r.Period != "" && r.Period != "week" && !monthly {
		return nil, apperr.Invalid(apperr.Field{Field: "period", Code: "invalid"})
	}
	defFrom := today.AddDate(0, 0, -7*(weeksBack-1))
	if monthly {
		defFrom = time.Date(today.Year(), today.Month()-5, 1, 0, 0, 0, 0, time.UTC)
	}
	from, to, err := parseRange(r, defFrom, today)
	if err != nil {
		return nil, err
	}
	if monthly {
		from = time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	} else {
		from = from.AddDate(0, 0, 1-isoWeekday(from))
	}
	var out []period
	for d := from; !d.After(to) && len(out) < 520; {
		next := d.AddDate(0, 0, 7)
		if monthly {
			next = d.AddDate(0, 1, 0)
		}
		out = append(out, period{label: d.Format(time.DateOnly), from: startOf(t.cal, d), to: startOf(t.cal, next), future: d.After(today)})
		d = next
	}
	return out, nil
}

// startOf is the instant a calendar day starts.
func startOf(cal calendar.Calendar, d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, cal.Location)
}

func isoWeekday(d time.Time) int {
	if w := int(d.Weekday()); w != 0 {
		return w
	}
	return 7
}

// Periodic is a report per week or month.
type Periodic struct {
	Meta
	// Periods are the first day of each week or month.
	Periods []string `json:"periods"`
	Series  []Series `json:"series"`
	// Average is the mean of the first series over past periods.
	Average float64 `json:"average"`
}

// milestoneTimes reads when the issues were created, completed, abandoned.
func (s *Service) milestoneTimes(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][3]*time.Time, error) {
	rows, err := s.pool.Query(ctx, `SELECT issue_id, created_at, completed_at, abandoned_at FROM report_issue_milestones WHERE issue_id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID][3]*time.Time{}
	for rows.Next() {
		var id uuid.UUID
		var created time.Time
		var completed, abandoned *time.Time
		if err := rows.Scan(&id, &created, &completed, &abandoned); err != nil {
			return nil, err
		}
		out[id] = [3]*time.Time{&created, completed, abandoned}
	}
	return out, rows.Err()
}

// bucket adds v to the period holding at.
func bucket(ps []period, sums []float64, at *time.Time, v float64) {
	if at == nil {
		return
	}
	for k, p := range ps {
		if !at.Before(p.from) && at.Before(p.to) {
			sums[k] += v
			return
		}
	}
}

func (s *Service) periodic(ctx context.Context, wc workspace.Context, r Request, weeksBack int, names []string,
	fill func(t *target, h *history, times map[uuid.UUID][3]*time.Time, ps []period, sums [][]float64)) (Periodic, error) {
	t, h, err := s.filtered(ctx, wc, r)
	if err != nil {
		return Periodic{}, err
	}
	ps, err := s.periods(t, r, weeksBack)
	if err != nil {
		return Periodic{}, err
	}
	times, err := s.milestoneTimes(ctx, t.issues)
	if err != nil {
		return Periodic{}, err
	}
	sums := make([][]float64, len(names))
	for k := range sums {
		sums[k] = make([]float64, len(ps))
	}
	fill(t, h, times, ps, sums)
	out := Periodic{Meta: t.meta, Periods: []string{}, Series: []Series{}}
	for _, p := range ps {
		out.Periods = append(out.Periods, p.label)
	}
	for k, name := range names {
		se := Series{Key: name, Name: name, Values: []*float64{}}
		for j, p := range ps {
			se.Values = append(se.Values, num(sums[k][j], p.future))
		}
		out.Series = append(out.Series, se)
	}
	past := 0
	for j, p := range ps {
		if !p.future {
			out.Average += sums[0][j]
			past++
		}
	}
	if past > 0 {
		out.Average /= float64(past)
	}
	return out, nil
}

// Throughput reports the issues completed per week (default: the last 12)
// or month (the last 6).
func (s *Service) Throughput(ctx context.Context, wc workspace.Context, r Request) (Periodic, error) {
	return s.periodic(ctx, wc, r, 12, []string{"completed"}, func(t *target, h *history, times map[uuid.UUID][3]*time.Time, ps []period, sums [][]float64) {
		for _, id := range t.issues {
			if at := times[id][1]; at != nil {
				bucket(ps, sums[0], at, h.measure(t, id, *at))
			}
		}
	})
}

// CreatedResolved reports the issues created and those resolved
// (completed or abandoned) per week or month.
func (s *Service) CreatedResolved(ctx context.Context, wc workspace.Context, r Request) (Periodic, error) {
	return s.periodic(ctx, wc, r, 12, []string{"created", "resolved"}, func(t *target, h *history, times map[uuid.UUID][3]*time.Time, ps []period, sums [][]float64) {
		for _, id := range t.issues {
			ts := times[id]
			if ts[0] != nil {
				bucket(ps, sums[0], ts[0], h.measure(t, id, *ts[0]))
			}
			for _, at := range ts[1:] {
				if at != nil {
					bucket(ps, sums[1], at, h.measure(t, id, *at))
				}
			}
		}
	})
}

// Group is a bar of a distribution.
type Group struct {
	// Key is the value grouped on (an id, a priority, a project key); "" for none.
	Key   string  `json:"key"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// Distribution groups the open issues.
type Distribution struct {
	Meta
	Group  string  `json:"group"`
	Groups []Group `json:"groups"`
}

// Distribution groups the open issues (unstarted or started, not
// archived) by r.Group: assignee (default), type, priority, label (an
// issue counts in each of its labels), project, or cf:<select field id>.
func (s *Service) Distribution(ctx context.Context, wc workspace.Context, r Request) (Distribution, error) {
	t, h, err := s.filtered(ctx, wc, r)
	if err != nil {
		return Distribution{}, err
	}
	group := r.Group
	if group == "" {
		group = "assignee"
	}
	var sql string
	var args []any
	var options map[string]string // a select field's option names by id
	switch {
	case group == "assignee":
		sql = `SELECT i.id, coalesce(u.id::text, ''), coalesce(u.name, '') FROM issues i LEFT JOIN users u ON u.id = i.assignee_id`
	case group == "type":
		sql = `SELECT i.id, t.id::text, t.name FROM issues i JOIN issue_types t ON t.id = i.type_id`
	case group == "priority":
		sql = `SELECT i.id, i.priority, i.priority FROM issues i`
	case group == "label":
		sql = `SELECT i.id, coalesce(l.id::text, ''), coalesce(l.name, '') FROM issues i LEFT JOIN issue_labels il ON il.issue_id = i.id LEFT JOIN labels l ON l.id = il.label_id`
	case group == "project":
		sql = `SELECT i.id, p.key, p.name FROM issues i JOIN projects p ON p.id = i.project_id`
	case strings.HasPrefix(group, "cf:"):
		field, err := uuid.Parse(strings.TrimPrefix(group, "cf:"))
		if err != nil {
			return Distribution{}, apperr.Invalid(apperr.Field{Field: "group", Code: "invalid"})
		}
		var typ string
		var raw json.RawMessage
		if err := s.pool.QueryRow(ctx, `SELECT type, options FROM custom_fields WHERE workspace_id = $1 AND id = $2`, wc.Workspace.ID, field).Scan(&typ, &raw); err != nil || typ != "select" {
			return Distribution{}, apperr.Invalid(apperr.Field{Field: "group", Code: "invalid"})
		}
		var opts struct {
			Options []struct{ ID, Name string } `json:"options"`
		}
		_ = json.Unmarshal(raw, &opts)
		options = map[string]string{}
		for _, o := range opts.Options {
			options[o.ID] = o.Name
		}
		sql = `SELECT i.id, coalesce(i.fields->>$3, ''), '' FROM issues i`
		args = append(args, field.String())
	default:
		return Distribution{}, apperr.Invalid(apperr.Field{Field: "group", Code: "invalid"})
	}
	rows, err := s.pool.Query(ctx, sql+` WHERE i.workspace_id = $1 AND i.id = ANY($2::uuid[]) AND i.archived_at IS NULL`,
		append([]any{wc.Workspace.ID, t.issues}, args...)...)
	if err != nil {
		return Distribution{}, err
	}
	defer rows.Close()
	now := s.now()
	sums := map[string]*Group{}
	var order []string
	for rows.Next() {
		var id uuid.UUID
		var key, name string
		if err := rows.Scan(&id, &key, &name); err != nil {
			return Distribution{}, err
		}
		if o := h.outcome(id, now); o != "unstarted" && o != "started" {
			continue
		}
		if options != nil {
			name = options[key]
		}
		g := sums[key]
		if g == nil {
			g = &Group{Key: key, Name: name}
			sums[key] = g
			order = append(order, key)
		}
		g.Value += h.measure(t, id, now)
	}
	if err := rows.Err(); err != nil {
		return Distribution{}, err
	}
	out := Distribution{Meta: t.meta, Group: group, Groups: []Group{}}
	for _, k := range order {
		out.Groups = append(out.Groups, *sums[k])
	}
	slices.SortStableFunc(out.Groups, func(a, b Group) int {
		switch {
		case a.Value > b.Value:
			return -1
		case a.Value < b.Value:
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out, nil
}
