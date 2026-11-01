package issue

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/configdef"
	configdb "github.com/lidisoft/horizon/internal/configdef/db"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue/db"
)

// The configuration module asks issues to follow its changes: statuses
// removed by a publication, a type moving to another workflow, labels
// merged. Service implements configdef.Issues.

var _ configdef.Issues = (*Service)(nil)

// CountByStatus implements configdef.Issues.
func (s *Service) CountByStatus(ctx context.Context, tx pgx.Tx, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	rows, err := db.New(tx).CountByStatus(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]int64{}
	for _, r := range rows {
		out[r.StatusID] = r.N
	}
	return out, nil
}

// CountTypeByStatus implements configdef.Issues.
func (s *Service) CountTypeByStatus(ctx context.Context, tx pgx.Tx, typeID uuid.UUID) (map[uuid.UUID]int64, error) {
	rows, err := db.New(tx).CountTypeByStatus(ctx, typeID)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]int64{}
	for _, r := range rows {
		out[r.StatusID] = r.N
	}
	return out, nil
}

// CountByType implements configdef.Issues.
func (s *Service) CountByType(ctx context.Context, tx pgx.Tx, typeID uuid.UUID) (int64, error) {
	return db.New(tx).CountByType(ctx, typeID)
}

// MoveStatuses implements configdef.Issues.
func (s *Service) MoveStatuses(ctx context.Context, tx pgx.Tx, actor uuid.NullUUID, mapping map[uuid.UUID]uuid.UUID) error {
	return s.moveStatuses(ctx, tx, actor, uuid.NullUUID{}, mapping, "workflow_published")
}

// MoveTypeStatuses implements configdef.Issues.
func (s *Service) MoveTypeStatuses(ctx context.Context, tx pgx.Tx, actor uuid.NullUUID, typeID uuid.UUID, mapping map[uuid.UUID]uuid.UUID) error {
	return s.moveStatuses(ctx, tx, actor, uuid.NullUUID{UUID: typeID, Valid: true}, mapping, "type_workflow_changed")
}

// moveStatuses moves each issue (of a type, if set) to its new status, each
// with its own IssueStatusUpdated, so history and reports see the move.
func (s *Service) moveStatuses(ctx context.Context, tx pgx.Tx, actor, typeID uuid.NullUUID, mapping map[uuid.UUID]uuid.UUID, reason string) error {
	q := db.New(tx)
	for from, to := range mapping {
		rows, err := q.IssuesInStatus(ctx, db.IssuesInStatusParams{StatusID: from, TypeID: typeID})
		if err != nil {
			return err
		}
		for _, r := range rows {
			i, err := q.LockIssue(ctx, db.LockIssueParams{WorkspaceID: r.WorkspaceID, ID: r.ID})
			if err != nil {
				return err
			}
			cfg, err := s.config.Resolve(ctx, tx, i.WorkspaceID, uuid.NullUUID{UUID: i.ProjectID, Valid: true})
			if err != nil {
				return err
			}
			i.StatusID = to
			e := &edit{tx: tx, r: cfg, cur: i, next: &i}
			s.trackOutcome(e, from)
			saved, err := s.save(ctx, tx, i)
			if err != nil {
				return err
			}
			if err := eventlog.Append(ctx, tx, s.systemEvent(saved, actor, "IssueStatusUpdated",
				statusFacts(cfg, to, map[string]any{"from": from, "to": to, "by": reason}))); err != nil {
				return err
			}
		}
	}
	return nil
}

// CountByLabel implements configdef.Issues.
func (s *Service) CountByLabel(ctx context.Context, conn configdb.DBTX, workspaceID uuid.UUID) (map[uuid.UUID]int64, error) {
	rows, err := db.New(conn).CountByLabel(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]int64{}
	for _, r := range rows {
		out[r.LabelID] = r.N
	}
	return out, nil
}

// Relabel implements configdef.Issues.
func (s *Service) Relabel(ctx context.Context, tx pgx.Tx, actor uuid.NullUUID, from, into uuid.UUID) error {
	q := db.New(tx)
	ids, err := q.IssuesWithLabel(ctx, from)
	if err != nil {
		return err
	}
	for _, id := range ids {
		var ws uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT workspace_id FROM issues WHERE id = $1`, id).Scan(&ws); err != nil {
			return err
		}
		i, err := q.LockIssue(ctx, db.LockIssueParams{WorkspaceID: ws, ID: id})
		if err != nil {
			return err
		}
		if err := q.AddIssueLabel(ctx, db.AddIssueLabelParams{WorkspaceID: ws, IssueID: id, LabelID: into}); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM issue_labels WHERE issue_id = $1 AND label_id = $2`, id, from); err != nil {
			return err
		}
		saved, err := s.save(ctx, tx, i)
		if err != nil {
			return err
		}
		if err := eventlog.Append(ctx, tx, s.systemEvent(saved, actor, "IssueUpdated",
			map[string]any{"fields": []string{"labels"}, "removed": from, "added": into, "by": "label_merged"})); err != nil {
			return err
		}
	}
	return nil
}

// ScrubField implements configdef.Issues: a deleted field's values go.
func (s *Service) ScrubField(ctx context.Context, tx pgx.Tx, workspaceID, fieldID uuid.UUID) error {
	return db.New(tx).ScrubField(ctx, db.ScrubFieldParams{WorkspaceID: workspaceID, Field: fieldID.String()})
}

func (s *Service) systemEvent(i db.Issue, actor uuid.NullUUID, typ string, payload map[string]any) eventlog.Event {
	return eventlog.Event{
		WorkspaceID: uuid.NullUUID{UUID: i.WorkspaceID, Valid: true}, AggregateType: "issue", AggregateID: i.ID,
		AggregateVersion: int64(i.Version), Type: typ, ActorID: actor, OccurredAt: s.now(), Payload: eventlog.Payload(payload),
	}
}
