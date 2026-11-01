package search_test

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/search"
	"github.com/lidisoft/horizon/internal/testenv"
	"github.com/lidisoft/horizon/internal/workspace"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres)) }

const password = "correct horse battery"

type fixture struct {
	t        *testing.T
	ids      *identity.Service
	ws       *workspace.Service
	cfg      *configdef.Service
	projects *project.Service
	issues   *issue.Service
	svc      *search.Service
	owner    identity.Principal
	task     uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	pool := testenv.DB(t)
	base := &url.URL{Scheme: "https", Host: "horizon.example.com"}
	f := &fixture{t: t}
	f.ids = identity.New(identity.Options{Pool: pool, Log: zerolog.Nop(), BaseURL: base, SessionSecret: "test-session-secret-at-least-32-chars"})
	acc := access.New(pool, nil)
	f.cfg = configdef.New(pool, acc, nil, nil)
	f.ws = workspace.New(workspace.Options{Pool: pool, Log: zerolog.Nop(), Identity: f.ids, Access: acc, Mail: &platformmail.Memory{}, BaseURL: base,
		OnCreate: []workspace.CreateHook{f.cfg.SeedDefaults}})
	f.projects = project.New(pool, acc, nil)
	f.issues = issue.New(pool, acc, f.cfg, f.projects, nil)
	f.svc = search.New(pool, acc, f.projects, f.issues)
	f.owner = f.user("owner@example.com")
	ws, err := f.ws.Create(context.Background(), uuid.NullUUID{}, workspace.NewWorkspace{Slug: "acme", Name: "Acme", OwnerID: f.owner.User.ID})
	if err != nil {
		t.Fatal(err)
	}
	types, _ := f.cfg.IssueTypes(context.Background(), configdef.Target{Principal: f.owner, WorkspaceID: ws.ID})
	for _, it := range types {
		if it.Name == "Task" {
			f.task = it.ID
		}
	}
	return f
}

func (f *fixture) user(email string) identity.Principal {
	f.t.Helper()
	ctx := context.Background()
	if _, err := f.ids.CreateUser(ctx, uuid.NullUUID{}, identity.NewUser{Email: email, Name: strings.Split(email, "@")[0], Password: password}); err != nil {
		f.t.Fatal(err)
	}
	in, _ := f.ids.Login(ctx, email, password, identity.Client{})
	p, err := f.ids.Authenticate(ctx, in.Token)
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *fixture) wc(p identity.Principal) workspace.Context {
	wc, err := f.ws.Enter(context.Background(), p, "acme")
	if err != nil {
		f.t.Fatal(err)
	}
	return wc
}

func (f *fixture) project(key, visibility string) {
	if _, err := f.projects.Create(context.Background(), f.wc(f.owner), project.Input{Key: &key, Name: &key, Visibility: &visibility}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) create(key, title, description string) issue.Issue {
	f.t.Helper()
	ctx := context.Background()
	pc, _ := f.projects.Enter(ctx, f.wc(f.owner), key)
	i, err := f.issues.Create(ctx, pc, issue.NewIssue{TypeID: f.task, Title: title, Description: issue.PlainText(description)})
	if err != nil {
		f.t.Fatal(err)
	}
	return i
}

func (f *fixture) find(p identity.Principal, q string, o search.Options) string {
	f.t.Helper()
	r, err := f.svc.Search(context.Background(), f.wc(p), q, o)
	if err != nil {
		f.t.Fatal(err)
	}
	var out []string
	for _, i := range r.Items {
		out = append(out, i.Key+":"+i.Match)
	}
	return strings.Join(out, " ")
}

func TestSearch(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.project("HZN", "workspace")
	f.project("SEC", "private")
	relay := f.create("HZN", "Outbox relay publishes domain events", "The relay reads unpublished rows with FOR UPDATE SKIP LOCKED.")
	f.create("HZN", "Workflow transition rules", "Validators and post-actions run when the relay is idle.")
	ws := f.create("HZN", "WebSocket replay after reconnect", "")
	secret := f.create("SEC", "Secret relay plans", "")

	// Words, in titles first, then descriptions; each word a prefix.
	// Title matches (in either order) come before description matches.
	if got := f.find(f.owner, "relay", search.Options{}); got != "HZN-1:title SEC-1:title HZN-2:description" &&
		got != "SEC-1:title HZN-1:title HZN-2:description" {
		t.Errorf("relay = %s", got)
	}
	if got := f.find(f.owner, "outb rel", search.Options{}); got != "HZN-1:title" {
		t.Errorf("prefixes = %s", got)
	}
	// A typo still finds the title.
	if got := f.find(f.owner, "Websoket replay", search.Options{}); !strings.HasPrefix(got, "HZN-3:") {
		t.Errorf("typo = %s", got)
	}
	// Snippets mark the words.
	r, _ := f.svc.Search(ctx, f.wc(f.owner), "skip locked", search.Options{})
	if len(r.Items) != 1 || r.Items[0].Match != "description" || !strings.Contains(r.Items[0].Snippet, search.MarkStart+"SKIP"+search.MarkStop) {
		t.Errorf("snippet = %+v", r.Items)
	}

	// Comments.
	ic, _ := f.issues.Enter(ctx, f.wc(f.owner), ws.Key)
	if _, err := f.issues.AddComment(ctx, ic, issue.PlainText("Seen on Firefox with a flaky proxy")); err != nil {
		t.Fatal(err)
	}
	if got := f.find(f.owner, "flaky proxy", search.Options{}); got != "HZN-3:comment" {
		t.Errorf("comment = %s", got)
	}

	// Keys come first, former keys too.
	if got := f.find(f.owner, "hzn-1", search.Options{}); !strings.HasPrefix(got, "HZN-1:key") {
		t.Errorf("key = %s", got)
	}
	key := "HRZ"
	pc, _ := f.projects.Enter(ctx, f.wc(f.owner), "HZN")
	if _, err := f.projects.Update(ctx, pc, 0, project.Input{Key: &key}); err != nil {
		t.Fatal(err)
	}
	if got := f.find(f.owner, "HZN-1", search.Options{}); !strings.HasPrefix(got, "HRZ-1:key") {
		t.Errorf("former key = %s", got)
	}

	// Private projects stay hidden; one project only; archived on request.
	ann := f.user("ann@example.com")
	if err := f.ws.AddMember(ctx, f.wc(f.owner).Workspace.ID, ann.User.ID); err != nil {
		t.Fatal(err)
	}
	if got := f.find(ann, "relay", search.Options{}); strings.Contains(got, "SEC") || got == "" {
		t.Errorf("ann sees %s", got)
	}
	if got := f.find(ann, secret.Key, search.Options{}); got != "" {
		t.Errorf("ann finds the private key: %s", got)
	}
	if got := f.find(f.owner, "relay", search.Options{Project: "SEC"}); got != "SEC-1:title" {
		t.Errorf("SEC only = %s", got)
	}
	ic, _ = f.issues.Enter(ctx, f.wc(f.owner), "HRZ-1")
	if _, err := f.issues.SetArchived(ctx, ic, 0, true); err != nil {
		t.Fatal(err)
	}
	if got := f.find(f.owner, "outbox", search.Options{}); got != "" {
		t.Errorf("archived listed: %s", got)
	}
	if got := f.find(f.owner, "outbox", search.Options{Archived: true}); got != "HRZ-1:title" {
		t.Errorf("archived = %s", got)
	}
	// Nothing to search for.
	if got := f.find(f.owner, " %%& ", search.Options{}); got != "" {
		t.Errorf("symbols = %s", got)
	}
	_ = relay
}
