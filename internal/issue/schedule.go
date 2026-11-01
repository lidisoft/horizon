package issue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/calendar"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/issue/db"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Rescheduling (docs/frontend.md#timeline): moving an issue's dates proposes
// moving the issues it precedes, in working days of each one's project,
// and nothing moves until the person confirms.

// maxMoves bounds a plan; more successors than that is a 422.
const maxMoves = 200

// Span is an issue's dates.
type Span struct {
	StartDate *string `json:"start_date"`
	DueDate   *string `json:"due_date"`
}

// Move is an issue a plan reschedules.
type Move struct {
	Key   string `json:"key" example:"HZN-7"`
	Title string `json:"title"`
	From  Span   `json:"from"`
	To    Span   `json:"to"`
	// Shift is how many working days later it starts.
	Shift int `json:"shift"`
	// After is the issue whose new dates push it.
	After string `json:"after" example:"HZN-6"`
	// Blocked says why it can't move: forbidden (no issue.edit), archived.
	// Its own successors are left as they are.
	Blocked string `json:"blocked,omitempty" enums:"forbidden,archived"`
	// Result, once applied: moved, or the error code that stopped it.
	Result string `json:"result,omitempty" example:"moved"`
	// Version is the issue's version the plan was made from.
	Version int32 `json:"version"`
}

// SchedulePlan is what changing an issue's dates does.
type SchedulePlan struct {
	Issue Move   `json:"issue"`
	Moves []Move `json:"moves"`
	// Applied is true when the plan was carried out (confirm).
	Applied bool `json:"applied"`
}

// node is an issue in a plan.
type node struct {
	issue      db.Issue
	key        string
	cal        calendar.Calendar
	start, due time.Time // the span used: a lone date is both ends
	hasStart   bool
	hasDue     bool
}

// planner computes moves, with a calendar per project.
type planner struct {
	s       *Service
	wc      workspace.Context
	cals    map[uuid.UUID]calendar.Calendar
	keys    map[uuid.UUID]string
	canEdit map[uuid.UUID]bool
	visible map[uuid.UUID]bool
}

func (s *Service) planner(ctx context.Context, wc workspace.Context) (*planner, error) {
	p := &planner{s: s, wc: wc, cals: map[uuid.UUID]calendar.Calendar{}, keys: map[uuid.UUID]string{}, canEdit: map[uuid.UUID]bool{}, visible: map[uuid.UUID]bool{}}
	projects, err := s.queryProjects(ctx, wc, nil)
	if err != nil {
		return nil, err
	}
	for _, pr := range projects {
		p.visible[pr.ID] = true
		p.keys[pr.ID] = pr.Key
	}
	return p, nil
}

func (p *planner) calendar(ctx context.Context, projectID uuid.UUID) (calendar.Calendar, error) {
	if c, ok := p.cals[projectID]; ok {
		return c, nil
	}
	settings, err := p.s.config.CalendarSettings(ctx, p.s.pool, configdef.Target{
		Principal: p.wc.Principal, WorkspaceID: p.wc.Workspace.ID, ProjectID: uuid.NullUUID{UUID: projectID, Valid: true},
	})
	if err != nil {
		return calendar.Calendar{}, err
	}
	c := settings.Calendar()
	p.cals[projectID] = c
	return c, nil
}

func (p *planner) editable(ctx context.Context, projectID uuid.UUID) (bool, error) {
	if ok, seen := p.canEdit[projectID]; seen {
		return ok, nil
	}
	ok, err := p.s.access.Can(ctx, p.wc.Principal, access.Scope{WorkspaceID: p.wc.Workspace.ID, ProjectID: uuid.NullUUID{UUID: projectID, Valid: true}}, access.IssueEdit)
	if err != nil {
		return false, err
	}
	p.canEdit[projectID] = ok
	return ok, nil
}

// node loads an issue the caller sees; ok is false for others and for
// issues without dates (nothing to move).
func (p *planner) node(ctx context.Context, i db.Issue) (node, bool, error) {
	if !p.visible[i.ProjectID] || (!i.StartDate.Valid && !i.DueDate.Valid) {
		return node{}, false, nil
	}
	c, err := p.calendar(ctx, i.ProjectID)
	if err != nil {
		return node{}, false, err
	}
	n := node{issue: i, key: keyOf(p.keys[i.ProjectID], i.Number), cal: c, hasStart: i.StartDate.Valid, hasDue: i.DueDate.Valid}
	n.start, n.due = i.StartDate.Time, i.DueDate.Time
	if !n.hasStart {
		n.start = n.due
	}
	if !n.hasDue {
		n.due = n.start
	}
	return n, true, nil
}

func spanOf(start, due time.Time, hasStart, hasDue bool) Span {
	var out Span
	if hasStart {
		s := start.Format(time.DateOnly)
		out.StartDate = &s
	}
	if hasDue {
		d := due.Format(time.DateOnly)
		out.DueDate = &d
	}
	return out
}

// successors are the issues id precedes, with the lags.
func (s *Service) successors(ctx context.Context, workspaceID, id uuid.UUID) ([]db.Issue, []int, error) {
	rows, err := s.pool.Query(ctx, `SELECT target_id, coalesce(lag_days, 0) FROM issue_links WHERE workspace_id = $1 AND source_id = $2 AND type = 'precedes'`, workspaceID, id)
	if err != nil {
		return nil, nil, err
	}
	var ids []uuid.UUID
	var lags []int
	for rows.Next() {
		var t uuid.UUID
		var lag int
		if err := rows.Scan(&t, &lag); err != nil {
			rows.Close()
			return nil, nil, err
		}
		ids, lags = append(ids, t), append(lags, lag)
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, nil, rows.Err()
	}
	out := make([]db.Issue, 0, len(ids))
	keep := lags[:0]
	q := db.New(s.pool)
	for k, t := range ids {
		i, err := q.IssueByID(ctx, db.IssueByIDParams{WorkspaceID: workspaceID, ID: t})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		out, keep = append(out, i), append(keep, lags[k])
	}
	return out, keep, nil
}

// Schedule plans new dates for the issue ("" clears one) and the moves of
// the issues it precedes: a successor that would start before the working
// day after its predecessor's due date (plus the lag) starts then instead,
// keeping its length in working days. Successors only move later. With
// confirm, the plan is applied, each issue at the version it was planned
// from; an issue changed meanwhile is left as it is (result
// version_conflict).
func (s *Service) Schedule(ctx context.Context, ic Context, version int32, startDate, dueDate string, confirm bool) (SchedulePlan, error) {
	if err := s.require(ctx, ic.Context, access.IssueEdit); err != nil {
		return SchedulePlan{}, err
	}
	var fields apperr.Fields
	start, okStart := parseDate(startDate)
	due, okDue := parseDate(dueDate)
	if !okStart {
		fields.Add("start_date", "invalid")
	}
	if !okDue {
		fields.Add("due_date", "invalid")
	}
	checkDates(&fields, start, due)
	if err := fields.Err(); err != nil {
		return SchedulePlan{}, err
	}
	if version != 0 && ic.Issue.Version != version {
		return SchedulePlan{}, apperr.Conflict("version_conflict")
	}
	p, err := s.planner(ctx, ic.Context.Context)
	if err != nil {
		return SchedulePlan{}, err
	}
	head := ic.Issue
	plan := SchedulePlan{Issue: Move{Key: ic.key(), Title: head.Title, Version: head.Version,
		From: spanOf(head.StartDate.Time, head.DueDate.Time, head.StartDate.Valid, head.DueDate.Valid),
		To:   spanOf(start.Time, due.Time, start.Valid, due.Valid)}, Moves: []Move{}}

	// Relax the successors breadth first: each keeps the latest start any
	// predecessor pushes it to.
	type placed struct {
		n          node
		start, due time.Time
		after      string
		blocked    string
	}
	moved := map[uuid.UUID]*placed{}
	order := []uuid.UUID{}
	end := due.Time
	if !due.Valid {
		end = start.Time
	}
	type item struct {
		id  uuid.UUID
		key string
		due time.Time
	}
	queue := []item{}
	if start.Valid || due.Valid {
		queue = append(queue, item{head.ID, plan.Issue.Key, end})
	}
	for steps := 0; len(queue) > 0; steps++ {
		if steps > maxMoves*4 {
			return SchedulePlan{}, apperr.Invalid(apperr.Field{Field: "links", Code: "cycle"})
		}
		cur := queue[0]
		queue = queue[1:]
		next, lags, err := s.successors(ctx, ic.Workspace.ID, cur.id)
		if err != nil {
			return SchedulePlan{}, err
		}
		for k, i := range next {
			if i.ID == head.ID {
				continue // a cycle back to the issue moved
			}
			n, ok, err := p.node(ctx, i)
			if err != nil {
				return SchedulePlan{}, err
			}
			if !ok {
				continue
			}
			earliest := n.cal.Next(n.cal.Add(cur.due, 1+lags[k]), 1)
			from := n.start
			if prev := moved[i.ID]; prev != nil {
				from = prev.start
			}
			if !from.Before(earliest) {
				continue
			}
			length := max(n.cal.Between(n.start, n.due), 1)
			pl := &placed{n: n, start: earliest, due: n.cal.Add(earliest, length-1), after: cur.key}
			if moved[i.ID] == nil {
				order = append(order, i.ID)
				if len(order) > maxMoves {
					return SchedulePlan{}, apperr.Invalid(apperr.Field{Field: "links", Code: "too_many"})
				}
			}
			moved[i.ID] = pl
			switch ok, err := p.editable(ctx, i.ProjectID); {
			case err != nil:
				return SchedulePlan{}, err
			case !ok:
				pl.blocked = "forbidden"
			case i.ArchivedAt != nil:
				pl.blocked = "archived"
			}
			if pl.blocked == "" {
				queue = append(queue, item{i.ID, n.key, pl.due})
			}
		}
	}
	for _, id := range order {
		pl := moved[id]
		n := pl.n
		plan.Moves = append(plan.Moves, Move{
			Key: n.key, Title: n.issue.Title, Version: n.issue.Version, After: pl.after, Blocked: pl.blocked,
			From:  spanOf(n.issue.StartDate.Time, n.issue.DueDate.Time, n.hasStart, n.hasDue),
			To:    spanOf(pl.start, pl.due, n.hasStart, n.hasDue || !n.hasStart),
			Shift: n.cal.Between(n.start, pl.start) - 1,
		})
	}
	if !confirm {
		return plan, nil
	}

	// Apply: the issue first; then each successor at its planned version.
	if _, err := s.Update(ctx, ic, version, Update{StartDate: &startDate, DueDate: &dueDate}); err != nil {
		return SchedulePlan{}, err
	}
	plan.Issue.Result = "moved"
	for k := range plan.Moves {
		m := &plan.Moves[k]
		if m.Blocked != "" {
			m.Result = m.Blocked
			continue
		}
		mic, err := s.Enter(ctx, ic.Context.Context, m.Key)
		if err != nil {
			m.Result = "issue_not_found"
			continue
		}
		in := Update{StartDate: m.To.StartDate, DueDate: m.To.DueDate}
		if _, err := s.Update(ctx, mic, m.Version, in); err != nil {
			var ae *apperr.Error
			if errors.As(err, &ae) {
				m.Result = ae.Code
				continue
			}
			return SchedulePlan{}, err
		}
		m.Result = "moved"
	}
	plan.Applied = true
	return plan, nil
}

// Dependency is a precedes link between two issues.
type Dependency struct {
	ID      uuid.UUID `json:"id"`
	From    string    `json:"from" example:"HZN-6"`
	To      string    `json:"to" example:"HZN-7"`
	LagDays int32     `json:"lag_days"`
}

// Dependencies lists the precedes links touching a project's issues whose
// both ends the caller sees, for the timeline.
func (s *Service) Dependencies(ctx context.Context, wc workspace.Context, projectID uuid.UUID) ([]Dependency, error) {
	p, err := s.planner(ctx, wc)
	if err != nil {
		return nil, err
	}
	if !p.visible[projectID] {
		return nil, apperr.NotFound("project_not_found")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT l.id, a.project_id, a.number, b.project_id, b.number, coalesce(l.lag_days, 0)
		FROM issue_links l JOIN issues a ON a.id = l.source_id JOIN issues b ON b.id = l.target_id
		WHERE l.workspace_id = $1 AND l.type = 'precedes' AND (a.project_id = $2 OR b.project_id = $2)
		ORDER BY l.created_at`, wc.Workspace.ID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Dependency{}
	for rows.Next() {
		var d Dependency
		var pa, pb uuid.UUID
		var na, nb int64
		if err := rows.Scan(&d.ID, &pa, &na, &pb, &nb, &d.LagDays); err != nil {
			return nil, err
		}
		if !p.visible[pa] || !p.visible[pb] {
			continue
		}
		d.From, d.To = keyOf(p.keys[pa], na), keyOf(p.keys[pb], nb)
		out = append(out, d)
	}
	return out, rows.Err()
}
