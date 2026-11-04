package notify

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/notify/db"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Status is an issue's status, named.
type Status struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name" example:"In progress"`
	Outcome string    `json:"outcome" enums:"unstarted,started,completed,abandoned"`
}

// IssueRef is the issue an inbox row is about.
type IssueRef struct {
	ID       uuid.UUID `json:"id"`
	Key      string    `json:"key" example:"HZN-42"`
	Title    string    `json:"title"`
	Priority string    `json:"priority"`
	Status   Status    `json:"status"`
}

// Item is an inbox row: an issue and its latest event.
type Item struct {
	ID    uuid.UUID `json:"id"`
	Issue IssueRef  `json:"issue"`
	// Reason is the strongest since the row was last read.
	Reason string `json:"reason" enums:"assigned,mentioned,subscribed"`
	Read   bool   `json:"read"`
	// UnreadCount is the events since it was last read.
	UnreadCount  int        `json:"unread_count"`
	UpdatedAt    time.Time  `json:"updated_at"`
	SnoozedUntil *time.Time `json:"snoozed_until"`
	// Event is the latest event, in the domain event format (null once
	// scrubbed).
	Event *issue.PublicEvent `json:"event"`
}

// Page is a page of the inbox.
type Page struct {
	Items []Item `json:"items"`
	Next  string `json:"next_cursor,omitempty"`
}

// Inbox states.
var states = []string{"unread", "all", "snoozed"}

// Inbox lists the caller's inbox in the workspace (state unread, all or
// snoozed), newest first, on issues they still see.
func (s *Service) Inbox(ctx context.Context, wc workspace.Context, state, cursor string, limit int) (Page, error) {
	if state == "" {
		state = "unread"
	}
	if !slices.Contains(states, state) {
		return Page{}, apperr.Invalid(apperr.Field{Field: "state", Code: "invalid"})
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	arg := db.InboxPageParams{UserID: wc.Principal.User.ID, WorkspaceID: wc.Workspace.ID, State: state, Now: s.now(), MaxRows: int32(limit + 1)}
	if cursor != "" {
		at, id, ok := strings.Cut(cursor, "_")
		t, terr := time.Parse(time.RFC3339Nano, at)
		u, uerr := uuid.Parse(id)
		if !ok || terr != nil || uerr != nil {
			return Page{}, apperr.Invalid(apperr.Field{Field: "cursor", Code: "invalid"})
		}
		arg.BeforeAt, arg.BeforeID = &t, u
	}
	var err error
	if arg.ProjectIds, err = s.visibleProjects(ctx, wc); err != nil {
		return Page{}, err
	}
	rows, err := db.New(s.o.Pool).InboxPage(ctx, arg)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: make([]Item, 0, len(rows))}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1].InboxItem
		page.Next = last.UpdatedAt.Format(time.RFC3339Nano) + "_" + last.ID.String()
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.InboxItem.EventID
	}
	events, err := s.describe(ctx, ids)
	if err != nil {
		return Page{}, err
	}
	for _, r := range rows {
		n := r.InboxItem
		it := Item{
			ID: n.ID, Reason: n.Reason, Read: n.ReadAt != nil, UnreadCount: int(n.UnreadCount), UpdatedAt: n.UpdatedAt, SnoozedUntil: n.SnoozedUntil,
			Issue: IssueRef{ID: n.IssueID, Key: issue.KeyOf(r.ProjectKey, r.Number), Title: r.Title, Priority: r.Priority,
				Status: Status{ID: r.StatusID, Name: r.StatusName, Outcome: r.StatusOutcome}},
		}
		if e, ok := events[n.EventID]; ok {
			it.Event = &e
		}
		page.Items = append(page.Items, it)
	}
	return page, nil
}

// UnreadCount counts the caller's unread rows in the workspace.
func (s *Service) UnreadCount(ctx context.Context, wc workspace.Context) (int, error) {
	projects, err := s.visibleProjects(ctx, wc)
	if err != nil {
		return 0, err
	}
	n, err := db.New(s.o.Pool).UnreadCount(ctx, db.UnreadCountParams{UserID: wc.Principal.User.ID, WorkspaceID: wc.Workspace.ID, Now: s.now(), ProjectIds: projects})
	return int(n), err
}

func (s *Service) visibleProjects(ctx context.Context, wc workspace.Context) ([]uuid.UUID, error) {
	visible, err := s.o.Issues.VisibleProjects(ctx, wc)
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(visible))
	for id := range visible {
		out = append(out, id)
	}
	return out, nil
}

// describe loads and describes events by id; scrubbed ones are missing.
func (s *Service) describe(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]issue.PublicEvent, error) {
	out := map[uuid.UUID]issue.PublicEvent{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.o.Pool.Query(ctx, `
		SELECT id, workspace_id, aggregate_type, aggregate_id, aggregate_version, type, actor_id, occurred_at, payload
		FROM events WHERE id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, err
	}
	var events []eventlog.Event
	for rows.Next() {
		var e eventlog.Event
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.AggregateType, &e.AggregateID, &e.AggregateVersion, &e.Type, &e.ActorID, &e.OccurredAt, &e.Payload); err != nil {
			rows.Close()
			return nil, err
		}
		events = append(events, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, e := range events {
		p, ok, err := s.o.Issues.Public(ctx, e)
		if err != nil {
			return nil, err
		}
		if ok {
			out[e.ID] = p.Event
		}
	}
	return out, nil
}

// ItemUpdate changes an inbox row; nil fields stay as they are.
type ItemUpdate struct {
	Read     *bool `json:"read,omitempty"`
	Archived *bool `json:"archived,omitempty"`
	// SnoozedUntil hides the row until then (new activity brings it back
	// sooner); null wakes it.
	SnoozedUntil NullableTime `json:"snoozed_until" swaggertype:"string" format:"date-time"`
}

// NullableTime is a time that may be absent, null or set.
type NullableTime struct {
	Set   bool
	Value *time.Time
}

// UnmarshalJSON records that the field was present.
func (o *NullableTime) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	var v time.Time
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

// UpdateItem marks a row of the caller's read or unread, archives or
// restores it, or snoozes it.
func (s *Service) UpdateItem(ctx context.Context, wc workspace.Context, id uuid.UUID, in ItemUpdate) error {
	q := db.New(s.o.Pool)
	cur, err := q.InboxItem(ctx, db.InboxItemParams{ID: id, UserID: wc.Principal.User.ID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && cur.WorkspaceID != wc.Workspace.ID) {
		return apperr.NotFound("inbox_item_not_found")
	}
	if err != nil {
		return err
	}
	now := s.now()
	arg := db.UpdateInboxItemParams{ID: id, UserID: wc.Principal.User.ID, ReadAt: cur.ReadAt, ArchivedAt: cur.ArchivedAt, SnoozedUntil: cur.SnoozedUntil}
	if in.Read != nil {
		arg.ReadAt = nil
		if *in.Read {
			arg.ReadAt = &now
		}
	}
	if in.Archived != nil {
		arg.ArchivedAt = nil
		if *in.Archived {
			arg.ArchivedAt = &now
		}
	}
	if in.SnoozedUntil.Set {
		if v := in.SnoozedUntil.Value; v != nil && !v.After(now) {
			return apperr.Invalid(apperr.Field{Field: "snoozed_until", Code: "past"})
		}
		arg.SnoozedUntil = in.SnoozedUntil.Value
	}
	if _, err := q.UpdateInboxItem(ctx, arg); err != nil {
		return err
	}
	s.changed(wc.Principal.User.ID, wc.Workspace.Slug)
	return nil
}

// ReadAll marks the caller's inbox in the workspace read.
func (s *Service) ReadAll(ctx context.Context, wc workspace.Context) error {
	if _, err := db.New(s.o.Pool).ReadAll(ctx, db.ReadAllParams{Now: s.now(), UserID: wc.Principal.User.ID, WorkspaceID: wc.Workspace.ID}); err != nil {
		return err
	}
	s.changed(wc.Principal.User.ID, wc.Workspace.Slug)
	return nil
}

// ArchiveRead archives the caller's read rows in the workspace.
func (s *Service) ArchiveRead(ctx context.Context, wc workspace.Context) error {
	if _, err := db.New(s.o.Pool).ArchiveRead(ctx, db.ArchiveReadParams{Now: s.now(), UserID: wc.Principal.User.ID, WorkspaceID: wc.Workspace.ID}); err != nil {
		return err
	}
	s.changed(wc.Principal.User.ID, wc.Workspace.Slug)
	return nil
}

// changed tells the user's open pages that their inbox in the workspace
// changed.
func (s *Service) changed(userID uuid.UUID, slug string) {
	if s.o.Publish == nil {
		return
	}
	b, _ := json.Marshal(map[string]any{"t": "inbox", "d": map[string]string{"workspace": slug}})
	s.o.Publish(userID, b)
}

// --- Subscriptions ----------------------------------------------------------------

// Subscription says whether the caller follows an issue, and why.
type Subscription struct {
	Subscribed bool   `json:"subscribed"`
	Reason     string `json:"reason,omitempty" enums:"creator,assignee,mention,commenter,manual"`
}

// Subscription returns the caller's subscription to the issue.
func (s *Service) Subscription(ctx context.Context, ic issue.Context) (Subscription, error) {
	reason, err := db.New(s.o.Pool).Subscription(ctx, db.SubscriptionParams{IssueID: ic.Issue.ID, UserID: ic.Principal.User.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Subscription{}, nil
	}
	if err != nil {
		return Subscription{}, err
	}
	return Subscription{Subscribed: true, Reason: reason}, nil
}

// Subscribe makes the caller follow the issue (an existing subscription
// keeps its reason).
func (s *Service) Subscribe(ctx context.Context, ic issue.Context) (Subscription, error) {
	if err := db.New(s.o.Pool).Subscribe(ctx, db.SubscribeParams{WorkspaceID: ic.Workspace.ID, IssueID: ic.Issue.ID, UserID: ic.Principal.User.ID,
		Reason: ReasonManual, CreatedAt: s.now()}); err != nil {
		return Subscription{}, err
	}
	return s.Subscription(ctx, ic)
}

// Unsubscribe stops the caller following the issue. Being assigned or
// mentioned again subscribes them again.
func (s *Service) Unsubscribe(ctx context.Context, ic issue.Context) error {
	return db.New(s.o.Pool).Unsubscribe(ctx, db.UnsubscribeParams{IssueID: ic.Issue.ID, UserID: ic.Principal.User.ID})
}

// --- Settings ----------------------------------------------------------------

// Settings are a user's email choices.
type Settings struct {
	// Email "unread" sends what stays unread for EmailDelay; "off" sends
	// nothing but the digest.
	Email string `json:"email" enums:"unread,off"`
	// Digest sends a daily summary of unread rows in the user's morning.
	Digest bool `json:"digest"`
}

// Settings returns the user's settings (the defaults when never saved).
func (s *Service) Settings(ctx context.Context, userID uuid.UUID) (Settings, error) {
	r, err := db.New(s.o.Pool).Settings(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{Email: "unread"}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	return Settings{Email: r.Email, Digest: r.Digest}, nil
}

// SaveSettings stores the user's settings.
func (s *Service) SaveSettings(ctx context.Context, userID uuid.UUID, in Settings) (Settings, error) {
	if in.Email != "unread" && in.Email != "off" {
		return Settings{}, apperr.Invalid(apperr.Field{Field: "email", Code: "invalid"})
	}
	r, err := db.New(s.o.Pool).SaveSettings(ctx, db.SaveSettingsParams{UserID: userID, Email: in.Email, Digest: in.Digest, UpdatedAt: s.now()})
	if err != nil {
		return Settings{}, err
	}
	return Settings{Email: r.Email, Digest: r.Digest}, nil
}
