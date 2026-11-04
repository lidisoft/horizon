package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/version"
	"github.com/lidisoft/horizon/internal/webhook/db"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Delivery limits.
const (
	// RetryFor is how long a failing delivery is retried.
	RetryFor = 24 * time.Hour
	// Timeout bounds one attempt.
	Timeout = 10 * time.Second
	// Retention is how long finished deliveries stay in the log.
	Retention = 30 * 24 * time.Hour
	lease     = 2 * Timeout
	batch     = 20
	parallel  = 4
	excerpt   = 1024
)

// retries are the delays after each failed attempt (the last one repeats).
var retries = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour}

// Consumer is the durable webhook consumer: every event of a workspace.
func (s *Service) Consumer() eventlog.Consumer {
	return eventlog.Consumer{Name: "webhook", Filter: []string{"*"}, Handle: s.Handle}
}

// Handle queues the event for each active endpoint that wants it. It is
// idempotent: an endpoint gets one delivery per event.
func (s *Service) Handle(ctx context.Context, e eventlog.Event) error {
	if !e.WorkspaceID.Valid {
		return nil
	}
	q := db.New(s.o.Pool)
	hooks, err := q.ActiveWebhooks(ctx, e.WorkspaceID.UUID)
	if err != nil || len(hooks) == 0 {
		return err
	}
	public, projects, ok, err := s.describe(ctx, e)
	if err != nil || !ok {
		return err
	}
	var body []byte
	for _, w := range hooks {
		if !wants(w, public.Type, projects) {
			continue
		}
		if body == nil {
			if body, err = json.Marshal(public); err != nil {
				return err
			}
		}
		if _, err := q.CreateDelivery(ctx, db.CreateDeliveryParams{WorkspaceID: e.WorkspaceID.UUID, ID: uuid.Must(uuid.NewV7()),
			WebhookID: w.ID, EventID: e.ID, EventType: public.Type, Body: body, CreatedAt: s.now()}); err != nil {
			return err
		}
	}
	return nil
}

// wants tells whether the endpoint takes an event of the type about the
// projects (none: workspace-wide).
func wants(w db.Webhook, typ string, projects []uuid.UUID) bool {
	if w.ProjectID.Valid && !slices.Contains(projects, w.ProjectID.UUID) {
		return false
	}
	if len(w.EventTypes) == 0 {
		return true
	}
	aggregate, _, _ := strings.Cut(typ, ".")
	return slices.Contains(w.EventTypes, typ) || slices.Contains(w.EventTypes, aggregate+".*")
}

// describe puts the event in the public format, with the projects it is
// about. Issue events name what they refer to, as pushes do; deleted
// issues are told from their tombstone. Events of personal views stay
// private: ok is false.
func (s *Service) describe(ctx context.Context, e eventlog.Event) (issue.PublicEvent, []uuid.UUID, bool, error) {
	if e.AggregateType == "issue" {
		p, ok, err := s.o.Issues.Public(ctx, e)
		return p.Event, p.Projects, ok, err
	}
	var payload map[string]any
	_ = json.Unmarshal(e.Payload, &payload)
	if _, personal := payload["owner_id"]; personal && e.AggregateType == "view" {
		return issue.PublicEvent{}, nil, false, nil
	}
	var slug string
	if err := s.o.Pool.QueryRow(ctx, `SELECT slug FROM workspaces WHERE id = $1`, e.WorkspaceID.UUID).Scan(&slug); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return issue.PublicEvent{}, nil, false, nil
		}
		return issue.PublicEvent{}, nil, false, err
	}
	pe := issue.PublicEvent{
		ID: e.ID, Type: e.PublicType(), Workspace: slug, OccurredAt: e.OccurredAt, Payload: payload,
		Aggregate: issue.EventAggregate{Type: e.AggregateType, ID: e.AggregateID, Version: e.AggregateVersion},
	}
	if e.ActorID.Valid {
		pe.Actor = &issue.Ref{ID: e.ActorID.UUID}
	}
	var projects []uuid.UUID
	if e.AggregateType == "project" {
		projects = []uuid.UUID{e.AggregateID}
	} else if pid, ok := payload["project_id"].(string); ok {
		if id, err := uuid.Parse(pid); err == nil {
			projects = []uuid.UUID{id}
		}
	}
	return pe, projects, true, nil
}

// Send delivers what is due (a periodic job). Every worker may run it:
// deliveries are leased, so each attempt is made once, unless a worker
// dies mid-attempt (then it is made again: at least once).
func (s *Service) Send(ctx context.Context) error {
	now := s.now()
	due, err := db.New(s.o.Pool).LeaseDeliveries(ctx, db.LeaseDeliveriesParams{Now: now, LeaseUntil: now.Add(lease), MaxRows: batch})
	if err != nil || len(due) == 0 {
		return err
	}
	work := make(chan db.LeaseDeliveriesRow)
	var wg sync.WaitGroup
	for range min(parallel, len(due)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for d := range work {
				if err := s.attempt(ctx, d); err != nil {
					s.o.Log.Warn().Err(err).Stringer("delivery", d.ID).Msg("could not record a webhook attempt")
				}
			}
		}()
	}
	for _, d := range due {
		work <- d
	}
	close(work)
	wg.Wait()
	return nil
}

// Sign is the Horizon-Signature of a body.
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// attempt POSTs one delivery and records the outcome: 2xx succeeds;
// anything else is retried until RetryFor has passed since it was queued.
func (s *Service) attempt(ctx context.Context, d db.LeaseDeliveriesRow) error {
	rec := db.RecordAttemptParams{ID: d.ID}
	secret, err := s.o.Secrets.Open(d.Secret, ad(d.WebhookID))
	if err != nil {
		rec.Error = "cannot read the endpoint's secret: " + err.Error()
	} else {
		req, rerr := http.NewRequestWithContext(ctx, http.MethodPost, d.Url, bytes.NewReader(d.Body))
		if rerr != nil {
			rec.Error = rerr.Error()
		} else {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "Horizon-Webhook/"+version.Version)
			req.Header.Set("Horizon-Event", d.EventType)
			req.Header.Set("Horizon-Delivery", d.ID.String())
			req.Header.Set("Horizon-Signature", Sign(secret, d.Body))
			start := time.Now()
			resp, herr := s.o.Client.Do(req)
			ms := int32(time.Since(start).Milliseconds())
			rec.DurationMs = &ms
			if herr != nil {
				rec.Error = herr.Error()
			} else {
				b, _ := io.ReadAll(io.LimitReader(resp.Body, excerpt))
				_ = resp.Body.Close()
				code := int32(resp.StatusCode)
				rec.StatusCode, rec.ResponseExcerpt = &code, strings.ToValidUTF8(string(b), "�")
			}
		}
	}
	now := s.now()
	switch {
	case rec.StatusCode != nil && *rec.StatusCode >= 200 && *rec.StatusCode < 300:
		rec.State, rec.DeliveredAt = "succeeded", &now
	default:
		next := now.Add(retries[min(int(d.Attempts), len(retries)-1)])
		if next.After(d.CreatedAt.Add(RetryFor)) {
			rec.State = "failed"
		} else {
			rec.State, rec.NextAttemptAt = "pending", &next
		}
	}
	if ctx.Err() != nil {
		return nil // the lease runs out and another attempt follows
	}
	return db.New(s.o.Pool).RecordAttempt(context.WithoutCancel(ctx), rec)
}

// Purge drops finished deliveries older than Retention (a periodic job).
func (s *Service) Purge(ctx context.Context) error {
	_, err := db.New(s.o.Pool).PurgeDeliveries(ctx, s.now().Add(-Retention))
	return err
}

// --- The delivery log ----------------------------------------------------------------

// Delivery is an attempt log entry.
type Delivery struct {
	ID        uuid.UUID `json:"id"`
	EventID   uuid.UUID `json:"event_id"`
	EventType string    `json:"event_type" example:"issue.created"`
	// State is pending (to try, or to retry at next_attempt_at), succeeded
	// or failed (gave up after a day).
	State           string          `json:"state" enums:"pending,succeeded,failed"`
	Attempts        int             `json:"attempts"`
	StatusCode      *int            `json:"status_code"`
	ResponseExcerpt string          `json:"response_excerpt"`
	Error           string          `json:"error"`
	DurationMS      *int            `json:"duration_ms"`
	RedeliveryOf    *uuid.UUID      `json:"redelivery_of"`
	CreatedAt       time.Time       `json:"created_at"`
	NextAttemptAt   *time.Time      `json:"next_attempt_at"`
	DeliveredAt     *time.Time      `json:"delivered_at"`
	Body            json.RawMessage `json:"body" swaggertype:"object"`
}

func deliveryFrom(d db.WebhookDelivery) Delivery {
	out := Delivery{ID: d.ID, EventID: d.EventID, EventType: d.EventType, State: d.State, Attempts: int(d.Attempts),
		ResponseExcerpt: d.ResponseExcerpt, Error: d.Error, CreatedAt: d.CreatedAt, NextAttemptAt: d.NextAttemptAt,
		DeliveredAt: d.DeliveredAt, Body: d.Body}
	if d.StatusCode != nil {
		c := int(*d.StatusCode)
		out.StatusCode = &c
	}
	if d.DurationMs != nil {
		ms := int(*d.DurationMs)
		out.DurationMS = &ms
	}
	if d.RedeliveryOf.Valid {
		out.RedeliveryOf = &d.RedeliveryOf.UUID
	}
	if out.State == "pending" && d.Attempts == 0 {
		out.NextAttemptAt = nil // queued: going out now
	}
	return out
}

// DeliveryPage is a page of the log.
type DeliveryPage struct {
	Items []Delivery `json:"items"`
	Next  string     `json:"next_cursor,omitempty"`
}

// Deliveries lists an endpoint's deliveries, newest first.
func (s *Service) Deliveries(ctx context.Context, wc workspace.Context, id uuid.UUID, cursor string, limit int) (DeliveryPage, error) {
	if err := s.require(ctx, wc); err != nil {
		return DeliveryPage{}, err
	}
	if _, err := s.webhook(ctx, s.o.Pool, wc, id, false); err != nil {
		return DeliveryPage{}, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	arg := db.DeliveriesParams{WebhookID: id, MaxRows: int32(limit + 1)}
	if cursor != "" {
		at, rest, ok := strings.Cut(cursor, "_")
		t, terr := time.Parse(time.RFC3339Nano, at)
		u, uerr := uuid.Parse(rest)
		if !ok || terr != nil || uerr != nil {
			return DeliveryPage{}, apperr.Invalid(apperr.Field{Field: "cursor", Code: "invalid"})
		}
		arg.BeforeAt, arg.BeforeID = &t, u
	}
	rows, err := db.New(s.o.Pool).Deliveries(ctx, arg)
	if err != nil {
		return DeliveryPage{}, err
	}
	page := DeliveryPage{Items: make([]Delivery, 0, len(rows))}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		page.Next = last.CreatedAt.Format(time.RFC3339Nano) + "_" + last.ID.String()
	}
	for _, r := range rows {
		page.Items = append(page.Items, deliveryFrom(r))
	}
	return page, nil
}

// Redeliver queues a delivery's body again, as a delivery of its own.
func (s *Service) Redeliver(ctx context.Context, wc workspace.Context, id, deliveryID uuid.UUID) (Delivery, error) {
	if err := s.require(ctx, wc); err != nil {
		return Delivery{}, err
	}
	q := db.New(s.o.Pool)
	orig, err := q.Delivery(ctx, db.DeliveryParams{WebhookID: id, ID: deliveryID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && orig.WorkspaceID != wc.Workspace.ID) {
		return Delivery{}, apperr.NotFound("delivery_not_found")
	}
	if err != nil {
		return Delivery{}, err
	}
	return s.queue(ctx, wc, id, orig.EventID, orig.EventType, orig.Body, uuid.NullUUID{UUID: orig.ID, Valid: true})
}

// Ping queues a "ping" event to try the endpoint.
func (s *Service) Ping(ctx context.Context, wc workspace.Context, id uuid.UUID) (Delivery, error) {
	if err := s.require(ctx, wc); err != nil {
		return Delivery{}, err
	}
	w, err := s.webhook(ctx, s.o.Pool, wc, id, false)
	if err != nil {
		return Delivery{}, err
	}
	eventID := uuid.Must(uuid.NewV7())
	body, err := json.Marshal(issue.PublicEvent{
		ID: eventID, Type: "ping", Workspace: wc.Workspace.Slug, OccurredAt: s.now(),
		Aggregate: issue.EventAggregate{Type: "webhook", ID: w.ID, Version: int64(w.Version)},
		Actor:     &issue.Ref{ID: wc.Principal.User.ID, Name: wc.Principal.User.Name},
		Payload:   map[string]any{"name": w.Name},
	})
	if err != nil {
		return Delivery{}, err
	}
	return s.queue(ctx, wc, id, eventID, "ping", body, uuid.NullUUID{})
}

func (s *Service) queue(ctx context.Context, wc workspace.Context, webhookID, eventID uuid.UUID, typ string, body []byte, of uuid.NullUUID) (Delivery, error) {
	q := db.New(s.o.Pool)
	id := uuid.Must(uuid.NewV7())
	if _, err := q.CreateDelivery(ctx, db.CreateDeliveryParams{WorkspaceID: wc.Workspace.ID, ID: id, WebhookID: webhookID, EventID: eventID,
		EventType: typ, Body: body, RedeliveryOf: of, CreatedAt: s.now()}); err != nil {
		return Delivery{}, err
	}
	d, err := q.Delivery(ctx, db.DeliveryParams{WebhookID: webhookID, ID: id})
	if err != nil {
		return Delivery{}, fmt.Errorf("webhook: queued delivery: %w", err)
	}
	return deliveryFrom(d), nil
}
