package identity_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
)

func TestScopeIncludes(t *testing.T) {
	cases := []struct {
		have, need identity.Scope
		want       bool
	}{
		{identity.ScopeRead, identity.ScopeRead, true},
		{identity.ScopeRead, identity.ScopeWrite, false},
		{identity.ScopeWrite, identity.ScopeRead, true},
		{identity.ScopeWrite, identity.ScopeAdmin, false},
		{identity.ScopeAdmin, identity.ScopeWrite, true},
		{identity.ScopeAdmin, "", false},
		{"", identity.ScopeRead, false},
	}
	for _, c := range cases {
		if got := c.have.Includes(c.need); got != c.want {
			t.Errorf("%q includes %q = %v", c.have, c.need, got)
		}
	}
}

// signedIn returns a session principal for a new user.
func (f fixture) signedIn(t *testing.T, email string) identity.Principal {
	t.Helper()
	f.user(t, email)
	in, err := f.svc.Login(context.Background(), email, password, browser)
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.svc.Authenticate(context.Background(), in.Token)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAPITokens(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := f.signedIn(t, "ada@example.com")
	if p.Scope != identity.ScopeAdmin {
		t.Errorf("session scope = %q", p.Scope)
	}

	_, err := f.svc.CreateAPIToken(ctx, p, identity.NewAPIToken{Name: " ", Scope: "root"})
	wantField(t, err, "name", "required")
	wantField(t, err, "scope", "invalid")
	past := f.clock.Now().Add(-time.Hour)
	_, err = f.svc.CreateAPIToken(ctx, p, identity.NewAPIToken{Name: "ci", Scope: identity.ScopeRead, ExpiresAt: &past})
	wantField(t, err, "expires_at", "past")

	expires := f.clock.Now().Add(24 * time.Hour)
	tok, err := f.svc.CreateAPIToken(ctx, p, identity.NewAPIToken{Name: " CI ", Scope: identity.ScopeWrite, ExpiresAt: &expires})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tok.Token, "hzn_") || len(tok.Token) != 47 || tok.Hint != tok.Token[:8] || tok.Name != "CI" {
		t.Errorf("token = %+v", tok)
	}

	got, err := f.svc.AuthenticateToken(ctx, tok.Token)
	if err != nil || got.User.ID != p.User.ID || got.TokenID != tok.ID || got.Scope != identity.ScopeWrite || got.SessionID != uuid.Nil {
		t.Fatalf("authenticate = %+v, %v", got, err)
	}
	list, _ := f.svc.APITokens(ctx, p)
	if len(list) != 1 || list[0].LastUsedAt == nil || list[0].Hint != tok.Hint {
		t.Errorf("list = %+v", list)
	}

	for _, bad := range []string{"", "hzn_", tok.Token[4:], tok.Token + "x"} {
		_, err := f.svc.AuthenticateToken(ctx, bad)
		wantCode(t, err, "unauthenticated")
	}

	// Tokens expire.
	f.clock.Advance(24 * time.Hour)
	_, err = f.svc.AuthenticateToken(ctx, tok.Token)
	wantCode(t, err, "unauthenticated")

	// Revoking: once, by its owner only.
	other := f.signedIn(t, "grace@example.com")
	wantCode(t, f.svc.RevokeAPIToken(ctx, other, tok.ID), "token_not_found")
	if err := f.svc.RevokeAPIToken(ctx, p, tok.ID); err != nil {
		t.Fatal(err)
	}
	wantCode(t, f.svc.RevokeAPIToken(ctx, p, tok.ID), "token_not_found")

	events, _ := eventlog.History(ctx, f.pool, "api_token", tok.ID)
	if len(events) != 2 || events[0].PublicType() != "api_token.created" || events[1].PublicType() != "api_token.revoked" {
		t.Errorf("events = %v", events)
	}
	for _, e := range events {
		if strings.Contains(string(e.Payload), tok.Token[4:]) || strings.Contains(string(e.Payload), "CI") {
			t.Errorf("payload leaks the token or its name: %s", e.Payload)
		}
	}
}

func TestAPITokenLimitAndDeactivation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := f.signedIn(t, "ada@example.com")
	var last identity.CreatedAPIToken
	for i := range identity.MaxAPITokens {
		var err error
		if last, err = f.svc.CreateAPIToken(ctx, p, identity.NewAPIToken{Name: fmt.Sprint("t", i), Scope: identity.ScopeRead}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := f.svc.CreateAPIToken(ctx, p, identity.NewAPIToken{Name: "one too many", Scope: identity.ScopeRead})
	wantCode(t, err, "too_many_tokens")

	if _, err := f.svc.SetDeactivated(ctx, uuid.NullUUID{}, p.User.ID, true); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.AuthenticateToken(ctx, last.Token)
	wantCode(t, err, "unauthenticated")
	if _, err := f.svc.SetDeactivated(ctx, uuid.NullUUID{}, p.User.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AuthenticateToken(ctx, last.Token); err == nil {
		t.Error("deactivation did not delete the tokens")
	}
}

func TestHTTPBearerTokens(t *testing.T) {
	f := newFixture(t)
	f.user(t, "ada@example.com")
	b := newBrowser(t, f.handler(false))
	b.login("ada@example.com", password)

	create := func(scope string) string {
		rec := b.do("POST", "/api/v1/me/tokens", `{"name":"`+scope+`","scope":"`+scope+`"}`, identity.HeaderCSRF, b.csrf)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %s: %d %s", scope, rec.Code, rec.Body)
		}
		_, after, _ := strings.Cut(rec.Body.String(), `"token":"`)
		return after[:47]
	}
	read, write := create("read"), create("write")

	api := newBrowser(t, f.handler(false)) // no cookies
	bearer := func(tok string) []string { return []string{"Authorization", "Bearer " + tok} }

	if rec := api.do("GET", "/api/v1/me", "", bearer(read)...); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"csrf_token":""`) {
		t.Errorf("GET /me with a read token: %d %s", rec.Code, rec.Body)
	}
	rec := api.do("PATCH", "/api/v1/me", `{"name":"Ada K"}`, bearer(read)...)
	if rec.Code != http.StatusForbidden || problemType(t, rec) != "insufficient_scope" {
		t.Errorf("PATCH with a read token: %d %s", rec.Code, rec.Body)
	}
	if rec := api.do("PATCH", "/api/v1/me", `{"name":"Ada K"}`, bearer(write)...); rec.Code != http.StatusOK {
		t.Errorf("PATCH with a write token (no CSRF needed): %d %s", rec.Code, rec.Body)
	}

	// Credentials are managed from a browser session only.
	for _, r := range []struct{ method, path string }{
		{"GET", "/api/v1/me/tokens"}, {"POST", "/api/v1/me/tokens"}, {"GET", "/api/v1/me/sessions"},
		{"POST", "/api/v1/me/password"}, {"POST", "/api/v1/auth/logout"},
	} {
		rec := api.do(r.method, r.path, `{}`, bearer(write)...)
		if rec.Code != http.StatusForbidden || problemType(t, rec) != "session_required" {
			t.Errorf("%s %s with a token: %d %s", r.method, r.path, rec.Code, rec.Body)
		}
	}

	// A bad Authorization header is rejected even with a valid cookie.
	for _, h := range []string{"Bearer nope", "Basic YWRhOnB3", "Bearer " + read + "x"} {
		rec := b.do("GET", "/api/v1/me", "", "Authorization", h)
		if rec.Code != http.StatusUnauthorized || !strings.HasPrefix(rec.Header().Get("WWW-Authenticate"), "Bearer") {
			t.Errorf("Authorization %q: %d %v", h, rec.Code, rec.Header())
		}
	}

	rec = b.do("GET", "/api/v1/me/tokens", "")
	if rec.Code != http.StatusOK || strings.Count(rec.Body.String(), `"id"`) != 2 || strings.Contains(rec.Body.String(), read) {
		t.Errorf("list: %d %s", rec.Code, rec.Body)
	}
}
