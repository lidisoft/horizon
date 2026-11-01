package document_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lidisoft/horizon/internal/document"
)

// The vectors in testdata/ are shared with the JavaScript implementation
// (web/test/editor/corpus.test.js): both must agree on every one.

func load(t *testing.T, name string, v any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatal(err)
	}
}

var schema = document.Default()

func parse(t *testing.T, raw json.RawMessage) *document.Node {
	t.Helper()
	n, err := schema.ParseJSON(raw)
	if err != nil {
		t.Fatalf("doc %s: %v", raw, err)
	}
	return n
}

func steps(t *testing.T, raws []json.RawMessage) []document.Step {
	t.Helper()
	out := make([]document.Step, len(raws))
	for i, r := range raws {
		s, err := schema.ParseStep(r)
		if err != nil {
			t.Fatalf("step %s: %v", r, err)
		}
		out[i] = s
	}
	return out
}

func sameJSON(a, b []byte) bool {
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && reflect.DeepEqual(x, y)
}

func show(n *document.Node) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestApply(t *testing.T) {
	var cases []struct {
		Name   string
		Doc    json.RawMessage
		Steps  []json.RawMessage
		Result json.RawMessage
		Error  bool
	}
	load(t, "apply.json", &cases)
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			doc := parse(t, c.Doc)
			docs := []*document.Node{doc}
			var err error
			ss := steps(t, c.Steps)
			for _, s := range ss {
				if doc, err = s.Apply(doc); err != nil {
					break
				}
				docs = append(docs, doc)
			}
			if c.Error {
				if err == nil {
					t.Fatalf("applied: %s", show(doc))
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if want := parse(t, c.Result); !doc.Eq(want) {
				t.Fatalf("got  %s\nwant %s", show(doc), show(want))
			}
			// Inverting every step, last first, gives the document back.
			for i := len(ss) - 1; i >= 0; i-- {
				inv, err := ss[i].Invert(docs[i])
				if err != nil {
					t.Fatal(err)
				}
				if doc, err = inv.Apply(doc); err != nil {
					t.Fatalf("invert step %d: %v", i, err)
				}
			}
			if !doc.Eq(docs[0]) {
				t.Fatalf("inverted to %s\nwant %s", show(doc), show(docs[0]))
			}
			// Steps write the JSON they were read from.
			for i, s := range ss {
				raw, _ := document.MarshalStep(s)
				if !sameJSON(raw, c.Steps[i]) {
					t.Fatalf("step %d writes %s, read from %s", i, raw, c.Steps[i])
				}
			}
		})
	}
}

func TestMapping(t *testing.T) {
	var cases []struct {
		Name      string
		Doc       json.RawMessage
		Steps     []json.RawMessage
		Positions []struct {
			Pos, Assoc, To int
			Deleted        *bool
		}
	}
	load(t, "mapping.json", &cases)
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			doc := parse(t, c.Doc)
			m := &document.Mapping{}
			for _, s := range steps(t, c.Steps) {
				var err error
				if doc, err = s.Apply(doc); err != nil {
					t.Fatal(err)
				}
				m.AppendMap(s.StepMap(), -1)
			}
			for _, p := range c.Positions {
				r := m.MapResult(p.Pos, p.Assoc)
				if r.Pos != p.To || (p.Deleted != nil && r.Deleted() != *p.Deleted) {
					t.Errorf("%d (assoc %d) → %d deleted %v; want %d deleted %v", p.Pos, p.Assoc, r.Pos, r.Deleted(), p.To, p.Deleted)
				}
			}
		})
	}
}

func TestRebase(t *testing.T) {
	var cases []struct {
		Name                   string
		Doc, Result            json.RawMessage
		Local, Remote, Rebased []json.RawMessage
	}
	load(t, "rebase.json", &cases)
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			doc, rebased, err := document.Rebase(parse(t, c.Doc), steps(t, c.Local), steps(t, c.Remote))
			if err != nil {
				t.Fatal(err)
			}
			if want := parse(t, c.Result); !doc.Eq(want) {
				t.Fatalf("got  %s\nwant %s", show(doc), show(want))
			}
			want := steps(t, c.Rebased)
			if len(rebased) != len(want) {
				t.Fatalf("rebased %d steps, want %d", len(rebased), len(want))
			}
			for i := range want {
				if !document.StepsEqual(rebased[i], want[i]) {
					a, _ := document.MarshalStep(rebased[i])
					b, _ := document.MarshalStep(want[i])
					t.Errorf("step %d: %s, want %s", i, a, b)
				}
			}
		})
	}
}

func TestSchema(t *testing.T) {
	var cases []struct {
		Name  string
		Doc   json.RawMessage
		Valid bool
	}
	load(t, "schema.json", &cases)
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			n, err := schema.ParseJSON(c.Doc)
			if c.Valid != (err == nil) {
				t.Fatalf("valid = %v (%v), want %v", err == nil, err, c.Valid)
			}
			if err == nil {
				// The JSON form round-trips.
				again, err := schema.ParseJSON([]byte(show(n)))
				if err != nil || !again.Eq(n) {
					t.Fatalf("round trip: %v", err)
				}
			}
		})
	}
}
