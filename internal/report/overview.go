package report

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/query"
)

// The project overview (docs/reports.md#project-overview-page): every
// block in one response.

// SprintBlock is the active iteration.
type SprintBlock struct {
	Iteration IterationRef `json:"iteration"`
	Goal      string       `json:"goal"`
	// DaysLeft counts the working days from today to the end date.
	DaysLeft int `json:"days_left"`
	// Remaining and Ideal are its burndown, a value a day.
	Remaining []*float64 `json:"remaining"`
	Ideal     []float64  `json:"ideal"`
	// ByOutcome is its issues now, by outcome.
	ByOutcome map[string]float64 `json:"by_outcome"`
}

// TargetBlock is an upcoming release or milestone.
type TargetBlock struct {
	ID        uuid.UUID          `json:"id"`
	Name      string             `json:"name"`
	Date      *string            `json:"date"`
	ByOutcome map[string]float64 `json:"by_outcome"`
}

// WIPStatus counts the issues in a started status.
type WIPStatus struct {
	Name  string `json:"name" example:"In review"`
	Count int    `json:"count"`
}

// FlowBlock is the project's flow health.
type FlowBlock struct {
	// Throughput is the issues completed in each of the last 4 weeks.
	Weeks      []string    `json:"weeks"`
	Throughput []*float64  `json:"throughput"`
	Cycle      Percentiles `json:"cycle"`
	WIP        []WIPStatus `json:"wip"`
}

// Overview is a project's overview page.
type Overview struct {
	Sprint        *SprintBlock        `json:"sprint"`
	NextRelease   *TargetBlock        `json:"next_release"`
	NextMilestone *TargetBlock        `json:"next_milestone"`
	Flow          FlowBlock           `json:"flow"`
	Activity      []issue.PublicEvent `json:"activity"`
	// MyWork is the viewer's open issues in the project, in rank order.
	MyWork []issue.Issue `json:"my_work"`
}

// Overview gathers the project's overview, in issue count.
func (s *Service) Overview(ctx context.Context, pc project.Context) (Overview, error) {
	wc := pc.Context
	if _, err := s.projectContext(ctx, wc, pc.Project.ID); err != nil {
		return Overview{}, err
	}
	var out Overview
	only := Request{Projects: []string{pc.Project.Key}}
	now := s.now()

	// The active iteration (the earliest started, when several are).
	var active uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT id FROM iterations WHERE workspace_id = $1 AND project_id = $2 AND state = 'active'
		ORDER BY started_at LIMIT 1`, wc.Workspace.ID, pc.Project.ID).Scan(&active)
	if err == nil {
		r := only
		r.Iteration = uuid.NullUUID{UUID: active, Valid: true}
		bd, err := s.Burndown(ctx, wc, r)
		if err != nil {
			return Overview{}, err
		}
		sp, err := s.sprint(ctx, wc, r)
		if err != nil {
			return Overview{}, err
		}
		block := &SprintBlock{Iteration: bd.Iteration, Goal: sp.it.Goal, Remaining: bd.Remaining, Ideal: bd.Ideal,
			ByOutcome: map[string]float64{"completed": 0, "started": 0, "unstarted": 0, "abandoned": 0}}
		for _, id := range sp.in(now) {
			if o := sp.h.outcome(id, now); o != "" {
				block.ByOutcome[o]++
			}
		}
		if sp.it.EndDate != nil {
			if end, err := time.Parse(time.DateOnly, *sp.it.EndDate); err == nil {
				block.DaysLeft = sp.t.cal.Between(sp.t.cal.Today(now), end)
			}
		}
		out.Sprint = block
	}

	// The next release and milestone: the soonest dated, else the oldest undated.
	var release, milestone uuid.UUID
	_ = s.pool.QueryRow(ctx, `SELECT id FROM releases WHERE workspace_id = $1 AND project_id = $2 AND state = 'unreleased'
		ORDER BY release_date NULLS LAST, created_at LIMIT 1`, wc.Workspace.ID, pc.Project.ID).Scan(&release)
	_ = s.pool.QueryRow(ctx, `SELECT id FROM milestones WHERE workspace_id = $1 AND (project_id = $2 OR project_id IS NULL) AND date >= $3::date
		ORDER BY date, created_at LIMIT 1`, wc.Workspace.ID, pc.Project.ID, now.Format(time.DateOnly)).Scan(&milestone)
	if release != uuid.Nil {
		r := only
		r.Release = uuid.NullUUID{UUID: release, Valid: true}
		p, err := s.Progress(ctx, wc, r)
		if err != nil {
			return Overview{}, err
		}
		out.NextRelease = &TargetBlock{ID: release, Name: p.Name, Date: p.Date, ByOutcome: p.ByOutcome}
	}
	if milestone != uuid.Nil {
		r := only
		r.Milestone = uuid.NullUUID{UUID: milestone, Valid: true}
		p, err := s.Progress(ctx, wc, r)
		if err == nil {
			out.NextMilestone = &TargetBlock{ID: milestone, Name: p.Name, Date: p.Date, ByOutcome: p.ByOutcome}
		}
	}

	// Flow health: four weeks of throughput, 90 days of cycle time, WIP now.
	tp, err := s.Throughput(ctx, wc, Request{Projects: only.Projects, From: now.AddDate(0, 0, -21).Format(time.DateOnly)})
	if err != nil {
		return Overview{}, err
	}
	out.Flow.Weeks, out.Flow.Throughput = tp.Periods, tp.Series[0].Values
	ct, err := s.CycleTime(ctx, wc, only)
	if err != nil {
		return Overview{}, err
	}
	out.Flow.Cycle = ct.Cycle
	rows, err := s.pool.Query(ctx, `SELECT s.name, count(*) FROM issues i JOIN statuses s ON s.id = i.status_id JOIN status_categories sc ON sc.id = s.category_id
		WHERE i.workspace_id = $1 AND i.project_id = $2 AND i.archived_at IS NULL AND sc.outcome = 'started'
		GROUP BY s.name ORDER BY min(sc.position), min(s.position)`, wc.Workspace.ID, pc.Project.ID)
	if err != nil {
		return Overview{}, err
	}
	out.Flow.WIP = []WIPStatus{}
	for rows.Next() {
		var w WIPStatus
		if err := rows.Scan(&w.Name, &w.Count); err != nil {
			rows.Close()
			return Overview{}, err
		}
		out.Flow.WIP = append(out.Flow.WIP, w)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Overview{}, err
	}

	if out.Activity, err = s.issues.ProjectActivity(ctx, pc, 10); err != nil {
		return Overview{}, err
	}
	mine := query.Query{Filter: &query.Node{And: []*query.Node{
		{Field: "assignee", Op: "eq", Value: json.RawMessage(`"@me"`)},
		{Field: "status.outcome", Op: "eq", Value: json.RawMessage(`"open"`)},
	}}, Sort: []query.Sort{{Field: "rank"}}, Limit: 10}
	res, err := s.issues.Query(ctx, wc, mine, only.Projects, "")
	if err != nil {
		return Overview{}, err
	}
	out.MyWork = res.Items
	return out, nil
}
