// <hzn-board>: /w/:ws/p/:key/board[?iteration=ID|all] — kanban (wireframes:
// Board). Columns are statuses, one per name across the project's
// workflows, in category order; cards are the active iteration's issues
// (a picker when several are active), or the project's when none is.
// Dragging a card to another column moves it to its own workflow's status
// of that name, when a transition allows it; within a column it ranks.
// Swimlanes group cards by assignee, parent or priority, remembered per
// project in this browser.
//
// Keyboard: j/k move within a column, h/l between columns; Alt+←/→ moves
// the current card to the column before or after, Alt+↑/↓ up or down;
// Enter peeks, o opens; ⌘K lists the columns it can go to.

import "horizon/components/hzn-issue-create.js";
import "horizon/components/hzn-issue-detail.js";
import {
  PRIORITIES, avatar, changesProjectData, forgetProjects, issueHref, issueStyles, labelChips, priorityGlyph, projectData, statusGlyph,
} from "horizon/components/issue-ui.js";
import { daysLeft, loadIterations, planningStyles, stateBadge, viewSwitch } from "horizon/components/planning-ui.js";
import { paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const LANES = ["none", "assignee", "parent", "priority"];
const ALL = "all";
const NONE = "-"; // the lane of cards without a value

const styles = sheet(`
  :host { display: flex; flex-direction: column; min-block-size: 100%; }
  header.bar { display: flex; align-items: center; justify-content: space-between; gap: var(--lds-space-3);
    min-block-size: 3.25rem; padding: 0 var(--lds-space-5); border-block-end: 1px solid var(--lds-color-border-default); }
  .crumb { display: flex; align-items: center; gap: var(--lds-space-2); }
  .crumb strong { font-weight: var(--lds-font-weight-semibold); }
  .crumb lds-icon { color: var(--lds-color-fg-subtle); }
  .swatch { display: inline-block; inline-size: 0.625rem; block-size: 0.625rem; border-radius: var(--lds-radius-xs); }
  .toolbar { display: flex; align-items: center; gap: var(--lds-space-3); padding: var(--lds-space-2) var(--lds-space-5);
    border-block-end: 1px solid var(--lds-color-border-subtle); flex-wrap: wrap; }
  .toolbar .grow { flex: 1; }
  .toolbar lds-select { min-inline-size: 11rem; }
  .board { flex: 1; overflow: auto; padding: var(--lds-space-4) var(--lds-space-5); }
  .grid { display: grid; grid-template-columns: repeat(var(--columns), minmax(15rem, 1fr)); gap: var(--lds-space-3); min-inline-size: min-content; }
  .colhead { display: flex; align-items: center; gap: var(--lds-space-2); padding: var(--lds-space-1) var(--lds-space-2);
    font-size: var(--lds-font-size-sm); font-weight: var(--lds-font-weight-semibold); position: sticky; top: 0; background: var(--lds-color-bg-canvas); z-index: 1; }
  .colhead .count { color: var(--lds-color-fg-muted); font-weight: normal; }
  .lane { grid-column: 1 / -1; display: flex; align-items: center; gap: var(--lds-space-2); padding: var(--lds-space-2) var(--lds-space-1) 0;
    font-size: var(--lds-font-size-sm); font-weight: var(--lds-font-weight-semibold); border-block-start: 1px solid var(--lds-color-border-subtle); }
  .lane .count { color: var(--lds-color-fg-muted); font-weight: normal; }
  .cell { display: flex; flex-direction: column; gap: var(--lds-space-2); min-block-size: 4rem; padding: var(--lds-space-2);
    border-radius: var(--lds-radius-md); background: var(--lds-color-bg-subtle); }
  .cell.blocked { opacity: 0.45; }
  .cell.drop-end { box-shadow: inset 0 -2px 0 var(--lds-color-accent-fg); }
  .card { display: grid; gap: var(--lds-space-2); padding: var(--lds-space-3); border: 1px solid var(--lds-color-border-default);
    border-radius: var(--lds-radius-md); background: var(--lds-color-bg-surface); cursor: pointer; outline: none; }
  .card:hover { border-color: var(--lds-color-border-strong); }
  .card:focus-visible, .card[aria-current="true"] { border-color: var(--lds-color-accent-border); box-shadow: 0 0 0 1px var(--lds-color-accent-border); }
  .card.drop-before { box-shadow: 0 -3px 0 var(--lds-color-accent-fg); }
  .card.dragging { opacity: 0.5; }
  .card .top, .card .bottom { display: flex; align-items: center; gap: var(--lds-space-2); }
  .card .top .key { flex: 1; font-size: var(--lds-font-size-sm); }
  .card .bottom .grow { flex: 1; }
  .card .num { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); font-variant-numeric: tabular-nums; }
  .card .title { overflow-wrap: anywhere; }
  .empty { padding: var(--lds-space-12) var(--lds-space-6); }
  lds-drawer { --lds-drawer-width: min(34rem, 100vw); }
  [slot="header"] { display: flex; align-items: center; gap: var(--lds-space-2); font-size: var(--lds-font-size-sm); }
`);

function storedLanes(id) {
  try {
    return localStorage.getItem(`horizon.board.lanes.${id}`);
  } catch {
    return null;
  }
}

function storeLanes(id, value) {
  try {
    localStorage.setItem(`horizon.board.lanes.${id}`, value);
  } catch {
    // private mode: the choice lasts until the page closes
  }
}

class HznBoard extends HTMLElement {
  static observedAttributes = ["ws", "key"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [issueStyles, planningStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("dblclick", (e) => {
      const card = e.target.closest?.(".card");
      if (card) this.open(card.dataset.key);
    });
    root.addEventListener("lds-change", (e) => this.onChange(e));
    root.addEventListener("dragstart", (e) => this.onDragStart(e));
    root.addEventListener("dragover", (e) => this.onDragOver(e));
    root.addEventListener("drop", (e) => this.onDrop(e));
    root.addEventListener("dragend", () => this.endDrag());
    const { shortcuts, socket } = this.context;
    this.offScope = shortcuts.pushScope("board");
    const key = (id, keys, description, handler) => shortcuts.register({ id: `board-${id}`, keys, scope: "board", description, handler });
    this.offKeys = [
      key("down", "j", "shortcut.next", () => this.step(0, 1)),
      key("up", "k", "shortcut.previous", () => this.step(0, -1)),
      key("left", "h", "shortcut.columnLeft", () => this.step(-1, 0)),
      key("right", "l", "shortcut.columnRight", () => this.step(1, 0)),
      key("peek", "enter", "shortcut.peek", () => this.peek(this.current)),
      key("open", "o", "shortcut.open", () => this.current && this.open(this.current)),
      key("move-left", "alt+arrowleft", "shortcut.moveLeft", () => this.moveColumn(-1)),
      key("move-right", "alt+arrowright", "shortcut.moveRight", () => this.moveColumn(1)),
      key("rank-up", "alt+arrowup", "shortcut.rankUp", () => this.nudge(-1)),
      key("rank-down", "alt+arrowdown", "shortcut.rankDown", () => this.nudge(1)),
    ];
    this.offSocket = socket.on((m) => this.onPush(m));
    this.onChanged = () => this.refresh();
    document.addEventListener("hzn-issue-changed", this.onChanged);
    this.load();
  }

  disconnectedCallback() {
    this.offScope?.();
    for (const off of this.offKeys ?? []) off();
    this.offSocket?.();
    this.offCommands?.();
    document.removeEventListener("hzn-issue-changed", this.onChanged);
    clearTimeout(this.timer);
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.load();
  }

  get ws() {
    return this.getAttribute("ws");
  }

  get can() {
    return this.data?.project.can ?? [];
  }

  get writable() {
    return !this.data.project.archived_at;
  }

  async load() {
    const { i18n, router } = this.context;
    const key = this.getAttribute("key");
    try {
      this.data = await projectData(this.context, this.ws, key);
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`
        <lds-empty-state class="empty"><lds-icon slot="icon" name="search"></lds-icon>
          <h2 slot="title">${err.status === 404 ? i18n.t("projects.notFound") : errorMessage(err, i18n)}</h2>
          <a href="/w/${this.ws}">${i18n.t("notFound.home")}</a>
        </lds-empty-state>`);
      return;
    }
    if (this.data.project.key !== key.toUpperCase()) {
      router.navigate(`/w/${this.ws}/p/${this.data.project.key}/board`, { replace: true }); // a former key
      return;
    }
    this.laneId = `${this.ws}/${this.data.project.key}`;
    this.lanes = LANES.includes(storedLanes(this.laneId)) ? storedLanes(this.laneId) : "none";
    this.columns = this.columnsOf();
    await this.refresh();
  }

  /**
   * The columns: one per status name, ordered by their category, then
   * their place in the workflow, each with the statuses it stands for.
   */
  columnsOf() {
    const byName = new Map();
    for (const s of this.data.statuses.values()) {
      const c = byName.get(s.name);
      if (!c) byName.set(s.name, { name: s.name, position: s.position, statuses: [s] });
      else {
        c.statuses.push(s);
        c.position = Math.min(c.position, s.position);
      }
    }
    return [...byName.values()].sort((a, b) => a.position - b.position);
  }

  /** The column of a status. */
  columnOf(statusId) {
    return this.columns.findIndex((c) => c.statuses.some((s) => s.id === statusId));
  }

  /** The status of an issue's own workflow standing for a column, if any. */
  statusIn(issue, column) {
    const wf = this.data.types.get(issue.type_id)?.workflow_id;
    return this.columns[column]?.statuses.find((s) => s.workflow === wf) ?? null;
  }

  /** Fetches the iterations and the issues, then renders. */
  async refresh() {
    if (!this.data) return;
    const { api, i18n } = this.context;
    const key = this.data.project.key;
    try {
      const items = [];
      const [list] = await Promise.all([
        loadIterations(this.context, this.ws, key),
        (async () => {
          let cursor = "";
          do {
            const { data } = await api.get(`/w/${encodeURIComponent(this.ws)}/projects/${key}/issues`, { query: { limit: 500, cursor: cursor || undefined } });
            items.push(...data.items);
            cursor = data.next_cursor;
          } while (cursor);
        })(),
      ]);
      this.active = list.items.filter((it) => it.state === "active");
      this.items = items;
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    // The scope: ?iteration=, else the first active iteration, else all.
    const wanted = this.context.router.current?.query.get("iteration");
    this.scope = wanted === ALL ? ALL : this.active.find((it) => it.id === wanted)?.id ?? this.active[0]?.id ?? ALL;
    if (!this.framed) this.renderFrame();
    this.renderBoard();
  }

  get iteration() {
    return this.active.find((it) => it.id === this.scope) ?? null;
  }

  /** The cards in scope, in rank order. */
  cards() {
    return this.scope === ALL ? this.items : this.items.filter((i) => i.iteration_id === this.scope);
  }

  renderFrame() {
    const { t } = this.context.i18n;
    const p = this.data.project;
    this.shadowRoot.innerHTML = String(html`
      <header class="bar">
        <div class="crumb" data-crumb></div>
        ${p.can.includes("issue.create") && !p.archived_at ? html`<lds-button variant="primary" size="sm" data-action="create"><lds-icon slot="start" name="plus" size="sm"></lds-icon>${t("issues.new")}</lds-button>` : ""}
      </header>
      ${p.archived_at ? html`<lds-alert intent="warning">${t("projects.isArchived")}</lds-alert>` : ""}
      <div class="toolbar" data-toolbar></div>
      <div class="board" data-board></div>
      <lds-drawer non-modal size="md" data-peek>
        <span slot="header" data-peek-title></span>
        <lds-button slot="actions" variant="ghost" size="sm" data-action="open" label="${t("issues.openPage")}"><lds-icon name="external-link" size="sm"></lds-icon></lds-button>
      </lds-drawer>
      <hzn-issue-create></hzn-issue-create>`);
    this.framed = true;
    this.shadowRoot.querySelector("[data-peek]").addEventListener("lds-close", () => {
      this.peeked = null;
      this.markCards();
    });
  }

  renderBoard() {
    const { i18n } = this.context;
    const { t } = i18n;
    const p = this.data.project;
    const it = this.iteration;
    const left = it ? daysLeft(it) : null;
    this.shadowRoot.querySelector("[data-crumb]").innerHTML = String(html`
      <span class="swatch" data-colour="${p.colour}"></span><strong>${p.name}</strong>
      <lds-icon name="chevron-right" size="sm"></lds-icon><span>${it ? it.name : t("board.allIssues")}</span>
      ${it ? html`${stateBadge(t, it.state)}${left !== null ? html`<span class="muted">${left >= 0 ? t("board.daysLeft", { count: left }) : t("board.overdue", { count: -left })}</span>` : ""}` : ""}`);
    this.shadowRoot.querySelector("[data-toolbar]").innerHTML = String(html`
      ${viewSwitch(t, this.ws, p.key, "board")}
      <span class="grow"></span>
      <lds-select size="sm" data-scope value="${this.scope}" label="${t("board.scope")}">
        ${this.active.map((x) => html`<option value="${x.id}">${x.name}</option>`)}
        <option value="${ALL}">${t("board.allIssues")}</option>
      </lds-select>
      <lds-select size="sm" data-lanes value="${this.lanes}" label="${t("board.swimlanes")}">
        ${LANES.map((l) => html`<option value="${l}">${t(`board.lanes.${l}`)}</option>`)}
      </lds-select>`);
    const cards = this.cards();
    const lanes = this.laneGroups(cards);
    const board = this.shadowRoot.querySelector("[data-board]");
    board.innerHTML = String(html`
      <div class="grid" role="grid" aria-label="${t("board.label")}">
        ${this.columns.map((c, ci) => html`
          <div class="colhead" role="columnheader">${statusGlyph(this.data, c.statuses[0].id)}<span>${c.name}</span>
            <span class="count">${cards.filter((i) => this.columnOf(i.status_id) === ci).length}</span></div>`)}
        ${lanes.map((lane) => html`
          ${lane.id === null ? "" : html`<div class="lane" role="rowheader">${lane.label}<span class="count">${lane.cards.length}</span></div>`}
          ${this.columns.map((c, ci) => html`
            <div class="cell" role="gridcell" data-column="${ci}" data-lane="${lane.id ?? ""}" aria-label="${c.name}${lane.id === null ? "" : ` · ${lane.text}`}">
              ${lane.cards.filter((i) => this.columnOf(i.status_id) === ci).map((i) => this.card(i))}
            </div>`)}`)}
      </div>`);
    // The column count, through the CSSOM: the CSP forbids style attributes.
    board.querySelector(".grid").style.setProperty("--columns", String(this.columns.length));
    paintSwatches(this.shadowRoot);
    this.cardEls = [...board.querySelectorAll(".card")];
    if (!this.cardEls.some((c) => c.dataset.key === this.current)) this.current = null;
    this.markCards();
    this.registerCommands();
  }

  card(i) {
    const { t } = this.context.i18n;
    const draggable = this.writable && (this.can.includes("issue.transition") || this.can.includes("issue.rank"));
    return html`
      <div class="card" role="button" tabindex="-1" data-key="${i.key}" aria-label="${i.key} ${i.title}" ${draggable ? html`draggable="true"` : ""}>
        <div class="top"><span class="key">${i.key}</span>${avatar(this.data, i.assignee_id)}</div>
        <span class="title">${i.title}</span>
        <div class="bottom">${priorityGlyph(i.priority, t)}${labelChips(this.data, i.labels)}<span class="grow"></span><span class="num">${i.estimate ?? ""}</span></div>
      </div>`;
  }

  /** The swimlanes: one for everything, or a lane per value of the field. */
  laneGroups(cards) {
    const { t } = this.context.i18n;
    if (this.lanes === "none") return [{ id: null, cards }];
    const groups = new Map();
    const add = (id, label, text, i) => {
      if (!groups.has(id)) groups.set(id, { id, label, text, cards: [] });
      if (i) groups.get(id).cards.push(i);
    };
    if (this.lanes === "priority") for (const p of PRIORITIES) add(p, html`${priorityGlyph(p, t)} ${t(`priority.${p}`)}`, t(`priority.${p}`));
    for (const i of cards) {
      switch (this.lanes) {
        case "assignee": {
          const name = i.assignee_id ? this.data.people.get(i.assignee_id)?.name ?? t("issue.someone") : t("issue.unassigned");
          add(i.assignee_id ?? NONE, html`${avatar(this.data, i.assignee_id)} ${name}`, name, i);
          break;
        }
        case "parent": {
          const parent = i.parent_id && this.items.find((x) => x.id === i.parent_id);
          const name = parent ? `${parent.key} ${parent.title}` : i.parent_id ? t("board.otherParent") : t("board.noParent");
          add(parent ? parent.key : NONE, name, name, i);
          break;
        }
        case "priority":
          add(i.priority, "", "", i);
          break;
      }
    }
    // Cards without a value last; priority lanes in priority order.
    const out = [...groups.values()].filter((g) => g.cards.length || this.lanes === "priority");
    return out.filter((g) => g.id !== NONE).concat(out.filter((g) => g.id === NONE));
  }

  markCards() {
    for (const c of this.cardEls ?? []) {
      c.setAttribute("aria-current", String(c.dataset.key === (this.peeked ?? this.current)));
      c.tabIndex = c.dataset.key === this.current ? 0 : -1;
    }
  }

  focusCard(key) {
    this.current = key;
    this.markCards();
    const el = this.cardEls?.find((c) => c.dataset.key === key);
    el?.focus();
    el?.scrollIntoView({ block: "nearest", inline: "nearest" });
    this.registerCommands();
  }

  /** Moves the current card: dx columns (to the first card there) or dy cards. */
  step(dx, dy) {
    const els = this.cardEls ?? [];
    if (!els.length) return;
    const cur = els.find((c) => c.dataset.key === this.current);
    if (!cur) return this.focusCard(els[0].dataset.key);
    const cell = cur.closest(".cell");
    if (dy) {
      const inCell = [...cell.querySelectorAll(".card")];
      const next = inCell[inCell.indexOf(cur) + dy];
      if (next) this.focusCard(next.dataset.key);
    } else {
      const col = Number(cell.dataset.column) + dx;
      const target = els.find((c) => Number(c.closest(".cell").dataset.column) === col && c.closest(".cell").dataset.lane === cell.dataset.lane)
        ?? els.find((c) => Number(c.closest(".cell").dataset.column) === col);
      if (target) this.focusCard(target.dataset.key);
    }
    if (this.peeked) this.peek(this.current);
  }

  issue(key) {
    return this.items.find((i) => i.key === key);
  }

  /** Alt+←/→: to the column before or after, if its workflow has it. */
  moveColumn(dx) {
    const i = this.current && this.issue(this.current);
    if (!i) return;
    this.place(i.key, this.columnOf(i.status_id) + dx, null, null, null);
  }

  /** Alt+↑/↓: one place up or down in its cell. */
  nudge(dy) {
    const el = this.cardEls?.find((c) => c.dataset.key === this.current);
    if (!el || !this.can.includes("issue.rank")) return;
    const inCell = [...el.closest(".cell").querySelectorAll(".card")].map((c) => c.dataset.key);
    const j = inCell.indexOf(this.current) + dy;
    if (j < 0 || j >= inCell.length) return;
    const i = this.issue(this.current);
    if (dy < 0) this.place(i.key, this.columnOf(i.status_id), null, inCell[j - 1] ?? null, inCell[j]);
    else this.place(i.key, this.columnOf(i.status_id), null, inCell[j], null);
  }

  // --- Drag and drop -----------------------------------------------------------------

  async onDragStart(e) {
    const card = e.target.closest?.(".card[draggable]");
    if (!card) return;
    this.dragKey = card.dataset.key;
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", card.dataset.key);
    card.classList.add("dragging");
    // Columns it can't go to are dimmed as soon as its transitions are known.
    this.allowed = await this.allowedColumns(this.issue(card.dataset.key));
    if (this.dragKey !== card.dataset.key) return;
    for (const cell of this.shadowRoot.querySelectorAll(".cell")) {
      cell.classList.toggle("blocked", !this.allowed.has(Number(cell.dataset.column)));
    }
  }

  /** The columns an issue can move to now: its own, and those a transition reaches. */
  async allowedColumns(issue) {
    const allowed = new Set([this.columnOf(issue.status_id)]);
    if (!this.can.includes("issue.transition") || !this.writable) return allowed;
    try {
      const { data } = await this.context.api.get(`/w/${encodeURIComponent(this.ws)}/issues/${encodeURIComponent(issue.key)}/transitions`);
      for (const tr of data.items) if (!tr.blocked) allowed.add(this.columnOf(tr.to_status_id));
    } catch {
      // unknown: let the server decide on drop
      for (let c = 0; c < this.columns.length; c++) allowed.add(c);
    }
    return allowed;
  }

  dropAt(e) {
    const cell = e.target.closest?.(".cell");
    if (!cell) return null;
    const cards = [...cell.querySelectorAll(".card")].filter((c) => c.dataset.key !== this.dragKey);
    const before = cards.find((c) => {
      const box = c.getBoundingClientRect();
      return e.clientY < box.top + box.height / 2;
    });
    const at = before ? cards.indexOf(before) : cards.length;
    return { cell, column: Number(cell.dataset.column), lane: cell.dataset.lane, before: before ?? null, after: cards[at - 1] ?? null };
  }

  onDragOver(e) {
    if (!this.dragKey) return;
    const at = this.dropAt(e);
    if (!at || (this.allowed && !this.allowed.has(at.column))) return;
    e.preventDefault();
    this.clearDrop();
    if (at.before) at.before.classList.add("drop-before");
    else at.cell.classList.add("drop-end");
  }

  onDrop(e) {
    if (!this.dragKey) return;
    const at = this.dropAt(e);
    const key = this.dragKey;
    const allowed = this.allowed;
    this.endDrag();
    if (!at || (allowed && !allowed.has(at.column))) return;
    e.preventDefault();
    this.place(key, at.column, this.lanes === "none" ? null : at.lane, at.after?.dataset.key ?? null, at.before?.dataset.key ?? null);
  }

  clearDrop() {
    for (const el of this.shadowRoot.querySelectorAll(".drop-before, .drop-end")) el.classList.remove("drop-before", "drop-end");
  }

  endDrag() {
    this.clearDrop();
    for (const el of this.shadowRoot.querySelectorAll(".blocked, .dragging")) el.classList.remove("blocked", "dragging");
    this.dragKey = null;
    this.allowed = null;
  }

  /**
   * Puts a card in a column (its workflow's status of that name, through a
   * transition), in a lane (changing the lane's field), between two cards
   * (rank): each as its own request, then a reload.
   */
  async place(key, column, lane, afterKey, beforeKey) {
    const { api, i18n } = this.context;
    const { t } = i18n;
    const i = this.issue(key);
    if (!i || column < 0 || column >= this.columns.length) return;
    const base = `/w/${encodeURIComponent(this.ws)}/issues/${encodeURIComponent(key)}`;
    const requests = [];
    if (column !== this.columnOf(i.status_id)) {
      const status = this.statusIn(i, column);
      if (!status || !this.can.includes("issue.transition")) {
        globalThis.LDS?.toast?.({ intent: "warning", title: t("board.notInWorkflow", { key, status: this.columns[column].name }) });
        return;
      }
      i.status_id = status.id;
      requests.push(() => api.post(`${base}/transition`, { body: { status_id: status.id } }));
    }
    const laneValue = lane === NONE ? null : lane;
    if (lane !== null && lane !== undefined) {
      if (this.lanes === "assignee" && laneValue !== (i.assignee_id ?? null)) {
        i.assignee_id = laneValue;
        requests.push(() => api.post(`${base}/assign`, { body: { assignee_id: laneValue } }));
      } else if (this.lanes === "priority" && laneValue !== i.priority) {
        i.priority = laneValue;
        requests.push(() => api.post(`${base}/prioritize`, { body: { priority: laneValue } }));
      } else if (this.lanes === "parent" && (laneValue ?? "") !== (this.items.find((x) => x.id === i.parent_id)?.key ?? "")) {
        requests.push(() => api.patch(base, { body: { parent: laneValue ?? "" } }));
      }
    }
    // Rank: right after the card above; else right before the card below.
    let after = afterKey;
    if (after === null && beforeKey) {
      const order = this.items.filter((x) => x.key !== key);
      after = order[order.findIndex((x) => x.key === beforeKey) - 1]?.key ?? "";
    }
    if (after !== null && this.can.includes("issue.rank")) {
      this.items.splice(this.items.indexOf(i), 1);
      this.items.splice(after ? this.items.findIndex((x) => x.key === after) + 1 : 0, 0, i);
      requests.push(() => api.post(`${base}/rank`, { body: { after } }));
    }
    this.renderBoard();
    this.focusCard(key);
    for (const r of requests) {
      try {
        await r();
      } catch (err) {
        globalThis.LDS?.toast?.({ intent: "danger", title: err.code === "transition_not_allowed" ? t("board.notAllowed", { key }) : errorMessage(err, i18n) });
        break;
      }
    }
    await this.refresh();
    this.focusCard(key);
  }

  // --- Commands and controls ---------------------------------------------------------

  /** ⌘K: move the current card to another column. */
  registerCommands() {
    const { commands, i18n } = this.context;
    const { t } = i18n;
    this.offCommands?.();
    const i = this.current && this.issue(this.current);
    const items = [];
    if (i && this.writable && this.can.includes("issue.transition")) {
      const here = this.columnOf(i.status_id);
      this.columns.forEach((c, ci) => {
        if (ci !== here && this.statusIn(i, ci)) {
          items.push({ label: t("board.moveTo", { key: i.key, status: c.name }), icon: "arrow-right", run: () => this.place(i.key, ci, null, null, null) });
        }
      });
    }
    this.offCommands = commands.set("board", { heading: t("planning.layouts.board"), items });
  }

  onChange(e) {
    if (e.target.matches("[data-lanes]")) {
      this.lanes = e.detail.value;
      storeLanes(this.laneId, this.lanes);
      this.renderBoard();
    } else if (e.target.matches("[data-scope]")) {
      const { router } = this.context;
      router.navigate(`/w/${this.ws}/p/${this.data.project.key}/board?iteration=${encodeURIComponent(e.detail.value)}`, { replace: true });
      this.scope = e.detail.value;
      this.renderBoard();
    }
  }

  onClick(e) {
    const action = e.target.closest?.("[data-action]")?.dataset.action;
    if (action === "create") return this.shadowRoot.querySelector("hzn-issue-create").show({ ws: this.ws, project: this.data.project.key });
    if (action === "open") return this.open(this.peeked);
    const card = e.target.closest?.(".card");
    if (card) {
      this.focusCard(card.dataset.key);
      this.peek(card.dataset.key);
    }
  }

  peek(key) {
    if (!key) return;
    const drawer = this.shadowRoot.querySelector("[data-peek]");
    this.peeked = key;
    this.markCards();
    let detail = drawer.querySelector("hzn-issue-detail");
    if (!detail) {
      detail = document.createElement("hzn-issue-detail");
      detail.setAttribute("mode", "peek");
      detail.setAttribute("ws", this.ws);
      detail.addEventListener("hzn-issue-loaded", (ev) => {
        drawer.querySelector("[data-peek-title]").textContent = ev.detail.parent ? `${ev.detail.parent} · ${ev.detail.key}` : ev.detail.key;
      });
      drawer.append(detail);
    }
    detail.setAttribute("issue", key);
    drawer.setAttribute("open", "");
    this.cardEls?.find((c) => c.dataset.key === key)?.focus();
  }

  open(key) {
    if (key) this.context.router.navigate(issueHref(this.ws, key));
  }

  /** Refreshes on pushed events about this project's issues and iterations. */
  onPush(m) {
    if (m.t !== "event" && m.t !== "resync") return;
    if (m.t === "event" && changesProjectData(m.d)) {
      forgetProjects();
      clearTimeout(this.timer);
      this.framed = false;
      this.timer = setTimeout(() => this.load(), 200);
      return;
    }
    const key = m.d?.aggregate?.key ?? "";
    const planning = m.d?.aggregate?.type === "iteration" && m.d?.payload?.project_id === this.data?.project.id;
    if (m.t === "resync" || planning || key.startsWith(`${this.data?.project.key}-`) || m.d?.type === "issue.moved") {
      clearTimeout(this.timer);
      this.timer = setTimeout(() => this.refresh(), 150);
    }
  }
}

customElements.define("hzn-board", HznBoard);
