// Input rules: Markdown-style shortcuts while typing (docs/frontend.md#block-editor).
// At the start of a paragraph: "# " to "### " headings, "- " or "* " a
// bulleted list, "1. " a numbered one, "[] " or "[x] " a task list, "> " a
// quote, "```" a code block, "---" a divider. Inline: `code`, **bold**,
// ~~strike~~, :emoji:. Each returns the command to run, which removes the
// trigger text and applies the change.

import { blockText, setBlockType, toggleList, toggleWrap, insertNodes } from "horizon/editor/commands.js";
import { schema } from "horizon/editor/model.js";
import { EMOJI } from "horizon/editor/render.js";
import { Selection } from "horizon/editor/state.js";

const BLOCK_RULES = [
  { re: /^(#{1,3}) $/, run: (m) => setBlockType("heading", { level: m[1].length }) },
  { re: /^[-*] $/, run: () => toggleList("bullet_list") },
  { re: /^(\d+)[.)] $/, run: (m) => withAttrs(toggleList("ordered_list"), "ordered_list", { start: Math.max(1, Number(m[1])) }) },
  { re: /^\[([ xX]?)\] $/, run: (m) => withAttrs(toggleList("task_list"), "task_item", { checked: m[1].toLowerCase() === "x" }) },
  { re: /^> $/, run: () => toggleWrap("quote") },
  { re: /^```$/, run: () => setBlockType("code_block") },
  { re: /^---$/, run: () => insertNodes([schema.nodes.divider.create(), schema.nodes.paragraph.create()]) },
];

/** Runs command, then sets attrs on the nearest ancestor of type typeName. */
function withAttrs(command, typeName, attrs) {
  return (state, dispatch) => command(state, (tr) => {
    const $pos = tr.doc.resolve(tr.selection.head);
    for (let d = $pos.depth; d > 0; d--) {
      if ($pos.node(d).type.name === typeName) {
        tr.setNodeAttrs($pos.before(d), attrs);
        break;
      }
    }
    dispatch(tr);
  });
}

/**
 * The input rule triggered by the text before the cursor, if any: a
 * command for the state after typing.
 */
export function inputRule(state) {
  const { selection } = state;
  if (!selection.empty) return null;
  const $pos = state.doc.resolve(selection.head);
  const block = $pos.parent;
  if (!block.isTextblock || block.type.code) return null;
  const before = blockText(block).slice(0, $pos.parentOffset);
  const start = $pos.start();
  if (block.type.name === "paragraph" && $pos.parentOffset === before.length) {
    for (const rule of BLOCK_RULES) {
      const m = rule.re.exec(before);
      if (!m) continue;
      // Remove the trigger, then apply the block change in the same transaction's wake.
      return chain(removeText(start, start + before.length), rule.run(m));
    }
  }
  let m;
  if ((m = /`([^`]+)`$/.exec(before))) return wrapMark(start + m.index, m[0].length, m[1], "code");
  if ((m = /\*\*([^*]+)\*\*$/.exec(before))) return wrapMark(start + m.index, m[0].length, m[1], "bold");
  if ((m = /~~([^~]+)~~$/.exec(before))) return wrapMark(start + m.index, m[0].length, m[1], "strike");
  if ((m = /(^|\s):([a-z0-9_+-]+):$/.exec(before)) && EMOJI[m[2]]) {
    const from = start + m.index + m[1].length;
    return (s, dispatch) => {
      const tr = s.tr.replace(from, $pos.pos);
      tr.insert(from, schema.nodes.emoji.create({ name: m[2] }));
      dispatch(tr.setSelection(new Selection(from + 1)));
      return true;
    };
  }
  return null;
}

function removeText(from, to) {
  return (state, dispatch) => {
    dispatch(state.tr.delete(from, to).setSelection(new Selection(from)));
    return true;
  };
}

/** Runs a then b on the state a leaves, as one transaction. */
function chain(a, b) {
  return (state, dispatch) => {
    let tr1 = null;
    a(state, (tr) => (tr1 = tr));
    if (!tr1) return false;
    const mid = state.apply(tr1);
    return b(mid, (tr2) => {
      // Fold tr2's steps into tr1.
      for (const step of tr2.steps) tr1.step(step);
      tr1.setSelection(tr2.selection);
      dispatch(tr1);
    });
  };
}

/** Replaces from–from+length with text in a mark (the typed markers go). */
function wrapMark(from, length, text, markName) {
  return (state, dispatch) => {
    const mark = schema.marks[markName].create();
    const $from = state.doc.resolve(from);
    const marks = mark.addToSet($from.marks());
    const tr = state.tr.insertText(text, from, from + length, marks);
    // Typing after it goes on unmarked.
    dispatch(tr.setStoredMarks(mark.removeFromSet(marks)));
    return true;
  };
}
