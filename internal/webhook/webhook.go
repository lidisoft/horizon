// Package webhook delivers domain events to HTTP endpoints (docs/api.md#webhooks):
// endpoints of a workspace or one of its projects, filtered by event type,
// each POST signed with the endpoint's secret (HMAC-SHA256), retried with
// backoff for a day and logged.
//
// The webhook consumer turns each event into one pending delivery per
// matching endpoint; the delivery job sends them. Order across deliveries
// is not guaranteed: receivers order by the event's id (a UUIDv7).
package webhook

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/platform/egress"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/platform/secrets"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/webhook/db"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Options are the service's dependencies.
type Options struct {
	Pool     *pgxpool.Pool
	Log      zerolog.Logger
	Access   *access.Service
	Projects *project.Service
	Issues   *issue.Service
	Secrets  *secrets.Box
	// AllowPrivate lets endpoints resolve to loopback, private and
	// link-local addresses (an instance whose receivers are on its own
	// network). Off, such requests are refused, so endpoints can't probe
	// the instance's network.
	AllowPrivate bool
	// Client sends deliveries; nil builds one that refuses private
	// addresses unless AllowPrivate, follows no redirects, and times out.
	Client *http.Client
	Now    func() time.Time
}

// Service manages endpoints and their deliveries.
type Service struct {
	o Options
}

// New returns a Service.
func New(o Options) *Service {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Client == nil {
		o.Client = egress.Client(Timeout, o.AllowPrivate)
	}
	return &Service{o: o}
}

func (s *Service) now() time.Time { return s.o.Now().UTC().Truncate(time.Microsecond) }

// Webhook is an endpoint.
type Webhook struct {
	ID uuid.UUID `json:"id"`
	// Project limits it to one project's events (key); empty: the whole
	// workspace, project events and workspace-wide ones.
	Project string `json:"project,omitempty" example:"HZN"`
	Name    string `json:"name" example:"Chat bridge"`
	URL     string `json:"url" example:"https://hooks.example.com/horizon"`
	// EventTypes are public event types ("issue.created") or a whole
	// aggregate ("issue.*"); empty: every event.
	EventTypes []string  `json:"event_types"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
	Version    int32     `json:"version"`
	// Secret signs the deliveries; only answered when created or rotated.
	Secret string `json:"secret,omitempty"`
}

// Input creates or changes an endpoint; nil fields stay as they are.
type Input struct {
	// Project is a project key; "" makes it workspace-wide.
	Project    *string   `json:"project,omitempty"`
	Name       *string   `json:"name,omitempty"`
	URL        *string   `json:"url,omitempty"`
	EventTypes *[]string `json:"event_types,omitempty"`
	Active     *bool     `json:"active,omitempty"`
	// RotateSecret replaces the secret; the answer carries the new one.
	RotateSecret bool `json:"rotate_secret,omitempty"`
}

var eventPattern = regexp.MustCompile(`^[a-z][a-z_]*\.(\*|[a-z][a-z_]*)$`)

func (s *Service) require(ctx context.Context, wc workspace.Context) error {
	return access.Require(ctx, s.o.Access, wc.Principal, wc.Scope(), access.WorkspaceWebhooksManage)
}

// keys names the workspace's projects (archived ones included).
func (s *Service) keys(ctx context.Context, workspaceID uuid.UUID) (map[uuid.UUID]string, error) {
	rows, err := s.o.Pool.Query(ctx, `SELECT id, key FROM projects WHERE workspace_id = $1`, workspaceID)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]string{}
	for rows.Next() {
		var id uuid.UUID
		var key string
		if err := rows.Scan(&id, &key); err != nil {
			rows.Close()
			return nil, err
		}
		out[id] = key
	}
	rows.Close()
	return out, rows.Err()
}

func from(w db.Webhook, keys map[uuid.UUID]string) Webhook {
	out := Webhook{ID: w.ID, Name: w.Name, URL: w.Url, EventTypes: w.EventTypes, Active: w.Active, CreatedAt: w.CreatedAt, Version: w.Version}
	if out.EventTypes == nil {
		out.EventTypes = []string{}
	}
	if w.ProjectID.Valid {
		out.Project = keys[w.ProjectID.UUID]
	}
	return out
}

// List returns the workspace's endpoints (workspace.webhooks.manage).
func (s *Service) List(ctx context.Context, wc workspace.Context) ([]Webhook, error) {
	if err := s.require(ctx, wc); err != nil {
		return nil, err
	}
	rows, err := db.New(s.o.Pool).Webhooks(ctx, wc.Workspace.ID)
	if err != nil {
		return nil, err
	}
	keys, err := s.keys(ctx, wc.Workspace.ID)
	if err != nil {
		return nil, err
	}
	out := make([]Webhook, len(rows))
	for i, r := range rows {
		out[i] = from(r, keys)
	}
	return out, nil
}

// Get returns one endpoint.
func (s *Service) Get(ctx context.Context, wc workspace.Context, id uuid.UUID) (Webhook, error) {
	if err := s.require(ctx, wc); err != nil {
		return Webhook{}, err
	}
	w, err := s.webhook(ctx, s.o.Pool, wc, id, false)
	if err != nil {
		return Webhook{}, err
	}
	keys, err := s.keys(ctx, wc.Workspace.ID)
	return from(w, keys), err
}

func (s *Service) webhook(ctx context.Context, conn db.DBTX, wc workspace.Context, id uuid.UUID, lock bool) (db.Webhook, error) {
	q := db.New(conn)
	var w db.Webhook
	var err error
	if lock {
		w, err = q.LockWebhook(ctx, db.LockWebhookParams{WorkspaceID: wc.Workspace.ID, ID: id})
	} else {
		w, err = q.Webhook(ctx, db.WebhookParams{WorkspaceID: wc.Workspace.ID, ID: id})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return w, apperr.NotFound("webhook_not_found")
	}
	return w, err
}

// newSecret is 32 random bytes, prefixed so it is recognisable.
func newSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return "whsec_" + base64.RawURLEncoding.EncodeToString(b)
}

func ad(id uuid.UUID) string { return "webhooks:" + id.String() }

// validate checks the input against the current values (nil when
// creating), filling what it resolves.
func (s *Service) validate(ctx context.Context, wc workspace.Context, in Input, creating bool) (project uuid.NullUUID, fields apperr.Fields, err error) {
	if creating {
		if in.Name == nil {
			fields.Add("name", "required")
		}
		if in.URL == nil {
			fields.Add("url", "required")
		}
	}
	if in.Name != nil {
		*in.Name = strings.TrimSpace(*in.Name)
		if n := len([]rune(*in.Name)); n < 1 || n > 100 {
			fields.Add("name", "length")
		}
	}
	if in.URL != nil {
		*in.URL = strings.TrimSpace(*in.URL)
		u, perr := url.Parse(*in.URL)
		switch {
		case perr != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http"):
			fields.Add("url", "invalid")
		case u.User != nil:
			fields.Add("url", "credentials")
		case len(*in.URL) > 2000:
			fields.Add("url", "length")
		}
	}
	if in.EventTypes != nil {
		for _, t := range *in.EventTypes {
			if !eventPattern.MatchString(t) {
				fields.Add("event_types", "invalid")
				break
			}
		}
		slices.Sort(*in.EventTypes)
		*in.EventTypes = slices.Compact(*in.EventTypes)
	}
	if in.Project != nil && *in.Project != "" {
		pc, perr := s.o.Projects.Enter(ctx, wc, *in.Project)
		switch {
		case apperr.Is(perr, "project_not_found"):
			fields.Add("project", "not_found")
		case perr != nil:
			return project, fields, perr
		default:
			project = uuid.NullUUID{UUID: pc.Project.ID, Valid: true}
		}
	}
	return project, fields, nil
}

// Create adds an endpoint; the answer carries its secret, shown once.
func (s *Service) Create(ctx context.Context, wc workspace.Context, in Input) (Webhook, error) {
	if err := s.require(ctx, wc); err != nil {
		return Webhook{}, err
	}
	project, fields, err := s.validate(ctx, wc, in, true)
	if err != nil {
		return Webhook{}, err
	}
	if err := fields.Err(); err != nil {
		return Webhook{}, err
	}
	id := uuid.Must(uuid.NewV7())
	secret := newSecret()
	sealed, err := s.o.Secrets.Seal(secret, ad(id))
	if err != nil {
		return Webhook{}, err
	}
	types := []string{}
	if in.EventTypes != nil {
		types = *in.EventTypes
	}
	active := in.Active == nil || *in.Active
	var created db.Webhook
	err = postgres.InTx(ctx, s.o.Pool, func(tx pgx.Tx) error {
		created, err = db.New(tx).CreateWebhook(ctx, db.CreateWebhookParams{WorkspaceID: wc.Workspace.ID, ID: id, ProjectID: project,
			Name: *in.Name, Url: *in.URL, Secret: sealed, EventTypes: types, Active: active,
			CreatedBy: uuid.NullUUID{UUID: wc.Principal.User.ID, Valid: true}, CreatedAt: s.now()})
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(wc, created, "WebhookCreated", nil))
	})
	if err != nil {
		return Webhook{}, err
	}
	keys, err := s.keys(ctx, wc.Workspace.ID)
	out := from(created, keys)
	out.Secret = secret
	return out, err
}

// Update changes an endpoint (If-Match: version, 0 skips the check).
func (s *Service) Update(ctx context.Context, wc workspace.Context, id uuid.UUID, version int32, in Input) (Webhook, error) {
	if err := s.require(ctx, wc); err != nil {
		return Webhook{}, err
	}
	project, fields, err := s.validate(ctx, wc, in, false)
	if err != nil {
		return Webhook{}, err
	}
	if err := fields.Err(); err != nil {
		return Webhook{}, err
	}
	var updated db.Webhook
	var secret string
	err = postgres.InTx(ctx, s.o.Pool, func(tx pgx.Tx) error {
		cur, err := s.webhook(ctx, tx, wc, id, true)
		if err != nil {
			return err
		}
		if version != 0 && version != cur.Version {
			return apperr.Conflict("version_conflict")
		}
		arg := db.UpdateWebhookParams{ID: id, ProjectID: cur.ProjectID, Name: cur.Name, Url: cur.Url, Secret: cur.Secret,
			EventTypes: cur.EventTypes, Active: cur.Active, UpdatedAt: s.now()}
		var changed []string
		if in.Project != nil {
			arg.ProjectID, changed = project, append(changed, "project")
		}
		if in.Name != nil {
			arg.Name, changed = *in.Name, append(changed, "name")
		}
		if in.URL != nil {
			arg.Url, changed = *in.URL, append(changed, "url")
		}
		if in.EventTypes != nil {
			arg.EventTypes, changed = *in.EventTypes, append(changed, "event_types")
		}
		if in.Active != nil {
			arg.Active, changed = *in.Active, append(changed, "active")
		}
		if in.RotateSecret {
			secret = newSecret()
			if arg.Secret, err = s.o.Secrets.Seal(secret, ad(id)); err != nil {
				return err
			}
			changed = append(changed, "secret")
		}
		if updated, err = db.New(tx).UpdateWebhook(ctx, arg); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(wc, updated, "WebhookUpdated", map[string]any{"fields": changed}))
	})
	if err != nil {
		return Webhook{}, err
	}
	keys, err := s.keys(ctx, wc.Workspace.ID)
	out := from(updated, keys)
	out.Secret = secret
	return out, err
}

// Delete removes an endpoint and its delivery log.
func (s *Service) Delete(ctx context.Context, wc workspace.Context, id uuid.UUID) error {
	if err := s.require(ctx, wc); err != nil {
		return err
	}
	return postgres.InTx(ctx, s.o.Pool, func(tx pgx.Tx) error {
		cur, err := s.webhook(ctx, tx, wc, id, true)
		if err != nil {
			return err
		}
		if err := db.New(tx).DeleteWebhook(ctx, id); err != nil {
			return err
		}
		cur.Version++
		return eventlog.Append(ctx, tx, s.event(wc, cur, "WebhookDeleted", nil))
	})
}

// event records a change to an endpoint: its name and scope, never its
// address or secret (workspace events reach every member's pages).
func (s *Service) event(wc workspace.Context, w db.Webhook, typ string, extra map[string]any) eventlog.Event {
	payload := map[string]any{"name": w.Name}
	if w.ProjectID.Valid {
		payload["project_id"] = w.ProjectID.UUID
	}
	for k, v := range extra {
		payload[k] = v
	}
	return eventlog.Event{
		WorkspaceID: uuid.NullUUID{UUID: wc.Workspace.ID, Valid: true}, AggregateType: "webhook", AggregateID: w.ID,
		AggregateVersion: int64(w.Version), Type: typ, ActorID: uuid.NullUUID{UUID: wc.Principal.User.ID, Valid: true},
		OccurredAt: s.now(), Payload: eventlog.Payload(payload),
	}
}

// Rekey seals every endpoint secret with the current key (admin rekey);
// it returns how many it resealed.
func (s *Service) Rekey(ctx context.Context) (int, error) {
	q := db.New(s.o.Pool)
	rows, err := q.SealedSecrets(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rows {
		if s.o.Secrets.Current(r.Secret) {
			continue
		}
		plain, err := s.o.Secrets.Open(r.Secret, ad(r.ID))
		if err != nil {
			return n, err
		}
		sealed, err := s.o.Secrets.Seal(plain, ad(r.ID))
		if err != nil {
			return n, err
		}
		if err := q.SetSecret(ctx, db.SetSecretParams{ID: r.ID, Secret: sealed}); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
