package issue_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue"
)

func (f *fixture) proj(key string) configdef.Target {
	return configdef.Target{Principal: f.owner, WorkspaceID: f.acme.ID, ProjectID: uuid.NullUUID{UUID: f.pc(f.owner, key).Project.ID, Valid: true}}
}

func TestDelete(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	story := f.create(f.owner, "Story", "Story")
	task := f.create(f.owner, "Task", "Task")
	pc := f.pc(f.owner, "HZN")
	sub, err := f.svc.Create(ctx, pc, issue.NewIssue{TypeID: f.typeID("Sub-task"), Title: "Sub", ParentKey: task.Key})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Update(ctx, f.enter(f.owner, task.Key), 0, issue.Update{ParentKey: s(story.Key)}); err != nil {
		t.Fatal(err)
	}
	other := f.create(f.owner, "Bug", "Other")
	if _, err := f.svc.AddLink(ctx, f.enter(f.owner, other.Key), issue.LinkInput{Type: "blocks", TargetKey: task.Key}); err != nil {
		t.Fatal(err)
	}

	err = f.svc.Delete(ctx, f.enter(f.owner, task.Key), "")
	wantCode(t, err, "children_choice_required")
	if err := f.svc.Delete(ctx, f.enter(f.owner, task.Key), issue.ChildrenReparent); err != nil {
		t.Fatal(err)
	}
	got, _ := f.svc.Get(ctx, f.enter(f.owner, sub.Key))
	if got.ParentID.UUID != story.ID {
		t.Errorf("sub's parent = %v, want the story", got.ParentID)
	}
	if _, err := f.svc.Enter(ctx, f.wc(f.owner), task.Key); !apperr.Is(err, "issue_not_found") {
		t.Errorf("deleted issue: %v", err)
	}
	if h := f.history(task.ID); strings.Join(h, " ") != "issue.deleted" {
		t.Errorf("history of the deleted issue = %v", h)
	}
	if h := f.history(other.ID); h[len(h)-1] != "issue.unlinked" {
		t.Errorf("history of the linked issue = %v", h)
	}
	if links, _ := f.svc.Links(ctx, f.enter(f.owner, other.Key)); len(links) != 0 {
		t.Errorf("links left: %+v", links)
	}

	// A Contributor can't delete; cascade takes the subtree.
	ann := f.member("ann@example.com")
	wantCode(t, f.svc.Delete(ctx, f.enter(ann, story.Key), issue.ChildrenCascade), "forbidden")
	if err := f.svc.Delete(ctx, f.enter(f.owner, story.Key), issue.ChildrenCascade); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Enter(ctx, f.wc(f.owner), sub.Key); !apperr.Is(err, "issue_not_found") {
		t.Errorf("descendant left: %v", err)
	}
	if h := f.history(sub.ID); strings.Join(h, " ") != "issue.deleted" {
		t.Errorf("history of the descendant = %v", h)
	}
	// Numbers aren't reused.
	if next := f.create(f.owner, "Task", "Next"); next.Key != "HZN-5" {
		t.Errorf("next key = %s", next.Key)
	}
}

func TestMove(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	ops := f.project("OPS", "Operations", "workspace")

	// OPS has its own Incident type and workflow, its own "backend" label
	// and a Severity field of the same name.
	w := f.defaultWorkflow()
	wf, err := f.cfg.CreateWorkflow(ctx, f.proj("OPS"), "Ops", w.ID)
	if err != nil {
		t.Fatal(err)
	}
	incident, err := f.cfg.CreateIssueType(ctx, f.proj("OPS"), configdef.IssueTypeInput{Name: s("Incident"), Icon: s("alert-circle"),
		Colour: s("coral"), WorkflowID: &wf.ID})
	if err != nil {
		t.Fatal(err)
	}
	severity, _ := f.cfg.CreateField(ctx, f.ws_(), configdef.FieldInput{Name: s("Severity"), Type: s("select"),
		Options: &[]configdef.Option{{Name: "S1", Colour: "coral"}}})
	opsSeverity, _ := f.cfg.CreateField(ctx, f.proj("OPS"), configdef.FieldInput{Name: s("Severity"), Type: s("select"),
		Options: &[]configdef.Option{{Name: "s1", Colour: "coral"}}})
	notes, _ := f.cfg.CreateField(ctx, f.ws_(), configdef.FieldInput{Name: s("Notes"), Type: s("text")})
	bug := f.typeID("Bug")
	if err := f.cfg.SetTypeFields(ctx, f.ws_(), bug, []configdef.TypeField{{FieldID: severity.ID}, {FieldID: notes.ID}}); err != nil {
		t.Fatal(err)
	}
	if err := f.cfg.SetTypeFields(ctx, f.proj("OPS"), incident.ID, []configdef.TypeField{{FieldID: opsSeverity.ID}}); err != nil {
		t.Fatal(err)
	}
	hznLabel, _ := f.cfg.CreateLabel(ctx, f.proj("HZN"), configdef.LabelInput{Name: s("backend"), Colour: s("blue")})
	opsLabel, _ := f.cfg.CreateLabel(ctx, f.proj("OPS"), configdef.LabelInput{Name: s("Backend"), Colour: s("blue")})

	a, err := f.svc.Create(ctx, f.pc(f.owner, "HZN"), issue.NewIssue{TypeID: bug, Title: "Outage", Labels: []uuid.UUID{hznLabel.ID},
		Estimate: s("5"), Fields: map[string]json.RawMessage{
			severity.ID.String(): json.RawMessage(`"` + severity.Options[0].ID.String() + `"`), notes.ID.String(): json.RawMessage(`"n"`)}})
	if err != nil {
		t.Fatal(err)
	}
	sub, err := f.svc.Create(ctx, f.pc(f.owner, "HZN"), issue.NewIssue{TypeID: f.typeID("Sub-task"), Title: "Fix", ParentKey: a.Key})
	if err != nil {
		t.Fatal(err)
	}

	// Bug has no match in OPS (it does: workspace types are shared), so
	// map it to Incident explicitly.
	p, err := f.svc.PreviewMove(ctx, f.enter(f.owner, a.Key), "OPS", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Issues != 2 || p.Children != 1 || len(p.Types) != 2 || p.Types[0].Match == nil {
		t.Errorf("preview = %+v", p)
	}
	_, err = f.svc.Move(ctx, f.enter(f.owner, a.Key), issue.MoveInput{Project: "OPS"})
	wantField(t, err, "children", "required")
	_, err = f.svc.Move(ctx, f.enter(f.owner, a.Key), issue.MoveInput{Project: "HZN", Children: issue.ChildrenMove})
	wantField(t, err, "project", "same")

	moved, err := f.svc.Move(ctx, f.enter(f.owner, a.Key), issue.MoveInput{Project: "OPS", Children: issue.ChildrenLeave,
		Types: map[uuid.UUID]uuid.UUID{bug: incident.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if moved.Key != "OPS-1" || moved.ProjectID != ops.ID || moved.TypeID != incident.ID || moved.Rank == "" {
		t.Errorf("moved = %+v", moved)
	}
	// Same status names in the copied workflow; Severity by name with its
	// option by name; Notes isn't on Incident; the label by name.
	backlog := ""
	for _, st := range f.resolvedIn("OPS").Workflows[wf.ID].Statuses {
		if st.Name == "Backlog" {
			backlog = st.ID.String()
		}
	}
	if moved.StatusID.String() != backlog {
		t.Errorf("status = %s, want OPS Backlog %s", moved.StatusID, backlog)
	}
	want := `{"` + opsSeverity.ID.String() + `": "` + opsSeverity.Options[0].ID.String() + `"}`
	if string(moved.Fields) != want {
		t.Errorf("fields = %s, want %s", moved.Fields, want)
	}
	if len(moved.Labels) != 1 || moved.Labels[0] != opsLabel.ID || *moved.Estimate != "5" {
		t.Errorf("labels = %v, estimate = %v", moved.Labels, moved.Estimate)
	}

	// The old key resolves to the new one; the child stayed with a parent
	// in OPS.
	if got, err := f.svc.Get(ctx, f.enter(f.owner, a.Key)); err != nil || got.Key != "OPS-1" {
		t.Errorf("old key gives %s, %v", got.Key, err)
	}
	child, _ := f.svc.Get(ctx, f.enter(f.owner, sub.Key))
	if child.Key != sub.Key || child.ParentID.UUID != a.ID {
		t.Errorf("child = %+v", child)
	}
	h := f.history(a.ID)
	if h[len(h)-1] != "issue.moved" {
		t.Errorf("history = %v", h)
	}
	events, _ := eventlog.History(ctx, f.pool, "issue", a.ID)
	if p := string(events[len(events)-1].Payload); !strings.Contains(p, `"from_key": "HZN-1"`) && !strings.Contains(p, `"from_key":"HZN-1"`) {
		t.Errorf("payload = %s", p)
	}

	// Moving back with the subtree: Incident has no match in HZN.
	_, err = f.svc.Move(ctx, f.enter(f.owner, "OPS-1"), issue.MoveInput{Project: "HZN", Children: issue.ChildrenMove})
	wantCode(t, err, "mapping_required")
	wantField(t, err, "mapping.types."+incident.ID.String(), "required")
	back, err := f.svc.Move(ctx, f.enter(f.owner, "OPS-1"), issue.MoveInput{Project: "HZN", Children: issue.ChildrenMove,
		Types: map[uuid.UUID]uuid.UUID{incident.ID: bug}})
	if err != nil {
		t.Fatal(err)
	}
	child, _ = f.svc.Get(ctx, f.enter(f.owner, sub.Key))
	if back.Key != "HZN-3" || child.Key != "HZN-4" || child.ParentID.UUID != a.ID {
		t.Errorf("back = %s, child = %s", back.Key, child.Key)
	}
	for _, k := range []string{"HZN-1", "OPS-1"} {
		if got, _ := f.svc.Get(ctx, f.enter(f.owner, k)); got.Key != "HZN-3" {
			t.Errorf("%s gives %s", k, got.Key)
		}
	}
}

func (f *fixture) resolvedIn(key string) *configdef.Resolved {
	f.t.Helper()
	r, err := f.cfg.Resolve(context.Background(), f.pool, f.acme.ID, uuid.NullUUID{UUID: f.pc(f.owner, key).Project.ID, Valid: true})
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}

func TestDeleteProject(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	old := f.project("OLD", "Old", "workspace")
	a, err := f.svc.Create(ctx, f.pc(f.owner, "OLD"), issue.NewIssue{TypeID: f.typeID("Task"), Title: "A"})
	if err != nil {
		t.Fatal(err)
	}
	label, _ := f.cfg.CreateLabel(ctx, f.proj("OLD"), configdef.LabelInput{Name: s("old"), Colour: s("grey")})
	child, err := f.svc.Create(ctx, f.pc(f.owner, "HZN"), issue.NewIssue{TypeID: f.typeID("Sub-task"), Title: "Elsewhere", ParentKey: a.Key})
	if err != nil {
		t.Fatal(err)
	}

	wantCode(t, f.projects.Delete(ctx, f.pc(f.owner, "OLD"), "OLD"), "project_not_archived")
	if _, err := f.projects.SetArchived(ctx, f.pc(f.owner, "OLD"), true); err != nil {
		t.Fatal(err)
	}
	wantField(t, f.projects.Delete(ctx, f.pc(f.owner, "OLD"), "old"), "confirm", "mismatch")
	ann := f.member("ann@example.com")
	wantCode(t, f.projects.Delete(ctx, f.pc(ann, "OLD"), "OLD"), "forbidden")
	if err := f.projects.Delete(ctx, f.pc(f.owner, "OLD"), "OLD"); err != nil {
		t.Fatal(err)
	}

	if _, err := f.projects.Enter(ctx, f.wc(f.owner), "OLD"); !apperr.Is(err, "project_not_found") {
		t.Errorf("project left: %v", err)
	}
	for _, id := range []uuid.UUID{a.ID} {
		if h := f.history(id); len(h) != 0 {
			t.Errorf("issue history left: %v", h)
		}
	}
	if h, _ := eventlog.History(ctx, f.pool, "label", label.ID); len(h) != 0 {
		t.Errorf("label history left: %d", len(h))
	}
	if h, _ := eventlog.History(ctx, f.pool, "project", old.ID); len(h) != 1 || h[0].Type != "ProjectDeleted" {
		t.Errorf("project history = %+v", h)
	}
	got, _ := f.svc.Get(ctx, f.enter(f.owner, child.Key))
	if got.ParentID.Valid {
		t.Errorf("child keeps its parent %v", got.ParentID)
	}
	var purges int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM purges`).Scan(&purges)
	if purges < 3 {
		t.Errorf("%d purges queued", purges)
	}
	// The key is free again.
	f.project("OLD", "Old again", "workspace")
}
