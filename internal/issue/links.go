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
	"github.com/lidisoft/horizon/internal/platform/postgres"
)

// Links lists the issue's links seen from it, with the other issues' keys
// and titles. Links to issues the caller can't see are left out.
func (s *Service) Links(ctx context.Context, ic Context) ([]Link, error) {
	rows, err := db.New(s.pool).LinksOf(ctx, ic.Issue.ID)
	if err != nil {
		return nil, err
	}
	out := []Link{}
	for _, l := range rows {
		otherID, typ := l.TargetID, l.Type
		if l.TargetID == ic.Issue.ID {
			otherID, typ = l.SourceID, linkInverse[l.Type]
		}
		other, key, ok, err := s.visibleIssue(ctx, ic, otherID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		out = append(out, Link{ID: l.ID, Type: typ, IssueID: other.ID, Key: key, Title: other.Title, LagDays: l.LagDays})
	}
	return out, nil
}

// visibleIssue loads another issue of the workspace if the caller can see
// its project.
func (s *Service) visibleIssue(ctx context.Context, ic Context, id uuid.UUID) (db.Issue, string, bool, error) {
	i, err := db.New(s.pool).IssueByID(ctx, db.IssueByIDParams{WorkspaceID: ic.Workspace.ID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return i, "", false, nil
	}
	if err != nil {
		return i, "", false, err
	}
	var projectKey string
	if err := s.pool.QueryRow(ctx, `SELECT key FROM projects WHERE id = $1`, i.ProjectID).Scan(&projectKey); err != nil {
		return i, "", false, err
	}
	other, err := s.Enter(ctx, ic.Context.Context, keyOf(projectKey, i.Number))
	if apperr.Is(err, "issue_not_found") {
		return i, "", false, nil
	}
	if err != nil {
		return i, "", false, err
	}
	return other.Issue, keyOf(projectKey, i.Number), true, nil
}

// LinkInput creates a link from the issue.
type LinkInput struct {
	Type      string
	TargetKey string
	LagDays   *int32
}

// AddLink links the issue to another one it can see (issue.link). The
// event goes on both issues' histories.
func (s *Service) AddLink(ctx context.Context, ic Context, in LinkInput) (Link, error) {
	if err := s.require(ctx, ic.Context, access.IssueLink); err != nil {
		return Link{}, err
	}
	inverse, ok := linkInverse[in.Type]
	if !ok {
		return Link{}, apperr.Invalid(apperr.Field{Field: "type", Code: "invalid"})
	}
	target, err := s.Enter(ctx, ic.Context.Context, in.TargetKey)
	if err != nil {
		return Link{}, apperr.Invalid(apperr.Field{Field: "target", Code: "unknown"})
	}
	if target.Issue.ID == ic.Issue.ID {
		return Link{}, apperr.Invalid(apperr.Field{Field: "target", Code: "self"})
	}
	if in.LagDays != nil && in.Type != "precedes" && in.Type != "follows" {
		return Link{}, apperr.Invalid(apperr.Field{Field: "lag_days", Code: "not_scheduling"})
	}
	if ic.Issue.ArchivedAt != nil {
		return Link{}, apperr.Conflict("issue_archived")
	}
	// Store the canonical type from its source.
	source, dest, typ := ic.Issue.ID, target.Issue.ID, in.Type
	if !storedLinkType[typ] {
		source, dest, typ = dest, source, inverse
	}
	var link db.IssueLink
	err = postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var dup bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM issue_links WHERE type = $3 AND
			((source_id = $1 AND target_id = $2) OR (type = 'relates_to' AND source_id = $2 AND target_id = $1)))`,
			source, dest, typ).Scan(&dup); err != nil {
			return err
		}
		if dup {
			return apperr.Conflict("link_exists")
		}
		var err error
		link, err = db.New(tx).CreateLink(ctx, db.CreateLinkParams{
			WorkspaceID: ic.Workspace.ID, ID: uuid.Must(uuid.NewV7()), SourceID: source, TargetID: dest, Type: typ,
			LagDays: in.LagDays, CreatedBy: uuid.NullUUID{UUID: ic.Principal.User.ID, Valid: true}, CreatedAt: s.now(),
		})
		if err != nil {
			return err
		}
		return s.linkEvents(ctx, tx, ic, link, "IssueLinked")
	})
	if err != nil {
		return Link{}, err
	}
	return Link{ID: link.ID, Type: in.Type, IssueID: target.Issue.ID, Key: target.key(), Title: target.Issue.Title, LagDays: link.LagDays}, nil
}

// RemoveLink deletes one of the issue's links (issue.link).
func (s *Service) RemoveLink(ctx context.Context, ic Context, id uuid.UUID) error {
	if err := s.require(ctx, ic.Context, access.IssueLink); err != nil {
		return err
	}
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		l, err := q.LockLink(ctx, db.LockLinkParams{WorkspaceID: ic.Workspace.ID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && l.SourceID != ic.Issue.ID && l.TargetID != ic.Issue.ID) {
			return apperr.NotFound("link_not_found")
		}
		if err != nil {
			return err
		}
		if err := q.DeleteLink(ctx, l.ID); err != nil {
			return err
		}
		return s.linkEvents(ctx, tx, ic, l, "IssueUnlinked")
	})
}

// linkEvents records a link change on both issues, each seeing its own
// side of it. Links don't change the issues' rows or versions.
func (s *Service) linkEvents(ctx context.Context, tx pgx.Tx, ic Context, l db.IssueLink, typ string) error {
	q := db.New(tx)
	var events []eventlog.Event
	for _, side := range []struct {
		id, other uuid.UUID
		typ       string
	}{{l.SourceID, l.TargetID, l.Type}, {l.TargetID, l.SourceID, linkInverse[l.Type]}} {
		i, err := q.IssueByID(ctx, db.IssueByIDParams{WorkspaceID: ic.Workspace.ID, ID: side.id})
		if err != nil {
			return err
		}
		events = append(events, s.event(ic.Context.Context, i, typ, map[string]any{"link_id": l.ID, "type": side.typ, "issue_id": side.other}))
	}
	return eventlog.Append(ctx, tx, events...)
}
