// <hzn-editor>: the block editor (docs/frontend.md#block-editor) for
// descriptions and comments.
//
// Attributes: placeholder, label, ws and issue-key (for images, issue
// references and uploads), readonly, autofocus, bare (no border until
// focused). Properties: value (the document's JSON; setting it replaces the
// content), state, people (a Map id → { name } for mentions). Events
// (bubbling, composed): hzn-change { doc, steps } on every change (steps as
// JSON), hzn-blur when focus leaves.
//
// Keys: ⌘B ⌘I ⌘E (code) ⌘⇧X (strike) ⌘K (link); ⌘⌥0–3 paragraph and
// headings; ⌘⇧7 ⌘⇧8 ⌘⇧9 lists; Tab / ⇧Tab in lists and tables; ⌘Z ⌘⇧Z.
// Typing: "/" opens the block menu, "@" mentions, "KEY-" references an
// issue, ":name" an emoji; Markdown shortcuts (horizon/editor/rules.js).
// Paste takes HTML, Markdown and images.

import { fromHTML, fromMarkdown, inlineOnly, looksLikeMarkdown } from "horizon/editor/clipboard.js";
import { History } from "horizon/editor/history.js";
import {
  blockActive, blockText, deleteBy, deleteSelection, insertBlocks, insertLineBreak, insertNodes, insertText, liftListItem, markActive, setBlockType,
  sinkListItem, splitBlock, toggleList, toggleMark, toggleTask, toggleWrap,
} from "horizon/editor/commands.js";
import { Fragment, Slice, schema } from "horizon/editor/model.js";
import { EMOJI, docStyles } from "horizon/editor/render.js";
import { inputRule } from "horizon/editor/rules.js";
import { EditorState, Selection } from "horizon/editor/state.js";
import { addColumn, addRow, createTable, deleteColumn, deleteRow, deleteTable, goToCell, tableContext } from "horizon/editor/tables.js";
import { EditorView } from "horizon/editor/view.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const isMac = /Mac|iPhone|iPad/.test(globalThis.navigator?.platform || "");

const styles = sheet(`
  :host { display: block; }
  .frame { position: relative; border: 1px solid var(--lds-color-border-default); border-radius: var(--lds-radius-md); background: var(--lds-color-bg-surface); }
  .frame:focus-within { border-color: var(--lds-color-accent-border); box-shadow: 0 0 0 1px var(--lds-color-accent-border); }
  :host([bare]) .frame { border-color: transparent; background: transparent; }
  :host([bare]) .frame:focus-within { border-color: var(--lds-color-border-default); box-shadow: none; }
  .toolbar { display: flex; flex-wrap: wrap; gap: 2px; padding: 2px var(--lds-space-1); border-block-end: 1px solid var(--lds-color-border-subtle); }
  :host(:not(:focus-within)) .toolbar, :host([readonly]) .toolbar { display: none; }
  .toolbar button { display: inline-flex; align-items: center; justify-content: center; min-inline-size: 1.75rem; block-size: 1.75rem;
    padding: 0 var(--lds-space-1); border: 0; border-radius: var(--lds-radius-sm); background: none; color: var(--lds-color-fg-muted);
    font: inherit; font-size: var(--lds-font-size-sm); cursor: pointer; }
  .toolbar button:hover { background: var(--lds-color-bg-subtle); color: var(--lds-color-fg-default); }
  .toolbar button[aria-pressed="true"] { background: var(--lds-color-accent-subtle); color: var(--lds-color-accent-fg); }
  .toolbar button[hidden], .toolbar .sep[hidden] { display: none; }
  .toolbar .sep { inline-size: 1px; margin: var(--lds-space-1) 2px; background: var(--lds-color-border-subtle); }
  .editor { min-block-size: var(--hzn-editor-min-height, 4.5rem); padding: var(--lds-space-2) var(--lds-space-3); outline: none;
    white-space: pre-wrap; word-wrap: break-word; line-height: 1.55; }
  .editor[data-empty]::before { content: attr(data-placeholder); position: absolute; color: var(--lds-color-fg-subtle); pointer-events: none; }
  .editor ul.tasks li { display: flex; gap: var(--lds-space-2); }
  .editor ul.tasks li > div { flex: 1; min-inline-size: 0; }
  .editor .check { flex: none; inline-size: 1rem; block-size: 1rem; margin-block-start: 0.3em; border: 1px solid var(--lds-color-border-strong);
    border-radius: var(--lds-radius-xs); cursor: pointer; display: inline-flex; align-items: center; justify-content: center; }
  .editor li[data-checked="true"] > .check { background: var(--lds-color-accent-fg); border-color: transparent; }
  .editor li[data-checked="true"] > .check::after { content: "✓"; color: var(--lds-color-bg-surface); font-size: 0.75rem; }
  .editor li[data-checked="true"] > div { color: var(--lds-color-fg-muted); text-decoration: line-through; }
  .editor figure.image { margin: 0 0 var(--lds-space-2); }
  .editor figure.image img { max-inline-size: 100%; border-radius: var(--lds-radius-md); }
  .editor a { pointer-events: none; }
  .popup { position: absolute; z-index: 10; min-inline-size: 14rem; max-inline-size: 22rem; max-block-size: 16rem; overflow: auto;
    padding: var(--lds-space-1); border: 1px solid var(--lds-color-border-default); border-radius: var(--lds-radius-md);
    background: var(--lds-color-bg-surface); box-shadow: var(--lds-shadow-md, 0 4px 16px rgb(0 0 0 / 0.15)); }
  .popup[hidden] { display: none; }
  .popup [role="option"] { display: flex; gap: var(--lds-space-2); align-items: center; padding: var(--lds-space-1) var(--lds-space-2);
    border-radius: var(--lds-radius-sm); cursor: pointer; font-size: var(--lds-font-size-sm); }
  .popup [role="option"][aria-selected="true"] { background: var(--lds-color-accent-subtle); }
  .popup .hint { color: var(--lds-color-fg-muted); margin-inline-start: auto; font-size: var(--lds-font-size-xs); }
  .popup .none { padding: var(--lds-space-1) var(--lds-space-2); color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  .popup form { display: flex; gap: var(--lds-space-1); }
  .cursors { position: absolute; inset: 0; pointer-events: none; overflow: hidden; }
  .cursors .caret { position: absolute; inline-size: 2px; background: var(--hzn-cursor); }
  .cursors .caret span { position: absolute; inset-block-end: 100%; inset-inline-start: -1px; padding: 0 var(--lds-space-1);
    border-radius: var(--lds-radius-xs) var(--lds-radius-xs) var(--lds-radius-xs) 0; background: var(--hzn-cursor);
    color: var(--lds-color-bg-surface); font-size: var(--lds-font-size-2xs, 0.65rem); line-height: 1.4; white-space: nowrap; }
  .cursors .range { position: absolute; background: var(--hzn-cursor); opacity: 0.18; }
  .cursors [data-color="0"] { --hzn-cursor: var(--lds-color-accent-fg); }
  .cursors [data-color="1"] { --hzn-cursor: var(--lds-color-success-fg); }
  .cursors [data-color="2"] { --hzn-cursor: var(--lds-color-warning-fg); }
  .cursors [data-color="3"] { --hzn-cursor: var(--lds-color-danger-fg); }
  .cursors [data-color="4"] { --hzn-cursor: var(--lds-color-neutral-fg); }
  .popup input { flex: 1; min-inline-size: 0; font: inherit; padding: var(--lds-space-1) var(--lds-space-2);
    border: 1px solid var(--lds-color-border-default); border-radius: var(--lds-radius-sm); background: var(--lds-color-bg-canvas); color: inherit; }
`);

const mod = (e) => (isMac ? e.metaKey : e.ctrlKey);
const key = (spec, run) => {
  // "Mod-Shift-x": modifiers then the key (KeyboardEvent.key) or a code.
  const parts = spec.split("-");
  const k = parts.pop();
  const want = { mod: parts.includes("Mod"), shift: parts.includes("Shift"), alt: parts.includes("Alt") };
  return {
    match: (e) => mod(e) === want.mod && e.shiftKey === want.shift && e.altKey === want.alt && (e.key.toLowerCase() === k.toLowerCase() || e.code === k),
    run,
  };
};

// What the text before the cursor asks for.
const TRIGGERS = [
  { kind: "slash", re: /(?:^|\s)\/([\p{L}\d ]{0,20})$/u },
  { kind: "mention", re: /(?:^|\s)@([\p{L}\d._-]{0,30})$/u },
  { kind: "issue", re: /(?:^|[\s(])([A-Z][A-Z0-9]{1,9}-\d*)$/ },
  { kind: "emoji", re: /(?:^|\s):([a-z0-9_+-]{2,20})$/ },
];

class HznEditor extends HTMLElement {
  static observedAttributes = ["readonly", "placeholder"];

  constructor() {
    super();
    this.history = new History();
    this.pending = null; // a value set before connecting
    this.people = new Map();
    this.cursors = new Map(); // others' selections: conn → { user, name, anchor, head }
  }

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open", delegatesFocus: true });
    root.adoptedStyleSheets = [docStyles, styles];
    const { t } = this.context.i18n;
    const button = (cmd, label, text, hidden = false) =>
      html`<button type="button" tabindex="-1" data-cmd="${cmd}" title="${label}" aria-label="${label}" ${hidden ? html`hidden` : ""}>${text}</button>`;
    root.innerHTML = String(html`
      <div class="frame">
        <div class="toolbar" role="toolbar" aria-label="${t("editor.toolbar")}">
          ${button("bold", t("editor.bold"), html`<b>B</b>`)}${button("italic", t("editor.italic"), html`<i>I</i>`)}
          ${button("strike", t("editor.strike"), html`<s>S</s>`)}${button("code", t("editor.code"), "</>")}${button("link", t("editor.link"), "🔗")}
          <span class="sep"></span>
          ${button("h1", t("editor.heading", { level: 1 }), "H1")}${button("h2", t("editor.heading", { level: 2 }), "H2")}
          ${button("bullet", t("editor.bulletList"), "•")}${button("ordered", t("editor.orderedList"), "1.")}${button("task", t("editor.taskList"), "☐")}
          ${button("quote", t("editor.quote"), "❝")}${button("codeblock", t("editor.codeBlock"), "{ }")}
          <span class="sep" data-table-tools hidden></span>
          ${button("addRow", t("editor.addRow"), "+▭", true)}${button("addColumn", t("editor.addColumn"), "+▯", true)}
          ${button("deleteRow", t("editor.deleteRow"), "−▭", true)}${button("deleteColumn", t("editor.deleteColumn"), "−▯", true)}
          ${button("deleteTable", t("editor.deleteTable"), "✕▦", true)}
        </div>
        <div class="editor" spellcheck="true"></div>
        <div class="cursors" aria-hidden="true"></div>
        <div class="popup" role="listbox" hidden></div>
        <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" multiple hidden data-file>
      </div>`);
    const dom = root.querySelector(".editor");
    dom.setAttribute("aria-label", this.getAttribute("label") ?? this.getAttribute("placeholder") ?? "");
    dom.dataset.placeholder = this.getAttribute("placeholder") ?? "";
    this.popup = root.querySelector(".popup");
    const doc = this.pending ?? schema.nodeFromJSON({ type: "doc", content: [{ type: "paragraph" }] });
    const host = this;
    this.view = new EditorView(dom, {
      state: new EditorState(doc, Selection.atStart(doc)),
      dispatch: (tr) => this.dispatch(tr),
      context: {
        ws: this.getAttribute("ws"),
        issueKey: this.getAttribute("issue-key"),
        get people() {
          return host.people;
        },
      },
      editable: !this.hasAttribute("readonly"),
      inputHandlers: this.inputHandlers(),
      keymap: this.keymap(),
      onToggleTask: (pos) => this.view.run(toggleTask(pos)),
    });
    root.querySelector(".toolbar").addEventListener("mousedown", (e) => {
      const b = e.target.closest("[data-cmd]");
      if (!b) return;
      e.preventDefault(); // keep the selection in the editor
      this.command(b.dataset.cmd);
    });
    this.popup.addEventListener("mousedown", (e) => {
      const option = e.target.closest('[role="option"]');
      if (!option) return;
      e.preventDefault();
      this.choose(Number(option.dataset.index));
    });
    root.querySelector("[data-file]").addEventListener("change", (e) => {
      this.upload([...e.target.files]);
      e.target.value = "";
    });
    dom.addEventListener("hzn-selection", () => {
      this.refreshToolbar();
      this.suggest();
      this.shareCursor();
    });
    this.addEventListener("focusout", (e) => {
      if (!this.contains(e.relatedTarget) && !root.contains(e.relatedTarget)) {
        this.history.close();
        this.closePopup();
        this.dispatchEvent(new CustomEvent("hzn-blur", { bubbles: true, composed: true }));
      }
    });
    if (this.hasAttribute("autofocus")) requestAnimationFrame(() => this.focus());
  }

  disconnectedCallback() {
    this.collab?.stop();
    this.collab = null;
  }

  /**
   * Edits collaboratively through collab (horizon/editor/collab.js): read
   * only until it has joined; local steps go to it, remote ones come back
   * through receive.
   */
  attachCollab(collab) {
    this.collab?.stop();
    this.collab = collab;
    this.setOffline(true);
    collab.start();
  }

  /** The document as the server has it (joining from scratch). */
  reset(json) {
    this.value = json;
  }

  /** Others' steps: the document after them and the mapping for the selection and history. */
  receive(doc, mapping) {
    const state = this.view.state;
    let selection = state.selection.map(mapping);
    try {
      if (!doc.resolve(selection.head).parent.isTextblock) selection = Selection.near(doc, selection.head);
    } catch {
      selection = Selection.atEnd(doc);
    }
    this.history.mapThrough(mapping);
    this.mapCursors(mapping);
    this.view.updateState(new EditorState(doc, selection, state.storedMarks));
    this.drawCursors();
    this.refreshToolbar();
    this.dispatchEvent(new CustomEvent("hzn-change", { bubbles: true, composed: true, detail: { doc: doc.toJSON(), steps: [], remote: true } }));
  }

  /** Sets (or, with null, removes) someone else's selection, in local positions. */
  remoteCursor(conn, cursor) {
    if (cursor) this.cursors.set(conn, { ...cursor, at: Date.now() });
    else this.cursors.delete(conn);
    this.drawCursors();
  }

  mapCursors(mapping) {
    for (const c of this.cursors.values()) {
      c.anchor = mapping.map(c.anchor);
      c.head = mapping.map(c.head);
    }
  }

  /** Tells the others where this user's selection is, while editing. */
  shareCursor() {
    if (!this.collab || !this.view?.hasFocus()) return;
    const { anchor, head } = this.view.state.selection;
    this.collab.cursor(anchor, head);
  }

  /** Draws the others' carets and selections over the text. */
  drawCursors() {
    const layer = this.shadowRoot?.querySelector(".cursors");
    if (!layer) return;
    layer.textContent = "";
    const frame = layer.getBoundingClientRect();
    const max = this.view.state.doc.content.size;
    for (const [conn, c] of this.cursors) {
      if (Date.now() - c.at > 60_000 || c.head > max || c.anchor > max) continue;
      const color = String([...String(c.user)].reduce((n, ch) => (n * 31 + ch.charCodeAt(0)) >>> 0, 7) % 5);
      try {
        const range = document.createRange();
        const a = this.view.domFromPos(Math.min(c.anchor, c.head));
        const b = this.view.domFromPos(Math.max(c.anchor, c.head));
        range.setStart(a.node, a.offset);
        range.setEnd(b.node, b.offset);
        if (c.anchor !== c.head) {
          for (const r of range.getClientRects()) {
            const box = document.createElement("div");
            box.className = "range";
            box.dataset.color = color;
            place(box, r, frame);
            layer.append(box);
          }
        }
        const h = this.view.domFromPos(c.head);
        const caretRange = document.createRange();
        caretRange.setStart(h.node, h.offset);
        caretRange.collapse(true);
        const r = caretRange.getClientRects()[0] ?? (h.node.nodeType === Node.ELEMENT_NODE ? h.node.getBoundingClientRect() : null);
        if (!r) continue;
        const caret = document.createElement("div");
        caret.className = "caret";
        caret.dataset.color = color;
        caret.dataset.conn = conn;
        const label = document.createElement("span");
        label.textContent = c.name;
        caret.append(label);
        place(caret, { left: r.left, top: r.top, height: r.height || 18, width: 2 }, frame);
        layer.append(caret);
      } catch {
        // a position the DOM can't show right now
      }
    }
  }

  /** Stops (or resumes) editing while the connection is away. */
  setOffline(offline) {
    this.offline = offline;
    this.view?.setEditable(!offline && !this.hasAttribute("readonly"));
  }

  attributeChangedCallback(name) {
    if (!this.view) return;
    if (name === "readonly") this.view.setEditable(!this.hasAttribute("readonly") && !this.offline);
    if (name === "placeholder") this.view.dom.dataset.placeholder = this.getAttribute("placeholder") ?? "";
  }

  /** The document's JSON. */
  get value() {
    return (this.view?.state.doc ?? this.pending)?.toJSON() ?? { type: "doc", content: [{ type: "paragraph" }] };
  }

  /** Replaces the content (no history, no change event). */
  set value(json) {
    const doc = schema.nodeFromJSON(json ?? { type: "doc", content: [{ type: "paragraph" }] });
    if (!this.view) {
      this.pending = doc;
      return;
    }
    const sel = this.view.state.selection;
    let selection = Selection.atEnd(doc);
    try {
      if (sel.anchor <= doc.content.size && sel.head <= doc.content.size && doc.resolve(sel.head).parent.isTextblock) selection = sel;
    } catch {
      // keep the end
    }
    this.history = new History();
    this.view.updateState(new EditorState(doc, selection));
  }

  /** Whether the document holds only empty textblocks. */
  get empty() {
    const doc = this.view?.state.doc ?? this.pending;
    if (!doc) return true;
    let empty = true;
    doc.nodesBetween(0, doc.content.size, (n) => {
      if (n.isInline || (n.isLeaf && !n.isTextblock)) empty = false;
      return empty;
    });
    return empty;
  }

  get state() {
    return this.view?.state;
  }

  focus() {
    this.view?.focus();
  }

  /** Applies a transaction: history, view, input rules, change event. */
  dispatch(tr) {
    const before = this.view.state.selection;
    if (tr.docChanged && !tr.meta.history) this.history.record(tr, before);
    this.view.updateState(this.view.state.apply(tr));
    if (tr.docChanged) {
      this.collab?.local(tr.steps);
      this.mapCursors(tr.mapping);
      this.drawCursors();
    }
    this.shareCursor();
    if (tr.meta.typing === "insert" && !tr.meta.rule) {
      const rule = inputRule(this.view.state);
      if (rule) rule(this.view.state, (rtr) => this.dispatch(rtr.setMeta("rule", true)));
    }
    this.refreshToolbar();
    this.suggest();
    if (tr.docChanged) {
      this.dispatchEvent(new CustomEvent("hzn-change", {
        bubbles: true, composed: true, detail: { doc: this.view.state.doc.toJSON(), steps: tr.steps.map((s) => s.toJSON()) },
      }));
    }
  }

  undo(redo = false) {
    const tr = this.history.pop(this.view.state, redo);
    if (tr) this.dispatch(tr);
    return true;
  }

  inputHandlers() {
    const run = (cmd) => (view) => view.run(cmd);
    const text = (view, e) => {
      const data = e.data ?? e.dataTransfer?.getData("text/plain") ?? "";
      if (data) view.run(insertText(data));
    };
    return {
      insertText: text,
      insertReplacementText: text,
      insertParagraph: run(splitBlock),
      insertLineBreak: run(insertLineBreak),
      deleteContentBackward: run(deleteBy("char", true)),
      deleteContentForward: run(deleteBy("char", false)),
      deleteWordBackward: run(deleteBy("word", true)),
      deleteWordForward: run(deleteBy("word", false)),
      deleteSoftLineBackward: run(deleteBy("line", true)),
      deleteHardLineBackward: run(deleteBy("line", true)),
      deleteSoftLineForward: run(deleteBy("line", false)),
      deleteHardLineForward: run(deleteBy("line", false)),
      deleteByCut: run(deleteSelection),
      deleteContent: run(deleteSelection),
      deleteByDrag: run(deleteSelection),
      historyUndo: () => this.undo(),
      historyRedo: () => this.undo(true),
      formatBold: run(toggleMark(schema.marks.bold)),
      formatItalic: run(toggleMark(schema.marks.italic)),
      formatStrikeThrough: run(toggleMark(schema.marks.strike)),
      insertFromPaste: (view, e) => this.paste(e.dataTransfer),
      insertFromDrop: (view, e) => this.paste(e.dataTransfer),
    };
  }

  /** Pastes images (uploaded), HTML, Markdown or plain text. */
  paste(data) {
    if (!data) return;
    const files = [...(data.files ?? [])].filter((f) => f.type.startsWith("image/"));
    if (files.length) return this.upload(files);
    const htmlText = data.getData("text/html");
    const text = data.getData("text/plain") ?? "";
    const $from = this.view.state.doc.resolve(this.view.state.selection.from);
    if ($from.parent.type.code) return this.view.run(insertText(text));
    let blocks;
    if (htmlText) blocks = fromHTML(htmlText);
    else if (looksLikeMarkdown(text)) blocks = fromMarkdown(text);
    else blocks = text.replace(/\r\n?/g, "\n").split("\n").map((line) => schema.nodes.paragraph.create(null, line ? [schema.text(line)] : []));
    if (!blocks.length) return;
    if (inlineOnly(blocks)) {
      const inline = blocks[0].content;
      if (!inline.size) return;
      this.view.run((state, dispatch) => {
        const tr = state.tr;
        tr.replace(state.selection.from, state.selection.to, new Slice(inline, 0, 0));
        dispatch(tr.setSelection(new Selection(state.selection.from + inline.size)));
        return true;
      });
      return;
    }
    this.view.run(insertBlocks(blocks));
  }

  /** Uploads images as attachments of the issue, then inserts them. */
  async upload(files) {
    const ws = this.getAttribute("ws");
    const issueKey = this.getAttribute("issue-key");
    const { api, i18n } = this.context;
    if (!ws || !issueKey || !files.length) return;
    const base = `/w/${encodeURIComponent(ws)}/issues/${encodeURIComponent(issueKey)}/attachments`;
    for (const file of files) {
      try {
        const { data } = await api.post(base, { body: { filename: file.name || "image", content_type: file.type, size: file.size } });
        const put = await fetch(data.upload_url, { method: data.method, body: file });
        if (!put.ok) throw new Error(`upload failed: ${put.status}`);
        await api.post(`${base}/${data.attachment.id}/complete`);
        this.view.run(insertNodes([schema.nodes.image.create({ attachment: data.attachment.id, alt: file.name || "" })]));
      } catch (err) {
        globalThis.LDS?.toast?.({ intent: "danger", title: i18n.t("editor.uploadFailed", { name: file.name, error: errorMessage(err, i18n) }) });
      }
    }
  }

  keymap() {
    const run = (cmd) => (view) => view.run(cmd);
    const popup = (f) => () => (this.popup.hidden || this.popupMode !== "list" ? false : (f(), true));
    return [
      // The suggestion list has the keys while open.
      key("ArrowDown", popup(() => this.moveChoice(1))),
      key("ArrowUp", popup(() => this.moveChoice(-1))),
      key("Enter", popup(() => this.choose(this.choice))),
      key("Tab", popup(() => this.choose(this.choice))),
      key("Escape", () => (this.popup.hidden ? false : (this.closePopup(), true))),
      key("Mod-b", run(toggleMark(schema.marks.bold))),
      key("Mod-i", run(toggleMark(schema.marks.italic))),
      key("Mod-e", run(toggleMark(schema.marks.code))),
      key("Mod-Shift-x", run(toggleMark(schema.marks.strike))),
      key("Mod-k", () => (this.editLink(), true)),
      key("Mod-z", () => this.undo()),
      key("Mod-Shift-z", () => this.undo(true)),
      key("Mod-y", () => this.undo(true)),
      key("Mod-Alt-Digit0", run(setBlockType("paragraph"))),
      key("Mod-Alt-Digit1", run(setBlockType("heading", { level: 1 }))),
      key("Mod-Alt-Digit2", run(setBlockType("heading", { level: 2 }))),
      key("Mod-Alt-Digit3", run(setBlockType("heading", { level: 3 }))),
      key("Mod-Shift-Digit7", run(toggleList("ordered_list"))),
      key("Mod-Shift-Digit8", run(toggleList("bullet_list"))),
      key("Mod-Shift-Digit9", run(toggleList("task_list"))),
      key("Tab", (view) => view.run(goToCell(1)) || view.run(sinkListItem)),
      key("Shift-Tab", (view) => view.run(goToCell(-1)) || view.run(liftListItem)),
    ];
  }

  /** Runs a toolbar command. */
  command(name) {
    const toggleBlock = (type, attrs) => (s, d) => (blockActive(s, type, attrs) ? setBlockType("paragraph") : setBlockType(type, attrs))(s, d);
    const commands = {
      bold: toggleMark(schema.marks.bold),
      italic: toggleMark(schema.marks.italic),
      strike: toggleMark(schema.marks.strike),
      code: toggleMark(schema.marks.code),
      h1: toggleBlock("heading", { level: 1 }),
      h2: toggleBlock("heading", { level: 2 }),
      bullet: toggleList("bullet_list"),
      ordered: toggleList("ordered_list"),
      task: toggleList("task_list"),
      quote: toggleWrap("quote"),
      codeblock: toggleBlock("code_block"),
      addRow: addRow(1),
      addColumn: addColumn(1),
      deleteRow,
      deleteColumn,
      deleteTable,
    };
    if (name === "link") return this.editLink();
    this.view.run(commands[name]);
    this.view.focus();
  }

  refreshToolbar() {
    const s = this.view.state;
    const pressed = {
      bold: markActive(s, schema.marks.bold), italic: markActive(s, schema.marks.italic), strike: markActive(s, schema.marks.strike),
      code: markActive(s, schema.marks.code), link: markActive(s, schema.marks.link), h1: blockActive(s, "heading", { level: 1 }),
      h2: blockActive(s, "heading", { level: 2 }), codeblock: blockActive(s, "code_block"),
    };
    const inTable = !!tableContext(s);
    for (const b of this.shadowRoot.querySelectorAll("[data-cmd]")) {
      if (b.dataset.cmd in pressed) b.setAttribute("aria-pressed", String(pressed[b.dataset.cmd]));
      if (["addRow", "addColumn", "deleteRow", "deleteColumn", "deleteTable"].includes(b.dataset.cmd)) b.hidden = !inTable;
    }
    this.shadowRoot.querySelector("[data-table-tools]").hidden = !inTable;
  }

  // --- Suggestions -----------------------------------------------------------------------

  /** Opens, updates or closes the suggestion list for the text before the cursor. */
  suggest() {
    if (this.popupMode === "link") return;
    const state = this.view.state;
    if (!state.selection.empty || !this.view.hasFocus()) return this.closePopup();
    const $pos = state.doc.resolve(state.selection.head);
    if (!$pos.parent.isTextblock || $pos.parent.type.code) return this.closePopup();
    const before = blockText($pos.parent).slice(0, $pos.parentOffset);
    for (const trigger of TRIGGERS) {
      const m = trigger.re.exec(before);
      if (!m) continue;
      const query = m[1];
      const from = $pos.pos - query.length - (trigger.kind === "issue" ? 0 : 1);
      return this.showList(trigger.kind, query, from);
    }
    this.closePopup();
  }

  async showList(kind, query, from) {
    const { t } = this.context.i18n;
    const q = query.toLowerCase();
    let items;
    switch (kind) {
      case "slash":
        items = this.blockItems().filter((it) => it.label.toLowerCase().includes(q) || it.keywords.includes(q));
        break;
      case "mention":
        items = [...this.people.entries()].filter(([, p]) => p.name.toLowerCase().includes(q)).slice(0, 8)
          .map(([id, p]) => ({ label: `@${p.name}`, nodes: () => [schema.nodes.mention.create({ user: id })] }));
        break;
      case "emoji":
        items = Object.entries(EMOJI).filter(([name]) => name.includes(q)).slice(0, 8)
          .map(([name, ch]) => ({ label: `${ch} :${name}:`, nodes: () => [schema.nodes.emoji.create({ name })] }));
        break;
      case "issue": {
        const seq = (this.searchSeq = (this.searchSeq ?? 0) + 1);
        items = await this.searchIssues(query);
        if (seq !== this.searchSeq) return;
        break;
      }
    }
    this.popupMode = "list";
    this.suggestion = { kind, from, items };
    this.choice = 0;
    if (!items.length && kind !== "slash") return this.closePopup();
    this.popup.innerHTML = items.length
      ? String(html`${items.map((it, i) => html`<div role="option" data-index="${i}" aria-selected="${String(i === 0)}">${it.label}${it.hint ? html`<span class="hint">${it.hint}</span>` : ""}</div>`)}`)
      : String(html`<div class="none">${t("editor.noMatch")}</div>`);
    this.placePopup(from);
  }

  async searchIssues(query) {
    const ws = this.getAttribute("ws");
    if (!ws) return [];
    const exact = /^[A-Z][A-Z0-9]{1,9}-[1-9]\d*$/.test(query);
    try {
      const { data } = await this.context.api.get(`/w/${encodeURIComponent(ws)}/search`, { query: { q: exact ? query : query.replace(/-$/, ""), limit: 6 } });
      const items = data.items.map((r) => ({ label: r.key, hint: r.title, nodes: () => [schema.nodes.issue_ref.create({ key: r.key })] }));
      if (exact && !items.some((it) => it.label === query)) items.unshift({ label: query, nodes: () => [schema.nodes.issue_ref.create({ key: query })] });
      return items.slice(0, 6);
    } catch {
      return exact ? [{ label: query, nodes: () => [schema.nodes.issue_ref.create({ key: query })] }] : [];
    }
  }

  /** The slash menu: blocks to turn the paragraph into, or insert. */
  blockItems() {
    const { t } = this.context.i18n;
    const block = (label, keywords, command) => ({ label, keywords, command });
    return [
      block(t("editor.text"), "paragraph text", setBlockType("paragraph")),
      block(t("editor.heading", { level: 1 }), "h1 heading title", setBlockType("heading", { level: 1 })),
      block(t("editor.heading", { level: 2 }), "h2 heading", setBlockType("heading", { level: 2 })),
      block(t("editor.heading", { level: 3 }), "h3 heading", setBlockType("heading", { level: 3 })),
      block(t("editor.bulletList"), "bullet list ul", toggleList("bullet_list")),
      block(t("editor.orderedList"), "numbered ordered list ol", toggleList("ordered_list")),
      block(t("editor.taskList"), "task todo checkbox", toggleList("task_list")),
      block(t("editor.quote"), "quote blockquote", toggleWrap("quote")),
      block(t("editor.callout"), "callout note info", toggleWrap("callout")),
      block(t("editor.codeBlock"), "code pre", setBlockType("code_block")),
      block(t("editor.divider"), "divider rule hr", insertNodes([schema.nodes.divider.create()])),
      block(t("editor.table"), "table grid", insertBlocks([createTable(3, 3)], true)),
      ...(this.getAttribute("issue-key") ? [block(t("editor.image"), "image picture upload", () => (this.shadowRoot.querySelector("[data-file]").click(), true))] : []),
    ];
  }

  moveChoice(delta) {
    const n = this.suggestion?.items.length ?? 0;
    if (!n) return;
    this.choice = (this.choice + delta + n) % n;
    for (const o of this.popup.querySelectorAll('[role="option"]')) o.setAttribute("aria-selected", String(Number(o.dataset.index) === this.choice));
    this.popup.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest" });
  }

  /** Applies a suggestion: the typed trigger goes, the choice takes its place. */
  choose(index) {
    const s = this.suggestion;
    const item = s?.items[index];
    this.closePopup();
    if (!item) return;
    const head = this.view.state.selection.head;
    this.view.run((state, dispatch) => {
      const tr = state.tr.delete(s.from, head).setSelection(new Selection(s.from));
      if (item.nodes) {
        const nodes = item.nodes();
        tr.insert(s.from, nodes);
        const after = s.from + Fragment.from(nodes).size;
        tr.insertText(" ", after, after, []);
      }
      dispatch(tr);
      return true;
    });
    if (item.command) this.view.run(item.command);
    this.view.focus();
  }

  placePopup(pos) {
    const frame = this.shadowRoot.querySelector(".frame").getBoundingClientRect();
    let rect = null;
    try {
      const { node, offset } = this.view.domFromPos(pos);
      const range = document.createRange();
      range.setStart(node, offset);
      range.collapse(true);
      rect = range.getClientRects()[0] ?? (node.nodeType === Node.ELEMENT_NODE ? node.getBoundingClientRect() : node.parentNode.getBoundingClientRect());
    } catch {
      rect = null;
    }
    this.popup.hidden = false;
    const left = rect ? rect.left - frame.left : 8;
    const top = rect ? rect.bottom - frame.top + 4 : 8;
    // The CSSOM, as the CSP forbids style attributes.
    this.popup.style.setProperty("left", `${Math.max(0, left)}px`);
    this.popup.style.setProperty("top", `${top}px`);
  }

  closePopup() {
    if (!this.popup) return;
    this.popup.hidden = true;
    this.popupMode = null;
    this.suggestion = null;
  }

  // --- Links -------------------------------------------------------------------------------

  /** ⌘K: a small field for the link of the selection (or of the link the cursor is in). */
  editLink() {
    const { t } = this.context.i18n;
    const state = this.view.state;
    let { from, to } = state.selection;
    const linkType = schema.marks.link;
    if (from === to) {
      // The link around the cursor, if any.
      const $pos = state.doc.resolve(from);
      const range = linkRange($pos, linkType);
      if (!range) return;
      ({ from, to } = range);
    }
    let current = "";
    state.doc.nodesBetween(from, to, (n) => {
      const m = n.marks?.find((x) => x.type === linkType);
      if (m) current = m.attrs.href;
    });
    this.popupMode = "link";
    this.popup.innerHTML = String(html`<form><input type="url" placeholder="https://" value="${current}" aria-label="${t("editor.link")}"></form>`);
    this.placePopup(to);
    const input = this.popup.querySelector("input");
    const form = this.popup.querySelector("form");
    const apply = () => {
      const href = input.value.trim();
      this.closePopup();
      this.view.run((s, dispatch) => {
        const tr = s.tr;
        s.doc.nodesBetween(from, to, (n, pos) => {
          const m = n.marks?.find((x) => x.type === linkType);
          if (m) tr.removeMark(Math.max(pos, from), Math.min(pos + n.nodeSize, to), m);
        });
        if (/^(https?:|mailto:)/i.test(href)) tr.addMark(from, to, linkType.create({ href }));
        dispatch(tr);
        return true;
      });
      this.view.focus();
    };
    form.addEventListener("submit", (e) => {
      e.preventDefault();
      apply();
    });
    input.addEventListener("keydown", (e) => {
      if (e.key === "Escape") {
        e.preventDefault();
        this.closePopup();
        this.view.focus();
      }
    });
    requestAnimationFrame(() => input.focus());
  }
}

/** Positions an overlay box over a client rect, through the CSSOM (no style attributes under the CSP). */
function place(el, r, frame) {
  el.style.setProperty("left", `${r.left - frame.left}px`);
  el.style.setProperty("top", `${r.top - frame.top}px`);
  el.style.setProperty("height", `${r.height}px`);
  if (el.classList.contains("range")) el.style.setProperty("width", `${r.width}px`);
}

/** The extent of the link mark around a position, within its textblock. */
function linkRange($pos, linkType) {
  const parent = $pos.parent;
  const index = $pos.index();
  const at = parent.maybeChild(index) ?? parent.maybeChild(index - 1);
  const mark = at?.marks.find((m) => m.type === linkType);
  if (!mark) return null;
  let start = index;
  let end = index;
  while (start > 0 && mark.isInSet(parent.child(start - 1).marks)) start--;
  while (end < parent.childCount && mark.isInSet(parent.child(end).marks)) end++;
  let from = $pos.start();
  for (let i = 0; i < start; i++) from += parent.child(i).nodeSize;
  let to = from;
  for (let i = start; i < end; i++) to += parent.child(i).nodeSize;
  return { from, to };
}

customElements.define("hzn-editor", HznEditor);
