package issue_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lidisoft/horizon/internal/issue"
)

const listDoc = `{"type":"doc","content":[
	{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Context"}]},
	{"type":"bullet_list","content":[{"type":"list_item","content":[{"type":"paragraph","content":[{"type":"text","text":"relay"}]}]}]}]}`

func TestDescriptionDocuments(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pc := f.pc(f.owner, "HZN")

	// Created with a document: the text follows it.
	i, err := f.svc.Create(ctx, pc, issue.NewIssue{TypeID: f.typeID("Task"), Title: "Doc", Description: issue.Text{Doc: json.RawMessage(listDoc)}})
	if err != nil {
		t.Fatal(err)
	}
	if i.Description != "Context\n- relay" || !strings.Contains(string(i.DescriptionDoc), `"bullet_list"`) {
		t.Errorf("created: %q %s", i.Description, i.DescriptionDoc)
	}
	got, err := f.svc.Get(ctx, f.enter(f.owner, i.Key))
	if err != nil || !strings.Contains(string(got.DescriptionDoc), `"heading"`) {
		t.Fatalf("get: %s, %v", got.DescriptionDoc, err)
	}

	// Plain text still works: one paragraph per line.
	updated, err := f.svc.Update(ctx, f.enter(f.owner, i.Key), 0, issue.Update{Description: issue.PlainText("one\ntwo")})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != "one\ntwo" || strings.Count(string(updated.DescriptionDoc), `"paragraph"`) != 2 {
		t.Errorf("plain update: %q %s", updated.Description, updated.DescriptionDoc)
	}
	// The same document again changes nothing.
	again, err := f.svc.Update(ctx, f.enter(f.owner, i.Key), 0, issue.Update{Description: issue.Text{Doc: updated.DescriptionDoc}})
	if err != nil || again.Version != updated.Version {
		t.Errorf("no-op update: version %d → %d, %v", updated.Version, again.Version, err)
	}

	// Invalid documents and images of unknown attachments are refused.
	_, err = f.svc.Update(ctx, f.enter(f.owner, i.Key), 0, issue.Update{Description: issue.Text{Doc: json.RawMessage(`{"type":"doc"}`)}})
	wantField(t, err, "description_doc", "invalid")
	image := `{"type":"doc","content":[{"type":"image","attrs":{"attachment":"01929b2a-0000-7000-8000-0000000000bb"}}]}`
	_, err = f.svc.Update(ctx, f.enter(f.owner, i.Key), 0, issue.Update{Description: issue.Text{Doc: json.RawMessage(image)}})
	wantField(t, err, "description_doc", "unknown_attachment")

	// Issues from before documents read as paragraphs of their text.
	old := f.create(f.owner, "Task", "Old")
	if _, err := f.pool.Exec(ctx, `UPDATE issues SET description = 'line one' || chr(10) || 'line two' WHERE id = $1`, old.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.svc.Get(ctx, f.enter(f.owner, old.Key)); !strings.Contains(string(got.DescriptionDoc), `"line two"`) {
		t.Errorf("legacy: %s", got.DescriptionDoc)
	}
}

func TestCommentDocuments(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.create(f.owner, "Task", "A")
	ic := f.enter(f.owner, a.Key)

	c, err := f.svc.AddComment(ctx, ic, issue.Text{Doc: json.RawMessage(listDoc)})
	if err != nil {
		t.Fatal(err)
	}
	if c.Body != "Context\n- relay" || !strings.Contains(string(c.BodyDoc), "bullet_list") {
		t.Errorf("comment: %q %s", c.Body, c.BodyDoc)
	}
	_, err = f.svc.AddComment(ctx, ic, issue.Text{Doc: json.RawMessage(`{"type":"doc","content":[{"type":"paragraph"}]}`)})
	wantField(t, err, "body", "required")
	edited, err := f.svc.EditComment(ctx, ic, c.ID, c.Version, issue.PlainText("plain now"))
	if err != nil || edited.Body != "plain now" || edited.Version != c.Version+1 {
		t.Fatalf("edit: %+v, %v", edited, err)
	}
	page, err := f.svc.Comments(ctx, ic, "", 0)
	if err != nil || len(page.Items) != 1 || !strings.Contains(string(page.Items[0].BodyDoc), "plain now") {
		t.Fatalf("list: %+v, %v", page, err)
	}
}
