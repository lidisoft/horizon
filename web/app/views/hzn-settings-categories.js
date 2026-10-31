// <hzn-settings-categories>: /w/:ws/settings/categories[?project=KEY] —
// status categories and their outcomes (wireframes: StatusCategories).
// Deleting one in use asks for a replacement of the same kind.

import { requestContext } from "horizon/core/context.js";
import { html, ldsLightSheet, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, formValues, showFieldErrors } from "horizon/core/forms.js";
import { loadScope, originBadge, scopePicker, statusIcon, watchScope } from "horizon/views/config-scope.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

const OUTCOMES = ["unstarted", "started", "completed", "abandoned"];
const open = (o) => o === "unstarted" || o === "started";

const styles = sheet(`
  .layout { display: grid; grid-template-columns: minmax(0, 1fr) 20rem; gap: var(--lds-space-6); align-items: start; }
  .lead { margin: 0 0 var(--lds-space-4); max-width: 48rem; }
  .name { display: inline-flex; align-items: center; gap: var(--lds-space-2); }
  td.end { text-align: end; white-space: nowrap; width: 1%; }
  h3 { margin: var(--lds-space-6) 0 var(--lds-space-3); font-size: var(--lds-font-size-md); }
  .columns { display: flex; gap: var(--lds-space-3); overflow-x: auto; }
  .column { flex: 1; min-width: 7rem; padding: var(--lds-space-3); border-radius: var(--lds-radius-md);
    background: var(--lds-color-bg-subtle); font-size: var(--lds-font-size-sm); display: flex; gap: var(--lds-space-2); align-items: center; }
  aside h3 { margin-top: 0; }
  aside lds-card { margin-bottom: var(--lds-space-3); }
  aside p { margin: var(--lds-space-1) 0 0; color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  lds-modal form { display: grid; gap: var(--lds-space-4); }
`);

class HznSettingsCategories extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [ldsLightSheet(), settingsStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("lds-select", (e) => this.onMenu(e));
    this.unsubscribe = this.context.router.subscribe((m) => m.route?.name === "settings-categories" && this.load());
    this.load();
  }

  disconnectedCallback() {
    this.unsubscribe?.();
  }

  async load() {
    const { api, i18n } = this.context;
    try {
      this.scope = await loadScope(this.context, this.getAttribute("ws"));
      const { data } = await api.get(`${this.scope.base}/categories`);
      this.items = data.items;
      this.canManage = data.can_manage;
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    this.render();
  }

  editable(c) {
    return this.canManage && !c.inherited;
  }

  render() {
    const { t } = this.context.i18n;
    const scope = this.scope;
    const visible = this.items.filter((c) => !c.disabled);
    const body = html`
      <div class="layout">
        <div>
          <p class="muted lead">${t("categories.lead")}</p>
          <lds-table density="compact">
            <table>
              <thead><tr>
                <th scope="col">${t("categories.category")}</th>
                <th scope="col">${t("categories.outcome")}</th>
                <th scope="col">${t("categories.statuses")}</th>
                <th scope="col"><span class="lds-visually-hidden">${t("account.actions")}</span></th>
              </tr></thead>
              <tbody>${this.items.map((c) => html`
                <tr>
                  <td><span class="name"><lds-icon name="${statusIcon(c.outcome)}"></lds-icon>${c.name} ${originBadge(t, c, scope)}</span></td>
                  <td>${this.editable(c) ? html`
                    <lds-select size="sm" data-outcome="${c.id}" value="${c.outcome}" label="${t("categories.outcomeOf", { name: c.name })}">
                      ${OUTCOMES.map((o) => html`<option value="${o}">${t(`outcome.${o}`)} · ${t(open(o) ? "outcome.open" : "outcome.final")}</option>`)}
                    </lds-select>` : html`${t(`outcome.${c.outcome}`)} · ${t(open(c.outcome) ? "outcome.open" : "outcome.final")}`}</td>
                  <td class="muted">${c.statuses.length ? t("categories.usage", { statuses: c.statuses.join(", "), count: c.workflows }) : "—"}</td>
                  <td class="end">${this.menu(c)}</td>
                </tr>`)}</tbody>
            </table>
          </lds-table>
          <h3>${t("categories.columns")}</h3>
          <div class="columns">${visible.map((c) => html`<div class="column"><lds-icon name="${statusIcon(c.outcome)}"></lds-icon>${c.name}</div>`)}</div>
        </div>
        <aside>
          <h3>${t("categories.drives")}</h3>
          ${OUTCOMES.map((o) => html`<lds-card><strong><lds-icon name="${statusIcon(o)}"></lds-icon> ${t(`outcome.${o}`)}</strong><p>${t(`outcome.${o}.help`)}</p></lds-card>`)}
        </aside>
      </div>
      ${this.dialogs()}`;
    const actions = html`${scopePicker(t, scope)}
      ${this.canManage ? html`<lds-button data-action="new"><lds-icon slot="start" name="plus"></lds-icon>${t("categories.new")}</lds-button>` : ""}`;
    this.shadowRoot.innerHTML = String(settingsPage(t, scope.ws, "categories", body, actions));
    watchScope(this.shadowRoot, this.context, scope.ws.slug, "categories");
    for (const select of this.shadowRoot.querySelectorAll("[data-outcome]")) {
      select.addEventListener("lds-change", (e) => this.update(select.dataset.outcome, { outcome: e.detail.value }));
    }
  }

  menu(c) {
    const { t } = this.context.i18n;
    const items = [];
    if (this.editable(c)) {
      const own = this.items.filter((x) => !x.inherited || !this.scope.project);
      const idx = own.indexOf(c);
      items.push(["rename", "categories.rename"]);
      if (idx > 0) items.push(["up", "categories.up"]);
      if (idx >= 0 && idx < own.length - 1) items.push(["down", "categories.down"]);
      items.push(["delete", "categories.delete"]);
    } else if (this.canManage && c.inherited) {
      items.push(c.disabled ? ["enable", "config.switchOn"] : ["disable", "config.switchOff"]);
    }
    if (!items.length) return "";
    return html`
      <lds-menu placement="bottom-end" data-id="${c.id}">
        <lds-button slot="trigger" size="sm" variant="ghost" label="${t("members.actionsFor", { name: c.name })}"><lds-icon name="more-horizontal"></lds-icon></lds-button>
        ${items.map(([v, l]) => html`<lds-menu-item value="${v}">${t(l)}</lds-menu-item>`)}
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
          <lds-field label="${t("categories.name")}" required><lds-input name="name" required maxlength="40"></lds-input></lds-field>
          <lds-field label="${t("categories.outcome")}" data-outcome-field>
            <lds-select name="outcome" value="started">${OUTCOMES.map((o) => html`<option value="${o}">${t(`outcome.${o}`)}</option>`)}</lds-select>
          </lds-field>
        </form>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button data-action="save">${t("account.save")}</lds-button>
        </div>
      </lds-modal>
      <lds-modal data-dialog="delete" size="sm">
        <span slot="header" data-title></span>
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <p class="muted" data-body></p>
          <lds-field label="${t("categories.replacement")}" data-replacement-field>
            <lds-select name="replacement" value=""></lds-select>
          </lds-field>
        </form>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button variant="danger" data-action="delete">${t("categories.delete")}</lds-button>
        </div>
      </lds-modal>`;
  }

  dialog(name) {
    return this.shadowRoot.querySelector(`[data-dialog="${name}"]`);
  }

  onClick(e) {
    const target = e.target.closest?.("[data-action], [data-close]");
    if (!target) return;
    if (target.hasAttribute("data-close")) return target.closest("lds-modal").removeAttribute("open");
    if (target.dataset.action === "new") this.openEdit(null);
    if (target.dataset.action === "save") this.save();
    if (target.dataset.action === "delete") this.remove();
  }

  onMenu(e) {
    const menu = e.target.closest?.("lds-menu[data-id]");
    const value = e.detail?.value ?? e.target.getAttribute?.("value");
    if (!menu || !value) return;
    const c = this.items.find((x) => x.id === menu.dataset.id);
    const run = (fn) => fn().catch((err) => globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, this.context.i18n) })).then(() => this.load());
    const api = this.context.api;
    if (value === "rename") this.openEdit(c);
    else if (value === "delete") this.openDelete(c);
    else if (value === "disable" || value === "enable") run(() => api.post(`${this.scope.base}/categories/${c.id}/${value}`));
    else if (value === "up" || value === "down") {
      const own = this.items.filter((x) => !x.inherited || !this.scope.project).map((x) => x.id);
      const i = own.indexOf(c.id);
      const j = value === "up" ? i - 1 : i + 1;
      [own[i], own[j]] = [own[j], own[i]];
      run(() => api.put(`${this.scope.base}/categories/order`, { body: { ids: own } }));
    }
  }

  async update(id, body) {
    const { api, i18n } = this.context;
    const c = this.items.find((x) => x.id === id);
    try {
      await api.patch(`${this.scope.base}/categories/${id}`, { body, ifMatch: `"${c.version}"` });
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
    this.load();
  }

  openEdit(c) {
    const { t } = this.context.i18n;
    this.editing = c;
    const modal = this.dialog("edit");
    const form = modal.querySelector("form");
    clearFieldErrors(form);
    form.querySelector("lds-alert").textContent = "";
    modal.querySelector("[data-title]").textContent = c ? t("categories.renameTitle", { name: c.name }) : t("categories.new");
    form.querySelector('[name="name"]').setAttribute("value", c?.name ?? "");
    form.querySelector("[data-outcome-field]").hidden = !!c;
    modal.setAttribute("open", "");
  }

  async save() {
    const { api, i18n } = this.context;
    const modal = this.dialog("edit");
    const form = modal.querySelector("form");
    clearFieldErrors(form);
    const { name = "", outcome } = formValues(form);
    try {
      await busy(modal.querySelector('[data-action="save"]'), () => this.editing
        ? api.patch(`${this.scope.base}/categories/${this.editing.id}`, { body: { name }, ifMatch: `"${this.editing.version}"` })
        : api.post(`${this.scope.base}/categories`, { body: { name, outcome } }));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else form.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      return;
    }
    modal.removeAttribute("open");
    this.load();
  }

  openDelete(c) {
    const { t } = this.context.i18n;
    this.deleting = c;
    const modal = this.dialog("delete");
    const inUse = c.statuses.length > 0;
    modal.querySelector("[data-title]").textContent = t("categories.deleteTitle", { name: c.name });
    modal.querySelector("[data-body]").textContent = inUse ? t("categories.deleteInUse", { count: c.workflows }) : t("categories.deleteUnused");
    const field = modal.querySelector("[data-replacement-field]");
    field.hidden = !inUse;
    const select = field.querySelector("lds-select");
    // Same kind of outcome; a workspace category is replaced by a workspace one.
    const options = this.items.filter((x) => x.id !== c.id && open(x.outcome) === open(c.outcome) && (!x.project_id || x.project_id === c.project_id));
    select.innerHTML = String(html`${options.map((x) => html`<option value="${x.id}">${x.name}</option>`)}`);
    select.setAttribute("value", options[0]?.id ?? "");
    modal.querySelector("lds-alert").textContent = "";
    modal.setAttribute("open", "");
  }

  async remove() {
    const { api, i18n } = this.context;
    const modal = this.dialog("delete");
    const replacement = this.deleting.statuses.length ? modal.querySelector('[name="replacement"]').getAttribute("value") : "";
    try {
      await busy(modal.querySelector('[data-action="delete"]'), () =>
        api.delete(`${this.scope.base}/categories/${this.deleting.id}`, { query: { replacement: replacement || undefined } }));
    } catch (err) {
      modal.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      return;
    }
    modal.removeAttribute("open");
    this.load();
  }
}

customElements.define("hzn-settings-categories", HznSettingsCategories);
