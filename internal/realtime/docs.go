package realtime

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/issue"
)

// Collaborative editing of descriptions over the socket
// (docs/realtime.md#protocol): doc.join, doc.steps and doc.leave from
// clients; accepted batches go to every replica on NATS core
// (<prefix>.docs.<document id>) and from there to the sockets that joined.

// joined is a description a socket edits or follows.
type joined struct {
	key     string // as the client named it
	ws      string
	issueID uuid.UUID
}

// docMessage is a batch on NATS core.
type docMessage struct {
	DocumentID uuid.UUID   `json:"document"`
	Batch      issue.Batch `json:"batch"`
}

func (h *Hub) docSubject(id string) string { return h.o.Stream.Prefix + ".docs." + id }

// publishDoc sends an accepted batch to every replica.
func (h *Hub) publishDoc(d issue.DocSteps) {
	b, err := json.Marshal(docMessage{DocumentID: d.DocumentID, Batch: d.Batch})
	if err != nil {
		return
	}
	if err := h.o.NATS.Publish(h.docSubject(d.DocumentID.String()), b); err != nil {
		h.o.Log.Warn().Err(err).Stringer("document", d.DocumentID).Msg("could not publish steps")
	}
}

// runDocs forwards published batches to the sockets that joined.
func (h *Hub) runDocs(ctx context.Context) error {
	ch := make(chan *nats.Msg, 1024)
	sub, err := h.o.NATS.ChanSubscribe(h.docSubject("*"), ch)
	if err != nil {
		return err
	}
	defer func() { _ = sub.Unsubscribe() }()
	for {
		select {
		case <-ctx.Done():
			return nil
		case m := <-ch:
			var dm docMessage
			if json.Unmarshal(m.Data, &dm) != nil {
				continue
			}
			h.docMu.Lock()
			targets := make([]*conn, 0, len(h.docConns[dm.DocumentID]))
			for c := range h.docConns[dm.DocumentID] {
				targets = append(targets, c)
			}
			h.docMu.Unlock()
			for _, c := range targets {
				c.mu.Lock()
				j, ok := c.docs[dm.DocumentID]
				c.mu.Unlock()
				if ok {
					c.reply(message{T: "doc.steps", D: map[string]any{"issue": j.key, "version": dm.Batch.Version,
						"steps": dm.Batch.Steps, "client": dm.Batch.ClientID, "user": dm.Batch.UserID}})
				}
			}
		}
	}
}

// docInbound is the body of the doc.* messages.
type docInbound struct {
	WS      string            `json:"ws"`
	Issue   string            `json:"issue"`
	Version *int64            `json:"version,omitempty"`
	Client  string            `json:"client,omitempty"`
	Steps   []json.RawMessage `json:"steps,omitempty"`
}

func (c *conn) docError(in inbound, key string, err error) {
	code := "error"
	var ae *apperr.Error
	if errors.As(err, &ae) {
		code = ae.Code
	}
	c.reply(message{T: "doc.error", ID: in.ID, D: map[string]string{"issue": key, "code": code}})
}

// enterIssue resolves the issue a doc message names, as REST would.
func (c *conn) enterIssue(ctx context.Context, d docInbound) (issue.Context, error) {
	wc, err := c.workspace(ctx, d.WS)
	if err != nil {
		return issue.Context{}, apperr.NotFound("issue_not_found")
	}
	return c.hub.o.Issues.Enter(ctx, wc, d.Issue)
}

// handleDoc handles doc.join, doc.steps and doc.leave.
func (c *conn) handleDoc(ctx context.Context, in inbound) {
	var d docInbound
	if err := json.Unmarshal(in.D, &d); err != nil || d.Issue == "" {
		c.reply(message{T: "error", ID: in.ID, D: map[string]string{"code": "bad_message"}})
		return
	}
	switch in.T {
	case "doc.join":
		ic, err := c.enterIssue(ctx, d)
		if err != nil {
			c.docError(in, d.Issue, err)
			return
		}
		since := int64(-1)
		if d.Version != nil {
			since = *d.Version
		}
		j, err := c.hub.o.Issues.JoinDescription(ctx, ic, since)
		if err != nil {
			c.docError(in, d.Issue, err)
			return
		}
		c.join(j.DocumentID, joined{key: d.Issue, ws: d.WS, issueID: ic.Issue.ID})
		body := map[string]any{"issue": d.Issue, "version": j.Version, "steps": j.Batches}
		if j.Doc != nil {
			body["doc"] = j.Doc
		}
		c.reply(message{T: "doc.joined", ID: in.ID, D: body})
	case "doc.steps":
		ic, err := c.enterIssue(ctx, d)
		if err != nil {
			c.docError(in, d.Issue, err)
			return
		}
		if d.Version == nil {
			c.docError(in, d.Issue, apperr.Invalid(apperr.Field{Field: "version", Code: "required"}))
			return
		}
		res, err := c.hub.o.Issues.SubmitSteps(ctx, ic, *d.Version, d.Client, d.Steps)
		if err != nil {
			c.docError(in, d.Issue, err)
			return
		}
		if res.Accepted {
			c.reply(message{T: "doc.ack", ID: in.ID, D: map[string]any{"issue": d.Issue, "version": res.Version}})
		} else {
			c.reply(message{T: "doc.reject", ID: in.ID, D: map[string]any{"issue": d.Issue, "version": res.Version, "steps": res.Missing}})
		}
	case "doc.leave":
		c.mu.Lock()
		var gone []uuid.UUID
		for id, j := range c.docs {
			if j.key == d.Issue {
				gone = append(gone, id)
			}
		}
		c.mu.Unlock()
		for _, id := range gone {
			c.leave(id)
		}
	}
}

func (c *conn) join(docID uuid.UUID, j joined) {
	c.mu.Lock()
	c.docs[docID] = j
	c.mu.Unlock()
	h := c.hub
	h.docMu.Lock()
	if h.docConns[docID] == nil {
		h.docConns[docID] = map[*conn]struct{}{}
	}
	h.docConns[docID][c] = struct{}{}
	h.docMu.Unlock()
}

// leave stops following a document; the last socket of this replica to
// leave it snapshots it.
func (c *conn) leave(docID uuid.UUID) {
	c.mu.Lock()
	delete(c.docs, docID)
	c.mu.Unlock()
	h := c.hub
	h.docMu.Lock()
	delete(h.docConns[docID], c)
	last := len(h.docConns[docID]) == 0
	if last {
		delete(h.docConns, docID)
	}
	h.docMu.Unlock()
	if last {
		go func() {
			if err := h.o.Issues.FlushDescription(context.Background(), docID); err != nil {
				h.o.Log.Warn().Err(err).Stringer("document", docID).Msg("could not snapshot description")
			}
		}()
	}
}

// leaveAll is called when the socket closes.
func (c *conn) leaveAll() {
	c.mu.Lock()
	ids := make([]uuid.UUID, 0, len(c.docs))
	for id := range c.docs {
		ids = append(ids, id)
	}
	c.mu.Unlock()
	for _, id := range ids {
		c.leave(id)
	}
}
