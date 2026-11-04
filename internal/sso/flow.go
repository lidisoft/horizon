package sso

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/sso/db"
)

// stateTTL bounds a sign-in round trip through the provider.
const stateTTL = 10 * time.Minute

// state travels in a signed cookie from the start to the callback: it binds
// the callback to the browser that started, and carries the PKCE verifier
// and the nonce the ID token must hold.
type state struct {
	Provider string `json:"p"`
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Next     string `json:"r"`
	Expires  int64  `json:"e"`
}

func random() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (s *Service) sign(st state) string {
	b, _ := json.Marshal(st)
	mac := hmac.New(sha256.New, []byte(s.o.StateSecret))
	mac.Write(b)
	return base64.RawURLEncoding.EncodeToString(b) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Service) verify(cookie string) (state, bool) {
	var st state
	data, sig, ok := strings.Cut(cookie, ".")
	if !ok {
		return st, false
	}
	b, err1 := base64.RawURLEncoding.DecodeString(data)
	got, err2 := base64.RawURLEncoding.DecodeString(sig)
	if err1 != nil || err2 != nil {
		return st, false
	}
	mac := hmac.New(sha256.New, []byte(s.o.StateSecret))
	mac.Write(b)
	if !hmac.Equal(got, mac.Sum(nil)) || json.Unmarshal(b, &st) != nil || s.now().Unix() > st.Expires {
		return st, false
	}
	return st, true
}

// SignInProvider is a provider offered on the sign-in page.
type SignInProvider struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// SignInProviders lists the instance's enabled providers, for the sign-in
// page; workspaces' providers are found by email (Discover) or by link.
func (s *Service) SignInProviders(ctx context.Context) ([]SignInProvider, error) {
	rows, err := db.New(s.o.Pool).SignInProviders(ctx)
	out := make([]SignInProvider, len(rows))
	for i, r := range rows {
		out[i] = SignInProvider{Slug: r.Slug, Name: r.Name}
	}
	return out, err
}

// Discover finds the provider for an email by its domain.
func (s *Service) Discover(ctx context.Context, email string) (SignInProvider, error) {
	_, domain, ok := strings.Cut(strings.ToLower(strings.TrimSpace(email)), "@")
	if !ok || domain == "" {
		return SignInProvider{}, apperr.Invalid(apperr.Field{Field: "email", Code: "invalid"})
	}
	rows, err := db.New(s.o.Pool).ProviderForDomain(ctx, domain)
	if err != nil {
		return SignInProvider{}, err
	}
	if len(rows) == 0 {
		return SignInProvider{}, apperr.NotFound("no_sso")
	}
	return SignInProvider{Slug: rows[0].Slug, Name: rows[0].Name}, nil
}

// oidcProvider discovers (and caches) a provider's endpoints and keys.
func (s *Service) oidcProvider(ctx context.Context, issuer string) (*oidc.Provider, error) {
	s.mu.Lock()
	d, ok := s.discovery[issuer]
	s.mu.Unlock()
	if ok && s.now().Sub(d.at) < discoveryTTL {
		return d.provider, nil
	}
	p, err := oidc.NewProvider(oidc.ClientContext(ctx, s.o.Client), issuer)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.discovery[issuer] = discovered{provider: p, at: s.now()}
	s.mu.Unlock()
	return p, nil
}

func (s *Service) config(ctx context.Context, p db.OidcProvider) (*oauth2.Config, *oidc.Provider, error) {
	op, err := s.oidcProvider(ctx, p.Issuer)
	if err != nil {
		return nil, nil, err
	}
	secret, err := s.o.Secrets.Open(p.ClientSecret, secretAD(p.ID))
	if err != nil {
		return nil, nil, err
	}
	return &oauth2.Config{
		ClientID: p.ClientID, ClientSecret: secret, Endpoint: op.Endpoint(), RedirectURL: s.redirectURI(p.Slug),
		Scopes: []string{oidc.ScopeOpenID, "email", "profile"},
	}, op, nil
}

func (s *Service) enabledProvider(ctx context.Context, slug string) (db.OidcProvider, error) {
	p, err := db.New(s.o.Pool).ProviderBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !p.Enabled) {
		return p, apperr.NotFound("provider_not_found")
	}
	return p, err
}

// Start begins a sign-in: it returns the provider's authorization URL and
// the state cookie's value. next is where to land afterwards.
func (s *Service) Start(ctx context.Context, slug, next string) (authURL, cookie string, err error) {
	p, err := s.enabledProvider(ctx, slug)
	if err != nil {
		return "", "", err
	}
	cfg, _, err := s.config(ctx, p)
	if err != nil {
		return "", "", apperr.BadRequest("the provider can't be reached")
	}
	st := state{Provider: p.Slug, State: random(), Nonce: random(), Verifier: oauth2.GenerateVerifier(), Next: SafeNext(next),
		Expires: s.now().Add(stateTTL).Unix()}
	authURL = cfg.AuthCodeURL(st.State, oidc.Nonce(st.Nonce), oauth2.S256ChallengeOption(st.Verifier))
	return authURL, s.sign(st), nil
}

// SafeNext keeps same-origin paths only, as the sign-in page does.
func SafeNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/"
	}
	return next
}

// claims are what Horizon reads from an ID token.
type claims struct {
	Email         string `json:"email"`
	EmailVerified *bool  `json:"email_verified"`
	Name          string `json:"name"`
	Username      string `json:"preferred_username"`
}

// Finish completes a sign-in from the provider's answer: it checks the
// state cookie, exchanges the code (with the PKCE verifier), verifies the
// ID token, finds or provisions the account and opens a session. It
// returns where to go next. Errors are codes for the sign-in page.
func (s *Service) Finish(ctx context.Context, slug, cookie, gotState, code string, client identity.Client) (identity.SignedIn, string, error) {
	st, ok := s.verify(cookie)
	if !ok || st.Provider != slug || gotState == "" || !hmac.Equal([]byte(st.State), []byte(gotState)) {
		return identity.SignedIn{}, "", apperr.BadRequest("sso_state")
	}
	p, err := s.enabledProvider(ctx, slug)
	if err != nil {
		return identity.SignedIn{}, "", err
	}
	cfg, op, err := s.config(ctx, p)
	if err != nil {
		return identity.SignedIn{}, "", err
	}
	tok, err := cfg.Exchange(context.WithValue(ctx, oauth2.HTTPClient, s.o.Client), code, oauth2.VerifierOption(st.Verifier))
	if err != nil {
		return identity.SignedIn{}, "", apperr.Unauthorized("sso_failed")
	}
	raw, _ := tok.Extra("id_token").(string)
	idt, err := op.Verifier(&oidc.Config{ClientID: p.ClientID}).Verify(oidc.ClientContext(ctx, s.o.Client), raw)
	if err != nil || idt.Nonce != st.Nonce {
		return identity.SignedIn{}, "", apperr.Unauthorized("sso_failed")
	}
	var c claims
	if err := idt.Claims(&c); err != nil {
		return identity.SignedIn{}, "", apperr.Unauthorized("sso_failed")
	}
	userID, err := s.account(ctx, p, idt.Subject, c)
	if err != nil {
		return identity.SignedIn{}, "", err
	}
	in, err := s.o.Identity.OpenSession(ctx, userID, client, identity.MethodOIDC)
	if err != nil {
		return identity.SignedIn{}, "", err
	}
	return in, st.Next, nil
}

// account finds the account the provider vouches for: by its subject, else
// by email (unless the provider says the email is unverified), within the
// allowed domains. A workspace's provider that provisions creates the
// account and the membership when missing.
func (s *Service) account(ctx context.Context, p db.OidcProvider, subject string, c claims) (uuid.UUID, error) {
	q := db.New(s.o.Pool)
	email := strings.TrimSpace(c.Email)
	_, domain, _ := strings.Cut(strings.ToLower(email), "@")
	if len(p.EmailDomains) > 0 && !slices.Contains(p.EmailDomains, domain) {
		return uuid.Nil, apperr.Forbidden("sso_domain")
	}
	userID, err := q.IdentityUser(ctx, db.IdentityUserParams{ProviderID: p.ID, Subject: subject})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		if email == "" || (c.EmailVerified != nil && !*c.EmailVerified) {
			return uuid.Nil, apperr.Forbidden("sso_email")
		}
		u, err := s.o.Identity.UserByEmail(ctx, email)
		switch {
		case err == nil:
			userID = u.ID
		case apperr.Is(err, "user_not_found") && p.WorkspaceID.Valid && p.ProvisionRoleID.Valid:
			name := strings.TrimSpace(c.Name)
			if name == "" {
				name = c.Username
			}
			if name == "" {
				name, _, _ = strings.Cut(email, "@")
			}
			created, err := s.o.Identity.CreateUser(ctx, uuid.NullUUID{}, identity.NewUser{Email: email, Name: name})
			if err != nil {
				return uuid.Nil, err
			}
			userID = created.ID
		case apperr.Is(err, "user_not_found"):
			return uuid.Nil, apperr.Forbidden("sso_no_account")
		default:
			return uuid.Nil, err
		}
	}
	// A workspace's provider: its members, or new ones when it provisions.
	if p.WorkspaceID.Valid {
		status, err := q.Membership(ctx, db.MembershipParams{WorkspaceID: p.WorkspaceID.UUID, UserID: userID})
		switch {
		case errors.Is(err, pgx.ErrNoRows) && p.ProvisionRoleID.Valid:
			if err := s.o.Workspaces.Provision(ctx, p.WorkspaceID.UUID, userID, p.ProvisionRoleID.UUID); err != nil && !apperr.Is(err, "already_member") {
				return uuid.Nil, err
			}
		case errors.Is(err, pgx.ErrNoRows):
			return uuid.Nil, apperr.Forbidden("sso_no_account")
		case err != nil:
			return uuid.Nil, err
		case status != "active":
			return uuid.Nil, apperr.Forbidden("sso_no_account")
		}
	}
	if err := q.LinkIdentity(ctx, db.LinkIdentityParams{ProviderID: p.ID, Subject: subject, UserID: userID, At: s.now()}); err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}
