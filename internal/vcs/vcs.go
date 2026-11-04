// Package vcs integrates Git forges, GitHub, GitLab and Forgejo
// (docs/integrations.md): a connection receives the forge's webhooks,
// links the branches, pull requests and commits that name issue keys,
// moves issues when their pull request merges, and keeps one comment on
// each pull request listing its issues and their status.
package vcs

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/platform/secrets"
	"github.com/lidisoft/horizon/internal/vcs/db"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Options are the service's dependencies.
type Options struct {
	Pool    *pgxpool.Pool
	Log     zerolog.Logger
	Access  *access.Service
	Config  *configdef.Service
	Issues  *issue.Service
	Secrets *secrets.Box
	// BaseURL is Horizon's public address: the inbound webhook URL and the
	// issue links in comments.
	BaseURL *url.URL
	// Client calls the forges' APIs; nil uses one with a timeout.
	Client *http.Client
	Now    func() time.Time
}

// Service manages connections and what they link.
type Service struct {
	o Options
}

// New returns a Service.
func New(o Options) *Service {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Client == nil {
		o.Client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Service{o: o}
}

func (s *Service) now() time.Time { return s.o.Now().UTC().Truncate(time.Microsecond) }

// Connection is a forge connection.
type Connection struct {
	ID       uuid.UUID `json:"id"`
	Provider string    `json:"provider" enums:"github,gitlab,forgejo"`
	Name     string    `json:"name" example:"Acme on GitHub"`
	// BaseURL is the forge's web address (https://github.com, a GitLab or
	// Forgejo instance).
	BaseURL string `json:"base_url" example:"https://github.com"`
	// WebhookURL is where the forge sends its webhooks.
	WebhookURL string `json:"webhook_url"`
	// HasToken tells whether Horizon may comment on pull requests.
	HasToken    bool       `json:"has_token"`
	LastEventAt *time.Time `json:"last_event_at"`
	CreatedAt   time.Time  `json:"created_at"`
	Version     int32      `json:"version"`
	// WebhookSecret is answered once, at creation and rotation.
	WebhookSecret string `json:"webhook_secret,omitempty"`
}

// Input creates or changes a connection; nil fields stay as they are.
type Input struct {
	Provider *string `json:"provider,omitempty" enums:"github,gitlab,forgejo"`
	Name     *string `json:"name,omitempty"`
	BaseURL  *string `json:"base_url,omitempty"`
	// Token is an access token able to comment on pull requests; "" removes it.
	Token *string `json:"token,omitempty"`
	// RotateSecret replaces the webhook secret; the answer carries it.
	RotateSecret bool `json:"rotate_secret,omitempty"`
}

func (s *Service) require(ctx context.Context, wc workspace.Context) error {
	return access.Require(ctx, s.o.Access, wc.Principal, wc.Scope(), access.WorkspaceIntegrationsManage)
}

func (s *Service) webhookURL(id uuid.UUID) string {
	base := ""
	if s.o.BaseURL != nil {
		base = strings.TrimRight(s.o.BaseURL.String(), "/")
	}
	return base + "/api/v1/hooks/vcs/" + id.String()
}

func (s *Service) from(c db.VcsConnection) Connection {
	return Connection{ID: c.ID, Provider: c.Provider, Name: c.Name, BaseURL: c.BaseUrl, WebhookURL: s.webhookURL(c.ID),
		HasToken: c.Token != nil, LastEventAt: c.LastEventAt, CreatedAt: c.CreatedAt, Version: c.Version}
}

func secretAD(id uuid.UUID) string { return "vcs_connections:" + id.String() + ":webhook" }
func tokenAD(id uuid.UUID) string  { return "vcs_connections:" + id.String() + ":token" }

func newSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// List returns the workspace's connections (workspace.integrations.manage).
func (s *Service) List(ctx context.Context, wc workspace.Context) ([]Connection, error) {
	if err := s.require(ctx, wc); err != nil {
		return nil, err
	}
	rows, err := db.New(s.o.Pool).Connections(ctx, wc.Workspace.ID)
	if err != nil {
		return nil, err
	}
	out := make([]Connection, len(rows))
	for i, r := range rows {
		out[i] = s.from(r)
	}
	return out, nil
}

func (s *Service) connection(ctx context.Context, conn db.DBTX, wc workspace.Context, id uuid.UUID, lock bool) (db.VcsConnection, error) {
	q := db.New(conn)
	var c db.VcsConnection
	var err error
	if lock {
		c, err = q.LockConnection(ctx, db.LockConnectionParams{WorkspaceID: wc.Workspace.ID, ID: id})
	} else {
		c, err = q.Connection(ctx, db.ConnectionParams{WorkspaceID: wc.Workspace.ID, ID: id})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return c, apperr.NotFound("connection_not_found")
	}
	return c, err
}

// Get returns a connection.
func (s *Service) Get(ctx context.Context, wc workspace.Context, id uuid.UUID) (Connection, error) {
	if err := s.require(ctx, wc); err != nil {
		return Connection{}, err
	}
	c, err := s.connection(ctx, s.o.Pool, wc, id, false)
	return s.from(c), err
}

func validate(in Input, creating bool) apperr.Fields {
	var fields apperr.Fields
	if creating {
		if in.Provider == nil || !slices.Contains(Providers, *in.Provider) {
			fields.Add("provider", "invalid")
		}
		if in.Name == nil {
			fields.Add("name", "required")
		}
		if in.BaseURL == nil {
			fields.Add("base_url", "required")
		}
	}
	if in.Name != nil {
		*in.Name = strings.TrimSpace(*in.Name)
		if n := len([]rune(*in.Name)); n < 1 || n > 100 {
			fields.Add("name", "length")
		}
	}
	if in.BaseURL != nil {
		*in.BaseURL = strings.TrimRight(strings.TrimSpace(*in.BaseURL), "/")
		u, err := url.Parse(*in.BaseURL)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
			fields.Add("base_url", "invalid")
		}
	}
	if in.Token != nil {
		*in.Token = strings.TrimSpace(*in.Token)
		if len(*in.Token) > 1000 {
			fields.Add("token", "length")
		}
	}
	return fields
}

// Create adds a connection; the answer carries the webhook secret, once.
func (s *Service) Create(ctx context.Context, wc workspace.Context, in Input) (Connection, error) {
	if err := s.require(ctx, wc); err != nil {
		return Connection{}, err
	}
	if err := validate(in, true).Err(); err != nil {
		return Connection{}, err
	}
	id := uuid.Must(uuid.NewV7())
	secret := newSecret()
	sealed, err := s.o.Secrets.Seal(secret, secretAD(id))
	if err != nil {
		return Connection{}, err
	}
	var token *string
	if in.Token != nil && *in.Token != "" {
		t, err := s.o.Secrets.Seal(*in.Token, tokenAD(id))
		if err != nil {
			return Connection{}, err
		}
		token = &t
	}
	var created db.VcsConnection
	err = postgres.InTx(ctx, s.o.Pool, func(tx pgx.Tx) error {
		created, err = db.New(tx).CreateConnection(ctx, db.CreateConnectionParams{WorkspaceID: wc.Workspace.ID, ID: id, Provider: *in.Provider,
			Name: *in.Name, BaseUrl: *in.BaseURL, WebhookSecret: sealed, Token: token,
			CreatedBy: uuid.NullUUID{UUID: wc.Principal.User.ID, Valid: true}, CreatedAt: s.now()})
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(wc, created, "VcsConnectionCreated", nil))
	})
	if err != nil {
		return Connection{}, err
	}
	out := s.from(created)
	out.WebhookSecret = secret
	return out, nil
}

// Update changes a connection (If-Match: version, 0 skips the check).
func (s *Service) Update(ctx context.Context, wc workspace.Context, id uuid.UUID, version int32, in Input) (Connection, error) {
	if err := s.require(ctx, wc); err != nil {
		return Connection{}, err
	}
	if in.Provider != nil {
		return Connection{}, apperr.Invalid(apperr.Field{Field: "provider", Code: "fixed"})
	}
	if err := validate(in, false).Err(); err != nil {
		return Connection{}, err
	}
	var updated db.VcsConnection
	var secret string
	err := postgres.InTx(ctx, s.o.Pool, func(tx pgx.Tx) error {
		cur, err := s.connection(ctx, tx, wc, id, true)
		if err != nil {
			return err
		}
		if version != 0 && version != cur.Version {
			return apperr.Conflict("version_conflict")
		}
		arg := db.UpdateConnectionParams{ID: id, Name: cur.Name, BaseUrl: cur.BaseUrl, WebhookSecret: cur.WebhookSecret, Token: cur.Token, UpdatedAt: s.now()}
		var changed []string
		if in.Name != nil {
			arg.Name, changed = *in.Name, append(changed, "name")
		}
		if in.BaseURL != nil {
			arg.BaseUrl, changed = *in.BaseURL, append(changed, "base_url")
		}
		if in.Token != nil {
			arg.Token, changed = nil, append(changed, "token")
			if *in.Token != "" {
				t, err := s.o.Secrets.Seal(*in.Token, tokenAD(id))
				if err != nil {
					return err
				}
				arg.Token = &t
			}
		}
		if in.RotateSecret {
			secret = newSecret()
			if arg.WebhookSecret, err = s.o.Secrets.Seal(secret, secretAD(id)); err != nil {
				return err
			}
			changed = append(changed, "webhook_secret")
		}
		if updated, err = db.New(tx).UpdateConnection(ctx, arg); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(wc, updated, "VcsConnectionUpdated", map[string]any{"fields": changed}))
	})
	if err != nil {
		return Connection{}, err
	}
	out := s.from(updated)
	out.WebhookSecret = secret
	return out, nil
}

// Delete removes a connection and the links it made.
func (s *Service) Delete(ctx context.Context, wc workspace.Context, id uuid.UUID) error {
	if err := s.require(ctx, wc); err != nil {
		return err
	}
	return postgres.InTx(ctx, s.o.Pool, func(tx pgx.Tx) error {
		cur, err := s.connection(ctx, tx, wc, id, true)
		if err != nil {
			return err
		}
		if err := db.New(tx).DeleteConnection(ctx, id); err != nil {
			return err
		}
		cur.Version++
		return eventlog.Append(ctx, tx, s.event(wc, cur, "VcsConnectionDeleted", nil))
	})
}

// event records a change to a connection, never its secrets.
func (s *Service) event(wc workspace.Context, c db.VcsConnection, typ string, extra map[string]any) eventlog.Event {
	payload := map[string]any{"name": c.Name, "provider": c.Provider}
	for k, v := range extra {
		payload[k] = v
	}
	return eventlog.Event{
		WorkspaceID: uuid.NullUUID{UUID: wc.Workspace.ID, Valid: true}, AggregateType: "vcs_connection", AggregateID: c.ID,
		AggregateVersion: int64(c.Version), Type: typ, ActorID: uuid.NullUUID{UUID: wc.Principal.User.ID, Valid: true},
		OccurredAt: s.now(), Payload: eventlog.Payload(payload),
	}
}

// Rekey reseals every connection's secrets with the current key.
func (s *Service) Rekey(ctx context.Context) (int, error) {
	q := db.New(s.o.Pool)
	rows, err := q.SealedConnectionSecrets(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	reseal := func(sealed, ad string) (string, error) {
		plain, err := s.o.Secrets.Open(sealed, ad)
		if err != nil {
			return "", err
		}
		return s.o.Secrets.Seal(plain, ad)
	}
	for _, r := range rows {
		if s.o.Secrets.Current(r.WebhookSecret) && (r.Token == nil || s.o.Secrets.Current(*r.Token)) {
			continue
		}
		secret, err := reseal(r.WebhookSecret, secretAD(r.ID))
		if err != nil {
			return n, err
		}
		token := r.Token
		if token != nil {
			t, err := reseal(*token, tokenAD(r.ID))
			if err != nil {
				return n, err
			}
			token = &t
		}
		if err := q.SetConnectionSecrets(ctx, db.SetConnectionSecretsParams{ID: r.ID, WebhookSecret: secret, Token: token}); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
