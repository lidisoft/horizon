package workspace_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/workspace"
)

// client keeps cookies and the CSRF token, like the SPA.
type client struct {
	t       *testing.T
	h       http.Handler
	cookies map[string]*http.Cookie
	csrf    string
}

func (f *fixture) handler() http.Handler {
	auth := identity.NewHTTP(f.ids, false)
	ws := workspace.NewHTTP(f.svc, auth)
	return server.NewEngine(server.Deps{Log: zerolog.Nop(), API: func(api *gin.RouterGroup) {
		api.Use(auth.Authenticate())
		auth.Register(api)
		ws.Register(api)
	}})
}

func (f *fixture) client(email string) *client {
	c := &client{t: f.t, h: f.handler(), cookies: map[string]*http.Cookie{}}
	if email != "" {
		rec := c.do("POST", "/api/v1/auth/login", `{"email":"`+email+`","password":"`+password+`"}`)
		if rec.Code != http.StatusOK {
			f.t.Fatalf("login %s: %d %s", email, rec.Code, rec.Body)
		}
	}
	return c
}

func (c *client) do(method, path, body string) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequestWithContext(c.t.Context(), method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.csrf != "" {
		req.Header.Set(identity.HeaderCSRF, c.csrf)
	}
	for _, ck := range c.cookies {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	for _, ck := range rec.Result().Cookies() {
		c.cookies[ck.Name] = ck
	}
	var me identity.MeResponse
	if json.Unmarshal(rec.Body.Bytes(), &me) == nil && me.CSRFToken != "" {
		c.csrf = me.CSRFToken
	}
	return rec
}

func (c *client) want(rec *httptest.ResponseRecorder, status int, contains string) {
	c.t.Helper()
	if rec.Code != status || !strings.Contains(rec.Body.String(), contains) {
		c.t.Fatalf("got %d %s; want %d containing %q", rec.Code, rec.Body, status, contains)
	}
}

func TestHTTPWorkspaces(t *testing.T) {
	f := newFixture(t)
	f.principal("ada@example.com", false)
	admin, ada := f.client("admin@example.com"), f.client("ada@example.com")

	// Only instance admins create workspaces.
	ada.want(ada.do("POST", "/api/v1/w", `{"slug":"beta","name":"Beta"}`), http.StatusForbidden, `"forbidden"`)
	admin.want(admin.do("POST", "/api/v1/w", `{"slug":"beta","name":"Beta","time_zone":"Europe/Paris"}`), http.StatusCreated, `"slug":"beta"`)
	admin.want(admin.do("GET", "/api/v1/w", ""), http.StatusOK, `"slug":"beta"`)
	ada.want(ada.do("GET", "/api/v1/w", ""), http.StatusOK, `"items":[]`)

	// Non-members don't see that a workspace exists.
	ada.want(ada.do("GET", "/api/v1/w/acme", ""), http.StatusNotFound, `"workspace_not_found"`)
	ada.want(ada.do("GET", "/api/v1/w/acme/members", ""), http.StatusNotFound, `"workspace_not_found"`)
	admin.want(admin.do("GET", "/api/v1/w/acme", ""), http.StatusOK, `"workspace.members.manage"`)

	// Invite Ada, who accepts while signed in; then she sees the workspace
	// but can't administer it.
	admin.want(admin.do("POST", "/api/v1/w/acme/teams", `{"name":"Core"}`), http.StatusCreated, `"name":"Core"`)
	admin.want(admin.do("POST", "/api/v1/w/acme/invitations", `{"emails":["ada@example.com"]}`), http.StatusCreated, `"email":"ada@example.com"`)
	token := f.lastToken()
	ada.want(ada.do("POST", "/api/v1/invitations/lookup", `{"token":"`+token+`"}`), http.StatusOK, `"account_exists":true`)
	ada.want(ada.do("POST", "/api/v1/invitations/accept", `{"token":"`+token+`"}`), http.StatusOK, `"slug":"acme"`)
	ada.want(ada.do("GET", "/api/v1/w/acme", ""), http.StatusOK, `"can":[]`)
	ada.want(ada.do("GET", "/api/v1/w/acme/members", ""), http.StatusOK, `"email":"ada@example.com"`)
	ada.want(ada.do("GET", "/api/v1/w/acme/invitations", ""), http.StatusForbidden, `"forbidden"`)
	ada.want(ada.do("PATCH", "/api/v1/w/acme", `{"name":"Mine"}`), http.StatusForbidden, `"forbidden"`)
}

func TestHTTPAcceptCreatesAccountAndSignsIn(t *testing.T) {
	f := newFixture(t)
	token := f.invite("new@example.com")
	anon := f.client("")
	anon.want(anon.do("POST", "/api/v1/invitations/lookup", `{"token":"`+token+`"}`), http.StatusOK, `"account_exists":false`)
	anon.want(anon.do("POST", "/api/v1/invitations/accept", `{"token":"`+token+`","name":"Newcomer","password":"`+password+`"}`), http.StatusOK, `"slug":"acme"`)
	if anon.cookies["horizon_session"] == nil {
		t.Fatal("no session cookie after creating the account")
	}
	anon.want(anon.do("GET", "/api/v1/me", ""), http.StatusOK, `"name":"Newcomer"`)
	anon.want(anon.do("GET", "/api/v1/w", ""), http.StatusOK, `"slug":"acme"`)
	anon.want(anon.do("POST", "/api/v1/invitations/lookup", `{"token":"`+token+`"}`), http.StatusNotFound, `"invitation_invalid"`)
}

// lastToken is the token of the most recent invitation email.
func (f *fixture) lastToken() string {
	f.t.Helper()
	for i := 0; i < 500; i++ {
		if sent := f.mail.Sent(); len(sent) > 0 {
			_, token, _ := strings.Cut(sent[len(sent)-1].Body, "#token=")
			return strings.Fields(token)[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	f.t.Fatal("no invitation email")
	return ""
}
