// Package sso signs people in through OpenID Connect providers and lets a
// workspace enforce it (docs/security.md#authentication-methods):
// providers are instance-wide (instance admins) or a workspace's
// (workspace.manage), the flow is the authorization code with PKCE
// (github.com/coreos/go-oidc for discovery and ID tokens), and a provider
// of a workspace can provision accounts and memberships just in time.
package sso

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/platform/secrets"
	"github.com/lidisoft/horizon/internal/sso/db"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Options are the service's dependencies.
type Options struct {
	Pool       *pgxpool.Pool
	Log        zerolog.Logger
	Identity   *identity.Service
	Workspaces *workspace.Service
	Access     *access.Service
	Secrets    *secrets.Box
	// BaseURL builds the redirect URIs providers send people back to.
	BaseURL *url.URL
	// StateSecret signs the sign-in state cookie (SESSION_SECRET).
	StateSecret string
	// Client fetches discovery documents, keys and tokens.
	Client *http.Client
	Now    func() time.Time
}

// Service manages providers and SSO sign-ins.
type Service struct {
	o Options

	mu        sync.Mutex
	discovery map[string]discovered // by issuer
}

type discovered struct {
	provider *oidc.Provider
	at       time.Time
}

// discoveryTTL is how long a provider's discovery document is reused.
const discoveryTTL = 10 * time.Minute

// New returns a Service.
func New(o Options) *Service {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Client == nil {
		o.Client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Service{o: o, discovery: map[string]discovered{}}
}

func (s *Service) now() time.Time { return s.o.Now().UTC().Truncate(time.Microsecond) }

// Provider is an OpenID Connect provider.
type Provider struct {
	ID uuid.UUID `json:"id"`
	// Slug names it in URLs (/auth/oidc/{slug}/…).
	Slug   string `json:"slug" example:"acme-okta"`
	Name   string `json:"name" example:"Acme Okta"`
	Issuer string `json:"issuer" example:"https://acme.okta.com"`
	// ClientID is the client registered at the provider; the secret is
	// write-only.
	ClientID string `json:"client_id"`
	// EmailDomains limits who may sign in (and are offered this provider by
	// email); empty: anyone the provider vouches for.
	EmailDomains []string `json:"email_domains"`
	// ProvisionRoleID, for a workspace's provider, creates accounts and
	// memberships just in time with this role; null: only existing members.
	ProvisionRoleID *uuid.UUID `json:"provision_role_id"`
	Enabled         bool       `json:"enabled"`
	// RedirectURI is the address to register at the provider.
	RedirectURI string    `json:"redirect_uri"`
	CreatedAt   time.Time `json:"created_at"`
	Version     int32     `json:"version"`
}

// Input creates or changes a provider; nil fields stay as they are.
type Input struct {
	Slug            *string    `json:"slug,omitempty"`
	Name            *string    `json:"name,omitempty"`
	Issuer          *string    `json:"issuer,omitempty"`
	ClientID        *string    `json:"client_id,omitempty"`
	ClientSecret    *string    `json:"client_secret,omitempty"`
	EmailDomains    *[]string  `json:"email_domains,omitempty"`
	ProvisionRoleID *uuid.UUID `json:"provision_role_id,omitempty"`
	// NoProvision clears ProvisionRoleID.
	NoProvision bool  `json:"no_provision,omitempty"`
	Enabled     *bool `json:"enabled,omitempty"`
}

var (
	slugPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	domainPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
)

func secretAD(id uuid.UUID) string { return "oidc_providers:" + id.String() }

func (s *Service) redirectURI(slug string) string {
	base := ""
	if s.o.BaseURL != nil {
		base = strings.TrimRight(s.o.BaseURL.String(), "/")
	}
	return base + "/api/v1/auth/oidc/" + slug + "/callback"
}

func (s *Service) from(p db.OidcProvider) Provider {
	out := Provider{ID: p.ID, Slug: p.Slug, Name: p.Name, Issuer: p.Issuer, ClientID: p.ClientID, EmailDomains: p.EmailDomains,
		Enabled: p.Enabled, RedirectURI: s.redirectURI(p.Slug), CreatedAt: p.CreatedAt, Version: p.Version}
	if out.EmailDomains == nil {
		out.EmailDomains = []string{}
	}
	if p.ProvisionRoleID.Valid {
		out.ProvisionRoleID = &p.ProvisionRoleID.UUID
	}
	return out
}

// Scope says whose providers: a workspace's (Workspace set) or the
// instance's.
type Scope struct {
	Workspace *workspace.Context
	Principal identity.Principal
}

func (sc Scope) workspaceID() uuid.NullUUID {
	if sc.Workspace == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: sc.Workspace.Workspace.ID, Valid: true}
}

// require checks the caller manages the scope's providers: workspace.manage
// in a workspace, an instance admin with the admin scope for the
// instance's.
func (s *Service) require(ctx context.Context, sc Scope) error {
	if sc.Workspace != nil {
		return access.Require(ctx, s.o.Access, sc.Workspace.Principal, sc.Workspace.Scope(), access.WorkspaceManage)
	}
	if !sc.Principal.User.IsInstanceAdmin || !sc.Principal.Scope.Includes(identity.ScopeAdmin) {
		return apperr.Forbidden("forbidden")
	}
	return nil
}

func (s *Service) provider(ctx context.Context, conn db.DBTX, sc Scope, id uuid.UUID, lock bool) (db.OidcProvider, error) {
	q := db.New(conn)
	var p db.OidcProvider
	var err error
	if lock {
		p, err = q.LockProvider(ctx, id)
	} else {
		p, err = q.Provider(ctx, id)
	}
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && p.WorkspaceID != sc.workspaceID()) {
		return p, apperr.NotFound("provider_not_found")
	}
	return p, err
}

// List returns the scope's providers.
func (s *Service) List(ctx context.Context, sc Scope) ([]Provider, error) {
	if err := s.require(ctx, sc); err != nil {
		return nil, err
	}
	rows, err := db.New(s.o.Pool).Providers(ctx, sc.workspaceID())
	if err != nil {
		return nil, err
	}
	out := make([]Provider, len(rows))
	for i, r := range rows {
		out[i] = s.from(r)
	}
	return out, nil
}

func (s *Service) validate(ctx context.Context, sc Scope, in Input, creating bool) apperr.Fields {
	var fields apperr.Fields
	need := func(field string, v *string) {
		if creating && v == nil {
			fields.Add(field, "required")
		}
	}
	need("slug", in.Slug)
	need("name", in.Name)
	need("issuer", in.Issuer)
	need("client_id", in.ClientID)
	need("client_secret", in.ClientSecret)
	if in.Slug != nil && !slugPattern.MatchString(*in.Slug) {
		fields.Add("slug", "invalid")
	}
	if in.Name != nil {
		*in.Name = strings.TrimSpace(*in.Name)
		if n := len([]rune(*in.Name)); n < 1 || n > 100 {
			fields.Add("name", "length")
		}
	}
	if in.Issuer != nil {
		*in.Issuer = strings.TrimRight(strings.TrimSpace(*in.Issuer), "/")
		u, err := url.Parse(*in.Issuer)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" {
			fields.Add("issuer", "invalid")
		}
	}
	for _, f := range []struct {
		name string
		v    *string
	}{{"client_id", in.ClientID}, {"client_secret", in.ClientSecret}} {
		if f.v != nil {
			*f.v = strings.TrimSpace(*f.v)
			if *f.v == "" || len(*f.v) > 2000 {
				fields.Add(f.name, "invalid")
			}
		}
	}
	if in.EmailDomains != nil {
		if *in.EmailDomains == nil {
			*in.EmailDomains = []string{}
		}
		for i, d := range *in.EmailDomains {
			(*in.EmailDomains)[i] = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(d), "@"))
			if !domainPattern.MatchString((*in.EmailDomains)[i]) {
				fields.Add("email_domains", "invalid")
				break
			}
		}
		slices.Sort(*in.EmailDomains)
		*in.EmailDomains = slices.Compact(*in.EmailDomains)
	}
	if in.ProvisionRoleID != nil {
		if sc.Workspace == nil {
			fields.Add("provision_role_id", "workspace_only")
		} else if roles, err := s.o.Access.Roles(ctx, sc.Workspace.Workspace.ID); err != nil ||
			!slices.ContainsFunc(roles, func(r access.Role) bool { return r.ID == *in.ProvisionRoleID }) {
			fields.Add("provision_role_id", "not_found")
		}
	}
	return fields
}

// Create adds a provider.
func (s *Service) Create(ctx context.Context, sc Scope, in Input) (Provider, error) {
	if err := s.require(ctx, sc); err != nil {
		return Provider{}, err
	}
	if err := s.validate(ctx, sc, in, true).Err(); err != nil {
		return Provider{}, err
	}
	id := uuid.Must(uuid.NewV7())
	sealed, err := s.o.Secrets.Seal(*in.ClientSecret, secretAD(id))
	if err != nil {
		return Provider{}, err
	}
	domains := []string{}
	if in.EmailDomains != nil {
		domains = *in.EmailDomains
	}
	arg := db.CreateProviderParams{ID: id, WorkspaceID: sc.workspaceID(), Slug: *in.Slug, Name: *in.Name, Issuer: *in.Issuer,
		ClientID: *in.ClientID, ClientSecret: sealed, EmailDomains: domains, Enabled: in.Enabled == nil || *in.Enabled, CreatedAt: s.now()}
	if in.ProvisionRoleID != nil {
		arg.ProvisionRoleID = uuid.NullUUID{UUID: *in.ProvisionRoleID, Valid: true}
	}
	var created db.OidcProvider
	err = postgres.InTx(ctx, s.o.Pool, func(tx pgx.Tx) error {
		created, err = db.New(tx).CreateProvider(ctx, arg)
		if isUnique(err) {
			return apperr.Invalid(apperr.Field{Field: "slug", Code: "taken"})
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(sc, created, "OidcProviderCreated", nil))
	})
	return s.from(created), err
}

// Update changes a provider (If-Match: version, 0 skips the check). The
// slug is fixed: it is in the redirect URI registered at the provider.
func (s *Service) Update(ctx context.Context, sc Scope, id uuid.UUID, version int32, in Input) (Provider, error) {
	if err := s.require(ctx, sc); err != nil {
		return Provider{}, err
	}
	if in.Slug != nil {
		return Provider{}, apperr.Invalid(apperr.Field{Field: "slug", Code: "fixed"})
	}
	if err := s.validate(ctx, sc, in, false).Err(); err != nil {
		return Provider{}, err
	}
	var updated db.OidcProvider
	err := postgres.InTx(ctx, s.o.Pool, func(tx pgx.Tx) error {
		cur, err := s.provider(ctx, tx, sc, id, true)
		if err != nil {
			return err
		}
		if version != 0 && version != cur.Version {
			return apperr.Conflict("version_conflict")
		}
		arg := db.UpdateProviderParams{ID: id, Name: cur.Name, Issuer: cur.Issuer, ClientID: cur.ClientID, ClientSecret: cur.ClientSecret,
			EmailDomains: cur.EmailDomains, ProvisionRoleID: cur.ProvisionRoleID, Enabled: cur.Enabled, UpdatedAt: s.now()}
		var changed []string
		if in.Name != nil {
			arg.Name, changed = *in.Name, append(changed, "name")
		}
		if in.Issuer != nil {
			arg.Issuer, changed = *in.Issuer, append(changed, "issuer")
		}
		if in.ClientID != nil {
			arg.ClientID, changed = *in.ClientID, append(changed, "client_id")
		}
		if in.ClientSecret != nil {
			if arg.ClientSecret, err = s.o.Secrets.Seal(*in.ClientSecret, secretAD(id)); err != nil {
				return err
			}
			changed = append(changed, "client_secret")
		}
		if in.EmailDomains != nil {
			arg.EmailDomains, changed = *in.EmailDomains, append(changed, "email_domains")
		}
		if in.ProvisionRoleID != nil {
			arg.ProvisionRoleID, changed = uuid.NullUUID{UUID: *in.ProvisionRoleID, Valid: true}, append(changed, "provision_role_id")
		}
		if in.NoProvision {
			arg.ProvisionRoleID, changed = uuid.NullUUID{}, append(changed, "provision_role_id")
		}
		if in.Enabled != nil {
			arg.Enabled, changed = *in.Enabled, append(changed, "enabled")
		}
		if updated, err = db.New(tx).UpdateProvider(ctx, arg); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(sc, updated, "OidcProviderUpdated", map[string]any{"fields": changed}))
	})
	return s.from(updated), err
}

// Delete removes a provider and the identities linked through it.
func (s *Service) Delete(ctx context.Context, sc Scope, id uuid.UUID) error {
	if err := s.require(ctx, sc); err != nil {
		return err
	}
	return postgres.InTx(ctx, s.o.Pool, func(tx pgx.Tx) error {
		cur, err := s.provider(ctx, tx, sc, id, true)
		if err != nil {
			return err
		}
		if err := db.New(tx).DeleteProvider(ctx, id); err != nil {
			return err
		}
		cur.Version++
		return eventlog.Append(ctx, tx, s.event(sc, cur, "OidcProviderDeleted", nil))
	})
}

func (s *Service) event(sc Scope, p db.OidcProvider, typ string, extra map[string]any) eventlog.Event {
	payload := map[string]any{"slug": p.Slug, "name": p.Name}
	for k, v := range extra {
		payload[k] = v
	}
	actor := sc.Principal.User.ID
	if sc.Workspace != nil {
		actor = sc.Workspace.Principal.User.ID
	}
	return eventlog.Event{
		WorkspaceID: p.WorkspaceID, AggregateType: "oidc_provider", AggregateID: p.ID, AggregateVersion: int64(p.Version), Type: typ,
		ActorID: uuid.NullUUID{UUID: actor, Valid: true}, OccurredAt: s.now(), Payload: eventlog.Payload(payload),
	}
}

// --- Enforcement ----------------------------------------------------------------

// Enforcement is a workspace's SSO requirement.
type Enforcement struct {
	Enforced bool `json:"enforced"`
}

// Enforcement returns the workspace's SSO requirement.
func (s *Service) Enforcement(ctx context.Context, wc workspace.Context) (Enforcement, error) {
	on, err := db.New(s.o.Pool).SSOEnforced(ctx, wc.Workspace.ID)
	return Enforcement{Enforced: on}, err
}

// SetEnforcement turns the workspace's SSO requirement on or off
// (workspace.manage). On, it needs an enabled provider (the workspace's or
// the instance's), and members' sessions opened with a password or a magic
// link close; instance admins and the caller keep theirs.
func (s *Service) SetEnforcement(ctx context.Context, wc workspace.Context, enforced bool) (Enforcement, error) {
	if err := access.Require(ctx, s.o.Access, wc.Principal, wc.Scope(), access.WorkspaceManage); err != nil {
		return Enforcement{}, err
	}
	q := db.New(s.o.Pool)
	if enforced {
		own, err := q.Providers(ctx, uuid.NullUUID{UUID: wc.Workspace.ID, Valid: true})
		if err != nil {
			return Enforcement{}, err
		}
		instance, err := q.SignInProviders(ctx)
		if err != nil {
			return Enforcement{}, err
		}
		if !slices.ContainsFunc(own, func(p db.OidcProvider) bool { return p.Enabled }) && len(instance) == 0 {
			return Enforcement{}, apperr.Conflict("no_sso_provider")
		}
	}
	err := postgres.InTx(ctx, s.o.Pool, func(tx pgx.Tx) error {
		if err := db.New(tx).SetSSOEnforced(ctx, db.SetSSOEnforcedParams{ID: wc.Workspace.ID, Enforced: enforced, At: s.now()}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, eventlog.Event{
			WorkspaceID: uuid.NullUUID{UUID: wc.Workspace.ID, Valid: true}, AggregateType: "workspace", AggregateID: wc.Workspace.ID,
			AggregateVersion: int64(max(wc.Workspace.Version, 1)), Type: "SsoEnforcementChanged", ActorID: uuid.NullUUID{UUID: wc.Principal.User.ID, Valid: true},
			OccurredAt: s.now(), Payload: eventlog.Payload(map[string]any{"enforced": enforced}),
		})
	})
	if err != nil {
		return Enforcement{}, err
	}
	if enforced {
		members, err := q.EnforcedMembers(ctx, wc.Workspace.ID)
		if err != nil {
			return Enforcement{}, err
		}
		for _, id := range members {
			if id == wc.Principal.User.ID {
				continue
			}
			if _, err := s.o.Identity.RevokeSessionsExcept(ctx, id, identity.MethodOIDC); err != nil {
				return Enforcement{}, err
			}
		}
	}
	return Enforcement{Enforced: enforced}, nil
}

// Policy is the identity sign-in policy: members of a workspace enforcing
// SSO sign in with a provider only; instance admins may still use their
// password, to recover from a broken provider.
func (s *Service) Policy(ctx context.Context, u identity.User, method string) error {
	if method == identity.MethodOIDC || u.IsInstanceAdmin {
		return nil
	}
	enforced, err := db.New(s.o.Pool).SSOEnforcedFor(ctx, u.ID)
	if err != nil {
		return err
	}
	if enforced {
		return apperr.Forbidden("sso_required")
	}
	return nil
}

// Rekey reseals every provider's client secret with the current key.
func (s *Service) Rekey(ctx context.Context) (int, error) {
	q := db.New(s.o.Pool)
	rows, err := q.SealedProviderSecrets(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rows {
		if s.o.Secrets.Current(r.ClientSecret) {
			continue
		}
		plain, err := s.o.Secrets.Open(r.ClientSecret, secretAD(r.ID))
		if err != nil {
			return n, err
		}
		sealed, err := s.o.Secrets.Seal(plain, secretAD(r.ID))
		if err != nil {
			return n, err
		}
		if err := q.SetProviderSecret(ctx, db.SetProviderSecretParams{ID: r.ID, ClientSecret: sealed}); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
