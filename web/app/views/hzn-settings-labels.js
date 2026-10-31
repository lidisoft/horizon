// <hzn-settings-labels>: /w/:ws/settings/labels[?project=KEY] — labels
// (wireframes: Labels). Inherited labels can be switched off in a project:
// issues keep them, greyed. Merging relabels issues; a project label can
// move to the workspace.

import { COLOURS, paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, ldsLightSheet, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, formValues, showFieldErrors } from "horizon/core/forms.js";
import { loadScope, scopePicker, watchScope } from "horizon/views/config-scope.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

const styles = sheet(`
  .toolbar { display: flex; gap: var(--lds-space-3); align-items: center; margin-bottom: var(--lds-space-4); flex-wrap: wrap; }
  .toolbar lds-input { width: 16rem; }
  .toolbar .hint { margin-inline-start: auto; }
  .dot { display: inline-block; width: 0.625rem; height: 0.625rem; border-radius: var(--lds-radius-full); margin-inline-end: var(--lds-space-2); }
  tr.off td { color: var(--lds-color-fg-subtle); }
  td.num { text-align: end; }
  td.end { text-align: end; white-space: nowrap; width: 1%; }
  lds-modal form { display: grid; gap: var(--lds-space-4); }
`);

class HznSettingsLabels extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [ldsLightSheet(), settingsStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("lds-select", (e) => this.onMenu(e));
    this.filter = { text: "", source: "all" };
    this.unsubscribe = this.context.router.subscribe((m) => m.route?.name === "settings-labels" && this.load());
    this.load();
  }

  disconnectedCallback() {
    this.unsubscribe?.();
  }

  async load() {
    const { api, i18n } = this.context;
    try {
      this.scope = await loadScope(this.context, this.getAttribute("ws"));
      const { data } = await api.get(`${this.scope.base}/labels`);
      this.items = data.items;
      this.canManage = data.can_manage;
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    this.render();
  }

  render() {
    const { t } = this.context.i18n;
    const scope = this.scope;
    const inherited = this.items.filter((l) => l.inherited).length;
    const own = this.items.length - inherited;
    const body = html`
      <div class="toolbar">
        <lds-input data-filter type="search" size="sm" placeholder="${t("labels.filter")}" value="${this.filter.text}"><lds-icon slot="prefix" name="search"></lds-icon></lds-input>
        ${scope.project ? html`
          <lds-segmented size="sm" data-source value="${this.filter.source}" label="${t("labels.source")}">
            <lds-segment value="all">${t("labels.all", { count: this.items.length })}</lds-segment>
            <lds-segment value="inherited">${t("labels.fromWorkspace", { name: scope.ws.name, count: inherited })}</lds-segment>
            <lds-segment value="own">${t("labels.projectOnly", { name: scope.project.name, count: own })}</lds-segment>
          </lds-segmented>
          <span class="muted hint">${t("labels.hint", { name: scope.ws.name })}</span>` : ""}
      </div>
      <lds-table density="compact">
        <table>
          <thead><tr>
            <th scope="col">${t("labels.label")}</th><th scope="col">${t("labels.description")}</th>
            <th scope="col">${t("labels.definedIn")}</th><th scope="col">${t("labels.issues")}</th>
            ${scope.project ? html`<th scope="col">${t("labels.useIn", { name: scope.project.name })}</th>` : ""}
            <th scope="col"><span class="lds-visually-hidden">${t("account.actions")}</span></th>
          </tr></thead>
          <tbody data-rows></tbody>
        </table>
      </lds-table>
      <p class="muted">${t("labels.note")}</p>
      ${this.dialogs()}`;
    const actions = html`${scopePicker(t, scope)}
      ${this.canManage ? html`<lds-button data-action="new"><lds-icon slot="start" name="plus"></lds-icon>${t("labels.new")}</lds-button>` : ""}`;
    this.shadowRoot.innerHTML = String(settingsPage(t, scope.ws, "labels", body, actions));
    watchScope(this.shadowRoot, this.context, scope.ws.slug, "labels");
    this.shadowRoot.querySelector("[data-filter]").addEventListener("lds-input", (e) => {
      this.filter.text = e.detail.value;
      this.renderRows();
    });
    this.shadowRoot.querySelector("[data-source]")?.addEventListener("lds-change", (e) => {
      this.filter.source = e.detail.value;
      this.renderRows();
    });
    this.renderRows();
  }

  renderRows() {
    const { t } = this.context.i18n;
    const scope = this.scope;
    const text = this.filter.text.trim().toLowerCase();
    const rows = this.items.filter((l) =>
      (this.filter.source === "all" || (this.filter.source === "inherited") === l.inherited) &&
      (!text || l.name.toLowerCase().includes(text) || l.description.toLowerCase().includes(text)));
    const body = this.shadowRoot.querySelector("[data-rows]");
    body.innerHTML = String(html`${rows.map((l) => html`
      <tr class="${l.disabled ? "off" : ""}">
        <td><span class="dot" data-colour="${l.colour}"></span>${l.name}</td>
        <td>${l.description}</td>
        <td>${l.project_id ? scope.project?.name : t("config.workspaceScope", { name: scope.ws.name })}</td>
        <td class="num">${l.issues}</td>
        ${scope.project ? html`<td>${l.inherited
          ? html`<lds-switch data-toggle="${l.id}" ${l.disabled ? "" : html`checked`} ${this.canManage ? "" : html`disabled`} label="${t("labels.useLabel", { name: l.name })}"></lds-switch>`
          : t("labels.always")}</td>` : ""}
        <td class="end">${this.menu(l)}</td>
      </tr>`)}`);
    paintSwatches(body);
    for (const sw of body.querySelectorAll("[data-toggle]")) {
      sw.addEventListener("lds-change", (e) => this.toggle(sw.dataset.toggle, e.detail.checked));
    }
  }

  menu(l) {
    if (!this.canManage || l.inherited) return "";
    const { t } = this.context.i18n;
    return html`
      <lds-menu placement="bottom-end" data-id="${l.id}">
        <lds-button slot="trigger" size="sm" variant="ghost" label="${t("members.actionsFor", { name: l.name })}"><lds-icon name="more-horizontal"></lds-icon></lds-button>
        <lds-menu-item value="edit">${t("labels.edit")}</lds-menu-item>
        <lds-menu-item value="merge">${t("labels.merge")}</lds-menu-item>
        ${this.scope.project ? html`<lds-menu-item value="move">${t("labels.moveToWorkspace")}</lds-menu-item>` : ""}
        <lds-menu-item value="delete">${t("labels.delete")}</lds-menu-item>
      </lds-menu>`;
  }

  dialogs() {
    if (!this.canManage) return "";
    const { t } = this.context.i18n;
    return html`
      <lds-modal data-dialog="edit" size="sm">
        <span slot="header" data-title></span>
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <lds-field label="${t("labels.name")}" required><lds-input name="name" required maxlength="40"></lds-input></lds-field>
          <lds-field label="${t("projects.colour")}">
            <lds-select name="colour" value="grey">${COLOURS.map((c) => html`<option value="${c}">${t(`colour.${c}`)}</option>`)}</lds-select>
          </lds-field>
          <lds-field label="${t("labels.description")}"><lds-input name="description" maxlength="200"></lds-input></lds-field>
        </form>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button data-action="save">${t("account.save")}</lds-button>
        </div>
      </lds-modal>
      <lds-modal data-dialog="merge" size="sm">
        <span slot="header" data-title></span>
        <p class="muted" data-body></p>
        <lds-field label="${t("labels.mergeInto")}"><lds-select data-into value=""></lds-select></lds-field>
        <lds-alert intent="danger" role="alert"></lds-alert>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button data-action="merge">${t("labels.merge")}</lds-button>
        </div>
      </lds-modal>`;
  }

  dialog(name) {
    return this.shadowRoot.querySelector(`[data-dialog="${name}"]`);
  }

  onClick(e) {
    const el = e.target.closest?.("[data-action], [data-close]");
    if (!el) return;
    if (el.hasAttribute("data-close")) return el.closest("lds-modal").removeAttribute("open");
    if (el.dataset.action === "new") this.openEdit(null);
    if (el.dataset.action === "save") this.save();
    if (el.dataset.action === "merge") this.merge();
  }

  onMenu(e) {
    const menu = e.target.closest?.("lds-menu[data-id]");
    const value = e.detail?.value ?? e.target.getAttribute?.("value");
    if (!menu || !value) return;
    const { api, i18n } = this.context;
    const l = this.items.find((x) => x.id === menu.dataset.id);
    const run = (fn) => fn().catch((err) => globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) })).then(() => this.load());
    if (value === "edit") this.openEdit(l);
    if (value === "merge") this.openMerge(l);
    if (value === "move") run(() => api.post(`${this.scope.base}/labels/${l.id}/move-to-workspace`));
    if (value === "delete" && confirm(i18n.t("labels.deleteConfirm", { name: l.name, count: l.issues }))) run(() => api.delete(`${this.scope.base}/labels/${l.id}`));
  }

  async toggle(id, on) {
    const { api, i18n } = this.context;
    try {
      await api.post(`${this.scope.base}/labels/${id}/${on ? "enable" : "disable"}`);
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
    this.load();
  }

  openEdit(l) {
    const { t } = this.context.i18n;
    this.editing = l;
    const modal = this.dialog("edit");
    const form = modal.querySelector("form");
    clearFieldErrors(form);
    form.querySelector("lds-alert").textContent = "";
    modal.querySelector("[data-title]").textContent = l ? t("labels.editTitle", { name: l.name }) : t("labels.new");
    form.querySelector('[name="name"]').setAttribute("value", l?.name ?? "");
    form.querySelector('[name="colour"]').setAttribute("value", l?.colour ?? "grey");
    form.querySelector('[name="description"]').setAttribute("value", l?.description ?? "");
    modal.setAttribute("open", "");
  }

  async save() {
    const { api, i18n } = this.context;
    const modal = this.dialog("edit");
    const form = modal.querySelector("form");
    clearFieldErrors(form);
    const body = formValues(form);
    try {
      await busy(modal.querySelector('[data-action="save"]'), () => this.editing
        ? api.patch(`${this.scope.base}/labels/${this.editing.id}`, { body, ifMatch: `"${this.editing.version}"` })
        : api.post(`${this.scope.base}/labels`, { body }));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else form.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      return;
    }
    modal.removeAttribute("open");
    this.load();
  }

  openMerge(l) {
    const { t } = this.context.i18n;
    this.merging = l;
    const modal = this.dialog("merge");
    modal.querySelector("[data-title]").textContent = t("labels.mergeTitle", { name: l.name });
    modal.querySelector("[data-body]").textContent = t("labels.mergeBody", { name: l.name, count: l.issues });
    // A workspace label can only merge into another workspace label.
    const targets = this.items.filter((x) => x.id !== l.id && !x.disabled && (l.project_id || !x.project_id));
    const select = modal.querySelector("[data-into]");
    select.innerHTML = String(html`${targets.map((x) => html`<option value="${x.id}">${x.name}</option>`)}`);
    select.setAttribute("value", targets[0]?.id ?? "");
    modal.querySelector("lds-alert").textContent = "";
    modal.setAttribute("open", "");
  }

  async merge() {
    const { api, i18n } = this.context;
    const modal = this.dialog("merge");
    try {
      await busy(modal.querySelector('[data-action="merge"]'), () =>
        api.post(`${this.scope.base}/labels/${this.merging.id}/merge`, { body: { into: modal.querySelector("[data-into]").getAttribute("value") } }));
    } catch (err) {
      modal.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      return;
    }
    modal.removeAttribute("open");
    this.load();
  }
}

customElements.define("hzn-settings-labels", HznSettingsLabels);
