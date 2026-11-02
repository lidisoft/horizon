package report_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/planning"
)

func TestOverview(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.create("HZN", "Relay", "3")
	b := f.create("HZN", "Replay", "5")
	if _, err := f.issues.Assign(ctx, f.enter(a.Key), 0, f.owner.User.ID); err != nil {
		t.Fatal(err)
	}
	pc := f.pc("HZN")
	it, err := f.plans.Plan(ctx, pc, planning.PlanInput{Name: "Sprint 1", Goal: "Ship the relay", StartDate: "2026-11-02", EndDate: "2026-11-13"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.plans.Load(ctx, pc, it.ID, planning.LoadInput{Add: []string{a.Key, b.Key}}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.plans.Start(ctx, pc, it.ID, 0); err != nil {
		t.Fatal(err)
	}
	f.at("2026-11-03 10:00")
	f.move(a.Key, "Todo", "In progress")
	rel, err := f.plans.CreateRelease(ctx, pc, planning.ReleaseInput{Name: ptr("v1"), ReleaseDate: ptr("2026-11-20")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.issues.SetRelease(ctx, f.enter(a.Key), 0, uuid.NullUUID{UUID: rel.ID, Valid: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.plans.CreateMilestone(ctx, f.wc(f.owner), planning.MilestoneInput{Name: ptr("Beta"), Date: ptr("2026-11-30")}); err != nil {
		t.Fatal(err)
	}
	f.at("2026-11-04 12:00")
	f.catchUp()

	o, err := f.reports().Overview(ctx, f.pc("HZN"))
	if err != nil {
		t.Fatal(err)
	}
	if s := o.Sprint; s == nil || s.Iteration.Name != "Sprint 1" || s.Goal != "Ship the relay" || s.DaysLeft != 8 ||
		s.ByOutcome["started"] != 1 || s.ByOutcome["unstarted"] != 1 || len(s.Remaining) != 12 || vals(s.Remaining[:4]) != "2 2 2 -" {
		t.Errorf("sprint = %+v", o.Sprint)
	}
	if o.NextRelease == nil || o.NextRelease.Name != "v1" || o.NextRelease.ByOutcome["started"] != 1 {
		t.Errorf("release = %+v", o.NextRelease)
	}
	if o.NextMilestone == nil || o.NextMilestone.Name != "Beta" {
		t.Errorf("milestone = %+v", o.NextMilestone)
	}
	if len(o.Flow.WIP) != 1 || o.Flow.WIP[0].Name != "In progress" || o.Flow.WIP[0].Count != 1 || len(o.Flow.Throughput) != 4 {
		t.Errorf("flow = %+v", o.Flow)
	}
	if len(o.Activity) == 0 || o.Activity[0].Aggregate.Key != a.Key || o.Activity[0].Type != "issue.release_changed" {
		t.Errorf("activity = %+v", o.Activity)
	}
	if len(o.MyWork) != 1 || o.MyWork[0].Key != a.Key {
		t.Errorf("my work = %+v", o.MyWork)
	}
}
