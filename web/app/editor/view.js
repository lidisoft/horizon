// The editor's view (docs/frontend.md#block-editor): renders an
// EditorState into a contenteditable element and turns what the user does
// into transactions. The model is the source of truth: beforeinput events
// are cancelled and replayed as commands, then the DOM is reconciled from
// the new document. The DOM is read back only after IME composition.

import { EMOJI, safeHref } from "horizon/editor/render.js";
import { blockText } from "horizon/editor/commands.js";
import { EditorState, Selection, findTextPos } from "horizon/editor/state.js";

const MARK_TAGS = { link: "a", bold: "strong", italic: "em", strike: "s", code: "code" };

/** The element children of a container that stand for document nodes. */
function pmChildren(container) {
  const out = [];
  for (let c = container.firstChild; c; c = c.nextSibling) if (c.pmNode) out.push(c);
  return out;
}

/** Where a block's children live: the element itself, or an inner one. */
function contentDOM(dom) {
  return dom.pmContent ?? dom;
}

export class EditorView {
  /**
   * dom: the contenteditable element. options: state, dispatch(tr),
   * context ({ ws, issueKey, people }), editable, inputHandlers (by
   * beforeinput inputType: f(view, event)), keymap ([{ match(event),
   * run(view) }]), onToggleTask(pos).
   */
  constructor(dom, { state, dispatch, context = {}, editable = true, inputHandlers = {}, keymap = [], onToggleTask = null }) {
    this.dom = dom;
    this.state = state;
    this.context = context;
    this.inputHandlers = inputHandlers;
    this.keymap = keymap;
    this.onToggleTask = onToggleTask;
    this.dispatchTransaction = dispatch ?? ((tr) => this.updateState(this.state.apply(tr)));
    this.dom.classList.add("doc", "editor");
    this.setEditable(editable);
    this.handlers = [];
    const on = (target, type, f, opts) => {
      target.addEventListener(type, f, opts);
      this.handlers.push(() => target.removeEventListener(type, f, opts));
    };
    on(dom, "beforeinput", (e) => this.onBeforeInput(e));
    on(dom, "keydown", (e) => this.onKeyDown(e));
    on(dom, "compositionstart", () => (this.composing = true));
    on(dom, "compositionend", () => this.onCompositionEnd());
    on(dom, "mousedown", (e) => this.onMouseDown(e));
    // On focus, the state's selection wins over where the browser puts the caret.
    on(dom, "focus", () => requestAnimationFrame(() => this.hasFocus() && this.writeSelection()));
    on(document, "selectionchange", () => this.onSelectionChange());
    this.render();
  }

  destroy() {
    for (const off of this.handlers) off();
  }

  setEditable(editable) {
    this.editable = editable;
    this.dom.contentEditable = editable ? "true" : "false";
    this.dom.setAttribute("role", "textbox");
    this.dom.setAttribute("aria-multiline", "true");
  }

  /** Runs a command against the current state. */
  run(command) {
    this.readSelection();
    return command(this.state, (tr) => this.dispatch(tr));
  }

  dispatch(tr) {
    this.dispatchTransaction(tr);
  }

  /** Shows a new state: the DOM follows the document, then the selection. */
  updateState(state) {
    const docChanged = state.doc !== this.state.doc;
    this.state = state;
    if (docChanged) this.render();
    if (this.hasFocus()) this.writeSelection();
  }

  focus() {
    this.dom.focus({ preventScroll: true });
    this.writeSelection();
  }

  hasFocus() {
    let root = this.dom.getRootNode();
    return root.activeElement === this.dom || this.dom.contains(root.activeElement);
  }

  // --- Rendering -----------------------------------------------------------------------

  render() {
    this.rendering = true;
    this.sync(this.dom, this.state.doc.content);
    const empty = this.state.doc.childCount === 1 && this.state.doc.firstChild.isTextblock && !this.state.doc.firstChild.content.size;
    this.dom.toggleAttribute("data-empty", empty);
    this.rendering = false;
  }

  /** Reconciles a container's element children with a fragment's nodes, reusing what it can. */
  sync(container, fragment) {
    const olds = pmChildren(container);
    const used = new Set();
    let cursor = 0;
    const wanted = [];
    for (let i = 0; i < fragment.childCount; i++) {
      const node = fragment.child(i);
      let dom = null;
      // The same node object anywhere ahead, or a node of the same kind in its place.
      for (let j = cursor; j < olds.length; j++) {
        if (olds[j].pmNode === node && !used.has(olds[j])) {
          dom = olds[j];
          cursor = j + 1;
          break;
        }
      }
      if (!dom) {
        const cand = olds[cursor];
        if (cand && !used.has(cand) && cand.pmNode.type === node.type && sameAttrs(cand.pmNode, node) && !node.isLeaf) {
          dom = cand;
          cursor++;
          this.update(dom, node);
        } else {
          dom = this.renderBlock(node);
        }
      }
      used.add(dom);
      wanted.push(dom);
    }
    for (const o of olds) if (!used.has(o)) o.remove();
    // Put them in order, last first.
    let next = null;
    for (let i = wanted.length - 1; i >= 0; i--) {
      const dom = wanted[i];
      if (dom.parentNode !== container || dom.nextSibling !== next) container.insertBefore(dom, next);
      next = dom;
    }
  }

  update(dom, node) {
    if (dom.pmNode === node) return;
    dom.pmNode = node;
    if (node.isTextblock) this.renderInline(contentDOM(dom), node);
    else this.sync(contentDOM(dom), node.content);
    if (node.type.name === "task_item") dom.dataset.checked = String(!!node.attrs.checked);
  }

  renderBlock(node) {
    const d = document;
    let dom;
    let content = null;
    switch (node.type.name) {
      case "paragraph":
        dom = d.createElement("p");
        break;
      case "heading":
        dom = d.createElement(`h${Math.min(3, node.attrs.level) + 1}`);
        break;
      case "bullet_list":
        dom = d.createElement("ul");
        break;
      case "ordered_list":
        dom = d.createElement("ol");
        dom.start = node.attrs.start;
        break;
      case "list_item":
        dom = d.createElement("li");
        break;
      case "task_list":
        dom = d.createElement("ul");
        dom.className = "tasks";
        break;
      case "task_item": {
        dom = d.createElement("li");
        dom.dataset.checked = String(!!node.attrs.checked);
        const box = d.createElement("span");
        box.className = "check";
        box.contentEditable = "false";
        box.setAttribute("role", "checkbox");
        box.setAttribute("aria-checked", String(!!node.attrs.checked));
        content = d.createElement("div");
        dom.append(box, content);
        break;
      }
      case "quote":
        dom = d.createElement("blockquote");
        break;
      case "callout":
        dom = d.createElement("div");
        dom.className = "callout";
        dom.dataset.tone = node.attrs.tone;
        break;
      case "code_block":
        dom = d.createElement("pre");
        content = d.createElement("code");
        dom.append(content);
        break;
      case "divider":
        dom = d.createElement("hr");
        dom.contentEditable = "false";
        break;
      case "image": {
        dom = d.createElement("figure");
        dom.contentEditable = "false";
        dom.className = "image";
        const img = d.createElement("img");
        const { ws, issueKey } = this.context;
        if (ws && issueKey) img.src = `/api/v1/w/${encodeURIComponent(ws)}/issues/${encodeURIComponent(issueKey)}/attachments/${node.attrs.attachment}/download`;
        img.alt = node.attrs.alt;
        dom.append(img);
        break;
      }
      case "table":
        dom = d.createElement("table");
        content = d.createElement("tbody");
        dom.append(content);
        break;
      case "table_row":
        dom = d.createElement("tr");
        break;
      case "table_cell":
        dom = d.createElement(node.attrs.header ? "th" : "td");
        break;
      default:
        dom = d.createElement("div");
    }
    dom.pmNode = node;
    if (content) dom.pmContent = content;
    if (node.isTextblock) this.renderInline(contentDOM(dom), node);
    else if (!node.isLeaf) this.sync(contentDOM(dom), node.content);
    return dom;
  }

  /** Renders a textblock's inline content: one piece per inline node. */
  renderInline(container, node) {
    container.pmTextblock = true;
    container.textContent = "";
    for (const child of node.content.content) container.append(this.renderInlineNode(child));
    // An empty block, or one ending in a line break, needs a filler to show its line.
    if (!node.content.size || node.lastChild?.type.name === "hard_break") {
      const br = document.createElement("br");
      br.className = "filler";
      container.append(br);
    }
  }

  renderInlineNode(node) {
    const d = document;
    let inner;
    switch (node.type.name) {
      case "text":
        inner = d.createTextNode(node.text);
        break;
      case "hard_break":
        inner = d.createElement("br");
        break;
      case "mention": {
        inner = d.createElement("span");
        inner.className = "mention";
        inner.textContent = `@${this.context.people?.get(node.attrs.user)?.name ?? "someone"}`;
        break;
      }
      case "issue_ref":
        inner = d.createElement("span");
        inner.className = "issue-ref";
        inner.textContent = node.attrs.key;
        break;
      case "emoji":
        inner = d.createElement("span");
        inner.className = "emoji";
        inner.textContent = EMOJI[node.attrs.name] ?? `:${node.attrs.name}:`;
        break;
      default:
        inner = d.createElement("span");
    }
    if (!node.isText) inner.contentEditable = "false";
    let outer = inner;
    for (const m of [...node.marks].reverse()) {
      const wrap = d.createElement(MARK_TAGS[m.type.name] ?? "span");
      if (m.type.name === "link") {
        wrap.href = safeHref(m.attrs.href);
        wrap.rel = "noopener noreferrer";
      }
      wrap.append(outer);
      outer = wrap;
    }
    outer.pmInline = node;
    outer.pmSize = node.nodeSize;
    return outer;
  }

  // --- Positions -------------------------------------------------------------------------

  /** The position before the node a block element stands for. */
  posBefore(dom) {
    let pos = 0;
    for (let el = dom; el !== this.dom;) {
      const container = el.parentNode;
      for (const sib of pmChildren(container)) {
        if (sib === el) break;
        pos += sib.pmNode.nodeSize;
      }
      // Up to the block owning the container.
      let owner = container;
      while (owner !== this.dom && !owner.pmNode) owner = owner.parentNode;
      if (owner === this.dom) break;
      pos += 1; // its opening token
      el = owner;
    }
    return pos;
  }

  /** The document position of a DOM point inside the editor (null outside). */
  posFromDOM(node, offset) {
    if (node !== this.dom && !this.dom.contains(node)) return null;
    // Inside a textblock's content: count the pieces before the point.
    const container = findTextblockContainer(node);
    if (container && this.dom.contains(container)) {
      const start = this.posBefore(findOwner(container)) + 1;
      if (node === container) return start + inlineSize(container, offset);
      let piece = node;
      while (piece.parentNode !== container) piece = piece.parentNode;
      const before = inlineSize(container, indexOf(container, piece));
      if (!piece.pmInline) return start + before; // the filler
      if (!piece.pmInline.isText) {
        // In or on a leaf inline node: before it, or after it past its start.
        const after = node === piece ? offset > 0 : true;
        return start + before + (after ? piece.pmSize : 0);
      }
      if (node.nodeType === Node.TEXT_NODE) return start + before + offset;
      return start + before + (offset > 0 ? piece.pmSize : 0);
    }
    // In a leaf block (a divider, an image): before or after it.
    let el = node.nodeType === Node.TEXT_NODE ? node.parentNode : node;
    for (let up = el; up && up !== this.dom; up = up.parentNode) {
      if (up.pmNode?.isLeaf) return this.posBefore(up) + (offset > 0 || up !== node ? 1 : 0);
    }
    // Between blocks: a container's child index.
    let index = offset;
    while (el !== this.dom && !isContainer(el)) {
      index = indexOf(el.parentNode, el) + 1;
      el = el.parentNode;
    }
    let pos = el === this.dom ? 0 : this.posBefore(findOwner(el)) + 1;
    const kids = [...el.childNodes];
    for (let i = 0; i < Math.min(index, kids.length); i++) if (kids[i].pmNode) pos += kids[i].pmNode.nodeSize;
    return pos;
  }

  /** The DOM point of a document position: { node, offset }. */
  domFromPos(pos) {
    const $pos = this.state.doc.resolve(pos);
    let dom = this.dom;
    for (let d = 0; d < $pos.depth; d++) {
      const kids = pmChildren(contentDOM(dom));
      const next = kids[$pos.index(d)];
      if (!next) break;
      dom = next;
    }
    const container = contentDOM(dom);
    if (!$pos.parent.isTextblock || dom === this.dom) {
      // Between blocks: before the index-th child.
      const kids = pmChildren(container);
      const at = kids[$pos.index()];
      return at ? { node: container, offset: indexOf(container, at) } : { node: container, offset: container.childNodes.length };
    }
    let offset = $pos.parentOffset;
    for (let i = 0; i < container.childNodes.length; i++) {
      const piece = container.childNodes[i];
      if (!piece.pmInline) break; // the filler
      const size = piece.pmSize;
      if (piece.pmInline.isText && offset <= size) {
        const text = innermostText(piece);
        // At a piece's end, prefer the start of the next text piece only when it is text too.
        if (offset < size || !container.childNodes[i + 1]?.pmInline?.isText) return { node: text, offset };
      } else if (!piece.pmInline.isText && offset === 0) {
        return { node: container, offset: i };
      }
      offset -= size;
      if (offset < 0) return { node: container, offset: i + 1 };
    }
    return { node: container, offset: [...container.childNodes].filter((c) => c.pmInline).length };
  }

  // --- Selection -------------------------------------------------------------------------

  /** The DOM selection as anchor and focus points, through shadow roots. */
  domSelection() {
    const sel = document.getSelection();
    if (!sel || !sel.rangeCount) return null;
    const roots = [];
    for (let r = this.dom.getRootNode(); r instanceof ShadowRoot; r = r.host.getRootNode()) roots.push(r);
    let range = null;
    if (sel.getComposedRanges) {
      try {
        range = sel.getComposedRanges({ shadowRoots: roots })[0];
      } catch {
        range = sel.getComposedRanges(...roots)[0];
      }
    } else if (roots[0]?.getSelection) {
      const inner = roots[0].getSelection();
      if (inner?.rangeCount) range = inner.getRangeAt(0);
    } else {
      range = sel.getRangeAt(0);
    }
    if (!range) return null;
    const backward = sel.direction === "backward";
    const start = { node: range.startContainer, offset: range.startOffset };
    const end = { node: range.endContainer, offset: range.endOffset };
    return backward ? { anchor: end, head: start } : { anchor: start, head: end };
  }

  /** Reads the DOM selection into the state (without a transaction). */
  readSelection() {
    if (this.rendering) return;
    const ds = this.domSelection();
    if (!ds) return;
    let anchor = this.posFromDOM(ds.anchor.node, ds.anchor.offset);
    let head = this.posFromDOM(ds.head.node, ds.head.offset);
    if (anchor == null || head == null) return;
    // A caret between blocks (the browser's, on focus) goes into the nearest textblock.
    const doc = this.state.doc;
    const snap = (pos, bias) => (doc.resolve(pos).parent.isTextblock ? pos : findTextPos(doc, pos, bias) ?? findTextPos(doc, pos, -bias) ?? pos);
    anchor = snap(anchor, anchor <= head ? 1 : -1);
    head = snap(head, head < anchor ? 1 : -1);
    const sel = new Selection(anchor, head);
    // A moved cursor forgets marks toggled for typing.
    if (!sel.eq(this.state.selection)) this.state = new EditorState(this.state.doc, sel, null);
  }

  /** Puts the DOM selection where the state's is. */
  writeSelection() {
    const { anchor, head } = this.state.selection;
    const sel = document.getSelection();
    if (!sel) return;
    try {
      const a = this.domFromPos(anchor);
      const h = this.domFromPos(head);
      this.writing = true;
      sel.setBaseAndExtent(a.node, a.offset, h.node, h.offset);
    } catch {
      // a position the DOM can't show (between blocks): leave it
    } finally {
      this.writing = false;
    }
  }

  syncSelection() {
    this.readSelection();
  }

  onSelectionChange() {
    if (this.writing || this.rendering || this.composing || !this.hasFocus()) return;
    this.readSelection();
    this.dom.dispatchEvent(new CustomEvent("hzn-selection", { bubbles: false }));
  }

  // --- Input ---------------------------------------------------------------------------

  onBeforeInput(e) {
    if (!this.editable) {
      e.preventDefault();
      return;
    }
    if (e.inputType === "insertCompositionText" || e.inputType === "insertFromComposition") return;
    e.preventDefault();
    // The input of a key a binding already handled (some browsers and
    // drivers send it despite the cancelled keydown).
    const h = this.handledKey;
    this.handledKey = null;
    if (h && performance.now() - h.time < 100 && ((h.key === "Enter" && /^insert(Paragraph|LineBreak)$/.test(e.inputType)) ||
      (e.inputType === "insertText" && (e.data === h.key || (h.key === "Tab" && e.data === "\t"))))) return;
    this.readSelection();
    const handler = this.inputHandlers[e.inputType];
    if (handler) handler(this, e);
  }

  onKeyDown(e) {
    if (this.composing || e.isComposing) return;
    for (const binding of this.keymap) {
      if (binding.match(e)) {
        this.readSelection();
        if (binding.run(this)) {
          e.preventDefault();
          this.handledKey = { key: e.key, time: performance.now() };
          return;
        }
      }
    }
  }

  onMouseDown(e) {
    const check = e.target.closest?.(".check");
    if (check && this.editable) {
      e.preventDefault();
      const item = check.parentNode;
      this.onToggleTask?.(this.posBefore(item));
    }
  }

  /** After IME composition: read the composed block's text back and replace what changed. */
  onCompositionEnd() {
    this.composing = false;
    const ds = this.domSelection();
    const pos = ds && this.posFromDOM(ds.head.node, ds.head.offset);
    if (pos == null) return this.render();
    const $pos = this.state.doc.resolve(Math.min(pos, this.state.doc.content.size));
    const block = $pos.parent;
    if (!block.isTextblock) return this.render();
    const container = findTextblockContainer(this.domFromPos($pos.start()).node);
    if (!container) return this.render();
    const domText = readInlineText(container);
    const modelText = blockText(block);
    let start = 0;
    while (start < domText.length && start < modelText.length && domText[start] === modelText[start]) start++;
    let endDom = domText.length;
    let endModel = modelText.length;
    while (endDom > start && endModel > start && domText[endDom - 1] === modelText[endModel - 1]) {
      endDom--;
      endModel--;
    }
    const inserted = domText.slice(start, endDom);
    if (!inserted && endModel === start) return this.render();
    const from = $pos.start() + start;
    const to = $pos.start() + endModel;
    const tr = this.state.tr;
    if (inserted) tr.insertText(inserted, from, to);
    else tr.delete(from, to).setSelection(new Selection(from));
    // The block re-renders from the model, dropping the browser's nodes.
    this.dispatch(tr);
  }
}

function sameAttrs(a, b) {
  return JSON.stringify(a.attrs) === JSON.stringify(b.attrs);
}

/** Whether an element holds blocks: the editor, or a block's content element. */
function isContainer(el) {
  return !!el.pmNode || el.parentNode?.pmContent === el;
}

function indexOf(parent, child) {
  return Array.prototype.indexOf.call(parent.childNodes, child);
}

/** The size in positions of the first count inline pieces of a textblock container. */
function inlineSize(container, count) {
  let size = 0;
  for (let i = 0; i < count && i < container.childNodes.length; i++) size += container.childNodes[i].pmSize ?? 0;
  return size;
}

function innermostText(el) {
  if (el.nodeType === Node.TEXT_NODE) return el;
  let n = el;
  while (n.firstChild) n = n.firstChild;
  return n;
}

/** The block element whose content a textblock container is. */
function findOwner(container) {
  let el = container;
  while (el && !el.pmNode) el = el.parentNode;
  return el;
}

function findTextblockContainer(node) {
  let el = node.nodeType === Node.TEXT_NODE ? node.parentNode : node;
  while (el && !el.pmTextblock) el = el.parentNode;
  return el;
}

/** A textblock container's text as the user sees it, leaf inline nodes as U+FFFC. */
function readInlineText(container) {
  let out = "";
  for (const piece of container.childNodes) {
    if (piece.pmInline && !piece.pmInline.isText) out += "￼";
    else if (piece.nodeType === Node.TEXT_NODE) out += piece.data;
    else if (!piece.classList?.contains("filler")) out += piece.textContent;
  }
  return out;
}
