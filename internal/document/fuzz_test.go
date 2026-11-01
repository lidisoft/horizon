package document_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lidisoft/horizon/internal/document"
)

// FuzzApply feeds arbitrary steps to the corpus documents: steps come from
// clients, so a bad one must fail, never panic.
func FuzzApply(f *testing.F) {
	raw, err := os.ReadFile(filepath.Join("testdata", "apply.json"))
	if err != nil {
		f.Fatal(err)
	}
	var cases []struct {
		Doc   json.RawMessage
		Steps []json.RawMessage
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		f.Fatal(err)
	}
	for _, c := range cases {
		for _, s := range c.Steps {
			f.Add([]byte(c.Doc), []byte(s))
		}
	}
	f.Fuzz(func(t *testing.T, docJSON, stepJSON []byte) {
		doc, err := schema.ParseJSON(docJSON)
		if err != nil {
			return
		}
		step, err := schema.ParseStep(stepJSON)
		if err != nil {
			return
		}
		after, err := step.Apply(doc)
		if err != nil || after.Check() != nil {
			return
		}
		// What applies and checks survives its JSON form, and inverts (mark
		// steps only invert exactly over uniformly marked text, as in
		// ProseMirror: removing a mark the text lacks changes nothing).
		if _, err := schema.ParseJSON([]byte(show(after))); err != nil {
			t.Fatalf("step %s made an invalid document: %v", stepJSON, err)
		}
		switch step.(type) {
		case *document.AddMarkStep, *document.RemoveMarkStep:
			return
		}
		inv, err := step.Invert(doc)
		if err != nil {
			t.Fatalf("invert: %v", err)
		}
		back, err := inv.Apply(after)
		if err != nil || !back.Eq(doc) {
			t.Fatalf("inverting %s: %v", stepJSON, err)
		}
	})
}
