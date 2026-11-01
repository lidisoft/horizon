package realtime_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/realtime"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/testenv"
	"github.com/lidisoft/horizon/internal/workspace"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres|testenv.NATS)) }

const password = "correct horse battery"

type env struct {
	t        *testing.T
	ids      *identity.Service
	ws       *workspace.Service
	projects *project.Service
	issues   *issue.Service
	cfg      *configdef.Service
	relay    *eventlog.Relay
	srv      *httptest.Server
	owner    identity.Principal
}

func setup(t *testing.T) *env {
	ctx := context.Background()
	pool := testenv.DB(t)
	bus := testenv.JetStream(t)
	if _, err := bus.Stream.Ensure(ctx, bus.JetStream); err != nil {
		t.Fatal(err)
	}
	base := &url.URL{Scheme: "https", Host: "horizon.example.com"}
	e := &env{t: t}
	e.ids = identity.New(identity.Options{Pool: pool, Log: zerolog.Nop(), BaseURL: base, SessionSecret: "test-session-secret-at-least-32-chars"})
	acc := access.New(pool, nil)
	e.cfg = configdef.New(pool, acc, nil, nil)
	e.ws = workspace.New(workspace.Options{Pool: pool, Log: zerolog.Nop(), Identity: e.ids, Access: acc, Mail: &platformmail.Memory{}, BaseURL: base,
		OnCreate: []workspace.CreateHook{e.cfg.SeedDefaults}})
	e.projects = project.New(pool, acc, nil)
	e.issues = issue.New(pool, acc, e.cfg, e.projects, nil)
	e.relay = &eventlog.Relay{Pool: pool, JetStream: bus.JetStream, Stream: bus.Stream, Log: zerolog.Nop(), BatchSize: 100}
	hub := realtime.New(realtime.Options{Log: zerolog.Nop(), Pool: pool, NATS: bus.Conn, Stream: bus.Stream, Identity: e.ids, Access: acc,
		Workspaces: e.ws, Projects: e.projects, Issues: e.issues})
	runCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	go func() { _ = hub.Run(runCtx) }()

	auth := identity.NewHTTP(e.ids, false)
	e.srv = httptest.NewServer(server.NewEngine(server.Deps{Log: zerolog.Nop(), API: func(api *gin.RouterGroup) {
		api.Use(auth.Authenticate())
		auth.Register(api)
		hub.Register(api)
	}}))
	t.Cleanup(e.srv.Close)

	e.owner = e.user("owner@example.com")
	if _, err := e.ws.Create(ctx, uuid.NullUUID{}, workspace.NewWorkspace{Slug: "acme", Name: "Acme", OwnerID: e.owner.User.ID}); err != nil {
		t.Fatal(err)
	}
	e.project("HZN", "workspace")
	e.project("SEC", "private")
	// Let the hub's subscription settle before events flow.
	time.Sleep(100 * time.Millisecond)
	return e
}

func (e *env) user(email string) identity.Principal {
	e.t.Helper()
	ctx := context.Background()
	if _, err := e.ids.CreateUser(ctx, uuid.NullUUID{}, identity.NewUser{Email: email, Name: strings.Split(email, "@")[0], Password: password}); err != nil {
		e.t.Fatal(err)
	}
	in, _ := e.ids.Login(ctx, email, password, identity.Client{})
	p, err := e.ids.Authenticate(ctx, in.Token)
	if err != nil {
		e.t.Fatal(err)
	}
	return p
}

func (e *env) wc(p identity.Principal) workspace.Context {
	wc, err := e.ws.Enter(context.Background(), p, "acme")
	if err != nil {
		e.t.Fatal(err)
	}
	return wc
}

func (e *env) project(key, visibility string) {
	name := key
	if _, err := e.projects.Create(context.Background(), e.wc(e.owner), project.Input{Key: &key, Name: &name, Visibility: &visibility}); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) create(key, title string) issue.Issue {
	e.t.Helper()
	ctx := context.Background()
	pc, err := e.projects.Enter(ctx, e.wc(e.owner), key)
	if err != nil {
		e.t.Fatal(err)
	}
	var task uuid.UUID
	types, _ := e.cfg.IssueTypes(ctx, configdef.Target{Principal: e.owner, WorkspaceID: pc.Workspace.ID})
	for _, it := range types {
		if it.Name == "Task" {
			task = it.ID
		}
	}
	i, err := e.issues.Create(ctx, pc, issue.NewIssue{TypeID: task, Title: title})
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.relay.Drain(ctx); err != nil {
		e.t.Fatal(err)
	}
	return i
}

// socket is a signed-in WebSocket client.
type socket struct {
	t  *testing.T
	ws *websocket.Conn
}

func (e *env) dial(email string) *socket {
	e.t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	res, err := client.Post(e.srv.URL+"/api/v1/auth/login", "application/json", strings.NewReader(`{"email":"`+email+`","password":"`+password+`"}`))
	if err != nil || res.StatusCode != http.StatusOK {
		e.t.Fatalf("login: %v %v", res, err)
	}
	res.Body.Close()
	ws, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(e.srv.URL, "http")+"/api/v1/ws", &websocket.DialOptions{HTTPClient: client})
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { _ = ws.CloseNow() })
	return &socket{t: e.t, ws: ws}
}

func (s *socket) send(v any) {
	s.t.Helper()
	b, _ := json.Marshal(v)
	if err := s.ws.Write(context.Background(), websocket.MessageText, b); err != nil {
		s.t.Fatal(err)
	}
}

type frame struct {
	T  string          `json:"t"`
	ID string          `json:"id"`
	D  json.RawMessage `json:"d"`
}

// next reads the next frame, or returns T "" after wait.
func (s *socket) next(wait time.Duration) frame {
	s.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	_, b, err := s.ws.Read(ctx)
	if err != nil {
		return frame{}
	}
	var f frame
	_ = json.Unmarshal(b, &f)
	return f
}

func (s *socket) sub(topics []string, since string) (accepted, denied []string) {
	s.t.Helper()
	s.send(map[string]any{"t": "sub", "id": "1", "d": map[string]any{"topics": topics, "since": since}})
	f := s.next(5 * time.Second)
	var d struct{ Topics, Denied []string }
	_ = json.Unmarshal(f.D, &d)
	if f.T != "ack" {
		s.t.Fatalf("sub answered %+v", f)
	}
	return d.Topics, d.Denied
}

type pushed struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Workspace string `json:"workspace"`
	Aggregate struct {
		Type string `json:"type"`
		Key  string `json:"key"`
	} `json:"aggregate"`
	Payload map[string]any `json:"payload"`
}

// events reads event frames until none comes for a while.
func (s *socket) events() []pushed {
	var out []pushed
	for {
		f := s.next(700 * time.Millisecond)
		if f.T == "" {
			return out
		}
		if f.T == "event" {
			var p pushed
			_ = json.Unmarshal(f.D, &p)
			out = append(out, p)
		}
	}
}

func keys(ps []pushed) string {
	var k []string
	for _, p := range ps {
		k = append(k, p.Type+" "+p.Aggregate.Key)
	}
	return strings.Join(k, ", ")
}

func TestPushes(t *testing.T) {
	e := setup(t)
	ann := e.user("ann@example.com")
	if err := e.ws.AddMember(context.Background(), e.wc(e.owner).Workspace.ID, ann.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.relay.Drain(context.Background()); err != nil {
		t.Fatal(err)
	}

	a := e.dial("ann@example.com")
	ok, denied := a.sub([]string{"ws:acme", "project:acme/SEC", "issue:acme/SEC-1", "ws:nope", "bogus"}, "")
	if strings.Join(ok, ",") != "ws:acme" || len(denied) != 4 {
		t.Errorf("ann: accepted %v, denied %v", ok, denied)
	}
	o := e.dial("owner@example.com")
	if ok, _ := o.sub([]string{"project:acme/SEC"}, ""); len(ok) != 1 {
		t.Fatalf("owner: %v", ok)
	}
	o.send(map[string]any{"t": "ping", "id": "p"})
	if f := o.next(5 * time.Second); f.T != "pong" || f.ID != "p" {
		t.Errorf("ping answered %+v", f)
	}

	open := e.create("HZN", "Open")
	e.create("SEC", "Secret")

	// Ann sees the open issue, not the private one; the owner, subscribed to
	// SEC only, sees the private one only.
	got := a.events()
	if keys(got) != "issue.created HZN-1" {
		t.Errorf("ann got %s", keys(got))
	}
	if got[0].Workspace != "acme" || got[0].Payload["status_id"].(map[string]any)["name"] != "Backlog" {
		t.Errorf("event = %+v", got[0])
	}
	if got := o.events(); keys(got) != "issue.created SEC-1" {
		t.Errorf("owner got %s", keys(got))
	}

	// A later socket replays from an event id.
	ctx := context.Background()
	ic, _ := e.issues.Enter(ctx, e.wc(e.owner), open.Key)
	if _, err := e.issues.Prioritize(ctx, ic, 0, "high"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.relay.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	a.events()
	late := e.dial("ann@example.com")
	late.sub([]string{"issue:acme/HZN-1"}, got[0].ID)
	if got := late.events(); keys(got) != "issue.prioritized HZN-1" {
		t.Errorf("replay: %s", keys(got))
	}
	late2 := e.dial("ann@example.com")
	late2.sub([]string{"issue:acme/HZN-1"}, oldV7().String())
	if f := late2.next(5 * time.Second); f.T != "resync" {
		t.Errorf("old since answered %+v", f)
	}
}

// oldV7 is a UUIDv7 from two days ago: its first 48 bits are the time.
func oldV7() uuid.UUID {
	u := uuid.Must(uuid.NewV7())
	ms := uint64(time.Now().Add(-48 * time.Hour).UnixMilli())
	for i := range 6 {
		u[i] = byte(ms >> (40 - 8*i))
	}
	return u
}
