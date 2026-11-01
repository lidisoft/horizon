package issue

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue/db"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/platform/s3"
)

// Attachment limits and lifetimes.
const (
	MaxAttachmentSize = 100 << 20
	uploadTTL         = 15 * time.Minute
	downloadTTL       = 5 * time.Minute
	// UploadExpiry is how long a started upload may stay incomplete.
	UploadExpiry = 24 * time.Hour
)

// ErrNotStored is what Storage.Stat returns for a key with no object.
var ErrNotStored = s3.ErrNotFound

// Storage is the object storage attachments live in
// (docs/architecture.md#object-storage). URLs are presigned for browsers.
type Storage interface {
	PresignPut(ctx context.Context, key string, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, params url.Values, ttl time.Duration) (string, error)
	// Stat returns the object's size, or ErrNotStored.
	Stat(ctx context.Context, key string) (int64, error)
	ReadHead(ctx context.Context, key string, n int64) ([]byte, error)
}

// SetStorage gives the service its object storage; without it, attachment
// commands fail with 503.
func (s *Service) SetStorage(st Storage) { s.storage = st }

// inlineTypes are served inline; everything else downloads (SVG included:
// docs/security.md#other-measures).
var inlineTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

// Attachment is a file attached to an issue.
type Attachment struct {
	ID           uuid.UUID     `json:"id"`
	IssueID      uuid.UUID     `json:"issue_id"`
	UploaderID   uuid.NullUUID `json:"uploader_id" swaggertype:"string"`
	UploaderName string        `json:"uploader_name"`
	Filename     string        `json:"filename" example:"trace.log"`
	ContentType  string        `json:"content_type" example:"text/plain; charset=utf-8"`
	Size         int64         `json:"size"`
	Inline       bool          `json:"inline"` // shown in the page (safe images)
	CreatedAt    time.Time     `json:"created_at"`
}

func attachmentFrom(a db.Attachment, uploader string) Attachment {
	return Attachment{ID: a.ID, IssueID: a.IssueID, UploaderID: a.UploaderID, UploaderName: uploader, Filename: a.Filename,
		ContentType: a.ContentType, Size: a.Size, Inline: inlineTypes[a.ContentType], CreatedAt: a.CreatedAt}
}

// Upload is a started upload: PUT the file's bytes to URL before
// ExpiresAt, then complete it.
type Upload struct {
	Attachment Attachment `json:"attachment"`
	URL        string     `json:"upload_url"`
	Method     string     `json:"method" example:"PUT"`
	ExpiresAt  time.Time  `json:"expires_at"`
}

func objectKey(workspaceID, id uuid.UUID) string {
	return fmt.Sprintf("ws/%s/att/%s", workspaceID, id)
}

// cleanFilename keeps the last path element, without control characters.
func cleanFilename(name string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "." || name == ".." || name == "/" {
		return ""
	}
	if r := []rune(name); len(r) > 255 {
		name = string(r[len(r)-255:])
	}
	return name
}

func (s *Service) needStorage() error {
	if s.storage == nil {
		return &apperr.Error{Status: http.StatusServiceUnavailable, Code: "storage_unavailable"}
	}
	return nil
}

// StartUpload records a pending attachment (issue.attach) and presigns the
// upload of its bytes.
func (s *Service) StartUpload(ctx context.Context, ic Context, filename, contentType string, size int64) (Upload, error) {
	if err := s.require(ctx, ic.Context, access.IssueAttach); err != nil {
		return Upload{}, err
	}
	if err := s.needStorage(); err != nil {
		return Upload{}, err
	}
	var fields apperr.Fields
	if filename = cleanFilename(filename); filename == "" {
		fields.Add("filename", "required")
	}
	switch {
	case size <= 0:
		fields.Add("size", "required")
	case size > MaxAttachmentSize:
		fields.Add("size", "too_large")
	}
	if mt, _, err := mime.ParseMediaType(contentType); err != nil || contentType == "" {
		contentType = "application/octet-stream"
	} else {
		contentType = mt
	}
	if err := fields.Err(); err != nil {
		return Upload{}, err
	}
	if ic.Issue.ArchivedAt != nil {
		return Upload{}, apperr.Conflict("issue_archived")
	}
	id := uuid.Must(uuid.NewV7())
	a, err := db.New(s.pool).CreateAttachment(ctx, db.CreateAttachmentParams{
		WorkspaceID: ic.Workspace.ID, ID: id, IssueID: ic.Issue.ID, UploaderID: uuid.NullUUID{UUID: ic.Principal.User.ID, Valid: true},
		Filename: filename, ContentType: contentType, Size: size, ObjectKey: objectKey(ic.Workspace.ID, id), CreatedAt: s.now(),
	})
	if err != nil {
		return Upload{}, err
	}
	u, err := s.storage.PresignPut(ctx, a.ObjectKey, uploadTTL)
	if err != nil {
		return Upload{}, err
	}
	return Upload{Attachment: attachmentFrom(a, ic.Principal.User.Name), URL: u, Method: http.MethodPut, ExpiresAt: s.now().Add(uploadTTL)}, nil
}

// CompleteUpload checks the uploaded object (its size as announced), sniffs
// its content type and makes the attachment visible.
func (s *Service) CompleteUpload(ctx context.Context, ic Context, id uuid.UUID) (Attachment, error) {
	if err := s.require(ctx, ic.Context, access.IssueAttach); err != nil {
		return Attachment{}, err
	}
	if err := s.needStorage(); err != nil {
		return Attachment{}, err
	}
	var a db.Attachment
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockAttachment(ctx, db.LockAttachmentParams{WorkspaceID: ic.Workspace.ID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (cur.IssueID != ic.Issue.ID || cur.UploaderID.UUID != ic.Principal.User.ID)) {
			return apperr.NotFound("attachment_not_found")
		}
		if err != nil {
			return err
		}
		if cur.State == "ready" {
			a = cur
			return nil
		}
		size, err := s.storage.Stat(ctx, cur.ObjectKey)
		if errors.Is(err, ErrNotStored) {
			return apperr.Conflict("upload_missing")
		}
		if err != nil {
			return err
		}
		if size != cur.Size {
			return apperr.Invalid(apperr.Field{Field: "size", Code: "mismatch"})
		}
		head, err := s.storage.ReadHead(ctx, cur.ObjectKey, 512)
		if err != nil {
			return err
		}
		// Sniffed, so a declared type can't make a page render as an image
		// or script; when sniffing can't tell, the declared type stays (it
		// downloads anyway).
		contentType := http.DetectContentType(head)
		if contentType == "application/octet-stream" || (strings.HasPrefix(contentType, "text/plain") && !strings.HasPrefix(cur.ContentType, "text/")) {
			if !inlineTypes[cur.ContentType] {
				contentType = cur.ContentType
			}
		}
		if a, err = q.CompleteAttachment(ctx, db.CompleteAttachmentParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID, ContentType: contentType, CompletedAt: new(s.now())}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(ic.Context.Context, ic.Issue, "IssueAttachmentAdded", map[string]any{"attachment_id": a.ID}))
	})
	if err != nil {
		return Attachment{}, err
	}
	return attachmentFrom(a, ic.Principal.User.Name), nil
}

// Attachments lists the issue's completed attachments.
func (s *Service) Attachments(ctx context.Context, ic Context) ([]Attachment, error) {
	rows, err := db.New(s.pool).IssueAttachments(ctx, ic.Issue.ID)
	if err != nil {
		return nil, err
	}
	out := make([]Attachment, len(rows))
	for i, r := range rows {
		out[i] = Attachment{ID: r.ID, IssueID: r.IssueID, UploaderID: r.UploaderID, UploaderName: r.UploaderName, Filename: r.Filename,
			ContentType: r.ContentType, Size: r.Size, Inline: inlineTypes[r.ContentType], CreatedAt: r.CreatedAt}
	}
	return out, nil
}

// DownloadURL presigns a short-lived download: safe images inline, every
// other type as an attachment.
func (s *Service) DownloadURL(ctx context.Context, ic Context, id uuid.UUID) (string, error) {
	if err := s.needStorage(); err != nil {
		return "", err
	}
	a, err := db.New(s.pool).AttachmentByID(ctx, db.AttachmentByIDParams{WorkspaceID: ic.Workspace.ID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (a.IssueID != ic.Issue.ID || a.State != "ready")) {
		return "", apperr.NotFound("attachment_not_found")
	}
	if err != nil {
		return "", err
	}
	disposition := "attachment"
	if inlineTypes[a.ContentType] {
		disposition = "inline"
	}
	params := url.Values{}
	params.Set("response-content-type", a.ContentType)
	params.Set("response-content-disposition", mime.FormatMediaType(disposition, map[string]string{"filename": a.Filename}))
	return s.storage.PresignGet(ctx, a.ObjectKey, params, downloadTTL)
}

// DeleteAttachment removes an attachment and, through the purge job, its
// object: the uploader's own (issue.attach), or any with
// issue.comment.moderate.
func (s *Service) DeleteAttachment(ctx context.Context, ic Context, id uuid.UUID) error {
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		a, err := q.LockAttachment(ctx, db.LockAttachmentParams{WorkspaceID: ic.Workspace.ID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && a.IssueID != ic.Issue.ID) {
			return apperr.NotFound("attachment_not_found")
		}
		if err != nil {
			return err
		}
		perm := access.IssueCommentModerate
		if a.UploaderID.Valid && a.UploaderID.UUID == ic.Principal.User.ID {
			perm = access.IssueAttach
		}
		if err := s.require(ctx, ic.Context, perm); err != nil {
			return err
		}
		if err := q.DeleteAttachment(ctx, db.DeleteAttachmentParams{WorkspaceID: a.WorkspaceID, ID: a.ID}); err != nil {
			return err
		}
		if err := eventlog.Scrub(ctx, tx, uuid.NullUUID{UUID: a.WorkspaceID, Valid: true}, "attachment", []uuid.UUID{a.ID}, a.ObjectKey); err != nil {
			return err
		}
		if a.State != "ready" {
			return nil
		}
		return eventlog.Append(ctx, tx, s.event(ic.Context.Context, ic.Issue, "IssueAttachmentRemoved", map[string]any{"attachment_id": a.ID}))
	})
}

// ExpireUploads forgets uploads started before cutoff and never completed,
// queueing their objects (if any) for the purge job. It is a worker job.
func ExpireUploads(ctx context.Context, pool *pgxpool.Pool, cutoff time.Time) error {
	return postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		rows, err := db.New(tx).ExpiredUploads(ctx, cutoff)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if err := eventlog.Scrub(ctx, tx, uuid.NullUUID{UUID: r.WorkspaceID, Valid: true}, "attachment", []uuid.UUID{r.ID}, r.ObjectKey); err != nil {
				return err
			}
		}
		return nil
	})
}
