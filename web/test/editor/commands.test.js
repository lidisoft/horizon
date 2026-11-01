// Editing commands on documents, without a view: the steps they build and
// the documents and selections they leave.

import { joinBackward, joinForward, deleteBy, insertLineBreak, insertText, liftListItem, setBlockType, sinkListItem, splitBlock,
  toggleList, toggleMark, toggleTask, toggleWrap } from "horizon/editor/commands.js";
import { schema } from "horizon/editor/model.js";
import { EditorState, Selection } from "horizon/editor/state.js";
import { assert, describe, test } from "horizon-test/runner.js";

// Compact documents: "<p>ab|c</p>" style strings would need a parser; JSON
// builders keep the tests close to the format.
const t = (text, ...marks) => ({ type: "text", text, ...(marks.length ? { marks: marks.map((m) => ({ type: m })) } : {}) });
const n = (type, attrs, ...content) => ({ type, ...(attrs ? { attrs } : {}), ...(content.length ? { content: content.map((c) => (typeof c === "string" ? t(c) : c)) } : {}) });
const doc = (...c) => n("doc", null, ...c);
const p = (...c) => n("paragraph", null, ...c);
const h = (level, ...c) => n("heading", { level }, ...c);
const ul = (...c) => n("bullet_list", null, ...c);
const li = (...c) => n("list_item", null, ...c);

function state(json, anchor, head = anchor) {
  const d = schema.nodeFromJSON(json);
  return new EditorState(d, new Selection(anchor, head));
}

/** Runs a command; returns the state after it (and checks the undo steps). */
function run(st, command) {
  let next = null;
  const ok = command(st, (tr) => {
    next = st.apply(tr);
    // Every transaction inverts back to where it started.
    let back = tr.doc;
    for (let i = tr.steps.length - 1; i >= 0; i--) back = tr.steps[i].invert(tr.docs[i]).apply(back);
    assert.ok(back.eq(st.doc), "inverts");
    next.doc.content.content.forEach(() => {});
  });
  assert.ok(ok, "applies");
  return next;
}

const same = (st, json) => assert.equal(JSON.stringify(st.doc.toJSON()), JSON.stringify(schema.nodeFromJSON(json).toJSON()));

describe("editor commands", () => {
  test("typing replaces the selection and moves the cursor", () => {
    const s = run(state(doc(p("abcd")), 2, 4), insertText("X"));
    same(s, doc(p("aXd")));
    assert.equal(s.selection.head, 3);
  });

  test("Enter splits a paragraph; after a heading's end comes a paragraph", () => {
    same(run(state(doc(p("abcd")), 3), splitBlock), doc(p("ab"), p("cd")));
    const s = run(state(doc(h(1, "ab")), 3), splitBlock);
    same(s, doc(h(1, "ab"), p()));
    assert.equal(s.selection.head, 5);
  });

  test("Enter in a list item makes an item; in an empty one, leaves the list", () => {
    same(run(state(doc(ul(li(p("ab")))), 4), splitBlock), doc(ul(li(p("a")), li(p("b")))));
    same(run(state(doc(ul(li(p("a")), li(p()))), 8), splitBlock), doc(ul(li(p("a"))), p()));
  });

  test("Backspace joins paragraphs, lifts list items and resets headings", () => {
    const s = run(state(doc(p("ab"), p("cd")), 5), joinBackward);
    same(s, doc(p("abcd")));
    assert.equal(s.selection.head, 3);
    same(run(state(doc(ul(li(p("ab")))), 3), joinBackward), doc(p("ab")));
    same(run(state(doc(h(2, "ab")), 1), joinBackward), doc(p("ab")));
    // Into the last item of a list before.
    same(run(state(doc(ul(li(p("a"))), p("b")), 8), joinBackward), doc(ul(li(p("ab")))));
    // A divider before goes.
    same(run(state(doc(p("a"), n("divider"), p("b")), 5), joinBackward), doc(p("a"), p("b")));
  });

  test("Delete pulls the next paragraph in", () => {
    same(run(state(doc(p("ab"), p("cd")), 3), joinForward), doc(p("abcd")));
  });

  test("deleting characters, words and surrogate pairs", () => {
    same(run(state(doc(p("a😀")), 4), deleteBy("char", true)), doc(p("a")));
    same(run(state(doc(p("one two")), 8), deleteBy("word", true)), doc(p("one ")));
    same(run(state(doc(p("one two")), 1), deleteBy("word", false)), doc(p(" two")));
  });

  test("Shift-Enter inserts a hard break", () => {
    same(run(state(doc(p("ab")), 2), insertLineBreak), doc(p("a", n("hard_break"), "b")));
  });

  test("marks toggle on and off over exact ranges", () => {
    const bold = schema.marks.bold;
    let s = run(state(doc(p("abc")), 1, 3), toggleMark(bold));
    same(s, doc(p(t("ab", "bold"), "c")));
    s = run(new EditorState(s.doc, new Selection(1, 4)), toggleMark(bold));
    same(s, doc(p(t("abc", "bold"))));
    s = run(new EditorState(s.doc, new Selection(1, 4)), toggleMark(bold));
    same(s, doc(p("abc")));
    // With nothing selected, typed text takes it.
    s = run(state(doc(p("ab")), 3), toggleMark(bold));
    s = run(s, insertText("c"));
    same(s, doc(p("ab", t("c", "bold"))));
  });

  test("block types: headings change level, code blocks lose marks", () => {
    let s = run(state(doc(p("ab")), 1), setBlockType("heading", { level: 2 }));
    same(s, doc(h(2, "ab")));
    s = run(s, setBlockType("heading", { level: 3 }));
    same(s, doc(h(3, "ab")));
    same(run(state(doc(p(t("ab", "bold"))), 1), setBlockType("code_block")), doc(n("code_block", { language: "" }, "ab")));
  });

  test("lists: wrap, retype, unwrap", () => {
    let s = run(state(doc(p("a"), p("b")), 1, 4), toggleList("bullet_list"));
    same(s, doc(ul(li(p("a")), li(p("b")))));
    s = run(new EditorState(s.doc, new Selection(3)), toggleList("ordered_list"));
    same(s, doc(n("ordered_list", { start: 1 }, li(p("a")), li(p("b")))));
    s = run(new EditorState(s.doc, new Selection(3)), toggleList("task_list"));
    same(s, doc(n("task_list", null, n("task_item", { checked: false }, p("a")), n("task_item", { checked: false }, p("b")))));
    s = run(s, toggleTask(1));
    assert.equal(s.doc.child(0).child(0).attrs.checked, true);
  });

  test("Tab nests an item; Shift-Tab brings it back", () => {
    let s = run(state(doc(ul(li(p("a")), li(p("b")))), 8), sinkListItem);
    same(s, doc(ul(li(p("a"), ul(li(p("b")))))));
    s = run(s, liftListItem);
    same(s, doc(ul(li(p("a")), li(p("b")))));
    // A middle item leaving the list splits it.
    s = run(state(doc(ul(li(p("a")), li(p("b")), li(p("c")))), 8), liftListItem);
    same(s, doc(ul(li(p("a"))), p("b"), ul(li(p("c")))));
  });

  test("quotes wrap and unwrap", () => {
    let s = run(state(doc(p("a")), 1), toggleWrap("quote"));
    same(s, doc(n("quote", null, p("a"))));
    s = run(s, toggleWrap("quote"));
    same(s, doc(p("a")));
  });
});

import { History } from "horizon/editor/history.js";

describe("editor history", () => {
  test("undo and redo, typing grouped", () => {
    const h = new History();
    let s = state(doc(p()), 1);
    const apply = (command, time) => {
      command(s, (tr) => {
        h.record(tr, s.selection, { time });
        s = s.apply(tr);
      });
    };
    apply(insertText("a"), 1000);
    apply(insertText("b"), 1100); // grouped with a
    apply(splitBlock, 5000);
    same(s, doc(p("ab"), p()));
    s = s.apply(h.pop(s));
    same(s, doc(p("ab")));
    s = s.apply(h.pop(s));
    same(s, doc(p()));
    assert.equal(s.selection.head, 1);
    s = s.apply(h.pop(s, true));
    same(s, doc(p("ab")));
    assert.equal(h.pop(s, true) !== null, true);
  });
});
