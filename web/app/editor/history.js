// Undo history, local to the user (docs/frontend.md#block-editor): each
// entry holds the inverses of the user's own steps and the selection
// before them. Steps from others are mapped through, so undo never
// reverts their work; an inverse they made meaningless is dropped.

import { Selection } from "horizon/editor/state.js";
import { Mapping } from "horizon/editor/transform.js";

const GROUP_DELAY = 500; // ms: keystrokes closer than this undo together
const DEPTH = 200;

export class History {
  constructor() {
    this.done = [];
    this.undone = [];
    this.lastTime = 0;
  }

  /**
   * Records a local transaction. Typing (tr.meta.typing: "insert" or
   * "delete") joins the entry before when it is the same kind of typing
   * and recent; anything else is an entry of its own.
   */
  record(tr, selectionBefore, { time = Date.now() } = {}) {
    if (!tr.steps.length) return;
    const inverses = tr.steps.map((s, i) => s.invert(tr.docs[i]));
    const last = this.done.at(-1);
    const typing = tr.meta.typing ?? null;
    if (typing && last && last.typing === typing && time - this.lastTime < GROUP_DELAY) last.steps.push(...inverses);
    else this.done.push({ steps: inverses, selection: selectionBefore, typing });
    if (this.done.length > DEPTH) this.done.shift();
    this.undone = [];
    this.lastTime = time;
  }

  /** Closes the current entry: the next edit starts another. */
  close() {
    this.lastTime = 0;
  }

  /**
   * Maps every entry through steps made by others (a Mapping, or a list of
   * step maps), so they still apply afterwards.
   */
  mapThrough(maps) {
    const mapping = maps instanceof Mapping ? maps : new Mapping(maps);
    const mapEntry = (e) => {
      const steps = e.steps.map((s) => s.map(mapping)).filter(Boolean);
      return steps.length ? { steps, selection: e.selection.map(mapping), typing: e.typing } : null;
    };
    this.done = this.done.map(mapEntry).filter(Boolean);
    this.undone = this.undone.map(mapEntry).filter(Boolean);
  }

  /** Builds the undo (or redo) transaction for state; null when there is nothing. */
  pop(state, redo = false) {
    const from = redo ? this.undone : this.done;
    const to = redo ? this.done : this.undone;
    const entry = from.pop();
    if (!entry) return null;
    const tr = state.tr;
    // Inverses run last first: each applies to what the one after it left.
    for (let i = entry.steps.length - 1; i >= 0; i--) tr.maybeStep(entry.steps[i]);
    const inverses = tr.steps.map((s, i) => s.invert(tr.docs[i]));
    to.push({ steps: inverses, selection: state.selection, typing: null });
    const sel = entry.selection;
    const max = tr.doc.content.size;
    tr.setSelection(sel.anchor <= max && sel.head <= max ? sel : Selection.atEnd(tr.doc));
    tr.setMeta("history", true);
    this.lastTime = 0;
    return tr;
  }
}
