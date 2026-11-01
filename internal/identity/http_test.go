package identity_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/server"
)

// browserClient keeps cookies between requests, like a browser.
type browserClient struct {
	t       *testing.T
	h       http.Handler
	cookies map[string]*http.Cookie
	csrf    string
}

func (f fixture) handler(secure bool) http.Handler {
	auth := identity.NewHTTP(f.svc, secure)
	return server.NewEngine(server.Deps{Log: zerolog.Nop(), API: func(api *gin.RouterGroup) {
		api.Use(auth.Authenticate())
		auth.Register(api)
	}})
}

func newBrowser(t *testing.T, h http.Handler) *browserClient {
	return &browserClient{t: t, h: h, cookies: map[string]*http.Cookie{}}
}

func (b *browserClient) do(method, path, body string, header ...string) *httptest.ResponseRecorder {
	b.t.Helper()
	req := httptest.NewRequestWithContext(b.t.Context(), method, path, strings.NewReader(body))
	req.RemoteAddr = "192.0.2.10:5555"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	for _, c := range b.cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	b.h.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c
		}
	}
	return rec
}

func problemType(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var p server.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("%d %s: %v", rec.Code, rec.Body, err)
	}
	return p.Type
}

func (b *browserClient) login(email, pw string) *httptest.ResponseRecorder {
	b.t.Helper()
	rec := b.do("POST", "/api/v1/auth/login", `{"email":"`+email+`","password":"`+pw+`"}`)
	if rec.Code == http.StatusOK {
		var me identity.MeResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
			b.t.Fatal(err)
		}
		b.csrf = me.CSRFToken
	}
	return rec
}

func TestHTTPSessionCookie(t *testing.T) {
	f := newFixture(t)
	f.user(t, "ada@example.com")

	for _, c := range []struct {
		secure bool
		name   string
	}{{false, "horizon_session"}, {true, "__Host-horizon_session"}} {
		b := newBrowser(t, f.handler(c.secure))
		rec := b.login("ada@example.com", password)
		if rec.Code != http.StatusOK {
			t.Fatalf("login: %d %s", rec.Code, rec.Body)
		}
		ck := b.cookies[c.name]
		if ck == nil || !ck.HttpOnly || ck.Secure != c.secure || ck.SameSite != http.SameSiteLaxMode || ck.Path != "/" || ck.MaxAge != 30*24*3600 {
			t.Errorf("secure=%v: cookie = %+v", c.secure, ck)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q", got)
		}
	}
}

func TestHTTPAuthFlow(t *testing.T) {
	f := newFixture(t)
	f.user(t, "ada@example.com")
	b := newBrowser(t, f.handler(false))

	if rec := b.do("GET", "/api/v1/me", ""); rec.Code != http.StatusUnauthorized || problemType(t, rec) != "unauthenticated" {
		t.Fatalf("anonymous /me: %d %s", rec.Code, rec.Body)
	}
	rec := b.login("ada@example.com", "wrong password!")
	if rec.Code != http.StatusUnauthorized || problemType(t, rec) != "invalid_credentials" {
		t.Fatalf("bad login: %d %s", rec.Code, rec.Body)
	}
	if rec := b.login("ada@example.com", password); rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}

	rec = b.do("GET", "/api/v1/me", "")
	if rec.Code != http.StatusOK || rec.Header().Get("ETag") != `"1"` {
		t.Fatalf("/me: %d %v %s", rec.Code, rec.Header(), rec.Body)
	}
	var me identity.MeResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.User.Email != "ada@example.com" || me.CSRFToken != b.csrf {
		t.Errorf("/me = %+v", me)
	}

	// State changes need the CSRF token.
	rec = b.do("PATCH", "/api/v1/me", `{"name":"Ada King"}`)
	if rec.Code != http.StatusForbidden || problemType(t, rec) != "csrf_invalid" {
		t.Fatalf("PATCH without CSRF: %d %s", rec.Code, rec.Body)
	}
	rec = b.do("PATCH", "/api/v1/me", `{"name":"Ada King"}`, identity.HeaderCSRF, b.csrf, "If-Match", `"1"`)
	if rec.Code != http.StatusOK || rec.Header().Get("ETag") != `"2"` {
		t.Fatalf("PATCH: %d %s", rec.Code, rec.Body)
	}
	rec = b.do("PATCH", "/api/v1/me", `{"name":"Ada"}`, identity.HeaderCSRF, b.csrf, "If-Match", `"1"`)
	if rec.Code != http.StatusConflict || problemType(t, rec) != "version_conflict" {
		t.Fatalf("stale PATCH: %d %s", rec.Code, rec.Body)
	}
	rec = b.do("PATCH", "/api/v1/me", `{"nickname":"Ada"}`, identity.HeaderCSRF, b.csrf)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown field: %d %s", rec.Code, rec.Body)
	}
	rec = b.do("POST", "/api/v1/me/password", `{"current_password":"`+password+`","new_password":"x"}`, identity.HeaderCSRF, b.csrf)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `{"field":"new_password","code":"too_short"}`) {
		t.Fatalf("weak password: %d %s", rec.Code, rec.Body)
	}

	// Browsers flag cross-site requests; those are refused outright.
	rec = b.do("POST", "/api/v1/auth/logout", "", identity.HeaderCSRF, b.csrf, "Sec-Fetch-Site", "cross-site")
	if rec.Code != http.StatusForbidden || problemType(t, rec) != "cross_site_request" {
		t.Fatalf("cross-site logout: %d %s", rec.Code, rec.Body)
	}

	rec = b.do("GET", "/api/v1/me/sessions", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"current":true`) || !strings.Contains(rec.Body.String(), `"ip":"192.0.2.10"`) {
		t.Fatalf("sessions: %d %s", rec.Code, rec.Body)
	}

	if rec := b.do("POST", "/api/v1/auth/logout", "", identity.HeaderCSRF, b.csrf); rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body)
	}
	if _, ok := b.cookies["horizon_session"]; ok {
		t.Error("logout kept the cookie")
	}
	if rec := b.do("GET", "/api/v1/me", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("/me after logout: %d", rec.Code)
	}
}

func TestHTTPLoginThrottleSetsRetryAfter(t *testing.T) {
	f := newFixture(t)
	b := newBrowser(t, f.handler(false))
	var rec *httptest.ResponseRecorder
	for range 11 {
		rec = b.login("nobody@example.com", "wrong password!")
	}
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "900" {
		t.Fatalf("throttled login: %d %v %s", rec.Code, rec.Header(), rec.Body)
	}
}

func TestHTTPForgotPasswordDoesNotRevealAccounts(t *testing.T) {
	f := newFixture(t)
	f.user(t, "ada@example.com")
	b := newBrowser(t, f.handler(false))
	for _, email := range []string{"ada@example.com", "nobody@example.com"} {
		if rec := b.do("POST", "/api/v1/auth/password/forgot", `{"email":"`+email+`"}`); rec.Code != http.StatusAccepted {
			t.Errorf("%s: %d %s", email, rec.Code, rec.Body)
		}
	}
	if rec := b.do("POST", "/api/v1/auth/password/forgot", `{"email":"not an email"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid email: %d", rec.Code)
	}
}
