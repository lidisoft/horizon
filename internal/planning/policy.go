package planning

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/planning/db"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/workspace"
)

// PolicyConsumer is the policy worker (docs/architecture.md#workers): it
// applies each completed iteration's completion policy to its issues.
func (s *Service) PolicyConsumer() eventlog.Consumer {
	return eventlog.Consumer{
		Name:   "policy",
		Filter: []string{"iteration.IterationCompleted"},
		Handle: s.applyCompletion,
	}
}

// applyCompletion applies the completion recorded on the iteration (not the
// event's copy), once: the iteration's completion is marked applied in the
// same transaction, so a redelivered event changes nothing, nor does it
// undo what people changed since. The changes are attributed to whoever
// completed the iteration.
func (s *Service) applyCompletion(ctx context.Context, e eventlog.Event) error {
	if !e.WorkspaceID.Valid {
		return nil
	}
	wc := workspace.Context{
		Workspace: workspace.Workspace{ID: e.WorkspaceID.UUID},
		Principal: identity.Principal{User: identity.User{ID: e.ActorID.UUID}},
	}
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		it, err := q.LockIterationByID(ctx, db.LockIterationByIDParams{WorkspaceID: wc.Workspace.ID, ID: e.AggregateID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // deleted since
		}
		if err != nil {
			return err
		}
		var c Completion
		if len(it.Completion) == 0 || json.Unmarshal(it.Completion, &c) != nil || c.AppliedAt != nil {
			return nil
		}
		rows, err := q.IterationIssueOutcomes(ctx, db.IterationIssueOutcomesParams{WorkspaceID: it.WorkspaceID, IterationID: uuid.NullUUID{UUID: it.ID, Valid: true}})
		if err != nil {
			return err
		}
		var finished, unfinished []uuid.UUID
		for _, r := range rows {
			if r.ArchivedAt != nil {
				continue
			}
			if configdef.Open(r.Outcome) {
				unfinished = append(unfinished, r.ID)
			} else {
				finished = append(finished, r.ID)
			}
		}
		why := map[string]any{"by": "iteration_completed", "iteration_id": it.ID}
		switch c.Policy {
		case configdef.CompletionArchiveAll:
			if err := s.issues.ArchiveIssues(ctx, tx, wc, append(finished, unfinished...), why); err != nil {
				return err
			}
		case configdef.CompletionRollover:
			if err := s.issues.ArchiveIssues(ctx, tx, wc, finished, why); err != nil {
				return err
			}
			if err := s.rollover(ctx, tx, wc, it, c.RolloverTo, unfinished); err != nil {
				return err
			}
		}
		now := s.now()
		c.AppliedAt = &now
		raw, _ := json.Marshal(c)
		return q.SetCompletion(ctx, db.SetCompletionParams{WorkspaceID: it.WorkspaceID, ID: it.ID, Completion: raw})
	})
}

// rollover moves a completed iteration's unfinished issues to the chosen
// iteration, or to the backlog when there is none or it is no longer open.
// Both iterations record it.
func (s *Service) rollover(ctx context.Context, tx pgx.Tx, wc workspace.Context, from db.Iteration, to uuid.NullUUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	q := db.New(tx)
	if to.Valid {
		target, err := q.LockIterationByID(ctx, db.LockIterationByIDParams{WorkspaceID: from.WorkspaceID, ID: to.UUID})
		switch {
		case errors.Is(err, pgx.ErrNoRows) || (err == nil && (!open(target.State) || target.ProjectID != from.ProjectID)):
			to = uuid.NullUUID{}
		case err != nil:
			return err
		}
	}
	if _, err := s.issues.SetIteration(ctx, tx, wc, from.ProjectID, ids, to, "rolled_over", false); err != nil {
		return err
	}
	saved, err := q.BumpIteration(ctx, db.BumpIterationParams{WorkspaceID: from.WorkspaceID, ID: from.ID, UpdatedAt: s.now()})
	if err != nil {
		return err
	}
	left := map[string]any{"added": []uuid.UUID{}, "removed": ids, "reason": "rolled_over"}
	if to.Valid {
		left["to"] = to.UUID
	}
	events := []eventlog.Event{s.event(wc, "iteration", saved.ID, saved.ProjectID, saved.Version, "IterationLoaded", left)}
	if to.Valid {
		target, err := q.BumpIteration(ctx, db.BumpIterationParams{WorkspaceID: from.WorkspaceID, ID: to.UUID, UpdatedAt: s.now()})
		if err != nil {
			return err
		}
		events = append(events, s.event(wc, "iteration", target.ID, target.ProjectID, target.Version, "IterationLoaded",
			map[string]any{"added": ids, "removed": []uuid.UUID{}, "reason": "rolled_over", "from": from.ID}))
	}
	return eventlog.Append(ctx, tx, events...)
}
