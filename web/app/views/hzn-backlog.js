// <hzn-backlog>: /w/:ws/p/:key/backlog — sprint planning (wireframes:
// Backlog). The project's open iterations, active first, then the backlog:
// unfinished issues in no open iteration. Issues are dragged between them
// and up and down (rank); with the keyboard, Alt+↑/↓ moves the current
// issue within its bucket and Alt+Shift+↑/↓ to the bucket above or below,
// and ⌘K lists where it can go. Facilitators plan, edit, start, complete
// and delete iterations from here.

import "horizon/components/hzn-issue-create.js";
import "horizon/components/hzn-issue-detail.js";
import "horizon/components/hzn-iteration-dialog.js";
import {
  avatar, changesProjectData, forgetProjects, issueHref, issueStyles, priorityGlyph, projectData, statusGlyph,
} from "horizon/components/issue-ui.js";
import { dateSpan, isOpen, loadIterations, planningStyles, points, stateBadge, viewSwitch } from "horizon/components/planning-ui.js";
import { paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { busy, errorMessage } from "horizon/core/forms.js";

const BACKLOG = "";

const styles = sheet(`
  :host { display: flex; flex-direction: column; min-block-size: 100%; }
  header.bar { display: flex; align-items: center; justify-content: space-between; gap: var(--lds-space-3);
    min-block-size: 3.25rem; padding: 0 var(--lds-space-5); border-block-end: 1px solid var(--lds-color-border-default); }
  header.bar .actions { display: flex; gap: var(--lds-space-2); }
  .crumb { display: flex; align-items: center; gap: var(--lds-space-2); }
  .crumb strong { font-weight: var(--lds-font-weight-semibold); }
  .crumb lds-icon { color: var(--lds-color-fg-subtle); }
  .swatch { display: inline-block; inline-size: 0.625rem; block-size: 0.625rem; border-radius: var(--lds-radius-xs); }
  .toolbar { display: flex; align-items: center; gap: var(--lds-space-3); padding: var(--lds-space-2) var(--lds-space-5);
    border-block-end: 1px solid var(--lds-color-border-subtle); }
  .toolbar .grow { flex: 1; }
  .velocity { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  .velocity strong { color: var(--lds-color-fg-default); font-variant-numeric: tabular-nums; }
  .buckets { display: grid; gap: var(--lds-space-4); padding: var(--lds-space-4) var(--lds-space-5) var(--lds-space-8); }
  section.bucket { border: 1px solid var(--lds-color-border-default); border-radius: var(--lds-radius-md); background: var(--lds-color-bg-surface); }
  section.bucket[data-state="active"] { border-color: var(--lds-color-accent-border); }
  .head { display: flex; align-items: center; gap: var(--lds-space-3); padding: var(--lds-space-2) var(--lds-space-3);
    border-block-end: 1px solid var(--lds-color-border-subtle); }
  .head .name { font-weight: var(--lds-font-weight-semibold); }
  .head .dates, .head .summary { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); white-space: nowrap; }
  .head .grow { flex: 1; }
  .head lds-progress { inline-size: 6rem; }
  .toggle { display: inline-flex; padding: 0; border: 0; background: none; color: var(--lds-color-fg-muted); cursor: pointer; }
  section.bucket[data-collapsed] .toggle lds-icon { transform: rotate(-90deg); }
  section.bucket[data-collapsed] .rows, section.bucket[data-collapsed] .goal { display: none; }
  .goal { margin: 0; padding: var(--lds-space-2) var(--lds-space-4) 0; color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  .rows { min-block-size: 2.5rem; padding-block: var(--lds-space-1); }
  .rows.drop-end { box-shadow: inset 0 -2px 0 var(--lds-color-accent-fg); }
  .row { display: grid; grid-template-columns: 0.75rem 1.25rem 4.5rem 1.25rem minmax(0, 1fr) auto 2.5rem 1.75rem;
    align-items: center; gap: var(--lds-space-3); block-size: 2.25rem; padding: 0 var(--lds-space-3); cursor: pointer; outline: none; }
  .row:hover { background: var(--lds-color-bg-subtle); }
  .row:focus-visible, .row[aria-current="true"] { background: var(--lds-color-accent-subtle); }
  .row.drop-before { box-shadow: inset 0 2px 0 var(--lds-color-accent-fg); }
  .row.dragging { opacity: 0.5; }
  .handle { color: var(--lds-color-fg-subtle); cursor: grab; font-size: var(--lds-font-size-xs); }
  .row:not([draggable="true"]) .handle { visibility: hidden; }
  .title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .num { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); text-align: end; font-variant-numeric: tabular-nums; }
  .none { padding: var(--lds-space-3) var(--lds-space-4); color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  .empty { padding: var(--lds-space-12) var(--lds-space-6); }
  lds-drawer { --lds-drawer-width: min(34rem, 100vw); }
  [slot="header"] { display: flex; align-items: center; gap: var(--lds-space-2); font-size: var(--lds-font-size-sm); }
  .confirm { display: flex; justify-content: flex-end; gap: var(--lds-space-3); }
  @media (max-width: 48rem) {
    .row { grid-template-columns: 0.75rem 1.25rem 4.5rem 1.25rem minmax(0, 1fr) 1.75rem; }
    .row lds-badge, .row .num { display: none; }
  }
`);

class HznBacklog extends HTMLElement {
  static observedAttributes = ["ws", "key"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    this.collapsed = new Set();
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [issueStyles, planningStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("dblclick", (e) => {
      const row = e.target.closest?.(".row");
      if (row) this.open(row.dataset.key);
    });
    root.addEventListener("dragstart", (e) => this.onDragStart(e));
    root.addEventListener("dragover", (e) => this.onDragOver(e));
    root.addEventListener("dragleave", (e) => e.target === e.currentTarget && this.clearDrop());
    root.addEventListener("drop", (e) => this.onDrop(e));
    root.addEventListener("dragend", () => this.endDrag());
    const { shortcuts, socket } = this.context;
    this.offScope = shortcuts.pushScope("backlog");
    const key = (id, keys, description, handler) => shortcuts.register({ id: `backlog-${id}`, keys, scope: "backlog", description, handler });
    this.offKeys = [
      key("down", "j", "shortcut.next", () => this.step(1)),
      key("up", "k", "shortcut.previous", () => this.step(-1)),
      key("peek", "enter", "shortcut.peek", () => this.peek(this.current)),
      key("open", "o", "shortcut.open", () => this.current && this.open(this.current)),
      key("rank-up", "alt+arrowup", "shortcut.rankUp", () => this.nudge(-1)),
      key("rank-down", "alt+arrowdown", "shortcut.rankDown", () => this.nudge(1)),
      key("bucket-up", "alt+shift+arrowup", "shortcut.bucketUp", () => this.shift(-1)),
      key("bucket-down", "alt+shift+arrowdown", "shortcut.bucketDown", () => this.shift(1)),
    ];
    this.offSocket = socket.on((m) => this.onPush(m));
    this.onChanged = () => this.refresh();
    document.addEventListener("hzn-issue-changed", this.onChanged);
    document.addEventListener("hzn-iteration-changed", this.onChanged);
    this.load();
  }

  disconnectedCallback() {
    this.offScope?.();
    for (const off of this.offKeys ?? []) off();
    this.offSocket?.();
    this.offCommands?.();
    document.removeEventListener("hzn-issue-changed", this.onChanged);
    document.removeEventListener("hzn-iteration-changed", this.onChanged);
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

  /** Whether the caller moves issues between iterations, and ranks them. */
  get canPlan() {
    return this.can.includes("iteration.manage") && !this.data.project.archived_at;
  }

  get canRank() {
    return this.can.includes("issue.rank") && !this.data.project.archived_at;
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
      router.navigate(`/w/${this.ws}/p/${this.data.project.key}/backlog`, { replace: true }); // a former key
      return;
    }
    this.renderFrame();
    await this.refresh();
  }

  /** Fetches the iterations and every unarchived issue, then renders. */
  async refresh() {
    if (!this.data) return;
    const seq = (this.seq = (this.seq ?? 0) + 1); // a later refresh wins over this one
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
      if (seq !== this.seq) return;
      this.list = list;
      this.items = items;
    } catch (err) {
      if (seq !== this.seq) return;
      this.shadowRoot.querySelector("[data-buckets]").innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    this.renderBuckets();
  }

  /** The buckets in display order: open iterations, then the backlog. */
  buckets() {
    const open = this.list.items.filter(isOpen);
    const ids = new Set(open.map((it) => it.id));
    const unfinished = (i) => {
      const o = this.data.statuses.get(i.status_id)?.outcome;
      return o !== "completed" && o !== "abandoned";
    };
    return [
      ...open.map((it) => ({ id: it.id, iteration: it, items: this.items.filter((i) => i.iteration_id === it.id) })),
      { id: BACKLOG, iteration: null, items: this.items.filter((i) => !ids.has(i.iteration_id) && unfinished(i)) },
    ];
  }

  renderFrame() {
    const { t } = this.context.i18n;
    const p = this.data.project;
    this.shadowRoot.innerHTML = String(html`
      <header class="bar">
        <div class="crumb">
          <span class="swatch" data-colour="${p.colour}"></span><strong>${p.name}</strong>
          <lds-icon name="chevron-right" size="sm"></lds-icon><span>${t("planning.backlog")}</span>
        </div>
        <div class="actions">
          ${this.canPlan ? html`<lds-button variant="secondary" size="sm" data-action="plan"><lds-icon slot="start" name="calendar" size="sm"></lds-icon>${t("iterations.plan")}</lds-button>` : ""}
          ${p.can.includes("issue.create") && !p.archived_at ? html`<lds-button variant="primary" size="sm" data-action="create"><lds-icon slot="start" name="plus" size="sm"></lds-icon>${t("issues.new")}</lds-button>` : ""}
        </div>
      </header>
      ${p.archived_at ? html`<lds-alert intent="warning">${t("projects.isArchived")}</lds-alert>` : ""}
      <div class="toolbar">${viewSwitch(t, this.ws, p.key, "backlog")}<span class="grow"></span><span class="velocity" data-velocity></span></div>
      <div class="buckets" data-buckets><lds-skeleton></lds-skeleton></div>
      <lds-drawer non-modal size="md" data-peek>
        <span slot="header" data-peek-title></span>
        <lds-button slot="actions" variant="ghost" size="sm" data-action="open" label="${t("issues.openPage")}"><lds-icon name="external-link" size="sm"></lds-icon></lds-button>
      </lds-drawer>
      <lds-modal size="sm" data-confirm>
        <span slot="header" data-confirm-title></span>
        <p data-confirm-body></p>
        <div slot="footer" class="confirm">
          <lds-button variant="ghost" data-action="cancel">${t("tokens.cancel")}</lds-button>
          <lds-button variant="danger" data-action="confirm-delete">${t("iterations.delete")}</lds-button>
        </div>
      </lds-modal>
      <hzn-issue-create></hzn-issue-create>
      <hzn-iteration-dialog></hzn-iteration-dialog>`);
    paintSwatches(this.shadowRoot);
    this.shadowRoot.querySelector("[data-peek]").addEventListener("lds-close", () => {
      this.peeked = null;
      this.markRows();
    });
  }

  renderBuckets() {
    const { i18n } = this.context;
    const { t } = i18n;
    const velocity = this.list.velocity;
    this.shadowRoot.querySelector("[data-velocity]").innerHTML = velocity > 0
      ? String(html`${t("planning.velocity")} <strong>${t("planning.pts", { points: points(velocity, i18n) })}</strong>`)
      : "";
    const box = this.shadowRoot.querySelector("[data-buckets]");
    box.innerHTML = String(html`${this.buckets().map((b) => this.bucket(b))}`);
    paintSwatches(box);
    this.rows = [...box.querySelectorAll(".row")];
    if (!this.rows.some((r) => r.dataset.key === this.current)) this.current = null;
    this.markRows();
    this.registerCommands();
  }

  bucket(b) {
    const { i18n } = this.context;
    const { t } = i18n;
    const it = b.iteration;
    const name = it ? it.name : t("planning.backlog");
    const total = b.items.reduce((n, i) => n + this.weight(i), 0);
    const done = b.items.filter((i) => this.data.statuses.get(i.status_id)?.outcome === "completed").length;
    const summary = [t("planning.issues", { count: b.items.length }), t("planning.pts", { points: points(total, i18n) })];
    if (it && done) summary.push(t("planning.done", { count: done }));
    const velocity = this.list.velocity;
    const load = it && velocity > 0 ? Math.round((total / velocity) * 100) : null;
    const draggable = this.canPlan || this.canRank;
    return html`
      <section class="bucket" aria-label="${name}" data-bucket="${b.id}" data-state="${it?.state ?? "backlog"}" ${this.collapsed.has(b.id) ? html`data-collapsed` : ""}>
        <div class="head">
          <button type="button" class="toggle" data-action="toggle" aria-expanded="${String(!this.collapsed.has(b.id))}" aria-label="${t("planning.toggle", { name })}"><lds-icon name="chevron-down" size="sm"></lds-icon></button>
          <span class="name">${name}</span>
          ${it ? stateBadge(t, it.state) : ""}
          ${it ? html`<span class="dates">${dateSpan(it, i18n)}</span>` : ""}
          <span class="grow"></span>
          <span class="summary">${summary.join(" · ")}</span>
          ${load !== null ? html`<lds-progress value="${Math.min(load, 100)}" max="100" label="${t("planning.load", { percent: load })}" title="${t("planning.load", { percent: load })}"></lds-progress>` : ""}
          ${it && this.canPlan ? this.bucketActions(it) : ""}
        </div>
        ${it?.goal ? html`<p class="goal">${t("iterations.goalLine", { goal: it.goal })}</p>` : ""}
        <div class="rows" role="list" data-rows>
          ${b.items.length ? b.items.map((i) => html`
            <div class="row" role="listitem" tabindex="-1" data-key="${i.key}" aria-label="${i.key} ${i.title}" ${draggable ? html`draggable="true"` : ""}>
              <span class="handle" aria-hidden="true">⋮⋮</span>
              ${priorityGlyph(i.priority, t)}
              <span class="key">${i.key}</span>
              ${statusGlyph(this.data, i.status_id)}
              <span class="title">${i.title}</span>
              <lds-badge size="sm">${this.data.types.get(i.type_id)?.name ?? ""}</lds-badge>
              <span class="num">${i.estimate ?? ""}</span>
              <span>${avatar(this.data, i.assignee_id)}</span>
            </div>`) : html`<div class="none">${t(it ? "planning.emptyIteration" : "planning.emptyBacklog")}</div>`}
        </div>
      </section>`;
  }

  bucketActions(it) {
    const { t } = this.context.i18n;
    const active = this.list.items.filter((x) => x.state === "active").length;
    const canStart = it.state === "scheduled" && active < this.list.settings.max_active_iterations.value;
    return html`
      ${it.state === "active" ? html`<lds-button size="sm" variant="secondary" data-action="complete" data-id="${it.id}">${t("iterations.complete")}</lds-button>` : ""}
      ${canStart ? html`<lds-button size="sm" variant="primary" data-action="start" data-id="${it.id}">${t("iterations.start")}</lds-button>` : ""}
      <lds-menu placement="bottom-end">
        <lds-button slot="trigger" variant="ghost" size="sm" label="${t("iterations.more", { name: it.name })}"><lds-icon name="more-horizontal"></lds-icon></lds-button>
        <lds-menu-item data-action="edit" data-id="${it.id}"><lds-icon slot="start" name="pencil"></lds-icon>${t("iterations.edit")}</lds-menu-item>
        ${it.state !== "active" ? html`<lds-menu-item data-action="delete" data-id="${it.id}"><lds-icon slot="start" name="trash-2"></lds-icon>${t("iterations.delete")}</lds-menu-item>` : ""}
      </lds-menu>`;
  }

  /** The points an issue's estimate counts for. */
  weight(i) {
    if (i.estimate == null) return 0;
    const v = this.data.config.estimation.values.find((x) => x.label === i.estimate);
    return v ? v.weight : Number(i.estimate) || 0;
  }

  markRows() {
    for (const r of this.rows ?? []) {
      r.setAttribute("aria-current", String(r.dataset.key === (this.peeked ?? this.current)));
      r.tabIndex = r.dataset.key === this.current ? 0 : -1;
    }
  }

  /** Moves the current row by delta (j/k), across buckets. */
  step(delta) {
    const rows = (this.rows ?? []).filter((r) => !r.closest("[data-collapsed]"));
    if (!rows.length) return;
    const i = rows.findIndex((r) => r.dataset.key === this.current);
    const next = rows[Math.max(0, Math.min(rows.length - 1, i < 0 ? 0 : i + delta))];
    this.focusRow(next.dataset.key);
    if (this.peeked) this.peek(this.current);
  }

  focusRow(key) {
    this.current = key;
    this.markRows();
    const row = this.rows?.find((r) => r.dataset.key === key);
    row?.focus();
    row?.scrollIntoView({ block: "nearest" });
    this.registerCommands();
  }

  /** Where an issue sits: its bucket and index. */
  locate(key) {
    const buckets = this.buckets();
    for (const [bi, b] of buckets.entries()) {
      const i = b.items.findIndex((x) => x.key === key);
      if (i >= 0) return { buckets, bi, i };
    }
    return null;
  }

  /** Alt+↑/↓: one place up or down in the bucket. */
  nudge(delta) {
    const at = this.current && this.locate(this.current);
    if (!at || !this.canRank) return;
    const rows = at.buckets[at.bi].items;
    const j = at.i + delta;
    if (j < 0 || j >= rows.length) return;
    // Up: before the previous one; down: after the next one.
    this.place(this.current, at.buckets[at.bi].id, delta < 0 ? rows[j - 1]?.key ?? null : rows[j].key, delta < 0 ? rows[j].key : null);
  }

  /** Alt+Shift+↑/↓: to the end of the bucket above or below. */
  shift(delta) {
    const at = this.current && this.locate(this.current);
    if (!at || !this.canPlan) return;
    const target = at.buckets[at.bi + delta];
    if (!target) return;
    this.place(this.current, target.id, target.items.at(-1)?.key ?? null, null);
  }

  // --- Drag and drop -----------------------------------------------------------------

  onDragStart(e) {
    const row = e.target.closest?.(".row[draggable]");
    if (!row) return;
    this.dragKey = row.dataset.key;
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", row.dataset.key);
    row.classList.add("dragging");
  }

  /** The drop position under the pointer: the bucket, and the rows around it. */
  dropAt(e) {
    const section = e.target.closest?.("[data-bucket]");
    if (!section) return null;
    const rows = [...section.querySelectorAll(".row")].filter((r) => r.dataset.key !== this.dragKey);
    const before = rows.find((r) => {
      const box = r.getBoundingClientRect();
      return e.clientY < box.top + box.height / 2;
    });
    const i = before ? rows.indexOf(before) : rows.length;
    return { section, bucket: section.dataset.bucket, before: before ?? null, after: rows[i - 1] ?? null };
  }

  onDragOver(e) {
    if (!this.dragKey) return;
    const at = this.dropAt(e);
    if (!at) return;
    const from = this.locate(this.dragKey);
    const moving = from && from.buckets[from.bi].id !== at.bucket;
    if (moving ? !this.canPlan : !this.canRank) return;
    e.preventDefault();
    this.clearDrop();
    if (at.before) at.before.classList.add("drop-before");
    else at.section.querySelector("[data-rows]").classList.add("drop-end");
  }

  onDrop(e) {
    if (!this.dragKey) return;
    const at = this.dropAt(e);
    const key = this.dragKey;
    this.endDrag();
    if (!at) return;
    e.preventDefault();
    this.place(key, at.bucket, at.after?.dataset.key ?? null, at.before?.dataset.key ?? null);
  }

  clearDrop() {
    for (const el of this.shadowRoot.querySelectorAll(".drop-before, .drop-end")) el.classList.remove("drop-before", "drop-end");
  }

  endDrag() {
    this.clearDrop();
    this.shadowRoot.querySelector(".dragging")?.classList.remove("dragging");
    this.dragKey = null;
  }

  /**
   * Puts an issue in a bucket between two of its rows (keys; null at an
   * end): loads it there if it comes from another bucket, then ranks it.
   */
  async place(key, bucket, afterKey, beforeKey) {
    const { api, i18n } = this.context;
    const at = this.locate(key);
    if (!at) return;
    const from = at.buckets[at.bi];
    const issue = from.items[at.i];
    const base = `/w/${encodeURIComponent(this.ws)}/projects/${this.data.project.key}/iterations`;
    // Rank: right after the row above; else right before the row below,
    // that is after whatever precedes it in the project's order.
    let after = afterKey;
    if (after === null && beforeKey) {
      const order = this.items.filter((i) => i.key !== key);
      after = order[order.findIndex((i) => i.key === beforeKey) - 1]?.key ?? "";
    }
    const moving = from.id !== bucket;
    const ranking = after !== null && this.canRank;
    // Optimistic: show it there now; the reload settles it.
    if (moving) issue.iteration_id = bucket || null;
    if (ranking) {
      this.items.splice(this.items.indexOf(issue), 1);
      const pos = after ? this.items.findIndex((i) => i.key === after) + 1 : 0;
      this.items.splice(pos, 0, issue);
    }
    this.renderBuckets();
    this.focusRow(key);
    try {
      if (moving && bucket) await api.post(`${base}/${bucket}/issues`, { body: { add: [key] } });
      else if (moving && from.iteration && isOpen(from.iteration)) await api.post(`${base}/${from.id}/issues`, { body: { remove: [key] } });
      if (ranking) await api.post(`/w/${encodeURIComponent(this.ws)}/issues/${encodeURIComponent(key)}/rank`, { body: { after } });
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
    await this.refresh();
    this.focusRow(key);
  }

  // --- Commands ----------------------------------------------------------------------

  /** ⌘K: plan an iteration; move the current issue to another bucket. */
  registerCommands() {
    const { commands, i18n } = this.context;
    const { t } = i18n;
    this.offCommands?.();
    const items = [];
    if (this.canPlan) items.push({ label: t("iterations.plan"), icon: "calendar", run: () => this.plan() });
    const at = this.current && this.canPlan && this.locate(this.current);
    if (at) {
      for (const b of at.buckets) {
        if (b.id === at.buckets[at.bi].id) continue;
        const name = b.iteration?.name ?? t("planning.backlog");
        items.push({ label: t("planning.moveTo", { key: this.current, name }), icon: "arrow-right", run: () => this.place(this.current, b.id, b.items.at(-1)?.key ?? null, null) });
      }
    }
    this.offCommands = commands.set("backlog", { heading: t("planning.backlog"), items });
  }

  // --- Actions -----------------------------------------------------------------------

  onClick(e) {
    const el = e.target.closest?.("[data-action]");
    const action = el?.dataset.action;
    const it = el?.dataset.id && this.list?.items.find((x) => x.id === el.dataset.id);
    switch (action) {
      case "create":
        return this.shadowRoot.querySelector("hzn-issue-create").show({ ws: this.ws, project: this.data.project.key });
      case "plan":
        return this.plan();
      case "edit":
        return this.dialog().edit({ ws: this.ws, project: this.data.project.key, iteration: it });
      case "complete":
        return this.dialog().complete({ ws: this.ws, project: this.data.project.key, iteration: it, list: this.list });
      case "start":
        return this.start(el, it);
      case "delete":
        return this.confirmDelete(it);
      case "confirm-delete":
        return this.delete(el);
      case "cancel":
        return this.shadowRoot.querySelector("[data-confirm]").removeAttribute("open");
      case "open":
        return this.open(this.peeked);
      case "toggle": {
        const id = el.closest("[data-bucket]").dataset.bucket;
        if (this.collapsed.has(id)) this.collapsed.delete(id);
        else this.collapsed.add(id);
        return this.renderBuckets();
      }
    }
    const row = e.target.closest?.(".row");
    if (row) {
      this.current = row.dataset.key;
      this.peek(row.dataset.key);
      this.registerCommands();
    }
  }

  dialog() {
    return this.shadowRoot.querySelector("hzn-iteration-dialog");
  }

  plan() {
    if (this.canPlan) this.dialog().plan({ ws: this.ws, project: this.data.project.key, next: this.list?.next });
  }

  async start(button, it) {
    const { api, i18n } = this.context;
    try {
      // No If-Match: loading issues bumps the version, and starting doesn't depend on them.
      await busy(button, () => api.post(`/w/${encodeURIComponent(this.ws)}/projects/${this.data.project.key}/iterations/${it.id}/start`));
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      return;
    }
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("iterations.started", { name: it.name }) });
    this.refresh();
  }

  confirmDelete(it) {
    const { t } = this.context.i18n;
    this.deleting = it;
    const modal = this.shadowRoot.querySelector("[data-confirm]");
    modal.querySelector("[data-confirm-title]").textContent = t("iterations.deleteTitle", { name: it.name });
    modal.querySelector("[data-confirm-body]").textContent = t("iterations.deleteBody", { count: it.stats.issues });
    modal.setAttribute("open", "");
  }

  async delete(button) {
    const { api, i18n } = this.context;
    const it = this.deleting;
    try {
      await busy(button, () => api.delete(`/w/${encodeURIComponent(this.ws)}/projects/${this.data.project.key}/iterations/${it.id}`));
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      return;
    }
    this.shadowRoot.querySelector("[data-confirm]").removeAttribute("open");
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("iterations.deleted", { name: it.name }) });
    this.refresh();
  }

  peek(key) {
    if (!key) return;
    const drawer = this.shadowRoot.querySelector("[data-peek]");
    this.peeked = key;
    this.markRows();
    let detail = drawer.querySelector("hzn-issue-detail");
    if (!detail) {
      detail = document.createElement("hzn-issue-detail");
      detail.setAttribute("mode", "peek");
      detail.setAttribute("ws", this.ws);
      detail.addEventListener("hzn-issue-loaded", (e) => {
        drawer.querySelector("[data-peek-title]").textContent = e.detail.parent ? `${e.detail.parent} · ${e.detail.key}` : e.detail.key;
      });
      drawer.append(detail);
    }
    detail.setAttribute("issue", key);
    drawer.setAttribute("open", "");
    this.rows?.find((r) => r.dataset.key === key)?.focus();
  }

  open(key) {
    if (key) this.context.router.navigate(issueHref(this.ws, key));
  }

  /** Refreshes on pushed events about this project's issues and plans. */
  onPush(m) {
    if (m.t !== "event" && m.t !== "resync") return;
    if (m.t === "event" && changesProjectData(m.d)) {
      forgetProjects();
      clearTimeout(this.timer);
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

customElements.define("hzn-backlog", HznBacklog);
