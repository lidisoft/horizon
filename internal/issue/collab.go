package issue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/document"
	"github.com/lidisoft/horizon/internal/issue/db"
	"github.com/lidisoft/horizon/internal/platform/postgres"
)

// The step protocol for descriptions (docs/realtime.md#protocol): clients
// send batches of steps made against a version; a batch applies only to
// that version (the row lock and document_steps' key serialise writers),
// is checked against the schema and stored, and its steps go to everyone
// editing. The server never transforms steps; stale clients rebase.

const (
	snapshotEvery = 50   // steps between snapshots
	keepSteps     = 1000 // steps kept before the last snapshot, for catching up
)

// Batch is a set of steps applied at once, against Version.
type Batch struct {
	Version  int64             `json:"version"`
	Steps    []json.RawMessage `json:"steps"`
	ClientID string            `json:"client"`
	UserID   uuid.NullUUID     `json:"user" swaggertype:"string"`
}

// DocSteps is an accepted batch, for broadcasting.
type DocSteps struct {
	WorkspaceID uuid.UUID
	DocumentID  uuid.UUID
	IssueID     uuid.UUID
	Batch       Batch
}

// OnDocSteps registers what runs after a batch commits (the realtime hub
// broadcasts it).
func (s *Service) OnDocSteps(f func(DocSteps)) { s.docSteps = f }

func (s *Service) broadcast(d DocSteps) {
	if s.docSteps != nil {
		s.docSteps(d)
	}
}

// docCache keeps the latest document of each recently edited description,
// so a batch doesn't replay the steps since the snapshot.
var docCache sync.Map // document id → cachedDoc

type cachedDoc struct {
	version int64
	doc     *document.Node
}

// currentDoc is the document at its latest version: the snapshot and the
// batches since.
func currentDoc(ctx context.Context, conn db.DBTX, d db.Document) (*document.Node, error) {
	if c, ok := docCache.Load(d.ID); ok && c.(cachedDoc).version == d.Version {
		return c.(cachedDoc).doc, nil
	}
	doc, err := docSchema.ParseJSON(d.Content)
	if err != nil {
		return nil, err
	}
	if d.Version > d.SnapshotVersion {
		rows, err := db.New(conn).StepsSince(ctx, db.StepsSinceParams{DocumentID: d.ID, Version: d.SnapshotVersion})
		if err != nil {
			return nil, err
		}
		at := d.SnapshotVersion
		for _, r := range rows {
			if r.Version != at {
				return nil, fmt.Errorf("issue: document %s: steps from %d missing", d.ID, at)
			}
			steps, err := parseSteps(r.Steps)
			if err != nil {
				return nil, err
			}
			for _, st := range steps {
				if doc, err = st.Apply(doc); err != nil {
					return nil, err
				}
			}
			at += int64(len(steps))
		}
	}
	docCache.Store(d.ID, cachedDoc{version: d.Version, doc: doc})
	return doc, nil
}

func parseSteps(raw json.RawMessage) ([]document.Step, error) {
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	out := make([]document.Step, len(list))
	for i, r := range list {
		st, err := docSchema.ParseStep(r)
		if err != nil {
			return nil, err
		}
		out[i] = st
	}
	return out, nil
}

// lockDescription locks the issue's description document, creating it from
// the issue's text if it has none yet.
func (s *Service) lockDescription(ctx context.Context, tx pgx.Tx, i db.Issue) (db.Document, error) {
	q := db.New(tx)
	d, err := q.LockDescription(ctx, i.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := q.CreateDescription(ctx, db.CreateDescriptionParams{WorkspaceID: i.WorkspaceID, ID: uuid.Must(uuid.NewV7()), IssueID: i.ID,
			Content: docJSON(document.FromText(docSchema, i.Description)), UpdatedAt: s.now()}); err != nil {
			return d, err
		}
		return q.LockDescription(ctx, i.ID)
	}
	return d, err
}

// applyBatch applies steps to the locked document d (at its latest
// version, cur): checked, stored, snapshotted every snapshotEvery steps.
// It returns the new document and the stored batch.
func (s *Service) applyBatch(ctx context.Context, tx pgx.Tx, i db.Issue, d db.Document, cur *document.Node, steps []document.Step,
	clientID string, user uuid.NullUUID, snapshot bool) (*document.Node, Batch, error) {
	doc := cur
	raws := make([]json.RawMessage, len(steps))
	for k, st := range steps {
		next, err := st.Apply(doc)
		if err != nil {
			return nil, Batch{}, &apperr.Error{Status: 422, Code: "invalid_steps", Detail: err.Error()}
		}
		doc = next
		raws[k], _ = document.MarshalStep(st)
	}
	if err := doc.Check(); err != nil {
		return nil, Batch{}, &apperr.Error{Status: 422, Code: "invalid_steps", Detail: err.Error()}
	}
	content := docJSON(doc)
	if len(content) > docSchema.MaxBytes {
		return nil, Batch{}, &apperr.Error{Status: 422, Code: "too_large", Detail: "the description is over 1 MiB"}
	}
	q := db.New(tx)
	all, _ := json.Marshal(raws)
	now := s.now()
	if err := q.AddSteps(ctx, db.AddStepsParams{DocumentID: d.ID, Version: d.Version, Steps: all, ClientID: clientID, UserID: user, CreatedAt: now}); err != nil {
		return nil, Batch{}, err
	}
	version := d.Version + int64(len(steps))
	if err := q.SetDocumentVersion(ctx, db.SetDocumentVersionParams{ID: d.ID, Version: version, UpdatedAt: now}); err != nil {
		return nil, Batch{}, err
	}
	if snapshot || version-d.SnapshotVersion >= snapshotEvery {
		if err := s.snapshot(ctx, tx, i.ID, d.ID, doc, version); err != nil {
			return nil, Batch{}, err
		}
	}
	// Not cached: the transaction may yet roll back. currentDoc caches what
	// it reads at the start of the next one.
	return doc, Batch{Version: d.Version, Steps: raws, ClientID: clientID, UserID: user}, nil
}

// snapshot stores the document at version, its text on the issue (search,
// text clients), and drops batches no catching-up client needs.
func (s *Service) snapshot(ctx context.Context, tx pgx.Tx, issueID, docID uuid.UUID, doc *document.Node, version int64) error {
	q := db.New(tx)
	now := s.now()
	if err := q.Snapshot(ctx, db.SnapshotParams{ID: docID, Content: docJSON(doc), SnapshotVersion: version, UpdatedAt: now}); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE issues SET description = $2 WHERE id = $1`, issueID, document.PlainText(doc)); err != nil {
		return err
	}
	return q.CompactSteps(ctx, db.CompactStepsParams{DocumentID: docID, Version: version - keepSteps})
}

// DocJoin is what a client joining a description gets: the batches since
// the version it holds, or the whole document when they are gone (or it
// holds none).
type DocJoin struct {
	DocumentID uuid.UUID       `json:"-"`
	Version    int64           `json:"version"`
	Doc        json.RawMessage `json:"doc,omitempty"`
	Batches    []Batch         `json:"steps"`
}

// JoinDescription opens the issue's description for editing (or
// following): since is the version the client holds, or -1.
func (s *Service) JoinDescription(ctx context.Context, ic Context, since int64) (DocJoin, error) {
	var out DocJoin
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		d, err := s.lockDescription(ctx, tx, ic.Issue)
		if err != nil {
			return err
		}
		out = DocJoin{DocumentID: d.ID, Version: d.Version, Batches: []Batch{}}
		if since >= 0 && since <= d.Version {
			batches, ok, err := batchesSince(ctx, tx, d, since)
			if err != nil {
				return err
			}
			if ok {
				out.Batches = batches
				return nil
			}
		}
		doc, err := currentDoc(ctx, tx, d)
		if err != nil {
			return err
		}
		out.Doc = docJSON(doc)
		return nil
	})
	return out, err
}

// batchesSince lists the batches from version since to d's; ok is false
// when some are gone.
func batchesSince(ctx context.Context, conn db.DBTX, d db.Document, since int64) ([]Batch, bool, error) {
	if since == d.Version {
		return []Batch{}, true, nil
	}
	rows, err := db.New(conn).StepsSince(ctx, db.StepsSinceParams{DocumentID: d.ID, Version: since})
	if err != nil {
		return nil, false, err
	}
	out := []Batch{}
	at := since
	for _, r := range rows {
		if r.Version != at {
			return nil, false, nil
		}
		var steps []json.RawMessage
		if err := json.Unmarshal(r.Steps, &steps); err != nil {
			return nil, false, err
		}
		out = append(out, Batch{Version: r.Version, Steps: steps, ClientID: r.ClientID, UserID: r.UserID})
		at += int64(len(steps))
	}
	return out, at == d.Version, nil
}

// Submitted is the outcome of SubmitSteps: the stored batch, or, when the
// client was behind, the batches it misses (Missing, with the current
// Version).
type Submitted struct {
	Accepted bool
	Batch    Batch
	Version  int64
	Missing  []Batch
}

// SubmitSteps applies a client's batch made against version base
// (issue.edit). A client behind gets the batches it misses instead, to
// rebase on; steps that don't apply, or make an invalid document, are
// refused (422 invalid_steps).
func (s *Service) SubmitSteps(ctx context.Context, ic Context, base int64, clientID string, raw []json.RawMessage) (Submitted, error) {
	if err := s.require(ctx, ic.Context, access.IssueEdit); err != nil {
		return Submitted{}, err
	}
	if ic.Issue.ArchivedAt != nil {
		return Submitted{}, apperr.Conflict("issue_archived")
	}
	if len(raw) == 0 || len(raw) > 1000 || clientID == "" || len(clientID) > 64 {
		return Submitted{}, &apperr.Error{Status: 422, Code: "invalid_steps", Detail: "a batch holds 1 to 1000 steps and names its client"}
	}
	steps := make([]document.Step, len(raw))
	for k, r := range raw {
		st, err := docSchema.ParseStep(r)
		if err != nil {
			return Submitted{}, &apperr.Error{Status: 422, Code: "invalid_steps", Detail: err.Error()}
		}
		steps[k] = st
	}
	var out Submitted
	var docID uuid.UUID
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		d, err := s.lockDescription(ctx, tx, ic.Issue)
		if err != nil {
			return err
		}
		docID = d.ID
		if d.Version != base {
			out.Version = d.Version
			if base > d.Version {
				return apperr.Conflict("resync")
			}
			missing, ok, err := batchesSince(ctx, tx, d, base)
			if err != nil {
				return err
			}
			if !ok {
				return apperr.Conflict("resync")
			}
			out.Missing = missing
			return nil
		}
		cur, err := currentDoc(ctx, tx, d)
		if err != nil {
			return err
		}
		_, batch, err := s.applyBatch(ctx, tx, ic.Issue, d, cur, steps, clientID, uuid.NullUUID{UUID: ic.Principal.User.ID, Valid: true}, false)
		if err != nil {
			return err
		}
		out = Submitted{Accepted: true, Batch: batch, Version: batch.Version + int64(len(steps))}
		return nil
	})
	if err != nil {
		return Submitted{}, err
	}
	if out.Accepted {
		s.broadcast(DocSteps{WorkspaceID: ic.Workspace.ID, DocumentID: docID, IssueID: ic.Issue.ID, Batch: out.Batch})
	}
	return out, nil
}

// FlushDescription snapshots a description now (the last editor left), if
// it changed since its last snapshot.
func (s *Service) FlushDescription(ctx context.Context, docID uuid.UUID) error {
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var d db.Document
		row := tx.QueryRow(ctx, `SELECT workspace_id, id, issue_id, version, content, updated_at, snapshot_version FROM documents WHERE id = $1 FOR UPDATE`, docID)
		if err := row.Scan(&d.WorkspaceID, &d.ID, &d.IssueID, &d.Version, &d.Content, &d.UpdatedAt, &d.SnapshotVersion); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if d.Version == d.SnapshotVersion {
			return nil
		}
		doc, err := currentDoc(ctx, tx, d)
		if err != nil {
			return err
		}
		return s.snapshot(ctx, tx, d.IssueID, d.ID, doc, d.Version)
	})
}

// replaceDescription writes a whole new description (PATCH, workflow
// post-actions) as one replace step, so editors receive it; it returns the
// broadcast to send after commit (nil when nothing changed).
func (s *Service) replaceDescription(ctx context.Context, tx pgx.Tx, i db.Issue, doc *document.Node, user uuid.NullUUID) (*DocSteps, error) {
	d, err := s.lockDescription(ctx, tx, i)
	if err != nil {
		return nil, err
	}
	cur, err := currentDoc(ctx, tx, d)
	if err != nil {
		return nil, err
	}
	if cur.Eq(doc) {
		return nil, nil
	}
	step := &document.ReplaceStep{From: 0, To: cur.Content.Size(), Slice: document.Slice{Content: doc.Content}}
	_, batch, err := s.applyBatch(ctx, tx, i, d, cur, []document.Step{step}, "api", user, true)
	if err != nil {
		return nil, err
	}
	return &DocSteps{WorkspaceID: i.WorkspaceID, DocumentID: d.ID, IssueID: i.ID, Batch: batch}, nil
}
