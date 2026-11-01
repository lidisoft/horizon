// Editing commands: functions of (state, dispatch) that return whether they
// apply and, given dispatch, dispatch a transaction. They build steps only
// (state.js), so every edit can be undone and sent as steps. List lifting
// and sinking follow ProseMirror's schema-list and transform modules.

import { Fragment, Slice } from "horizon/editor/model.js";
import { ReplaceAroundStep, ReplaceStep } from "horizon/editor/transform.js";
import { Selection } from "horizon/editor/state.js";

const ITEM_TYPES = new Set(["list_item", "task_item"]);
const LIST_TYPES = new Set(["bullet_list", "ordered_list", "task_list"]);
const itemTypeOf = (listName) => (listName === "task_list" ? "task_item" : "list_item");

/** The depth of the nearest list item around $pos, or -1. */
function itemDepth($pos) {
  for (let d = $pos.depth; d > 0; d--) if (ITEM_TYPES.has($pos.node(d).type.name)) return d;
  return -1;
}

// --- Text and deletion -------------------------------------------------------------------

/** Types text over the selection. */
export function insertText(text) {
  return (state, dispatch) => {
    const { selection } = state;
    const $from = state.doc.resolve(selection.from);
    if (!$from.parent.isTextblock || selection.node) return false;
    if (dispatch) {
      const tr = state.tr;
      if (!deleteRange(tr, selection.from, selection.to)) return false;
      const pos = tr.selection.from;
      tr.insertText(text, pos, pos, state.storedMarks ?? tr.doc.resolve(pos).marks());
      dispatch(tr.setMeta("typing", "insert"));
    }
    return true;
  };
}

/** Inserts a line break: a newline in code, a hard break elsewhere. */
export function insertLineBreak(state, dispatch) {
  const $from = state.doc.resolve(state.selection.from);
  if (!$from.parent.isTextblock) return false;
  if ($from.parent.type.code) return insertText("\n")(state, dispatch);
  if (dispatch) {
    const tr = state.tr;
    deleteRange(tr, state.selection.from, state.selection.to);
    const pos = tr.selection.from;
    tr.insert(pos, state.doc.type.schema.nodes.hard_break.create()).setSelection(new Selection(pos + 1));
    dispatch(tr);
  }
  return true;
}

/**
 * Deletes from–to in tr: one replace when the two sides join, else the
 * covered blocks and the partial text at either end, leaving them apart.
 */
export function deleteRange(tr, from, to) {
  if (from === to) return true;
  if (tr.maybeStep(new ReplaceStep(from, to, Slice.empty))) {
    tr.setSelection(Selection.near(tr.doc, tr.mapping.map(from, -1)));
    return true;
  }
  const $from = tr.doc.resolve(from);
  const $to = tr.doc.resolve(to);
  const shared = $from.sharedDepth(to);
  // The blocks at shared + 1 fully between the two ends go whole.
  const start = $from.depth > shared ? $from.after(shared + 1) : from;
  const end = $to.depth > shared ? $to.before(shared + 1) : to;
  const steps = [];
  if (end > start) steps.push([start, end]);
  if ($from.parent.isTextblock && $from.pos < $from.end()) steps.push([$from.pos, $from.end()]);
  if ($to.parent.isTextblock && $to.pos > $to.start()) steps.push([$to.start(), $to.pos]);
  // Last first, so earlier positions stay valid.
  steps.sort((a, b) => b[0] - a[0]);
  for (const [a, b] of steps) tr.maybeStep(new ReplaceStep(a, b, Slice.empty));
  tr.setSelection(Selection.near(tr.doc, tr.mapping.map(from, -1)));
  return true;
}

/** Deletes the selection, if any. */
export function deleteSelection(state, dispatch) {
  if (state.selection.empty) return false;
  if (dispatch) {
    const tr = state.tr;
    const { from, to } = state.selection;
    deleteRange(tr, from, to);
    dispatch(tr);
  }
  return true;
}

/** The length of the character before (or after) an offset, surrogate pairs counting 2. */
function charLength(text, offset, backward) {
  if (backward) {
    const c = text.charCodeAt(offset - 1);
    return c >= 0xdc00 && c <= 0xdfff && offset >= 2 ? 2 : 1;
  }
  const c = text.charCodeAt(offset);
  return c >= 0xd800 && c <= 0xdbff ? 2 : 1;
}

/** The text of a textblock, leaf inline nodes as U+FFFC, so offsets match positions. */
export function blockText(node) {
  let out = "";
  node.content.content.forEach((c) => (out += c.isText ? c.text : "￼"));
  return out;
}

/** Deletes a character, a word or to the line's start or end; at a block edge, joins. */
export function deleteBy(unit, backward) {
  return (state, dispatch) => {
    if (!state.selection.empty) return deleteSelection(state, dispatch);
    const $pos = state.doc.resolve(state.selection.head);
    if (!$pos.parent.isTextblock) return false;
    const offset = $pos.parentOffset;
    if (backward && offset === 0) return joinBackward(state, dispatch);
    if (!backward && offset === $pos.parent.content.size) return joinForward(state, dispatch);
    const text = blockText($pos.parent);
    let other;
    if (unit === "char") other = backward ? offset - charLength(text, offset, true) : offset + charLength(text, offset, false);
    else if (unit === "line") other = backward ? 0 : text.length;
    else other = wordEdge(text, offset, backward);
    if (dispatch) {
      const start = $pos.start();
      const [a, b] = backward ? [start + other, $pos.pos] : [$pos.pos, start + other];
      dispatch(state.tr.delete(a, b).setSelection(new Selection(a)).setMeta("typing", "delete"));
    }
    return true;
  };
}

/** The offset of the word edge before or after offset (spaces then a word). */
export function wordEdge(text, offset, backward) {
  const isWord = (ch) => /[\p{L}\p{N}_]/u.test(ch);
  let i = offset;
  if (backward) {
    while (i > 0 && !isWord(text[i - 1])) i--;
    while (i > 0 && isWord(text[i - 1])) i--;
    return i === offset ? offset - 1 : i;
  }
  while (i < text.length && !isWord(text[i])) i++;
  while (i < text.length && isWord(text[i])) i++;
  return i === offset ? offset + 1 : i;
}

/**
 * The deepest textblock at the start (first) or end of node, with pos the
 * offset of the position before it from the position before node.
 */
function edgeTextblock(node, first) {
  let pos = 0;
  for (let n = node; ;) {
    if (n.isTextblock) return { node: n, pos };
    if (n.isLeaf || !n.childCount) return null;
    if (first) {
      pos += 1;
      n = n.firstChild;
    } else {
      pos += n.content.size - n.lastChild.nodeSize + 1;
      n = n.lastChild;
    }
  }
}

/**
 * Backspace at the start of a textblock: in a list item, lift it out; a
 * heading or code block becomes a paragraph; otherwise join with the block
 * before (into its last textblock), or delete a leaf block before.
 */
export function joinBackward(state, dispatch) {
  const $pos = state.doc.resolve(state.selection.head);
  const block = $pos.parent;
  if (!block.isTextblock || $pos.parentOffset > 0) return false;
  const d = itemDepth($pos);
  if (d > 0 && $pos.index(d) === 0 && $pos.depth === d + 1) return liftListItem(state, dispatch);
  if (block.type.name !== "paragraph") return setBlockType("paragraph")(state, dispatch);
  // The nearest ancestor with something before it.
  let depth = $pos.depth;
  while (depth > 0 && $pos.index(depth - 1) === 0) depth--;
  if (depth === 0) return false;
  const parent = $pos.node(depth - 1);
  const before = parent.child($pos.index(depth - 1) - 1);
  const cut = $pos.before(depth);
  if (!dispatch) return true;
  const tr = state.tr;
  if (before.isLeaf) {
    tr.delete(cut - before.nodeSize, cut);
    dispatch(tr);
    return true;
  }
  const target = edgeTextblock(before, false);
  if (!target) return false;
  const targetEnd = cut - before.nodeSize + target.pos + 1 + target.node.content.size;
  // Joining the boundary works when the two sides are alike…
  if (depth === $pos.depth && target.node.type.compatibleContent(block.type) && before.isTextblock) {
    tr.delete(cut - 1, cut + 1);
  } else {
    // …else the block's text moves to the end of that textblock.
    const blockStart = $pos.before();
    tr.delete(blockStart, blockStart + block.nodeSize);
    if (block.content.size) tr.insert(targetEnd, block.content);
    // A list item emptied by the move goes too.
    const $left = tr.doc.resolve(tr.mapping.map(blockStart, -1));
    for (let dd = $left.depth; dd > 0; dd--) {
      if ($left.node(dd).childCount === 0) tr.maybeStep(new ReplaceStep($left.before(dd), $left.after(dd), Slice.empty));
      else break;
    }
  }
  tr.setSelection(new Selection(targetEnd));
  dispatch(tr);
  return true;
}

/** Delete at the end of a textblock: pull the next textblock in, or delete a leaf block after. */
export function joinForward(state, dispatch) {
  const $pos = state.doc.resolve(state.selection.head);
  const block = $pos.parent;
  if (!block.isTextblock || $pos.parentOffset < block.content.size) return false;
  let depth = $pos.depth;
  while (depth > 0 && $pos.indexAfter(depth - 1) === $pos.node(depth - 1).childCount) depth--;
  if (depth === 0) return false;
  const parent = $pos.node(depth - 1);
  const after = parent.child($pos.index(depth - 1) + 1);
  const cut = $pos.after(depth);
  if (!dispatch) return true;
  const tr = state.tr;
  if (after.isLeaf) {
    dispatch(tr.delete(cut, cut + after.nodeSize));
    return true;
  }
  const source = edgeTextblock(after, true);
  if (!source) return false;
  if (depth === $pos.depth && after.isTextblock && after.type.compatibleContent(block.type)) {
    tr.delete(cut - 1, cut + 1);
  } else {
    const sourceStart = cut + source.pos;
    tr.delete(sourceStart, sourceStart + source.node.nodeSize);
    if (source.node.content.size) tr.insert($pos.pos, source.node.content);
    const $left = tr.doc.resolve(tr.mapping.map(sourceStart, -1));
    for (let dd = $left.depth; dd > 0; dd--) {
      if ($left.node(dd).childCount === 0) tr.maybeStep(new ReplaceStep($left.before(dd), $left.after(dd), Slice.empty));
      else break;
    }
  }
  tr.setSelection(new Selection($pos.pos));
  dispatch(tr);
  return true;
}

// --- Splitting ---------------------------------------------------------------------------

/**
 * Splits the node at pos depth levels up (ProseMirror's split): the new
 * nodes are copies, or typesAfter[i] ({ type, attrs }) from the outermost.
 */
function split(tr, pos, depth, typesAfter = []) {
  const $pos = tr.doc.resolve(pos);
  let before = Fragment.empty;
  let after = Fragment.empty;
  for (let d = $pos.depth, e = $pos.depth - depth, i = depth - 1; d > e; d--, i--) {
    before = Fragment.from($pos.node(d).copy(before));
    const t = typesAfter[i];
    after = Fragment.from(t ? t.type.create(t.attrs ?? null, after) : $pos.node(d).copy(after));
  }
  tr.step(new ReplaceStep(pos, pos, new Slice(before.append(after), depth, depth), true));
}

/**
 * Enter: in a code block, a newline; in an empty list item, lift it out;
 * in a list item, a new item; elsewhere, a new block (a paragraph after a
 * heading).
 */
export function splitBlock(state, dispatch) {
  const $from = state.doc.resolve(state.selection.from);
  if (!$from.parent.isTextblock) return false;
  if ($from.parent.type.code) return insertText("\n")(state, dispatch);
  const schema = state.doc.type.schema;
  const d = itemDepth($from);
  if (d > 0 && $from.depth === d + 1 && $from.index(d) === 0 && !$from.parent.content.size && state.selection.empty) {
    return liftListItem(state, dispatch);
  }
  if (!dispatch) return true;
  const tr = state.tr;
  deleteRange(tr, state.selection.from, state.selection.to);
  const pos = tr.selection.from;
  const $pos = tr.doc.resolve(pos);
  const atEnd = $pos.parentOffset === $pos.parent.content.size;
  const textType = atEnd || $pos.parent.type.name !== "heading" ? schema.nodes.paragraph : null;
  const paragraphAfter = $pos.parent.type.name === "heading" && atEnd ? { type: schema.nodes.paragraph } : null;
  const di = itemDepth($pos);
  if (di > 0 && $pos.depth === di + 1 && $pos.index(di) === 0) {
    const item = $pos.node(di);
    const itemAfter = { type: item.type, attrs: item.type.name === "task_item" ? { checked: false } : null };
    split(tr, pos, 2, [itemAfter, paragraphAfter ?? (textType ? { type: textType } : null)]);
  } else {
    split(tr, pos, 1, [paragraphAfter]);
  }
  // Marks carry on into the new block.
  dispatch(tr.setStoredMarks(state.storedMarks ?? $pos.marks()));
  return true;
}

// --- Marks -------------------------------------------------------------------------------

/** Whether the selection (or the stored marks) has the mark everywhere. */
export function markActive(state, type) {
  const { from, to, empty } = state.selection;
  if (empty) return !!(state.storedMarks ?? state.doc.resolve(from).marks()).some((m) => m.type === type);
  let all = true;
  let any = false;
  state.doc.nodesBetween(from, to, (node) => {
    if (node.isInline) {
      any = true;
      if (!node.marks.some((m) => m.type === type)) all = false;
    }
  });
  return any && all;
}

/**
 * Toggles a mark: off where the whole selection has it, else on where it
 * lacks it. Steps cover exact ranges, so undoing restores what was there.
 */
export function toggleMark(type, attrs = null) {
  return (state, dispatch) => {
    const { from, to, empty } = state.selection;
    const mark = type.create(attrs);
    if (empty) {
      if (dispatch) {
        const current = state.storedMarks ?? state.doc.resolve(from).marks();
        dispatch(state.tr.setStoredMarks(markActive(state, type) ? current.filter((m) => m.type !== type) : mark.addToSet(current)));
      }
      return true;
    }
    if (!dispatch) return true;
    const tr = state.tr;
    const remove = markActive(state, type);
    const ranges = [];
    state.doc.nodesBetween(from, to, (node, pos, parent) => {
      if (!node.isInline || !parent.type.allowsMark(type)) return;
      const has = node.marks.find((m) => m.type === type);
      if (remove ? !has : has && has.eq(mark)) return;
      const a = Math.max(pos, from);
      const b = Math.min(pos + node.nodeSize, to);
      const last = ranges.at(-1);
      if (last && last.to === a && (!remove || last.mark.eq(has))) last.to = b;
      else ranges.push({ from: a, to: b, mark: remove ? has : mark });
    });
    for (const r of ranges) {
      if (remove) tr.removeMark(r.from, r.to, r.mark);
      else tr.addMark(r.from, r.to, r.mark);
    }
    dispatch(tr);
    return true;
  };
}

// --- Block types -------------------------------------------------------------------------

/** The textblocks touched by the selection: [{ node, pos }]. */
function selectedTextblocks(state) {
  const out = [];
  const { from, to } = state.selection;
  state.doc.nodesBetween(from, Math.max(to, from + 1), (node, pos) => {
    if (node.isTextblock) {
      out.push({ node, pos });
      return false;
    }
    return true;
  });
  if (!out.length) {
    const $from = state.doc.resolve(from);
    if ($from.parent.isTextblock) out.push({ node: $from.parent, pos: $from.before() });
  }
  return out;
}

/**
 * Turns the selected textblocks into type (with attrs): headings change
 * level in place; a code block loses marks and inline nodes.
 */
export function setBlockType(typeName, attrs = null) {
  return (state, dispatch) => {
    const type = state.doc.type.schema.nodes[typeName];
    const blocks = selectedTextblocks(state);
    if (!blocks.length) return false;
    if (!dispatch) return true;
    const tr = state.tr;
    for (const { node, pos: oldPos } of blocks.reverse()) {
      const pos = tr.mapping.map(oldPos);
      const cur = tr.doc.nodeAt(pos);
      if (cur.type === type) {
        if (attrs && JSON.stringify({ ...cur.attrs, ...attrs }) !== JSON.stringify(cur.attrs)) tr.setNodeAttrs(pos, attrs);
        continue;
      }
      if (type.code && cur.content.size) {
        const text = node.textContent();
        tr.replace(pos + 1, pos + 1 + cur.content.size, text ? new Slice(Fragment.from(state.doc.type.schema.text(text)), 0, 0) : Slice.empty);
      }
      const now = tr.doc.nodeAt(pos);
      tr.replaceAround(pos, pos + now.nodeSize, pos + 1, pos + now.nodeSize - 1, new Slice(Fragment.from(type.create(attrs)), 0, 0), 1, true);
    }
    dispatch(tr);
    return true;
  };
}

/** Whether every selected textblock is of type (with attrs). */
export function blockActive(state, typeName, attrs = null) {
  const blocks = selectedTextblocks(state);
  return blocks.length > 0 && blocks.every(({ node }) => node.type.name === typeName && (!attrs || Object.entries(attrs).every(([k, v]) => node.attrs?.[k] === v)));
}

// --- Wrapping and lifting ----------------------------------------------------------------

/**
 * The blocks the selection spans, as siblings: their parent's depth and
 * the positions before the first and after the last.
 */
function blockRange(state) {
  const $from = state.doc.resolve(state.selection.from);
  const $to = state.doc.resolve(state.selection.to);
  let depth = Math.min($from.depth, $to.depth) - 1;
  while (depth > 0 && ($from.before(depth + 1) > $to.pos || $from.node(depth) !== $to.node(depth))) depth--;
  depth = Math.max(0, Math.min(depth, $from.sharedDepth($to.pos)));
  // Never split a list from its items: wrap whole items.
  while (depth > 0 && ITEM_TYPES.has($from.node(depth + 1).type.name)) depth--;
  return { depth, start: $from.before(depth + 1), end: $to.after(depth + 1), $from, $to, parent: $from.node(depth) };
}

/** Wraps the selected blocks in a quote or a callout, or unwraps them. */
export function toggleWrap(typeName, attrs = null) {
  return (state, dispatch) => {
    const type = state.doc.type.schema.nodes[typeName];
    const $from = state.doc.resolve(state.selection.from);
    for (let d = $from.depth; d > 0; d--) {
      if ($from.node(d).type === type) {
        if (dispatch) {
          const tr = state.tr;
          const before = $from.before(d);
          const node = $from.node(d);
          tr.replaceAround(before, before + node.nodeSize, before + 1, before + node.nodeSize - 1, Slice.empty, 0, true);
          dispatch(tr);
        }
        return true;
      }
    }
    const r = blockRange(state);
    if (dispatch) {
      dispatch(state.tr.replaceAround(r.start, r.end, r.start, r.end, new Slice(Fragment.from(type.create(attrs)), 0, 0), 1, true));
    }
    return true;
  };
}

/**
 * Turns the selected blocks into a list of listType: changes the type of a
 * list they are in, lifts them out of one of that type, or wraps each in an
 * item of a new list.
 */
export function toggleList(listName) {
  return (state, dispatch) => {
    const schema = state.doc.type.schema;
    const listType = schema.nodes[listName];
    const itemType = schema.nodes[itemTypeOf(listName)];
    const $from = state.doc.resolve(state.selection.from);
    const d = itemDepth($from);
    if (d > 0) {
      const list = $from.node(d - 1);
      if (list.type === listType) return liftListItem(state, dispatch);
      if (!dispatch) return true;
      // Another kind of list: retype it; when its items change type too
      // (task lists), rebuild it in one step, as both must change at once.
      const tr = state.tr;
      const listPos = $from.before(d - 1);
      const end = listPos + list.nodeSize;
      if (itemType !== list.firstChild.type) {
        const items = [];
        for (let i = 0; i < list.childCount; i++) items.push(itemType.create(null, list.child(i).content));
        // Same sizes: the selection stays where it was.
        tr.replace(listPos, end, new Slice(Fragment.from(listType.create(null, items)), 0, 0));
        tr.setSelection(new Selection(state.selection.anchor, state.selection.head));
      } else {
        tr.replaceAround(listPos, end, listPos + 1, end - 1, new Slice(Fragment.from(listType.create()), 0, 0), 1, true);
        tr.setSelection(state.selection.map(tr.mapping));
      }
      dispatch(tr);
      return true;
    }
    const r = blockRange(state);
    if (!dispatch) return true;
    const tr = state.tr;
    // Each block in its own item; then the lists join into one.
    const starts = [];
    for (let i = r.$from.index(r.depth), pos = r.start; pos < r.end; i++) {
      starts.push(pos);
      pos += r.parent.child(i).nodeSize;
    }
    for (const pos of starts.reverse()) {
      const node = tr.doc.nodeAt(pos);
      tr.replaceAround(pos, pos + node.nodeSize, pos, pos + node.nodeSize, new Slice(Fragment.from(listType.create(null, itemType.create())), 0, 0), 2, true);
    }
    // Join the new lists: their boundaries sit after each but the last.
    let pos = r.start;
    const joins = [];
    for (let i = 0; i < starts.length - 1; i++) {
      pos += tr.doc.nodeAt(pos).nodeSize;
      joins.push(pos);
    }
    for (const j of joins.reverse()) tr.step(new ReplaceStep(j - 1, j + 1, Slice.empty, true));
    dispatch(tr);
    return true;
  };
}

/**
 * ProseMirror's lift: moves the blocks of a range (a parent at depth and
 * the positions of the first and last) out to depth target, splitting the
 * ancestors in between around them.
 */
function lift(tr, $from, $to, depth, target) {
  const gapStart = $from.before(depth + 1);
  const gapEnd = $to.after(depth + 1);
  let start = gapStart;
  let end = gapEnd;
  let before = Fragment.empty;
  let openStart = 0;
  for (let d = depth, splitting = false; d > target; d--) {
    if (splitting || $from.index(d) > 0) {
      splitting = true;
      before = Fragment.from($from.node(d).copy(before));
      openStart++;
    } else {
      start--;
    }
  }
  let after = Fragment.empty;
  let openEnd = 0;
  for (let d = depth, splitting = false; d > target; d--) {
    if (splitting || $to.after(d + 1) < $to.end(d)) {
      splitting = true;
      after = Fragment.from($to.node(d).copy(after));
      openEnd++;
    } else {
      end++;
    }
  }
  tr.step(new ReplaceAroundStep(start, end, gapStart, gapEnd, new Slice(before.append(after), openStart, openEnd), before.size - openStart, true));
}

/**
 * Shift-Tab, Enter in an empty item, Backspace at an item's start: a
 * nested item moves to the outer list (the items after it become its
 * children); a top-level item leaves the list.
 */
export function liftListItem(state, dispatch) {
  const $from = state.doc.resolve(state.selection.from);
  const d = itemDepth($from);
  if (d < 0) return false;
  if (!dispatch) return true;
  const tr = state.tr;
  const listDepth = d - 1;
  const list = $from.node(listDepth);
  const index = $from.index(listDepth);
  const itemStart = $from.before(d);
  const itemEnd = $from.after(d);
  const outer = listDepth >= 2 && ITEM_TYPES.has($from.node(listDepth - 1).type.name);
  if (outer) {
    // The items after it become children of it.
    const endOfList = $from.end(listDepth);
    if (itemEnd < endOfList) {
      tr.step(new ReplaceAroundStep(itemEnd - 1, endOfList, itemEnd, endOfList,
        new Slice(Fragment.from($from.node(d).type.create(null, Fragment.from(list.copy()))), 1, 0), 1, true));
    }
    const $a = tr.doc.resolve(itemStart + 1);
    const $b = tr.doc.resolve(tr.mapping.map(itemEnd, -1) - 1);
    lift(tr, $a, $b, listDepth, listDepth - 2);
  } else {
    // Out of the list: its content takes the item's place; the list is
    // closed around it on the sides it isn't at the edge of.
    const atStart = index === 0;
    const atEnd = index === list.childCount - 1;
    const empty = list.copy(Fragment.empty);
    tr.step(new ReplaceAroundStep(itemStart - (atStart ? 1 : 0), itemEnd + (atEnd ? 1 : 0), itemStart + 1, itemEnd - 1,
      new Slice((atStart ? Fragment.empty : Fragment.from(empty)).append(atEnd ? Fragment.empty : Fragment.from(empty)),
        atStart ? 0 : 1, atEnd ? 0 : 1), atStart ? 0 : 1, true));
  }
  tr.setSelection(state.selection.map(tr.mapping));
  dispatch(tr);
  return true;
}

/** Tab: the item goes into the previous item's nested list. */
export function sinkListItem(state, dispatch) {
  const $from = state.doc.resolve(state.selection.from);
  const d = itemDepth($from);
  if (d < 0) return false;
  const listDepth = d - 1;
  const index = $from.index(listDepth);
  if (index === 0) return false;
  const list = $from.node(listDepth);
  const itemType = $from.node(d).type;
  const nodeBefore = list.child(index - 1);
  if (nodeBefore.type !== itemType) return false;
  if (dispatch) {
    const nestedBefore = nodeBefore.lastChild && nodeBefore.lastChild.type === list.type;
    const slice = new Slice(Fragment.from(itemType.create(null, Fragment.from(list.type.create(null, nestedBefore ? Fragment.from(itemType.create()) : Fragment.empty)))),
      nestedBefore ? 3 : 1, 0);
    const before = $from.before(d);
    const after = $from.after(d);
    const tr = state.tr;
    tr.step(new ReplaceAroundStep(before - (nestedBefore ? 3 : 1), after, before, after, slice, 1, true));
    tr.setSelection(state.selection.map(tr.mapping));
    dispatch(tr);
  }
  return true;
}

// --- Attributes --------------------------------------------------------------------------

/** Checks or unchecks the task item at pos. */
export function toggleTask(pos) {
  return (state, dispatch) => {
    const node = state.doc.nodeAt(pos);
    if (node?.type.name !== "task_item") return false;
    if (dispatch) dispatch(state.tr.setNodeAttrs(pos, { checked: !node.attrs.checked }).setMeta("keepSelection", true));
    return true;
  };
}

/** Selects the whole document. */
export function selectAll(state, dispatch) {
  if (dispatch) dispatch(state.tr.setSelection(new Selection(Selection.atStart(state.doc).anchor, Selection.atEnd(state.doc).anchor)));
  return true;
}

/** Inserts nodes at the selection, as blocks after the current one when they are blocks. */
export function insertNodes(nodes) {
  return (state, dispatch) => {
    const fragment = Fragment.from(nodes);
    if (!fragment.childCount) return false;
    if (!dispatch) return true;
    const tr = state.tr;
    deleteRange(tr, state.selection.from, state.selection.to);
    const pos = tr.selection.from;
    const $pos = tr.doc.resolve(pos);
    if (fragment.child(0).isInline) {
      tr.insert(pos, fragment).setSelection(new Selection(pos + fragment.size));
    } else if ($pos.parent.isTextblock && !$pos.parent.content.size && $pos.depth === 1) {
      // An empty paragraph gives way to the blocks.
      tr.replace($pos.before(), $pos.after(), new Slice(fragment, 0, 0));
      tr.setSelection(Selection.near(tr.doc, $pos.before() + fragment.size, -1));
    } else {
      const at = $pos.after(1);
      tr.insert(at, fragment);
      tr.setSelection(Selection.near(tr.doc, at + fragment.size, -1));
    }
    dispatch(tr);
    return true;
  };
}

/**
 * Inserts blocks at the selection: in place of an empty paragraph, before
 * or after the textblock the cursor is at the edge of, or between the two
 * halves of a top-level paragraph split at the cursor; elsewhere after the
 * top-level block. The cursor goes to the end of them, or with
 * atStart to their first textblock (a new table's first cell).
 */
export function insertBlocks(blocks, atStart = false) {
  return (state, dispatch) => {
    const fragment = Fragment.from(blocks);
    if (!fragment.childCount) return false;
    if (!dispatch) return true;
    const tr = state.tr;
    deleteRange(tr, state.selection.from, state.selection.to);
    const $pos = tr.doc.resolve(tr.selection.from);
    let at;
    if ($pos.parent.isTextblock && $pos.depth === 1) {
      if (!$pos.parent.content.size) {
        tr.replace($pos.before(), $pos.after(), new Slice(fragment, 0, 0));
        tr.setSelection(atStart ? Selection.near(tr.doc, $pos.before(), 1) : Selection.near(tr.doc, $pos.before() + fragment.size, -1));
        dispatch(tr);
        return true;
      }
      if ($pos.parentOffset === 0) at = $pos.before();
      else if ($pos.parentOffset === $pos.parent.content.size) at = $pos.after();
      else {
        tr.step(new ReplaceStep($pos.pos, $pos.pos, new Slice(Fragment.from([$pos.parent.copy(), $pos.parent.copy()]), 1, 1), true));
        at = $pos.pos + 1;
      }
    } else {
      at = $pos.after(1);
    }
    tr.insert(at, fragment);
    tr.setSelection(atStart ? Selection.near(tr.doc, at, 1) : Selection.near(tr.doc, at + fragment.size, -1));
    dispatch(tr);
    return true;
  };
}
