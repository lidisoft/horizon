import { isPlain, renderDoc, safeHref } from "horizon/editor/render.js";
import { assert, describe, test } from "horizon-test/runner.js";

describe("document rendering", () => {
  test("renders blocks, marks and inline nodes, escaped", () => {
    const doc = { type: "doc", content: [
      { type: "heading", attrs: { level: 1 }, content: [{ type: "text", text: "<Title>" }] },
      { type: "paragraph", content: [
        { type: "text", text: "a", marks: [{ type: "link", attrs: { href: "https://x.test/" } }, { type: "bold" }] },
        { type: "issue_ref", attrs: { key: "HZN-1" } }, { type: "hard_break" }, { type: "emoji", attrs: { name: "tada" } }] },
      { type: "task_list", content: [{ type: "task_item", attrs: { checked: true }, content: [{ type: "paragraph", content: [{ type: "text", text: "done" }] }] }] },
    ] };
    const out = String(renderDoc(doc, { ws: "acme" }));
    assert.ok(out.includes("<h2>&lt;Title&gt;</h2>"), out);
    assert.ok(out.includes('<a href="https://x.test/" target="_blank" rel="noopener noreferrer"><strong>a</strong></a>'), out);
    assert.ok(out.includes('<a class="issue-ref" href="/w/acme/issues/HZN-1">HZN-1</a><br>🎉'), out);
    assert.ok(out.includes('data-checked="true"><input type="checkbox" disabled checked>'), out);
  });

  test("only safe links", () => {
    assert.equal(safeHref("javascript:alert(1)"), "#");
    assert.equal(safeHref("mailto:a@b.test"), "mailto:a@b.test");
  });

  test("isPlain tells paragraphs of text from richer documents", () => {
    assert.ok(isPlain({ type: "doc", content: [{ type: "paragraph", content: [{ type: "text", text: "x" }] }, { type: "paragraph" }] }));
    assert.ok(!isPlain({ type: "doc", content: [{ type: "heading", attrs: { level: 1 } }] }));
    assert.ok(!isPlain({ type: "doc", content: [{ type: "paragraph", content: [{ type: "text", text: "x", marks: [{ type: "bold" }] }] }] }));
  });
});
