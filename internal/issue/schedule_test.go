package issue_test

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"testing"

	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/issue"
)

func TestSchedule(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	// HZN works Monday to Friday, without Wednesday 11 November.
	if _, err := f.cfg.SetCalendar(ctx, configdef.Target{Principal: f.owner, WorkspaceID: f.acme.ID, ProjectID: uuid.NullUUID{UUID: f.hzn.ID, Valid: true}},
		configdef.CalendarInput{"holidays": json.RawMessage(`[{"date":"2026-11-11","name":"Armistice"}]`), "holidays_mode": json.RawMessage(`"replace"`)}); err != nil {
		t.Fatal(err)
	}
	dated := func(title, start, due string) issue.Issue {
		i := f.create(f.owner, "Task", title)
		u, err := f.svc.Update(ctx, f.enter(f.owner, i.Key), 0, issue.Update{StartDate: &start, DueDate: &due})
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	link := func(from, to issue.Issue, lag int32) {
		var l *int32
		if lag != 0 {
			l = &lag
		}
		if _, err := f.svc.AddLink(ctx, f.enter(f.owner, from.Key), issue.LinkInput{Type: "precedes", TargetKey: to.Key, LagDays: l}); err != nil {
			t.Fatal(err)
		}
	}
	a := dated("Design", "2026-11-02", "2026-11-04")
	b := dated("Build", "2026-11-05", "2026-11-06") // two working days
	c := dated("Review", "", "2026-11-09")          // a due date only
	d := dated("Ship", "2026-11-20", "2026-11-20")
	link(a, b, 0)
	link(b, c, 1)
	link(c, d, 0)
	link(c, a, 0) // a cycle back to the issue moved is ignored

	dates := func(m issue.Span) string {
		s := func(p *string) string {
			if p == nil {
				return "-"
			}
			return *p
		}
		return s(m.StartDate) + ".." + s(m.DueDate)
	}
	summary := func(p issue.SchedulePlan) []string {
		var out []string
		for _, m := range p.Moves {
			out = append(out, m.Key+" "+dates(m.To)+" +"+strconv.Itoa(m.Shift)+" after "+m.After)
		}
		return out
	}

	// Design slips two days: Build follows on Monday, Review a working day
	// after Build (skipping the holiday); Ship has room.
	plan, err := f.svc.Schedule(ctx, f.enter(f.owner, a.Key), a.Version, "2026-11-04", "2026-11-06", false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{b.Key + " 2026-11-09..2026-11-10 +2 after " + a.Key, c.Key + " -..2026-11-13 +3 after " + b.Key}
	if got := summary(plan); !slices.Equal(got, want) || plan.Applied {
		t.Fatalf("plan = %v, want %v", got, want)
	}
	if got, _ := f.svc.Enter(ctx, f.wc(f.owner), b.Key); got.Issue.StartDate.Time.Format("2006-01-02") != "2026-11-05" {
		t.Error("a proposal moved Build")
	}

	// Confirmed, everything moves.
	plan, err = f.svc.Schedule(ctx, f.enter(f.owner, a.Key), a.Version, "2026-11-04", "2026-11-06", true)
	if err != nil || !plan.Applied || plan.Issue.Result != "moved" || plan.Moves[0].Result != "moved" || plan.Moves[1].Result != "moved" {
		t.Fatalf("applied = %+v, %v", plan, err)
	}
	for key, want := range map[string]string{a.Key: "2026-11-04..2026-11-06", b.Key: "2026-11-09..2026-11-10", c.Key: "-..2026-11-13", d.Key: "2026-11-20..2026-11-20"} {
		i, _ := f.svc.Get(ctx, f.enter(f.owner, key))
		if got := dates(issue.Span{StartDate: i.StartDate, DueDate: i.DueDate}); got != want {
			t.Errorf("%s = %s, want %s", key, got, want)
		}
	}

	// Moving earlier pulls nothing; a stale version is refused.
	if plan, err := f.svc.Schedule(ctx, f.enter(f.owner, a.Key), 0, "2026-11-02", "2026-11-03", false); err != nil || len(plan.Moves) != 0 {
		t.Errorf("earlier = %+v, %v", plan, err)
	}
	_, err = f.svc.Schedule(ctx, f.enter(f.owner, a.Key), a.Version, "2026-11-02", "2026-11-03", false)
	wantCode(t, err, "version_conflict")
	_, err = f.svc.Schedule(ctx, f.enter(f.owner, a.Key), 0, "2026-11-05", "2026-11-03", false)
	wantField(t, err, "due_date", "before_start")

	deps, err := f.svc.Dependencies(ctx, f.wc(f.owner), f.hzn.ID)
	if err != nil || len(deps) != 4 || deps[1].From != b.Key || deps[1].To != c.Key || deps[1].LagDays != 1 {
		t.Errorf("dependencies = %+v, %v", deps, err)
	}
}
