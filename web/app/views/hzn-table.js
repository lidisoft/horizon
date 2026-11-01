// <hzn-table>: /w/:ws/p/:key/table — a project's issues as a grid of
// fields (wireframes: Table). Columns are picked from the built-in fields
// and the project's custom fields; headers sort (the query's sort keys).
// A saved table view keeps its columns (display.columns) and sort; without
// one they are remembered per project in this browser, and the sort is in
// the address (?sort=field:dir).
//
// Cells edit in place: Enter or a double click opens the editor, Enter or a
// choice saves (If-Match), Escape cancels. Arrow keys move between cells,
// Space selects the row. Selected rows take bulk changes: status (by name,
// in each issue's own workflow), priority, assignee, archive.

import "horizon/components/hzn-view-bar.js";
import {
  PRIORITIES, avatar, changesProjectData, forgetProjects, issueHref, issueStyles, labelChips, personName, priorityGlyph, projectData, shortDate,
  statusGlyph,
} from "horizon/components/issue-ui.js";
import { fetchIssues, loadIterations, loadReleases, planningStyles, viewSwitch } from "horizon/components/planning-ui.js";
import { paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, keepOpenOnInsidePress, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

/** Built-in columns: the sort key they map to, and how they edit. */
const BUILTIN = [
  { id: "status", sort: "status", edit: "status" },
  { id: "type" },
  { id: "priority", sort: "priority", edit: "priority" },
  { id: "assignee", edit: "assignee" },
  { id: "labels" },
  { id: "estimate", sort: "estimate", edit: "estimate" },
  { id: "iteration" },
  { id: "release" },
  { id: "start_date", sort: "start_date", edit: "date" },
  { id: "due_date", sort: "due_date", edit: "date" },
  { id: "created", sort: "created" },
  { id: "updated", sort: "updated" },
];
const DEFAULT_COLUMNS = ["status", "priority", "assignee", "labels", "estimate", "due_date"];
/** Custom field types the query sorts and the grid edits. */
const CF_SORTS = new Set(["text", "number", "date", "select", "url"]);
const CF_EDITS = new Set(["text", "number", "date", "select", "url", "checkbox"]);

const styles = sheet(`
  :host { display: flex; flex-direction: column; min-block-size: 100%; }
  header.bar { display: flex; align-items: center; justify-content: space-between; gap: var(--lds-space-3);
    min-block-size: 3.25rem; padding: 0 var(--lds-space-5); border-block-end: 1px solid var(--lds-color-border-default); }
  .crumb { display: flex; align-items: center; gap: var(--lds-space-2); }
  .crumb strong { font-weight: var(--lds-font-weight-semibold); }
  .crumb lds-icon { color: var(--lds-color-fg-subtle); }
  .swatch { display: inline-block; inline-size: 0.625rem; block-size: 0.625rem; border-radius: var(--lds-radius-xs); }
  .toolbar { display: flex; align-items: center; gap: var(--lds-space-2); padding: var(--lds-space-2) var(--lds-space-5);
    border-block-end: 1px solid var(--lds-color-border-subtle); flex-wrap: wrap; }
  .columns { display: grid; gap: var(--lds-space-2); padding: var(--lds-space-3); max-block-size: 24rem; overflow: auto; }
  .bulk { display: flex; align-items: center; gap: var(--lds-space-3); padding: var(--lds-space-2) var(--lds-space-5);
    background: var(--lds-color-accent-subtle); border-block-end: 1px solid var(--lds-color-border-subtle); }
  .bulk[hidden] { display: none; }
  .bulk lds-select { min-inline-size: 9rem; }
  .scroll { flex: 1; overflow: auto; }
  table { border-collapse: separate; border-spacing: 0; min-inline-size: 100%; font-size: var(--lds-font-size-sm); }
  th, td { padding: 0 var(--lds-space-3); block-size: 2.5rem; border-block-end: 1px solid var(--lds-color-border-subtle);
    text-align: start; white-space: nowrap; outline: none; }
  th { position: sticky; top: 0; z-index: 1; background: var(--lds-color-bg-canvas); font-weight: var(--lds-font-weight-semibold);
    color: var(--lds-color-fg-muted); border-block-end-color: var(--lds-color-border-default); }
  th button { all: unset; display: inline-flex; align-items: center; gap: var(--lds-space-1); cursor: pointer; }
  th button:focus-visible { outline: 2px solid var(--lds-color-accent-border); outline-offset: 2px; }
  td.title { max-inline-size: 28rem; overflow: hidden; text-overflow: ellipsis; }
  td .cell { display: inline-flex; align-items: center; gap: var(--lds-space-2); }
  td.num { text-align: end; font-variant-numeric: tabular-nums; }
  td.muted { color: var(--lds-color-fg-muted); }
  td[aria-readonly="false"] { cursor: cell; }
  td:focus-visible, td[data-active] { box-shadow: inset 0 0 0 2px var(--lds-color-accent-border); }
  td.editing { padding: 0 var(--lds-space-1); }
  td.editing lds-select, td.editing lds-input, td.editing lds-date-picker { min-inline-size: 10rem; }
  tr[aria-selected="true"] td { background: var(--lds-color-accent-subtle); }
  tbody tr:hover td { background: var(--lds-color-bg-subtle); }
  .pick { inline-size: 2.25rem; }
  .empty { padding: var(--lds-space-12) var(--lds-space-6); }
`);

function stored(id) {
  try {
    const v = JSON.parse(localStorage.getItem(`horizon.table.columns.${id}`) ?? "null");
    return Array.isArray(v) ? v : null;
  } catch {
    return null;
  }
}

function store(id, columns) {
  try {
    localStorage.setItem(`horizon.table.columns.${id}`, JSON.stringify(columns));
  } catch {
    // private mode: the choice lasts until the page closes
  }
}

/** ?sort=field:dir,field:dir */
export function parseSort(text) {
  return String(text ?? "").split(",").filter(Boolean).map((s) => {
    const [field, dir] = s.split(":");
    return { field, dir: dir === "desc" ? "desc" : "asc" };
  });
}

class HznTable extends HTMLElement {
  static observedAttributes = ["ws", "key"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [issueStyles, planningStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("dblclick", (e) => {
      const td = e.target.closest?.("td[data-col]");
      if (td) this.edit(td);
    });
    root.addEventListener("keydown", (e) => this.onKey(e));
    // Leaving an editor saves typed text and drops an untouched choice.
    root.addEventListener("focusout", () => setTimeout(() => {
      const ed = this.editing;
      if (!ed || ed.td.matches(":focus-within")) return;
      if (ed.kind === "input") this.saveInput();
      else this.cancelEdit();
    }));
    root.addEventListener("lds-change", (e) => this.onChange(e));
    root.addEventListener("hzn-view-change", () => {
      this.applyView();
      this.reload();
    });
    this.offSocket = this.context.socket.on((m) => this.onPush(m));
    this.onChanged = () => this.reload();
    document.addEventListener("hzn-issue-changed", this.onChanged);
    this.selected = new Set();
    this.load();
  }

  disconnectedCallback() {
    this.offSocket?.();
    document.removeEventListener("hzn-issue-changed", this.onChanged);
    clearTimeout(this.timer);
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.load();
  }

  get ws() {
    return this.getAttribute("ws");
  }

  get base() {
    return `/w/${encodeURIComponent(this.ws)}`;
  }

  get can() {
    return this.data?.project.can ?? [];
  }

  get writable() {
    return !this.data?.project.archived_at;
  }

  async load() {
    const { i18n, router } = this.context;
    const key = this.getAttribute("key");
    try {
      this.data = await projectData(this.context, this.ws, key);
      const [iterations, releases] = await Promise.all([
        loadIterations(this.context, this.ws, key).catch(() => ({ items: [] })),
        loadReleases(this.context, this.ws, key).catch(() => ({ items: [] })),
      ]);
      this.iterations = new Map(iterations.items.map((x) => [x.id, x]));
      this.releases = new Map(releases.items.map((x) => [x.id, x]));
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`
        <lds-empty-state class="empty"><lds-icon slot="icon" name="search"></lds-icon>
          <h2 slot="title">${err.status === 404 ? i18n.t("projects.notFound") : errorMessage(err, i18n)}</h2>
          <a href="${this.base}">${i18n.t("notFound.home")}</a>
        </lds-empty-state>`);
      return;
    }
    if (this.data.project.key !== key.toUpperCase()) {
      router.navigate(`${this.base}/p/${this.data.project.key}/table${location.search}`, { replace: true }); // a former key
      return;
    }
    this.storeId = `${this.ws}/${this.data.project.key}`;
    this.columns = stored(this.storeId) ?? DEFAULT_COLUMNS;
    this.sort = parseSort(router.current?.query.get("sort"));
    this.renderFrame();
    this.bar = this.shadowRoot.querySelector("hzn-view-bar");
    await this.bar.setup({
      ws: this.ws, data: this.data, layout: "table",
      display: () => ({ ...this.bar.view?.display, columns: this.columns }),
      sort: () => this.sort,
      changedDisplay: () => JSON.stringify(this.columns) !== JSON.stringify(this.bar.view?.display?.columns ?? this.columns),
    });
    if (this.bar.view) this.applyView(!router.current?.query.has("sort"));
    await this.reload();
  }

  /** A saved view's columns, and its sort unless the address has one. */
  applyView(withSort = true) {
    const v = this.bar?.view;
    if (!v) return;
    if (Array.isArray(v.display?.columns)) this.columns = v.display.columns.filter((c) => this.column(c));
    if (withSort) this.sort = v.query.sort ?? [];
  }

  /** Every column the project offers: built-in, then custom fields. */
  allColumns() {
    const { t } = this.context.i18n;
    const builtin = BUILTIN.map((c) => ({ ...c, label: t(`table.col.${c.id}`) }));
    const custom = [...this.data.fields.values()].sort((a, b) => a.position - b.position).map((f) => ({
      id: `cf:${f.id}`, label: f.name, field: f, sort: CF_SORTS.has(f.type) ? `cf:${f.id}` : null, edit: CF_EDITS.has(f.type) ? "field" : null,
    }));
    return [...builtin, ...custom];
  }

  column(id) {
    return this.allColumns().find((c) => c.id === id) ?? null;
  }

  async reload() {
    const seq = (this.seq = (this.seq ?? 0) + 1); // a later reload wins over this one
    const { i18n } = this.context;
    try {
      const items = await fetchIssues(this.context, this.ws, this.data.project.key, this.bar?.filter, this.sort);
      if (seq !== this.seq) return;
      this.items = items;
    } catch (err) {
      if (seq !== this.seq) return;
      this.shadowRoot.querySelector("[data-grid]").innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    const keys = new Set(this.items.map((i) => i.key));
    for (const k of this.selected) if (!keys.has(k)) this.selected.delete(k);
    this.renderGrid();
  }

  // --- Rendering ---------------------------------------------------------------------

  renderFrame() {
    const { t } = this.context.i18n;
    const p = this.data.project;
    this.shadowRoot.innerHTML = String(html`
      <header class="bar">
        <div class="crumb">
          <span class="swatch" data-colour="${p.colour}"></span><strong>${p.name}</strong>
          <lds-icon name="chevron-right" size="sm"></lds-icon><span>${t("planning.layouts.table")}</span>
        </div>
      </header>
      ${p.archived_at ? html`<lds-alert intent="warning">${t("projects.isArchived")}</lds-alert>` : ""}
      <div class="toolbar">
        ${viewSwitch(t, this.ws, p.key, "table")}
        <hzn-view-bar></hzn-view-bar>
        <lds-popover label="${t("table.columns")}" placement="bottom-end" data-columns>
          <lds-button slot="trigger" variant="ghost" size="sm"><lds-icon slot="start" name="sliders-horizontal" size="sm"></lds-icon>${t("table.columns")}</lds-button>
          <div class="columns" data-column-list></div>
        </lds-popover>
      </div>
      <div class="bulk" data-bulk hidden></div>
      <div class="scroll" data-grid><lds-skeleton></lds-skeleton></div>`);
    paintSwatches(this.shadowRoot);
    keepOpenOnInsidePress(this.shadowRoot.querySelector("[data-columns]"));
  }

  renderColumnList() {
    this.shadowRoot.querySelector("[data-column-list]").innerHTML = String(html`${this.allColumns().map((c) => html`
      <lds-switch data-toggle-column="${c.id}" ${this.columns.includes(c.id) ? html`checked` : ""}>${c.label}</lds-switch>`)}`);
  }

  renderGrid() {
    const { i18n } = this.context;
    const { t } = i18n;
    this.renderColumnList();
    this.renderBulk();
    const box = this.shadowRoot.querySelector("[data-grid]");
    // Whether a cell has the focus, asked before the cells are replaced.
    const focused = !!this.shadowRoot.activeElement?.matches?.("td[data-col]");
    if (!this.items.length) {
      box.innerHTML = String(html`
        <lds-empty-state class="empty"><lds-icon slot="icon" name="${this.bar?.filter ? "filter" : "table"}"></lds-icon>
          <h2 slot="title">${this.bar?.filter ? t("views.noMatch") : t("issues.empty")}</h2>
        </lds-empty-state>`);
      return;
    }
    const cols = [{ id: "key", label: t("table.col.key"), sort: "key" }, { id: "title", label: t("table.col.title"), sort: "title", edit: "title" },
      ...this.columns.map((id) => this.column(id)).filter(Boolean)];
    this.cols = cols;
    const all = this.items.length > 0 && this.items.every((i) => this.selected.has(i.key));
    box.innerHTML = String(html`
      <table role="grid" aria-label="${t("table.label")}" aria-rowcount="${this.items.length + 1}" aria-multiselectable="true">
        <thead><tr>
          <th class="pick"><lds-checkbox data-pick-all ${all ? html`checked` : ""} aria-label="${t("table.selectAll")}"></lds-checkbox></th>
          ${cols.map((c) => {
            const s = this.sort.find((x) => x.field === c.sort);
            const sort = s ? (s.dir === "desc" ? "descending" : "ascending") : "none";
            return html`<th scope="col" aria-sort="${sort}">${c.sort
              ? html`<button type="button" data-sort="${c.sort}">${c.label}${s ? html`<lds-icon name="${s.dir === "desc" ? "chevron-down" : "chevron-up"}" size="sm"></lds-icon>` : ""}</button>`
              : c.label}</th>`;
          })}
        </tr></thead>
        <tbody>${this.items.map((i) => html`
          <tr data-key="${i.key}" aria-selected="${String(this.selected.has(i.key))}">
            <td class="pick"><lds-checkbox data-pick="${i.key}" ${this.selected.has(i.key) ? html`checked` : ""} aria-label="${t("table.select", { key: i.key })}"></lds-checkbox></td>
            ${cols.map((c) => this.cell(i, c))}
          </tr>`)}
        </tbody>
      </table>`);
    paintSwatches(box);
    this.restoreActive(focused);
  }

  /** A cell: its display, and whether it edits. */
  cell(i, c) {
    const { i18n } = this.context;
    const { t } = i18n;
    const editable = this.writable && !i.archived_at && !!c.edit && (c.edit !== "estimate" || this.data.config.estimation.kind !== "none");
    const attrs = html`role="gridcell" tabindex="-1" data-col="${c.id}" aria-readonly="${String(!editable)}"`;
    switch (c.id) {
      case "key":
        return html`<td ${attrs}><a class="key" href="${issueHref(this.ws, i.key)}" tabindex="-1">${i.key}</a></td>`;
      case "title":
        return html`<td class="title" ${attrs} title="${i.title}">${i.title}</td>`;
      case "status":
        return html`<td ${attrs}><span class="cell">${statusGlyph(this.data, i.status_id)}${this.data.statuses.get(i.status_id)?.name ?? ""}</span></td>`;
      case "type":
        return html`<td ${attrs}>${this.data.types.get(i.type_id)?.name ?? ""}</td>`;
      case "priority":
        return html`<td ${attrs}><span class="cell">${priorityGlyph(i.priority, t)}${t(`priority.${i.priority}`)}</span></td>`;
      case "assignee":
        return html`<td ${attrs}><span class="cell">${avatar(this.data, i.assignee_id)}${i.assignee_id ? personName(this.data, i.assignee_id, t) : ""}</span></td>`;
      case "labels":
        return html`<td ${attrs}><span class="cell">${labelChips(this.data, i.labels)}</span></td>`;
      case "estimate":
        return html`<td class="num" ${attrs}>${i.estimate ?? ""}</td>`;
      case "iteration":
        return html`<td ${attrs}>${this.iterations.get(i.iteration_id)?.name ?? ""}</td>`;
      case "release":
        return html`<td ${attrs}>${this.releases.get(i.release_id)?.name ?? ""}</td>`;
      case "start_date":
      case "due_date":
        return html`<td class="muted" ${attrs}>${shortDate(i[c.id], i18n)}</td>`;
      case "created":
      case "updated":
        return html`<td class="muted" ${attrs}>${shortDate(i[`${c.id}_at`]?.slice(0, 10), i18n)}</td>`;
    }
    return html`<td ${c.field?.type === "number" ? html`class="num"` : ""} ${attrs}>${this.fieldText(c.field, i.fields?.[c.field.id])}</td>`;
  }

  fieldText(f, v) {
    if (v == null || v === "") return "";
    switch (f.type) {
      case "select":
        return f.options?.find((o) => o.id === v)?.name ?? "";
      case "multi_select":
        return (v ?? []).map((id) => f.options?.find((o) => o.id === id)?.name).filter(Boolean).join(", ");
      case "user":
        return personName(this.data, v, this.context.i18n.t);
      case "checkbox":
        return v ? "✓" : "";
      case "date":
        return shortDate(v, this.context.i18n);
    }
    return String(v);
  }

  renderBulk() {
    const { t } = this.context.i18n;
    const bar = this.shadowRoot.querySelector("[data-bulk]");
    bar.hidden = this.selected.size === 0;
    if (bar.hidden) return;
    const names = [...new Set([...this.data.statuses.values()].sort((a, b) => a.position - b.position).map((s) => s.name))];
    bar.innerHTML = String(html`
      <strong>${t("table.selected", { count: this.selected.size })}</strong>
      ${this.can.includes("issue.transition") ? html`<lds-select size="sm" data-bulk-action="status" value="" label="${t("table.setStatus")}">
        <option value="">${t("table.setStatus")}</option>${names.map((n) => html`<option value="${n}">${n}</option>`)}</lds-select>` : ""}
      ${this.can.includes("issue.edit") ? html`<lds-select size="sm" data-bulk-action="priority" value="" label="${t("table.setPriority")}">
        <option value="">${t("table.setPriority")}</option>${PRIORITIES.map((p) => html`<option value="${p}">${t(`priority.${p}`)}</option>`)}</lds-select>` : ""}
      ${this.can.includes("issue.assign") ? html`<lds-select size="sm" data-bulk-action="assignee" value="" label="${t("table.setAssignee")}">
        <option value="">${t("table.setAssignee")}</option><option value="-">${t("issue.unassigned")}</option>
        ${this.data.peopleList.map((p) => html`<option value="${p.id}">${p.name}</option>`)}</lds-select>` : ""}
      ${this.can.includes("issue.archive") ? html`<lds-button size="sm" variant="secondary" data-action="bulk-archive"><lds-icon slot="start" name="archive" size="sm"></lds-icon>${t("table.archive")}</lds-button>` : ""}
      <lds-button size="sm" variant="ghost" data-action="clear-selection">${t("table.clearSelection")}</lds-button>`);
  }

  // --- Navigation --------------------------------------------------------------------

  cells(tr) {
    return [...tr.querySelectorAll("td[data-col]")];
  }

  activate(td, focus = true) {
    for (const el of this.shadowRoot.querySelectorAll("td[data-active]")) {
      el.removeAttribute("data-active");
      el.tabIndex = -1;
    }
    if (!td) return;
    td.setAttribute("data-active", "");
    td.tabIndex = 0;
    this.active = { key: td.closest("tr").dataset.key, col: td.dataset.col };
    if (focus) {
      td.focus();
      td.scrollIntoView({ block: "nearest", inline: "nearest" });
    }
  }

  /** After a re-render, the active cell is where it was (by issue and column). */
  restoreActive(focused) {
    const a = this.active;
    const td = a && this.shadowRoot.querySelector(`tr[data-key="${CSS.escape(a.key)}"] td[data-col="${CSS.escape(a.col)}"]`);
    const first = this.shadowRoot.querySelector("tbody td[data-col]");
    const had = focused || this.refocus;
    this.refocus = false;
    this.activate(td ?? first, !!(td && had));
  }

  onKey(e) {
    if (this.editing) {
      if (e.key === "Escape") {
        e.preventDefault();
        e.stopPropagation();
        this.cancelEdit();
      } else if (e.key === "Enter" && this.editing.kind === "input") {
        e.preventDefault();
        e.stopPropagation();
        this.saveInput();
      }
      return;
    }
    const td = e.composedPath().find((n) => n.matches?.("td[data-col]"));
    if (!td) return;
    const tr = td.closest("tr");
    const cells = this.cells(tr);
    const at = cells.indexOf(td);
    let next = null;
    switch (e.key) {
      case "ArrowRight":
        next = cells[at + 1];
        break;
      case "ArrowLeft":
        next = cells[at - 1];
        break;
      case "ArrowDown":
        next = tr.nextElementSibling && this.cells(tr.nextElementSibling)[at];
        break;
      case "ArrowUp":
        next = tr.previousElementSibling && this.cells(tr.previousElementSibling)[at];
        break;
      case "Enter":
        e.preventDefault();
        if (td.dataset.col === "key") this.context.router.navigate(issueHref(this.ws, tr.dataset.key));
        else this.edit(td);
        return;
      case " ":
        e.preventDefault();
        this.toggle(tr.dataset.key);
        return;
      default:
        return;
    }
    e.preventDefault();
    if (next) this.activate(next);
  }

  toggle(key, on = !this.selected.has(key)) {
    if (on) this.selected.add(key);
    else this.selected.delete(key);
    const tr = this.shadowRoot.querySelector(`tr[data-key="${CSS.escape(key)}"]`);
    tr?.setAttribute("aria-selected", String(on));
    tr?.querySelector("[data-pick]")?.toggleAttribute("checked", on);
    this.shadowRoot.querySelector("[data-pick-all]")?.toggleAttribute("checked", this.items.every((i) => this.selected.has(i.key)));
    this.renderBulk();
  }

  // --- Editing -----------------------------------------------------------------------

  issue(key) {
    return this.items.find((i) => i.key === key);
  }

  /** Opens the editor of a cell. */
  edit(td) {
    if (td.getAttribute("aria-readonly") !== "false" || this.editing) return;
    const { t } = this.context.i18n;
    const i = this.issue(td.closest("tr").dataset.key);
    const c = this.cols.find((x) => x.id === td.dataset.col);
    if (!i || !c) return;
    this.activate(td, false);
    let editor;
    let kind = "choice";
    const select = (value, options) => html`<lds-select size="sm" data-editor value="${value}" label="${c.label}">${options}</lds-select>`;
    switch (c.edit) {
      case "title":
        kind = "input";
        editor = html`<lds-input size="sm" data-editor value="${i.title}" maxlength="255" aria-label="${c.label}"></lds-input>`;
        break;
      case "status":
        editor = select(i.status_id, this.data.statusesOf(i.type_id).map((s) => html`<option value="${s.id}">${s.name}</option>`));
        break;
      case "priority":
        editor = select(i.priority, PRIORITIES.map((p) => html`<option value="${p}">${t(`priority.${p}`)}</option>`));
        break;
      case "assignee":
        editor = select(i.assignee_id ?? "", html`<option value="">${t("issue.unassigned")}</option>${this.data.peopleList.map((p) => html`<option value="${p.id}">${p.name}</option>`)}`);
        break;
      case "estimate":
        editor = select(i.estimate ?? "", html`<option value="">${t("issues.noEstimate")}</option>${this.data.config.estimation.values.map((v) => html`<option value="${v.label}">${v.label}</option>`)}`);
        break;
      case "date":
        editor = html`<lds-date-picker size="sm" data-editor value="${i[c.id] ?? ""}" aria-label="${c.label}"></lds-date-picker>`;
        break;
      case "field": {
        const f = c.field;
        const v = i.fields?.[f.id];
        if (f.type === "checkbox") return this.save(i, c, !v); // a toggle
        if (f.type === "select") editor = select(v ?? "", html`<option value="">–</option>${(f.options ?? []).map((o) => html`<option value="${o.id}">${o.name}</option>`)}`);
        else if (f.type === "date") editor = html`<lds-date-picker size="sm" data-editor value="${v ?? ""}" aria-label="${c.label}"></lds-date-picker>`;
        else {
          kind = "input";
          editor = html`<lds-input size="sm" data-editor ${f.type === "number" ? html`type="number"` : ""} value="${v ?? ""}" aria-label="${c.label}"></lds-input>`;
        }
        break;
      }
      default:
        return;
    }
    this.editing = { td, issue: i, col: c, kind, html: td.innerHTML };
    td.classList.add("editing");
    td.innerHTML = String(editor);
    const el = td.querySelector("[data-editor]");
    requestAnimationFrame(() => el?.focus());
  }

  cancelEdit() {
    const ed = this.editing;
    if (!ed) return;
    this.editing = null;
    ed.td.classList.remove("editing");
    ed.td.innerHTML = ed.html;
    this.activate(ed.td);
  }

  saveInput() {
    const ed = this.editing;
    const raw = ed.td.querySelector("[data-editor]").getAttribute("value") ?? "";
    let value = raw;
    if (ed.col.edit === "title") {
      value = raw.trim();
      if (!value || value === ed.issue.title) return this.cancelEdit();
    } else if (ed.col.field?.type === "number") {
      value = raw === "" ? null : Number(raw);
    } else {
      value = raw === "" ? null : raw;
    }
    this.save(ed.issue, ed.col, value);
  }

  onChange(e) {
    const el = e.target;
    if (el.matches?.("[data-editor]") && this.editing && el.localName !== "lds-input") {
      return this.save(this.editing.issue, this.editing.col, e.detail?.value ?? "");
    }
    if (el.matches?.("[data-toggle-column]")) return this.toggleColumn(el.dataset.toggleColumn, !!e.detail?.checked);
    if (el.matches?.("[data-pick]")) return this.toggle(el.dataset.pick, !!e.detail?.checked);
    if (el.matches?.("[data-pick-all]")) {
      for (const i of this.items) this.toggle(i.key, !!e.detail?.checked);
      return;
    }
    if (el.matches?.("[data-bulk-action]") && e.detail?.value) return this.bulk(el.dataset.bulkAction, e.detail.value);
  }

  /** The request that sets a column of an issue to value. */
  request(i, c, value) {
    const { api } = this.context;
    const base = `${this.base}/issues/${encodeURIComponent(i.key)}`;
    const ifMatch = `"${i.version}"`;
    switch (c.edit) {
      case "title":
        return () => api.patch(base, { body: { title: value }, ifMatch });
      case "status":
        return value === i.status_id ? null : () => api.post(`${base}/transition`, { body: { status_id: value }, ifMatch });
      case "priority":
        return value === i.priority ? null : () => api.post(`${base}/prioritize`, { body: { priority: value }, ifMatch });
      case "assignee":
        return value === (i.assignee_id ?? "") ? null : () => api.post(`${base}/assign`, { body: { assignee_id: value || null }, ifMatch });
      case "estimate":
        return value === (i.estimate ?? "") ? null : () => api.post(`${base}/estimate`, { body: { estimate: value }, ifMatch });
      case "date":
        return value === (i[c.id] ?? "") ? null : () => api.patch(base, { body: { [c.id]: value }, ifMatch });
      case "field":
        return () => api.patch(base, { body: { fields: { [c.field.id]: value } }, ifMatch });
    }
    return null;
  }

  async save(i, c, value) {
    const { i18n } = this.context;
    const run = this.request(i, c, value);
    this.editing = null;
    this.refocus = true;
    if (run) {
      try {
        await run();
        document.dispatchEvent(new CustomEvent("hzn-issue-changed", { detail: { key: i.key } }));
      } catch (err) {
        const field = err.fields?.[0];
        const title = err.code === "version_conflict" ? i18n.t("issues.conflict")
          : err.code === "transition_not_allowed" ? i18n.t("board.notAllowed", { key: i.key })
            : field && i18n.has(`error.${field.field}.${field.code}`) ? i18n.t(`error.${field.field}.${field.code}`) : errorMessage(err, i18n);
        globalThis.LDS?.toast?.({ intent: "danger", title });
      }
    }
    await this.reload();
  }

  /** Applies one change to every selected issue, one request each. */
  async bulk(action, value) {
    const { api, i18n } = this.context;
    const { t } = i18n;
    const keys = [...this.selected];
    let done = 0;
    let skipped = 0;
    let failed = 0;
    for (const key of keys) {
      const i = this.issue(key);
      if (!i) continue;
      const base = `${this.base}/issues/${encodeURIComponent(key)}`;
      let run = null;
      if (action === "status") {
        const s = this.data.statusesOf(i.type_id).find((x) => x.name === value);
        if (s && s.id !== i.status_id) run = () => api.post(`${base}/transition`, { body: { status_id: s.id } });
        else if (!s) skipped++;
      } else if (action === "priority" && value !== i.priority) {
        run = () => api.post(`${base}/prioritize`, { body: { priority: value } });
      } else if (action === "assignee" && (value === "-" ? null : value) !== (i.assignee_id ?? null)) {
        run = () => api.post(`${base}/assign`, { body: { assignee_id: value === "-" ? null : value } });
      } else if (action === "archive" && !i.archived_at) {
        run = () => api.post(`${base}/archive`);
      }
      if (!run) continue;
      try {
        await run();
        done++;
      } catch {
        failed++;
      }
    }
    const parts = [t("table.bulkDone", { count: done })];
    if (skipped) parts.push(t("table.bulkSkipped", { count: skipped }));
    if (failed) parts.push(t("table.bulkFailed", { count: failed }));
    globalThis.LDS?.toast?.({ intent: failed ? "warning" : "success", title: parts.join(" ") });
    if (action === "archive") this.selected.clear();
    document.dispatchEvent(new CustomEvent("hzn-issue-changed", { detail: { key: "" } }));
    await this.reload();
  }

  // --- Columns and sorting -----------------------------------------------------------

  toggleColumn(id, on) {
    const order = this.allColumns().map((c) => c.id);
    const next = new Set(this.columns);
    if (on) next.add(id);
    else next.delete(id);
    this.columns = order.filter((c) => next.has(c));
    if (!this.bar?.view) store(this.storeId, this.columns);
    this.renderGrid();
    this.bar?.refresh();
  }

  /** Header click: ascending, then descending, then back to rank order. */
  cycleSort(field) {
    const cur = this.sort.find((s) => s.field === field);
    if (!cur) this.sort = [{ field, dir: "asc" }];
    else if (cur.dir === "asc") this.sort = [{ field, dir: "desc" }];
    else this.sort = [];
    const url = new URL(location.href);
    if (this.sort.length) url.searchParams.set("sort", this.sort.map((s) => `${s.field}:${s.dir}`).join(","));
    else url.searchParams.delete("sort");
    this.context.router.navigate(url.pathname + url.search, { replace: true });
    this.bar?.refresh();
    this.reload();
  }

  onClick(e) {
    const action = e.target.closest?.("[data-action]")?.dataset.action;
    if (action === "clear-selection") {
      this.selected.clear();
      return this.renderGrid();
    }
    if (action === "bulk-archive") return this.bulk("archive", true);
    const sort = e.target.closest?.("[data-sort]");
    if (sort) return this.cycleSort(sort.dataset.sort);
    const td = e.target.closest?.("td[data-col]");
    if (td && !td.classList.contains("editing")) this.activate(td);
  }

  /** Refreshes on pushed events about this project's issues. */
  onPush(m) {
    if (m.t !== "event" && m.t !== "resync") return;
    if (m.t === "event" && changesProjectData(m.d)) {
      forgetProjects();
      clearTimeout(this.timer);
      this.timer = setTimeout(() => this.load(), 200);
      return;
    }
    const key = m.d?.aggregate?.key ?? "";
    if (this.editing) return; // after the edit
    if (m.t === "resync" || key.startsWith(`${this.data?.project.key}-`) || m.d?.type === "issue.moved") {
      clearTimeout(this.timer);
      this.timer = setTimeout(() => this.reload(), 150);
    }
  }
}

customElements.define("hzn-table", HznTable);
