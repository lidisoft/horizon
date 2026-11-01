package issue_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/query"
)

func TestRoadmap(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.project("SEC", "Secret", project.VisibilityPrivate)
	pc := f.pc(f.owner, "HZN")
	str := func(s string) *string { return &s }
	create := func(typ, title, parent, start, due string) issue.Issue {
		i, err := f.svc.Create(ctx, pc, issue.NewIssue{TypeID: f.typeID(typ), Title: title, ParentKey: parent})
		if err != nil {
			t.Fatal(err)
		}
		if start != "" || due != "" {
			if i, err = f.svc.Update(ctx, f.enter(f.owner, i.Key), 0, issue.Update{StartDate: str(start), DueDate: str(due)}); err != nil {
				t.Fatal(err)
			}
		}
		return i
	}
	sync := create("Epic", "Sync", "", "", "")
	create("Story", "Pull", sync.Key, "2026-11-02", "2026-11-06")
	push := create("Story", "Push", sync.Key, "", "2026-11-20")
	if _, err := f.svc.Transition(ctx, f.enter(f.owner, push.Key), 0, f.status("Canceled")); err != nil {
		t.Fatal(err)
	}
	search := create("Epic", "Search", "", "2026-12-01", "2026-12-18")
	create("Story", "Index", search.Key, "", "")
	create("Epic", "Lonely", "", "2026-11-01", "2026-11-02") // no children: not on the roadmap
	spc := f.pc(f.owner, "SEC")
	secret, _ := f.svc.Create(ctx, spc, issue.NewIssue{TypeID: f.typeID("Epic"), Title: "Hidden"})
	if _, err := f.svc.Create(ctx, spc, issue.NewIssue{TypeID: f.typeID("Story"), Title: "Part", ParentKey: secret.Key}); err != nil {
		t.Fatal(err)
	}

	r, err := f.svc.Roadmap(ctx, f.wc(f.owner), query.Query{}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	byTitle := map[string]issue.RoadmapItem{}
	for _, it := range r.Items {
		byTitle[it.Issue.Title] = it
	}
	if r.Total != 3 || len(byTitle) != 3 || byTitle["Hidden"].Issue.Key == "" {
		t.Fatalf("roadmap = %+v", r)
	}
	s := byTitle["Sync"]
	if !s.Derived || *s.StartDate != "2026-11-02" || *s.DueDate != "2026-11-20" || s.Children.Count != 2 || s.Children.Abandoned != 1 {
		t.Errorf("Sync = %+v %+v", s, s.Children)
	}
	if se := byTitle["Search"]; se.Derived || *se.StartDate != "2026-12-01" || se.Children.Count != 1 || se.Children.StartDate != nil {
		t.Errorf("Search = %+v", se)
	}

	// A member doesn't see the private project's epic; a filter narrows.
	member := f.member("member@example.com")
	if r, _ := f.svc.Roadmap(ctx, f.wc(member), query.Query{}, nil, ""); r.Total != 2 {
		t.Errorf("member sees %d", r.Total)
	}
	q := query.Query{Filter: &query.Node{Field: "title", Op: "contains", Value: json.RawMessage(`"sea"`)}}
	if r, _ := f.svc.Roadmap(ctx, f.wc(f.owner), q, []string{"HZN"}, ""); r.Total != 1 || r.Items[0].Issue.Title != "Search" {
		t.Errorf("filtered = %+v", r)
	}
}
