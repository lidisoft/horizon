package issue_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/workspace"
)

// client keeps cookies and the CSRF token, like the SPA.
type client struct {
	t       *testing.T
	h       http.Handler
	cookies map[string]*http.Cookie
	csrf    string
	header  map[string]string
}

func (f *fixture) handler() http.Handler {
	auth := identity.NewHTTP(f.ids, false)
	ws := workspace.NewHTTP(f.ws, auth)
	projects := project.NewHTTP(f.projects)
	config := configdef.NewHTTP(f.cfg)
	issues := issue.NewHTTP(f.svc, f.projects)
	return server.NewEngine(server.Deps{Log: zerolog.Nop(), API: func(api *gin.RouterGroup) {
		api.Use(auth.Authenticate())
		auth.Register(api)
		ws.Register(api, config.RegisterWorkspace, issues.RegisterWorkspace, func(w *gin.RouterGroup) {
			projects.Register(w, config.RegisterProject, issues.RegisterProject)
		})
	}})
}

func (f *fixture) client(email string) *client {
	c := &client{t: f.t, h: f.handler(), cookies: map[string]*http.Cookie{}, header: map[string]string{}}
	rec := c.do("POST", "/api/v1/auth/login", `{"email":"`+email+`","password":"`+password+`"}`)
	if rec.Code != http.StatusOK {
		f.t.Fatalf("login %s: %d %s", email, rec.Code, rec.Body)
	}
	return c
}

func (c *client) do(method, path, body string) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.csrf != "" {
		req.Header.Set(identity.HeaderCSRF, c.csrf)
	}
	for k, v := range c.header {
		req.Header.Set(k, v)
	}
	c.header = map[string]string{}
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

func TestHTTPIssues(t *testing.T) {
	f := newFixture(t)
	c := f.client("owner@example.com")
	task := f.typeID("Task").String()

	rec := c.do("POST", "/api/v1/w/acme/issues", `{"project":"HZN","type_id":"`+task+`","title":"First"}`)
	c.want(rec, http.StatusCreated, `"key":"HZN-1"`)
	if rec.Header().Get("ETag") != `"1"` {
		t.Errorf("ETag = %q", rec.Header().Get("ETag"))
	}
	c.want(c.do("POST", "/api/v1/w/acme/issues", `{"project":"NOPE","type_id":"`+task+`","title":"x"}`), http.StatusUnprocessableEntity, `"project"`)
	c.want(c.do("POST", "/api/v1/w/acme/issues", `{"project":"HZN","type_id":"`+task+`","title":"Second"}`), http.StatusCreated, `"HZN-2"`)

	c.want(c.do("GET", "/api/v1/w/acme/issues/hzn-1", ""), http.StatusOK, `"title":"First"`)
	c.want(c.do("GET", "/api/v1/w/acme/issues/HZN-9", ""), http.StatusNotFound, "issue_not_found")
	c.want(c.do("GET", "/api/v1/w/acme/projects/HZN/issues?limit=1", ""), http.StatusOK, `"next_cursor"`)

	c.header["If-Match"] = `"1"`
	c.want(c.do("PATCH", "/api/v1/w/acme/issues/HZN-1", `{"title":"Renamed"}`), http.StatusOK, `"version":2`)
	c.header["If-Match"] = `"1"`
	c.want(c.do("PATCH", "/api/v1/w/acme/issues/HZN-1", `{"title":"Stale"}`), http.StatusConflict, "version_conflict")

	c.want(c.do("GET", "/api/v1/w/acme/issues/HZN-1/transitions", ""), http.StatusOK, `"name":"Plan"`)
	c.want(c.do("POST", "/api/v1/w/acme/issues/HZN-1/transition", `{"status_id":"`+f.status("Done").String()+`"}`), http.StatusConflict, "transition_not_allowed")
	c.want(c.do("POST", "/api/v1/w/acme/issues/HZN-1/transition", `{"status_id":"`+f.status("Todo").String()+`"}`), http.StatusOK, f.status("Todo").String())
	c.want(c.do("POST", "/api/v1/w/acme/issues/HZN-1/assign", `{"assignee_id":"`+f.owner.User.ID.String()+`"}`), http.StatusOK, f.owner.User.ID.String())
	c.want(c.do("POST", "/api/v1/w/acme/issues/HZN-1/assign", `{"assignee_id":null}`), http.StatusOK, `"assignee_id":null`)
	c.want(c.do("POST", "/api/v1/w/acme/issues/HZN-1/estimate", `{"estimate":"8"}`), http.StatusOK, `"estimate":"8"`)
	c.want(c.do("POST", "/api/v1/w/acme/issues/HZN-1/prioritize", `{"priority":"high"}`), http.StatusOK, `"priority":"high"`)
	c.want(c.do("POST", "/api/v1/w/acme/issues/HZN-1/rank", `{"after":"HZN-2"}`), http.StatusOK, `"HZN-1"`)

	rec = c.do("POST", "/api/v1/w/acme/issues/HZN-1/links", `{"type":"blocks","target":"HZN-2"}`)
	c.want(rec, http.StatusCreated, `"type":"blocks"`)
	var l issue.Link
	_ = json.Unmarshal(rec.Body.Bytes(), &l)
	c.want(c.do("GET", "/api/v1/w/acme/issues/HZN-2/links", ""), http.StatusOK, `"type":"blocked_by"`)
	c.want(c.do("DELETE", "/api/v1/w/acme/issues/HZN-2/links/"+l.ID.String(), ""), http.StatusNoContent, "")
	c.want(c.do("GET", "/api/v1/w/acme/issues/HZN-1/children", ""), http.StatusOK, `"items":[]`)

	c.want(c.do("POST", "/api/v1/w/acme/issues/HZN-1/archive", ""), http.StatusOK, `"archived_at":"`)
	c.want(c.do("POST", "/api/v1/w/acme/issues/HZN-1/prioritize", `{"priority":"low"}`), http.StatusConflict, "issue_archived")
	c.want(c.do("POST", "/api/v1/w/acme/issues/HZN-1/unarchive", ""), http.StatusOK, `"archived_at":null`)

	// A member who can't see the project gets 404, not 403.
	f.project("SEC", "Secret", "private")
	c.want(c.do("POST", "/api/v1/w/acme/issues", `{"project":"SEC","type_id":"`+task+`","title":"Hidden"}`), http.StatusCreated, `"SEC-1"`)
	f.member("ann@example.com")
	c.want(f.client("ann@example.com").do("GET", "/api/v1/w/acme/issues/SEC-1", ""), http.StatusNotFound, "issue_not_found")
}
