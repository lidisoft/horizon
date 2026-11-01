// Editor state: a document, a selection and the marks to apply to typed
// text; transactions turn edits into steps (horizon/editor/transform.js),
// so they can be undone, mapped and, later, sent to the server.

import { Fragment, Slice } from "horizon/editor/model.js";
import { AddMarkStep, Mapping, RemoveMarkStep, ReplaceAroundStep, ReplaceStep, SetAttrsStep } from "horizon/editor/transform.js";

/**
 * A selection: anchor and head positions. Inside textblocks it is a text
 * selection; node is set when a whole leaf block (image, divider) is
 * selected, from anchor to anchor + 1.
 */
export class Selection {
  constructor(anchor, head = anchor, node = false) {
    this.anchor = anchor;
    this.head = head;
    this.node = node;
  }

  get from() {
    return Math.min(this.anchor, this.head);
  }

  get to() {
    return Math.max(this.anchor, this.head);
  }

  get empty() {
    return this.anchor === this.head;
  }

  eq(other) {
    return other && this.anchor === other.anchor && this.head === other.head && this.node === other.node;
  }

  map(mapping) {
    if (this.node) {
      const r = mapping.mapResult(this.anchor, 1);
      return r.deleted ? new Selection(r.pos) : new Selection(r.pos, r.pos + 1, true);
    }
    return new Selection(mapping.map(this.anchor, this.anchor > this.head ? -1 : 1), mapping.map(this.head, this.head > this.anchor ? -1 : 1));
  }

  /** A selection whose ends sit in textblocks (or that selects a leaf block). */
  static near(doc, pos, bias = 1) {
    pos = Math.max(0, Math.min(doc.content.size, pos));
    const found = findTextPos(doc, pos, bias) ?? findTextPos(doc, pos, -bias);
    return new Selection(found ?? 0);
  }

  static atStart(doc) {
    return Selection.near(doc, 0, 1);
  }

  static atEnd(doc) {
    return Selection.near(doc, doc.content.size, -1);
  }

  toJSON() {
    return { anchor: this.anchor, head: this.head, node: this.node || undefined };
  }
}

/** The nearest position inside a textblock, searching in direction bias. */
export function findTextPos(doc, pos, bias) {
  const $pos = doc.resolve(pos);
  if ($pos.parent.isTextblock) return pos;
  let best = null;
  doc.nodesBetween(bias > 0 ? pos : 0, bias > 0 ? doc.content.size : pos, (node, start) => {
    if (!node.isTextblock) return true;
    const candidate = bias > 0 ? start + 1 : start + 1 + node.content.size;
    if (bias > 0 ? best === null : true) best = candidate;
    return false;
  });
  return best;
}

export class EditorState {
  constructor(doc, selection = Selection.atStart(doc), storedMarks = null) {
    this.doc = doc;
    this.selection = selection;
    this.storedMarks = storedMarks; // marks typed text gets, after toggling with nothing selected
  }

  get tr() {
    return new Transaction(this);
  }

  apply(tr) {
    return new EditorState(tr.doc, tr.selection, tr.storedMarksSet ? tr.storedMarks : tr.steps.length ? null : this.storedMarks);
  }
}

/** A set of steps from one state, with the selection after them. */
export class Transaction {
  constructor(state) {
    this.before = state.doc;
    this.doc = state.doc;
    this.steps = [];
    this.docs = []; // the document before each step, for inverting
    this.mapping = new Mapping();
    this.selection = state.selection;
    this.storedMarks = state.storedMarks;
    this.storedMarksSet = false;
    this.meta = {};
  }

  get docChanged() {
    return this.steps.length > 0;
  }

  step(step) {
    const doc = step.apply(this.doc);
    this.docs.push(this.doc);
    this.steps.push(step);
    this.mapping.appendMap(step.getMap());
    this.doc = doc;
    this.selection = this.selection.map(new Mapping([step.getMap()]));
    return this;
  }

  /** step, unless it fails; reports whether it applied. */
  maybeStep(step) {
    try {
      this.step(step);
      return true;
    } catch {
      return false;
    }
  }

  setSelection(selection) {
    this.selection = selection;
    return this;
  }

  setStoredMarks(marks) {
    this.storedMarks = marks;
    this.storedMarksSet = true;
    return this;
  }

  setMeta(key, value) {
    this.meta[key] = value;
    return this;
  }

  replace(from, to, slice = Slice.empty) {
    return this.step(new ReplaceStep(from, to, slice));
  }

  delete(from, to) {
    return this.replace(from, to, Slice.empty);
  }

  insert(pos, content) {
    return this.replace(pos, pos, new Slice(Fragment.from(content), 0, 0));
  }

  /** Replaces the selection with text, with marks (default: those at the cursor). */
  insertText(text, from = this.selection.from, to = this.selection.to, marks = null) {
    const schema = this.doc.type.schema;
    const $from = this.doc.resolve(from);
    const set = marks ?? this.storedMarks ?? $from.marks();
    const allowed = set.filter((m) => $from.parent.type.allowsMark(m.type));
    this.replace(from, to, new Slice(Fragment.from(schema.text(text, allowed)), 0, 0));
    const end = from + text.length;
    this.setSelection(new Selection(end));
    return this;
  }

  addMark(from, to, mark) {
    return this.step(new AddMarkStep(from, to, mark));
  }

  removeMark(from, to, mark) {
    return this.step(new RemoveMarkStep(from, to, mark));
  }

  setNodeAttrs(pos, attrs) {
    return this.step(new SetAttrsStep(pos, attrs));
  }

  replaceAround(from, to, gapFrom, gapTo, slice, insert, structure = true) {
    return this.step(new ReplaceAroundStep(from, to, gapFrom, gapTo, slice, insert, structure));
  }
}
