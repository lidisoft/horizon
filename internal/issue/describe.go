package issue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/document"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue/db"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Keystrokes make no domain events (docs/realtime.md#domain-events-for-documents):
// an editing session is described once, by one IssueDescribed event, when
// it goes quiet or its last editor leaves. Notifications and webhooks react
// to that event.

// QuietAfter is how long a description goes without steps before its
// session counts as over.
const QuietAfter = 30 * time.Second

// endSession snapshots the locked document d and records IssueDescribed
// for what changed since the last one: the versions, who edited, and the
// text's length before and after.
func (s *Service) endSession(ctx context.Context, tx pgx.Tx, d db.Document) error {
	snapshotted, described := d.Version == d.SnapshotVersion, d.Version == d.DescribedVersion
	if snapshotted && described {
		return nil
	}
	q := db.New(tx)
	doc, err := currentDoc(ctx, tx, d)
	if err != nil {
		return err
	}
	i, err := q.IssueByID(ctx, db.IssueByIDParams{WorkspaceID: d.WorkspaceID, ID: d.IssueID}) // locked by the caller
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	before := i.Description
	if !snapshotted {
		if err := s.snapshot(ctx, tx, d.IssueID, d.ID, doc, d.Version); err != nil {
			return err
		}
	}
	if described {
		return nil
	}
	editors, err := q.Editors(ctx, db.EditorsParams{DocumentID: d.ID, Version: d.DescribedVersion})
	if err != nil {
		return err
	}
	// The event is the last editor's.
	var actor uuid.NullUUID
	if last, err := q.LastEditor(ctx, d.ID); err == nil {
		actor = last
	}
	after := document.PlainText(doc)
	wc := workspace.Context{Workspace: workspace.Workspace{ID: d.WorkspaceID}, Principal: identity.Principal{User: identity.User{ID: actor.UUID}}}
	if editors == nil {
		editors = []uuid.UUID{}
	}
	if err := eventlog.Append(ctx, tx, s.event(wc, i, "IssueDescribed", map[string]any{
		"from_version": d.DescribedVersion, "to_version": d.Version, "editors": editors,
		"chars_before": len([]rune(before)), "chars_after": len([]rune(after)),
	})); err != nil {
		return err
	}
	return q.SetDescribed(ctx, db.SetDescribedParams{ID: d.ID, DescribedVersion: d.Version})
}

// DescribeQuiet ends the editing sessions with no steps for QuietAfter
// (a periodic job: whatever replica the editors were on, or if it went
// away). It returns how many it described.
func (s *Service) DescribeQuiet(ctx context.Context) (int, error) {
	ids, err := db.New(s.pool).QuietDocuments(ctx, s.now().Add(-QuietAfter))
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
			d, ok, err := lockIssueAndDocument(ctx, tx, id)
			if err != nil || !ok {
				return err
			}
			// Edited again meanwhile: not quiet after all.
			if d.UpdatedAt.After(s.now().Add(-QuietAfter)) {
				return nil
			}
			return s.endSession(ctx, tx, d)
		})
		if err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}
