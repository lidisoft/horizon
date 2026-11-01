package planning_test

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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/planning"
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
	ws       *workspace.Service
	cfg      *configdef.Service
	projects *project.Service
	issues   *issue.Service
	svc      *planning.Service
	owner    identity.Principal
	acme     workspace.Workspace
	hzn      project.Project
	now      time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, pool: testenv.DB(t), now: time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)} // a Wednesday
	clock := func() time.Time { return f.now }
	base := &url.URL{Scheme: "https", Host: "horizon.example.com"}
	f.ids = identity.New(identity.Options{Pool: f.pool, Log: zerolog.Nop(), BaseURL: base, SessionSecret: "test-session-secret-at-least-32-chars"})
	acc := access.New(f.pool, nil)
	f.cfg = configdef.New(f.pool, acc, nil, nil)
	f.ws = workspace.New(workspace.Options{Pool: f.pool, Log: zerolog.Nop(), Identity: f.ids, Access: acc, Mail: &platformmail.Memory{}, BaseURL: base,
		OnCreate: []workspace.CreateHook{f.cfg.SeedDefaults}})
	f.projects = project.New(f.pool, acc, nil)
	f.projects.OnDelete(f.cfg.DeleteProject)
	f.issues = issue.New(f.pool, acc, f.cfg, f.projects, nil)
	f.svc = planning.New(f.pool, acc, f.cfg, f.projects, f.issues, clock)
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

func (f *fixture) resolved(key string) *configdef.Resolved {
	f.t.Helper()
	pc := f.pc(key)
	r, err := f.cfg.Resolve(context.Background(), f.pool, f.acme.ID, uuid.NullUUID{UUID: pc.Project.ID, Valid: true})
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}

func (f *fixture) create(key, title string, estimate string) issue.Issue {
	f.t.Helper()
	var typeID uuid.UUID
	for _, it := range f.resolved(key).IssueTypes {
		if it.Name == "Task" {
			typeID = it.ID
		}
	}
	in := issue.NewIssue{TypeID: typeID, Title: title}
	if estimate != "" {
		in.Estimate = &estimate
	}
	i, err := f.issues.Create(context.Background(), f.pc(key), in)
	if err != nil {
		f.t.Fatal(err)
	}
	return i
}

func (f *fixture) issue(key string) issue.Context {
	f.t.Helper()
	ic, err := f.issues.Enter(context.Background(), f.wc(f.owner), key)
	if err != nil {
		f.t.Fatal(err)
	}
	return ic
}

func (f *fixture) plan(name, start, end string) planning.Iteration {
	f.t.Helper()
	it, err := f.svc.Plan(context.Background(), f.pc("HZN"), planning.PlanInput{Name: name, StartDate: start, EndDate: end})
	if err != nil {
		f.t.Fatal(err)
	}
	return it
}

func (f *fixture) load(it planning.Iteration, add []string, remove []string) planning.Iteration {
	f.t.Helper()
	out, err := f.svc.Load(context.Background(), f.pc("HZN"), it.ID, planning.LoadInput{Add: add, Remove: remove})
	if err != nil {
		f.t.Fatal(err)
	}
	return out
}

// history is an aggregate's events: type and payload.
func (f *fixture) history(aggregate string, id uuid.UUID) []eventlog.Event {
	f.t.Helper()
	events, err := eventlog.History(context.Background(), f.pool, aggregate, id)
	if err != nil {
		f.t.Fatal(err)
	}
	return events
}

func types(events []eventlog.Event) []string {
	var out []string
	for _, e := range events {
		out = append(out, e.Type)
	}
	return out
}

func payload(e eventlog.Event) map[string]any {
	var m map[string]any
	_ = json.Unmarshal(e.Payload, &m)
	return m
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

func TestPlanAndSchedule(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pc := f.pc("HZN")

	list, err := f.svc.Iterations(ctx, pc, false)
	if err != nil {
		t.Fatal(err)
	}
	// Wednesday 4 November, two weeks: 4 → Tuesday 17 (a weekday already).
	if list.Next != (planning.NextIteration{Name: "Sprint 1", StartDate: "2026-11-04", EndDate: "2026-11-17"}) || !list.CanManage {
		t.Errorf("next = %+v, can = %v", list.Next, list.CanManage)
	}
	// In a project calendar with the 4th off and no Tuesdays, it starts on
	// the next working day, and still ends on one.
	target := configdef.Target{Principal: f.owner, WorkspaceID: f.acme.ID, ProjectID: uuid.NullUUID{UUID: f.hzn.ID, Valid: true}}
	if _, err := f.cfg.SetCalendar(ctx, target, configdef.CalendarInput{
		"working_days": json.RawMessage(`[1,3,4,5]`), "holidays": json.RawMessage(`[{"date":"2026-11-04","name":"Off"}]`), "holidays_mode": json.RawMessage(`"replace"`),
	}); err != nil {
		t.Fatal(err)
	}
	if l, _ := f.svc.Iterations(ctx, pc, false); l.Next != (planning.NextIteration{Name: "Sprint 1", StartDate: "2026-11-05", EndDate: "2026-11-18"}) {
		t.Errorf("next in the project calendar = %+v", l.Next)
	}
	if _, err := f.cfg.SetCalendar(ctx, target, configdef.CalendarInput{"working_days": json.RawMessage(`null`), "holidays": json.RawMessage(`null`)}); err != nil {
		t.Fatal(err)
	}
	if list.Settings.CompletionPolicy.Value != configdef.CompletionRollover || list.Settings.MaxActive.Value != 1 {
		t.Errorf("settings = %+v", list.Settings)
	}

	_, err = f.svc.Plan(ctx, pc, planning.PlanInput{Name: " ", StartDate: "2026-11-10"})
	wantField(t, err, "name", "required")
	wantField(t, err, "end_date", "required")
	_, err = f.svc.Plan(ctx, pc, planning.PlanInput{Name: "x", StartDate: "2026-11-10", EndDate: "2026-11-09"})
	wantField(t, err, "end_date", "before_start")

	later := f.plan("Later", "", "")
	if later.State != planning.StatePlanned || later.StartDate != nil {
		t.Errorf("planned = %+v", later)
	}
	s1 := f.plan("Sprint 1", "2026-11-09", "2026-11-20")
	if s1.State != planning.StateScheduled || *s1.StartDate != "2026-11-09" {
		t.Errorf("scheduled = %+v", s1)
	}
	if got := types(f.history("iteration", s1.ID)); !slices.Equal(got, []string{"IterationPlanned", "IterationScheduled"}) {
		t.Errorf("history = %v", got)
	}

	// The next one starts the working day after the last ends: Saturday 21
	// moves to Monday 23.
	list, _ = f.svc.Iterations(ctx, pc, false)
	if list.Next != (planning.NextIteration{Name: "Sprint 3", StartDate: "2026-11-23", EndDate: "2026-12-04"}) {
		t.Errorf("next = %+v", list.Next)
	}
	if len(list.Items) != 2 || list.Items[0].ID != s1.ID {
		t.Errorf("order = %+v", list.Items)
	}

	// Scheduling the planned one, renaming it, unscheduling it again.
	later, err = f.svc.Update(ctx, pc, later.ID, later.Version, planning.UpdateInput{Name: new("Sprint 2"), StartDate: new("2026-11-23"), EndDate: new("2026-12-04")})
	if err != nil {
		t.Fatal(err)
	}
	if later.State != planning.StateScheduled || later.Name != "Sprint 2" {
		t.Errorf("updated = %+v", later)
	}
	_, err = f.svc.Update(ctx, pc, later.ID, 1, planning.UpdateInput{Goal: new("x")})
	wantCode(t, err, "version_conflict")
	later, err = f.svc.Update(ctx, pc, later.ID, 0, planning.UpdateInput{StartDate: new(""), EndDate: new("")})
	if err != nil {
		t.Fatal(err)
	}
	if later.State != planning.StatePlanned {
		t.Errorf("unscheduled = %+v", later)
	}
	if got := types(f.history("iteration", later.ID)); !slices.Equal(got, []string{"IterationPlanned", "IterationUpdated", "IterationScheduled", "IterationScheduled"}) {
		t.Errorf("history = %v", got)
	}

	// Contributors don't plan.
	member := f.principal("member@example.com")
	if err := f.ws.AddMember(ctx, f.acme.ID, member.User.ID); err != nil {
		t.Fatal(err)
	}
	mpc, err := f.projects.Enter(ctx, f.wc(member), "HZN")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Plan(ctx, mpc, planning.PlanInput{Name: "Mine"})
	wantCode(t, err, "forbidden")
	if list, err := f.svc.Iterations(ctx, mpc, false); err != nil || list.CanManage || len(list.Items) != 2 {
		t.Errorf("member list = %+v, %v", list, err)
	}
}

func TestLoad(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pc := f.pc("HZN")
	a, b, c := f.create("HZN", "A", "5"), f.create("HZN", "B", "3"), f.create("HZN", "C", "")
	s1 := f.plan("Sprint 1", "2026-11-09", "2026-11-20")
	s2 := f.plan("Sprint 2", "", "")

	s1 = f.load(s1, []string{a.Key, b.Key, c.Key, "hzn-1"}, nil)
	if s1.Stats.Issues != 3 || s1.Stats.Points != 8 || s1.Stats.Estimated != 2 || s1.Stats.ByOutcome["unstarted"].Points != 8 {
		t.Errorf("stats = %+v", s1.Stats)
	}
	if got := f.issue(a.Key).Issue.IterationID; got.UUID != s1.ID {
		t.Errorf("iteration of A = %v", got)
	}
	if got := types(f.history("issue", a.ID)); got[len(got)-1] != "IssueIterationChanged" {
		t.Errorf("issue history = %v", got)
	}

	// B moves to Sprint 2: both iterations record it.
	s2 = f.load(s2, []string{b.Key}, nil)
	h1 := f.history("iteration", s1.ID)
	last := h1[len(h1)-1]
	if last.Type != "IterationLoaded" || payload(last)["reason"] != "moved" || payload(last)["to"] != s2.ID.String() {
		t.Errorf("Sprint 1 recorded %s %v", last.Type, payload(last))
	}
	h2 := f.history("iteration", s2.ID)
	if p := payload(h2[len(h2)-1]); len(p["added"].([]any)) != 1 || p["added"].([]any)[0] != b.ID.String() {
		t.Errorf("Sprint 2 recorded %v", p)
	}

	// Removing what isn't in it does nothing; removing C does.
	s1 = f.load(s1, nil, []string{b.Key, c.Key})
	if s1.Stats.Issues != 1 || f.issue(c.Key).Issue.IterationID.Valid {
		t.Errorf("after removal: %+v", s1.Stats)
	}

	// Issues of other projects, unknown keys, archived issues.
	f.project("OPS")
	ops := f.create("OPS", "Elsewhere", "")
	_, err := f.svc.Load(ctx, pc, s1.ID, planning.LoadInput{Add: []string{ops.Key, "HZN-99"}})
	wantField(t, err, "add", "unknown")
	if _, err := f.issues.SetArchived(ctx, f.issue(c.Key), 0, true); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Load(ctx, pc, s1.ID, planning.LoadInput{Add: []string{c.Key}})
	wantCode(t, err, "issue_archived")

	// Deleting an issue and moving one out of the project leave their
	// iteration, recorded with the reason.
	if err := f.issues.Delete(ctx, f.issue(a.Key), ""); err != nil {
		t.Fatal(err)
	}
	h1 = f.history("iteration", s1.ID)
	if p := payload(h1[len(h1)-1]); p["reason"] != issue.LeftDeleted || p["removed"].([]any)[0] != a.ID.String() {
		t.Errorf("after delete: %v", p)
	}
	if _, err := f.issues.Move(ctx, f.issue(b.Key), issue.MoveInput{Project: "OPS"}); err != nil {
		t.Fatal(err)
	}
	h2 = f.history("iteration", s2.ID)
	if p := payload(h2[len(h2)-1]); p["reason"] != issue.LeftMoved {
		t.Errorf("after move: %v", p)
	}
	if moved := f.issue("OPS-2").Issue; moved.IterationID.Valid {
		t.Errorf("moved issue kept its iteration")
	}
}

func TestStartCompleteArchive(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pc := f.pc("HZN")
	a := f.create("HZN", "A", "")
	s1 := f.plan("Sprint 1", "2026-11-09", "2026-11-20")
	s2 := f.plan("Sprint 2", "2026-11-23", "2026-12-04")
	s3 := f.plan("Sprint 3", "", "")
	s1 = f.load(s1, []string{a.Key}, nil)

	_, err := f.svc.Start(ctx, pc, s3.ID, 0)
	wantCode(t, err, "iteration_not_scheduled")
	s1, err = f.svc.Start(ctx, pc, s1.ID, s1.Version)
	if err != nil {
		t.Fatal(err)
	}
	if s1.State != planning.StateActive || s1.StartedAt == nil {
		t.Errorf("started = %+v", s1)
	}
	h := f.history("iteration", s1.ID)
	if p := payload(h[len(h)-1]); h[len(h)-1].Type != "IterationStarted" || p["issue_ids"].([]any)[0] != a.ID.String() {
		t.Errorf("started event %s %v", h[len(h)-1].Type, p)
	}

	// One active at a time by default; the project may allow two.
	_, err = f.svc.Start(ctx, pc, s2.ID, 0)
	wantCode(t, err, "too_many_active")
	target := configdef.Target{Principal: f.owner, WorkspaceID: f.acme.ID, ProjectID: uuid.NullUUID{UUID: f.hzn.ID, Valid: true}}
	settings, err := f.cfg.SetIterationSettings(ctx, target, configdef.IterationSettingsInput{"max_active_iterations": json.RawMessage(`2`)})
	if err != nil {
		t.Fatal(err)
	}
	if !settings.MaxActive.Overridden || settings.MaxActive.Value != 2 || settings.MaxActive.Workspace != 1 {
		t.Errorf("settings = %+v", settings.MaxActive)
	}
	_, err = f.cfg.SetIterationSettings(ctx, target, configdef.IterationSettingsInput{"max_active_iterations": json.RawMessage(`0`), "colour": json.RawMessage(`1`)})
	wantField(t, err, "max_active_iterations", "invalid")
	wantField(t, err, "colour", "unknown")
	if _, err := f.svc.Start(ctx, pc, s2.ID, 0); err != nil {
		t.Fatal(err)
	}

	// An active iteration keeps its start date; its end may move.
	_, err = f.svc.Update(ctx, pc, s1.ID, 0, planning.UpdateInput{StartDate: new("2026-11-10")})
	wantField(t, err, "start_date", "started")
	if s1, err = f.svc.Update(ctx, pc, s1.ID, 0, planning.UpdateInput{EndDate: new("2026-11-27")}); err != nil || *s1.EndDate != "2026-11-27" {
		t.Fatalf("end moved: %+v, %v", s1, err)
	}

	// Archiving needs completion; deleting an active iteration is refused.
	_, err = f.svc.Archive(ctx, pc, s1.ID, 0)
	wantCode(t, err, "iteration_not_completed")
	wantCode(t, f.svc.Delete(ctx, pc, s1.ID), "iteration_active")

	// Completing records the policy in effect and the rollover target.
	_, err = f.svc.Complete(ctx, pc, s1.ID, 0, uuid.NullUUID{UUID: s1.ID, Valid: true})
	wantField(t, err, "rollover_to", "invalid")
	s1, err = f.svc.Complete(ctx, pc, s1.ID, 0, uuid.NullUUID{UUID: s3.ID, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if s1.State != planning.StateCompleted || s1.Completion == nil || s1.Completion.Policy != configdef.CompletionRollover || s1.Completion.RolloverTo.UUID != s3.ID {
		t.Errorf("completed = %+v", s1)
	}
	_, err = f.svc.Load(ctx, pc, s1.ID, planning.LoadInput{Add: []string{a.Key}})
	wantCode(t, err, "iteration_closed")
	_, err = f.svc.Update(ctx, pc, s1.ID, 0, planning.UpdateInput{Name: new("x")})
	wantCode(t, err, "iteration_closed")
	if s1, err = f.svc.Archive(ctx, pc, s1.ID, 0); err != nil || s1.State != planning.StateArchived {
		t.Fatalf("archived: %+v, %v", s1, err)
	}
	if list, _ := f.svc.Iterations(ctx, pc, false); slices.ContainsFunc(list.Items, func(it planning.Iteration) bool { return it.ID == s1.ID }) {
		t.Errorf("archived iteration listed")
	}
	if list, _ := f.svc.Iterations(ctx, pc, true); len(list.Items) != 3 {
		t.Errorf("with archived: %d", len(list.Items))
	}
}

func TestDelete(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pc := f.pc("HZN")
	a, b := f.create("HZN", "A", ""), f.create("HZN", "B", "")
	s1 := f.plan("Sprint 1", "", "")
	f.load(s1, []string{a.Key, b.Key}, nil)
	if _, err := f.issues.SetArchived(ctx, f.issue(b.Key), 0, true); err != nil {
		t.Fatal(err)
	}

	if err := f.svc.Delete(ctx, pc, s1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Iteration(ctx, pc, s1.ID); !apperr.Is(err, "iteration_not_found") {
		t.Errorf("get after delete: %v", err)
	}
	for _, key := range []string{a.Key, b.Key} {
		if f.issue(key).Issue.IterationID.Valid {
			t.Errorf("%s kept the deleted iteration", key)
		}
	}
	if got := types(f.history("iteration", s1.ID)); !slices.Equal(got, []string{"IterationDeleted"}) {
		t.Errorf("history = %v", got)
	}

	// A deleted project takes its iterations along.
	s2 := f.plan("Sprint 2", "", "")
	if _, err := f.projects.SetArchived(ctx, pc, true); err != nil {
		t.Fatal(err)
	}
	if err := f.projects.Delete(ctx, f.pc("HZN"), "HZN"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM iterations`).Scan(&n); err != nil || n != 0 {
		t.Errorf("iterations left: %d, %v", n, err)
	}
	if h := f.history("iteration", s2.ID); len(h) != 0 {
		t.Errorf("history left: %v", types(h))
	}
}

func TestWorkflowClearsIteration(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.create("HZN", "A", "")
	s1 := f.plan("Sprint 1", "", "")
	f.load(s1, []string{a.Key}, nil)

	// A post-action on the way to Canceled clears the iteration.
	r := f.resolved("HZN")
	var wf configdef.Workflow
	for _, w := range r.Workflows {
		wf = w
	}
	var canceled uuid.UUID
	for _, st := range wf.Statuses {
		if st.Name == "Canceled" {
			canceled = st.ID
		}
	}
	draft := configdef.Draft{Statuses: wf.Statuses, Transitions: wf.Transitions}
	for i, tr := range draft.Transitions {
		if tr.To == canceled {
			draft.Transitions[i].Rules.PostActions = []configdef.PostAction{{Op: "clear", Field: "iteration"}}
		}
	}
	ws := configdef.Target{Principal: f.owner, WorkspaceID: f.acme.ID}
	w, err := f.cfg.SaveDraft(ctx, ws, wf.ID, wf.DraftVersion, draft)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.cfg.Publish(ctx, ws, wf.ID, w.DraftVersion, nil); err != nil {
		t.Fatal(err)
	}
	moved, err := f.issues.Transition(ctx, f.issue(a.Key), 0, canceled)
	if err != nil {
		t.Fatal(err)
	}
	if moved.IterationID.Valid {
		t.Errorf("iteration kept")
	}
	h := f.history("iteration", s1.ID)
	if p := payload(h[len(h)-1]); p["reason"] != issue.LeftCleared {
		t.Errorf("recorded %v", p)
	}
	ih := types(f.history("issue", a.ID))
	if !slices.Contains(ih, "IssueIterationChanged") {
		t.Errorf("issue history = %v", ih)
	}
}

func issueMove(project string) issue.MoveInput { return issue.MoveInput{Project: project} }
