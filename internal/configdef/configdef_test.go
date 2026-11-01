package configdef_test

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	configdefdb "github.com/lidisoft/horizon/internal/configdef/db"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/testenv"
	"github.com/lidisoft/horizon/internal/workspace"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres)) }

const password = "correct horse battery"

// fakeIssues pretends issues sit in some statuses and carry some labels,
// and records moves and relabels.
type fakeIssues struct {
	byStatus  map[uuid.UUID]int64
	byType    map[uuid.UUID]map[uuid.UUID]int64
	byLabel   map[uuid.UUID]int64
	moved     map[uuid.UUID]uuid.UUID
	relabeled map[uuid.UUID]uuid.UUID
}

func (f *fakeIssues) CountByLabel(context.Context, configdefdb.DBTX, uuid.UUID) (map[uuid.UUID]int64, error) {
	return f.byLabel, nil
}

func (f *fakeIssues) ScrubField(context.Context, pgx.Tx, uuid.UUID, uuid.UUID) error { return nil }

func (f *fakeIssues) Relabel(_ context.Context, _ pgx.Tx, _ uuid.NullUUID, from, into uuid.UUID) error {
	f.relabeled[from] = into
	f.byLabel[into] += f.byLabel[from]
	delete(f.byLabel, from)
	return nil
}

func (f *fakeIssues) CountByStatus(_ context.Context, _ pgx.Tx, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	out := map[uuid.UUID]int64{}
	for _, id := range ids {
		if n := f.byStatus[id]; n > 0 {
			out[id] = n
		}
	}
	return out, nil
}

func (f *fakeIssues) CountByType(_ context.Context, _ pgx.Tx, id uuid.UUID) (int64, error) {
	var n int64
	for _, c := range f.byType[id] {
		n += c
	}
	return n, nil
}

func (f *fakeIssues) MoveStatuses(_ context.Context, _ pgx.Tx, _ uuid.NullUUID, m map[uuid.UUID]uuid.UUID) error {
	for k, v := range m {
		f.moved[k] = v
	}
	return nil
}

func (f *fakeIssues) CountTypeByStatus(_ context.Context, _ pgx.Tx, id uuid.UUID) (map[uuid.UUID]int64, error) {
	return f.byType[id], nil
}

func (f *fakeIssues) MoveTypeStatuses(_ context.Context, _ pgx.Tx, _ uuid.NullUUID, _ uuid.UUID, m map[uuid.UUID]uuid.UUID) error {
	return f.MoveStatuses(context.Background(), nil, uuid.NullUUID{}, m)
}

type fixture struct {
	t       *testing.T
	db      *pgxpool.Pool
	ids     *identity.Service
	acc     *access.Service
	ws      *workspace.Service
	cfg     *configdef.Service
	issues  *fakeIssues
	owner   identity.Principal
	acme    workspace.Workspace
	project project.Project
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool := testenv.DB(t)
	base := &url.URL{Scheme: "https", Host: "horizon.example.com"}
	f := &fixture{t: t, db: pool, issues: &fakeIssues{byStatus: map[uuid.UUID]int64{}, byType: map[uuid.UUID]map[uuid.UUID]int64{},
		byLabel: map[uuid.UUID]int64{}, moved: map[uuid.UUID]uuid.UUID{}, relabeled: map[uuid.UUID]uuid.UUID{}}}
	f.ids = identity.New(identity.Options{Pool: pool, Log: zerolog.Nop(), BaseURL: base, SessionSecret: "test-session-secret-at-least-32-chars"})
	f.acc = access.New(pool, nil)
	f.cfg = configdef.New(pool, f.acc, f.issues, nil)
	f.ws = workspace.New(workspace.Options{Pool: pool, Log: zerolog.Nop(), Identity: f.ids, Access: f.acc, Mail: &platformmail.Memory{}, BaseURL: base,
		OnCreate: []workspace.CreateHook{f.cfg.SeedDefaults}})
	f.owner = f.principal("owner@example.com")
	var err error
	if f.acme, err = f.ws.Create(context.Background(), uuid.NullUUID{}, workspace.NewWorkspace{Slug: "acme", Name: "Acme", OwnerID: f.owner.User.ID}); err != nil {
		t.Fatal(err)
	}
	wc, _ := f.ws.Enter(context.Background(), f.owner, "acme")
	key, pname := "HZN", "Horizon"
	if f.project, err = project.New(pool, f.acc, nil).Create(context.Background(), wc, project.Input{Key: &key, Name: &pname}); err != nil {
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
	in, _ := f.ids.Login(ctx, email, password, identity.Client{})
	p, err := f.ids.Authenticate(ctx, in.Token)
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *fixture) ws_() configdef.Target {
	return configdef.Target{Principal: f.owner, WorkspaceID: f.acme.ID}
}

func (f *fixture) proj() configdef.Target {
	return configdef.Target{Principal: f.owner, WorkspaceID: f.acme.ID, ProjectID: uuid.NullUUID{UUID: f.project.ID, Valid: true}}
}

func (f *fixture) resolve(t configdef.Target) *configdef.Resolved {
	f.t.Helper()
	r, err := f.cfg.Resolve(context.Background(), f.db, t.WorkspaceID, t.ProjectID)
	if err != nil {
		f.t.Fatal(err)
	}
	return r
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
			if x.Field == field && (x.Code == code || strings.HasPrefix(x.Code, code+":")) {
				return
			}
		}
	}
	t.Fatalf("err = %v, want %s: %s", err, field, code)
}

func s(v string) *string { return &v }

func byName[T any](items []T, nameOf func(T) string, n string) T {
	for _, it := range items {
		if nameOf(it) == n {
			return it
		}
	}
	var zero T
	return zero
}

func cat(f *fixture, t configdef.Target, n string) configdef.Category {
	f.t.Helper()
	list, err := f.cfg.Categories(context.Background(), t)
	if err != nil {
		f.t.Fatal(err)
	}
	return byName(list, func(c configdef.Category) string { return c.Name }, n)
}

func defaultWorkflow(f *fixture) configdef.Workflow {
	f.t.Helper()
	list, err := f.cfg.Workflows(context.Background(), f.ws_())
	if err != nil {
		f.t.Fatal(err)
	}
	return byName(list, func(w configdef.Workflow) string { return w.Name }, "Default")
}

func draftOf(w configdef.Workflow) configdef.Draft {
	return configdef.Draft{Statuses: append([]configdef.Status(nil), w.Statuses...), Transitions: append([]configdef.Transition(nil), w.Transitions...)}
}

func TestDefaults(t *testing.T) {
	f := newFixture(t)
	r := f.resolve(f.ws_())
	if len(r.Categories) != 5 || len(r.Workflows) != 1 || len(r.IssueTypes) != 5 {
		t.Fatalf("resolved %d categories, %d workflows, %d types", len(r.Categories), len(r.Workflows), len(r.IssueTypes))
	}
	w := defaultWorkflow(f)
	if len(w.Statuses) != 5 || len(w.Transitions) != 8 || !w.Statuses[0].Initial || w.Statuses[0].Name != "Backlog" {
		t.Errorf("default workflow = %+v", w)
	}
	types, _ := f.cfg.IssueTypes(context.Background(), f.ws_())
	sub := byName(types, func(it configdef.IssueType) string { return it.Name }, "Sub-task")
	epic := byName(types, func(it configdef.IssueType) string { return it.Name }, "Epic")
	if sub.ParentRule != "required" || len(sub.ParentTypeIDs) != 3 || epic.ParentRule != "none" {
		t.Errorf("sub-task %+v, epic %+v", sub, epic)
	}
	// A project sees them all, inherited.
	pr := f.resolve(f.proj())
	if len(pr.IssueTypes) != 5 {
		t.Errorf("project types = %d", len(pr.IssueTypes))
	}
	pt, _ := f.cfg.IssueTypes(context.Background(), f.proj())
	if !pt[0].Inherited || pt[0].Disabled {
		t.Errorf("project view of a type = %+v", pt[0].Origin)
	}
}

func TestPermissions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	ada := f.principal("ada@example.com")
	if err := f.ws.AddMember(ctx, f.acme.ID, ada.User.ID); err != nil {
		t.Fatal(err)
	}
	_, err := f.cfg.CreateCategory(ctx, configdef.Target{Principal: ada, WorkspaceID: f.acme.ID}, configdef.CategoryInput{Name: s("QA"), Outcome: s("started")})
	wantCode(t, err, "forbidden")
	if f.cfg.CanManage(ctx, configdef.Target{Principal: ada, WorkspaceID: f.acme.ID}) || !f.cfg.CanManage(ctx, f.ws_()) {
		t.Error("CanManage disagrees with the roles")
	}
}

func TestCategories(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	qa, err := f.cfg.CreateCategory(ctx, f.ws_(), configdef.CategoryInput{Name: s("Verification"), Outcome: s("started"), Colour: s("teal")})
	if err != nil || qa.Position != 5 {
		t.Fatalf("created %+v, %v", qa, err)
	}
	_, err = f.cfg.CreateCategory(ctx, f.ws_(), configdef.CategoryInput{Name: s("verification"), Outcome: s("started")})
	wantField(t, err, "name", "taken")
	_, err = f.cfg.CreateCategory(ctx, f.ws_(), configdef.CategoryInput{Name: s("X"), Outcome: s("closed")})
	wantField(t, err, "outcome", "invalid")

	// The initial status's category stays open.
	backlog := cat(f, f.ws_(), "Backlog")
	_, err = f.cfg.UpdateCategory(ctx, f.ws_(), backlog.ID, 0, configdef.CategoryInput{Outcome: s("completed")})
	wantCode(t, err, "initial_status_in_category")

	// Deleting a category in use needs a replacement of the same kind.
	progress := cat(f, f.ws_(), "In progress")
	wantCode(t, f.cfg.DeleteCategory(ctx, f.ws_(), progress.ID, uuid.NullUUID{}), "replacement_required")
	done := cat(f, f.ws_(), "Done")
	wantField(t, f.cfg.DeleteCategory(ctx, f.ws_(), progress.ID, uuid.NullUUID{UUID: done.ID, Valid: true}), "replacement", "incompatible")
	if err := f.cfg.DeleteCategory(ctx, f.ws_(), progress.ID, uuid.NullUUID{UUID: qa.ID, Valid: true}); err != nil {
		t.Fatal(err)
	}
	w := defaultWorkflow(f)
	if w.Statuses[2].CategoryID != qa.ID {
		t.Errorf("In progress status category = %s, want Verification", w.Statuses[2].CategoryID)
	}
	if got := cat(f, f.ws_(), "Verification"); strings.Join(got.Statuses, ",") != "In progress" || got.Workflows != 1 {
		t.Errorf("usage = %+v", got)
	}
	// Unused: deleted outright.
	todo, _ := f.cfg.CreateCategory(ctx, f.ws_(), configdef.CategoryInput{Name: s("Spare"), Outcome: s("unstarted")})
	if err := f.cfg.DeleteCategory(ctx, f.ws_(), todo.ID, uuid.NullUUID{}); err != nil {
		t.Fatal(err)
	}
}

func TestDraftValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	w := defaultWorkflow(f)
	done := cat(f, f.ws_(), "Done")
	roles, _ := f.acc.Roles(ctx, f.acme.ID)

	bad := draftOf(w)
	bad.Statuses[1].Initial = true                       // two initials
	bad.Statuses[3].Name = "backlog"                     // duplicate name
	bad.Statuses[4].CategoryID = uuid.Must(uuid.NewV7()) // unknown category
	bad.Transitions = append(bad.Transitions,
		configdef.Transition{ID: uuid.Must(uuid.NewV7()), To: uuid.Must(uuid.NewV7()), Name: "Nowhere"},
		configdef.Transition{ID: uuid.Must(uuid.NewV7()), From: uuid.NullUUID{UUID: w.Statuses[0].ID, Valid: true}, To: w.Statuses[1].ID, Name: "Again"},
		configdef.Transition{ID: uuid.Must(uuid.NewV7()), To: w.Statuses[3].ID, Name: "Ruled", Rules: configdef.Rules{
			Roles: []uuid.UUID{uuid.Must(uuid.NewV7())}, Validators: []string{"mood"}, PostActions: []configdef.PostAction{{Field: "estimate", Op: "set_actor"}},
		}})
	_, err := f.cfg.SaveDraft(ctx, f.ws_(), w.ID, w.DraftVersion, bad)
	for _, want := range [][2]string{
		{"statuses", "one_initial"}, {"statuses[3].name", "taken"}, {"statuses[4].category_id", "unknown"},
		{"transitions[8].to", "unknown"}, {"transitions[9]", "duplicate"},
		{"transitions[10].rules.roles", "unknown"}, {"transitions[10].rules.validators", "unknown"}, {"transitions[10].rules.post_actions", "invalid"},
	} {
		wantField(t, err, want[0], want[1])
	}

	final := draftOf(w)
	final.Statuses[0].CategoryID = done.ID
	_, err = f.cfg.SaveDraft(ctx, f.ws_(), w.ID, w.DraftVersion, final)
	wantField(t, err, "statuses[0].initial", "final_category")

	// A valid draft with rules saves; the version moves on.
	ok := draftOf(w)
	ok.Transitions[4].Rules = configdef.Rules{Roles: []uuid.UUID{roles[1].ID}, ChildrenFinal: true, Validators: []string{"assignee"},
		PostActions: []configdef.PostAction{{Field: "assignee", Op: "set_actor"}}}
	saved, err := f.cfg.SaveDraft(ctx, f.ws_(), w.ID, w.DraftVersion, ok)
	if err != nil || saved.Draft == nil || saved.DraftVersion != w.DraftVersion+1 {
		t.Fatalf("saved = %+v, %v", saved, err)
	}
	_, err = f.cfg.SaveDraft(ctx, f.ws_(), w.ID, w.DraftVersion, ok)
	wantCode(t, err, "version_conflict")
	if err := f.cfg.DiscardDraft(ctx, f.ws_(), w.ID); err != nil {
		t.Fatal(err)
	}
	if w2, _ := f.cfg.Workflow(ctx, f.ws_(), w.ID); w2.Draft != nil {
		t.Error("draft not discarded")
	}
}

func TestPublish(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	w := defaultWorkflow(f)
	progress := cat(f, f.ws_(), "In progress")
	canceled := w.Statuses[4]

	// Add "In review", remove "Canceled" (which holds 4 issues).
	d := draftOf(w)
	review := configdef.Status{ID: uuid.Must(uuid.NewV7()), Name: "In review", CategoryID: progress.ID}
	d.Statuses = append(d.Statuses[:4], review)
	var kept []configdef.Transition
	for _, tr := range d.Transitions {
		if tr.To != canceled.ID && tr.From.UUID != canceled.ID {
			kept = append(kept, tr)
		}
	}
	kept = append(kept, configdef.Transition{ID: uuid.Must(uuid.NewV7()), From: uuid.NullUUID{UUID: w.Statuses[2].ID, Valid: true}, To: review.ID, Name: "Request review"})
	d.Transitions = kept
	saved, err := f.cfg.SaveDraft(ctx, f.ws_(), w.ID, w.DraftVersion, d)
	if err != nil {
		t.Fatal(err)
	}
	f.issues.byStatus[canceled.ID] = 4

	_, err = f.cfg.Publish(ctx, f.ws_(), w.ID, saved.DraftVersion, nil)
	wantField(t, err, "status_mapping."+canceled.ID.String(), "required")
	if e, _ := apperr.As(err); e.Fields[0].Code != "required:4" {
		t.Errorf("unmapped code = %s", e.Fields[0].Code)
	}

	res, err := f.cfg.Publish(ctx, f.ws_(), w.ID, saved.DraftVersion, map[uuid.UUID]uuid.UUID{canceled.ID: w.Statuses[3].ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Added) != 1 || len(res.Removed) != 1 || res.Moved[canceled.ID] != 4 || f.issues.moved[canceled.ID] != w.Statuses[3].ID {
		t.Errorf("result = %+v, moved %v", res, f.issues.moved)
	}
	after := defaultWorkflow(f)
	if after.Draft != nil || len(after.Statuses) != 5 || after.Statuses[4].Name != "In review" || after.Version != w.Version+1 {
		t.Errorf("published = %+v", after)
	}
	events, _ := eventlog.History(ctx, f.db, "workflow", w.ID)
	if events[len(events)-1].PublicType() != "workflow.published" {
		t.Errorf("last event = %s", events[len(events)-1].PublicType())
	}
	_, err = f.cfg.Publish(ctx, f.ws_(), w.ID, 0, nil)
	wantCode(t, err, "no_draft")
}

func TestProjectOverrides(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	types, _ := f.cfg.IssueTypes(ctx, f.proj())
	bug := byName(types, func(it configdef.IssueType) string { return it.Name }, "Bug")
	w := defaultWorkflow(f)

	// Inherited definitions are read-only from the project.
	_, err := f.cfg.UpdateIssueType(ctx, f.proj(), bug.ID, 0, configdef.IssueTypeInput{Name: s("Defect")}, nil)
	wantCode(t, err, "inherited_read_only")
	// The default workflow can't be switched off while types use it.
	wantCode(t, f.cfg.SetDisabled(ctx, f.proj(), configdef.TypeWorkflow, w.ID, true), "workflow_in_use")

	// Copying Bug switches the original off; the copy is editable.
	copyBug, err := f.cfg.CopyIssueType(ctx, f.proj(), bug.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.cfg.UpdateIssueType(ctx, f.proj(), copyBug.ID, 0, configdef.IssueTypeInput{Name: s("Defect")}, nil); err != nil {
		t.Fatal(err)
	}
	r := f.resolve(f.proj())
	var names []string
	for _, it := range r.IssueTypes {
		names = append(names, it.Name)
	}
	if got := strings.Join(names, ","); got != "Epic,Story,Task,Sub-task,Defect" {
		t.Errorf("project types = %s", got)
	}
	if got := len(f.resolve(f.ws_()).IssueTypes); got != 5 {
		t.Errorf("workspace types changed: %d", got)
	}

	// A project category and workflow; the workspace's workflows can't use
	// the project's category.
	pc, err := f.cfg.CreateCategory(ctx, f.proj(), configdef.CategoryInput{Name: s("Triage"), Outcome: s("unstarted")})
	if err != nil {
		t.Fatal(err)
	}
	pw, err := f.cfg.CreateWorkflow(ctx, f.proj(), "Bug triage", w.ID)
	if err != nil || !pw.ProjectID.Valid || len(pw.Statuses) != 5 || pw.Statuses[0].ID == w.Statuses[0].ID {
		t.Fatalf("project workflow = %+v, %v", pw, err)
	}
	d := draftOf(pw)
	d.Statuses[0].CategoryID = pc.ID
	if _, err := f.cfg.SaveDraft(ctx, f.proj(), pw.ID, pw.DraftVersion, d); err != nil {
		t.Fatal(err)
	}
	wd := draftOf(w)
	wd.Statuses[0].CategoryID = pc.ID
	_, err = f.cfg.SaveDraft(ctx, f.ws_(), w.ID, w.DraftVersion, wd)
	wantField(t, err, "statuses[0].category_id", "unknown")

	// Switching an unused inherited category off hides it from the project.
	canceledCat := cat(f, f.ws_(), "Canceled")
	wantCode(t, f.cfg.SetDisabled(ctx, f.proj(), configdef.TypeCategory, canceledCat.ID, true), "category_in_use")
	spare, _ := f.cfg.CreateCategory(ctx, f.ws_(), configdef.CategoryInput{Name: s("Spare"), Outcome: s("started")})
	if err := f.cfg.SetDisabled(ctx, f.proj(), configdef.TypeCategory, spare.ID, true); err != nil {
		t.Fatal(err)
	}
	if got := cat(f, f.proj(), "Spare"); !got.Disabled {
		t.Errorf("spare = %+v", got.Origin)
	}
	if _, ok := f.resolve(f.proj()).Categories[spare.ID]; ok {
		t.Error("disabled category still resolved")
	}
}

func TestIssueTypes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	w := defaultWorkflow(f)
	_, err := f.cfg.CreateIssueType(ctx, f.ws_(), configdef.IssueTypeInput{Name: s("Spike"), Icon: s("unicorn"), WorkflowID: &uuid.Nil, ParentRule: s("maybe")})
	wantField(t, err, "icon", "invalid")
	wantField(t, err, "workflow_id", "unknown")
	wantField(t, err, "parent_rule", "invalid")

	spike, err := f.cfg.CreateIssueType(ctx, f.ws_(), configdef.IssueTypeInput{Name: s("Spike"), Icon: s("flag"), Colour: s("yellow"), WorkflowID: &w.ID})
	if err != nil || spike.Position != 5 {
		t.Fatalf("spike = %+v, %v", spike, err)
	}

	// Moving a type with issues to another workflow needs a status mapping.
	other, err := f.cfg.CreateWorkflow(ctx, f.ws_(), "Research", w.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.issues.byType[spike.ID] = map[uuid.UUID]int64{w.Statuses[2].ID: 3}
	_, err = f.cfg.UpdateIssueType(ctx, f.ws_(), spike.ID, 0, configdef.IssueTypeInput{WorkflowID: &other.ID}, nil)
	wantCode(t, err, "unmapped_statuses")
	if _, err := f.cfg.UpdateIssueType(ctx, f.ws_(), spike.ID, 0, configdef.IssueTypeInput{WorkflowID: &other.ID},
		map[uuid.UUID]uuid.UUID{w.Statuses[2].ID: other.Statuses[2].ID}); err != nil {
		t.Fatal(err)
	}
	if f.issues.moved[w.Statuses[2].ID] != other.Statuses[2].ID {
		t.Errorf("moved = %v", f.issues.moved)
	}

	wantCode(t, f.cfg.DeleteIssueType(ctx, f.ws_(), spike.ID), "issue_type_in_use")
	delete(f.issues.byType, spike.ID)
	if err := f.cfg.DeleteIssueType(ctx, f.ws_(), spike.ID); err != nil {
		t.Fatal(err)
	}
	wantCode(t, f.cfg.DeleteWorkflow(ctx, f.ws_(), w.ID), "workflow_in_use")
	if err := f.cfg.DeleteWorkflow(ctx, f.ws_(), other.ID); err != nil {
		t.Fatal(err)
	}
}

func typeNamed(f *fixture, t configdef.Target, n string) configdef.IssueType {
	f.t.Helper()
	types, err := f.cfg.IssueTypes(context.Background(), t)
	if err != nil {
		f.t.Fatal(err)
	}
	return byName(types, func(it configdef.IssueType) string { return it.Name }, n)
}

func TestFields(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := f.cfg.CreateField(ctx, f.ws_(), configdef.FieldInput{Name: s("Mood"), Type: s("emoji")})
	wantField(t, err, "type", "invalid")
	opts := []configdef.Option{{Name: "S1", Colour: "coral"}, {Name: "S2"}, {Name: "s1"}}
	_, err = f.cfg.CreateField(ctx, f.ws_(), configdef.FieldInput{Name: s("Severity"), Type: s("select"), Options: &opts})
	wantField(t, err, "options[2].name", "taken")
	_, err = f.cfg.CreateField(ctx, f.ws_(), configdef.FieldInput{Name: s("Notes"), Type: s("text"), Options: &opts})
	wantField(t, err, "options", "not_select")

	opts = opts[:2]
	sev, err := f.cfg.CreateField(ctx, f.ws_(), configdef.FieldInput{Name: s("Severity"), Type: s("select"), Options: &opts})
	if err != nil || len(sev.Options) != 2 || sev.Options[1].Colour != "grey" || sev.Options[0].ID == uuid.Nil {
		t.Fatalf("severity = %+v, %v", sev, err)
	}
	unit, precision := "h", 1
	effort, err := f.cfg.CreateField(ctx, f.ws_(), configdef.FieldInput{Name: s("Effort"), Type: s("number"), Unit: &unit, Precision: &precision})
	if err != nil || effort.Unit != "h" || effort.Precision != 1 {
		t.Fatalf("effort = %+v, %v", effort, err)
	}
	_, err = f.cfg.UpdateField(ctx, f.ws_(), effort.ID, 0, configdef.FieldInput{Type: s("text")})
	wantField(t, err, "type", "fixed")

	// Attach to Bug, Severity required.
	bug := typeNamed(f, f.ws_(), "Bug")
	if err := f.cfg.SetTypeFields(ctx, f.ws_(), bug.ID, []configdef.TypeField{{FieldID: sev.ID, Required: true}, {FieldID: effort.ID}}); err != nil {
		t.Fatal(err)
	}
	r := f.resolve(f.ws_())
	if tf := r.TypeFields[bug.ID]; len(tf) != 2 || !tf[0].Required || tf[1].FieldID != effort.ID {
		t.Errorf("bug fields = %+v", tf)
	}
	// From the project, the inherited Bug is read-only; a copy keeps its fields.
	wantCode(t, f.cfg.SetTypeFields(ctx, f.proj(), bug.ID, nil), "inherited_read_only")
	copyBug, err := f.cfg.CopyIssueType(ctx, f.proj(), bug.ID)
	if err != nil {
		t.Fatal(err)
	}
	if tf := f.resolve(f.proj()).TypeFields[copyBug.ID]; len(tf) != 2 {
		t.Errorf("copied bug fields = %+v", tf)
	}
	// A project field can't go on a workspace type.
	env, _ := f.cfg.CreateField(ctx, f.proj(), configdef.FieldInput{Name: s("Environment"), Type: s("text")})
	err = f.cfg.SetTypeFields(ctx, f.ws_(), bug.ID, []configdef.TypeField{{FieldID: env.ID}})
	wantField(t, err, "fields[0].field_id", "unknown")

	// Rules may name custom fields; deleting the field drops it from them.
	w := defaultWorkflow(f)
	d := draftOf(w)
	d.Transitions[4].Rules = configdef.Rules{Validators: []string{sev.ID.String(), "assignee"}, PostActions: []configdef.PostAction{{Field: effort.ID.String(), Op: "clear"}}}
	saved, err := f.cfg.SaveDraft(ctx, f.ws_(), w.ID, w.DraftVersion, d)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.cfg.DeleteField(ctx, f.ws_(), sev.ID); err != nil {
		t.Fatal(err)
	}
	after, _ := f.cfg.Workflow(ctx, f.ws_(), w.ID)
	if rules := after.Draft.Transitions[4].Rules; strings.Join(rules.Validators, ",") != "assignee" || len(rules.PostActions) != 1 || after.DraftVersion != saved.DraftVersion+1 {
		t.Errorf("draft rules after delete = %+v", rules)
	}
	if tf := f.resolve(f.ws_()).TypeFields[bug.ID]; len(tf) != 1 {
		t.Errorf("bug fields after delete = %+v", tf)
	}
}

func TestLabels(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	backend, err := f.cfg.CreateLabel(ctx, f.ws_(), configdef.LabelInput{Name: s("backend"), Colour: s("blue"), Description: s("Go services")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.cfg.CreateLabel(ctx, f.ws_(), configdef.LabelInput{Name: s("Backend")})
	wantField(t, err, "name", "taken")
	customer, _ := f.cfg.CreateLabel(ctx, f.ws_(), configdef.LabelInput{Name: s("customer")})
	realtime, err := f.cfg.CreateLabel(ctx, f.proj(), configdef.LabelInput{Name: s("realtime"), Colour: s("green")})
	if err != nil {
		t.Fatal(err)
	}
	ws2, _ := f.cfg.CreateLabel(ctx, f.proj(), configdef.LabelInput{Name: s("ws")})

	// Switching an inherited label off hides it from the project's pickers.
	if err := f.cfg.SetDisabled(ctx, f.proj(), configdef.TypeLabel, customer.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.resolve(f.proj()).Labels[customer.ID]; ok {
		t.Error("switched-off label resolved")
	}
	wantCode(t, f.cfg.SetDisabled(ctx, f.proj(), configdef.TypeLabel, realtime.ID, true), "not_inherited")

	// Merge: issues follow, the label goes. A workspace label can't merge
	// into a project one.
	f.issues.byLabel[realtime.ID] = 3
	f.issues.byLabel[backend.ID] = 5
	wantField(t, f.cfg.MergeLabel(ctx, f.ws_(), backend.ID, realtime.ID), "into", "unknown")
	if err := f.cfg.MergeLabel(ctx, f.proj(), realtime.ID, backend.ID); err != nil {
		t.Fatal(err)
	}
	if f.issues.relabeled[realtime.ID] != backend.ID {
		t.Errorf("relabeled = %v", f.issues.relabeled)
	}
	labels, _ := f.cfg.Labels(ctx, f.proj())
	if got := byName(labels, func(l configdef.Label) string { return l.Name }, "backend"); got.Issues != 8 {
		t.Errorf("backend issues = %d", got.Issues)
	}
	if got := byName(labels, func(l configdef.Label) string { return l.Name }, "realtime"); got.ID != uuid.Nil {
		t.Error("merged label still listed")
	}

	// Move to workspace: every project sees it then.
	moved, err := f.cfg.MoveLabelToWorkspace(ctx, f.proj(), ws2.ID)
	if err != nil || moved.ProjectID.Valid {
		t.Fatalf("moved = %+v, %v", moved, err)
	}
	if _, ok := f.resolve(f.ws_()).Labels[ws2.ID]; !ok {
		t.Error("moved label not in the workspace")
	}
	dup, _ := f.cfg.CreateLabel(ctx, f.proj(), configdef.LabelInput{Name: s("Backend")})
	_, err = f.cfg.MoveLabelToWorkspace(ctx, f.proj(), dup.ID)
	wantField(t, err, "name", "taken")
}

func TestEstimation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	e, err := f.cfg.Estimation(ctx, f.proj())
	if err != nil || e.Kind != "fibonacci" || len(e.Values) != 8 || e.Overridden {
		t.Fatalf("default = %+v, %v", e, err)
	}
	tshirt := configdef.Scale{Kind: "tshirt", Values: []configdef.ScaleValue{{"XS", 1}, {"S", 2}, {"M", 4}}}
	if _, err := f.cfg.SetEstimation(ctx, f.ws_(), &tshirt); err != nil {
		t.Fatal(err)
	}
	if e, _ := f.cfg.Estimation(ctx, f.proj()); e.Kind != "tshirt" || e.Values[2].Weight != 4 || e.Overridden {
		t.Errorf("inherited = %+v", e)
	}
	bad := configdef.Scale{Kind: "custom", Values: []configdef.ScaleValue{{"tiny", 1}, {"TINY", 2}, {"", -1}}}
	_, err = f.cfg.SetEstimation(ctx, f.proj(), &bad)
	wantField(t, err, "values[1].label", "taken")
	wantField(t, err, "values[2].label", "invalid")
	wantField(t, err, "values[2].weight", "invalid")

	linear := configdef.Scale{Kind: "linear"}
	if e, err := f.cfg.SetEstimation(ctx, f.proj(), &linear); err != nil || e.Kind != "linear" || len(e.Values) != 9 || !e.Overridden || e.Workspace.Kind != "tshirt" {
		t.Fatalf("override = %+v, %v", e, err)
	}
	if r := f.resolve(f.proj()); r.Estimation.Kind != "linear" {
		t.Errorf("resolved = %+v", r.Estimation)
	}
	if e, err := f.cfg.SetEstimation(ctx, f.proj(), nil); err != nil || e.Kind != "tshirt" || e.Overridden {
		t.Errorf("cleared = %+v, %v", e, err)
	}
	_, err = f.cfg.SetEstimation(ctx, f.ws_(), nil)
	wantField(t, err, "scale", "required")
}

func TestCalendar(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c, err := f.cfg.CalendarSettings(ctx, f.db, f.proj())
	if err != nil || c.TimeZone.Value != "UTC" || !slices.Equal(c.WorkingDays.Value, []int{1, 2, 3, 4, 5}) || len(c.Holidays.Value) != 0 || c.Holidays.Mode != "inherit" {
		t.Fatalf("default = %+v, %v", c, err)
	}
	raw := func(s string) json.RawMessage { return json.RawMessage(s) }
	ws, err := f.cfg.SetCalendar(ctx, f.ws_(), configdef.CalendarInput{
		"working_days": raw(`[5,4,3,2,1,7]`),
		"holidays":     raw(`[{"date":"2027-01-01","name":"New Year"},{"date":"2026-12-25","name":" Christmas "}]`),
	})
	if err != nil || !slices.Equal(ws.WorkingDays.Value, []int{1, 2, 3, 4, 5, 7}) || ws.Holidays.Value[0].Name != "Christmas" {
		t.Fatalf("workspace = %+v, %v", ws, err)
	}
	_, err = f.cfg.SetCalendar(ctx, f.ws_(), configdef.CalendarInput{
		"time_zone": raw(`"Europe/Paris"`), "working_days": raw(`[0]`), "holidays": raw(`[{"date":"2026-13-01","name":"x"}]`), "holidays_mode": raw(`"add"`), "colour": raw(`1`),
	})
	wantField(t, err, "time_zone", "workspace_property")
	wantField(t, err, "working_days", "invalid")
	wantField(t, err, "holidays", "invalid")
	wantField(t, err, "holidays_mode", "invalid")
	wantField(t, err, "colour", "unknown")

	// The project inherits, then adds a date in its own time zone.
	p, err := f.cfg.SetCalendar(ctx, f.proj(), configdef.CalendarInput{
		"time_zone": raw(`"Europe/Paris"`), "holidays": raw(`[{"date":"2026-12-24","name":"Eve"}]`), "holidays_mode": raw(`"add"`),
	})
	if err != nil || p.TimeZone.Value != "Europe/Paris" || !p.TimeZone.Overridden || p.TimeZone.Workspace != "UTC" ||
		len(p.Holidays.Value) != 3 || p.Holidays.Mode != "add" || len(p.Holidays.Own) != 1 || p.WorkingDays.Overridden {
		t.Fatalf("project = %+v, %v", p, err)
	}
	cal := p.Calendar()
	if cal.Working(time.Date(2026, 12, 24, 0, 0, 0, 0, time.UTC)) || !cal.Working(time.Date(2026, 12, 27, 0, 0, 0, 0, time.UTC)) {
		t.Error("project calendar: Eve off, Sundays worked")
	}
	if p, _ = f.cfg.SetCalendar(ctx, f.proj(), configdef.CalendarInput{"holidays_mode": raw(`"replace"`)}); len(p.Holidays.Value) != 1 {
		t.Errorf("replace = %+v", p.Holidays)
	}
	// Clearing the dates drops the mode; clearing the zone follows the workspace.
	p, err = f.cfg.SetCalendar(ctx, f.proj(), configdef.CalendarInput{"holidays": raw(`null`), "time_zone": raw(`null`)})
	if err != nil || p.Holidays.Mode != "inherit" || len(p.Holidays.Value) != 2 || p.TimeZone.Overridden {
		t.Errorf("cleared = %+v, %v", p, err)
	}
}
