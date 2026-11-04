package notify

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/document"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/notify/db"
)

// quiet are the issue events that notify no follower: reordering, and
// edits to what was already told.
var quiet = []string{"IssueRanked", "IssueCommentEdited", "IssueCommentDeleted", "IssueAttachmentRemoved", "IssueDeleted"}

// Consumer is the durable notify consumer of issue events.
func (s *Service) Consumer() eventlog.Consumer {
	return eventlog.Consumer{Name: "notify", Filter: []string{"issue.*"}, Handle: s.Handle}
}

// target is the issue an event is about, as it is now.
type target struct {
	id, workspaceID uuid.UUID
	slug, key       string
}

// Handle notifies the people an issue event concerns. It is idempotent:
// subscriptions are kept as they are, and inbox rows already past the
// event stay as they are.
func (s *Service) Handle(ctx context.Context, e eventlog.Event) error {
	if e.AggregateType != "issue" || !e.WorkspaceID.Valid || slices.Contains(quiet, e.Type) {
		return nil
	}
	var t target
	err := s.o.Pool.QueryRow(ctx, `
		SELECT i.id, i.workspace_id, w.slug, p.key || '-' || i.number
		FROM issues i JOIN projects p ON p.id = i.project_id JOIN workspaces w ON w.id = i.workspace_id
		WHERE i.id = $1`, e.AggregateID).Scan(&t.id, &t.workspaceID, &t.slug, &t.key)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // deleted since: its inbox rows went with it
	}
	if err != nil {
		return err
	}
	var payload map[string]any
	_ = json.Unmarshal(e.Payload, &payload)
	actor := e.ActorID.UUID // uuid.Nil for the system

	// Who the event concerns directly, and why.
	direct := map[uuid.UUID]string{}
	subscribe := map[uuid.UUID]string{}
	switch e.Type {
	case "IssueCreated":
		if actor != uuid.Nil {
			subscribe[actor] = ReasonCreator
		}
		if id, ok := uuidOf(payload["assignee_id"]); ok {
			direct[id], subscribe[id] = Assigned, ReasonAssignee
		}
		mentioned, err := s.descriptionMentions(ctx, t.id)
		if err != nil {
			return err
		}
		for _, id := range mentioned {
			if _, ok := direct[id]; !ok {
				direct[id], subscribe[id] = Mentioned, ReasonMention
			}
		}
	case "IssueAssigned":
		if id, ok := uuidOf(payload["to"]); ok {
			direct[id], subscribe[id] = Assigned, ReasonAssignee
		}
	case "IssueCommented":
		if actor != uuid.Nil {
			subscribe[actor] = ReasonCommenter
		}
		if id, ok := uuidOf(payload["comment_id"]); ok {
			mentioned, err := s.commentMentions(ctx, id)
			if err != nil {
				return err
			}
			for _, id := range mentioned {
				direct[id], subscribe[id] = Mentioned, ReasonMention
			}
		}
	case "IssueDescribed":
		// Newly mentioned: those not following yet. Followers hear of the
		// edit anyway.
		mentioned, err := s.descriptionMentions(ctx, t.id)
		if err != nil {
			return err
		}
		followers, err := db.New(s.o.Pool).Subscribers(ctx, t.id)
		if err != nil {
			return err
		}
		for _, id := range mentioned {
			if !slices.Contains(followers, id) {
				direct[id], subscribe[id] = Mentioned, ReasonMention
			}
		}
	}

	// Subscribe first (only people who see the issue), then tell everyone
	// following, but never the actor of their own change.
	q := db.New(s.o.Pool)
	visible := map[uuid.UUID]bool{}
	sees := func(id uuid.UUID) (bool, error) {
		if v, ok := visible[id]; ok {
			return v, nil
		}
		v, err := s.sees(ctx, id, t.slug, t.key)
		visible[id] = v
		return v, err
	}
	for _, id := range sortedKeys(subscribe) {
		ok, err := sees(id)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if err := q.Subscribe(ctx, db.SubscribeParams{WorkspaceID: t.workspaceID, IssueID: t.id, UserID: id, Reason: subscribe[id], CreatedAt: e.OccurredAt}); err != nil {
			return err
		}
	}
	followers, err := q.Subscribers(ctx, t.id)
	if err != nil {
		return err
	}
	tell := map[uuid.UUID]string{}
	for _, id := range followers {
		tell[id] = Subscribed
	}
	for id, reason := range direct {
		tell[id] = reason
	}
	delete(tell, actor)
	for _, id := range sortedKeys(tell) {
		ok, err := sees(id)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		n, err := q.Bump(ctx, db.BumpParams{WorkspaceID: t.workspaceID, ID: uuid.Must(uuid.NewV7()), UserID: id, IssueID: t.id,
			Reason: tell[id], EventID: e.ID, UpdatedAt: e.OccurredAt})
		if err != nil {
			return err
		}
		if n > 0 {
			s.changed(id, t.slug)
		}
	}
	return nil
}

// descriptionMentions lists the people the issue's description mentions.
func (s *Service) descriptionMentions(ctx context.Context, issueID uuid.UUID) ([]uuid.UUID, error) {
	var raw []byte
	err := s.o.Pool.QueryRow(ctx, `SELECT content FROM documents WHERE issue_id = $1`, issueID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return mentions(raw), nil
}

// commentMentions lists the people a comment mentions.
func (s *Service) commentMentions(ctx context.Context, commentID uuid.UUID) ([]uuid.UUID, error) {
	var raw []byte
	err := s.o.Pool.QueryRow(ctx, `SELECT body_doc FROM comments WHERE id = $1`, commentID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // deleted since
	}
	if err != nil {
		return nil, err
	}
	return mentions(raw), nil
}

func mentions(raw []byte) []uuid.UUID {
	if len(raw) == 0 {
		return nil // plain text from before documents: no mentions
	}
	doc, err := document.Default().ParseJSON(raw)
	if err != nil {
		return nil
	}
	return document.Mentions(doc)
}

func uuidOf(v any) (uuid.UUID, bool) {
	s, _ := v.(string)
	id, err := uuid.Parse(s)
	return id, err == nil && id != uuid.Nil
}

// sortedKeys gives a stable order, so writes take locks alike.
func sortedKeys[V any](m map[uuid.UUID]V) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.SortFunc(out, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	return out
}

func isNotFound(err error) bool {
	e, ok := apperr.As(err)
	return ok && e.Status == 404
}
