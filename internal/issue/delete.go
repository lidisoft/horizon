package issue

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue/db"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/workspace"
)

// What to do with the children of a deleted issue.
const (
	ChildrenReparent = "reparent" // they move to the deleted issue's parent
	ChildrenCascade  = "cascade"  // they are deleted too
)

// Delete hard-deletes an issue (issue.delete; docs/domain.md#deletion): the
// row and what it owns go, its events are scrubbed, and an IssueDeleted
// tombstone remains. children is required when it has any; cascade needs
// issue.delete on every descendant, or nothing is deleted.
func (s *Service) Delete(ctx context.Context, ic Context, children string) error {
	if err := s.require(ctx, ic.Context, access.IssueDelete); err != nil {
		return err
	}
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockIssue(ctx, db.LockIssueParams{WorkspaceID: ic.Workspace.ID, ID: ic.Issue.ID})
		if err != nil {
			return apperr.NotFound("issue_not_found")
		}
		kids, err := q.Children(ctx, db.ChildrenParams{WorkspaceID: cur.WorkspaceID, ParentID: uuid.NullUUID{UUID: cur.ID, Valid: true}})
		if err != nil {
			return err
		}
		if len(kids) > 0 {
			switch children {
			case ChildrenReparent:
				if err := s.reparent(ctx, tx, ic.Context.Context, cur.ID, cur.ParentID, "parent_deleted"); err != nil {
					return err
				}
			case ChildrenCascade:
				subtree, err := q.Descendants(ctx, uuid.NullUUID{UUID: cur.ID, Valid: true})
				if err != nil {
					return err
				}
				keys := make([]string, len(subtree))
				for k, d := range subtree {
					dic, err := s.enterByID(ctx, ic.Context.Context, d)
					if err != nil {
						return apperr.Forbidden("forbidden")
					}
					if err := s.require(ctx, dic.Context, access.IssueDelete); err != nil {
						return err
					}
					keys[k] = dic.key()
				}
				for k, d := range subtree {
					if err := s.deleteOne(ctx, tx, ic.Context.Context, d, keys[k]); err != nil {
						return err
					}
				}
			default:
				return &apperr.Error{Status: 422, Code: "children_choice_required",
					Fields: apperr.Fields{{Field: "children", Code: "required"}}}
			}
		}
		return s.deleteOne(ctx, tx, ic.Context.Context, cur, ic.key())
	})
}

// enterByID is Enter for an issue known by id.
func (s *Service) enterByID(ctx context.Context, wc workspace.Context, i db.Issue) (Context, error) {
	var projectKey string
	if err := s.pool.QueryRow(ctx, `SELECT key FROM projects WHERE id = $1`, i.ProjectID).Scan(&projectKey); err != nil {
		return Context{}, err
	}
	return s.Enter(ctx, wc, keyOf(projectKey, i.Number))
}

// reparent moves the children of an issue to newParent, each with its event.
func (s *Service) reparent(ctx context.Context, tx pgx.Tx, wc workspace.Context, from uuid.UUID, newParent uuid.NullUUID, reason string) error {
	moved, err := db.New(tx).ReparentChildren(ctx, db.ReparentChildrenParams{
		NewParent: newParent, OldParent: uuid.NullUUID{UUID: from, Valid: true}, UpdatedAt: s.now(),
	})
	if err != nil {
		return err
	}
	events := make([]eventlog.Event, len(moved))
	for k, c := range moved {
		events[k] = s.event(wc, c, "IssueParentChanged", map[string]any{"from": from, "to": newParent, "by": reason})
	}
	return eventlog.Append(ctx, tx, events...)
}

// deleteOne deletes one issue whose children are already handled. The
// issues it was linked to record the unlink; their own history stays.
func (s *Service) deleteOne(ctx context.Context, tx pgx.Tx, wc workspace.Context, i db.Issue, key string) error {
	q := db.New(tx)
	links, err := q.LinksOf(ctx, i.ID)
	if err != nil {
		return err
	}
	var events []eventlog.Event
	for _, l := range links {
		otherID, typ := l.TargetID, linkInverse[l.Type]
		if l.TargetID == i.ID {
			otherID, typ = l.SourceID, l.Type
		}
		other, err := q.IssueByID(ctx, db.IssueByIDParams{WorkspaceID: i.WorkspaceID, ID: otherID})
		if err != nil {
			return err
		}
		events = append(events, s.event(wc, other, "IssueUnlinked", map[string]any{"link_id": l.ID, "type": typ, "issue_id": i.ID, "by": "issue_deleted"}))
	}
	if err := eventlog.Append(ctx, tx, events...); err != nil {
		return err
	}
	if err := s.leftIteration(ctx, tx, wc, i, LeftDeleted); err != nil {
		return err
	}
	objects, err := q.AttachmentKeys(ctx, []uuid.UUID{i.ID})
	if err != nil {
		return err
	}
	if err := q.DeleteIssue(ctx, db.DeleteIssueParams{WorkspaceID: i.WorkspaceID, ID: i.ID}); err != nil {
		return err
	}
	i.Version++
	return eventlog.Tombstone(ctx, tx, s.event(wc, i, "IssueDeleted", map[string]any{"key": key, "project_id": i.ProjectID}), objects...)
}

// deleteProject removes a deleted project's issues and scrubs their
// history; ProjectDeleted is their only tombstone. Children elsewhere
// become top-level. It is a project.DeleteHook.
func (s *Service) deleteProject(ctx context.Context, tx pgx.Tx, pc project.Context) error {
	q := db.New(tx)
	ids, err := q.ProjectIssueIDs(ctx, db.ProjectIssueIDsParams{WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID})
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT parent_id FROM issues
		WHERE workspace_id = $1 AND project_id <> $2 AND parent_id = ANY($3::uuid[])`, pc.Workspace.ID, pc.Project.ID, ids)
	if err != nil {
		return err
	}
	parents, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return err
	}
	for _, p := range parents {
		if err := s.reparent(ctx, tx, pc.Context, p, uuid.NullUUID{}, "project_deleted"); err != nil {
			return err
		}
	}
	objects, err := q.AttachmentKeys(ctx, ids)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM issues WHERE workspace_id = $1 AND project_id = $2`, pc.Workspace.ID, pc.Project.ID); err != nil {
		return err
	}
	return eventlog.Scrub(ctx, tx, uuid.NullUUID{UUID: pc.Workspace.ID, Valid: true}, "issue", ids, objects...)
}
