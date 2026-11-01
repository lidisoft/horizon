// Steps and position mapping (docs/realtime.md#steps): the five step
// types, their maps, inversion, mapping through other changes, and the
// rebase that carries unconfirmed local steps over remote ones.
// internal/document implements the same in Go; both run the vectors in
// internal/document/testdata. The algorithms follow ProseMirror's
// transform module.

import { DocumentError, Fragment, Slice, schema as defaultSchema } from "horizon/editor/model.js";

const fail = (message) => {
  throw new DocumentError("invalid_step", message);
};

// --- Maps --------------------------------------------------------------------------------

const DEL_BEFORE = 1;
const DEL_AFTER = 2;
const DEL_ACROSS = 4;
const DEL_SIDE = 8;
const FACTOR16 = 2 ** 16;

const makeRecover = (index, offset) => index + offset * FACTOR16;
const recoverIndex = (value) => value & 0xffff;
const recoverOffset = (value) => (value - (value & 0xffff)) / FACTOR16;

export class MapResult {
  constructor(pos, delInfo, recover) {
    this.pos = pos;
    this.delInfo = delInfo;
    this.recover = recover;
  }

  get deleted() {
    return (this.delInfo & DEL_SIDE) > 0;
  }

  get deletedBefore() {
    return (this.delInfo & (DEL_BEFORE | DEL_ACROSS)) > 0;
  }

  get deletedAfter() {
    return (this.delInfo & (DEL_AFTER | DEL_ACROSS)) > 0;
  }

  get deletedAcross() {
    return (this.delInfo & DEL_ACROSS) > 0;
  }
}

export class StepMap {
  constructor(ranges = [], inverted = false) {
    this.ranges = ranges;
    this.inverted = inverted;
  }

  invert() {
    return new StepMap(this.ranges, !this.inverted);
  }

  recover(value) {
    let diff = 0;
    const index = recoverIndex(value);
    if (!this.inverted) for (let i = 0; i < index; i++) diff += this.ranges[i * 3 + 2] - this.ranges[i * 3 + 1];
    return this.ranges[index * 3] + diff + recoverOffset(value);
  }

  mapResult(pos, assoc = 1) {
    let diff = 0;
    const oldIndex = this.inverted ? 2 : 1;
    const newIndex = this.inverted ? 1 : 2;
    for (let i = 0; i < this.ranges.length; i += 3) {
      const start = this.ranges[i] - (this.inverted ? diff : 0);
      if (start > pos) break;
      const oldSize = this.ranges[i + oldIndex];
      const newSize = this.ranges[i + newIndex];
      const end = start + oldSize;
      if (pos <= end) {
        const side = !oldSize ? assoc : pos === start ? -1 : pos === end ? 1 : assoc;
        const result = start + diff + (side < 0 ? 0 : newSize);
        const recover = pos === (assoc < 0 ? start : end) ? null : makeRecover(i / 3, pos - start);
        let del = pos === start ? DEL_AFTER : pos === end ? DEL_BEFORE : DEL_ACROSS;
        if (assoc < 0 ? pos !== start : pos !== end) del |= DEL_SIDE;
        return new MapResult(result, del, recover);
      }
      diff += newSize - oldSize;
    }
    return new MapResult(pos + diff, 0, null);
  }

  map(pos, assoc = 1) {
    return this.mapResult(pos, assoc).pos;
  }

  /** Calls f(oldStart, oldEnd, newStart, newEnd) for each changed range. */
  forEach(f) {
    const oldIndex = this.inverted ? 2 : 1;
    const newIndex = this.inverted ? 1 : 2;
    for (let i = 0, diff = 0; i < this.ranges.length; i += 3) {
      const start = this.ranges[i];
      const oldStart = start - (this.inverted ? diff : 0);
      const newStart = start + (this.inverted ? 0 : diff);
      const oldSize = this.ranges[i + oldIndex];
      const newSize = this.ranges[i + newIndex];
      f(oldStart, oldStart + oldSize, newStart, newStart + newSize);
      diff += newSize - oldSize;
    }
  }
}
StepMap.empty = new StepMap([]);

/** A sequence of maps; mirrored pairs (a step and its inverse) recover positions. */
export class Mapping {
  constructor(maps = [], mirror = null, from = 0, to = maps.length) {
    this.maps = maps;
    this.mirror = mirror;
    this.from = from;
    this.to = to;
  }

  slice(from = 0, to = this.maps.length) {
    return new Mapping(this.maps, this.mirror, from, to);
  }

  appendMap(map, mirrors) {
    this.to = this.maps.push(map);
    if (mirrors != null) this.setMirror(this.maps.length - 1, mirrors);
  }

  setMirror(n, m) {
    if (!this.mirror) this.mirror = [];
    this.mirror.push(n, m);
  }

  getMirror(n) {
    if (this.mirror) for (let i = 0; i < this.mirror.length; i++) if (this.mirror[i] === n) return this.mirror[i + (i % 2 ? -1 : 1)];
    return undefined;
  }

  mapResult(pos, assoc = 1) {
    let delInfo = 0;
    for (let i = this.from; i < this.to; i++) {
      const result = this.maps[i].mapResult(pos, assoc);
      if (result.recover != null) {
        const corr = this.getMirror(i);
        if (corr != null && corr > i && corr < this.to) {
          i = corr;
          pos = this.maps[corr].recover(result.recover);
          continue;
        }
      }
      delInfo |= result.delInfo;
      pos = result.pos;
    }
    return new MapResult(pos, delInfo, null);
  }

  map(pos, assoc = 1) {
    return this.mapResult(pos, assoc).pos;
  }
}

// --- Steps -------------------------------------------------------------------------------

function contentBetween(doc, from, to) {
  const $from = doc.resolve(from);
  let dist = to - from;
  let depth = $from.depth;
  while (dist > 0 && depth > 0 && $from.indexAfter(depth) === $from.node(depth).childCount) {
    depth--;
    dist--;
  }
  if (dist > 0) {
    let next = $from.node(depth).maybeChild($from.indexAfter(depth));
    while (dist > 0) {
      if (!next || next.isLeaf) return true;
      next = next.firstChild;
      dist--;
    }
  }
  return false;
}

export class ReplaceStep {
  constructor(from, to, slice, structure = false) {
    this.from = from;
    this.to = to;
    this.slice = slice;
    this.structure = structure;
  }

  apply(doc) {
    if (this.structure && contentBetween(doc, this.from, this.to)) fail("structure replace would overwrite content");
    return doc.replace(this.from, this.to, this.slice);
  }

  getMap() {
    return new StepMap([this.from, this.to - this.from, this.slice.size]);
  }

  invert(doc) {
    return new ReplaceStep(this.from, this.from + this.slice.size, doc.slice(this.from, this.to));
  }

  map(mapping) {
    const from = mapping.mapResult(this.from, 1);
    const to = mapping.mapResult(this.to, -1);
    if (from.deletedAcross && to.deletedAcross) return null;
    return new ReplaceStep(from.pos, Math.max(from.pos, to.pos), this.slice, this.structure);
  }

  /** Merges with a following step into one, when both are plain edits that touch. */
  merge(other) {
    if (!(other instanceof ReplaceStep) || other.structure || this.structure) return null;
    if (this.from + this.slice.size === other.from && !this.slice.openEnd && !other.slice.openStart) {
      const slice = this.slice.size + other.slice.size === 0 ? Slice.empty
        : new Slice(this.slice.content.append(other.slice.content), this.slice.openStart, other.slice.openEnd);
      return new ReplaceStep(this.from, this.to + (other.to - other.from), slice);
    }
    if (other.to === this.from && !this.slice.openStart && !other.slice.openEnd) {
      const slice = this.slice.size + other.slice.size === 0 ? Slice.empty
        : new Slice(other.slice.content.append(this.slice.content), other.slice.openStart, this.slice.openEnd);
      return new ReplaceStep(other.from, this.to, slice);
    }
    return null;
  }

  toJSON() {
    const out = { stepType: "replace", from: this.from, to: this.to };
    if (this.slice.content.size) out.slice = this.slice.toJSON();
    if (this.structure) out.structure = true;
    return out;
  }
}

export class ReplaceAroundStep {
  constructor(from, to, gapFrom, gapTo, slice, insert, structure = false) {
    Object.assign(this, { from, to, gapFrom, gapTo, slice, insert, structure });
  }

  apply(doc) {
    if (this.structure && (contentBetween(doc, this.from, this.gapFrom) || contentBetween(doc, this.gapTo, this.to))) {
      fail("structure gap-replace would overwrite content");
    }
    const gap = doc.slice(this.gapFrom, this.gapTo);
    if (gap.openStart || gap.openEnd) fail("gap is not a flat range");
    const inserted = this.slice.insertAt(this.insert, gap.content);
    if (!inserted) fail("content does not fit in gap");
    return doc.replace(this.from, this.to, inserted);
  }

  getMap() {
    return new StepMap([this.from, this.gapFrom - this.from, this.insert, this.gapTo, this.to - this.gapTo, this.slice.size - this.insert]);
  }

  invert(doc) {
    const gap = this.gapTo - this.gapFrom;
    return new ReplaceAroundStep(this.from, this.from + this.slice.size + gap, this.from + this.insert, this.from + this.insert + gap,
      doc.slice(this.from, this.to).removeBetween(this.gapFrom - this.from, this.gapTo - this.from), this.gapFrom - this.from, this.structure);
  }

  map(mapping) {
    const from = mapping.mapResult(this.from, 1);
    const to = mapping.mapResult(this.to, -1);
    const gapFrom = this.from === this.gapFrom ? from.pos : mapping.map(this.gapFrom, -1);
    const gapTo = this.to === this.gapTo ? to.pos : mapping.map(this.gapTo, 1);
    if ((from.deletedAcross && to.deletedAcross) || gapFrom < from.pos || gapTo > to.pos) return null;
    return new ReplaceAroundStep(from.pos, to.pos, gapFrom, gapTo, this.slice, this.insert, this.structure);
  }

  toJSON() {
    const out = { stepType: "replace_around", from: this.from, to: this.to, gapFrom: this.gapFrom, gapTo: this.gapTo, insert: this.insert };
    if (this.slice.content.size) out.slice = this.slice.toJSON();
    if (this.structure) out.structure = true;
    return out;
  }
}

function mapFragment(fragment, f, parent) {
  const mapped = [];
  for (let i = 0; i < fragment.childCount; i++) {
    let child = fragment.child(i);
    if (child.content.size) child = child.copy(mapFragment(child.content, f, child));
    if (child.isInline) child = f(child, parent);
    mapped.push(child);
  }
  return Fragment.fromArray(mapped);
}

function applyMark(doc, from, to, f, adding) {
  const old = doc.slice(from, to);
  const $from = doc.resolve(from);
  const parent = $from.node($from.sharedDepth(to));
  const content = mapFragment(old.content, (node, p) => (!node.isLeaf || (adding && !p.type.allowsMark(adding.type)) ? node : f(node)), parent);
  return doc.replace(from, to, new Slice(content, old.openStart, old.openEnd));
}

export class AddMarkStep {
  constructor(from, to, mark) {
    Object.assign(this, { from, to, mark });
  }

  apply(doc) {
    return applyMark(doc, this.from, this.to, (n) => n.mark(this.mark.addToSet(n.marks)), this.mark);
  }

  getMap() {
    return StepMap.empty;
  }

  invert() {
    return new RemoveMarkStep(this.from, this.to, this.mark);
  }

  map(mapping) {
    const from = mapping.mapResult(this.from, 1);
    const to = mapping.mapResult(this.to, -1);
    if ((from.deleted && to.deleted) || from.pos >= to.pos) return null;
    return new AddMarkStep(from.pos, to.pos, this.mark);
  }

  toJSON() {
    return { stepType: "add_mark", from: this.from, to: this.to, mark: this.mark.toJSON() };
  }
}

export class RemoveMarkStep {
  constructor(from, to, mark) {
    Object.assign(this, { from, to, mark });
  }

  apply(doc) {
    return applyMark(doc, this.from, this.to, (n) => n.mark(this.mark.removeFromSet(n.marks)), null);
  }

  getMap() {
    return StepMap.empty;
  }

  invert() {
    return new AddMarkStep(this.from, this.to, this.mark);
  }

  map(mapping) {
    const from = mapping.mapResult(this.from, 1);
    const to = mapping.mapResult(this.to, -1);
    if ((from.deleted && to.deleted) || from.pos >= to.pos) return null;
    return new RemoveMarkStep(from.pos, to.pos, this.mark);
  }

  toJSON() {
    return { stepType: "remove_mark", from: this.from, to: this.to, mark: this.mark.toJSON() };
  }
}

export class SetAttrsStep {
  constructor(pos, attrs) {
    this.pos = pos;
    this.attrs = attrs;
  }

  apply(doc) {
    const node = doc.nodeAt(this.pos);
    if (!node || node.isText) fail(`no node at ${this.pos}`);
    // The new node is open at its end: the old one's content joins it.
    const updated = node.type.create({ ...node.attrs, ...this.attrs }, null, node.marks);
    return doc.replace(this.pos, this.pos + 1, new Slice(Fragment.from(updated), 0, node.isLeaf ? 0 : 1));
  }

  getMap() {
    return StepMap.empty;
  }

  invert(doc) {
    const node = doc.nodeAt(this.pos);
    if (!node) fail(`no node at ${this.pos}`);
    return new SetAttrsStep(this.pos, Object.fromEntries(Object.keys(this.attrs).map((k) => [k, node.attrs?.[k] ?? null])));
  }

  map(mapping) {
    const r = mapping.mapResult(this.pos, 1);
    return r.deletedAfter ? null : new SetAttrsStep(r.pos, this.attrs);
  }

  toJSON() {
    return { stepType: "set_attrs", pos: this.pos, attrs: this.attrs };
  }
}

/** Reads a step from its JSON form. */
export function stepFromJSON(json, schema = defaultSchema) {
  const ok = (n) => Number.isInteger(n) && n >= 0;
  const from = json.from ?? 0;
  const to = json.to ?? 0;
  if (json.stepType !== "set_attrs" && !(ok(from) && ok(to) && to >= from)) fail(`bad range ${from}–${to}`);
  const mark = () => {
    if (!json.mark) fail(`${json.stepType} needs a mark`);
    return schema.marksFromJSON([json.mark])[0];
  };
  switch (json.stepType) {
    case "replace":
      return new ReplaceStep(from, to, schema.sliceFromJSON(json.slice), !!json.structure);
    case "replace_around": {
      const slice = schema.sliceFromJSON(json.slice);
      if (!(json.gapFrom >= from && json.gapTo >= json.gapFrom && to >= json.gapTo && json.insert >= 0 && json.insert <= slice.size)) fail("bad gap");
      return new ReplaceAroundStep(from, to, json.gapFrom, json.gapTo, slice, json.insert, !!json.structure);
    }
    case "add_mark":
      return new AddMarkStep(from, to, mark());
    case "remove_mark":
      return new RemoveMarkStep(from, to, mark());
    case "set_attrs":
      if (!ok(json.pos) || !json.attrs || !Object.keys(json.attrs).length) fail("set_attrs needs a position and attributes");
      return new SetAttrsStep(json.pos, json.attrs);
  }
  return fail(`unknown step type ${JSON.stringify(json.stepType)}`);
}

/**
 * Carries local steps (made on base) over remote ones (also made on base,
 * and accepted first): returns the document after both and the local steps
 * as they now apply. Each local step's inverse mirrors its rebased self, so
 * positions inside content a local step inserted survive.
 */
export function rebase(base, local, remote) {
  const docs = [base];
  for (const s of local) docs.push(s.apply(docs.at(-1)));
  let doc = docs.at(-1);
  const mapping = new Mapping();
  for (let i = local.length - 1; i >= 0; i--) {
    const inv = local[i].invert(docs[i]);
    doc = inv.apply(doc);
    mapping.appendMap(inv.getMap());
  }
  for (const s of remote) {
    doc = s.apply(doc);
    mapping.appendMap(s.getMap());
  }
  const rebased = [];
  for (let i = 0, mapFrom = local.length; i < local.length; i++) {
    const mapped = local[i].map(mapping.slice(mapFrom));
    mapFrom--;
    if (!mapped) continue;
    let next;
    try {
      next = mapped.apply(doc);
    } catch {
      continue; // it no longer fits: dropped
    }
    doc = next;
    mapping.appendMap(mapped.getMap(), mapFrom);
    rebased.push(mapped);
  }
  return { doc, steps: rebased };
}
