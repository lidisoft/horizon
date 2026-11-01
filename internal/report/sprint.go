package report

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/planning"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Sprint reports (docs/reports.md#sprint-reports-per-iteration).

// IterationRef names the iteration a report is about.
type IterationRef struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	State       string     `json:"state"`
	StartDate   *string    `json:"start_date"`
	EndDate     *string    `json:"end_date"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// SprintSeries is a burndown or burnup: one value a day, null for days to
// come.
type SprintSeries struct {
	Meta
	Iteration IterationRef `json:"iteration"`
	Days      []string     `json:"days"`
	// Working says which days are worked (the ideal line is flat on others).
	Working []bool `json:"working"`
	// Remaining is the open scope (neither completed nor abandoned).
	Remaining []*float64 `json:"remaining"`
	// Ideal goes from the scope committed at the start to nothing at the end.
	Ideal []float64 `json:"ideal"`
	// Scope is everything in the iteration but what was abandoned.
	Scope     []*float64 `json:"scope"`
	Completed []*float64 `json:"completed"`
}

// sprint is an iteration with what the sprint reports need.
type sprint struct {
	it      planning.Iteration
	t       *target
	members []membership
	h       *history
}

func (s *Service) sprint(ctx context.Context, wc workspace.Context, r Request) (*sprint, error) {
	if !r.Iteration.Valid {
		return nil, apperr.Invalid(apperr.Field{Field: "iteration", Code: "required"})
	}
	var projectID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT project_id FROM iterations WHERE workspace_id = $1 AND id = $2`, wc.Workspace.ID, r.Iteration.UUID).Scan(&projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("iteration_not_found")
	}
	if err != nil {
		return nil, err
	}
	pc, err := s.projectContext(ctx, wc, projectID)
	if err != nil {
		return nil, apperr.NotFound("iteration_not_found")
	}
	it, err := s.plans.Iteration(ctx, pc, r.Iteration.UUID)
	if err != nil {
		return nil, err
	}
	sp := &sprint{it: it}
	rows, err := s.pool.Query(ctx, `SELECT issue_id, added_at, removed_at FROM report_iteration_scope WHERE iteration_id = $1 ORDER BY added_at, id`, it.ID)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	seen := map[uuid.UUID]bool{}
	for rows.Next() {
		var m membership
		var to *time.Time
		if err := rows.Scan(&m.issue, &m.from, &to); err != nil {
			rows.Close()
			return nil, err
		}
		if to != nil {
			m.to = *to
		}
		sp.members = append(sp.members, m)
		if !seen[m.issue] {
			seen[m.issue] = true
			ids = append(ids, m.issue)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if sp.t, err = s.measure(ctx, wc, r, map[uuid.UUID]string{pc.Project.ID: pc.Project.Key}, ids); err != nil {
		return nil, err
	}
	sp.h, err = s.history(ctx, ids)
	return sp, err
}

func (sp *sprint) ref() IterationRef {
	return IterationRef{ID: sp.it.ID, Name: sp.it.Name, State: sp.it.State, StartDate: sp.it.StartDate, EndDate: sp.it.EndDate,
		StartedAt: sp.it.StartedAt, CompletedAt: sp.it.CompletedAt}
}

// in lists the issues in the iteration at.
func (sp *sprint) in(at time.Time) []uuid.UUID {
	var out []uuid.UUID
	seen := map[uuid.UUID]bool{}
	for _, m := range sp.members {
		if covers(m.from, m.to, at) && !seen[m.issue] {
			seen[m.issue] = true
			out = append(out, m.issue)
		}
	}
	return out
}

// totals sums the issues in the iteration at, by how they stand.
func (sp *sprint) totals(at time.Time) (remaining, scope, completed float64) {
	for _, id := range sp.in(at) {
		v := sp.h.measure(sp.t, id, at)
		switch sp.h.outcome(id, at) {
		case "completed":
			completed += v
			scope += v
		case "abandoned", "":
		default:
			remaining += v
			scope += v
		}
	}
	return remaining, scope, completed
}

// Burndown and Burnup share their series: from the start date to the end
// date, read at each day's end (now for today; after completion, the
// completion).
func (s *Service) Burndown(ctx context.Context, wc workspace.Context, r Request) (SprintSeries, error) {
	sp, err := s.sprint(ctx, wc, r)
	if err != nil {
		return SprintSeries{}, err
	}
	out := SprintSeries{Meta: sp.t.meta, Iteration: sp.ref(), Days: []string{}, Working: []bool{}, Remaining: []*float64{}, Ideal: []float64{},
		Scope: []*float64{}, Completed: []*float64{}}
	if sp.it.StartDate == nil || sp.it.EndDate == nil {
		return out, nil
	}
	start, _ := time.Parse(time.DateOnly, *sp.it.StartDate)
	end, _ := time.Parse(time.DateOnly, *sp.it.EndDate)
	cal := sp.t.cal
	// The ideal: the committed open scope, down by an equal share each working day.
	var committed float64
	if sp.it.StartedAt != nil {
		committed, _, _ = sp.totals(*sp.it.StartedAt)
	}
	working := cal.Between(start, end)
	worked := 0
	for _, d := range s.days(cal, start, end) {
		date, _ := time.Parse(time.DateOnly, d.Date)
		w := cal.Working(date)
		if w {
			worked++
		}
		out.Days = append(out.Days, d.Date)
		out.Working = append(out.Working, w)
		ideal := 0.0
		if working > 0 {
			ideal = committed * float64(working-worked) / float64(working)
		}
		out.Ideal = append(out.Ideal, ideal)
		at := d.at
		future := d.future || sp.it.StartedAt == nil || at.Before(*sp.it.StartedAt)
		if sp.it.CompletedAt != nil && at.After(*sp.it.CompletedAt) {
			at = *sp.it.CompletedAt
			future = future || date.After(cal.Today(*sp.it.CompletedAt))
		}
		remaining, scope, completed := sp.totals(at)
		out.Remaining = append(out.Remaining, num(remaining, future))
		out.Scope = append(out.Scope, num(scope, future))
		out.Completed = append(out.Completed, num(completed, future))
	}
	return out, nil
}

// SprintItem is an issue of a sprint report.
type SprintItem struct {
	IssueRef
	// Committed: in the iteration when it started. Added: after. Removed:
	// taken out before the end.
	Committed bool `json:"committed"`
	Added     bool `json:"added"`
	Removed   bool `json:"removed"`
	// Ended is how it ended: completed, abandoned, rolled_over (unfinished
	// at completion), removed, or open while the iteration runs.
	Ended string `json:"ended" enums:"completed,abandoned,rolled_over,removed,open"`
	// Value is what it counts for at the end (at the start for removed ones).
	Value float64 `json:"value"`
}

// SprintTotals sums a sprint report in its metric.
type SprintTotals struct {
	Committed  float64 `json:"committed"`
	Added      float64 `json:"added"`
	Removed    float64 `json:"removed"`
	Completed  float64 `json:"completed"`
	Abandoned  float64 `json:"abandoned"`
	RolledOver float64 `json:"rolled_over"`
	Open       float64 `json:"open"`
}

// SprintReport is what happened to an iteration's scope.
type SprintReport struct {
	Meta
	Iteration IterationRef `json:"iteration"`
	Items     []SprintItem `json:"items"`
	Totals    SprintTotals `json:"totals"`
}

// Sprint reports an iteration's committed, added and removed issues and
// how each ended. Before it starts, everything in it is committed.
func (s *Service) Sprint(ctx context.Context, wc workspace.Context, r Request) (SprintReport, error) {
	sp, err := s.sprint(ctx, wc, r)
	if err != nil {
		return SprintReport{}, err
	}
	out := SprintReport{Meta: sp.t.meta, Iteration: sp.ref(), Items: []SprintItem{}}
	now := s.now()
	start, end := now, now
	if sp.it.StartedAt != nil {
		start = *sp.it.StartedAt
	}
	if sp.it.CompletedAt != nil {
		end = *sp.it.CompletedAt
	}
	refs, err := s.refs(ctx, wc.Workspace.ID, sp.t.issues)
	if err != nil {
		return SprintReport{}, err
	}
	committed := map[uuid.UUID]bool{}
	for _, id := range sp.in(start) {
		committed[id] = true
	}
	atEnd := map[uuid.UUID]bool{}
	for _, id := range sp.in(end) {
		atEnd[id] = true
	}
	for _, id := range sp.t.issues {
		ref, ok := refs[id]
		if !ok {
			continue // deleted since
		}
		// Only issues there at some point while it ran (or now, before it starts).
		there := committed[id] || atEnd[id]
		for _, m := range sp.members {
			if m.issue == id && m.from.Before(end) && (m.to.IsZero() || m.to.After(start)) {
				there = true
			}
		}
		if !there {
			continue
		}
		item := SprintItem{IssueRef: ref, Committed: committed[id], Added: !committed[id], Removed: !atEnd[id]}
		switch {
		case item.Removed:
			item.Ended, item.Value = "removed", sp.h.measure(sp.t, id, start)
		default:
			item.Value = sp.h.measure(sp.t, id, end)
			switch o := sp.h.outcome(id, end); {
			case o == "completed" || o == "abandoned":
				item.Ended = o
			case sp.it.CompletedAt != nil:
				item.Ended = "rolled_over"
			default:
				item.Ended = "open"
			}
		}
		out.Items = append(out.Items, item)
		t := &out.Totals
		if item.Committed {
			t.Committed += sp.h.measure(sp.t, id, start)
		} else {
			t.Added += item.Value
		}
		switch item.Ended {
		case "removed":
			t.Removed += item.Value
		case "completed":
			t.Completed += item.Value
		case "abandoned":
			t.Abandoned += item.Value
		case "rolled_over":
			t.RolledOver += item.Value
		default:
			t.Open += item.Value
		}
	}
	return out, nil
}

// VelocityPoint is one completed iteration.
type VelocityPoint struct {
	Iteration IterationRef `json:"iteration"`
	Committed float64      `json:"committed"`
	Completed float64      `json:"completed"`
}

// Velocity is committed versus completed over the last iterations.
type Velocity struct {
	Meta
	Items []VelocityPoint `json:"items"`
	// Average is the mean completed.
	Average float64 `json:"average"`
}

// Velocity reports the project's last completed iterations (r.Last,
// default 7): committed at the start and completed at the end.
func (s *Service) Velocity(ctx context.Context, wc workspace.Context, r Request) (Velocity, error) {
	if len(r.Projects) != 1 {
		return Velocity{}, apperr.Invalid(apperr.Field{Field: "projects", Code: "one_required"})
	}
	pc, err := s.projects.Enter(ctx, wc, r.Projects[0])
	if err != nil {
		return Velocity{}, err
	}
	if _, err := s.projectContext(ctx, wc, pc.Project.ID); err != nil {
		return Velocity{}, err
	}
	last := r.Last
	if last <= 0 {
		last = 7
	}
	last = min(last, 20)
	rows, err := s.pool.Query(ctx, `SELECT id FROM iterations WHERE workspace_id = $1 AND project_id = $2 AND completed_at IS NOT NULL
		ORDER BY completed_at DESC LIMIT $3`, wc.Workspace.ID, pc.Project.ID, last)
	if err != nil {
		return Velocity{}, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return Velocity{}, err
	}
	out := Velocity{Meta: Meta{Metric: MetricCount}, Items: []VelocityPoint{}}
	for k := len(ids) - 1; k >= 0; k-- {
		rr := r
		rr.Iteration = uuid.NullUUID{UUID: ids[k], Valid: true}
		sp, err := s.sprint(ctx, wc, rr)
		if err != nil {
			return Velocity{}, err
		}
		out.Meta = sp.t.meta
		p := VelocityPoint{Iteration: sp.ref()}
		if sp.it.StartedAt != nil {
			for _, id := range sp.in(*sp.it.StartedAt) {
				p.Committed += sp.h.measure(sp.t, id, *sp.it.StartedAt)
			}
		}
		end := *sp.it.CompletedAt
		for _, id := range sp.in(end) {
			if sp.h.outcome(id, end) == "completed" {
				p.Completed += sp.h.measure(sp.t, id, end)
			}
		}
		out.Items = append(out.Items, p)
		out.Average += p.Completed
	}
	if n := len(out.Items); n > 0 {
		out.Average /= float64(n)
	}
	return out, nil
}
