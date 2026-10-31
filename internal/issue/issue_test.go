package issue_test

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/testenv"
	"github.com/lidisoft/horizon/internal/workspace"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres)) }

const password = "correct horse battery"

type fixture struct {
	t        *testing.T
	pool     *pgxpool.Pool
	ids      *identity.Service
	acc      *access.Service
	ws       *workspace.Service
	cfg      *configdef.Service
	projects *project.Service
	svc      *issue.Service
	owner    identity.Principal
	acme     workspace.Workspace
	hzn      project.Project
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, pool: testenv.DB(t)}
	base := &url.URL{Scheme: "https", Host: "horizon.example.com"}
	f.ids = identity.New(identity.Options{Pool: f.pool, Log: zerolog.Nop(), BaseURL: base, SessionSecret: "test-session-secret-at-least-32-chars"})
	f.acc = access.New(f.pool, nil)
	f.cfg = configdef.New(f.pool, f.acc, nil, nil)
	f.ws = workspace.New(workspace.Options{Pool: f.pool, Log: zerolog.Nop(), Identity: f.ids, Access: f.acc, Mail: &platformmail.Memory{}, BaseURL: base,
		OnCreate: []workspace.CreateHook{f.cfg.SeedDefaults}})
	f.projects = project.New(f.pool, f.acc, nil)
	f.svc = issue.New(f.pool, f.acc, f.cfg, f.projects, nil)
	f.owner = f.principal("owner@example.com")
	var err error
	if f.acme, err = f.ws.Create(context.Background(), uuid.NullUUID{}, workspace.NewWorkspace{Slug: "acme", Name: "Acme", OwnerID: f.owner.User.ID}); err != nil {
		t.Fatal(err)
	}
	f.hzn = f.project("HZN", "Horizon", "workspace")
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

func (f *fixture) project(key, name, visibility string) project.Project {
	f.t.Helper()
	p, err := f.projects.Create(context.Background(), f.wc(f.owner), project.Input{Key: &key, Name: &name, Visibility: &visibility})
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *fixture) pc(p identity.Principal, key string) project.Context {
	f.t.Helper()
	pc, err := f.projects.Enter(context.Background(), f.wc(p), key)
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

// defaultWorkflow is the seeded workflow: Backlog, Todo, In progress, Done,
// Canceled.
func (f *fixture) defaultWorkflow() configdef.Workflow {
	f.t.Helper()
	for _, w := range f.resolved().Workflows {
		if w.Name == "Default" {
			return w
		}
	}
	f.t.Fatal("no Default workflow")
	return configdef.Workflow{}
}

func (f *fixture) status(name string) uuid.UUID {
	f.t.Helper()
	for _, st := range f.defaultWorkflow().Statuses {
		if st.Name == name {
			return st.ID
		}
	}
	f.t.Fatalf("no status %s", name)
	return uuid.Nil
}

func (f *fixture) create(p identity.Principal, typ, title string) issue.Issue {
	f.t.Helper()
	i, err := f.svc.Create(context.Background(), f.pc(p, "HZN"), issue.NewIssue{TypeID: f.typeID(typ), Title: title})
	if err != nil {
		f.t.Fatal(err)
	}
	return i
}

func (f *fixture) enter(p identity.Principal, key string) issue.Context {
	f.t.Helper()
	ic, err := f.svc.Enter(context.Background(), f.wc(p), key)
	if err != nil {
		f.t.Fatal(err)
	}
	return ic
}

func (f *fixture) ws_() configdef.Target {
	return configdef.Target{Principal: f.owner, WorkspaceID: f.acme.ID}
}

func (f *fixture) history(id uuid.UUID) []string {
	f.t.Helper()
	events, err := eventlog.History(context.Background(), f.pool, "issue", id)
	if err != nil {
		f.t.Fatal(err)
	}
	var out []string
	for _, e := range events {
		out = append(out, e.PublicType())
	}
	return out
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
	a := f.create(f.owner, "Task", "  Write the resolver  ")
	b := f.create(f.owner, "Bug", "Fix the sidebar")
	if a.Key != "HZN-1" || b.Key != "HZN-2" || a.Title != "Write the resolver" {
		t.Errorf("keys %s %s, title %q", a.Key, b.Key, a.Title)
	}
	if a.StatusID != f.status("Backlog") || a.Priority != "none" || a.StartedAt != nil || !a.ReporterID.Valid {
		t.Errorf("created = %+v", a)
	}
	if !(a.Rank < b.Rank) {
		t.Errorf("ranks %s, %s", a.Rank, b.Rank)
	}
	if h := f.history(a.ID); len(h) != 1 || h[0] != "issue.created" {
		t.Errorf("history = %v", h)
	}

	pc := f.pc(f.owner, "HZN")
	_, err := f.svc.Create(ctx, pc, issue.NewIssue{TypeID: f.typeID("Task"), Title: " ", Priority: "asap", DueDate: "2026-11-01", StartDate: "2026-11-02"})
	wantField(t, err, "title", "required")
	wantField(t, err, "priority", "invalid")
	wantField(t, err, "due_date", "before_start")
	_, err = f.svc.Create(ctx, pc, issue.NewIssue{TypeID: uuid.New(), Title: "x"})
	wantField(t, err, "type_id", "unknown")
	_, err = f.svc.Create(ctx, pc, issue.NewIssue{TypeID: f.typeID("Task"), Title: "x", Estimate: s("4")})
	wantField(t, err, "estimate", "not_in_scale")
	_, err = f.svc.Create(ctx, pc, issue.NewIssue{TypeID: f.typeID("Task"), Title: "x", AssigneeID: uuid.NullUUID{UUID: uuid.New(), Valid: true}})
	wantField(t, err, "assignee_id", "not_member")

	// Parent rules: Sub-task needs one of Story, Task or Bug; Epic has none.
	epic := f.create(f.owner, "Epic", "Phase 3")
	_, err = f.svc.Create(ctx, pc, issue.NewIssue{TypeID: f.typeID("Sub-task"), Title: "x"})
	wantField(t, err, "parent", "required")
	_, err = f.svc.Create(ctx, pc, issue.NewIssue{TypeID: f.typeID("Sub-task"), Title: "x", ParentKey: epic.Key})
	wantField(t, err, "parent", "type_not_allowed")
	_, err = f.svc.Create(ctx, pc, issue.NewIssue{TypeID: f.typeID("Epic"), Title: "x", ParentKey: a.Key})
	wantField(t, err, "parent", "not_allowed")
	sub, err := f.svc.Create(ctx, pc, issue.NewIssue{TypeID: f.typeID("Sub-task"), Title: "Tests", ParentKey: "hzn-1", Estimate: s("3"), Priority: "high"})
	if err != nil {
		t.Fatal(err)
	}
	if sub.ParentID.UUID != a.ID || *sub.Estimate != "3" || sub.Priority != "high" {
		t.Errorf("sub = %+v", sub)
	}
	children, err := f.svc.Children(ctx, f.enter(f.owner, a.Key))
	if err != nil || len(children) != 1 || children[0].Key != sub.Key {
		t.Errorf("children = %+v, %v", children, err)
	}

	// Archived projects take no new issues.
	old := f.project("OLD", "Old", "workspace")
	if _, err := f.projects.SetArchived(ctx, f.pc(f.owner, old.Key), true); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Create(ctx, f.pc(f.owner, "OLD"), issue.NewIssue{TypeID: f.typeID("Task"), Title: "x"})
	wantCode(t, err, "project_archived")
}

func TestVisibility(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.project("SEC", "Secret", "private")
	secret, err := f.svc.Create(ctx, f.pc(f.owner, "SEC"), issue.NewIssue{TypeID: f.typeID("Task"), Title: "Hidden"})
	if err != nil {
		t.Fatal(err)
	}
	open := f.create(f.owner, "Task", "Open")
	ann := f.member("ann@example.com")

	if _, err := f.svc.Enter(ctx, f.wc(ann), secret.Key); !apperr.Is(err, "issue_not_found") {
		t.Errorf("private issue: %v", err)
	}
	for _, key := range []string{"HZN-99", "HZN", "nope-1", "HZN-0"} {
		if _, err := f.svc.Enter(ctx, f.wc(ann), key); !apperr.Is(err, "issue_not_found") {
			t.Errorf("%s: %v", key, err)
		}
	}
	if _, err := f.svc.Enter(ctx, f.wc(ann), "hzn-1"); err != nil {
		t.Errorf("lower-case key: %v", err)
	}

	// A link to a hidden issue isn't listed, and hidden issues can't be targets.
	if _, err := f.svc.AddLink(ctx, f.enter(f.owner, open.Key), issue.LinkInput{Type: "relates_to", TargetKey: secret.Key}); err != nil {
		t.Fatal(err)
	}
	links, _ := f.svc.Links(ctx, f.enter(ann, open.Key))
	if len(links) != 0 {
		t.Errorf("ann sees %+v", links)
	}
	_, err = f.svc.AddLink(ctx, f.enter(ann, open.Key), issue.LinkInput{Type: "blocks", TargetKey: secret.Key})
	wantField(t, err, "target", "unknown")

	// A renamed project's former key still resolves.
	key := "HRZ"
	if _, err := f.projects.Update(ctx, f.pc(f.owner, "HZN"), 0, project.Input{Key: &key}); err != nil {
		t.Fatal(err)
	}
	ic := f.enter(ann, "HZN-1")
	if got, _ := f.svc.Get(ctx, ic); got.Key != "HRZ-1" {
		t.Errorf("former key gives %s", got.Key)
	}
}

func TestUpdate(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.create(f.owner, "Task", "A")
	b := f.create(f.owner, "Story", "B")
	label, err := f.cfg.CreateLabel(ctx, f.ws_(), configdef.LabelInput{Name: s("backend"), Colour: s("blue")})
	if err != nil {
		t.Fatal(err)
	}

	got, err := f.svc.Update(ctx, f.enter(f.owner, a.Key), a.Version, issue.Update{
		Title: s("A, renamed"), Description: s("Some *text*"), Labels: &[]uuid.UUID{label.ID, label.ID},
		DueDate: s("2026-11-13"), ParentKey: s(b.Key),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "A, renamed" || got.Description != "Some *text*" || len(got.Labels) != 1 || *got.DueDate != "2026-11-13" ||
		got.ParentID.UUID != b.ID || got.Version != a.Version+1 {
		t.Errorf("updated = %+v", got)
	}
	_, err = f.svc.Update(ctx, f.enter(f.owner, a.Key), a.Version, issue.Update{Title: s("stale")})
	wantCode(t, err, "version_conflict")

	// Nothing changed: no new version, no event.
	same, err := f.svc.Update(ctx, f.enter(f.owner, a.Key), got.Version, issue.Update{Title: s("A, renamed")})
	if err != nil || same.Version != got.Version {
		t.Errorf("no-op = %d, %v", same.Version, err)
	}

	// Cycles are refused; "" clears the parent.
	_, err = f.svc.Update(ctx, f.enter(f.owner, b.Key), 0, issue.Update{ParentKey: s(a.Key)})
	wantField(t, err, "parent", "cycle")
	_, err = f.svc.Update(ctx, f.enter(f.owner, a.Key), 0, issue.Update{ParentKey: s(a.Key)})
	wantField(t, err, "parent", "cycle")
	got, err = f.svc.Update(ctx, f.enter(f.owner, a.Key), 0, issue.Update{ParentKey: s("")})
	if err != nil || got.ParentID.Valid {
		t.Errorf("cleared parent = %+v, %v", got.ParentID, err)
	}

	// Changing type keeps the status when both share a workflow.
	got, err = f.svc.Update(ctx, f.enter(f.owner, a.Key), 0, issue.Update{TypeID: new(f.typeID("Bug"))})
	if err != nil || got.TypeID != f.typeID("Bug") || got.StatusID != a.StatusID {
		t.Errorf("type change = %+v, %v", got, err)
	}

	want := []string{"issue.created", "issue.described", "issue.parent_changed", "issue.updated", "issue.parent_changed", "issue.type_changed"}
	if h := f.history(a.ID); strings.Join(h, " ") != strings.Join(want, " ") {
		t.Errorf("history = %v", h)
	}
}

func TestCustomFields(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	severity, err := f.cfg.CreateField(ctx, f.ws_(), configdef.FieldInput{Name: s("Severity"), Type: s("select"),
		Options: &[]configdef.Option{{Name: "S1", Colour: "coral"}, {Name: "S2", Colour: "orange"}}})
	if err != nil {
		t.Fatal(err)
	}
	link, err := f.cfg.CreateField(ctx, f.ws_(), configdef.FieldInput{Name: s("Spec"), Type: s("url")})
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.cfg.CreateField(ctx, f.ws_(), configdef.FieldInput{Name: s("Other"), Type: s("text")})
	if err != nil {
		t.Fatal(err)
	}
	bug := f.typeID("Bug")
	if err := f.cfg.SetTypeFields(ctx, f.ws_(), bug, []configdef.TypeField{{FieldID: severity.ID, Required: true}, {FieldID: link.ID}}); err != nil {
		t.Fatal(err)
	}
	s1 := severity.Options[0].ID.String()
	pc := f.pc(f.owner, "HZN")
	raw := func(v string) json.RawMessage { return json.RawMessage(v) }

	_, err = f.svc.Create(ctx, pc, issue.NewIssue{TypeID: bug, Title: "x"})
	wantField(t, err, "fields."+severity.ID.String(), "required")
	_, err = f.svc.Create(ctx, pc, issue.NewIssue{TypeID: bug, Title: "x", Fields: map[string]json.RawMessage{
		severity.ID.String(): raw(`"nope"`), link.ID.String(): raw(`"ftp://x"`), other.ID.String(): raw(`"x"`)}})
	wantField(t, err, "fields."+severity.ID.String(), "invalid")
	wantField(t, err, "fields."+link.ID.String(), "invalid")
	wantField(t, err, "fields."+other.ID.String(), "not_on_type")

	i, err := f.svc.Create(ctx, pc, issue.NewIssue{TypeID: bug, Title: "x", Fields: map[string]json.RawMessage{
		severity.ID.String(): raw(`"` + s1 + `"`), link.ID.String(): raw(`"https://example.com/spec"`)}})
	if err != nil {
		t.Fatal(err)
	}
	i, err = f.svc.Update(ctx, f.enter(f.owner, i.Key), 0, issue.Update{Fields: map[string]json.RawMessage{link.ID.String(): raw(`null`)}})
	if err != nil || strings.Contains(string(i.Fields), link.ID.String()) || !strings.Contains(string(i.Fields), s1) {
		t.Errorf("fields = %s, %v", i.Fields, err)
	}

	// Deleting a field scrubs its values.
	if err := f.cfg.DeleteField(ctx, f.ws_(), severity.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.svc.Get(ctx, f.enter(f.owner, i.Key)); string(got.Fields) != "{}" {
		t.Errorf("after delete: %s", got.Fields)
	}
}

func TestTransition(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.create(f.owner, "Task", "A")

	ic := f.enter(f.owner, a.Key)
	avail, err := f.svc.Transitions(ctx, ic)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, at := range avail {
		names = append(names, at.Name)
	}
	if strings.Join(names, ",") != "Plan,Cancel" {
		t.Errorf("from Backlog: %v", names)
	}

	_, err = f.svc.Transition(ctx, ic, 0, f.status("Done"))
	wantCode(t, err, "transition_not_allowed")
	got, err := f.svc.Transition(ctx, ic, 0, f.status("Todo"))
	if err != nil {
		t.Fatal(err)
	}
	got, err = f.svc.Transition(ctx, f.enter(f.owner, a.Key), got.Version, f.status("In progress"))
	if err != nil || got.StartedAt == nil || got.ResolvedAt != nil {
		t.Fatalf("started = %+v, %v", got, err)
	}
	got, err = f.svc.Transition(ctx, f.enter(f.owner, a.Key), 0, f.status("Done"))
	if err != nil || got.ResolvedAt == nil {
		t.Fatalf("done = %+v, %v", got, err)
	}
	got, err = f.svc.Transition(ctx, f.enter(f.owner, a.Key), 0, f.status("In progress"))
	if err != nil || got.ResolvedAt != nil || got.StartedAt == nil {
		t.Errorf("reopened = %+v, %v", got, err)
	}

	// Rules: Start is for Admins, needs an estimate, and assigns the actor;
	// Finish needs children closed.
	w := f.defaultWorkflow()
	admin := roleID(t, f, "Admin")
	d := configdef.Draft{Statuses: w.Statuses, Transitions: w.Transitions}
	for k, tr := range d.Transitions {
		switch tr.Name {
		case "Start":
			d.Transitions[k].Rules = configdef.Rules{Roles: []uuid.UUID{admin}, Validators: []string{"estimate"},
				PostActions: []configdef.PostAction{{Field: "assignee", Op: "set_actor"}}}
		case "Finish":
			d.Transitions[k].Rules = configdef.Rules{ChildrenFinal: true}
		}
	}
	saved, err := f.cfg.SaveDraft(ctx, f.ws_(), w.ID, w.DraftVersion, d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.cfg.Publish(ctx, f.ws_(), w.ID, saved.DraftVersion, nil); err != nil {
		t.Fatal(err)
	}

	ann := f.member("ann@example.com")
	b := f.create(ann, "Task", "B")
	if _, err := f.svc.Transition(ctx, f.enter(ann, b.Key), 0, f.status("Todo")); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Transition(ctx, f.enter(ann, b.Key), 0, f.status("In progress"))
	wantCode(t, err, "transition_forbidden")
	_, err = f.svc.Transition(ctx, f.enter(f.owner, b.Key), 0, f.status("In progress"))
	wantCode(t, err, "transition_requires_fields")
	wantField(t, err, "estimate", "required")
	if _, err := f.svc.Estimate(ctx, f.enter(f.owner, b.Key), 0, "5"); err != nil {
		t.Fatal(err)
	}
	got, err = f.svc.Transition(ctx, f.enter(f.owner, b.Key), 0, f.status("In progress"))
	if err != nil || got.AssigneeID.UUID != f.owner.User.ID {
		t.Fatalf("started b = %+v, %v", got, err)
	}

	sub, err := f.svc.Create(ctx, f.pc(f.owner, "HZN"), issue.NewIssue{TypeID: f.typeID("Sub-task"), Title: "child", ParentKey: b.Key})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Transition(ctx, f.enter(f.owner, b.Key), 0, f.status("Done"))
	wantCode(t, err, "children_open")
	if _, err := f.svc.Transition(ctx, f.enter(f.owner, sub.Key), 0, f.status("Canceled")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Transition(ctx, f.enter(f.owner, b.Key), 0, f.status("Done")); err != nil {
		t.Errorf("children closed: %v", err)
	}

	h := f.history(b.ID)
	if strings.Join(h, " ") != "issue.created issue.status_updated issue.estimated issue.status_updated issue.assigned issue.status_updated" {
		t.Errorf("history = %v", h)
	}
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

func TestPublishMovesIssues(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.create(f.owner, "Task", "A")
	if _, err := f.svc.Transition(ctx, f.enter(f.owner, a.Key), 0, f.status("Canceled")); err != nil {
		t.Fatal(err)
	}
	w := f.defaultWorkflow()
	canceled, done := f.status("Canceled"), f.status("Done")
	d := configdef.Draft{Statuses: w.Statuses[:4]}
	for _, tr := range w.Transitions {
		if tr.To != canceled && tr.From.UUID != canceled {
			d.Transitions = append(d.Transitions, tr)
		}
	}
	saved, err := f.cfg.SaveDraft(ctx, f.ws_(), w.ID, w.DraftVersion, d)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.cfg.Publish(ctx, f.ws_(), w.ID, saved.DraftVersion, nil)
	wantField(t, err, "status_mapping."+canceled.String(), "required:1")
	if _, err := f.cfg.Publish(ctx, f.ws_(), w.ID, saved.DraftVersion, map[uuid.UUID]uuid.UUID{canceled: done}); err != nil {
		t.Fatal(err)
	}
	got, _ := f.svc.Get(ctx, f.enter(f.owner, a.Key))
	if got.StatusID != done || got.ResolvedAt == nil {
		t.Errorf("moved = %+v", got)
	}
	if h := f.history(a.ID); h[len(h)-1] != "issue.status_updated" {
		t.Errorf("history = %v", h)
	}
}

func TestLabels(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	be, _ := f.cfg.CreateLabel(ctx, f.ws_(), configdef.LabelInput{Name: s("backend"), Colour: s("blue")})
	api, _ := f.cfg.CreateLabel(ctx, f.ws_(), configdef.LabelInput{Name: s("api"), Colour: s("teal")})
	a, err := f.svc.Create(ctx, f.pc(f.owner, "HZN"), issue.NewIssue{TypeID: f.typeID("Task"), Title: "A", Labels: []uuid.UUID{be.ID, api.ID}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := f.svc.Create(ctx, f.pc(f.owner, "HZN"), issue.NewIssue{TypeID: f.typeID("Task"), Title: "B", Labels: []uuid.UUID{api.ID}})
	_, err = f.svc.Create(ctx, f.pc(f.owner, "HZN"), issue.NewIssue{TypeID: f.typeID("Task"), Title: "C", Labels: []uuid.UUID{uuid.New()}})
	wantField(t, err, "labels", "unknown")

	if err := f.cfg.MergeLabel(ctx, f.ws_(), api.ID, be.ID); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{a.Key, b.Key} {
		got, _ := f.svc.Get(ctx, f.enter(f.owner, k))
		if len(got.Labels) != 1 || got.Labels[0] != be.ID {
			t.Errorf("%s labels = %v", k, got.Labels)
		}
	}
}

func TestRank(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.create(f.owner, "Task", "A")
	b := f.create(f.owner, "Task", "B")
	c := f.create(f.owner, "Task", "C")
	order := func() string {
		page, err := f.svc.List(ctx, f.pc(f.owner, "HZN"), "", 0, false)
		if err != nil {
			t.Fatal(err)
		}
		var keys []string
		for _, i := range page.Items {
			keys = append(keys, i.Title)
		}
		return strings.Join(keys, "")
	}
	if _, err := f.svc.Rank(ctx, f.enter(f.owner, c.Key), 0, a.Key); err != nil {
		t.Fatal(err)
	}
	if o := order(); o != "ACB" {
		t.Errorf("after C→A: %s", o)
	}
	if _, err := f.svc.Rank(ctx, f.enter(f.owner, b.Key), 0, ""); err != nil {
		t.Fatal(err)
	}
	if o := order(); o != "BAC" {
		t.Errorf("B first: %s", o)
	}
	// Already in place: no new version.
	cur, _ := f.svc.Get(ctx, f.enter(f.owner, b.Key))
	again, err := f.svc.Rank(ctx, f.enter(f.owner, b.Key), 0, "")
	if err != nil || again.Version != cur.Version {
		t.Errorf("no-op rank: %d → %d, %v", cur.Version, again.Version, err)
	}

	// Pages follow the rank.
	page, err := f.svc.List(ctx, f.pc(f.owner, "HZN"), "", 2, false)
	if err != nil || len(page.Items) != 2 || page.Next == "" {
		t.Fatalf("page 1 = %+v, %v", page, err)
	}
	page, _ = f.svc.List(ctx, f.pc(f.owner, "HZN"), page.Next, 2, false)
	if len(page.Items) != 1 || page.Items[0].Title != "C" || page.Next != "" {
		t.Errorf("page 2 = %+v", page)
	}
}

func TestArchiveAndPermissions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.create(f.owner, "Task", "A")
	ann := f.member("ann@example.com")

	got, err := f.svc.Assign(ctx, f.enter(ann, a.Key), 0, ann.User.ID)
	if err != nil || got.AssigneeID.UUID != ann.User.ID {
		t.Fatalf("assign = %+v, %v", got, err)
	}
	if _, err := f.svc.Prioritize(ctx, f.enter(ann, a.Key), 0, "urgent"); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Prioritize(ctx, f.enter(ann, a.Key), 0, "asap")
	wantField(t, err, "priority", "invalid")

	got, err = f.svc.SetArchived(ctx, f.enter(f.owner, a.Key), 0, true)
	if err != nil || got.ArchivedAt == nil {
		t.Fatalf("archive = %+v, %v", got, err)
	}
	_, err = f.svc.Update(ctx, f.enter(f.owner, a.Key), 0, issue.Update{Title: s("x")})
	wantCode(t, err, "issue_archived")
	if page, _ := f.svc.List(ctx, f.pc(f.owner, "HZN"), "", 0, false); len(page.Items) != 0 {
		t.Errorf("archived issue listed")
	}
	if page, _ := f.svc.List(ctx, f.pc(f.owner, "HZN"), "", 0, true); len(page.Items) != 1 {
		t.Errorf("archived issue missing with archived=true")
	}
	if _, err := f.svc.SetArchived(ctx, f.enter(f.owner, a.Key), 0, false); err != nil {
		t.Fatal(err)
	}

	// A Viewer sees issues but can't change them.
	_, err = f.acc.SetGrant(ctx, f.acme.ID, uuid.NullUUID{}, access.GrantInput{RoleID: roleID(t, f, "Viewer"),
		UserID: uuid.NullUUID{UUID: ann.User.ID, Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Update(ctx, f.enter(ann, a.Key), 0, issue.Update{Title: s("x")})
	wantCode(t, err, "forbidden")
	_, err = f.svc.Create(ctx, f.pc(ann, "HZN"), issue.NewIssue{TypeID: f.typeID("Task"), Title: "x"})
	wantCode(t, err, "forbidden")
}

func TestLinks(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.create(f.owner, "Task", "A")
	b := f.create(f.owner, "Task", "B")

	l, err := f.svc.AddLink(ctx, f.enter(f.owner, a.Key), issue.LinkInput{Type: "blocked_by", TargetKey: b.Key})
	if err != nil || l.Type != "blocked_by" || l.Key != b.Key {
		t.Fatalf("link = %+v, %v", l, err)
	}
	fromB, _ := f.svc.Links(ctx, f.enter(f.owner, b.Key))
	if len(fromB) != 1 || fromB[0].Type != "blocks" || fromB[0].Key != a.Key {
		t.Errorf("from B = %+v", fromB)
	}
	_, err = f.svc.AddLink(ctx, f.enter(f.owner, b.Key), issue.LinkInput{Type: "blocks", TargetKey: a.Key})
	wantCode(t, err, "link_exists")
	_, err = f.svc.AddLink(ctx, f.enter(f.owner, a.Key), issue.LinkInput{Type: "relates_to", TargetKey: a.Key})
	wantField(t, err, "target", "self")
	lag := int32(2)
	_, err = f.svc.AddLink(ctx, f.enter(f.owner, a.Key), issue.LinkInput{Type: "relates_to", TargetKey: b.Key, LagDays: &lag})
	wantField(t, err, "lag_days", "not_scheduling")
	p, err := f.svc.AddLink(ctx, f.enter(f.owner, a.Key), issue.LinkInput{Type: "follows", TargetKey: b.Key, LagDays: &lag})
	if err != nil || *p.LagDays != 2 {
		t.Fatalf("follows = %+v, %v", p, err)
	}
	if _, err := f.svc.AddLink(ctx, f.enter(f.owner, b.Key), issue.LinkInput{Type: "relates_to", TargetKey: a.Key}); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.AddLink(ctx, f.enter(f.owner, a.Key), issue.LinkInput{Type: "relates_to", TargetKey: b.Key})
	wantCode(t, err, "link_exists")

	if err := f.svc.RemoveLink(ctx, f.enter(f.owner, b.Key), l.ID); err != nil {
		t.Fatal(err)
	}
	if links, _ := f.svc.Links(ctx, f.enter(f.owner, a.Key)); len(links) != 2 {
		t.Errorf("after removal: %+v", links)
	}
	if h := f.history(b.ID); strings.Join(h, " ") != "issue.created issue.linked issue.linked issue.linked issue.unlinked" {
		t.Errorf("history of B = %v", h)
	}
}
