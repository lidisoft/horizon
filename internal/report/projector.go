// Package report keeps the report projections and answers the reports
// (docs/reports.md, ADR-0021).
package report

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/report/db"
)

// The projector applies the events table to the projections in id order,
// a batch per transaction that also marks the batch reported: each event
// counts exactly once, whatever JetStream redelivers. JetStream (and a
// periodic job) only wake it up; the table is the source, so it catches up
// after downtime and a rebuild replays everything.

const (
	batchSize = 500
	// lockKey serializes batches across worker replicas: events of one issue
	// must apply in order.
	lockKey = 0x68_7a_6e_72 // "hznr"
)

// Projector keeps the projections up to date.
type Projector struct {
	pool *pgxpool.Pool
	log  zerolog.Logger
}

// NewProjector returns a Projector.
func NewProjector(pool *pgxpool.Pool, log zerolog.Logger) *Projector {
	return &Projector{pool: pool, log: log}
}

// Consumer wakes the projector on every published event.
func (p *Projector) Consumer() eventlog.Consumer {
	return eventlog.Consumer{
		Name:   "reports",
		Filter: []string{"*"},
		Handle: func(ctx context.Context, _ eventlog.Event) error {
			_, err := p.CatchUp(ctx)
			return err
		},
	}
}

// CatchUp applies every event not reported yet; it returns how many. When
// another replica holds the lock it returns at once: that one catches up.
func (p *Projector) CatchUp(ctx context.Context) (int, error) {
	total := 0
	for {
		n, err := p.batch(ctx)
		total += n
		if err != nil || n < batchSize {
			return total, err
		}
	}
}

func (p *Projector) batch(ctx context.Context) (int, error) {
	n := 0
	err := postgres.InTx(ctx, p.pool, func(tx pgx.Tx) error {
		var locked bool
		if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, int64(lockKey)).Scan(&locked); err != nil || !locked {
			return err
		}
		q := db.New(tx)
		events, err := q.ClaimUnreported(ctx, batchSize)
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, len(events))
		for i, e := range events {
			ids[i] = e.ID
			if err := apply(ctx, q, e); err != nil {
				return err
			}
		}
		n = len(events)
		if n == 0 {
			return nil
		}
		return q.MarkReported(ctx, ids)
	})
	return n, err
}

// payload is what the projector reads from event payloads.
type payload struct {
	StatusID       uuid.NullUUID   `json:"status_id"`
	To             json.RawMessage `json:"to"`
	CategoryID     uuid.NullUUID   `json:"category_id"`
	Outcome        string          `json:"outcome"`
	ProjectID      uuid.NullUUID   `json:"project_id"`
	ToProjectID    uuid.NullUUID   `json:"to_project_id"`
	EstimateWeight *float64        `json:"estimate_weight"`
	Weight         *float64        `json:"weight"`
	From           json.RawMessage `json:"from"`
}

// id reads a uuid (or null) from a raw value.
func id(raw json.RawMessage) uuid.NullUUID {
	var v uuid.NullUUID
	_ = json.Unmarshal(raw, &v)
	return v
}

// apply projects one event; unknown types change nothing.
func apply(ctx context.Context, q *db.Queries, e db.ClaimUnreportedRow) error {
	if !e.WorkspaceID.Valid {
		return nil
	}
	var pl payload
	_ = json.Unmarshal(e.Payload, &pl)
	ws, at := e.WorkspaceID.UUID, e.OccurredAt
	switch e.AggregateType + "." + e.Type {
	case "issue.IssueCreated":
		project := pl.ProjectID.UUID
		if err := enter(ctx, q, ws, project, e.AggregateID, pl.StatusID.UUID, pl, at); err != nil {
			return err
		}
		if pl.EstimateWeight != nil {
			if err := q.OpenEstimate(ctx, db.OpenEstimateParams{WorkspaceID: ws, IssueID: e.AggregateID, Weight: pl.EstimateWeight, ValidFrom: at}); err != nil {
				return err
			}
		}
		return recompute(ctx, q, ws, e.AggregateID, project, &at)
	case "issue.IssueStatusUpdated":
		project, err := projectOf(ctx, q, e.AggregateID)
		if err != nil || project == uuid.Nil {
			return err
		}
		if err := enter(ctx, q, ws, project, e.AggregateID, id(pl.To).UUID, pl, at); err != nil {
			return err
		}
		return recompute(ctx, q, ws, e.AggregateID, project, nil)
	case "issue.IssueMoved":
		project := pl.ToProjectID.UUID
		if err := enter(ctx, q, ws, project, e.AggregateID, pl.StatusID.UUID, pl, at); err != nil {
			return err
		}
		return recompute(ctx, q, ws, e.AggregateID, project, nil)
	case "issue.IssueEstimated":
		if err := q.CloseEstimate(ctx, db.CloseEstimateParams{At: &at, IssueID: e.AggregateID}); err != nil {
			return err
		}
		return q.OpenEstimate(ctx, db.OpenEstimateParams{WorkspaceID: ws, IssueID: e.AggregateID, Weight: pl.Weight, ValidFrom: at})
	case "issue.IssueIterationChanged":
		if from := id(pl.From); from.Valid {
			if err := q.CloseScope(ctx, db.CloseScopeParams{At: &at, IssueID: e.AggregateID, IterationID: from.UUID}); err != nil {
				return err
			}
		}
		if to := id(pl.To); to.Valid {
			return q.OpenScope(ctx, db.OpenScopeParams{WorkspaceID: ws, IterationID: to.UUID, IssueID: e.AggregateID, AddedAt: at})
		}
		return nil
	case "issue.IssueDeleted":
		for _, forget := range []func(context.Context, uuid.UUID) error{q.ForgetIssueIntervals, q.ForgetIssueMilestones, q.ForgetIssueScope, q.ForgetIssueEstimates} {
			if err := forget(ctx, e.AggregateID); err != nil {
				return err
			}
		}
		return nil
	case "status_category.StatusCategoryUpdated":
		if pl.Outcome == "" {
			return nil
		}
		issues, err := q.SetCategoryOutcome(ctx, db.SetCategoryOutcomeParams{Outcome: pl.Outcome, CategoryID: uuid.NullUUID{UUID: e.AggregateID, Valid: true}})
		if err != nil {
			return err
		}
		seen := map[uuid.UUID]bool{}
		for _, i := range issues {
			if seen[i] {
				continue
			}
			seen[i] = true
			project, err := projectOf(ctx, q, i)
			if err != nil {
				return err
			}
			if err := recompute(ctx, q, ws, i, project, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// enter closes the issue's current stay and opens one in status: its
// category and outcome from the event, else the status's now.
func enter(ctx context.Context, q *db.Queries, ws, project, issue, status uuid.UUID, pl payload, at time.Time) error {
	category, outcome := pl.CategoryID, pl.Outcome
	if outcome == "" {
		f, err := q.StatusFacts(ctx, status)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			outcome = "unstarted" // a status long gone
		case err != nil:
			return err
		default:
			category, outcome = uuid.NullUUID{UUID: f.CategoryID, Valid: true}, f.Outcome
		}
	}
	if err := q.CloseInterval(ctx, db.CloseIntervalParams{At: &at, IssueID: issue}); err != nil {
		return err
	}
	return q.OpenInterval(ctx, db.OpenIntervalParams{WorkspaceID: ws, ProjectID: project, IssueID: issue, StatusID: status,
		CategoryID: category, Outcome: outcome, EnteredAt: at})
}

// projectOf is the issue's project in the projections (its row in issues
// for an issue they don't know); uuid.Nil when it's gone.
func projectOf(ctx context.Context, q *db.Queries, issue uuid.UUID) (uuid.UUID, error) {
	m, err := q.Milestones(ctx, issue)
	if err == nil {
		return m.ProjectID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	p, err := q.IssueProject(ctx, issue)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil
	}
	return p, err
}

// recompute derives the issue's milestones from its stays: first started,
// and completed or abandoned since the start of the run of stays it ends
// in (a reopened issue completes again). created sets the creation time.
func recompute(ctx context.Context, q *db.Queries, ws, issue, project uuid.UUID, created *time.Time) error {
	rows, err := q.IssueIntervals(ctx, issue)
	if err != nil {
		return err
	}
	m := db.SaveMilestonesParams{WorkspaceID: ws, IssueID: issue, ProjectID: project}
	if created != nil {
		m.CreatedAt = *created
	} else if cur, err := q.Milestones(ctx, issue); err == nil {
		m.CreatedAt = cur.CreatedAt
	} else if len(rows) > 0 {
		m.CreatedAt = rows[0].EnteredAt
	}
	for _, r := range rows {
		if r.Outcome == "started" && m.FirstStartedAt == nil {
			t := r.EnteredAt
			m.FirstStartedAt = &t
		}
	}
	if n := len(rows); n > 0 {
		last := rows[n-1].Outcome
		if last == "completed" || last == "abandoned" {
			k := n - 1
			for k > 0 && rows[k-1].Outcome == last {
				k--
			}
			t := rows[k].EnteredAt
			if last == "completed" {
				m.CompletedAt = &t
			} else {
				m.AbandonedAt = &t
			}
		}
	}
	return q.SaveMilestones(ctx, m)
}

// Rebuild empties the projections and queues every event again; CatchUp
// then replays them. It waits for a running batch to finish.
func (p *Projector) Rebuild(ctx context.Context) error {
	return postgres.InTx(ctx, p.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(lockKey)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `TRUNCATE report_status_intervals, report_issue_milestones, report_iteration_scope, report_estimates`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE events SET reported_at = NULL WHERE reported_at IS NOT NULL`)
		return err
	})
}
