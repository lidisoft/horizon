package sso_test

import (
	"bytes"
	"context"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/identity"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/platform/secrets"
	"github.com/lidisoft/horizon/internal/sso"
	"github.com/lidisoft/horizon/internal/testenv"
	"github.com/lidisoft/horizon/internal/workspace"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres|testenv.Dex)) }

const password = "correct horse battery"

type fixture struct {
	t     *testing.T
	pool  *pgxpool.Pool
	ids   *identity.Service
	ws    *workspace.Service
	acc   *access.Service
	svc   *sso.Service
	dex   *testenv.OIDCEnv
	admin identity.Principal
	acme  workspace.Workspace
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	f := &fixture{t: t, pool: testenv.DB(t), dex: testenv.OIDC(t)}
	base, _ := url.Parse(f.dex.AppURL)
	f.ids = identity.New(identity.Options{Pool: f.pool, Log: zerolog.Nop(), BaseURL: base, SessionSecret: "test-session-secret-at-least-32-chars",
		Mail: &platformmail.Memory{}})
	f.acc = access.New(f.pool, nil)
	cfg := configdef.New(f.pool, f.acc, nil, nil)
	f.ws = workspace.New(workspace.Options{Pool: f.pool, Log: zerolog.Nop(), Identity: f.ids, Access: f.acc, Mail: &platformmail.Memory{}, BaseURL: base,
		OnCreate: []workspace.CreateHook{cfg.SeedDefaults}})
	box, err := secrets.New(bytes.Repeat([]byte{5}, 32))
	if err != nil {
		t.Fatal(err)
	}
	f.svc = sso.New(sso.Options{Pool: f.pool, Log: zerolog.Nop(), Identity: f.ids, Workspaces: f.ws, Access: f.acc, Secrets: box, BaseURL: base,
		StateSecret: "test-session-secret-at-least-32-chars"})
	f.ids.SetSignInPolicy(f.svc.Policy)

	// The app, where Dex sends people back.
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	api := engine.Group("/api/v1")
	auth := identity.NewHTTP(f.ids, false)
	api.Use(auth.Authenticate())
	auth.Register(api)
	sso.NewHTTP(f.svc, auth, false).Register(api)
	f.dex.Serve(t, engine)

	if _, err := f.ids.CreateUser(ctx, uuid.NullUUID{}, identity.NewUser{Email: "admin@example.com", Name: "Admin", Password: password, InstanceAdmin: true}); err != nil {
		t.Fatal(err)
	}
	f.admin = f.login("admin@example.com")
	if f.acme, err = f.ws.Create(ctx, uuid.NullUUID{}, workspace.NewWorkspace{Slug: "acme", Name: "Acme", OwnerID: f.admin.User.ID}); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) login(email string) identity.Principal {
	f.t.Helper()
	in, err := f.ids.Login(context.Background(), email, password, identity.Client{})
	if err != nil {
		f.t.Fatalf("login %s: %v", email, err)
	}
	p, err := f.ids.Authenticate(context.Background(), in.Token)
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *fixture) wc(p identity.Principal) workspace.Context {
	f.t.Helper()
	wc, err := f.ws.Enter(context.Background(), p, "acme")
	if err != nil {
		f.t.Fatal(err)
	}
	return wc
}

func (f *fixture) provider(sc sso.Scope, slug string, domains []string, role *uuid.UUID) sso.Provider {
	f.t.Helper()
	name := "Dex " + slug
	p, err := f.svc.Create(context.Background(), sc, sso.Input{Slug: &slug, Name: &name, Issuer: &f.dex.Issuer, ClientID: &f.dex.ClientID,
		ClientSecret: &f.dex.ClientSecret, EmailDomains: &domains, ProvisionRoleID: role})
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

// get and post are http.Client.Get and PostForm with the test's context.
func get(t *testing.T, client *http.Client, target string) *http.Response {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func post(t *testing.T, client *http.Client, target string, form url.Values) *http.Response {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

var formAction = regexp.MustCompile(`<form[^>]*method="post"[^>]*action="([^"]*)"`)

// signIn goes through the provider as a browser would: start, Dex's login
// form, back to the callback. It returns the final URL and the browser.
func (f *fixture) signIn(slug string, user testenv.DexUser) (*url.URL, *http.Client) {
	f.t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	resp := get(f.t, client, f.dex.AppURL+"/api/v1/auth/oidc/"+slug+"/start?next=/w/acme")
	page, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(resp.Request.URL.Path, "/dex/") {
		return resp.Request.URL, client // refused before the provider
	}
	m := formAction.FindSubmatch(page)
	action := resp.Request.URL.String()
	if len(m) > 1 && len(m[1]) > 0 {
		ref, _ := url.Parse(html.UnescapeString(string(m[1])))
		action = resp.Request.URL.ResolveReference(ref).String()
	}
	resp = post(f.t, client, action, url.Values{"login": {user.Email}, "password": {user.Password}})
	_ = resp.Body.Close()
	return resp.Request.URL, client
}

func (f *fixture) me(client *http.Client) string {
	f.t.Helper()
	resp := get(f.t, client, f.dex.AppURL+"/api/v1/me")
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return resp.Status
	}
	return string(b)
}

func TestSignIn(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	grace, alan := testenv.DexUsers[0], testenv.DexUsers[1]
	if _, err := f.ids.CreateUser(ctx, uuid.NullUUID{}, identity.NewUser{Email: grace.Email, Name: grace.Name, Password: password}); err != nil {
		t.Fatal(err)
	}

	// Only instance admins manage instance providers.
	g := f.login(grace.Email)
	if _, err := f.svc.List(ctx, sso.Scope{Principal: g}); !apperr.Is(err, "forbidden") {
		t.Errorf("non-admin listed: %v", err)
	}
	dex := f.provider(sso.Scope{Principal: f.admin}, "dex", nil, nil)
	if dex.RedirectURI != f.dex.AppURL+"/api/v1/auth/oidc/dex/callback" {
		t.Errorf("redirect URI = %s", dex.RedirectURI)
	}
	if list, _ := f.svc.SignInProviders(ctx); len(list) != 1 || list[0].Slug != "dex" {
		t.Errorf("sign-in providers = %+v", list)
	}

	// Grace has an account: the provider signs her in, where she started.
	to, browser := f.signIn("dex", grace)
	if to.Path != "/w/acme" {
		t.Fatalf("landed on %s", to)
	}
	if got := f.me(browser); !strings.Contains(got, grace.Email) {
		t.Fatalf("me = %s", got)
	}
	// Alan has none, and the instance's provider creates none.
	if to, _ := f.signIn("dex", alan); to.Path != "/sign-in" || to.Query().Get("sso_error") != "sso_no_account" {
		t.Errorf("alan landed on %s", to)
	}

	// Acme's provider, for acme.example, provisions Contributors.
	roles, err := f.acc.Roles(ctx, f.acme.ID)
	if err != nil {
		t.Fatal(err)
	}
	var contributor uuid.UUID
	for _, r := range roles {
		if r.Builtin == "contributor" {
			contributor = r.ID
		}
	}
	wc := f.wc(f.admin)
	f.provider(sso.Scope{Workspace: &wc}, "acme-dex", []string{"acme.example"}, &contributor)
	if p, err := f.svc.Discover(ctx, "someone@ACME.example"); err != nil || p.Slug != "acme-dex" {
		t.Errorf("discover = %+v, %v", p, err)
	}
	if _, err := f.svc.Discover(ctx, "someone@other.example"); !apperr.Is(err, "no_sso") {
		t.Errorf("discover other: %v", err)
	}
	to, browser = f.signIn("acme-dex", alan)
	if to.Path != "/w/acme" || !strings.Contains(f.me(browser), alan.Email) {
		t.Fatalf("alan via acme landed on %s", to)
	}
	u, err := f.ids.UserByEmail(ctx, alan.Email)
	if err != nil || u.Name != alan.Name || u.HasPassword {
		t.Errorf("provisioned = %+v, %v", u, err)
	}
	var status string
	if err := f.pool.QueryRow(ctx, `SELECT status FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`, f.acme.ID, u.ID).Scan(&status); err != nil || status != "active" {
		t.Errorf("membership = %q, %v", status, err)
	}
	// Grace's domain isn't acme.example.
	if to, _ := f.signIn("acme-dex", grace); to.Query().Get("sso_error") != "sso_domain" {
		t.Errorf("grace via acme landed on %s", to)
	}

	// A callback without the browser's state is refused.
	resp := get(t, http.DefaultClient, f.dex.AppURL+"/api/v1/auth/oidc/dex/callback?code=x&state=y")
	_ = resp.Body.Close()
	if resp.Request.URL.Query().Get("sso_error") != "sso_state" {
		t.Errorf("forged callback landed on %s", resp.Request.URL)
	}
}

func TestEnforcedSSO(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	grace := testenv.DexUsers[0]
	g, err := f.ids.CreateUser(ctx, uuid.NullUUID{}, identity.NewUser{Email: grace.Email, Name: grace.Name, Password: password})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ws.AddMember(ctx, f.acme.ID, g.ID); err != nil {
		t.Fatal(err)
	}
	session := f.login(grace.Email)

	// No provider yet: nothing to enforce with.
	if _, err := f.svc.SetEnforcement(ctx, f.wc(f.admin), true); !apperr.Is(err, "no_sso_provider") {
		t.Fatalf("enforced without a provider: %v", err)
	}
	if _, err := f.svc.SetEnforcement(ctx, f.wc(session), true); !apperr.Is(err, "forbidden") {
		t.Errorf("a member enforced it: %v", err)
	}
	f.provider(sso.Scope{Principal: f.admin}, "dex", nil, nil)
	if e, err := f.svc.SetEnforcement(ctx, f.wc(f.admin), true); err != nil || !e.Enforced {
		t.Fatalf("enforce = %+v, %v", e, err)
	}

	// Grace's password session closed; her password no longer signs in,
	// nor does a magic link; the provider does. The admin keeps theirs.
	if _, err := f.ids.Login(ctx, grace.Email, password, identity.Client{}); !apperr.Is(err, "sso_required") {
		t.Errorf("password sign-in: %v", err)
	}
	var open int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE user_id = $1`, g.ID).Scan(&open); err != nil || open != 0 {
		t.Errorf("grace's sessions = %d, %v", open, err)
	}
	nonce := identity.NewNonce()
	if err := f.ids.RequestMagicLink(ctx, grace.Email, nonce, identity.Client{}); err != nil {
		t.Fatal(err)
	}
	if to, browser := f.signIn("dex", grace); to.Path != "/w/acme" || !strings.Contains(f.me(browser), grace.Email) {
		t.Errorf("SSO sign-in landed on %s", to)
	}
	f.login("admin@example.com")

	// Off again: the password works.
	if _, err := f.svc.SetEnforcement(ctx, f.wc(f.admin), false); err != nil {
		t.Fatal(err)
	}
	f.login(grace.Email)
}
