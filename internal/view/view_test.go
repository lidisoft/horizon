package view_test

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/query"
	"github.com/lidisoft/horizon/internal/testenv"
	"github.com/lidisoft/horizon/internal/view"
	"github.com/lidisoft/horizon/internal/workspace"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres)) }

const password = "correct horse battery"

type fixture struct {
	t        *testing.T
	pool     *pgxpool.Pool
	ids      *identity.Service
	ws       *workspace.Service
	projects *project.Service
	svc      *view.Service
	acme     workspace.Workspace
	owner    identity.Principal
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, pool: testenv.DB(t)}
	base := &url.URL{Scheme: "https", Host: "horizon.example.com"}
	f.ids = identity.New(identity.Options{Pool: f.pool, Log: zerolog.Nop(), BaseURL: base, SessionSecret: "test-session-secret-at-least-32-chars"})
	acc := access.New(f.pool, nil)
	f.ws = workspace.New(workspace.Options{Pool: f.pool, Log: zerolog.Nop(), Identity: f.ids, Access: acc, Mail: &platformmail.Memory{}, BaseURL: base})
	f.projects = project.New(f.pool, acc, nil)
	f.svc = view.New(f.pool, acc, f.projects)
	f.owner = f.principal("owner@example.com")
	var err error
	if f.acme, err = f.ws.Create(context.Background(), uuid.NullUUID{}, workspace.NewWorkspace{Slug: "acme", Name: "Acme", OwnerID: f.owner.User.ID}); err != nil {
		t.Fatal(err)
	}
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

func (f *fixture) project(key, visibility string) project.Project {
	f.t.Helper()
	name := key
	p, err := f.projects.Create(context.Background(), f.wc(f.owner), project.Input{Key: &key, Name: &name, Visibility: &visibility})
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

func ptr[T any](v T) *T { return &v }

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if !apperr.Is(err, code) {
		t.Fatalf("err = %v, want %s", err, code)
	}
}

func names(vs []view.View) []string {
	var out []string
	for _, v := range vs {
		out = append(out, v.Name)
	}
	slices.Sort(out)
	return out
}

func TestViews(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.project("HZN", project.VisibilityWorkspace)
	f.project("SEC", project.VisibilityPrivate)
	owc := f.wc(f.owner)
	member := f.principal("member@example.com")
	if err := f.ws.AddMember(ctx, f.acme.ID, member.User.ID); err != nil {
		t.Fatal(err)
	}
	mwc := f.wc(member)

	open := query.Query{Filter: &query.Node{Field: "status.outcome", Op: "eq", Value: json.RawMessage(`"open"`)}}
	bugs, err := f.svc.Create(ctx, owc, view.Input{Project: "HZN", Name: ptr("  Open bugs "), Layout: ptr("board"), Query: &open,
		Display: json.RawMessage(`{"wip":{"In progress":3}}`), Shared: ptr(true)})
	if err != nil {
		t.Fatal(err)
	}
	if bugs.Name != "Open bugs" || !bugs.Shared || !bugs.CanEdit || bugs.Query.Filter == nil || string(bugs.Display) != `{"wip": {"In progress": 3}}` {
		t.Errorf("created = %+v %s", bugs, bugs.Display)
	}
	if _, err := f.svc.Create(ctx, owc, view.Input{Project: "SEC", Name: ptr("Secret"), Layout: ptr("list"), Shared: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	mine, err := f.svc.Create(ctx, owc, view.Input{Name: ptr("Mine"), Layout: ptr("table")})
	if err != nil {
		t.Fatal(err)
	}
	if mine.Shared || string(mine.Display) != "{}" {
		t.Errorf("personal = %+v", mine)
	}

	// Members see shared views of what they see, not others' own ones.
	got, err := f.svc.List(ctx, mwc, "")
	if err != nil || !slices.Equal(names(got), []string{"Open bugs"}) || got[0].CanEdit {
		t.Errorf("member list = %v, %v", names(got), err)
	}
	if got, _ := f.svc.List(ctx, owc, "HZN"); !slices.Equal(names(got), []string{"Mine", "Open bugs"}) {
		t.Errorf("owner HZN list = %v", names(got))
	}
	_, err = f.svc.Get(ctx, mwc, mine.ID)
	wantCode(t, err, "view_not_found")

	// Sharing and editing a shared view need view.share; a personal one is the member's own.
	_, err = f.svc.Create(ctx, mwc, view.Input{Name: ptr("Team"), Layout: ptr("list"), Shared: ptr(true)})
	wantCode(t, err, "forbidden")
	_, err = f.svc.Update(ctx, mwc, bugs.ID, 0, view.Input{Name: ptr("Hijacked")})
	wantCode(t, err, "forbidden")
	own, err := f.svc.Create(ctx, mwc, view.Input{Project: "HZN", Name: ptr("Own"), Layout: ptr("list")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Update(ctx, mwc, own.ID, 0, view.Input{Shared: ptr(true)})
	wantCode(t, err, "forbidden")

	// Validation: the query must compile; layout and display are checked.
	bad := query.Query{Filter: &query.Node{Field: "nope", Op: "eq", Value: json.RawMessage(`1`)}}
	_, err = f.svc.Create(ctx, owc, view.Input{Name: ptr("Bad"), Layout: ptr("list"), Query: &bad})
	wantField(t, err, "query.filter.field", "unknown_field")
	_, err = f.svc.Create(ctx, owc, view.Input{Name: ptr("Bad"), Layout: ptr("gallery")})
	wantField(t, err, "layout", "invalid")
	_, err = f.svc.Create(ctx, owc, view.Input{Name: ptr("Bad"), Layout: ptr("list"), Display: json.RawMessage(`[1]`)})
	wantField(t, err, "display", "invalid")
	_, err = f.svc.Create(ctx, owc, view.Input{Name: ptr(" "), Layout: ptr("list")})
	wantField(t, err, "name", "invalid")

	// Updates check the version and can unshare.
	_, err = f.svc.Update(ctx, owc, bugs.ID, bugs.Version+1, view.Input{Name: ptr("Stale")})
	wantCode(t, err, "version_conflict")
	upd, err := f.svc.Update(ctx, owc, bugs.ID, bugs.Version, view.Input{Display: json.RawMessage(`{"wip":{"In progress":4}}`), Shared: ptr(false)})
	if err != nil || upd.Shared || upd.Version != bugs.Version+1 || upd.Name != "Open bugs" {
		t.Fatalf("updated = %+v, %v", upd, err)
	}
	if got, _ := f.svc.List(ctx, mwc, ""); !slices.Equal(names(got), []string{"Own"}) {
		t.Errorf("member list after unsharing = %v", names(got))
	}

	if err := f.svc.Delete(ctx, owc, bugs.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Get(ctx, owc, bugs.ID)
	wantCode(t, err, "view_not_found")
	events, err := eventlog.History(ctx, f.pool, "view", bugs.ID)
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, e := range events {
		types = append(types, e.Type)
	}
	if !slices.Equal(types, []string{"ViewDeleted"}) {
		t.Errorf("history = %v", types)
	}
	// A personal view's events name its owner, so only they are told.
	if mine := mustHistory(t, f, mine.ID); !strings.Contains(string(mine[0].Payload), `"owner_id": "`+f.owner.User.ID.String()) {
		t.Errorf("personal payload = %s", mine[0].Payload)
	}
}

func mustHistory(t *testing.T, f *fixture, id uuid.UUID) []eventlog.Event {
	t.Helper()
	events, err := eventlog.History(context.Background(), f.pool, "view", id)
	if err != nil || len(events) == 0 {
		t.Fatalf("history = %v, %v", events, err)
	}
	return events
}

func wantField(t *testing.T, err error, field, code string) {
	t.Helper()
	if e, ok := apperr.As(err); ok {
		for _, x := range e.Fields {
			if x.Field == field && x.Code == code {
				return
			}
		}
	}
	t.Fatalf("err = %v, want %s %s", err, field, code)
}
