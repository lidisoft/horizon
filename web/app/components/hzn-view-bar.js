// <hzn-view-bar>: a project layout's saved views and filter (phase 6).
//
//   await bar.setup({ ws, data, layout, display, sort, changedDisplay })
//       data from projectData; layout "list", "board" or "table";
//       display() returns the layout's options to save with a view, sort()
//       its sort keys; changedDisplay() says whether they differ from the
//       view's. Reads ?view= and ?filter= from the address.
//   bar.refresh()  re-renders after the layout's sort or display changed
//   bar.filter     the filter in force (a query AST, docs/api.md#querying-issues), or null
//   bar.view       the saved view in force, or null
//   bar.saveDisplay(display)  stores display options in the view in force
//
// It fires hzn-view-change when the filter or the view changes, and keeps
// the address in step (?view=ID, ?filter=JSON while it differs from the
// view's). The filter is an AND of conditions shown as chips, built field →
// operator → value, plus words searched in titles and descriptions.

import { PRIORITIES } from "horizon/components/issue-ui.js";
import { loadIterations } from "horizon/components/planning-ui.js";
import { requestContext } from "horizon/core/context.js";
import { html, keepOpenOnInsidePress, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, showFieldErrors, submitOnEnter } from "horizon/core/forms.js";

/** Where each layout lives under /w/:ws/p/:key/. */
export const LAYOUT_ROUTES = { list: "issues", board: "board", table: "table" };

const styles = sheet(`
  :host { display: contents; }
  .bar { display: flex; align-items: center; gap: var(--lds-space-2); flex-wrap: wrap; flex: 1; min-inline-size: 0; }
  .chip { display: inline-flex; align-items: center; gap: var(--lds-space-1); block-size: 1.75rem; padding-inline: var(--lds-space-2) 0;
    border: 1px solid var(--lds-color-border-default); border-radius: var(--lds-radius-full); background: var(--lds-color-bg-subtle);
    font-size: var(--lds-font-size-sm); white-space: nowrap; }
  .chip .op { color: var(--lds-color-fg-muted); }
  .grow { flex: 1; }
  lds-input[data-text] { inline-size: 14rem; }
  .builder { display: grid; gap: var(--lds-space-3); padding: var(--lds-space-3); min-inline-size: 18rem; }
  .builder .actions, .actions { display: flex; justify-content: flex-end; gap: var(--lds-space-2); }
  form { display: grid; gap: var(--lds-space-4); }
  lds-alert:empty { display: none; }
`);

/** The conditions of an AND filter (a lone condition is a list of one). */
export function conditionsOf(filter) {
  if (!filter) return [];
  if (filter.and) return filter.and;
  return [filter];
}

/** The filter of a list of conditions. */
export function filterOf(conditions) {
  if (!conditions.length) return null;
  return conditions.length === 1 ? conditions[0] : { and: conditions };
}

/** The conditions the builder knows; others (from the API) still show as chips. */
function fields(t, data, iterations) {
  const people = [{ value: "@me", label: t("views.me") }, ...data.peopleList.map((p) => ({ value: p.id, label: p.name }))];
  const statusNames = [...new Set([...data.statuses.values()].sort((a, b) => a.position - b.position).map((s) => s.name))];
  const dates = [
    { value: "@today", label: t("views.date.today") }, { value: "+7d", label: t("views.date.in7") },
    { value: "+14d", label: t("views.date.in14") }, { value: "+1m", label: t("views.date.inMonth") }, { value: "-7d", label: t("views.date.ago7") },
  ];
  return {
    "status.outcome": { ops: ["eq", "neq"], choices: ["open", "closed", "unstarted", "started", "completed", "abandoned"].map((v) => ({ value: v, label: t(`views.outcome.${v}`) })) },
    status: { ops: ["in", "nin"], choices: statusNames.map((name) => ({ value: [...data.statuses.values()].filter((s) => s.name === name).map((s) => s.id), label: name })) },
    type: { ops: ["eq", "neq"], choices: [...data.types.values()].map((x) => ({ value: x.id, label: x.name })) },
    priority: { ops: ["eq", "neq"], choices: PRIORITIES.map((p) => ({ value: p, label: t(`priority.${p}`) })) },
    assignee: { ops: ["eq", "neq", "is_empty", "is_not_empty"], choices: people },
    labels: { ops: ["contains", "nin", "is_empty"], choices: [...data.labels.values()].filter((l) => !l.disabled).map((l) => ({ value: l.id, label: l.name })) },
    iteration: { ops: ["eq", "is_empty"], choices: [{ value: "@current_iteration", label: t("views.currentIteration") }, ...iterations.map((it) => ({ value: it.id, label: it.name }))] },
    due_date: { ops: ["lt", "gt", "within", "is_empty"], choices: dates },
    title: { ops: ["contains"], text: true },
  };
}

const same = (a, b) => JSON.stringify(a ?? null) === JSON.stringify(b ?? null);

class HznViewBar extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("lds-change", (e) => this.onChange(e));
    root.addEventListener("lds-input", (e) => {
      if (!e.target.matches("[data-text]")) return;
      clearTimeout(this.timer);
      this.timer = setTimeout(() => this.setText(e.detail.value), 300);
    });
    this.offSocket = this.context.socket.on((m) => {
      if (m.t === "event" && m.d?.aggregate?.type === "view") this.reloadViews();
    });
  }

  disconnectedCallback() {
    this.offSocket?.();
    clearTimeout(this.timer);
  }

  async setup({ ws, data, layout, display = () => ({}), sort = null, changedDisplay = () => false }) {
    Object.assign(this, { ws, data, layout, display, sort, changedDisplay });
    const query = this.context.router.current?.query;
    let iterations = [];
    try {
      [, iterations] = await Promise.all([this.reloadViews(), loadIterations(this.context, ws, data.project.key).then((l) => l.items.filter((it) => it.state !== "completed"))]);
    } catch {
      // the bar still filters; views and iterations come with the next push
    }
    this.fields = fields(this.context.i18n.t, data, iterations);
    this.view = this.views?.find((v) => v.id === query?.get("view")) ?? null;
    let filter = this.view?.query.filter ?? null;
    try {
      if (query?.get("filter")) filter = JSON.parse(query.get("filter"));
    } catch {
      // a mangled address: the view's filter
    }
    this.filter = filter;
    this.render();
  }

  get base() {
    return `/w/${encodeURIComponent(this.ws)}`;
  }

  /** Whether the filter differs from the view's (or there's one without a view). */
  get dirty() {
    return !same(this.filter, this.view?.query.filter ?? null);
  }

  /** Whether anything differs from the view in force: the filter, the sort, the display. */
  get modified() {
    if (this.dirty) return true;
    if (!this.view) return false;
    return (this.sort && !same(this.sort() ?? [], this.view.query.sort ?? [])) || this.changedDisplay();
  }

  refresh() {
    if (this.fields) this.render();
  }

  /** The query to save: the filter, and the layout's sort. */
  query(base = {}) {
    const q = { ...base, filter: this.filter ?? undefined };
    if (this.sort) q.sort = this.sort()?.length ? this.sort() : undefined;
    return q;
  }

  async reloadViews() {
    const { data } = await this.context.api.get(`${this.base}/views`, { query: { project: this.data.project.key } });
    this.views = data.items.filter((v) => LAYOUT_ROUTES[v.layout]);
    if (this.view) {
      const now = this.views.find((v) => v.id === this.view.id) ?? null;
      const changed = !now || !same(now.query.filter, this.view.query.filter) || !same(now.display, this.view.display);
      const wasClean = !this.dirty;
      this.view = now;
      if (wasClean && now) this.filter = now.query.filter ?? null;
      if (changed) this.changed();
    }
    if (this.fields) this.render();
  }

  // --- Rendering ---------------------------------------------------------------------

  render() {
    const { t } = this.context.i18n;
    const conds = conditionsOf(this.filter);
    const text = conds.find((c) => c.field === "text" && c.op === "contains")?.value ?? "";
    const canShare = this.data.project.can.includes("view.share");
    this.shadowRoot.innerHTML = String(html`
      <div class="bar">
        <lds-menu placement="bottom-start">
          <lds-button slot="trigger" variant="ghost" size="sm" data-picker>
            <lds-icon slot="start" name="layers" size="sm"></lds-icon>${this.view ? this.view.name : t("views.none")}${this.view && this.modified ? " *" : ""}
            <lds-icon slot="end" name="chevron-down" size="sm"></lds-icon>
          </lds-button>
          <lds-menu-item data-action="pick" data-id="">${t("views.none")}</lds-menu-item>
          ${this.views?.map((v) => html`
            <lds-menu-item data-action="pick" data-id="${v.id}">
              <lds-icon slot="start" name="${v.shared ? "users" : "user"}"></lds-icon>${v.name}
              <span slot="end">${t(`views.layout.${v.layout}`)}</span>
            </lds-menu-item>`)}
          <hr>
          ${this.view?.can_edit && this.modified ? html`<lds-menu-item data-action="update"><lds-icon slot="start" name="check"></lds-icon>${t("views.saveChanges")}</lds-menu-item>` : ""}
          <lds-menu-item data-action="save-as"><lds-icon slot="start" name="plus"></lds-icon>${t("views.saveAs")}</lds-menu-item>
          ${this.view?.can_edit ? html`<lds-menu-item data-action="delete"><lds-icon slot="start" name="trash-2"></lds-icon>${t("views.delete")}</lds-menu-item>` : ""}
        </lds-menu>
        ${conds.map((c, i) => (c.field === "text" ? "" : html`
          <span class="chip" data-chip="${i}">${this.chipLabel(c)}
            <lds-button variant="ghost" size="sm" data-action="remove" data-index="${i}" label="${t("views.removeFilter")}"><lds-icon name="x" size="sm"></lds-icon></lds-button>
          </span>`))}
        <lds-popover label="${t("views.addFilter")}" data-builder>
          <lds-button slot="trigger" variant="ghost" size="sm" data-add><lds-icon slot="start" name="filter" size="sm"></lds-icon>${t("views.filter")}</lds-button>
          <div class="builder" data-form></div>
        </lds-popover>
        ${this.dirty ? html`<lds-button variant="ghost" size="sm" data-action="reset">${t(this.view ? "views.reset" : "views.clear")}</lds-button>` : ""}
        <span class="grow"></span>
        <lds-input type="search" size="sm" data-text value="${text}" placeholder="${t("views.textPlaceholder")}" aria-label="${t("views.text")}">
          <lds-icon slot="prefix" name="search"></lds-icon></lds-input>
      </div>
      <lds-modal size="sm" data-save>
        <span slot="header">${t("views.saveAs")}</span>
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <lds-field label="${t("views.name")}" required><lds-input name="name" required maxlength="100"></lds-input></lds-field>
          ${canShare ? html`<lds-switch name="shared">${t("views.share", { project: this.data.project.name })}</lds-switch>` : ""}
        </form>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-action="close-save">${t("tokens.cancel")}</lds-button>
          <lds-button variant="primary" data-action="submit-save">${t("account.save")}</lds-button>
        </div>
      </lds-modal>`);
    const form = this.shadowRoot.querySelector("[data-save] form");
    submitOnEnter(form);
    form.addEventListener("submit", (e) => {
      e.preventDefault();
      this.saveAs();
    });
    keepOpenOnInsidePress(this.shadowRoot.querySelector("[data-builder]"));
    this.builderField = this.builderField ?? "status.outcome";
    this.renderBuilder();
  }

  /** "Assignee is Ann", "Labels include bug"… */
  chipLabel(c) {
    const { t } = this.context.i18n;
    const f = this.fields[c.field];
    const name = f ? t(`views.field.${c.field}`) : c.field;
    const op = t(`views.op.${c.op}`);
    if (c.op === "is_empty" || c.op === "is_not_empty") return html`${name} <span class="op">${op}</span>`;
    const label = (v) => f?.choices?.find((x) => same(x.value, v))?.label
      ?? (c.field === "status" ? [...new Set([v].flat().map((id) => this.data.statuses.get(id)?.name ?? "?"))].join(", ") : Array.isArray(v) ? v.join(", ") : String(v));
    return html`${name} <span class="op">${op}</span> ${label(c.value)}`;
  }

  renderBuilder() {
    const { t } = this.context.i18n;
    const box = this.shadowRoot.querySelector("[data-form]");
    const f = this.fields[this.builderField];
    if (!f.ops.includes(this.builderOp)) this.builderOp = f.ops[0];
    const needsValue = this.builderOp !== "is_empty" && this.builderOp !== "is_not_empty";
    box.innerHTML = String(html`
      <lds-select size="sm" data-field value="${this.builderField}" label="${t("views.fieldLabel")}">
        ${Object.keys(this.fields).map((k) => html`<option value="${k}">${t(`views.field.${k}`)}</option>`)}
      </lds-select>
      <lds-select size="sm" data-op value="${this.builderOp}" label="${t("views.opLabel")}">
        ${f.ops.map((o) => html`<option value="${o}">${t(`views.op.${o}`)}</option>`)}
      </lds-select>
      ${needsValue ? (f.text
        ? html`<lds-input size="sm" data-value aria-label="${t("views.valueLabel")}"></lds-input>`
        : html`<lds-select size="sm" data-value value="0" label="${t("views.valueLabel")}">${f.choices.map((c, i) => html`<option value="${i}">${c.label}</option>`)}</lds-select>`) : ""}
      <div class="actions"><lds-button variant="primary" size="sm" data-action="apply">${t("views.apply")}</lds-button></div>`);
  }

  // --- Changes -----------------------------------------------------------------------

  onChange(e) {
    if (e.target.matches("[data-field]")) {
      this.builderField = e.detail.value;
      this.renderBuilder();
    } else if (e.target.matches("[data-op]")) {
      this.builderOp = e.detail.value;
      this.renderBuilder();
    }
  }

  onClick(e) {
    const el = e.target.closest?.("[data-action]");
    switch (el?.dataset.action) {
      case "pick":
        return this.pick(el.dataset.id);
      case "remove": {
        const conds = conditionsOf(this.filter).filter((_, i) => i !== Number(el.dataset.index));
        return this.setFilter(filterOf(conds));
      }
      case "apply":
        return this.apply();
      case "reset":
        return this.setFilter(this.view?.query.filter ?? null);
      case "save-as": {
        const modal = this.shadowRoot.querySelector("[data-save]");
        clearFieldErrors(modal.querySelector("form"));
        modal.querySelector("lds-alert").textContent = "";
        modal.setAttribute("open", "");
        return;
      }
      case "close-save":
        return this.shadowRoot.querySelector("[data-save]").removeAttribute("open");
      case "submit-save":
        return this.shadowRoot.querySelector("[data-save] form").requestSubmit();
      case "update":
        return this.update({ query: this.query(this.view.query), display: this.display() });
      case "delete":
        return this.remove();
    }
  }

  /** Adds the builder's condition. */
  apply() {
    const box = this.shadowRoot.querySelector("[data-form]");
    const f = this.fields[this.builderField];
    const cond = { field: this.builderField, op: this.builderOp };
    if (this.builderOp !== "is_empty" && this.builderOp !== "is_not_empty") {
      const input = box.querySelector("[data-value]");
      if (f.text) {
        const v = String(input.getAttribute("value") ?? "").trim();
        if (!v) return;
        cond.value = v;
      } else {
        const choice = f.choices[Number(input.getAttribute("value") ?? 0)];
        if (!choice) return;
        cond.value = choice.value;
      }
    }
    this.shadowRoot.querySelector("[data-builder]").removeAttribute("open");
    this.setFilter(filterOf([...conditionsOf(this.filter), cond]));
  }

  /** The words searched, replacing any before. */
  setText(text) {
    const conds = conditionsOf(this.filter).filter((c) => c.field !== "text");
    if (text.trim()) conds.push({ field: "text", op: "contains", value: text.trim() });
    const before = this.filter;
    this.filter = filterOf(conds);
    if (same(before, this.filter)) return;
    this.syncAddress();
    this.render();
    this.shadowRoot.querySelector("[data-text]").focus(); // where the person is typing
    this.changed();
  }

  setFilter(filter) {
    this.filter = filter;
    this.syncAddress();
    this.render();
    this.changed();
  }

  pick(id) {
    const v = this.views.find((x) => x.id === id) ?? null;
    if (v && v.layout !== this.layout) {
      this.context.router.navigate(`${this.base}/p/${encodeURIComponent(this.data.project.key)}/${LAYOUT_ROUTES[v.layout]}?view=${v.id}`);
      return;
    }
    this.view = v;
    this.setFilter(v?.query.filter ?? null);
  }

  syncAddress() {
    const url = new URL(location.href);
    if (this.view) url.searchParams.set("view", this.view.id);
    else url.searchParams.delete("view");
    if (this.dirty && this.filter) url.searchParams.set("filter", JSON.stringify(this.filter));
    else url.searchParams.delete("filter");
    if (url.href !== location.href) this.context.router.navigate(url.pathname + url.search, { replace: true });
  }

  changed() {
    this.dispatchEvent(new CustomEvent("hzn-view-change", { bubbles: true, composed: true, detail: { filter: this.filter, view: this.view } }));
  }

  // --- Saving ------------------------------------------------------------------------

  async saveAs() {
    const { api, i18n } = this.context;
    const modal = this.shadowRoot.querySelector("[data-save]");
    const form = modal.querySelector("form");
    clearFieldErrors(form);
    // LDS controls keep their state in attributes.
    const name = String(form.querySelector('[name="name"]').getAttribute("value") ?? "").trim();
    const shared = !!form.querySelector('[name="shared"]')?.hasAttribute("checked");
    await busy(modal.querySelector('[data-action="submit-save"]'), async () => {
      try {
        const { data } = await api.post(`${this.base}/views`, {
          body: { project: this.data.project.key, name, layout: this.layout, shared, query: this.query(), display: this.display() },
        });
        modal.removeAttribute("open");
        this.views = [...this.views, data];
        this.view = data;
        this.setFilter(data.query.filter ?? null);
        globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("views.saved", { name: data.name }) });
      } catch (err) {
        const rest = err.fields ? showFieldErrors(form, err.fields, i18n.t, i18n.has) : [];
        if (!err.fields || rest.length) form.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      }
    });
  }

  /** Changes the view in force; the If-Match keeps someone else's change. */
  async update(body) {
    const { api, i18n } = this.context;
    try {
      const { data } = await api.patch(`${this.base}/views/${this.view.id}`, { body, ifMatch: `"${this.view.version}"` });
      this.views = this.views.map((v) => (v.id === data.id ? data : v));
      this.view = data;
      this.syncAddress();
      this.render();
      return data;
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      if (err.code === "version_conflict") await this.reloadViews();
      return null;
    }
  }

  /** Stores display options (board WIP limits…) in the view in force. */
  async saveDisplay(display) {
    if (!this.view?.can_edit) return null;
    const saved = await this.update({ display });
    if (saved) this.changed();
    return saved;
  }

  async remove() {
    const { api, i18n } = this.context;
    if (!this.view) return;
    try {
      await api.delete(`${this.base}/views/${this.view.id}`);
      this.views = this.views.filter((v) => v.id !== this.view.id);
      this.view = null;
      this.setFilter(this.filter);
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
  }
}

customElements.define("hzn-view-bar", HznViewBar);
