// <hzn-settings-estimation>: /w/:ws/settings/estimation[?project=KEY] —
// the estimation scale (wireframes: Estimation). A project follows the
// workspace's scale unless it overrides it. T-shirt and custom scales carry
// labels with weights, which velocity counts.

import { requestContext } from "horizon/core/context.js";
import { html, ldsLightSheet, sheet } from "horizon/core/dom.js";
import { busy, errorMessage } from "horizon/core/forms.js";
import { loadScope, scopePicker, watchScope } from "horizon/views/config-scope.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

const KINDS = ["fibonacci", "linear", "tshirt", "custom", "none"];
const TSHIRT = [{ label: "XS", weight: 1 }, { label: "S", weight: 2 }, { label: "M", weight: 3 }, { label: "L", weight: 5 }, { label: "XL", weight: 8 }];

const styles = sheet(`
  .page-body { display: grid; gap: var(--lds-space-5); max-width: 62rem; }
  .override { display: flex; gap: var(--lds-space-4); align-items: center; padding: var(--lds-space-3) var(--lds-space-4);
    border-radius: var(--lds-radius-md); background: var(--lds-color-bg-subtle); }
  h3 { margin: 0; font-size: var(--lds-font-size-md); }
  .values td:last-child { text-align: end; width: 1%; }
  .values lds-input { width: 100%; }
`);

class HznSettingsEstimation extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [ldsLightSheet(), settingsStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    this.unsubscribe = this.context.router.subscribe((m) => m.route?.name === "settings-estimation" && this.load());
    this.load();
  }

  disconnectedCallback() {
    this.unsubscribe?.();
  }

  async load() {
    const { api, i18n } = this.context;
    try {
      this.scope = await loadScope(this.context, this.getAttribute("ws"));
      const { data } = await api.get(`${this.scope.base}/settings/estimation`);
      this.saved = data;
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    // The edited state: override flag and scale.
    this.override = !this.scope.project || this.saved.overridden;
    this.scale = structuredClone({ kind: this.saved.kind, values: this.saved.values });
    this.render();
  }

  get dirty() {
    const s = this.saved;
    const overrideChanged = this.scope.project && this.override !== s.overridden;
    return overrideChanged || (this.override && JSON.stringify(this.scale) !== JSON.stringify({ kind: s.kind, values: s.values }));
  }

  render() {
    const { t } = this.context.i18n;
    const scope = this.scope;
    const can = this.saved.can_manage;
    const editable = can && this.override;
    const shown = this.override ? this.scale : this.saved.workspace;
    const weighted = shown.kind === "tshirt" || shown.kind === "custom";
    const body = html`
      <div class="page-body">
        ${scope.project ? html`
          <div class="override">
            <lds-switch data-override ${this.override ? html`checked` : ""} ${can ? "" : html`disabled`}>${t("estimation.override", { name: scope.project.name })}</lds-switch>
            <span class="muted">${t("estimation.workspaceUses", { name: scope.ws.name, scale: t(`estimation.kind.${this.saved.workspace.kind}`), project: scope.project.name })}</span>
          </div>` : ""}
        <lds-radio-group data-kind value="${shown.kind}" ${editable ? "" : html`disabled`} label="${t("estimation.scale")}">
          ${KINDS.map((k) => html`<button type="button" value="${k}">${t(`estimation.kind.${k}`)} — ${t(`estimation.kindHelp.${k}`, { project: scope.project?.name ?? scope.ws.name })}</button>`)}
        </lds-radio-group>
        <lds-alert intent="info">${t("estimation.changeNote")}</lds-alert>
        ${weighted ? html`
          <h3>${t("estimation.values")}</h3>
          <lds-table density="compact" class="values">
            <table>
              <thead><tr><th scope="col">${t("estimation.label")}</th><th scope="col">${t("estimation.weight")}</th><th scope="col"><span class="lds-visually-hidden">${t("account.actions")}</span></th></tr></thead>
              <tbody>${shown.values.map((v, i) => html`
                <tr>
                  <td><lds-input size="sm" data-label="${i}" value="${v.label}" maxlength="10" ${editable ? "" : html`readonly`} label="${t("estimation.label")}"></lds-input></td>
                  <td><lds-input size="sm" type="number" data-weight="${i}" value="${v.weight}" ${editable ? "" : html`readonly`} label="${t("estimation.weight")}"><span slot="suffix">${t("estimation.pts")}</span></lds-input></td>
                  <td>${editable ? html`<lds-button size="sm" variant="ghost" data-remove="${i}" label="${t("grants.remove")}"><lds-icon name="x"></lds-icon></lds-button>` : ""}</td>
                </tr>`)}</tbody>
            </table>
          </lds-table>
          ${editable ? html`<div><lds-button variant="ghost" data-action="add"><lds-icon slot="start" name="plus"></lds-icon>${t("estimation.add")}</lds-button></div>` : ""}` : ""}
      </div>`;
    const actions = html`${scopePicker(t, scope)}
      ${can ? html`<lds-button variant="ghost" data-action="discard" ${this.dirty ? "" : html`disabled`}>${t("roles.discard")}</lds-button>
        <lds-button data-action="save" ${this.dirty ? "" : html`disabled`}>${t("account.save")}</lds-button>` : ""}`;
    this.shadowRoot.innerHTML = String(settingsPage(t, scope.ws, "estimation", body, actions));
    watchScope(this.shadowRoot, this.context, scope.ws.slug, "estimation");
    this.bind();
  }

  bind() {
    const root = this.shadowRoot;
    root.querySelector("[data-override]")?.addEventListener("lds-change", (e) => {
      this.override = e.detail.checked;
      if (this.override && !this.saved.overridden) this.scale = structuredClone(this.saved.workspace);
      this.render();
    });
    root.querySelector("[data-kind]")?.addEventListener("lds-change", (e) => {
      const kind = e.detail.value;
      const values = kind === "tshirt" ? structuredClone(TSHIRT) : kind === "custom" ? (this.scale.values.length ? this.scale.values : structuredClone(TSHIRT)) : [];
      this.scale = { kind, values };
      this.render();
    });
    for (const input of root.querySelectorAll("[data-label]")) {
      input.addEventListener("lds-input", (e) => {
        this.scale.values[input.dataset.label].label = e.detail.value;
        this.refreshButtons();
      });
    }
    for (const input of root.querySelectorAll("[data-weight]")) {
      input.addEventListener("lds-input", (e) => {
        this.scale.values[input.dataset.weight].weight = Number(e.detail.value);
        this.refreshButtons();
      });
    }
  }

  refreshButtons() {
    for (const b of this.shadowRoot.querySelectorAll('[data-action="save"], [data-action="discard"]')) b.toggleAttribute("disabled", !this.dirty);
  }

  onClick(e) {
    const el = e.target.closest?.("[data-action], [data-remove]");
    if (!el || el.hasAttribute("disabled")) return;
    if (el.dataset.remove) {
      this.scale.values.splice(Number(el.dataset.remove), 1);
      return this.render();
    }
    if (el.dataset.action === "add") {
      this.scale.values.push({ label: "", weight: 0 });
      return this.render();
    }
    if (el.dataset.action === "discard") return this.load();
    if (el.dataset.action === "save") this.save(el);
  }

  async save(button) {
    const { api, i18n } = this.context;
    const scale = this.override ? { kind: this.scale.kind, values: this.scale.values } : null;
    try {
      await busy(button, () => api.put(`${this.scope.base}/settings/estimation`, { body: { scale } }));
    } catch (err) {
      const f = err.fields?.[0];
      globalThis.LDS?.toast?.({ intent: "danger", title: f ? i18n.t("estimation.invalid", { field: f.field }) : errorMessage(err, i18n) });
      return;
    }
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("account.saved") });
    this.load();
  }
}

customElements.define("hzn-settings-estimation", HznSettingsEstimation);
