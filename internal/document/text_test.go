package document_test

import (
	"testing"

	"github.com/lidisoft/horizon/internal/document"
)

func TestTextProjection(t *testing.T) {
	doc := parse(t, []byte(`{"type":"doc","content":[
		{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Context"}]},
		{"type":"paragraph","content":[{"type":"text","text":"See "},{"type":"issue_ref","attrs":{"key":"HZN-12"}},{"type":"hard_break"},{"type":"text","text":"then"}]},
		{"type":"bullet_list","content":[{"type":"list_item","content":[{"type":"paragraph","content":[{"type":"text","text":"one"}]}]}]},
		{"type":"ordered_list","attrs":{"start":3},"content":[{"type":"list_item","content":[{"type":"paragraph","content":[{"type":"text","text":"three"}]}]}]},
		{"type":"task_list","content":[{"type":"task_item","attrs":{"checked":true},"content":[{"type":"paragraph","content":[{"type":"text","text":"done"}]}]}]},
		{"type":"table","content":[{"type":"table_row","content":[
			{"type":"table_cell","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]},
			{"type":"table_cell","content":[{"type":"paragraph","content":[{"type":"text","text":"b"}]}]}]}]},
		{"type":"paragraph"}]}`))
	want := "Context\nSee HZN-12\nthen\n- one\n3. three\n[x] done\na | b"
	if got := document.PlainText(doc); got != want {
		t.Errorf("text = %q\nwant  %q", got, want)
	}
	if document.IsEmpty(doc) {
		t.Error("empty")
	}

	from := document.FromText(schema, "first\n\nsecond\n\n")
	if got := show(from); got != `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"first"}]},{"type":"paragraph"},{"type":"paragraph","content":[{"type":"text","text":"second"}]}]}` {
		t.Errorf("from text = %s", got)
	}
	if err := from.Check(); err != nil {
		t.Error(err)
	}
	if !document.IsEmpty(document.FromText(schema, "")) {
		t.Error("empty text gives an empty document")
	}
}
