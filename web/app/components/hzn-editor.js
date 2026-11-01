// <hzn-editor>: the block editor (docs/frontend.md#block-editor) for
// descriptions and comments.
//
// Attributes: placeholder, label, ws and issue-key (for images and links),
// readonly, autofocus. Properties: value (the document's JSON; setting it
// replaces the content), state, people (a Map for mentions). Events
// (bubbling, composed): hzn-change { doc, steps } on every change (steps
// as JSON, for the collaboration protocol), hzn-blur when focus leaves.
// Keys: ⌘B ⌘I ⌘E (code) ⌘⇧X (strike); ⌘⌥0–3 paragraph and headings;
// ⌘⇧7 ⌘⇧8 ⌘⇧9 lists; Tab / ⇧Tab in lists; ⌘Z ⌘⇧Z.

import { History } from "horizon/editor/history.js";
import { blockActive, deleteBy, deleteSelection, insertLineBreak, insertText, liftListItem, markActive, setBlockType, sinkListItem,
  splitBlock, toggleList, toggleMark, toggleTask, toggleWrap } from "horizon/editor/commands.js";
import { schema } from "horizon/editor/model.js";
import { docStyles } from "horizon/editor/render.js";
import { EditorState, Selection } from "horizon/editor/state.js";
import { EditorView } from "horizon/editor/view.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";

const isMac = /Mac|iPhone|iPad/.test(globalThis.navigator?.platform || "");

const styles = sheet(`
  :host { display: block; }
  .frame { border: 1px solid var(--lds-color-border-default); border-radius: var(--lds-radius-md); background: var(--lds-color-bg-surface); }
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
  .toolbar .sep { inline-size: 1px; margin: var(--lds-space-1) 2px; background: var(--lds-color-border-subtle); }
  .editor { min-block-size: var(--hzn-editor-min-height, 4.5rem); padding: var(--lds-space-2) var(--lds-space-3); outline: none;
    white-space: pre-wrap; word-wrap: break-word; line-height: 1.55; }
  .editor[data-empty]::before { content: attr(data-placeholder); position: absolute; color: var(--lds-color-fg-subtle); pointer-events: none; }
  .editor ul.tasks li { display: flex; gap: var(--lds-space-2); }
  .editor ul.tasks li > div { flex: 1; min-inline-size: 0; }
  .editor .check { flex: none; inline-size: 1rem; block-size: 1rem; margin-block-start: 0.3em; border: 1px solid var(--lds-color-border-strong);
    border-radius: var(--lds-radius-xs); cursor: pointer; display: inline-flex; align-items: center; justify-content: center; }
  .editor li[data-checked="true"] > .check { background: var(--lds-color-accent-bg, var(--lds-color-accent-fg)); border-color: transparent; }
  .editor li[data-checked="true"] > .check::after { content: "✓"; color: var(--lds-color-fg-on-accent, white); font-size: 0.75rem; }
  .editor li[data-checked="true"] > div { color: var(--lds-color-fg-muted); text-decoration: line-through; }
  .editor figure.image { margin: 0 0 var(--lds-space-2); }
  .editor figure.image img { max-inline-size: 100%; border-radius: var(--lds-radius-md); }
  .editor .emoji { font-style: normal; }
  .editor a { pointer-events: none; }
`);

const mod = (e) => (isMac ? e.metaKey : e.ctrlKey);
const key = (spec, run) => {
  // "Mod-Shift-x": modifiers then the key (KeyboardEvent.key, lower case) or a code.
  const parts = spec.split("-");
  const k = parts.pop();
  const want = { mod: parts.includes("Mod"), shift: parts.includes("Shift"), alt: parts.includes("Alt") };
  return {
    match: (e) => mod(e) === want.mod && e.shiftKey === want.shift && e.altKey === want.alt &&
      (e.key.toLowerCase() === k.toLowerCase() || e.code === k),
    run,
  };
};

class HznEditor extends HTMLElement {
  static observedAttributes = ["readonly", "placeholder"];

  constructor() {
    super();
    this.history = new History();
    this.pending = null; // a value set before connecting
  }

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open", delegatesFocus: true });
    root.adoptedStyleSheets = [docStyles, styles];
    const { t } = this.context.i18n;
    const button = (cmd, label, text) => html`<button type="button" tabindex="-1" data-cmd="${cmd}" title="${label}" aria-label="${label}">${text}</button>`;
    root.innerHTML = String(html`
      <div class="frame">
        <div class="toolbar" role="toolbar" aria-label="${t("editor.toolbar")}">
          ${button("bold", t("editor.bold"), html`<b>B</b>`)}${button("italic", t("editor.italic"), html`<i>I</i>`)}
          ${button("strike", t("editor.strike"), html`<s>S</s>`)}${button("code", t("editor.code"), "</>")}
          <span class="sep"></span>
          ${button("h1", t("editor.heading", { level: 1 }), "H1")}${button("h2", t("editor.heading", { level: 2 }), "H2")}
          ${button("bullet", t("editor.bulletList"), "•")}${button("ordered", t("editor.orderedList"), "1.")}${button("task", t("editor.taskList"), "☐")}
          ${button("quote", t("editor.quote"), "❝")}${button("codeblock", t("editor.codeBlock"), "{ }")}
        </div>
        <div class="editor" spellcheck="true"></div>
      </div>`);
    const dom = root.querySelector(".editor");
    dom.setAttribute("aria-label", this.getAttribute("label") ?? this.getAttribute("placeholder") ?? "");
    dom.dataset.placeholder = this.getAttribute("placeholder") ?? "";
    const doc = this.pending ?? schema.nodeFromJSON({ type: "doc", content: [{ type: "paragraph" }] });
    this.view = new EditorView(dom, {
      state: new EditorState(doc, Selection.atStart(doc)),
      dispatch: (tr) => this.dispatch(tr),
      context: { ws: this.getAttribute("ws"), issueKey: this.getAttribute("issue-key"), people: this.people },
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
    dom.addEventListener("hzn-selection", () => this.refreshToolbar());
    this.addEventListener("focusout", (e) => {
      if (!this.contains(e.relatedTarget) && !root.contains(e.relatedTarget)) {
        this.history.close();
        this.dispatchEvent(new CustomEvent("hzn-blur", { bubbles: true, composed: true }));
      }
    });
    if (this.hasAttribute("autofocus")) requestAnimationFrame(() => this.focus());
  }

  attributeChangedCallback(name) {
    if (!this.view) return;
    if (name === "readonly") this.view.setEditable(!this.hasAttribute("readonly"));
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
    const max = doc.content.size;
    const keep = sel.anchor <= max && sel.head <= max ? sel : Selection.atEnd(doc);
    let selection;
    try {
      doc.resolve(keep.anchor);
      selection = keep;
    } catch {
      selection = Selection.atEnd(doc);
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

  /** Applies a transaction: history, view, change event. */
  dispatch(tr) {
    const before = this.view.state.selection;
    if (tr.docChanged && !tr.meta.history) this.history.record(tr, before);
    this.view.updateState(this.view.state.apply(tr));
    this.refreshToolbar();
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
      insertFromPaste: (view, e) => this.paste(view, e.dataTransfer),
      insertFromDrop: (view, e) => this.paste(view, e.dataTransfer),
    };
  }

  /** Plain text paste: one paragraph per line (rich paste comes with the editor extras). */
  paste(view, data) {
    const text = data?.getData("text/plain") ?? "";
    if (!text) return;
    const lines = text.replace(/\r\n?/g, "\n").split("\n");
    view.run(insertText(lines[0]));
    for (const line of lines.slice(1)) {
      view.run(splitBlock);
      if (line) view.run(insertText(line));
    }
  }

  keymap() {
    const run = (cmd) => (view) => view.run(cmd);
    const inList = (cmd) => (view) => view.run(cmd);
    return [
      key("Mod-b", run(toggleMark(schema.marks.bold))),
      key("Mod-i", run(toggleMark(schema.marks.italic))),
      key("Mod-e", run(toggleMark(schema.marks.code))),
      key("Mod-Shift-x", run(toggleMark(schema.marks.strike))),
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
      key("Tab", inList(sinkListItem)),
      key("Shift-Tab", inList(liftListItem)),
    ];
  }

  /** Runs a toolbar command. */
  command(name) {
    const commands = {
      bold: toggleMark(schema.marks.bold),
      italic: toggleMark(schema.marks.italic),
      strike: toggleMark(schema.marks.strike),
      code: toggleMark(schema.marks.code),
      h1: (s, d) => (blockActive(s, "heading", { level: 1 }) ? setBlockType("paragraph") : setBlockType("heading", { level: 1 }))(s, d),
      h2: (s, d) => (blockActive(s, "heading", { level: 2 }) ? setBlockType("paragraph") : setBlockType("heading", { level: 2 }))(s, d),
      bullet: toggleList("bullet_list"),
      ordered: toggleList("ordered_list"),
      task: toggleList("task_list"),
      quote: toggleWrap("quote"),
      codeblock: (s, d) => (blockActive(s, "code_block") ? setBlockType("paragraph") : setBlockType("code_block"))(s, d),
    };
    this.view.run(commands[name]);
    this.view.focus();
  }

  refreshToolbar() {
    const s = this.view.state;
    const pressed = {
      bold: markActive(s, schema.marks.bold), italic: markActive(s, schema.marks.italic), strike: markActive(s, schema.marks.strike),
      code: markActive(s, schema.marks.code), h1: blockActive(s, "heading", { level: 1 }), h2: blockActive(s, "heading", { level: 2 }),
      codeblock: blockActive(s, "code_block"),
    };
    for (const b of this.shadowRoot.querySelectorAll("[data-cmd]")) {
      if (b.dataset.cmd in pressed) b.setAttribute("aria-pressed", String(pressed[b.dataset.cmd]));
    }
  }
}

customElements.define("hzn-editor", HznEditor);
