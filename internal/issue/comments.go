package issue

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/document"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue/db"
	"github.com/lidisoft/horizon/internal/platform/postgres"
)

// Comment is a comment on an issue: a single-author block document, with
// its plain text.
type Comment struct {
	ID         uuid.UUID     `json:"id"`
	IssueID    uuid.UUID     `json:"issue_id"`
	AuthorID   uuid.NullUUID `json:"author_id" swaggertype:"string"`
	AuthorName string        `json:"author_name"`
	// Body is the plain text of BodyDoc.
	Body      string          `json:"body"`
	BodyDoc   json.RawMessage `json:"body_doc" swaggertype:"object"`
	CreatedAt time.Time       `json:"created_at"`
	EditedAt  *time.Time      `json:"edited_at"`
	Version   int32           `json:"version"`
}

func commentFrom(c db.Comment, author string) Comment {
	return Comment{ID: c.ID, IssueID: c.IssueID, AuthorID: c.AuthorID, AuthorName: author, Body: c.Body, BodyDoc: bodyOf(c),
		CreatedAt: c.CreatedAt, EditedAt: c.EditedAt, Version: c.Version}
}

// CommentPage is a page of comments, oldest first.
type CommentPage struct {
	Items []Comment `json:"items"`
	Next  string    `json:"next_cursor,omitempty"`
}

// Comments lists the issue's comments, oldest first, after cursor (the
// next_cursor of the previous page).
func (s *Service) Comments(ctx context.Context, ic Context, cursor string, limit int) (CommentPage, error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	after := uuid.Nil
	if cursor != "" {
		var err error
		if after, err = uuid.Parse(cursor); err != nil {
			return CommentPage{}, apperr.Invalid(apperr.Field{Field: "cursor", Code: "invalid"})
		}
	}
	rows, err := db.New(s.pool).IssueComments(ctx, db.IssueCommentsParams{IssueID: ic.Issue.ID, After: after, MaxRows: int32(limit + 1)})
	if err != nil {
		return CommentPage{}, err
	}
	page := CommentPage{Items: []Comment{}}
	for i, r := range rows {
		if i == limit {
			page.Next = rows[i-1].ID.String()
			break
		}
		page.Items = append(page.Items, commentFrom(db.Comment{ID: r.ID, IssueID: r.IssueID, AuthorID: r.AuthorID, Body: r.Body, BodyDoc: r.BodyDoc,
			CreatedAt: r.CreatedAt, EditedAt: r.EditedAt, Version: r.Version}, r.AuthorName))
	}
	return page, nil
}

// validComment reads a comment's document: not empty, its images
// attachments of the issue.
func validComment(ctx context.Context, conn db.DBTX, issueID uuid.UUID, body Text) (*document.Node, error) {
	var fields apperr.Fields
	if body.Plain != nil {
		trimmed := strings.TrimSpace(*body.Plain)
		body.Plain = &trimmed
	}
	doc := body.parse(&fields, "body")
	if doc != nil && document.IsEmpty(doc) {
		fields.Add("body", "required")
	}
	if doc != nil {
		if err := checkImages(ctx, conn, &fields, "body", issueID, doc); err != nil {
			return nil, err
		}
	}
	return doc, fields.Err()
}

// AddComment comments on the issue (issue.comment). Archived issues take
// no comments.
func (s *Service) AddComment(ctx context.Context, ic Context, body Text) (Comment, error) {
	if err := s.require(ctx, ic.Context, access.IssueComment); err != nil {
		return Comment{}, err
	}
	doc, err := validComment(ctx, s.pool, ic.Issue.ID, body)
	if err != nil {
		return Comment{}, err
	}
	var c db.Comment
	err = postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := db.New(tx).LockIssue(ctx, db.LockIssueParams{WorkspaceID: ic.Workspace.ID, ID: ic.Issue.ID})
		if err != nil {
			return apperr.NotFound("issue_not_found")
		}
		if cur.ArchivedAt != nil {
			return apperr.Conflict("issue_archived")
		}
		c, err = db.New(tx).CreateComment(ctx, db.CreateCommentParams{
			WorkspaceID: cur.WorkspaceID, ID: uuid.Must(uuid.NewV7()), IssueID: cur.ID,
			AuthorID: uuid.NullUUID{UUID: ic.Principal.User.ID, Valid: true}, Body: document.PlainText(doc), BodyDoc: docJSON(doc), CreatedAt: s.now(),
		})
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(ic.Context.Context, cur, "IssueCommented", map[string]any{"comment_id": c.ID}))
	})
	if err != nil {
		return Comment{}, err
	}
	return commentFrom(c, ic.Principal.User.Name), nil
}

// lockComment loads one of the issue's comments for a change: the author
// may edit and delete it, holders of issue.comment.moderate may delete it.
func (s *Service) lockComment(ctx context.Context, tx pgx.Tx, ic Context, id uuid.UUID, moderate bool) (db.Comment, error) {
	c, err := db.New(tx).LockComment(ctx, db.LockCommentParams{WorkspaceID: ic.Workspace.ID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && c.IssueID != ic.Issue.ID) {
		return c, apperr.NotFound("comment_not_found")
	}
	if err != nil {
		return c, err
	}
	if c.AuthorID.UUID == ic.Principal.User.ID && c.AuthorID.Valid {
		return c, s.require(ctx, ic.Context, access.IssueComment)
	}
	if moderate {
		return c, s.require(ctx, ic.Context, access.IssueCommentModerate)
	}
	return c, apperr.Forbidden("not_author")
}

// EditComment changes the body of the caller's own comment.
func (s *Service) EditComment(ctx context.Context, ic Context, id uuid.UUID, version int32, body Text) (Comment, error) {
	doc, err := validComment(ctx, s.pool, ic.Issue.ID, body)
	if err != nil {
		return Comment{}, err
	}
	text, raw := document.PlainText(doc), docJSON(doc)
	var c db.Comment
	var author string
	err = postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := s.lockComment(ctx, tx, ic, id, false)
		if err != nil {
			return err
		}
		if version != 0 && cur.Version != version {
			return apperr.Conflict("version_conflict")
		}
		if ic.Issue.ArchivedAt != nil {
			return apperr.Conflict("issue_archived")
		}
		author = ic.Principal.User.Name
		if old, err := docSchema.ParseJSON(bodyOf(cur)); err == nil && old.Eq(doc) {
			c = cur
			return nil
		}
		if c, err = db.New(tx).EditComment(ctx, db.EditCommentParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID, Body: text, BodyDoc: raw, EditedAt: new(s.now())}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(ic.Context.Context, ic.Issue, "IssueCommentEdited", map[string]any{"comment_id": c.ID}))
	})
	if err != nil {
		return Comment{}, err
	}
	return commentFrom(c, author), nil
}

// DeleteComment removes a comment for good: the author's own, or anyone's
// with issue.comment.moderate.
func (s *Service) DeleteComment(ctx context.Context, ic Context, id uuid.UUID) error {
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := s.lockComment(ctx, tx, ic, id, true)
		if err != nil {
			return err
		}
		if err := db.New(tx).DeleteComment(ctx, db.DeleteCommentParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(ic.Context.Context, ic.Issue, "IssueCommentDeleted", map[string]any{"comment_id": cur.ID}))
	})
}
