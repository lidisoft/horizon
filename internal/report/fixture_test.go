package report_test

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/planning"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/report"
	"github.com/lidisoft/horizon/internal/testenv"
	"github.com/lidisoft/horizon/internal/workspace"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres)) }

const password = "correct horse battery"

type fixture struct {
	t        *testing.T
	pool     *pgxpool.Pool
	ids      *identity.Service
	ws       *workspace.Service
	cfg      *configdef.Service
	projects *project.Service
	issues   *issue.Service
	plans    *planning.Service
	proj     *report.Projector
	acc      *access.Service
	owner    identity.Principal
	acme     workspace.Workspace
	hzn      project.Project
	now      time.Time
}

// newFixture starts the clock on Monday 2 November 2026, 09:00 UTC.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, pool: testenv.DB(t), now: time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)}
	clock := func() time.Time { return f.now }
	base := &url.URL{Scheme: "https", Host: "horizon.example.com"}
	f.ids = identity.New(identity.Options{Pool: f.pool, Log: zerolog.Nop(), BaseURL: base, SessionSecret: "test-session-secret-at-least-32-chars"})
	acc := access.New(f.pool, nil)
	f.acc = acc
	f.cfg = configdef.New(f.pool, acc, nil, nil)
	f.ws = workspace.New(workspace.Options{Pool: f.pool, Log: zerolog.Nop(), Identity: f.ids, Access: acc, Mail: &platformmail.Memory{}, BaseURL: base,
		OnCreate: []workspace.CreateHook{f.cfg.SeedDefaults}})
	f.projects = project.New(f.pool, acc, nil)
	f.projects.OnDelete(f.cfg.DeleteProject)
	f.issues = issue.New(f.pool, acc, f.cfg, f.projects, clock)
	f.plans = planning.New(f.pool, acc, f.cfg, f.projects, f.issues, clock)
	f.proj = report.NewProjector(f.pool, zerolog.Nop())
	f.owner = f.principal("owner@example.com")
	var err error
	if f.acme, err = f.ws.Create(context.Background(), uuid.NullUUID{}, workspace.NewWorkspace{Slug: "acme", Name: "Acme", OwnerID: f.owner.User.ID}); err != nil {
		t.Fatal(err)
	}
	f.hzn = f.project("HZN")
	return f
}

func (f *fixture) principal(email string) identity.Principal {
	f.t.Helper()
	ctx := context.Background()
	if _, err := f.ids.CreateUser(ctx, uuid.NullUUID{}, identity.NewUser{Email: email, Name: strings.Split(email, "@")[0], Password: password}); err != nil {
		f.t.Fatal(err)
	}
	in, err := f.ids.Login(ctx, email, password, identity.Client{})
	if err != nil {
		f.t.Fatal(err)
	}
	p, err := f.ids.Authenticate(ctx, in.Token)
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

func (f *fixture) project(key string) project.Project {
	f.t.Helper()
	name, vis := key, project.VisibilityWorkspace
	p, err := f.projects.Create(context.Background(), f.wc(f.owner), project.Input{Key: &key, Name: &name, Visibility: &vis})
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *fixture) pc(key string) project.Context {
	f.t.Helper()
	pc, err := f.projects.Enter(context.Background(), f.wc(f.owner), key)
	if err != nil {
		f.t.Fatal(err)
	}
	return pc
}

func (f *fixture) resolved() *configdef.Resolved {
	f.t.Helper()
	r, err := f.cfg.Resolve(context.Background(), f.pool, f.acme.ID, uuid.NullUUID{UUID: f.hzn.ID, Valid: true})
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}

func (f *fixture) status(name string) uuid.UUID {
	f.t.Helper()
	for _, w := range f.resolved().Workflows {
		for _, st := range w.Statuses {
			if st.Name == name {
				return st.ID
			}
		}
	}
	f.t.Fatalf("no status %s", name)
	return uuid.Nil
}

func (f *fixture) category(name string) configdef.Category {
	f.t.Helper()
	for _, c := range f.resolved().Categories {
		if c.Name == name {
			return c
		}
	}
	f.t.Fatalf("no category %s", name)
	return configdef.Category{}
}

func (f *fixture) typeID(name string) uuid.UUID {
	f.t.Helper()
	for _, it := range f.resolved().IssueTypes {
		if it.Name == name {
			return it.ID
		}
	}
	f.t.Fatalf("no type %s", name)
	return uuid.Nil
}

// create makes an issue in project key, with an estimate when one is given.
func (f *fixture) create(key, title, estimate string) issue.Issue {
	f.t.Helper()
	in := issue.NewIssue{TypeID: f.typeID("Task"), Title: title}
	if estimate != "" {
		in.Estimate = &estimate
	}
	i, err := f.issues.Create(context.Background(), f.pc(key), in)
	if err != nil {
		f.t.Fatal(err)
	}
	return i
}

func (f *fixture) enter(key string) issue.Context {
	f.t.Helper()
	ic, err := f.issues.Enter(context.Background(), f.wc(f.owner), key)
	if err != nil {
		f.t.Fatal(err)
	}
	return ic
}

// move transitions an issue through the statuses named, a step each.
func (f *fixture) move(key string, statuses ...string) {
	f.t.Helper()
	for _, s := range statuses {
		if _, err := f.issues.Transition(context.Background(), f.enter(key), 0, f.status(s)); err != nil {
			f.t.Fatalf("%s → %s: %v", key, s, err)
		}
	}
}

// at sets the clock.
func (f *fixture) at(s string) {
	t, err := time.Parse("2006-01-02 15:04", s)
	if err != nil {
		f.t.Fatal(err)
	}
	f.now = t
}

func (f *fixture) catchUp() int {
	f.t.Helper()
	n, err := f.proj.CatchUp(context.Background())
	if err != nil {
		f.t.Fatal(err)
	}
	return n
}

// reports is the report service on the fixture's clock.
func (f *fixture) reports() *report.Service {
	return report.New(f.pool, f.acc, f.cfg, f.projects, f.issues, f.plans, func() time.Time { return f.now })
}
