package issue

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue/db"
	"github.com/lidisoft/horizon/internal/workspace"
)

// SetRelease targets the issue to a release of its project, or to none
// (issue.edit): IssueReleaseChanged. Archived releases take no issues.
func (s *Service) SetRelease(ctx context.Context, ic Context, version int32, releaseID uuid.NullUUID) (Issue, error) {
	return s.mutate(ctx, ic, version, access.IssueEdit, false, func(e *edit) ([]change, error) {
		if releaseID == e.cur.ReleaseID {
			return nil, nil
		}
		if releaseID.Valid {
			var state string
			err := e.tx.QueryRow(ctx, `SELECT state FROM releases WHERE workspace_id = $1 AND project_id = $2 AND id = $3`,
				e.cur.WorkspaceID, e.cur.ProjectID, releaseID.UUID).Scan(&state)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, apperr.Invalid(apperr.Field{Field: "release_id", Code: "unknown"})
			}
			if err != nil {
				return nil, err
			}
			if state == "archived" {
				return nil, apperr.Invalid(apperr.Field{Field: "release_id", Code: "archived"})
			}
		}
		from := e.next.ReleaseID
		e.next.ReleaseID = releaseID
		return []change{{"IssueReleaseChanged", map[string]any{"from": from, "to": releaseID}}}, nil
	})
}

// ClearRelease takes a release being deleted off its issues, in the
// caller's transaction: each records IssueReleaseChanged with by.
func (s *Service) ClearRelease(ctx context.Context, tx pgx.Tx, wc workspace.Context, releaseID uuid.UUID, by string) error {
	rows, err := tx.Query(ctx, `SELECT id FROM issues WHERE workspace_id = $1 AND release_id = $2 ORDER BY id`, wc.Workspace.ID, releaseID)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return err
	}
	q := db.New(tx)
	var events []eventlog.Event
	for _, id := range ids {
		cur, err := q.LockIssue(ctx, db.LockIssueParams{WorkspaceID: wc.Workspace.ID, ID: id})
		if err != nil {
			return err
		}
		cur.ReleaseID = uuid.NullUUID{}
		saved, err := s.save(ctx, tx, cur)
		if err != nil {
			return err
		}
		events = append(events, s.event(wc, saved, "IssueReleaseChanged", map[string]any{"from": releaseID, "to": nil, "by": by}))
	}
	return eventlog.Append(ctx, tx, events...)
}
