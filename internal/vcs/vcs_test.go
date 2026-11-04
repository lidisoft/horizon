package vcs_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/platform/egress"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/platform/secrets"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/testenv"
	"github.com/lidisoft/horizon/internal/vcs"
	"github.com/lidisoft/horizon/internal/workspace"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres|testenv.Forgejo)) }

const password = "correct horse battery"

type fixture struct {
	t      *testing.T
	pool   *pgxpool.Pool
	ids    *identity.Service
	ws     *workspace.Service
	cfg    *configdef.Service
	issues *issue.Service
	svc    *vcs.Service
	owner  identity.Principal
	acme   workspace.Workspace
	last   uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, pool: testenv.DB(t)}
	ctx := context.Background()
	base := &url.URL{Scheme: "https", Host: "horizon.example.com"}
	f.ids = identity.New(identity.Options{Pool: f.pool, Log: zerolog.Nop(), BaseURL: base, SessionSecret: "test-session-secret-at-least-32-chars"})
	acc := access.New(f.pool, nil)
	f.cfg = configdef.New(f.pool, acc, nil, nil)
	f.ws = workspace.New(workspace.Options{Pool: f.pool, Log: zerolog.Nop(), Identity: f.ids, Access: acc, Mail: &platformmail.Memory{}, BaseURL: base,
		OnCreate: []workspace.CreateHook{f.cfg.SeedDefaults}})
	projects := project.New(f.pool, acc, nil)
	f.issues = issue.New(f.pool, acc, f.cfg, projects, nil)
	box, err := secrets.New(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	f.svc = vcs.New(vcs.Options{Pool: f.pool, Log: zerolog.Nop(), Access: acc, Config: f.cfg, Issues: f.issues, Secrets: box,
		BaseURL: base, Client: egress.Client(10*time.Second, true)})
	if _, err := f.ids.CreateUser(ctx, uuid.NullUUID{}, identity.NewUser{Email: "owner@example.com", Name: "Owner", Password: password}); err != nil {
		t.Fatal(err)
	}
	in, err := f.ids.Login(ctx, "owner@example.com", password, identity.Client{})
	if err != nil {
		t.Fatal(err)
	}
	if f.owner, err = f.ids.Authenticate(ctx, in.Token); err != nil {
		t.Fatal(err)
	}
	if f.acme, err = f.ws.Create(ctx, uuid.NullUUID{}, workspace.NewWorkspace{Slug: "acme", Name: "Acme", OwnerID: f.owner.User.ID}); err != nil {
		t.Fatal(err)
	}
	key, name, vis := "HZN", "Horizon", project.VisibilityWorkspace
	if _, err := projects.Create(ctx, f.wc(), project.Input{Key: &key, Name: &name, Visibility: &vis}); err != nil {
		t.Fatal(err)
	}
	pc, err := projects.Enter(ctx, f.wc(), "HZN")
	if err != nil {
		t.Fatal(err)
	}
	r, err := f.cfg.Resolve(ctx, f.pool, f.acme.ID, uuid.NullUUID{UUID: pc.Project.ID, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Outbox relay", "Replay after reconnect"} {
		if _, err := f.issues.Create(ctx, pc, issue.NewIssue{TypeID: r.IssueTypes[0].ID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	// Merged pull requests finish their issues (off by default).
	if _, err := f.cfg.SetGitSettings(ctx, configdef.Target{Principal: f.owner, WorkspaceID: f.acme.ID},
		configdef.GitSettingsInput{MergeStatus: json.RawMessage(`"@completed"`)}); err != nil {
		t.Fatal(err)
	}
	f.consume() // the setup's events
	return f
}

func (f *fixture) wc() workspace.Context {
	f.t.Helper()
	wc, err := f.ws.Enter(context.Background(), f.owner, "acme")
	if err != nil {
		f.t.Fatal(err)
	}
	return wc
}

// consume hands the issue events since the last call to the vcs consumer.
func (f *fixture) consume() {
	f.t.Helper()
	ctx := context.Background()
	rows, err := f.pool.Query(ctx, `
		SELECT id, workspace_id, aggregate_type, aggregate_id, aggregate_version, type, actor_id, occurred_at, payload
		FROM events WHERE id > $1 ORDER BY id`, f.last)
	if err != nil {
		f.t.Fatal(err)
	}
	var events []eventlog.Event
	for rows.Next() {
		var e eventlog.Event
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.AggregateType, &e.AggregateID, &e.AggregateVersion, &e.Type, &e.ActorID, &e.OccurredAt, &e.Payload); err != nil {
			f.t.Fatal(err)
		}
		events = append(events, e)
	}
	rows.Close()
	filter := f.svc.Consumer().Filter
	for _, e := range events {
		f.last = e.ID
		for _, want := range filter {
			if want == e.AggregateType+"."+e.Type {
				if err := f.svc.Handle(ctx, e); err != nil {
					f.t.Fatal(err)
				}
			}
		}
	}
}

func (f *fixture) links(key string) []vcs.Link {
	f.t.Helper()
	ic, err := f.issues.Enter(context.Background(), f.wc(), key)
	if err != nil {
		f.t.Fatal(err)
	}
	links, err := f.svc.Links(context.Background(), ic)
	if err != nil {
		f.t.Fatal(err)
	}
	return links
}

func (f *fixture) status(key string) string {
	f.t.Helper()
	var name string
	if err := f.pool.QueryRow(context.Background(), `SELECT s.name FROM issues i JOIN statuses s ON s.id = i.status_id JOIN projects p ON p.id = i.project_id
		WHERE p.key || '-' || i.number = $1`, key).Scan(&name); err != nil {
		f.t.Fatal(err)
	}
	return name
}

func (f *fixture) connect(provider, base, token string) vcs.Connection {
	f.t.Helper()
	name := provider + " connection"
	in := vcs.Input{Provider: &provider, Name: &name, BaseURL: &base}
	if token != "" {
		in.Token = &token
	}
	c, err := f.svc.Create(context.Background(), f.wc(), in)
	if err != nil {
		f.t.Fatal(err)
	}
	return c
}

// eventually polls cond for up to 20 s.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func kinds(links []vcs.Link) string {
	var out []string
	for _, l := range links {
		out = append(out, l.Kind+":"+l.State)
	}
	return strings.Join(out, ",")
}

// TestForgejo drives a real Forgejo: its webhooks link a branch, a commit
// and a pull request; Horizon comments on the pull request; merging it
// moves the issue to Done, and the comment follows.
func TestForgejo(t *testing.T) {
	f := newFixture(t)
	forge := testenv.Forge(t)
	ctx := context.Background()

	// Horizon's inbound hook, where the forge reaches it.
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	vcs.NewHTTP(f.svc).RegisterHooks(engine.Group("/api/v1"))
	ln, hookBase := forge.ListenForForge(t)
	srv := &http.Server{Handler: engine, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	c := f.connect(vcs.Forgejo, forge.URL, forge.Token)
	repo := "relay-" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	forge.API(t, http.MethodPost, "/user/repos", map[string]any{"name": repo, "auto_init": true, "default_branch": "main"}, nil)
	full := forge.User + "/" + repo
	forge.API(t, http.MethodPost, "/repos/"+full+"/hooks", map[string]any{
		"type": "forgejo", "active": true, "events": []string{"push", "pull_request", "create", "delete"},
		"config": map[string]string{"url": hookBase + "/api/v1/hooks/vcs/" + c.ID.String(), "content_type": "json", "secret": c.WebhookSecret},
	}, nil)

	// A branch named after HZN-1, a commit naming HZN-1.
	forge.API(t, http.MethodPost, "/repos/"+full+"/branches", map[string]any{"new_branch_name": "hzn-1-outbox", "old_branch_name": "main"}, nil)
	forge.API(t, http.MethodPost, "/repos/"+full+"/contents/relay.txt", map[string]any{
		"content": base64.StdEncoding.EncodeToString([]byte("relay\n")), "message": "HZN-1: relay events\n\nWith retries.", "branch": "hzn-1-outbox",
	}, nil)
	eventually(t, "branch and commit links", func() bool {
		k := kinds(f.links("HZN-1"))
		return strings.Contains(k, "branch:open") && strings.Contains(k, "commit:")
	})

	// The pull request names HZN-2 too, in its description.
	var pr struct{ Number int }
	forge.API(t, http.MethodPost, "/repos/"+full+"/pulls", map[string]any{"head": "hzn-1-outbox", "base": "main", "title": "Outbox relay", "body": "Also prepares HZN-2."}, &pr)
	eventually(t, "pull request links", func() bool {
		return strings.HasPrefix(kinds(f.links("HZN-1")), "pull_request:open") && strings.HasPrefix(kinds(f.links("HZN-2")), "pull_request:open")
	})
	links := f.links("HZN-1")
	if links[0].Title != "Outbox relay" || links[0].Ref != fmt.Sprint(pr.Number) || links[0].Repository != full || links[0].Author != forge.User {
		t.Errorf("pull request link = %+v", links[0])
	}

	// The comment lists both issues and their status.
	comments := func() []struct {
		ID   int64
		Body string
	} {
		var out []struct {
			ID   int64
			Body string
		}
		forge.API(t, http.MethodGet, fmt.Sprintf("/repos/%s/issues/%d/comments", full, pr.Number), nil, &out)
		return out
	}
	if err := f.svc.WriteComments(ctx); err != nil {
		t.Fatal(err)
	}
	cs := comments()
	if len(cs) != 1 || !strings.Contains(cs[0].Body, "HZN-1") || !strings.Contains(cs[0].Body, "HZN-2") || !strings.Contains(cs[0].Body, "| Backlog |") ||
		!strings.Contains(cs[0].Body, "https://horizon.example.com/w/acme/issues/HZN-1") {
		t.Fatalf("comments = %+v", cs)
	}

	// Merged: both issues move to Done (the nearest completed status), and
	// the same comment says so.
	forge.API(t, http.MethodPost, fmt.Sprintf("/repos/%s/pulls/%d/merge", full, pr.Number), map[string]any{"Do": "merge"}, nil)
	eventually(t, "the merge", func() bool { return f.status("HZN-1") == "Done" && f.status("HZN-2") == "Done" })
	if got := kinds(f.links("HZN-1")); !strings.HasPrefix(got, "pull_request:merged") {
		t.Errorf("links = %s", got)
	}
	f.consume()
	time.Sleep(10 * time.Millisecond) // past the dirty mark
	if err := f.svc.WriteComments(ctx); err != nil {
		t.Fatal(err)
	}
	cs = comments()
	if len(cs) != 1 || strings.Contains(cs[0].Body, "Backlog") || !strings.Contains(cs[0].Body, "| Done |") {
		t.Fatalf("comments after merge = %+v", cs)
	}

	// The history tells it, as the system.
	ic, _ := f.issues.Enter(ctx, f.wc(), "HZN-1")
	page, err := f.issues.Activity(ctx, ic, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, e := range page.Items {
		types = append(types, e.Type)
		if e.Type == "issue.status_updated" && (e.Actor != nil || e.Payload["by"] != "pull_request_merged") {
			t.Errorf("status event = %+v", e)
		}
	}
	if !strings.Contains(strings.Join(types, ","), "issue.git_updated") || !strings.Contains(strings.Join(types, ","), "issue.git_linked") {
		t.Errorf("activity = %v", types)
	}
}

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// fakeAPI records comment calls and answers like a forge.
type fakeAPI struct {
	mu    sync.Mutex
	calls []string
	auth  []string
	body  string
}

func (a *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	var in struct{ Body string }
	_ = json.Unmarshal(b, &in)
	a.mu.Lock()
	a.calls = append(a.calls, r.Method+" "+r.URL.EscapedPath())
	a.auth = append(a.auth, r.Header.Get("Authorization")+r.Header.Get("PRIVATE-TOKEN"))
	a.body = in.Body
	a.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"id": 777}`))
}

// TestGitHubAndGitLab posts deliveries signed as GitHub and GitLab sign
// them, and checks the comments go to their APIs.
func TestGitHubAndGitLab(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	api := &fakeAPI{}
	srv := httptest.NewServer(api)
	defer srv.Close()

	// GitHub (Enterprise: the API under /api/v3).
	gh := f.connect(vcs.GitHub, srv.URL, "ghp_secret")
	pr := func(action string, merged bool) []byte {
		b, _ := json.Marshal(map[string]any{
			"action": action, "number": 7,
			"repository": map[string]any{"full_name": "acme/relay", "html_url": "https://github.example.com/acme/relay"},
			"pull_request": map[string]any{"number": 7, "title": "HZN-2 replay", "body": "", "html_url": "https://github.example.com/acme/relay/pull/7",
				"state": map[bool]string{true: "closed", false: "open"}[merged], "merged": merged, "user": map[string]any{"login": "ada"},
				"head": map[string]any{"ref": "replay"}},
		})
		return b
	}
	header := func(event, sig string) http.Header {
		h := http.Header{}
		h.Set("X-GitHub-Event", event)
		h.Set("X-Hub-Signature-256", sig)
		return h
	}
	body := pr("opened", false)
	if err := f.svc.Receive(ctx, gh.ID, header("pull_request", sign("wrong", body)), body); !apperr.Is(err, "bad_signature") {
		t.Fatalf("a forged delivery: %v", err)
	}
	if err := f.svc.Receive(ctx, gh.ID, header("pull_request", sign(gh.WebhookSecret, body)), body); err != nil {
		t.Fatal(err)
	}
	ping := []byte(`{"zen":"hi"}`)
	if err := f.svc.Receive(ctx, gh.ID, header("ping", sign(gh.WebhookSecret, ping)), ping); err != nil {
		t.Fatal(err)
	}
	if got := kinds(f.links("HZN-2")); got != "pull_request:open" {
		t.Fatalf("links = %s", got)
	}
	if err := f.svc.WriteComments(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	// Merged: the issue moves to Done; the comment is edited.
	body = pr("closed", true)
	if err := f.svc.Receive(ctx, gh.ID, header("pull_request", sign(gh.WebhookSecret, body)), body); err != nil {
		t.Fatal(err)
	}
	if got := f.status("HZN-2"); got != "Done" {
		t.Errorf("status = %s", got)
	}
	f.consume()
	time.Sleep(10 * time.Millisecond)
	if err := f.svc.WriteComments(ctx); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(api.calls, ","); got != "POST /api/v3/repos/acme/relay/issues/7/comments,PATCH /api/v3/repos/acme/relay/issues/comments/777" {
		t.Errorf("calls = %s", got)
	}
	if api.auth[0] != "Bearer ghp_secret" || !strings.Contains(api.body, "| Done |") {
		t.Errorf("auth = %v, body = %q", api.auth, api.body)
	}

	// GitLab: a push of a new branch with a commit, then a merge request,
	// with the merge setting off for the project.
	if _, err := f.cfg.SetGitSettings(ctx, configdef.Target{Principal: f.owner, WorkspaceID: f.acme.ID}, configdef.GitSettingsInput{MergeStatus: json.RawMessage(`"@off"`)}); err != nil {
		t.Fatal(err)
	}
	gl := f.connect(vcs.GitLab, srv.URL, "glpat-secret")
	glHeader := func(event, token string) http.Header {
		h := http.Header{}
		h.Set("X-Gitlab-Event", event)
		h.Set("X-Gitlab-Token", token)
		return h
	}
	project := map[string]any{"id": 42, "path_with_namespace": "acme/relay", "web_url": "https://gitlab.example.com/acme/relay"}
	push, _ := json.Marshal(map[string]any{"object_kind": "push", "ref": "refs/heads/hzn-1-relay", "before": "0000000000000000000000000000000000000000",
		"after": "abc", "project": project,
		"commits": []any{map[string]any{"id": "abc123", "message": "Relay events (hzn-1)", "url": "https://gitlab.example.com/acme/relay/-/commit/abc123", "author": map[string]any{"name": "Ada"}}}})
	if err := f.svc.Receive(ctx, gl.ID, glHeader("Push Hook", "wrong"), push); !apperr.Is(err, "bad_signature") {
		t.Fatalf("a forged GitLab delivery: %v", err)
	}
	if err := f.svc.Receive(ctx, gl.ID, glHeader("Push Hook", gl.WebhookSecret), push); err != nil {
		t.Fatal(err)
	}
	mr, _ := json.Marshal(map[string]any{"object_kind": "merge_request", "user": map[string]any{"username": "ada"}, "project": project,
		"object_attributes": map[string]any{"iid": 3, "title": "Relay", "description": "", "url": "https://gitlab.example.com/acme/relay/-/merge_requests/3",
			"state": "merged", "source_branch": "hzn-1-relay"}})
	if err := f.svc.Receive(ctx, gl.ID, glHeader("Merge Request Hook", gl.WebhookSecret), mr); err != nil {
		t.Fatal(err)
	}
	links := f.links("HZN-1")
	if got := kinds(links); got != "pull_request:merged,branch:open,commit:" {
		t.Errorf("gitlab links = %s", got)
	}
	if got := f.status("HZN-1"); got == "Done" {
		t.Error("moved with the merge setting off")
	}
	if err := f.svc.WriteComments(ctx); err != nil {
		t.Fatal(err)
	}
	if got := api.calls[len(api.calls)-1]; got != "POST /api/v4/projects/42/merge_requests/3/notes" || api.auth[len(api.auth)-1] != "glpat-secret" {
		t.Errorf("gitlab call = %s (%s)", got, api.auth[len(api.auth)-1])
	}
}

func TestConnections(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c := f.connect(vcs.GitHub, "https://github.com", "")
	if c.HasToken || c.WebhookSecret == "" || c.WebhookURL != "https://horizon.example.com/api/v1/hooks/vcs/"+c.ID.String() {
		t.Errorf("created = %+v", c)
	}
	token := "ghp_x"
	updated, err := f.svc.Update(ctx, f.wc(), c.ID, c.Version, vcs.Input{Token: &token, RotateSecret: true})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.HasToken || updated.WebhookSecret == "" || updated.WebhookSecret == c.WebhookSecret {
		t.Errorf("updated = %+v", updated)
	}
	bad := "ftp://x"
	if _, err := f.svc.Update(ctx, f.wc(), c.ID, 0, vcs.Input{BaseURL: &bad}); !apperr.Is(err, "invalid_input") {
		t.Errorf("bad url: %v", err)
	}
	list, err := f.svc.List(ctx, f.wc())
	if err != nil || len(list) != 1 || list[0].WebhookSecret != "" {
		t.Errorf("list = %+v, %v", list, err)
	}
	if err := f.svc.Delete(ctx, f.wc(), c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Get(ctx, f.wc(), c.ID); !apperr.Is(err, "connection_not_found") {
		t.Errorf("deleted: %v", err)
	}
}
