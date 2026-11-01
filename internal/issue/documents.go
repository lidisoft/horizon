package issue

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/document"
	"github.com/lidisoft/horizon/internal/issue/db"
)

// Descriptions and comment bodies are block documents
// (docs/realtime.md#document-model). Rows keep their plain-text projection
// (issues.description, comments.body) for search and text clients; rows
// written before documents existed are converted when read.

var docSchema = document.Default()

// Text is a description or comment body as given: a block document (Doc,
// JSON) or plain text, wrapped in paragraphs.
type Text struct {
	Plain *string
	Doc   json.RawMessage
}

// IsSet reports whether either form is given.
func (t Text) IsSet() bool { return t.Plain != nil || len(t.Doc) > 0 }

// PlainText is a Text of plain text.
func PlainText(s string) Text { return Text{Plain: &s} }

// parse reads the document: a field error under field when it isn't one.
func (t Text) parse(fields *apperr.Fields, field string) *document.Node {
	if len(t.Doc) > 0 && string(t.Doc) != "null" {
		n, err := docSchema.ParseJSON(t.Doc)
		if err != nil {
			var de *document.Error
			code := "invalid"
			if errors.As(err, &de) && de.Code == "too_large" {
				code = "too_long"
			}
			fields.Add(field+"_doc", code)
			return nil
		}
		return n
	}
	if t.Plain != nil {
		if len(*t.Plain) > maxDescription {
			fields.Add(field, "too_long")
			return nil
		}
		return document.FromText(docSchema, *t.Plain)
	}
	return document.FromText(docSchema, "")
}

// checkImages requires the document's images to be ready attachments of
// the issue.
func checkImages(ctx context.Context, conn db.DBTX, fields *apperr.Fields, field string, issueID uuid.UUID, doc *document.Node) error {
	ids := document.Attachments(doc)
	if len(ids) == 0 {
		return nil
	}
	ready, err := db.New(conn).ReadyAttachmentIDs(ctx, db.ReadyAttachmentIDsParams{IssueID: issueID, Ids: ids})
	if err != nil {
		return err
	}
	for _, id := range ids {
		if !slices.Contains(ready, id) {
			fields.Add(field+"_doc", "unknown_attachment")
			break
		}
	}
	return nil
}

func docJSON(n *document.Node) json.RawMessage {
	b, _ := json.Marshal(n)
	return b
}

// descriptionOf returns the issue's description document and its version
// (0: never written as a document).
func descriptionOf(ctx context.Context, conn db.DBTX, i db.Issue) (*document.Node, int64, error) {
	d, err := db.New(conn).DescriptionDocument(ctx, i.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return document.FromText(docSchema, i.Description), 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	n, err := docSchema.ParseJSON(d.Content)
	if err != nil {
		return nil, 0, err
	}
	return n, d.Version, nil
}

// saveDescription writes the description document whole.
func (s *Service) saveDescription(ctx context.Context, tx pgx.Tx, i db.Issue, doc *document.Node) error {
	_, err := db.New(tx).SaveDescription(ctx, db.SaveDescriptionParams{
		WorkspaceID: i.WorkspaceID, ID: uuid.Must(uuid.NewV7()), IssueID: i.ID, Content: docJSON(doc), UpdatedAt: s.now(),
	})
	return err
}

// bodyOf returns a comment's document.
func bodyOf(c db.Comment) json.RawMessage {
	if len(c.BodyDoc) > 0 {
		return c.BodyDoc
	}
	return docJSON(document.FromText(docSchema, c.Body))
}
