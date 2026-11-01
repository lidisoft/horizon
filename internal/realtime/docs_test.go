package realtime_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
)

// waitFor reads frames until one of type t comes (other frames are skipped).
func (s *socket) waitFor(t string) frame {
	s.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if f := s.next(time.Until(deadline)); f.T == t {
			return f
		}
	}
	s.t.Fatalf("no %s frame", t)
	return frame{}
}

func TestPresenceAndDescriptions(t *testing.T) {
	e := setup(t)
	ann := e.user("ann@example.com")
	if err := e.ws.AddMember(context.Background(), e.wc(e.owner).Workspace.ID, ann.User.ID); err != nil {
		t.Fatal(err)
	}
	e.create("HZN", "Shared")
	o := e.dial("owner@example.com")
	a := e.dial("ann@example.com")

	// Presence: both on the issue, ann editing.
	o.send(map[string]any{"t": "presence", "d": map[string]string{"ws": "acme", "issue": "HZN-1", "state": "viewing"}})
	a.send(map[string]any{"t": "presence", "d": map[string]string{"ws": "acme", "issue": "HZN-1", "state": "editing"}})
	for {
		f := o.waitFor("presence")
		if strings.Count(string(f.D), `"id"`) == 2 && strings.Contains(string(f.D), `"state":"editing"`) {
			break
		}
	}

	// Both join the description; the owner's batch reaches ann, ann's stale one is rejected.
	o.send(map[string]any{"t": "doc.join", "id": "j", "d": map[string]string{"ws": "acme", "issue": "HZN-1"}})
	if f := o.waitFor("doc.joined"); !strings.Contains(string(f.D), `"version":0`) || !strings.Contains(string(f.D), `"doc"`) {
		t.Fatalf("joined: %s", f.D)
	}
	a.send(map[string]any{"t": "doc.join", "d": map[string]any{"ws": "acme", "issue": "HZN-1", "version": 0}})
	a.waitFor("doc.joined")
	step := json.RawMessage(`{"stepType":"replace","from":1,"to":1,"slice":{"content":[{"type":"text","text":"Hi"}]}}`)
	o.send(map[string]any{"t": "doc.steps", "d": map[string]any{"ws": "acme", "issue": "HZN-1", "version": 0, "client": "o1", "steps": []json.RawMessage{step}}})
	if f := o.waitFor("doc.ack"); !strings.Contains(string(f.D), `"version":1`) {
		t.Errorf("ack: %s", f.D)
	}
	if f := a.waitFor("doc.steps"); !strings.Contains(string(f.D), `"client":"o1"`) {
		t.Errorf("steps: %s", f.D)
	}
	a.send(map[string]any{"t": "doc.steps", "d": map[string]any{"ws": "acme", "issue": "HZN-1", "version": 0, "client": "a1", "steps": []json.RawMessage{step}}})
	if f := a.waitFor("doc.reject"); !strings.Contains(string(f.D), `"version":1`) || !strings.Contains(string(f.D), `"o1"`) {
		t.Errorf("reject: %s", f.D)
	}

	// Ann's cursor reaches the owner, not ann.
	a.send(map[string]any{"t": "doc.cursor", "d": map[string]any{"ws": "acme", "issue": "HZN-1", "version": 1, "anchor": 2, "head": 3}})
	if f := o.waitFor("doc.cursor"); !strings.Contains(string(f.D), `"name":"ann"`) || !strings.Contains(string(f.D), `"head":3`) {
		t.Errorf("cursor: %s", f.D)
	}

	// Leaving: the owner hears ann's cursor go, and the text is snapshotted.
	a.send(map[string]any{"t": "doc.leave", "d": map[string]string{"ws": "acme", "issue": "HZN-1"}})
	if f := o.waitFor("doc.cursor"); !strings.Contains(string(f.D), `"gone":true`) {
		t.Errorf("gone: %s", f.D)
	}
	o.send(map[string]any{"t": "doc.leave", "d": map[string]string{"ws": "acme", "issue": "HZN-1"}})
	deadline := time.Now().Add(5 * time.Second)
	var text string
	for time.Now().Before(deadline) {
		if i, err := e.issues.Get(context.Background(), e.enter(e.owner, "HZN-1")); err == nil {
			text = i.Description
		}
		if text == "Hi" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if text != "Hi" {
		t.Errorf("text after the last editor left = %q", text)
	}
}

func (e *env) enter(p identity.Principal, key string) issue.Context {
	e.t.Helper()
	ic, err := e.issues.Enter(context.Background(), e.wc(p), key)
	if err != nil {
		e.t.Fatal(err)
	}
	return ic
}
