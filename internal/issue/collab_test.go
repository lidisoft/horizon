package issue_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/lidisoft/horizon/internal/issue"
)

// insert is a step inserting text at pos.
func insert(pos int, text string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"stepType":"replace","from":%d,"to":%d,"slice":{"content":[{"type":"text","text":%q}]}}`, pos, pos, text))
}

func TestDescriptionSteps(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.create(f.owner, "Task", "A")
	var sent []issue.DocSteps
	f.svc.OnDocSteps(func(d issue.DocSteps) { sent = append(sent, d) })

	// Joining with nothing gives the document (made from the text).
	j, err := f.svc.JoinDescription(ctx, f.enter(f.owner, a.Key), -1)
	if err != nil || j.Version != 0 || !strings.Contains(string(j.Doc), `"paragraph"`) {
		t.Fatalf("join: %+v %s, %v", j, j.Doc, err)
	}

	// Two clients at version 0: the first wins, the second is told what it missed.
	one, err := f.svc.SubmitSteps(ctx, f.enter(f.owner, a.Key), 0, "c1", []json.RawMessage{insert(1, "Hello")})
	if err != nil || !one.Accepted || one.Version != 1 {
		t.Fatalf("first: %+v, %v", one, err)
	}
	two, err := f.svc.SubmitSteps(ctx, f.enter(f.owner, a.Key), 0, "c2", []json.RawMessage{insert(1, "Hi")})
	if err != nil || two.Accepted || two.Version != 1 || len(two.Missing) != 1 || two.Missing[0].ClientID != "c1" {
		t.Fatalf("second: %+v, %v", two, err)
	}
	// Rebased, it goes in.
	if two, err = f.svc.SubmitSteps(ctx, f.enter(f.owner, a.Key), 1, "c2", []json.RawMessage{insert(6, " world")}); err != nil || !two.Accepted {
		t.Fatalf("rebased: %+v, %v", two, err)
	}
	if len(sent) != 2 || sent[1].Batch.Version != 1 || sent[1].Batch.ClientID != "c2" {
		t.Errorf("broadcasts: %+v", sent)
	}

	// Steps that don't apply, and a future version, are refused.
	_, err = f.svc.SubmitSteps(ctx, f.enter(f.owner, a.Key), 2, "c1", []json.RawMessage{insert(99, "x")})
	wantCode(t, err, "invalid_steps")
	_, err = f.svc.SubmitSteps(ctx, f.enter(f.owner, a.Key), 9, "c1", []json.RawMessage{insert(1, "x")})
	wantCode(t, err, "resync")

	// Catching up from a version; reading the issue replays the steps.
	j, err = f.svc.JoinDescription(ctx, f.enter(f.owner, a.Key), 1)
	if err != nil || j.Version != 2 || len(j.Batches) != 1 || j.Doc != nil {
		t.Fatalf("catch up: %+v, %v", j, err)
	}
	got, _ := f.svc.Get(ctx, f.enter(f.owner, a.Key))
	if !strings.Contains(string(got.DescriptionDoc), `"Hello world"`) {
		t.Errorf("doc = %s", got.DescriptionDoc)
	}

	// The text follows at snapshots: the last editor leaving, or every 50 steps.
	if err := f.svc.FlushDescription(ctx, j.DocumentID); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.svc.Get(ctx, f.enter(f.owner, a.Key)); got.Description != "Hello world" {
		t.Errorf("text = %q", got.Description)
	}
	pos := 12
	for i := 0; i < 50; i++ {
		if _, err := f.svc.SubmitSteps(ctx, f.enter(f.owner, a.Key), int64(2+i), "c1", []json.RawMessage{insert(pos, "!")}); err != nil {
			t.Fatal(err)
		}
		pos++
	}
	if got, _ := f.svc.Get(ctx, f.enter(f.owner, a.Key)); !strings.HasSuffix(got.Description, strings.Repeat("!", 48)) {
		t.Errorf("snapshot text = %q", got.Description)
	}

	// A description written through the API is a step too.
	sent = nil
	if _, err := f.svc.Update(ctx, f.enter(f.owner, a.Key), 0, issue.Update{Description: issue.PlainText("Rewritten")}); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 || sent[0].Batch.ClientID != "api" || sent[0].Batch.Version != 52 {
		t.Errorf("api write broadcast: %+v", sent)
	}
	j, _ = f.svc.JoinDescription(ctx, f.enter(f.owner, a.Key), -1)
	if j.Version != 53 || !strings.Contains(string(j.Doc), "Rewritten") {
		t.Errorf("after api write: %d %s", j.Version, j.Doc)
	}

}
