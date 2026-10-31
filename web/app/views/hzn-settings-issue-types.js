// <hzn-settings-issue-types>: /w/:ws/settings/issue-types[?project=KEY] —
// issue types and custom fields (wireframes: IssueTypes; ?view=fields for
// the fields). Inherited types are switched on or off per project, or
// copied to make them editable there; fields are attached per type, in
// order, optionally required at creation.

import { requestContext } from "horizon/core/context.js";
import { html, ldsLightSheet, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, formValues, showFieldErrors } from "horizon/core/forms.js";
import { COLOURS, paintSwatches } from "horizon/components/project-colour.js";
import { loadScope, scopePicker, watchScope } from "horizon/views/config-scope.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

const FIELD_TYPES = ["text", "number", "select", "multi_select", "date", "user", "url", "checkbox"];
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
  .fields-head { display: flex; justify-content: space-between; align-items: center; margin: var(--lds-space-6) 0 var(--lds-space-3); max-width: 52rem; }
  .fields-head h3 { margin: 0; font-size: var(--lds-font-size-md); }
  .fields-head lds-select { width: 16rem; }
  .attached { max-width: 52rem; }
  td.end { text-align: end; white-space: nowrap; width: 1%; }
  .option { display: grid; grid-template-columns: 1fr 9rem auto; gap: var(--lds-space-2); margin-bottom: var(--lds-space-2); }
`);

class HznSettingsIssueTypes extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [ldsLightSheet(), settingsStyles, styles];
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
      const [types, workflows, fields] = await Promise.all([
        api.get(`${this.scope.base}/issue-types`), api.get(`${this.scope.base}/workflows`), api.get(`${this.scope.base}/fields`),
      ]);
      this.types = types.data.items;
      this.canManage = types.data.can_manage;
      this.workflows = workflows.data.items;
      this.fields = fields.data.items;
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    if (this.view === "fields") {
      const keep = selectId ?? this.selectedField;
      this.selectedField = this.fields.some((x) => x.id === keep) ? keep : this.fields[0]?.id;
      return this.render();
    }
    const keep = selectId ?? this.selected;
    this.selected = this.types.some((x) => x.id === keep) ? keep : this.types.find((x) => !x.disabled)?.id ?? this.types[0]?.id;
    this.typeFields = [];
    if (this.selected) {
      try {
        ({ data: { fields: this.typeFields } } = await api.get(`${this.scope.base}/issue-types/${this.selected}/fields`));
      } catch {
        // shown empty; editing reports the error
      }
    }
    this.render();
  }

  get view() {
    return this.context.router.current?.query.get("view") === "fields" ? "fields" : "types";
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
    const tabs = html`
      <lds-segmented size="sm" data-view value="${this.view}" label="${t("types.view")}">
        <lds-segment value="types">${t("types.tabTypes")}</lds-segment>
        <lds-segment value="fields">${t("types.tabFields")}</lds-segment>
      </lds-segmented>${scopePicker(t, scope)}`;
    const body = this.view === "fields"
      ? html`<div class="split">${this.fieldList()}<div class="detail">${this.fieldDetail()}</div></div>${this.dialogs()}`
      : html`<div class="split">${list}<div class="detail">${this.current ? this.detail() : ""}</div></div>${this.dialogs()}`;
    this.shadowRoot.innerHTML = String(settingsPage(t, scope.ws, "issue-types", body, tabs));
    watchScope(this.shadowRoot, this.context, scope.ws.slug, "issue-types");
    this.shadowRoot.querySelector("[data-view]").addEventListener("lds-change", (e) => {
      const q = new URLSearchParams(this.context.router.current.query);
      if (e.detail.value === "fields") q.set("view", "fields");
      else q.delete("view");
      this.context.router.navigate(`/w/${scope.ws.slug}/settings/issue-types${q.size ? `?${q}` : ""}`, { replace: true });
    });
    this.bindFields();
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
      ${this.attachedFields(edit)}`;
  }

  /** The fields of the selected type: order, required, attach, remove. */
  attachedFields(edit) {
    const { t } = this.context.i18n;
    const it = this.current;
    const byId = new Map(this.fields.map((f) => [f.id, f]));
    const attached = this.typeFields.filter((tf) => byId.has(tf.field_id));
    const free = this.fields.filter((f) => !f.disabled && !attached.some((tf) => tf.field_id === f.id) && (f.project_id ? f.project_id === it.project_id : true));
    return html`
      <div class="fields-head"><h3>${t("types.fieldsOn", { name: it.name })}</h3>
        ${edit && free.length ? html`<lds-select size="sm" data-attach value="" label="${t("types.attach")}">
          <option value="">${t("types.attach")}</option>${free.map((f) => html`<option value="${f.id}">${f.name}</option>`)}</lds-select>` : ""}</div>
      ${attached.length ? html`
        <lds-table density="compact" class="attached">
          <table>
            <thead><tr><th scope="col">${t("types.field")}</th><th scope="col">${t("types.fieldType")}</th><th scope="col">${t("types.definedIn")}</th>
              <th scope="col">${t("types.required")}</th><th scope="col"><span class="lds-visually-hidden">${t("account.actions")}</span></th></tr></thead>
            <tbody>${attached.map((tf, i) => {
              const f = byId.get(tf.field_id);
              return html`<tr>
                <td>${f.name}</td><td><lds-badge size="sm">${t(`fieldType.${f.type}`)}</lds-badge></td>
                <td>${f.project_id ? this.scope.project?.name ?? "" : t("config.workspaceScope", { name: this.scope.ws.name })}</td>
                <td><lds-switch data-required="${i}" ${tf.required ? html`checked` : ""} ${edit ? "" : html`disabled`} label="${t("types.required")}"></lds-switch></td>
                <td class="end">${edit ? html`
                  ${i > 0 ? html`<lds-button size="sm" variant="ghost" data-field-up="${i}" label="${t("categories.up")}"><lds-icon name="chevron-up"></lds-icon></lds-button>` : ""}
                  <lds-button size="sm" variant="ghost" data-detach="${i}" label="${t("grants.remove")}"><lds-icon name="x"></lds-icon></lds-button>` : ""}</td>
              </tr>`;
            })}</tbody>
          </table>
        </lds-table>` : html`<p class="muted">${t("types.noFields")}</p>`}
      <p class="note">${t("types.requiredNote")}</p>`;
  }

  fieldList() {
    const { t } = this.context.i18n;
    const scope = this.scope;
    const row = (f) => html`
      <div class="item" role="button" tabindex="0" data-select-field="${f.id}" aria-current="${f.id === this.selectedField ? "true" : "false"}">
        <span></span><span>${f.name}<small>${t(`fieldType.${f.type}`)}${f.disabled ? ` · ${t("config.switchedOff")}` : ""} · ${t("types.usedByCount", { count: f.used_by.length })}</small></span>
        ${scope.project && f.inherited && this.canManage
          ? html`<lds-switch data-field-toggle="${f.id}" ${f.disabled ? "" : html`checked`} label="${t("types.useIn", { name: f.name, project: scope.project.name })}"></lds-switch>`
          : html`<span></span>`}
      </div>`;
    const inherited = scope.project ? this.fields.filter((f) => f.inherited) : [];
    const own = scope.project ? this.fields.filter((f) => !f.inherited) : this.fields;
    return html`
      <div class="list">
        ${scope.project ? html`<h3>${t("types.inheritedFrom", { name: scope.ws.name })}</h3><div>${inherited.map(row)}</div>
          <h3>${t("types.projectOnly", { name: scope.project.name })}</h3>` : ""}
        <div>${own.map(row)}</div>
        ${this.canManage ? html`<lds-button variant="ghost" data-action="new-field"><lds-icon slot="start" name="plus"></lds-icon>${t("fields.new")}</lds-button>` : ""}
      </div>`;
  }

  fieldDetail() {
    const { t } = this.context.i18n;
    const f = this.fields.find((x) => x.id === this.selectedField);
    if (!f) return html`<p class="muted">${t("fields.none")}</p>`;
    const edit = this.canManage && !f.inherited;
    const ro = edit ? "" : html`readonly`;
    const select = f.type === "select" || f.type === "multi_select";
    return html`
      <div class="head"><div><h2>${f.name}</h2><span class="muted">${t(`fieldType.${f.type}`)}</span></div>
        ${edit ? html`<lds-button class="menu" variant="ghost" data-action="delete-field"><lds-icon slot="start" name="trash-2"></lds-icon>${t("fields.delete")}</lds-button>` : ""}</div>
      <form novalidate data-field-form>
        <lds-alert class="full" intent="danger" role="alert"></lds-alert>
        <lds-field label="${t("types.name")}" required class="wide"><lds-input name="name" maxlength="60" value="${f.name}" ${ro}></lds-input></lds-field>
        <lds-field label="${t("fields.description")}" class="full"><lds-input name="description" maxlength="200" value="${f.description}" ${ro}></lds-input></lds-field>
        ${f.type === "number" ? html`
          <lds-field label="${t("fields.unit")}"><lds-input name="unit" maxlength="10" value="${f.unit ?? ""}" ${ro}></lds-input></lds-field>
          <lds-field label="${t("fields.precision")}"><lds-input name="precision" type="number" min="0" max="6" value="${f.precision ?? 0}" ${ro}></lds-input></lds-field>` : ""}
        ${select ? html`
          <div class="full"><strong>${t("fields.options")}</strong>
            <div data-options>${(f.options ?? []).map((o, i) => html`
              <div class="option" data-option="${o.id}">
                <lds-input size="sm" data-option-name value="${o.name}" maxlength="60" ${ro} label="${t("fields.optionName")}"></lds-input>
                <lds-select size="sm" data-option-colour value="${o.colour}" ${edit ? "" : html`disabled`}>${COLOURS.map((c) => html`<option value="${c}">${t(`colour.${c}`)}</option>`)}</lds-select>
                ${edit ? html`<lds-button size="sm" variant="ghost" data-option-remove="${i}" label="${t("grants.remove")}"><lds-icon name="x"></lds-icon></lds-button>` : html`<span></span>`}
              </div>`)}</div>
            ${edit ? html`<lds-button size="sm" variant="ghost" data-action="add-option"><lds-icon slot="start" name="plus"></lds-icon>${t("fields.addOption")}</lds-button>` : ""}
          </div>` : ""}
        ${edit ? html`<div class="full"><lds-button data-action="save-field">${t("account.save")}</lds-button> <span class="muted" role="status" data-saved></span></div>` : ""}
      </form>`;
  }

  bindFields() {
    const root = this.shadowRoot;
    root.querySelector("[data-attach]")?.addEventListener("lds-change", (e) => {
      if (e.detail.value) this.setTypeFields([...this.typeFields, { field_id: e.detail.value, required: false }]);
    });
    for (const sw of root.querySelectorAll("[data-required]")) {
      sw.addEventListener("lds-change", (e) => {
        const list = structuredClone(this.typeFields);
        list[Number(sw.dataset.required)].required = e.detail.checked;
        this.setTypeFields(list);
      });
    }
    for (const sw of root.querySelectorAll("[data-field-toggle]")) {
      sw.addEventListener("lds-change", (e) => this.toggleField(sw.dataset.fieldToggle, e.detail.checked));
    }
  }

  async setTypeFields(list) {
    const { api, i18n } = this.context;
    try {
      await api.put(`${this.scope.base}/issue-types/${this.current.id}/fields`, { body: { fields: list } });
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
    this.load();
  }

  async toggleField(id, on) {
    const { api, i18n } = this.context;
    try {
      await api.post(`${this.scope.base}/fields/${id}/${on ? "enable" : "disable"}`);
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
    this.load();
  }

  async saveField() {
    const { api, i18n } = this.context;
    const f = this.fields.find((x) => x.id === this.selectedField);
    const form = this.shadowRoot.querySelector("[data-field-form]");
    clearFieldErrors(form);
    form.querySelector("lds-alert").textContent = "";
    const v = formValues(form);
    const body = { name: v.name, description: v.description };
    if (f.type === "number") Object.assign(body, { unit: v.unit ?? "", precision: Number(v.precision ?? 0) });
    if (f.type === "select" || f.type === "multi_select") {
      body.options = [...form.querySelectorAll("[data-option]")].map((row) => ({
        id: row.dataset.option.startsWith("new-") ? undefined : row.dataset.option,
        name: row.querySelector("[data-option-name]").getAttribute("value") ?? "",
        colour: row.querySelector("[data-option-colour]").getAttribute("value") ?? "grey",
      }));
    }
    try {
      await busy(this.shadowRoot.querySelector('[data-action="save-field"]'), () =>
        api.patch(`${this.scope.base}/fields/${f.id}`, { body, ifMatch: `"${f.version}"` }));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      if (err.fields?.length || !err.fields) form.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      return;
    }
    await this.load();
    const saved = this.shadowRoot.querySelector("[data-saved]");
    if (saved) saved.textContent = i18n.t("account.saved");
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
      <lds-modal data-dialog="new-field" size="sm">
        <span slot="header">${t("fields.new")}</span>
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <lds-field label="${t("types.name")}" required><lds-input name="name" required maxlength="60"></lds-input></lds-field>
          <lds-field label="${t("types.fieldType")}">
            <lds-select name="type" value="text">${FIELD_TYPES.map((ft) => html`<option value="${ft}">${t(`fieldType.${ft}`)}</option>`)}</lds-select>
            <span slot="help">${t("fields.typeFixed")}</span>
          </lds-field>
        </form>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button data-action="create-field">${t("fields.create")}</lds-button>
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
      this.load(sel.dataset.select);
      return;
    }
    const selField = e.target.closest?.("[data-select-field]");
    if (selField && !e.target.closest("lds-switch")) {
      this.selectedField = selField.dataset.selectField;
      return this.render();
    }
    const field = e.target.closest?.("[data-detach], [data-field-up], [data-option-remove]");
    if (field) {
      const list = structuredClone(this.typeFields);
      if (field.dataset.detach) {
        list.splice(Number(field.dataset.detach), 1);
        return this.setTypeFields(list);
      }
      if (field.dataset.fieldUp) {
        const i = Number(field.dataset.fieldUp);
        [list[i - 1], list[i]] = [list[i], list[i - 1]];
        return this.setTypeFields(list);
      }
      field.closest("[data-option]").remove();
      return;
    }
    const target = e.target.closest?.("[data-action], [data-close]");
    if (!target) return;
    const action = target.dataset.action;
    if (action === "new-field") return this.shadowRoot.querySelector('[data-dialog="new-field"]').setAttribute("open", "");
    if (action === "create-field") return this.createField();
    if (action === "save-field") return this.saveField();
    if (action === "add-option") {
      const box = this.shadowRoot.querySelector("[data-options]");
      const { t } = this.context.i18n;
      box.insertAdjacentHTML("beforeend", String(html`
        <div class="option" data-option="new-${box.children.length}">
          <lds-input size="sm" data-option-name value="" maxlength="60" label="${t("fields.optionName")}"></lds-input>
          <lds-select size="sm" data-option-colour value="grey">${COLOURS.map((c) => html`<option value="${c}">${t(`colour.${c}`)}</option>`)}</lds-select>
          <lds-button size="sm" variant="ghost" data-option-remove="${box.children.length}" label="${t("grants.remove")}"><lds-icon name="x"></lds-icon></lds-button>
        </div>`));
      box.lastElementChild.querySelector("lds-input").focus();
      return;
    }
    if (action === "delete-field") {
      const f = this.fields.find((x) => x.id === this.selectedField);
      if (confirm(this.context.i18n.t("fields.deleteConfirm", { name: f.name }))) {
        this.context.api.delete(`${this.scope.base}/fields/${f.id}`)
          .catch((err) => globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, this.context.i18n) }))
          .then(() => this.load());
      }
      return;
    }
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

  async createField() {
    const { api, i18n } = this.context;
    const modal = this.shadowRoot.querySelector('[data-dialog="new-field"]');
    const form = modal.querySelector("form");
    clearFieldErrors(form);
    let created;
    try {
      ({ data: created } = await busy(modal.querySelector('[data-action="create-field"]'), () => api.post(`${this.scope.base}/fields`, { body: formValues(form) })));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else form.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      return;
    }
    modal.removeAttribute("open");
    this.load(created.id);
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
