package planning_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/planning"
)

// complete starts and completes an iteration, then has the policy worker
// handle the completion (twice, as a redelivery would).
func (f *fixture) complete(it planning.Iteration, to uuid.NullUUID) planning.Iteration {
	f.t.Helper()
	ctx := context.Background()
	pc := f.pc("HZN")
	if it.State != planning.StateActive {
		if _, err := f.svc.Start(ctx, pc, it.ID, 0); err != nil {
			f.t.Fatal(err)
		}
	}
	if _, err := f.svc.Complete(ctx, pc, it.ID, 0, to); err != nil {
		f.t.Fatal(err)
	}
	h := f.history("iteration", it.ID)
	var completed eventlog.Event
	for _, e := range h {
		if e.Type == "IterationCompleted" {
			completed = e
		}
	}
	consumer := f.svc.PolicyConsumer()
	for range 2 {
		if err := consumer.Handle(ctx, completed); err != nil {
			f.t.Fatal(err)
		}
	}
	out, err := f.svc.Iteration(ctx, pc, it.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	return out
}

// finish moves an issue along the default workflow to Done.
func (f *fixture) finish(key string) {
	f.t.Helper()
	ids := map[string]uuid.UUID{}
	for _, w := range f.resolved("HZN").Workflows {
		for _, st := range w.Statuses {
			ids[st.Name] = st.ID
		}
	}
	for _, name := range []string{"Todo", "In progress", "Done"} {
		if _, err := f.issues.Transition(context.Background(), f.issue(key), 0, ids[name]); err != nil {
			f.t.Fatalf("%s: %v", name, err)
		}
	}
}

// redeliver hands the iteration's IterationCompleted to the policy worker
// again.
func (f *fixture) redeliver(it planning.Iteration) {
	f.t.Helper()
	for _, e := range f.history("iteration", it.ID) {
		if e.Type == "IterationCompleted" {
			if err := f.svc.PolicyConsumer().Handle(context.Background(), e); err != nil {
				f.t.Fatal(err)
			}
		}
	}
}

func (f *fixture) setPolicy(policy string) {
	f.t.Helper()
	ws := configdef.Target{Principal: f.owner, WorkspaceID: f.acme.ID}
	if _, err := f.cfg.SetIterationSettings(context.Background(), ws, configdef.IterationSettingsInput{"completion_policy": json.RawMessage(`"` + policy + `"`)}); err != nil {
		f.t.Fatal(err)
	}
}

func TestCompletionRollsOver(t *testing.T) {
	f := newFixture(t)
	done, open, gone := f.create("HZN", "Done", "3"), f.create("HZN", "Open", "5"), f.create("HZN", "Archived", "")
	s1 := f.plan("Sprint 1", "2026-11-09", "2026-11-20")
	s2 := f.plan("Sprint 2", "", "")
	f.load(s1, []string{done.Key, open.Key, gone.Key}, nil)
	f.finish(done.Key)
	if _, err := f.issues.SetArchived(context.Background(), f.issue(gone.Key), 0, true); err != nil {
		t.Fatal(err)
	}

	s1 = f.complete(s1, uuid.NullUUID{UUID: s2.ID, Valid: true})
	if s1.Completion.AppliedAt == nil {
		t.Errorf("completion not marked applied: %+v", s1.Completion)
	}
	d := f.issue(done.Key).Issue
	if d.ArchivedAt == nil || d.IterationID.UUID != s1.ID {
		t.Errorf("finished issue: archived %v, iteration %v", d.ArchivedAt, d.IterationID)
	}
	if o := f.issue(open.Key).Issue; o.ArchivedAt != nil || o.IterationID.UUID != s2.ID {
		t.Errorf("unfinished issue: archived %v, iteration %v", o.ArchivedAt, o.IterationID)
	}
	if g := f.issue(gone.Key).Issue; g.IterationID.UUID != s1.ID {
		t.Errorf("archived issue moved")
	}
	// The completed iteration keeps its finished issues; its stats say so.
	if s1.Stats.Issues != 2 || s1.Stats.ByOutcome["completed"].Points != 3 {
		t.Errorf("stats = %+v", s1.Stats)
	}

	// Each side records the rollover once, by whoever completed it.
	h1, h2 := f.history("iteration", s1.ID), f.history("iteration", s2.ID)
	last1, last2 := h1[len(h1)-1], h2[len(h2)-1]
	if p := payload(last1); last1.Type != "IterationLoaded" || p["reason"] != "rolled_over" || p["to"] != s2.ID.String() {
		t.Errorf("Sprint 1 recorded %s %v", last1.Type, p)
	}
	if p := payload(last2); p["from"] != s1.ID.String() || p["added"].([]any)[0] != open.ID.String() || last2.ActorID.UUID != f.owner.User.ID {
		t.Errorf("Sprint 2 recorded %v by %v", p, last2.ActorID)
	}
	n := 0
	for _, e := range f.history("issue", done.ID) {
		if e.Type == "IssueArchived" {
			n++
			if p := payload(e); p["by"] != "iteration_completed" {
				t.Errorf("archived by %v", p)
			}
		}
	}
	if n != 1 {
		t.Errorf("archived %d times", n)
	}
}

func TestCompletionToBacklogWhenTargetClosed(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.create("HZN", "A", "")
	s1 := f.plan("Sprint 1", "2026-11-09", "2026-11-20")
	s2 := f.plan("Sprint 2", "", "")
	f.load(s1, []string{a.Key}, nil)
	if _, err := f.svc.Start(ctx, f.pc("HZN"), s1.ID, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Complete(ctx, f.pc("HZN"), s1.ID, 0, uuid.NullUUID{UUID: s2.ID, Valid: true}); err != nil {
		t.Fatal(err)
	}
	// Sprint 2 goes before the worker gets to it.
	if err := f.svc.Delete(ctx, f.pc("HZN"), s2.ID); err != nil {
		t.Fatal(err)
	}
	h := f.history("iteration", s1.ID)
	if err := f.svc.PolicyConsumer().Handle(ctx, h[len(h)-1]); err != nil {
		t.Fatal(err)
	}
	if i := f.issue(a.Key).Issue; i.IterationID.Valid || i.ArchivedAt != nil {
		t.Errorf("issue: iteration %v, archived %v", i.IterationID, i.ArchivedAt)
	}
}

func TestCompletionArchiveAllAndNone(t *testing.T) {
	f := newFixture(t)
	a, b := f.create("HZN", "A", ""), f.create("HZN", "B", "")
	f.setPolicy(configdef.CompletionArchiveAll)
	s1 := f.plan("Sprint 1", "2026-11-09", "2026-11-20")
	f.load(s1, []string{a.Key}, nil)
	s1 = f.complete(s1, uuid.NullUUID{})
	if s1.Completion.Policy != configdef.CompletionArchiveAll || f.issue(a.Key).Issue.ArchivedAt == nil {
		t.Errorf("archive_all: %+v, archived %v", s1.Completion, f.issue(a.Key).Issue.ArchivedAt)
	}

	f.setPolicy(configdef.CompletionNone)
	s2 := f.plan("Sprint 2", "2026-11-23", "2026-12-04")
	f.load(s2, []string{b.Key}, nil)
	s2 = f.complete(s2, uuid.NullUUID{})
	if i := f.issue(b.Key).Issue; i.ArchivedAt != nil || i.IterationID.UUID != s2.ID || s2.Completion.AppliedAt == nil {
		t.Errorf("none: archived %v, iteration %v, %+v", i.ArchivedAt, i.IterationID, s2.Completion)
	}
	// What people do afterwards stays: a redelivery doesn't archive again.
	if _, err := f.issues.SetArchived(context.Background(), f.issue(a.Key), 0, false); err != nil {
		t.Fatal(err)
	}
	f.redeliver(s1)
	if f.issue(a.Key).Issue.ArchivedAt != nil {
		t.Errorf("redelivery archived again")
	}
}
