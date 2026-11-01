// Editor extras without a view: paste parsing, input rules, tables.

import { fromHTML, fromMarkdown, inlineMarkdown, looksLikeMarkdown } from "horizon/editor/clipboard.js";
import { insertText } from "horizon/editor/commands.js";
import { schema } from "horizon/editor/model.js";
import { inputRule } from "horizon/editor/rules.js";
import { EditorState, Selection } from "horizon/editor/state.js";
import { addColumn, addRow, createTable, deleteColumn, deleteRow, goToCell } from "horizon/editor/tables.js";
import { assert, describe, test } from "horizon-test/runner.js";

const json = (nodes) => JSON.stringify(nodes.map((n) => n.toJSON()));

/** Types text in a paragraph doc and applies the rule it triggers. */
function typeWithRules(text) {
  let state = new EditorState(schema.nodeFromJSON({ type: "doc", content: [{ type: "paragraph" }] }), new Selection(1));
  for (const ch of text) {
    insertText(ch)(state, (tr) => (state = state.apply(tr)));
    const rule = inputRule(state);
    if (rule) rule(state, (tr) => (state = state.apply(tr)));
  }
  return state;
}

describe("editor extras", () => {
  test("HTML paste keeps blocks, marks and lists", () => {
    const blocks = fromHTML(`<h2>Plan</h2><p>Use <b>bold</b> and <a href="https://x.test/">a link</a><br>then</p>
      <ul><li>one</li><li><p>two</p><ul><li>nested</li></ul></li></ul><pre>let x = 1</pre><p><img src="x.png"><script>bad()</script></p>`);
    assert.equal(json(blocks), JSON.stringify([
      { type: "heading", attrs: { level: 2 }, content: [{ type: "text", text: "Plan" }] },
      { type: "paragraph", content: [{ type: "text", text: "Use " }, { type: "text", text: "bold", marks: [{ type: "bold" }] }, { type: "text", text: " and " },
        { type: "text", text: "a link", marks: [{ type: "link", attrs: { href: "https://x.test/" } }] }, { type: "hard_break" }, { type: "text", text: "then" }] },
      { type: "bullet_list", content: [
        { type: "list_item", content: [{ type: "paragraph", content: [{ type: "text", text: "one" }] }] },
        { type: "list_item", content: [{ type: "paragraph", content: [{ type: "text", text: "two" }] },
          { type: "bullet_list", content: [{ type: "list_item", content: [{ type: "paragraph", content: [{ type: "text", text: "nested" }] }] }] }] }] },
      { type: "code_block", attrs: { language: "" }, content: [{ type: "text", text: "let x = 1" }] },
      { type: "paragraph" },
    ]));
    // What it makes is a valid document.
    schema.top.checkContent({ content: blocks });
  });

  test("Markdown paste", () => {
    const md = "# Title\n\nSome **bold** and `code`.\n\n- [ ] todo\n- [x] done\n\n1. one\n   - inner\n2. two\n\n> quoted\n\n```go\nx := 1\n```\n---";
    assert.ok(looksLikeMarkdown(md));
    assert.ok(!looksLikeMarkdown("just a sentence"));
    const blocks = fromMarkdown(md);
    assert.equal(blocks.map((b) => b.type.name).join(), "heading,paragraph,task_list,ordered_list,quote,code_block,divider");
    assert.equal(blocks[2].child(1).attrs.checked, true);
    assert.equal(blocks[3].child(0).childCount, 2, "nested list in the first item");
    assert.equal(blocks[5].attrs.language, "go");
    assert.equal(json(inlineMarkdown("[a](javascript:x) ~~s~~")), JSON.stringify([{ type: "text", text: "[a](javascript:x) " }, { type: "text", text: "s", marks: [{ type: "strike" }] }]));
  });

  test("input rules", () => {
    assert.equal(JSON.stringify(typeWithRules("## Title").doc.toJSON()), JSON.stringify({ type: "doc", content: [{ type: "heading", attrs: { level: 2 }, content: [{ type: "text", text: "Title" }] }] }));
    assert.equal(typeWithRules("- a").doc.firstChild.type.name, "bullet_list");
    const ordered = typeWithRules("3. a").doc.firstChild;
    assert.equal(ordered.type.name + ordered.attrs.start, "ordered_list3");
    assert.equal(typeWithRules("[x] a").doc.firstChild.firstChild.attrs.checked, true);
    assert.equal(typeWithRules("> a").doc.firstChild.type.name, "quote");
    assert.equal(typeWithRules("```").doc.firstChild.type.name, "code_block");
    assert.equal(typeWithRules("---").doc.content.content.map((n) => n.type.name).join(), "divider,paragraph");
    assert.equal(JSON.stringify(typeWithRules("a `x` b").doc.firstChild.toJSON().content), JSON.stringify([
      { type: "text", text: "a " }, { type: "text", text: "x", marks: [{ type: "code" }] }, { type: "text", text: " b" }]));
    assert.equal(typeWithRules("hi :tada:").doc.firstChild.lastChild.type.name, "emoji");
    // Not in the middle of a paragraph.
    assert.equal(typeWithRules("a - b").doc.firstChild.type.name, "paragraph");
  });

  test("tables: Tab moves on and adds rows; rows and columns come and go", () => {
    let state = new EditorState(schema.nodeFromJSON({ type: "doc", content: [createTable(2, 2).toJSON()] }), new Selection(4));
    const run = (cmd) => assert.ok(cmd(state, (tr) => (state = state.apply(tr))));
    const shape = () => state.doc.firstChild.content.content.map((r) => r.childCount).join("x");
    run(goToCell(1));
    run(goToCell(1));
    run(goToCell(1));
    run(goToCell(1)); // past the last cell: a new row
    assert.equal(shape(), "2x2x2");
    run(addColumn(1));
    assert.equal(shape(), "3x3x3");
    run(deleteRow);
    assert.equal(shape(), "3x3");
    run(deleteColumn);
    assert.equal(shape(), "2x2");
    run(addRow(-1));
    assert.equal(shape(), "2x2x2");
    state.doc.check?.();
  });
});
