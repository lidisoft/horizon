package planning_test

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
	"github.com/lidisoft/horizon/internal/planning"
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

func (f *fixture) client(email string) *client {
	auth := identity.NewHTTP(f.ids, false)
	ws := workspace.NewHTTP(f.ws, auth)
	projects := project.NewHTTP(f.projects)
	config := configdef.NewHTTP(f.cfg)
	issues := issue.NewHTTP(f.issues, f.projects)
	plans := planning.NewHTTP(f.svc)
	h := server.NewEngine(server.Deps{Log: zerolog.Nop(), API: func(api *gin.RouterGroup) {
		api.Use(auth.Authenticate())
		auth.Register(api)
		ws.Register(api, config.RegisterWorkspace, issues.RegisterWorkspace, plans.RegisterWorkspace, func(w *gin.RouterGroup) {
			projects.Register(w, config.RegisterProject, issues.RegisterProject, plans.RegisterProject)
		})
	}})
	c := &client{t: f.t, h: h, cookies: map[string]*http.Cookie{}, header: map[string]string{}}
	rec := c.do("POST", "/api/v1/auth/login", `{"email":"`+email+`","password":"`+password+`"}`)
	if rec.Code != http.StatusOK {
		f.t.Fatalf("login %s: %d %s", email, rec.Code, rec.Body)
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

func TestHTTPIterations(t *testing.T) {
	f := newFixture(t)
	f.create("HZN", "A", "5")
	c := f.client("owner@example.com")
	base := "/api/v1/w/acme/projects/HZN/iterations"

	c.want(c.do("GET", base, ""), http.StatusOK, `"next":{"name":"Sprint 1"`)
	rec := c.do("POST", base, `{"name":"Sprint 1","start_date":"2026-11-09","end_date":"2026-11-20"}`)
	c.want(rec, http.StatusCreated, `"state":"scheduled"`)
	var it planning.Iteration
	_ = json.Unmarshal(rec.Body.Bytes(), &it)
	one := base + "/" + it.ID.String()
	c.want(c.do("POST", base, `{"name":""}`), http.StatusUnprocessableEntity, `"name"`)

	c.want(c.do("POST", one+"/issues", `{"add":["HZN-1"]}`), http.StatusOK, `"points":5`)
	c.want(c.do("GET", "/api/v1/w/acme/issues/HZN-1", ""), http.StatusOK, `"iteration_id":"`+it.ID.String()+`"`)
	c.header["If-Match"] = `"1"`
	c.want(c.do("PATCH", one, `{"goal":"Ship it"}`), http.StatusConflict, "version_conflict")
	c.want(c.do("PATCH", one, `{"goal":"Ship it"}`), http.StatusOK, `"goal":"Ship it"`)
	c.want(c.do("POST", one+"/start", ""), http.StatusOK, `"state":"active"`)
	c.want(c.do("DELETE", one, ""), http.StatusConflict, "iteration_active")
	c.want(c.do("POST", one+"/complete", ""), http.StatusOK, `"policy":"archive_done_rollover_rest","rollover_to":null`)
	c.want(c.do("POST", one+"/archive", ""), http.StatusOK, `"state":"archived"`)
	c.want(c.do("GET", base+"?archived=true", ""), http.StatusOK, `"state":"archived"`)
	c.want(c.do("GET", base+"/nope", ""), http.StatusNotFound, "iteration_not_found")
	rec = c.do("DELETE", one, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}

	// Settings, at project scope.
	settings := "/api/v1/w/acme/projects/HZN/config/settings/iterations"
	c.want(c.do("GET", settings, ""), http.StatusOK, `"completion_policy":{"value":"archive_done_rollover_rest"`)
	c.want(c.do("PUT", settings, `{"completion_policy":"none","iteration_length_weeks":3}`), http.StatusOK, `"value":"none","workspace":"archive_done_rollover_rest","overridden":true`)
	c.want(c.do("PUT", settings, `{"completion_policy":null}`), http.StatusOK, `"completion_policy":{"value":"archive_done_rollover_rest"`)
	c.want(c.do("PUT", settings, `{"completion_policy":"sometimes"}`), http.StatusUnprocessableEntity, "completion_policy")
}

func TestHTTPReleasesAndMilestones(t *testing.T) {
	f := newFixture(t)
	f.create("HZN", "A", "5")
	c := f.client("owner@example.com")

	rec := c.do("POST", "/api/v1/w/acme/projects/HZN/releases", `{"name":"v0.1","release_date":"2026-12-18"}`)
	c.want(rec, http.StatusCreated, `"state":"unreleased"`)
	var r planning.Release
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	c.want(c.do("POST", "/api/v1/w/acme/issues/HZN-1/release", `{"release_id":"`+r.ID.String()+`"}`), http.StatusOK, `"release_id":"`+r.ID.String()+`"`)
	c.want(c.do("GET", "/api/v1/w/acme/projects/HZN/releases", ""), http.StatusOK, `"points":5`)
	c.want(c.do("POST", "/api/v1/w/acme/projects/HZN/releases/"+r.ID.String()+"/release", ""), http.StatusOK, `"state":"released"`)
	c.want(c.do("POST", "/api/v1/w/acme/issues/HZN-1/release", `{"release_id":null}`), http.StatusOK, `"release_id":null`)

	rec = c.do("POST", "/api/v1/w/acme/milestones", `{"name":"Launch","date":"2027-01-15"}`)
	c.want(rec, http.StatusCreated, `"project_id":null`)
	var m planning.Milestone
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	c.want(c.do("POST", "/api/v1/w/acme/milestones/"+m.ID.String()+"/issues", `{"add":["HZN-1"]}`), http.StatusOK, `"issues":1`)
	c.want(c.do("GET", "/api/v1/w/acme/milestones/"+m.ID.String()+"/issues", ""), http.StatusOK, `["HZN-1"]`)
	c.want(c.do("GET", "/api/v1/w/acme/projects/HZN/milestones", ""), http.StatusOK, `"name":"Launch"`)
	rec = c.do("DELETE", "/api/v1/w/acme/milestones/"+m.ID.String(), "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	c.want(c.do("GET", "/api/v1/w/acme/milestones/"+m.ID.String(), ""), http.StatusNotFound, "milestone_not_found")
}
