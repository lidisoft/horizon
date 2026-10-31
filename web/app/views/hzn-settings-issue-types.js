// <hzn-settings-issue-types>: /w/:ws/settings/issue-types[?project=KEY] —
// issue types (wireframes: IssueTypes). Inherited types are switched on or
// off per project, or copied to make them editable there. Fields join this
// page with custom fields.

import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, formValues, showFieldErrors } from "horizon/core/forms.js";
import { COLOURS, paintSwatches } from "horizon/components/project-colour.js";
import { loadScope, scopePicker, watchScope } from "horizon/views/config-scope.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

const ICONS = ["layers", "bookmark", "check", "alert-circle", "corner-down-right", "flag", "rocket", "milestone", "git-branch",
  "git-pull-request", "message-square", "eye", "search", "kanban", "list", "repeat", "tag", "user", "users", "calendar",
  "lock", "inbox", "paperclip", "link"];

const styles = sheet(`
  :host { --list: 17rem; }
  .content { padding: 0 !important; }
  .split { display: grid; grid-template-columns: var(--list) minmax(0, 1fr); min-height: calc(100vh - 4.5rem); }
  .list { border-inline-end: 1px solid var(--lds-color-border-subtle); padding: var(--lds-space-5) var(--lds-space-3); }
  .list h3 { font-size: var(--lds-font-size-2xs); letter-spacing: 0.08em; text-transform: uppercase; color: var(--lds-color-fg-muted);
    margin: 0 0 var(--lds-space-2) var(--lds-space-2); }
  .list h3 + * { margin-bottom: var(--lds-space-5); }
  .item { display: grid; grid-template-columns: auto 1fr auto; gap: var(--lds-space-3); align-items: center; width: 100%;
    padding: var(--lds-space-2); border-radius: var(--lds-radius-md); cursor: pointer; text-align: start; }
  .item[aria-current="true"] { background: var(--lds-color-accent-subtle); }
  .item:hover { background: var(--lds-color-bg-muted); }
  .item small { display: block; color: var(--lds-color-fg-muted); }
  .chip { width: 1.75rem; height: 1.75rem; border-radius: var(--lds-radius-md); display: grid; place-items: center;
    color: var(--lds-color-fg-on-accent); }
  .detail { padding: var(--lds-space-5) var(--lds-space-6); }
  .head { display: flex; gap: var(--lds-space-3); align-items: center; margin-bottom: var(--lds-space-5); }
  .head h2 { margin: 0; font-size: var(--lds-font-size-lg); }
  .head .menu { margin-inline-start: auto; }
  form { display: grid; grid-template-columns: 1fr 12rem 10rem; gap: var(--lds-space-4); max-width: 52rem; }
  form .wide { grid-column: span 2; }
  form .full { grid-column: 1 / -1; }
  .note { font-size: var(--lds-font-size-sm); color: var(--lds-color-fg-muted); margin: var(--lds-space-6) 0 0; }
  lds-modal form { display: grid; grid-template-columns: 1fr; }
  .map { display: grid; grid-template-columns: 1fr auto 1fr; gap: var(--lds-space-3); align-items: center; }
`);

class HznSettingsIssueTypes extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [settingsStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("lds-select", (e) => this.onMenu(e));
    this.unsubscribe = this.context.router.subscribe((m) => m.route?.name === "settings-issue-types" && this.load());
    this.load();
  }

  disconnectedCallback() {
    this.unsubscribe?.();
  }

  async load(selectId) {
    const { api, i18n } = this.context;
    try {
      this.scope = await loadScope(this.context, this.getAttribute("ws"));
      const [types, workflows] = await Promise.all([api.get(`${this.scope.base}/issue-types`), api.get(`${this.scope.base}/workflows`)]);
      this.types = types.data.items;
      this.canManage = types.data.can_manage;
      this.workflows = workflows.data.items;
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    const keep = selectId ?? this.selected;
    this.selected = this.types.some((x) => x.id === keep) ? keep : this.types.find((x) => !x.disabled)?.id ?? this.types[0]?.id;
    this.render();
  }

  get current() {
    return this.types.find((x) => x.id === this.selected);
  }

  wfName(id) {
    return this.workflows.find((w) => w.id === id)?.name ?? "—";
  }

  render() {
    const { t } = this.context.i18n;
    const scope = this.scope;
    const inherited = scope.project ? this.types.filter((x) => x.inherited) : [];
    const own = scope.project ? this.types.filter((x) => !x.inherited) : this.types;
    const row = (it) => html`
      <div class="item" role="button" tabindex="0" data-select="${it.id}" aria-current="${it.id === this.selected ? "true" : "false"}">
        <span class="chip" data-colour="${it.colour}"><lds-icon name="${it.icon}"></lds-icon></span>
        <span>${it.name}<small>${it.disabled ? t("types.switchedOff") : `${this.wfName(it.workflow_id)} · ${t(`types.parent.${it.parent_rule}`)}`}</small></span>
        ${scope.project && it.inherited && this.canManage
          ? html`<lds-switch size="sm" data-toggle="${it.id}" ${it.disabled ? "" : html`checked`} label="${t("types.useIn", { name: it.name, project: scope.project.name })}"></lds-switch>`
          : html`<span></span>`}
      </div>`;
    const list = html`
      <div class="list">
        ${scope.project ? html`<h3>${t("types.inheritedFrom", { name: scope.ws.name })}</h3><div>${inherited.map(row)}</div>
          <h3>${t("types.projectOnly", { name: scope.project.name })}</h3>` : ""}
        <div>${own.map(row)}</div>
        ${this.canManage ? html`<lds-button variant="ghost" data-action="new"><lds-icon slot="start" name="plus"></lds-icon>${t("types.new")}</lds-button>` : ""}
        ${scope.project ? html`<p class="note">${t("types.inheritedNote", { project: scope.project.name })}</p>` : ""}
      </div>`;
    const body = html`<div class="split">${list}<div class="detail">${this.current ? this.detail() : ""}</div></div>${this.dialogs()}`;
    this.shadowRoot.innerHTML = String(settingsPage(t, scope.ws, "issue-types", body, scopePicker(t, scope)));
    watchScope(this.shadowRoot, this.context, scope.ws.slug, "issue-types");
    paintSwatches(this.shadowRoot);
    for (const sw of this.shadowRoot.querySelectorAll("[data-toggle]")) {
      sw.addEventListener("lds-change", (e) => {
        e.stopPropagation();
        this.toggle(sw.dataset.toggle, e.detail.checked ?? sw.hasAttribute("checked"));
      });
    }
  }

  detail() {
    const { t } = this.context.i18n;
    const it = this.current;
    const edit = this.canManage && !it.inherited;
    const off = edit ? "" : html`disabled`;
    const others = this.types.filter((x) => x.id !== it.id && !x.disabled);
    const origin = this.scope.project
      ? it.inherited ? t("types.inheritedHint", { name: this.scope.ws.name }) : t("types.projectHint", { name: this.scope.project.name })
      : "";
    const actions = [];
    if (this.canManage && it.inherited) actions.push(["copy", "types.copy"]);
    if (edit) actions.push(["delete", "types.delete"]);
    return html`
      <div class="head">
        <span class="chip" data-colour="${it.colour}"><lds-icon name="${it.icon}"></lds-icon></span>
        <div><h2>${it.name}</h2><span class="muted">${origin}</span></div>
        ${actions.length ? html`
          <lds-menu class="menu" placement="bottom-end" data-type-menu>
            <lds-button slot="trigger" variant="secondary" size="sm"><lds-icon slot="start" name="more-horizontal"></lds-icon>${t("types.actions")}</lds-button>
            ${actions.map(([v, l]) => html`<lds-menu-item value="${v}">${t(l)}</lds-menu-item>`)}
          </lds-menu>` : ""}
      </div>
      <form novalidate data-form>
        <lds-alert class="full" intent="danger" role="alert"></lds-alert>
        <lds-field label="${t("types.name")}" required><lds-input name="name" maxlength="40" value="${it.name}" ${edit ? "" : html`readonly`}></lds-input></lds-field>
        <lds-field label="${t("types.icon")}">
          <lds-combobox name="icon" value="${it.icon}" ${off}>${ICONS.map((i) => html`<lds-option value="${i}"><lds-icon slot="start" name="${i}"></lds-icon>${i}</lds-option>`)}</lds-combobox>
        </lds-field>
        <lds-field label="${t("types.colour")}">
          <lds-select name="colour" value="${it.colour}" ${off}>${COLOURS.map((c) => html`<option value="${c}">${t(`colour.${c}`)}</option>`)}</lds-select>
        </lds-field>
        <lds-field label="${t("types.workflow")}" class="wide">
          <lds-select name="workflow_id" value="${it.workflow_id}" ${off}>
            ${this.workflows.filter((w) => !w.disabled).map((w) => html`<option value="${w.id}">${w.name}</option>`)}
          </lds-select>
          <span slot="help">${t("types.workflowHelp")}</span>
        </lds-field>
        <lds-field label="${t("types.parentRule")}">
          <lds-select name="parent_rule" value="${it.parent_rule}" ${off}>
            ${["any", "none", "required"].map((r) => html`<option value="${r}">${t(`types.parent.${r}`)}</option>`)}
          </lds-select>
        </lds-field>
        <lds-field label="${t("types.parentTypes")}" class="full">
          <lds-combobox name="parent_type_ids" multiple value="${it.parent_type_ids.join(",")}" placeholder="${t("types.anyParent")}" ${it.parent_rule === "none" || !edit ? html`disabled` : ""}>
            ${others.map((x) => html`<lds-option value="${x.id}"><lds-icon slot="start" name="${x.icon}"></lds-icon>${x.name}</lds-option>`)}
          </lds-combobox>
          <span slot="help">${t("types.parentTypesHelp")}</span>
        </lds-field>
        ${edit ? html`<div class="full"><lds-button data-action="save">${t("account.save")}</lds-button> <span class="muted" role="status" data-saved></span></div>` : ""}
      </form>
      <p class="note">${t("types.fieldsLater")}</p>`;
  }

  dialogs() {
    if (!this.canManage) return "";
    const { t } = this.context.i18n;
    return html`
      <lds-modal data-dialog="new" size="sm">
        <span slot="header">${t("types.new")}</span>
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <lds-field label="${t("types.name")}" required><lds-input name="name" required maxlength="40"></lds-input></lds-field>
          <lds-field label="${t("types.workflow")}">
            <lds-select name="workflow_id" value="${this.workflows.find((w) => !w.disabled)?.id ?? ""}">
              ${this.workflows.filter((w) => !w.disabled).map((w) => html`<option value="${w.id}">${w.name}</option>`)}
            </lds-select>
          </lds-field>
        </form>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button data-action="create">${t("types.create")}</lds-button>
        </div>
      </lds-modal>
      <lds-modal data-dialog="mapping" size="md">
        <span slot="header">${t("types.mappingTitle")}</span>
        <p class="muted">${t("types.mappingLead")}</p>
        <div data-map></div>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button data-action="apply-mapping">${t("types.mappingApply")}</lds-button>
        </div>
      </lds-modal>`;
  }

  onClick(e) {
    const sel = e.target.closest?.("[data-select]");
    if (sel && !e.target.closest("lds-switch")) {
      this.selected = sel.dataset.select;
      this.render();
      return;
    }
    const target = e.target.closest?.("[data-action], [data-close]");
    if (!target) return;
    if (target.hasAttribute("data-close")) return target.closest("lds-modal").removeAttribute("open");
    const a = target.dataset.action;
    if (a === "new") this.shadowRoot.querySelector('[data-dialog="new"]').setAttribute("open", "");
    if (a === "create") this.create();
    if (a === "save") this.save();
    if (a === "apply-mapping") this.save(this.readMapping());
  }

  onMenu(e) {
    if (!e.target.closest?.("[data-type-menu]")) return;
    const value = e.detail?.value ?? e.target.getAttribute?.("value");
    const { api, i18n } = this.context;
    const it = this.current;
    const run = (fn, then) => fn().then((r) => this.load(then?.(r))).catch((err) => globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) }));
    if (value === "copy") run(() => api.post(`${this.scope.base}/issue-types/${it.id}/copy`), (r) => r.data.id);
    if (value === "delete" && confirm(i18n.t("types.deleteConfirm", { name: it.name }))) run(() => api.delete(`${this.scope.base}/issue-types/${it.id}`));
  }

  async toggle(id, on) {
    const { api, i18n } = this.context;
    try {
      await api.post(`${this.scope.base}/issue-types/${id}/${on ? "enable" : "disable"}`);
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
    this.load();
  }

  async create() {
    const { api, i18n } = this.context;
    const modal = this.shadowRoot.querySelector('[data-dialog="new"]');
    const form = modal.querySelector("form");
    clearFieldErrors(form);
    let created;
    try {
      ({ data: created } = await busy(modal.querySelector('[data-action="create"]'), () => api.post(`${this.scope.base}/issue-types`, { body: formValues(form) })));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else form.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      return;
    }
    modal.removeAttribute("open");
    this.load(created.id);
  }

  async save(statusMapping) {
    const { api, i18n } = this.context;
    const form = this.shadowRoot.querySelector("[data-form]");
    clearFieldErrors(form);
    form.querySelector("lds-alert").textContent = "";
    const v = formValues(form);
    const ids = (form.querySelector('[name="parent_type_ids"]').getAttribute("value") || "").split(",").filter(Boolean);
    const body = { name: v.name, icon: form.querySelector('[name="icon"]').getAttribute("value"), colour: v.colour,
      workflow_id: v.workflow_id, parent_rule: v.parent_rule, parent_type_ids: ids, status_mapping: statusMapping };
    try {
      await busy(this.shadowRoot.querySelector('[data-action="save"]'), () =>
        api.patch(`${this.scope.base}/issue-types/${this.current.id}`, { body, ifMatch: `"${this.current.version}"` }));
    } catch (err) {
      if (err.code === "unmapped_statuses") return this.askMapping(err.fields, v.workflow_id);
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else form.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      return;
    }
    this.shadowRoot.querySelector('[data-dialog="mapping"]')?.removeAttribute("open");
    await this.load();
    const saved = this.shadowRoot.querySelector("[data-saved]");
    if (saved) saved.textContent = i18n.t("account.saved");
  }

  /** Asks where the issues of each old status go in the new workflow. */
  askMapping(fields, workflowId) {
    const { t } = this.context.i18n;
    const from = this.workflows.find((w) => w.id === this.current.workflow_id);
    const to = this.workflows.find((w) => w.id === workflowId);
    const modal = this.shadowRoot.querySelector('[data-dialog="mapping"]');
    modal.querySelector("[data-map]").innerHTML = String(html`${fields.map((f) => {
      const id = f.field.replace("status_mapping.", "");
      const count = f.code.split(":")[1] ?? "?";
      const st = from?.statuses.find((s) => s.id === id);
      return html`<div class="map" data-from="${id}">
        <span>${st?.name ?? id} · ${t("types.issuesCount", { count: Number(count) })}</span><lds-icon name="chevron-right"></lds-icon>
        <lds-select value="${to.statuses[0]?.id}">${to.statuses.map((s) => html`<option value="${s.id}">${s.name}</option>`)}</lds-select>
      </div>`;
    })}`);
    modal.setAttribute("open", "");
  }

  readMapping() {
    const out = {};
    for (const row of this.shadowRoot.querySelectorAll("[data-from]")) out[row.dataset.from] = row.querySelector("lds-select").getAttribute("value");
    return out;
  }
}

customElements.define("hzn-settings-issue-types", HznSettingsIssueTypes);
