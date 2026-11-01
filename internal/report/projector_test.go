package report_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/planning"
)

// dump renders the projections, in a stable order, without generated ids.
func (f *fixture) dump() string {
	f.t.Helper()
	var b strings.Builder
	keys := map[uuid.UUID]string{}
	rows, _ := f.pool.Query(context.Background(), `SELECT i.id, p.key || '-' || i.number FROM issues i JOIN projects p ON p.id = i.project_id`)
	for rows.Next() {
		var id uuid.UUID
		var k string
		_ = rows.Scan(&id, &k)
		keys[id] = k
	}
	rows.Close()
	ts := func(t *time.Time) string {
		if t == nil {
			return "-"
		}
		return t.UTC().Format("01-02 15:04")
	}
	for _, q := range []string{
		`SELECT issue_id, outcome, entered_at, left_at FROM report_status_intervals ORDER BY issue_id, entered_at, id`,
		`SELECT issue_id, 'milestones', created_at, first_started_at, completed_at, abandoned_at FROM report_issue_milestones ORDER BY issue_id`,
		`SELECT issue_id, 'scope', added_at, removed_at FROM report_iteration_scope ORDER BY issue_id, added_at`,
		`SELECT issue_id, coalesce(weight::text, 'none'), valid_from, valid_to FROM report_estimates ORDER BY issue_id, valid_from`,
	} {
		rows, err := f.pool.Query(context.Background(), q)
		if err != nil {
			f.t.Fatal(err)
		}
		for rows.Next() {
			vals, _ := rows.Values()
			id := vals[0].([16]byte)
			fmt.Fprintf(&b, "%s %v", keys[uuid.UUID(id)], vals[1])
			for _, v := range vals[2:] {
				var t *time.Time
				if tv, ok := v.(time.Time); ok {
					t = &tv
				}
				b.WriteString(" " + ts(t))
			}
			b.WriteString("\n")
		}
		rows.Close()
	}
	return b.String()
}

func TestProjector(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.create("HZN", "Relay", "3")
	b := f.create("HZN", "Replay", "5")
	c := f.create("HZN", "Search", "")
	pc := f.pc("HZN")
	it, err := f.plans.Plan(ctx, pc, planning.PlanInput{Name: "Sprint 1", StartDate: "2026-11-02", EndDate: "2026-11-13"})
	if err != nil {
		t.Fatal(err)
	}
	f.at("2026-11-02 09:30")
	if it, err = f.plans.Load(ctx, pc, it.ID, planning.LoadInput{Add: []string{a.Key, b.Key}}); err != nil {
		t.Fatal(err)
	}
	f.at("2026-11-02 10:00")
	if it, err = f.plans.Start(ctx, pc, it.ID, 0); err != nil {
		t.Fatal(err)
	}
	f.at("2026-11-03 10:00")
	f.move(a.Key, "Todo", "In progress")
	f.at("2026-11-04 10:00")
	if _, err = f.plans.Load(ctx, pc, it.ID, planning.LoadInput{Add: []string{c.Key}}); err != nil {
		t.Fatal(err)
	}
	f.at("2026-11-05 10:00")
	if _, err := f.issues.Estimate(ctx, f.enter(b.Key), 0, "8"); err != nil {
		t.Fatal(err)
	}
	f.at("2026-11-06 10:00")
	f.move(a.Key, "Done")
	f.move(b.Key, "Canceled")
	f.at("2026-11-06 11:00")
	if _, err = f.plans.Load(ctx, pc, it.ID, planning.LoadInput{Remove: []string{c.Key}}); err != nil {
		t.Fatal(err)
	}

	if n := f.catchUp(); n == 0 {
		t.Fatal("nothing replayed")
	}
	if n := f.catchUp(); n != 0 {
		t.Errorf("caught up twice: %d", n)
	}
	want := `HZN-1 unstarted 11-02 09:00 11-03 10:00
HZN-1 unstarted 11-03 10:00 11-03 10:00
HZN-1 started 11-03 10:00 11-06 10:00
HZN-1 completed 11-06 10:00 -
HZN-2 unstarted 11-02 09:00 11-06 10:00
HZN-2 abandoned 11-06 10:00 -
HZN-3 unstarted 11-02 09:00 -
HZN-1 milestones 11-02 09:00 11-03 10:00 11-06 10:00 -
HZN-2 milestones 11-02 09:00 - - 11-06 10:00
HZN-3 milestones 11-02 09:00 - - -
HZN-1 scope 11-02 09:30 -
HZN-2 scope 11-02 09:30 -
HZN-3 scope 11-04 10:00 11-06 11:00
HZN-1 3 11-02 09:00 -
HZN-2 5 11-02 09:00 11-05 10:00
HZN-2 8 11-05 10:00 -
`
	if got := sorted(f.dump()); got != sorted(want) {
		t.Errorf("projections:\n%s\nwant:\n%s", got, sorted(want))
	}

	// "In progress" stops counting as started: HZN-1 was never started.
	inProgress := f.category("In progress")
	unstarted := configdef.OutcomeUnstarted
	if _, err := f.cfg.UpdateCategory(ctx, configdef.Target{Principal: f.owner, WorkspaceID: f.acme.ID}, inProgress.ID, 0, configdef.CategoryInput{Outcome: &unstarted}); err != nil {
		t.Fatal(err)
	}
	f.catchUp()
	if got := f.dump(); !strings.Contains(got, "HZN-1 milestones 11-02 09:00 - 11-06 10:00 -") {
		t.Errorf("after the outcome change:\n%s", got)
	}

	// A deleted issue leaves nothing; a rebuild gives the same projections.
	if err := f.issues.Delete(ctx, f.enter(b.Key), ""); err != nil {
		t.Fatal(err)
	}
	f.catchUp()
	before := f.dump()
	if strings.Contains(before, "HZN-2") {
		t.Errorf("HZN-2 still projected:\n%s", before)
	}
	if err := f.proj.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.dump(); got != "" {
		t.Errorf("after the reset:\n%s", got)
	}
	f.catchUp()
	if got := f.dump(); got != before {
		t.Errorf("rebuilt:\n%s\nwant:\n%s", got, before)
	}
}

func sorted(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := 1; i < len(lines); i++ {
		for j := i; j > 0 && lines[j] < lines[j-1]; j-- {
			lines[j], lines[j-1] = lines[j-1], lines[j]
		}
	}
	return strings.Join(lines, "\n")
}
