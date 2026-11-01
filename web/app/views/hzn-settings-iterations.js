// <hzn-settings-iterations>: /w/:ws/settings/iterations[?project=KEY] —
// what completing an iteration does, how many may be active at once, and
// the length of new ones (wireframes: IterationSettings). Each setting is
// the workspace's unless the project overrides it.

import { requestContext } from "horizon/core/context.js";
import { html, ldsLightSheet, sheet } from "horizon/core/dom.js";
import { busy, errorMessage } from "horizon/core/forms.js";
import { loadScope, scopePicker, watchScope } from "horizon/views/config-scope.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

const POLICIES = ["archive_done_rollover_rest", "archive_all", "none"];
const LENGTHS = [1, 2, 3, 4];
// The settings, as the API names them.
const KEYS = ["completion_policy", "max_active_iterations", "iteration_length_weeks"];

const styles = sheet(`
  .page-body { display: grid; gap: var(--lds-space-5); max-width: 48rem; }
  section { display: grid; gap: var(--lds-space-3); }
  .section-head { display: flex; align-items: center; justify-content: space-between; gap: var(--lds-space-4); }
  h2 { margin: 0; font-size: var(--lds-font-size-md); }
  p { margin: 0; }
  lds-input[type="number"] { max-width: 8rem; }
`);

class HznSettingsIterations extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [ldsLightSheet(), settingsStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    this.unsubscribe = this.context.router.subscribe((m) => m.route?.name === "settings-iterations" && this.load());
    this.load();
  }

  disconnectedCallback() {
    this.unsubscribe?.();
  }

  async load() {
    const { api, i18n } = this.context;
    try {
      this.scope = await loadScope(this.context, this.getAttribute("ws"));
      const { data } = await api.get(`${this.scope.base}/settings/iterations`);
      this.saved = data;
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    // The edited state: for each setting, whether the scope sets it, and the value.
    this.edit = Object.fromEntries(KEYS.map((k) => [k, { set: !this.scope.project || this.saved[k].overridden, value: this.saved[k].value }]));
    this.render();
  }

  get dirty() {
    return KEYS.some((k) => {
      const e = this.edit[k];
      const s = this.saved[k];
      return (this.scope.project && e.set !== s.overridden) || (e.set && e.value !== s.value);
    });
  }

  render() {
    const { t } = this.context.i18n;
    const scope = this.scope;
    const can = this.saved.can_manage;
    const section = (key, id, control, note) => {
      const e = this.edit[key];
      const shown = e.set ? e.value : this.saved[key].workspace;
      return html`
        <section aria-labelledby="it-${id}">
          <div class="section-head">
            <h2 id="it-${id}">${t(`iterationSettings.heading.${id}`)}</h2>
            ${scope.project ? html`<lds-switch data-override="${key}" ${e.set ? html`checked` : ""} ${can ? "" : html`disabled`}>${t("iterationSettings.override", { name: scope.project.name })}</lds-switch>` : ""}
          </div>
          ${control(shown, can && e.set)}
          <p class="muted">${scope.project && !e.set ? t("iterationSettings.inherited", { name: scope.ws.name }) + " " : ""}${note}</p>
        </section>`;
    };
    const body = html`
      <div class="page-body">
        ${section("completion_policy", "completion", (v, on) => html`
          <lds-radio-group data-setting="completion_policy" value="${v}" label="${t("iterationSettings.policy")}" ${on ? "" : html`disabled`}>
            ${POLICIES.map((p) => html`<button type="button" value="${p}">${t(`iterationSettings.policies.${p}`)}</button>`)}
          </lds-radio-group>`, t("iterationSettings.completionNote"))}
        <lds-divider></lds-divider>
        ${section("max_active_iterations", "active", (v, on) => html`
          <lds-field label="${t("iterationSettings.maxActive")}">
            <lds-input type="number" size="sm" min="1" max="10" data-setting="max_active_iterations" value="${v}" ${on ? "" : html`readonly`}></lds-input>
          </lds-field>`, t("iterationSettings.activeNote", { count: this.saved.max_active_iterations.workspace, name: scope.ws.name }))}
        <lds-divider></lds-divider>
        ${section("iteration_length_weeks", "length", (v, on) => html`
          <lds-field label="${t("iterationSettings.length")}">
            <lds-segmented data-setting="iteration_length_weeks" size="sm" value="${v}" label="${t("iterationSettings.length")}" ${on ? "" : html`disabled`}>
              ${LENGTHS.map((n) => html`<lds-segment value="${n}">${t("iterationSettings.weeks", { count: n })}</lds-segment>`)}
            </lds-segmented>
          </lds-field>`, t("iterationSettings.lengthNote"))}
      </div>`;
    const actions = html`${scopePicker(t, scope)}
      ${can ? html`<lds-button variant="ghost" data-action="discard" ${this.dirty ? "" : html`disabled`}>${t("roles.discard")}</lds-button>
        <lds-button data-action="save" ${this.dirty ? "" : html`disabled`}>${t("account.save")}</lds-button>` : ""}`;
    this.shadowRoot.innerHTML = String(settingsPage(t, scope.ws, "iterations", body, actions));
    watchScope(this.shadowRoot, this.context, scope.ws.slug, "iterations");
    this.bind();
  }

  bind() {
    const root = this.shadowRoot;
    for (const sw of root.querySelectorAll("[data-override]")) {
      sw.addEventListener("lds-change", (e) => {
        const edit = this.edit[sw.dataset.override];
        edit.set = e.detail.checked;
        if (edit.set && !this.saved[sw.dataset.override].overridden) edit.value = this.saved[sw.dataset.override].workspace;
        this.render();
      });
    }
    for (const el of root.querySelectorAll("[data-setting]")) {
      const event = el.matches("lds-input") ? "lds-input" : "lds-change";
      el.addEventListener(event, (e) => {
        const key = el.dataset.setting;
        this.edit[key].value = key === "completion_policy" ? e.detail.value : Number(e.detail.value);
        for (const b of root.querySelectorAll('[data-action="save"], [data-action="discard"]')) b.toggleAttribute("disabled", !this.dirty);
      });
    }
  }

  onClick(e) {
    const el = e.target.closest?.("[data-action]");
    if (!el || el.hasAttribute("disabled")) return;
    if (el.dataset.action === "discard") return this.load();
    if (el.dataset.action === "save") this.save(el);
  }

  async save(button) {
    const { api, i18n } = this.context;
    const body = {};
    for (const k of KEYS) {
      const e = this.edit[k];
      const s = this.saved[k];
      if (!e.set && s.overridden) body[k] = null;
      else if (e.set && (e.value !== s.value || (this.scope.project && !s.overridden))) body[k] = e.value;
    }
    try {
      await busy(button, () => api.put(`${this.scope.base}/settings/iterations`, { body }));
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      return;
    }
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("account.saved") });
    this.load();
  }
}

customElements.define("hzn-settings-iterations", HznSettingsIterations);
