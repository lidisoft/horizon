package report

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Release and milestone progress (docs/reports.md#release-and-milestone-progress):
// the issues targeted now, by outcome, and a burnup of them over time. Who
// was targeted when isn't projected: the burnup counts today's issues from
// their creation.

// Progress is a release's or milestone's progress.
type Progress struct {
	Meta
	// Name and Date (the release or milestone date) say what it is about.
	Name string  `json:"name"`
	Date *string `json:"date"`
	// ByOutcome is the scope now, by outcome.
	ByOutcome map[string]float64 `json:"by_outcome"`
	Days      []string           `json:"days"`
	// Scope is every issue but the abandoned ones, Completed the completed.
	Scope     []*float64 `json:"scope"`
	Completed []*float64 `json:"completed"`
}

// Progress reports the release (r.Release) or milestone (r.Milestone).
func (s *Service) Progress(ctx context.Context, wc workspace.Context, r Request) (Progress, error) {
	var out Progress
	var start *string
	var projects map[uuid.UUID]string
	var ids []uuid.UUID
	switch {
	case r.Release.Valid:
		var projectID uuid.UUID
		err := s.pool.QueryRow(ctx, `SELECT project_id FROM releases WHERE workspace_id = $1 AND id = $2`, wc.Workspace.ID, r.Release.UUID).Scan(&projectID)
		if errors.Is(err, pgx.ErrNoRows) {
			return out, apperr.NotFound("release_not_found")
		}
		if err != nil {
			return out, err
		}
		pc, err := s.projectContext(ctx, wc, projectID)
		if err != nil {
			return out, apperr.NotFound("release_not_found")
		}
		rel, err := s.plans.Release(ctx, pc, r.Release.UUID)
		if err != nil {
			return out, err
		}
		out.Name, out.Date, start = rel.Name, rel.ReleaseDate, rel.StartDate
		projects = map[uuid.UUID]string{pc.Project.ID: pc.Project.Key}
		rows, err := s.pool.Query(ctx, `SELECT id FROM issues WHERE workspace_id = $1 AND release_id = $2`, wc.Workspace.ID, rel.ID)
		if err != nil {
			return out, err
		}
		if ids, err = pgx.CollectRows(rows, pgx.RowTo[uuid.UUID]); err != nil {
			return out, err
		}
	case r.Milestone.Valid:
		m, err := s.plans.Milestone(ctx, wc, r.Milestone.UUID)
		if err != nil {
			return out, err
		}
		out.Name, out.Date = m.Name, &m.Date
		visible, err := s.issues.VisibleProjects(ctx, wc)
		if err != nil {
			return out, err
		}
		// Only the linked issues the reader sees count.
		rows, err := s.pool.Query(ctx, `SELECT i.id, i.project_id FROM milestone_issues mi JOIN issues i ON i.id = mi.issue_id WHERE mi.milestone_id = $1`, m.ID)
		if err != nil {
			return out, err
		}
		projects = map[uuid.UUID]string{}
		for rows.Next() {
			var id, p uuid.UUID
			if err := rows.Scan(&id, &p); err != nil {
				rows.Close()
				return out, err
			}
			if key, ok := visible[p]; ok {
				ids = append(ids, id)
				projects[p] = key
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return out, err
		}
		if m.ProjectID.Valid {
			if key, ok := visible[m.ProjectID.UUID]; ok {
				projects[m.ProjectID.UUID] = key
			}
		}
	default:
		return out, apperr.Invalid(apperr.Field{Field: "release", Code: "required"})
	}
	t, err := s.measure(ctx, wc, r, projects, ids)
	if err != nil {
		return out, err
	}
	h, err := s.history(ctx, ids)
	if err != nil {
		return out, err
	}
	out.Meta = t.meta
	now := s.now()
	out.ByOutcome = map[string]float64{"unstarted": 0, "started": 0, "completed": 0, "abandoned": 0}
	first := t.cal.Today(now)
	for _, id := range ids {
		if o := h.outcome(id, now); o != "" {
			out.ByOutcome[o] += h.measure(t, id, now)
		}
		if c, ok := h.created[id]; ok && t.cal.Today(c).Before(first) {
			first = t.cal.Today(c)
		}
	}
	if start != nil {
		if d, err := time.Parse(time.DateOnly, *start); err == nil {
			first = d
		}
	}
	from, to, err := parseRange(r, first, t.cal.Today(now))
	if err != nil {
		return out, err
	}
	out.Days, out.Scope, out.Completed = []string{}, []*float64{}, []*float64{}
	for _, d := range s.days(t.cal, from, to) {
		var scope, completed float64
		for _, id := range ids {
			switch h.outcome(id, d.at) {
			case "completed":
				completed += h.measure(t, id, d.at)
				scope += h.measure(t, id, d.at)
			case "unstarted", "started":
				scope += h.measure(t, id, d.at)
			}
		}
		out.Days = append(out.Days, d.Date)
		out.Scope = append(out.Scope, num(scope, d.future))
		out.Completed = append(out.Completed, num(completed, d.future))
	}
	return out, nil
}
