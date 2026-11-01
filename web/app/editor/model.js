// Block document model (docs/realtime.md#document-model): the schema,
// immutable nodes, fragments, slices, resolved positions and the replace
// algorithm. internal/document implements the same in Go; both run the
// vectors in internal/document/testdata, so they agree. The algorithms
// follow ProseMirror's model module.

import spec from "horizon/editor/schema.js";

export class DocumentError extends Error {
  constructor(code, message) {
    super(message);
    this.name = "DocumentError";
    this.code = code;
  }
}

const fail = (code, message) => {
  throw new DocumentError(code, message);
};

// --- Schema ----------------------------------------------------------------------------

const ISSUE_KEY = /^[A-Z][A-Z0-9]{1,9}-[1-9][0-9]*$/;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function checkAttr(a, v, maxText) {
  if (a.values) {
    if (!a.values.includes(v)) throw new Error(`value ${JSON.stringify(v)} not allowed`);
    return;
  }
  switch (a.kind) {
    case "count":
      if (!Number.isInteger(v) || v < 1 || v > 1e6) throw new Error("want a whole number ≥ 1");
      return;
    case "string":
    case "uuid":
    case "issue_key":
    case "url":
      if (typeof v !== "string" || [...v].length > maxText) throw new Error(`want a string of at most ${maxText} characters`);
      if (a.kind === "uuid" && !UUID.test(v)) throw new Error("want an id");
      if (a.kind === "issue_key" && !ISSUE_KEY.test(v)) throw new Error("want an issue key");
      if (a.kind === "url" && !/^(https?:|mailto:)/i.test(v)) throw new Error("want an http, https or mailto link");
  }
}

function computeAttrs(specs, given, maxText, what) {
  const names = Object.keys(specs ?? {});
  if (!names.length) {
    if (given && Object.keys(given).length) fail("invalid_attrs", `${what} takes no attributes`);
    return null;
  }
  const out = {};
  for (const name of names.sort()) {
    const a = specs[name];
    let v = given?.[name];
    if (v === undefined || v === null) {
      if (!("default" in a)) fail("invalid_attrs", `${what} needs attribute ${name}`);
      v = a.default;
    }
    try {
      checkAttr(a, v, maxText);
    } catch (e) {
      fail("invalid_attrs", `${what} attribute ${name}: ${e.message}`);
    }
    out[name] = v;
  }
  for (const name of Object.keys(given ?? {})) if (!(name in specs)) fail("invalid_attrs", `${what} has no attribute ${name}`);
  return out;
}

export class NodeType {
  constructor(name, def, schema) {
    this.name = name;
    this.schema = schema;
    this.groups = def.group ? def.group.split(" ") : [];
    this.expr = def.content ?? "";
    this.markSet = def.marks ? new Set(def.marks) : null;
    this.code = !!def.code;
    this.attrs = def.attrs ?? {};
    this.terms = [];
  }

  get isText() {
    return this.name === "text";
  }

  get isInline() {
    return this.groups.includes("inline");
  }

  get isLeaf() {
    return this.terms.length === 0;
  }

  get isTextblock() {
    return this.terms.some((t) => t.names.some((n) => this.schema.nodes[n].isInline));
  }

  allowsMark(type) {
    return !this.markSet || this.markSet.has(type.name);
  }

  compatibleContent(other) {
    return this === other || (this.expr !== "" && this.expr === other.expr);
  }

  validContent(fragment) {
    let i = 0;
    const nodes = fragment.content;
    for (const t of this.terms) {
      let n = 0;
      while (i < nodes.length && (t.max < 0 || n < t.max) && t.names.includes(nodes[i].type.name)) {
        i++;
        n++;
      }
      if (n < t.min) return false;
    }
    if (i !== nodes.length) return false;
    return nodes.every((c) => c.marks.every((m) => this.allowsMark(m.type)));
  }

  checkContent(fragment) {
    if (!this.validContent(fragment)) fail("invalid_content", `invalid content for node ${this.name}`);
  }

  /** Makes a node, filling attribute defaults. */
  create(attrs = null, content = Fragment.empty, marks = []) {
    if (this.isText) throw new Error("use schema.text");
    return new Node(this, computeAttrs(this.attrs, attrs, this.schema.maxText, `node ${this.name}`), Fragment.from(content), marks);
  }

  /** create, checking the content. */
  createChecked(attrs, content, marks) {
    const n = this.create(attrs, content, marks);
    this.checkContent(n.content);
    return n;
  }
}

export class MarkType {
  constructor(name, def, rank) {
    this.name = name;
    this.rank = rank;
    this.attrs = def.attrs ?? {};
  }

  create(attrs = null) {
    return new Mark(this, computeAttrs(this.attrs, attrs, schema.maxText, `mark ${this.name}`));
  }
}

export class Schema {
  constructor(def) {
    this.nodes = {};
    this.marks = {};
    this.maxBytes = def.limits?.max_bytes ?? 0;
    this.maxDepth = def.limits?.max_depth ?? 0;
    this.maxText = def.limits?.max_string ?? 4000;
    def.marks.forEach((m, i) => (this.marks[m.name] = new MarkType(m.name, m, i)));
    for (const [name, n] of Object.entries(def.nodes)) this.nodes[name] = new NodeType(name, n, this);
    for (const t of Object.values(this.nodes)) t.terms = parseContent(t.expr, this);
    this.top = this.nodes[def.top];
  }

  text(text, marks = []) {
    if (!text) fail("invalid_document", "empty text node");
    return new Node(this.nodes.text, null, Fragment.empty, marks, text);
  }

  /** Reads and checks a document (or node) from its JSON form. */
  nodeFromJSON(json, depth = 0, loose = false) {
    if (this.maxDepth && depth > this.maxDepth) fail("too_large", `document nested deeper than ${this.maxDepth}`);
    const type = this.nodes[json?.type];
    if (!type) fail("invalid_document", `unknown node type ${JSON.stringify(json?.type)}`);
    const marks = this.marksFromJSON(json.marks);
    if (type.isText) {
      if (!json.text || (!loose && (json.content?.length || (json.attrs && Object.keys(json.attrs).length)))) {
        fail("invalid_document", "a text node holds non-empty text only");
      }
      return this.text(json.text, marks);
    }
    if (!loose && json.text) fail("invalid_document", `node ${type.name} holds no text`);
    if (!loose && marks.length && !type.isInline) fail("invalid_document", `node ${type.name} takes no marks`);
    const attrs = computeAttrs(type.attrs, json.attrs, this.maxText, `node ${type.name}`);
    const content = Fragment.fromArray((json.content ?? []).map((c) => this.nodeFromJSON(c, depth + 1, loose)));
    if (!loose) type.checkContent(content);
    return new Node(type, attrs, content, marks);
  }

  marksFromJSON(list) {
    let set = [];
    for (const m of list ?? []) {
      const type = this.marks[m.type];
      if (!type) fail("invalid_document", `unknown mark ${JSON.stringify(m.type)}`);
      set = new Mark(type, computeAttrs(type.attrs, m.attrs, this.maxText, `mark ${type.name}`)).addToSet(set);
    }
    return set;
  }

  sliceFromJSON(json) {
    if (!json) return Slice.empty;
    const content = Fragment.fromArray((json.content ?? []).map((c) => this.nodeFromJSON(c, 1, true)));
    const openStart = json.openStart ?? 0;
    const openEnd = json.openEnd ?? 0;
    // A slice is open at most as deep as its first and last descendants go.
    if (!(openStart >= 0 && openEnd >= 0 && openStart <= edgeDepth(content, true) && openEnd <= edgeDepth(content, false))) {
      fail("invalid_step", "slice open deeper than its content");
    }
    return new Slice(content, openStart, openEnd);
  }

  /** Parses a document's JSON text, checking its size. */
  parseDocument(text) {
    if (this.maxBytes && new TextEncoder().encode(text).length > this.maxBytes) fail("too_large", `document over ${this.maxBytes} bytes`);
    return this.nodeFromJSON(JSON.parse(text));
  }
}

function edgeDepth(fragment, first) {
  let d = 0;
  for (let f = fragment; f.childCount;) {
    const n = first ? f.child(0) : f.child(f.childCount - 1);
    if (n.isLeaf) break;
    d++;
    f = n.content;
  }
  return d;
}

function parseContent(expr, schema) {
  return expr.split(" ").filter(Boolean).map((part) => {
    const m = /^([a-z_]+)([?*+]?)$/.exec(part);
    if (!m) throw new Error(`bad content term ${part}`);
    const t = { names: [], min: 1, max: 1 };
    if (m[2] === "?") t.min = 0;
    if (m[2] === "*") [t.min, t.max] = [0, -1];
    if (m[2] === "+") t.max = -1;
    if (schema.nodes[m[1]]) t.names = [m[1]];
    else t.names = Object.values(schema.nodes).filter((n) => n.groups.includes(m[1])).map((n) => n.name);
    if (!t.names.length) throw new Error(`unknown content ${m[1]}`);
    return t;
  });
}

// --- Marks -----------------------------------------------------------------------------

const sameAttrs = (a, b) => JSON.stringify(sorted(a)) === JSON.stringify(sorted(b));
const sorted = (o) => (o ? Object.fromEntries(Object.entries(o).sort(([x], [y]) => (x < y ? -1 : 1))) : null);

export class Mark {
  constructor(type, attrs) {
    this.type = type;
    this.attrs = attrs;
  }

  eq(other) {
    return this === other || (this.type === other.type && sameAttrs(this.attrs, other.attrs));
  }

  /** The set with this mark in it (replacing one of its type), ordered by rank. */
  addToSet(set) {
    const out = [];
    let placed = false;
    for (const m of set) {
      if (m.type === this.type) {
        if (m.eq(this)) return set;
        continue;
      }
      if (!placed && this.type.rank < m.type.rank) {
        out.push(this);
        placed = true;
      }
      out.push(m);
    }
    if (!placed) out.push(this);
    return out;
  }

  removeFromSet(set) {
    return set.filter((m) => !m.eq(this));
  }

  isInSet(set) {
    return set.some((m) => m.eq(this));
  }

  toJSON() {
    return this.attrs ? { type: this.type.name, attrs: this.attrs } : { type: this.type.name };
  }
}

export const sameMarkSet = (a, b) => a.length === b.length && a.every((m, i) => m.eq(b[i]));

// --- Nodes -----------------------------------------------------------------------------

export class Node {
  constructor(type, attrs, content, marks = [], text = "") {
    this.type = type;
    this.attrs = attrs;
    this.content = content ?? Fragment.empty;
    this.marks = marks;
    this.text = text;
  }

  get isText() {
    return this.type.isText;
  }

  get isInline() {
    return this.type.isInline;
  }

  get isLeaf() {
    return this.type.isLeaf;
  }

  get isTextblock() {
    return this.type.isTextblock;
  }

  get nodeSize() {
    if (this.isText) return this.text.length;
    return this.isLeaf ? 1 : this.content.size + 2;
  }

  get childCount() {
    return this.content.content.length;
  }

  child(i) {
    return this.content.content[i];
  }

  maybeChild(i) {
    return this.content.content[i] ?? null;
  }

  get firstChild() {
    return this.content.content[0] ?? null;
  }

  get lastChild() {
    return this.content.content.at(-1) ?? null;
  }

  /** The node's text, blocks separated by sep. */
  textContent() {
    if (this.isText) return this.text;
    return this.content.content.map((c) => c.textContent()).join("");
  }

  copy(content = null) {
    return new Node(this.type, this.attrs, content ?? Fragment.empty, this.marks, this.text);
  }

  withText(text) {
    return new Node(this.type, this.attrs, this.content, this.marks, text);
  }

  mark(marks) {
    return new Node(this.type, this.attrs, this.content, marks, this.text);
  }

  sameMarkup(other) {
    return this.type === other.type && sameAttrs(this.attrs, other.attrs) && sameMarkSet(this.marks, other.marks);
  }

  eq(other) {
    return this === other || (this.sameMarkup(other) && this.text === other.text && this.content.eq(other.content));
  }

  cut(from, to) {
    if (this.isText) {
      if (from === 0 && to === this.text.length) return this;
      return this.withText(this.text.slice(from, to));
    }
    if (from === 0 && to === this.content.size) return this;
    return this.copy(this.content.cut(from, to));
  }

  nodeAt(pos) {
    for (let node = this; ;) {
      const { index, offset } = node.content.findIndex(pos);
      node = node.maybeChild(index);
      if (!node) return null;
      if (offset === pos || node.isText) return node;
      pos -= offset + 1;
    }
  }

  /** Calls f(node, pos, parent) for every descendant between from and to. */
  nodesBetween(from, to, f, start = 0) {
    this.content.nodesBetween(from, to, f, start, this);
  }

  slice(from, to = this.content.size) {
    if (from === to) return Slice.empty;
    const $from = this.resolve(from);
    const $to = this.resolve(to);
    const depth = $from.sharedDepth(to);
    const start = $from.start(depth);
    const node = $from.node(depth);
    return new Slice(node.content.cut($from.pos - start, $to.pos - start), $from.depth - depth, $to.depth - depth);
  }

  replace(from, to, slice) {
    return replace(this.resolve(from), this.resolve(to), slice);
  }

  resolve(pos) {
    return ResolvedPos.resolve(this, pos);
  }

  toJSON() {
    const out = { type: this.type.name };
    if (this.attrs) out.attrs = this.attrs;
    if (this.content.size || this.content.content.length) out.content = this.content.content.map((c) => c.toJSON());
    if (this.isText) out.text = this.text;
    if (this.marks.length) out.marks = this.marks.map((m) => m.toJSON());
    return out;
  }
}

// --- Fragments and slices --------------------------------------------------------------

export class Fragment {
  constructor(content, size) {
    this.content = content;
    this.size = size;
  }

  static fromArray(nodes) {
    const out = [];
    let size = 0;
    for (const c of nodes) {
      size += c.nodeSize;
      const last = out.at(-1);
      if (c.isText && last?.isText && last.sameMarkup(c)) out[out.length - 1] = last.withText(last.text + c.text);
      else out.push(c);
    }
    return new Fragment(out, size);
  }

  static from(nodes) {
    if (!nodes) return Fragment.empty;
    if (nodes instanceof Fragment) return nodes;
    if (Array.isArray(nodes)) return Fragment.fromArray(nodes);
    return Fragment.fromArray([nodes]);
  }

  get childCount() {
    return this.content.length;
  }

  child(i) {
    return this.content[i];
  }

  eq(other) {
    return this.content.length === other.content.length && this.content.every((c, i) => c.eq(other.content[i]));
  }

  append(other) {
    if (!other.size) return this;
    if (!this.size) return other;
    return Fragment.fromArray([...this.content, ...other.content]);
  }

  cut(from, to = this.size) {
    if (from === 0 && to === this.size) return this;
    const out = [];
    if (to > from) {
      for (let i = 0, pos = 0; pos < to; i++) {
        let child = this.content[i];
        const end = pos + child.nodeSize;
        if (end > from) {
          if (pos < from || end > to) {
            if (child.isText) child = child.cut(Math.max(0, from - pos), Math.min(child.text.length, to - pos));
            else child = child.cut(Math.max(0, from - pos - 1), Math.min(child.content.size, to - pos - 1));
          }
          out.push(child);
        }
        pos = end;
      }
    }
    return Fragment.fromArray(out);
  }

  replaceChild(index, node) {
    const content = this.content.slice();
    content[index] = node;
    return Fragment.fromArray(content);
  }

  findIndex(pos) {
    if (pos === 0) return { index: 0, offset: 0 };
    if (pos === this.size) return { index: this.content.length, offset: pos };
    for (let i = 0, cur = 0; ; i++) {
      const end = cur + this.content[i].nodeSize;
      if (end >= pos) return end === pos ? { index: i + 1, offset: end } : { index: i, offset: cur };
      cur = end;
    }
  }

  nodesBetween(from, to, f, nodeStart, parent) {
    for (let i = 0, pos = 0; pos < to; i++) {
      const child = this.content[i];
      const end = pos + child.nodeSize;
      if (end > from && f(child, nodeStart + pos, parent, i) !== false && child.content.size) {
        const start = pos + 1;
        child.nodesBetween(Math.max(0, from - start), Math.min(child.content.size, to - start), f, nodeStart + start);
      }
      pos = end;
    }
  }
}
Fragment.empty = new Fragment([], 0);

export class Slice {
  constructor(content, openStart, openEnd) {
    this.content = content;
    this.openStart = openStart;
    this.openEnd = openEnd;
  }

  get size() {
    return this.content.size - this.openStart - this.openEnd;
  }

  eq(other) {
    return this.openStart === other.openStart && this.openEnd === other.openEnd && this.content.eq(other.content);
  }

  insertAt(pos, fragment) {
    const content = insertInto(this.content, pos + this.openStart, fragment);
    return content && new Slice(content, this.openStart, this.openEnd);
  }

  removeBetween(from, to) {
    return new Slice(removeRange(this.content, from + this.openStart, to + this.openStart), this.openStart, this.openEnd);
  }

  toJSON() {
    if (!this.content.size) return null;
    const out = { content: this.content.content.map((c) => c.toJSON()) };
    if (this.openStart > 0) out.openStart = this.openStart;
    if (this.openEnd > 0) out.openEnd = this.openEnd;
    return out;
  }
}
Slice.empty = new Slice(Fragment.empty, 0, 0);

function insertInto(content, dist, insert) {
  const { index, offset } = content.findIndex(dist);
  const child = content.content[index];
  if (offset === dist || child?.isText) return content.cut(0, dist).append(insert).append(content.cut(dist));
  if (!child) return null;
  const inner = insertInto(child.content, dist - offset - 1, insert);
  return inner && content.replaceChild(index, child.copy(inner));
}

function removeRange(content, from, to) {
  const { index, offset } = content.findIndex(from);
  const child = content.content[index];
  const { index: indexTo, offset: offsetTo } = content.findIndex(to);
  if (offset === from || child?.isText) {
    if (offsetTo !== to && !content.child(indexTo).isText) fail("invalid_step", "removing a non-flat range");
    return content.cut(0, from).append(content.cut(to));
  }
  if (index !== indexTo) fail("invalid_step", "removing a non-flat range");
  return content.replaceChild(index, child.copy(removeRange(child.content, from - offset - 1, to - offset - 1)));
}

// --- Resolved positions ------------------------------------------------------------------

export class ResolvedPos {
  constructor(pos, path, parentOffset) {
    this.pos = pos;
    this.path = path;
    this.depth = path.length / 3 - 1;
    this.parentOffset = parentOffset;
  }

  static resolve(doc, pos) {
    if (!(pos >= 0 && pos <= doc.content.size)) fail("invalid_step", `position ${pos} out of range`);
    const path = [];
    let start = 0;
    let parentOffset = pos;
    for (let node = doc; ;) {
      const { index, offset } = node.content.findIndex(parentOffset);
      const rem = parentOffset - offset;
      path.push(node, index, start + offset);
      if (!rem) break;
      node = node.child(index);
      if (node.isText) {
        const c = node.text.charCodeAt(rem);
        if (c >= 0xdc00 && c <= 0xdfff) fail("invalid_step", `position ${pos} splits a character`);
        break;
      }
      parentOffset = rem - 1;
      start += offset + 1;
    }
    return new ResolvedPos(pos, path, parentOffset);
  }

  resolveDepth(d) {
    return d == null ? this.depth : d < 0 ? this.depth + d : d;
  }

  get parent() {
    return this.node(this.depth);
  }

  get doc() {
    return this.node(0);
  }

  node(d) {
    return this.path[this.resolveDepth(d) * 3];
  }

  index(d) {
    return this.path[this.resolveDepth(d) * 3 + 1];
  }

  indexAfter(d) {
    d = this.resolveDepth(d);
    return this.index(d) + (d === this.depth && !this.textOffset ? 0 : 1);
  }

  start(d) {
    d = this.resolveDepth(d);
    return d === 0 ? 0 : this.path[d * 3 - 1] + 1;
  }

  end(d) {
    d = this.resolveDepth(d);
    return this.start(d) + this.node(d).content.size;
  }

  before(d) {
    d = this.resolveDepth(d);
    if (!d) fail("invalid_step", "there is no position before the top-level node");
    return d === this.depth + 1 ? this.pos : this.path[d * 3 - 1];
  }

  after(d) {
    d = this.resolveDepth(d);
    if (!d) fail("invalid_step", "there is no position after the top-level node");
    return d === this.depth + 1 ? this.pos : this.path[d * 3 - 1] + this.path[d * 3].nodeSize;
  }

  get textOffset() {
    return this.pos - this.path.at(-1);
  }

  get nodeAfter() {
    const parent = this.parent;
    const index = this.index(this.depth);
    if (index === parent.childCount) return null;
    const dOff = this.pos - this.path.at(-1);
    const child = parent.child(index);
    return dOff ? child.cut(dOff, child.nodeSize) : child;
  }

  get nodeBefore() {
    const index = this.index(this.depth);
    const dOff = this.pos - this.path.at(-1);
    if (dOff) return this.parent.child(index).cut(0, dOff);
    return index === 0 ? null : this.parent.child(index - 1);
  }

  /** The marks at this position, for text typed here. */
  marks() {
    const parent = this.parent;
    const index = this.index();
    if (parent.content.size === 0) return [];
    if (this.textOffset) return parent.child(index).marks;
    let main = parent.maybeChild(index - 1);
    let other = parent.maybeChild(index);
    if (!main) [main, other] = [other, main];
    return main ? main.marks : [];
  }

  sharedDepth(pos) {
    for (let d = this.depth; d > 0; d--) if (this.start(d) <= pos && this.end(d) >= pos) return d;
    return 0;
  }
}

// --- Replace (ProseMirror's model/replace.js) ------------------------------------------

function replace($from, $to, slice) {
  if (slice.openStart > $from.depth) fail("invalid_step", "inserted content deeper than the insertion position");
  if ($from.depth - slice.openStart !== $to.depth - slice.openEnd) fail("invalid_step", "inconsistent open depths");
  return replaceOuter($from, $to, slice, 0);
}

function replaceOuter($from, $to, slice, depth) {
  const index = $from.index(depth);
  const node = $from.node(depth);
  if (index === $to.index(depth) && depth < $from.depth - slice.openStart) {
    const inner = replaceOuter($from, $to, slice, depth + 1);
    return node.copy(node.content.replaceChild(index, inner));
  }
  if (!slice.content.size) return close(node, replaceTwoWay($from, $to, depth));
  if (!slice.openStart && !slice.openEnd && $from.depth === depth && $to.depth === depth) {
    const parent = $from.parent;
    const content = parent.content;
    return close(parent, content.cut(0, $from.parentOffset).append(slice.content).append(content.cut($to.parentOffset)));
  }
  const { start, end } = prepareSliceForReplace(slice, $from);
  return close(node, replaceThreeWay($from, start, end, $to, depth));
}

function checkJoin(main, sub) {
  if (!sub.type.compatibleContent(main.type)) fail("invalid_step", `cannot join ${sub.type.name} onto ${main.type.name}`);
}

function joinable($before, $after, depth) {
  const node = $before.node(depth);
  checkJoin(node, $after.node(depth));
  return node;
}

function addNode(child, target) {
  const last = target.length - 1;
  if (last >= 0 && child.isText && child.sameMarkup(target[last])) target[last] = child.withText(target[last].text + child.text);
  else target.push(child);
}

function addRange($start, $end, depth, target) {
  const node = ($end || $start).node(depth);
  let startIndex = 0;
  const endIndex = $end ? $end.index(depth) : node.childCount;
  if ($start) {
    startIndex = $start.index(depth);
    if ($start.depth > depth) startIndex++;
    else if ($start.textOffset) {
      addNode($start.nodeAfter, target);
      startIndex++;
    }
  }
  for (let i = startIndex; i < endIndex; i++) addNode(node.child(i), target);
  if ($end && $end.depth === depth && $end.textOffset) addNode($end.nodeBefore, target);
}

function close(node, content) {
  node.type.checkContent(content);
  return node.copy(content);
}

function replaceThreeWay($from, $start, $end, $to, depth) {
  const openStart = $from.depth > depth && joinable($from, $start, depth + 1);
  const openEnd = $to.depth > depth && joinable($end, $to, depth + 1);
  const content = [];
  addRange(null, $from, depth, content);
  if (openStart && openEnd && $start.index(depth) === $end.index(depth)) {
    checkJoin(openStart, openEnd);
    addNode(close(openStart, replaceThreeWay($from, $start, $end, $to, depth + 1)), content);
  } else {
    if (openStart) addNode(close(openStart, replaceTwoWay($from, $start, depth + 1)), content);
    addRange($start, $end, depth, content);
    if (openEnd) addNode(close(openEnd, replaceTwoWay($end, $to, depth + 1)), content);
  }
  addRange($to, null, depth, content);
  return Fragment.fromArray(content);
}

function replaceTwoWay($from, $to, depth) {
  const content = [];
  addRange(null, $from, depth, content);
  if ($from.depth > depth) {
    const type = joinable($from, $to, depth + 1);
    addNode(close(type, replaceTwoWay($from, $to, depth + 1)), content);
  }
  addRange($to, null, depth, content);
  return Fragment.fromArray(content);
}

function prepareSliceForReplace(slice, $along) {
  const extra = $along.depth - slice.openStart;
  const parent = $along.node(extra);
  let node = parent.copy(slice.content);
  for (let i = extra - 1; i >= 0; i--) node = $along.node(i).copy(Fragment.from(node));
  return { start: node.resolve(slice.openStart + extra), end: node.resolve(node.content.size - slice.openEnd - extra) };
}

/** Horizon's schema (internal/document/schema.json). */
export const schema = new Schema(spec);
