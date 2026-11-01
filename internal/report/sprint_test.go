package report_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/planning"
	"github.com/lidisoft/horizon/internal/report"
)

func ptr[T any](v T) *T { return &v }

// vals renders a series, "-" for days to come.
func vals(s []*float64) string {
	out := ""
	for k, v := range s {
		if k > 0 {
			out += " "
		}
		if v == nil {
			out += "-"
		} else {
			b, _ := json.Marshal(*v)
			out += string(b)
		}
	}
	return out
}

// sprintFixture runs a sprint, Monday 2 to Friday 6 November:
// HZN-1 (3), HZN-2 (5) and HZN-4 (2) committed; HZN-3 added Wednesday
// (estimated 1) when HZN-4 is removed; HZN-1 done Thursday, when HZN-2
// goes to 8; completed Friday 17:00 with HZN-2 and HZN-3 unfinished.
func sprintFixture(t *testing.T) (*fixture, *report.Service, planning.Iteration) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.create("HZN", "Relay", "3")
	b := f.create("HZN", "Replay", "5")
	c := f.create("HZN", "Search", "")
	d := f.create("HZN", "Docs", "2")
	pc := f.pc("HZN")
	it, err := f.plans.Plan(ctx, pc, planning.PlanInput{Name: "Sprint 1", StartDate: "2026-11-02", EndDate: "2026-11-06"})
	if err != nil {
		t.Fatal(err)
	}
	f.at("2026-11-02 09:30")
	if _, err = f.plans.Load(ctx, pc, it.ID, planning.LoadInput{Add: []string{a.Key, b.Key, d.Key}}); err != nil {
		t.Fatal(err)
	}
	f.at("2026-11-02 10:00")
	if _, err = f.plans.Start(ctx, pc, it.ID, 0); err != nil {
		t.Fatal(err)
	}
	f.at("2026-11-03 10:00")
	f.move(a.Key, "Todo", "In progress")
	f.at("2026-11-04 10:00")
	if _, err = f.plans.Load(ctx, pc, it.ID, planning.LoadInput{Add: []string{c.Key}}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.issues.Estimate(ctx, f.enter(c.Key), 0, "1"); err != nil {
		t.Fatal(err)
	}
	f.at("2026-11-04 11:00")
	if _, err = f.plans.Load(ctx, pc, it.ID, planning.LoadInput{Remove: []string{d.Key}}); err != nil {
		t.Fatal(err)
	}
	f.at("2026-11-05 10:00")
	f.move(a.Key, "Done")
	if _, err := f.issues.Estimate(ctx, f.enter(b.Key), 0, "8"); err != nil {
		t.Fatal(err)
	}
	f.at("2026-11-06 17:00")
	if it, err = f.plans.Complete(ctx, pc, it.ID, 0, uuid.NullUUID{}); err != nil {
		t.Fatal(err)
	}
	f.at("2026-11-06 18:00")
	f.catchUp()
	svc := f.reports()
	return f, svc, it
}

func TestBurndown(t *testing.T) {
	f, svc, it := sprintFixture(t)
	ctx := context.Background()
	r := report.Request{Iteration: uuid.NullUUID{UUID: it.ID, Valid: true}, Metric: report.MetricEstimate}
	got, err := svc.Burndown(ctx, f.wc(f.owner), r)
	if err != nil {
		t.Fatal(err)
	}
	check := func(name, got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
	check("metric", got.Metric, "estimate")
	check("days", fmt.Sprint(got.Days), "[2026-11-02 2026-11-03 2026-11-04 2026-11-05 2026-11-06]")
	check("ideal", fmt.Sprint(got.Ideal), "[8 6 4 2 0]")
	check("remaining", vals(got.Remaining), "10 10 9 9 9")
	check("scope", vals(got.Scope), "10 10 9 12 12")
	check("completed", vals(got.Completed), "0 0 0 3 3")

	// In count, and on Wednesday: Thursday and Friday are to come.
	f.at("2026-11-04 12:00")
	r.Metric = ""
	got, _ = f.reports().Burndown(ctx, f.wc(f.owner), r)
	check("remaining Wednesday", vals(got.Remaining), "3 3 3 - -")
	check("count metric", got.Metric, "count")
}

func TestSprintReportAndVelocity(t *testing.T) {
	f, svc, it := sprintFixture(t)
	ctx := context.Background()
	got, err := svc.Sprint(ctx, f.wc(f.owner), report.Request{Iteration: uuid.NullUUID{UUID: it.ID, Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	ended := map[string]string{}
	for _, i := range got.Items {
		ended[i.Key] = fmt.Sprintf("%s committed=%v added=%v", i.Ended, i.Committed, i.Added)
	}
	want := map[string]string{
		"HZN-1": "completed committed=true added=false", "HZN-2": "rolled_over committed=true added=false",
		"HZN-3": "rolled_over committed=false added=true", "HZN-4": "removed committed=true added=false",
	}
	if fmt.Sprint(ended) != fmt.Sprint(want) {
		t.Errorf("sprint = %v", ended)
	}
	if tt := got.Totals; tt.Committed != 3 || tt.Added != 1 || tt.Removed != 1 || tt.Completed != 1 || tt.RolledOver != 2 {
		t.Errorf("totals = %+v", tt)
	}

	v, err := svc.Velocity(ctx, f.wc(f.owner), report.Request{Projects: []string{"HZN"}, Metric: report.MetricEstimate})
	if err != nil || len(v.Items) != 1 || v.Items[0].Committed != 10 || v.Items[0].Completed != 3 || v.Average != 3 {
		t.Errorf("velocity = %+v, %v", v, err)
	}
	_, err = svc.Velocity(ctx, f.wc(f.owner), report.Request{})
	if err == nil {
		t.Error("velocity without a project")
	}
}

func TestProgress(t *testing.T) {
	f, svc, _ := sprintFixture(t)
	ctx := context.Background()
	pc := f.pc("HZN")
	rel, err := f.plans.CreateRelease(ctx, pc, planning.ReleaseInput{Name: ptr("v1"), StartDate: ptr("2026-11-03"), ReleaseDate: ptr("2026-11-20")})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"HZN-1", "HZN-2"} {
		if _, err := f.issues.SetRelease(ctx, f.enter(key), 0, uuid.NullUUID{UUID: rel.ID, Valid: true}); err != nil {
			t.Fatal(err)
		}
	}
	m, err := f.plans.CreateMilestone(ctx, f.wc(f.owner), planning.MilestoneInput{Name: ptr("Beta"), Date: ptr("2026-11-30")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.plans.LinkMilestone(ctx, f.wc(f.owner), m.ID, planning.LinkInput{Add: []string{"HZN-1", "HZN-3"}}); err != nil {
		t.Fatal(err)
	}
	f.catchUp()

	p, err := svc.Progress(ctx, f.wc(f.owner), report.Request{Release: uuid.NullUUID{UUID: rel.ID, Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "v1" || p.ByOutcome["completed"] != 1 || p.ByOutcome["unstarted"] != 1 || fmt.Sprint(p.Days) != "[2026-11-03 2026-11-04 2026-11-05 2026-11-06]" ||
		vals(p.Completed) != "0 0 1 1" || vals(p.Scope) != "2 2 2 2" {
		t.Errorf("release = %+v %s %s", p, vals(p.Scope), vals(p.Completed))
	}
	p, err = svc.Progress(ctx, f.wc(f.owner), report.Request{Milestone: uuid.NullUUID{UUID: m.ID, Valid: true}, Metric: report.MetricEstimate})
	if err != nil || p.ByOutcome["completed"] != 3 || p.ByOutcome["unstarted"] != 1 {
		t.Errorf("milestone = %+v, %v", p, err)
	}
	_, err = svc.Progress(ctx, f.wc(f.owner), report.Request{})
	if err == nil {
		t.Error("progress without a release")
	}
}
