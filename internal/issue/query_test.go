package issue_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/query"
)

func TestQuery(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	wc := f.wc(f.owner)
	a := f.create(f.owner, "Task", "Outbox relay")
	b := f.create(f.owner, "Bug", "Replay after reconnect")
	c := f.create(f.owner, "Task", "Rank keys")
	if _, err := f.svc.Assign(ctx, f.enter(f.owner, a.Key), 0, f.owner.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Prioritize(ctx, f.enter(f.owner, b.Key), 0, "urgent"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Update(ctx, f.enter(f.owner, c.Key), 0, issue.Update{DueDate: s("2026-11-20")}); err != nil {
		t.Fatal(err)
	}
	// A custom number field.
	points, err := f.cfg.CreateField(ctx, f.ws_(), configdef.FieldInput{Name: s("Risk"), Type: s("number")})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.cfg.SetTypeFields(ctx, f.ws_(), f.typeID("Task"), []configdef.TypeField{{FieldID: points.ID}}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Update(ctx, f.enter(f.owner, a.Key), 0, issue.Update{Fields: map[string]json.RawMessage{points.ID.String(): json.RawMessage(`7`)}}); err != nil {
		t.Fatal(err)
	}

	run := func(filter string, sorts ...query.Sort) string {
		t.Helper()
		var n *query.Node
		if filter != "" {
			n = &query.Node{}
			if err := json.Unmarshal([]byte(filter), n); err != nil {
				t.Fatal(err)
			}
		}
		res, err := f.svc.Query(ctx, wc, query.Query{Filter: n, Sort: sorts}, nil, "")
		if err != nil {
			t.Fatalf("%s: %v", filter, err)
		}
		var keys []string
		for _, i := range res.Items {
			keys = append(keys, i.Key)
		}
		return strings.Join(keys, ",")
	}
	cases := []struct{ filter, want string }{
		{``, "HZN-1,HZN-2,HZN-3"},
		{`{"field":"assignee","op":"eq","value":"@me"}`, "HZN-1"},
		{`{"field":"assignee","op":"is_empty"}`, "HZN-2,HZN-3"},
		{`{"field":"priority","op":"in","value":["urgent","high"]}`, "HZN-2"},
		{`{"field":"status.outcome","op":"eq","value":"open"}`, "HZN-1,HZN-2,HZN-3"},
		{`{"field":"type","op":"eq","value":"` + f.typeID("Bug").String() + `"}`, "HZN-2"},
		{`{"field":"text","op":"contains","value":"repl"}`, "HZN-2"},
		{`{"field":"title","op":"contains","value":"rank"}`, "HZN-3"},
		{`{"field":"due_date","op":"lte","value":"2026-12-01"}`, "HZN-3"},
		{`{"field":"cf:` + points.ID.String() + `","op":"gte","value":5}`, "HZN-1"},
		{`{"or":[{"field":"assignee","op":"eq","value":"@me"},{"field":"priority","op":"eq","value":"urgent"}]}`, "HZN-1,HZN-2"},
		{`{"not":{"field":"project","op":"eq","value":"HZN"}}`, ""},
	}
	for _, tc := range cases {
		if got := run(tc.filter); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.filter, got, tc.want)
		}
	}
	if got := run("", query.Sort{Field: "priority"}); got != "HZN-2,HZN-1,HZN-3" {
		t.Errorf("by priority: %s", got)
	}

	// Errors name the place.
	_, err = f.svc.Query(ctx, wc, query.Query{Filter: &query.Node{And: []*query.Node{{Field: "colour", Op: "eq", Value: json.RawMessage(`"x"`)}}}}, nil, "")
	wantField(t, err, "filter.and[0].field", "unknown_field")
	_, err = f.svc.Query(ctx, wc, query.Query{Filter: &query.Node{Field: "priority", Op: "lt", Value: json.RawMessage(`"high"`)}}, nil, "")
	wantField(t, err, "filter.op", "bad_op")

	// Pages.
	res, err := f.svc.Query(ctx, wc, query.Query{Limit: 2}, nil, "")
	if err != nil || res.Total != 3 || len(res.Items) != 2 || res.Next != "2" {
		t.Fatalf("page 1: %+v, %v", res, err)
	}
	res, _ = f.svc.Query(ctx, wc, query.Query{Limit: 2}, nil, res.Next)
	if len(res.Items) != 1 || res.Next != "" {
		t.Errorf("page 2: %+v", res)
	}
	// Projects the caller doesn't see are left out.
	f.project("SEC", "Secret", "private")
	member := f.member("member@example.com")
	if res, _ := f.svc.Query(ctx, f.wc(member), query.Query{}, []string{"SEC"}, ""); len(res.Items) != 0 {
		t.Errorf("private project leaked: %+v", res)
	}
}
