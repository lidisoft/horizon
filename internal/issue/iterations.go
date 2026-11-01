package issue

import (
	"context"
	"errors"
	"maps"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue/db"
	"github.com/lidisoft/horizon/internal/workspace"
)

// IterationHook records, in tx, that issues left an iteration other than
// through planning's own commands: deleted, moved to another project, or
// cleared by a workflow post-action. Planning registers one, so the
// iteration's scope change is recorded (IterationLoaded, docs/domain.md#policies).
type IterationHook func(ctx context.Context, tx pgx.Tx, wc workspace.Context, iterationID uuid.UUID, issueIDs []uuid.UUID, reason string) error

// Reasons an issue leaves its iteration, passed to IterationHook.
const (
	LeftDeleted = "issue_deleted"
	LeftMoved   = "issue_moved"
	LeftCleared = "workflow"
)

// OnLeaveIteration registers the hook.
func (s *Service) OnLeaveIteration(h IterationHook) { s.leaveIteration = h }

func (s *Service) leftIteration(ctx context.Context, tx pgx.Tx, wc workspace.Context, i db.Issue, reason string) error {
	if !i.IterationID.Valid || s.leaveIteration == nil {
		return nil
	}
	return s.leaveIteration(ctx, tx, wc, i.IterationID.UUID, []uuid.UUID{i.ID}, reason)
}

// SetIteration moves issues of a project into an iteration, or out of any
// (to invalid), in the caller's transaction: planning's load commands,
// which check permissions and record the iterations' side. Each issue that
// changes records IssueIterationChanged. Archived issues are refused unless
// allowArchived (an iteration being deleted lets its issues go). It
// returns the issues as they were before.
func (s *Service) SetIteration(ctx context.Context, tx pgx.Tx, wc workspace.Context, projectID uuid.UUID, ids []uuid.UUID,
	to uuid.NullUUID, by string, allowArchived bool) ([]db.Issue, error) {
	q := db.New(tx)
	before := make([]db.Issue, 0, len(ids))
	var events []eventlog.Event
	for _, id := range ids {
		cur, err := q.LockIssue(ctx, db.LockIssueParams{WorkspaceID: wc.Workspace.ID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && cur.ProjectID != projectID) {
			return nil, apperr.Invalid(apperr.Field{Field: "issues", Code: "unknown"})
		}
		if err != nil {
			return nil, err
		}
		if cur.ArchivedAt != nil && !allowArchived {
			return nil, apperr.Conflict("issue_archived")
		}
		before = append(before, cur)
		if cur.IterationID == to {
			continue
		}
		saved, err := q.SetIssueIteration(ctx, db.SetIssueIterationParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID, IterationID: to, UpdatedAt: s.now()})
		if err != nil {
			return nil, err
		}
		payload := map[string]any{"from": cur.IterationID, "to": to}
		if by != "" {
			payload["by"] = by
		}
		events = append(events, s.event(wc, saved, "IssueIterationChanged", payload))
	}
	return before, eventlog.Append(ctx, tx, events...)
}

// ArchiveIssues archives issues in the caller's transaction, for policies
// (iteration completion): each one not archived yet records IssueArchived,
// with payload saying why. Issues already gone are skipped.
func (s *Service) ArchiveIssues(ctx context.Context, tx pgx.Tx, wc workspace.Context, ids []uuid.UUID, payload map[string]any) error {
	q := db.New(tx)
	var events []eventlog.Event
	for _, id := range ids {
		cur, err := q.LockIssue(ctx, db.LockIssueParams{WorkspaceID: wc.Workspace.ID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if cur.ArchivedAt != nil {
			continue
		}
		now := s.now()
		cur.ArchivedAt = &now
		saved, err := s.save(ctx, tx, cur)
		if err != nil {
			return err
		}
		events = append(events, s.event(wc, saved, "IssueArchived", maps.Clone(payload)))
	}
	return eventlog.Append(ctx, tx, events...)
}
