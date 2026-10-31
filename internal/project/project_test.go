package project_test

import (
	"context"
	"net/url"
	"os"
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
	"github.com/lidisoft/horizon/internal/testenv"
	"github.com/lidisoft/horizon/internal/workspace"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres)) }

const password = "correct horse battery"

type fixture struct {
	t     *testing.T
	pool  *pgxpool.Pool
	ids   *identity.Service
	acc   *access.Service
	ws    *workspace.Service
	svc   *project.Service
	owner identity.Principal // Admin of acme
	acme  workspace.Workspace
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, pool: testenv.DB(t)}
	base := &url.URL{Scheme: "https", Host: "horizon.example.com"}
	f.ids = identity.New(identity.Options{Pool: f.pool, Log: zerolog.Nop(), BaseURL: base, SessionSecret: "test-session-secret-at-least-32-chars"})
	f.acc = access.New(f.pool, nil)
	f.ws = workspace.New(workspace.Options{Pool: f.pool, Log: zerolog.Nop(), Identity: f.ids, Access: f.acc, Mail: &platformmail.Memory{}, BaseURL: base})
	f.svc = project.New(f.pool, f.acc, nil)
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

// member adds a Contributor to acme.
func (f *fixture) member(email string) identity.Principal {
	f.t.Helper()
	p := f.principal(email)
	if err := f.ws.AddMember(context.Background(), f.acme.ID, p.User.ID); err != nil {
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

func (f *fixture) create(key, name, visibility string) project.Project {
	f.t.Helper()
	p, err := f.svc.Create(context.Background(), f.wc(f.owner), project.Input{Key: &key, Name: &name, Visibility: &visibility})
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *fixture) enter(p identity.Principal, key string) (project.Context, error) {
	return f.svc.Enter(context.Background(), f.wc(p), key)
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if !apperr.Is(err, code) {
		t.Fatalf("err = %v, want %s", err, code)
	}
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
	t.Fatalf("err = %v, want %s: %s", err, field, code)
}

func s(v string) *string { return &v }

func TestCreate(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	wc := f.wc(f.owner)
	p := f.create("hzn", " Horizon ", "workspace")
	if p.Key != "HZN" || p.Name != "Horizon" || p.Colour != "blue" || p.LeadID.UUID != f.owner.User.ID {
		t.Errorf("project = %+v", p)
	}
	for _, key := range []string{"H", "1AB", "TOOLONGKEY1", "A-B", ""} {
		_, err := f.svc.Create(ctx, wc, project.Input{Key: s(key), Name: s("X")})
		wantField(t, err, "key", "invalid")
	}
	_, err := f.svc.Create(ctx, wc, project.Input{Key: s("Hzn"), Name: s("Again")})
	wantField(t, err, "key", "taken")
	_, err = f.svc.Create(ctx, wc, project.Input{Key: s("LDS"), Name: s("Design"), Colour: s("mauve"), Visibility: s("secret"), LeadID: &f.acme.ID})
	wantField(t, err, "colour", "invalid")
	wantField(t, err, "visibility", "invalid")
	wantField(t, err, "lead_id", "not_member")

	// Contributors can't create projects (project.create is Admin's).
	ada := f.member("ada@example.com")
	_, err = f.svc.Create(ctx, f.wc(ada), project.Input{Key: s("ADA"), Name: s("Mine")})
	wantCode(t, err, "forbidden")

	events, _ := eventlog.History(ctx, f.pool, "project", p.ID)
	if len(events) != 1 || events[0].PublicType() != "project.created" {
		t.Errorf("events = %+v", events)
	}
}

func TestKeyAliases(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	hzn := f.create("HZN", "Horizon", "workspace")
	f.create("TOC", "Tocsin", "workspace")

	pc, _ := f.enter(f.owner, "HZN")
	renamed, err := f.svc.Update(ctx, pc, pc.Project.Version, project.Input{Key: s("hz")})
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Key != "HZ" || strings.Join(renamed.Aliases, ",") != "HZN" {
		t.Fatalf("renamed = %+v", renamed)
	}
	// The old key still resolves, to the current one.
	old, err := f.enter(f.owner, "hzn")
	if err != nil || old.Project.ID != hzn.ID || old.Project.Key != "HZ" {
		t.Fatalf("by old key: %+v, %v", old.Project, err)
	}
	// Nobody else can take a former key, or a current one.
	toc, _ := f.enter(f.owner, "TOC")
	_, err = f.svc.Update(ctx, toc, 0, project.Input{Key: s("HZN")})
	wantField(t, err, "key", "taken")
	_, err = f.svc.Update(ctx, toc, 0, project.Input{Key: s("HZ")})
	wantField(t, err, "key", "taken")
	_, err = f.svc.Create(ctx, f.wc(f.owner), project.Input{Key: s("HZN"), Name: s("Clash")})
	wantField(t, err, "key", "taken")

	// The project itself can take it back; HZ becomes the alias.
	pc, _ = f.enter(f.owner, "HZ")
	back, err := f.svc.Update(ctx, pc, 0, project.Input{Key: s("HZN")})
	if err != nil || back.Key != "HZN" || strings.Join(back.Aliases, ",") != "HZ" {
		t.Fatalf("back = %+v, %v", back, err)
	}
	_, err = f.svc.Update(ctx, pc, pc.Project.Version, project.Input{Name: s("stale")})
	wantCode(t, err, "version_conflict")

	events, _ := eventlog.History(ctx, f.pool, "project", hzn.ID)
	if got := string(events[1].Payload); !strings.Contains(got, `"old_key": "HZN"`) || !strings.Contains(got, `"key": "HZ"`) {
		t.Errorf("rename event = %s", got)
	}
}

func TestVisibility(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.create("PUB", "Public", "workspace")
	secret := f.create("SEC", "Secret", "private")
	ada := f.member("ada@example.com")

	keys := func(p identity.Principal) string {
		list, err := f.svc.List(ctx, f.wc(p))
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, x := range list {
			out = append(out, x.Key)
		}
		return strings.Join(out, ",")
	}
	if got := keys(ada); got != "PUB" {
		t.Errorf("contributor sees %s", got)
	}
	_, err := f.enter(ada, "SEC")
	wantCode(t, err, "project_not_found")
	if got := keys(f.owner); got != "PUB,SEC" {
		t.Errorf("admin sees %s", got) // project.manage on the workspace
	}

	// A grant on the private project lets Ada in, as that role.
	_, err = f.acc.SetGrant(ctx, f.acme.ID, uuid.NullUUID{}, access.GrantInput{
		RoleID: roleID(t, f, "Viewer"), UserID: uuid.NullUUID{UUID: ada.User.ID, Valid: true},
		ProjectID: uuid.NullUUID{UUID: secret.ID, Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := keys(ada); got != "PUB,SEC" {
		t.Errorf("with a grant, ada sees %s", got)
	}
	pc, err := f.enter(ada, "SEC")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Update(ctx, pc, 0, project.Input{Name: s("Mine")})
	wantCode(t, err, "forbidden")

	// A guest with no grant at all sees nothing.
	guest := f.member("guest@example.com")
	grants, _ := f.acc.Grants(ctx, f.acme.ID)
	for _, g := range grants {
		if g.UserID.UUID == guest.User.ID {
			_ = f.acc.RemoveGrant(ctx, f.acme.ID, uuid.NullUUID{}, g.ID)
		}
	}
	if got := keys(guest); got != "" {
		t.Errorf("no grant, sees %s", got)
	}
}

func TestArchive(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.create("OLD", "Old", "workspace")
	pc, _ := f.enter(f.owner, "OLD")
	p, err := f.svc.SetArchived(ctx, pc, true)
	if err != nil || p.ArchivedAt == nil {
		t.Fatalf("archived = %+v, %v", p, err)
	}
	pc, _ = f.enter(f.owner, "OLD")
	if p, err = f.svc.SetArchived(ctx, pc, false); err != nil || p.ArchivedAt != nil {
		t.Fatalf("restored = %+v, %v", p, err)
	}
	ada := f.member("ada@example.com")
	ac, _ := f.enter(ada, "OLD")
	_, err = f.svc.SetArchived(ctx, ac, true)
	wantCode(t, err, "forbidden")
}

func roleID(t *testing.T, f *fixture, name string) uuid.UUID {
	t.Helper()
	roles, err := f.acc.Roles(context.Background(), f.acme.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range roles {
		if r.Name == name {
			return r.ID
		}
	}
	t.Fatalf("no role %s", name)
	return uuid.Nil
}
