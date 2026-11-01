package issue

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
)

// Ref is an entity an event mentions, named at read time: stored payloads
// hold ids only (docs/api.md#domain-event-format).
type Ref struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name,omitempty"`
	Key  string    `json:"key,omitempty"`
}

// EventAggregate is the aggregate an event belongs to.
type EventAggregate struct {
	Type    string    `json:"type" example:"issue"`
	ID      uuid.UUID `json:"id"`
	Key     string    `json:"key,omitempty" example:"HZN-42"`
	Version int64     `json:"version"`
}

// PublicEvent is a domain event as clients see it: the activity feed, and
// later WebSocket pushes and webhooks.
type PublicEvent struct {
	ID         uuid.UUID      `json:"id"`
	Type       string         `json:"type" example:"issue.status_updated"`
	Workspace  string         `json:"workspace" example:"acme"`
	Aggregate  EventAggregate `json:"aggregate"`
	Actor      *Ref           `json:"actor"`
	OccurredAt time.Time      `json:"occurred_at"`
	// Payload has the stored values, ids of known entities replaced by
	// {id, name} (issues: {id, key, name}).
	Payload map[string]any `json:"payload" swaggertype:"object"`
}

// ActivityPage is a page of an issue's history, newest first.
type ActivityPage struct {
	Items []PublicEvent `json:"items"`
	Next  string        `json:"next_cursor,omitempty"`
}

// Activity returns the issue's history, newest first, before cursor (the
// next_cursor of the previous page).
func (s *Service) Activity(ctx context.Context, ic Context, cursor string, limit int) (ActivityPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	before := uuid.Nil
	if cursor != "" {
		var err error
		if before, err = uuid.Parse(cursor); err != nil {
			return ActivityPage{}, apperr.Invalid(apperr.Field{Field: "cursor", Code: "invalid"})
		}
	}
	events, err := eventlog.Before(ctx, s.pool, ic.Workspace.ID, "issue", ic.Issue.ID, before, limit+1)
	if err != nil {
		return ActivityPage{}, err
	}
	page := ActivityPage{Items: []PublicEvent{}}
	if len(events) > limit {
		events = events[:limit]
		page.Next = events[limit-1].ID.String()
	}
	page.Items, err = s.Describe(ctx, ic, events)
	return page, err
}

// Describe turns events of the issue into the public format.
func (s *Service) Describe(ctx context.Context, ic Context, events []eventlog.Event) ([]PublicEvent, error) {
	r, err := s.config.Resolve(ctx, s.pool, ic.Workspace.ID, uuid.NullUUID{UUID: ic.Issue.ProjectID, Valid: true})
	if err != nil {
		return nil, err
	}
	n := &namer{s: s, ic: ic, r: r, refs: map[uuid.UUID]*Ref{}}
	payloads := make([]map[string]any, len(events))
	for i, e := range events {
		payloads[i] = map[string]any{}
		_ = json.Unmarshal(e.Payload, &payloads[i])
		if e.ActorID.Valid {
			n.want(e.ActorID.UUID)
		}
		for _, v := range payloads[i] {
			n.collect(v)
		}
	}
	if err := n.resolve(ctx); err != nil {
		return nil, err
	}
	out := make([]PublicEvent, len(events))
	for i, e := range events {
		pe := PublicEvent{
			ID: e.ID, Type: e.PublicType(), Workspace: ic.Workspace.Slug, OccurredAt: e.OccurredAt,
			Aggregate: EventAggregate{Type: e.AggregateType, ID: e.AggregateID, Key: ic.key(), Version: e.AggregateVersion},
			Payload:   map[string]any{},
		}
		if e.ActorID.Valid {
			pe.Actor = n.ref(e.ActorID.UUID)
		}
		for k, v := range payloads[i] {
			pe.Payload[k] = n.replace(v)
		}
		out[i] = pe
	}
	return out, nil
}

// namer resolves the ids mentioned by a batch of events.
type namer struct {
	s     *Service
	ic    Context
	r     *configdef.Resolved
	refs  map[uuid.UUID]*Ref
	wants []uuid.UUID
}

func (n *namer) want(id uuid.UUID) {
	if _, ok := n.refs[id]; !ok {
		n.refs[id] = nil
		n.wants = append(n.wants, id)
	}
}

func (n *namer) collect(v any) {
	switch x := v.(type) {
	case string:
		if id, err := uuid.Parse(x); err == nil {
			n.want(id)
		}
	case []any:
		for _, e := range x {
			n.collect(e)
		}
	}
}

func (n *namer) ref(id uuid.UUID) *Ref {
	if r := n.refs[id]; r != nil {
		return r
	}
	return &Ref{ID: id}
}

func (n *namer) replace(v any) any {
	switch x := v.(type) {
	case string:
		if id, err := uuid.Parse(x); err == nil && n.refs[id] != nil {
			return n.refs[id]
		}
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = n.replace(e)
		}
		return out
	}
	return v
}

// resolve names what it can: configuration from the resolver, then people,
// then issues the caller can see. Unknown ids stay ids.
func (n *namer) resolve(ctx context.Context) error {
	var rest []uuid.UUID
	for _, id := range n.wants {
		if st, _, ok := n.r.Status(id); ok {
			n.refs[id] = &Ref{ID: id, Name: st.Name}
			continue
		}
		if it, ok := findType(n.r, id); ok {
			n.refs[id] = &Ref{ID: id, Name: it.Name}
			continue
		}
		if l, ok := n.r.Labels[id]; ok {
			n.refs[id] = &Ref{ID: id, Name: l.Name}
			continue
		}
		if f, ok := n.r.Fields[id]; ok {
			n.refs[id] = &Ref{ID: id, Name: f.Name}
			continue
		}
		rest = append(rest, id)
	}
	if len(rest) == 0 {
		return nil
	}
	// People, and the planning items of the issue's workspace.
	rows, err := n.s.pool.Query(ctx, `
		SELECT u.id, u.name FROM users u
		JOIN workspace_members m ON m.user_id = u.id AND m.workspace_id = $1
		WHERE u.id = ANY($2::uuid[])
		UNION ALL
		SELECT it.id, it.name FROM iterations it WHERE it.workspace_id = $1 AND it.id = ANY($2::uuid[])`, n.ic.Workspace.ID, rest)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return err
		}
		n.refs[id] = &Ref{ID: id, Name: name}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range rest {
		if n.refs[id] != nil {
			continue
		}
		if id == n.ic.Issue.ID {
			n.refs[id] = &Ref{ID: id, Key: n.ic.key(), Name: n.ic.Issue.Title}
			continue
		}
		i, key, ok, err := n.s.visibleIssue(ctx, n.ic, id)
		if err != nil {
			return err
		}
		if ok {
			n.refs[id] = &Ref{ID: id, Key: key, Name: i.Title}
		}
	}
	return nil
}

// Pushed is an issue event described for pushes, with the projects whose
// viewers may receive it (the issue's, and the one it left on a move).
type Pushed struct {
	Event    PublicEvent
	Projects []uuid.UUID
}

// Public describes an issue event for WebSocket pushes, as for a viewer of
// its project: other issues stay ids, since recipients differ. A deleted
// issue is described from its tombstone. ok is false when the event's
// issue and project are gone.
func (s *Service) Public(ctx context.Context, e eventlog.Event) (Pushed, bool, error) {
	var out Pushed
	var payload map[string]any
	_ = json.Unmarshal(e.Payload, &payload)
	if from, ok := payload["from_project_id"].(string); ok {
		if id, err := uuid.Parse(from); err == nil {
			out.Projects = append(out.Projects, id)
		}
	}
	var ic Context
	row := s.pool.QueryRow(ctx, `
		SELECT w.slug, p.id, p.key, i.number, i.title
		FROM issues i JOIN projects p ON p.id = i.project_id JOIN workspaces w ON w.id = i.workspace_id
		WHERE i.workspace_id = $1 AND i.id = $2`, e.WorkspaceID.UUID, e.AggregateID)
	ic.Workspace.ID, ic.Issue.ID, ic.Issue.WorkspaceID = e.WorkspaceID.UUID, e.AggregateID, e.WorkspaceID.UUID
	err := row.Scan(&ic.Workspace.Slug, &ic.Issue.ProjectID, &ic.Project.Key, &ic.Issue.Number, &ic.Issue.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		// Deleted: the tombstone (or a late event) names the key and project.
		key, _ := payload["key"].(string)
		pid, _ := payload["project_id"].(string)
		projectID, perr := uuid.Parse(pid)
		if perr != nil {
			return out, false, nil
		}
		if err := s.pool.QueryRow(ctx, `SELECT slug FROM workspaces WHERE id = $1`, e.WorkspaceID.UUID).Scan(&ic.Workspace.Slug); err != nil {
			return out, false, nil
		}
		out.Projects = append(out.Projects, projectID)
		out.Event = PublicEvent{
			ID: e.ID, Type: e.PublicType(), Workspace: ic.Workspace.Slug, OccurredAt: e.OccurredAt, Payload: payload,
			Aggregate: EventAggregate{Type: e.AggregateType, ID: e.AggregateID, Key: key, Version: e.AggregateVersion},
		}
		if e.ActorID.Valid {
			out.Event.Actor = &Ref{ID: e.ActorID.UUID}
		}
		return out, true, nil
	}
	if err != nil {
		return out, false, err
	}
	ic.Project.ID = ic.Issue.ProjectID
	out.Projects = append(out.Projects, ic.Issue.ProjectID)
	described, err := s.Describe(ctx, ic, []eventlog.Event{e})
	if err != nil {
		return out, false, err
	}
	out.Event = described[0]
	return out, true, nil
}
