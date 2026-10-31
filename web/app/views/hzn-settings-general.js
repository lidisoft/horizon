// <hzn-settings-general>: /w/:ws/settings/general — the workspace's name,
// URL and time zone. Read-only without workspace.manage.

import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, formValues, showFieldErrors, submitOnEnter } from "horizon/core/forms.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

const styles = sheet(`
  form { display: grid; gap: var(--lds-space-4); max-width: 38rem; }
  h3 { margin: 0; font-size: var(--lds-font-size-md); }
`);

class HznSettingsGeneral extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    this.attachShadow({ mode: "open" }).adoptedStyleSheets = [settingsStyles, styles];
    this.load();
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.load();
  }

  async load() {
    const { api, i18n } = this.context;
    const { t } = i18n;
    let ws;
    try {
      const res = await api.get(`/w/${encodeURIComponent(this.getAttribute("ws"))}`);
      ws = res.data;
      this.etag = res.etag;
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    this.ws = ws;
    const canEdit = ws.can.includes("workspace.manage");
    const ro = canEdit ? "" : html`readonly`;
    const zones = Intl.supportedValuesOf?.("timeZone") ?? [ws.time_zone];
    const origin = `${location.host}/w/`;
    this.shadowRoot.innerHTML = String(settingsPage(t, ws, "general", html`
      <form novalidate>
        <h3>${t("settings.workspace")}</h3>
        <lds-alert intent="danger" role="alert"></lds-alert>
        ${canEdit ? "" : html`<p class="muted">${t("settings.readOnly")}</p>`}
        <lds-field label="${t("field.workspaceName")}" required>
          <lds-input name="name" required maxlength="100" value="${ws.name}" ${ro}></lds-input>
        </lds-field>
        <lds-field label="${t("field.workspaceUrl")}" required>
          <lds-input name="slug" required maxlength="40" value="${ws.slug}" ${ro}><span slot="prefix">${origin}</span></lds-input>
          <span slot="help">${t("settings.slugHelp")}</span>
        </lds-field>
        <lds-field label="${t("field.timezone")}">
          <lds-combobox name="time_zone" value="${ws.time_zone}" ${canEdit ? "" : html`disabled`}>
            ${zones.map((z) => html`<lds-option value="${z}">${z.replaceAll("_", " ")}</lds-option>`)}
          </lds-combobox>
          <span slot="help">${t("settings.timeZoneHelp")}</span>
        </lds-field>
      </form>
    `, canEdit ? html`<span class="muted" role="status" data-saved></span><lds-button data-action="save">${t("account.save")}</lds-button>` : ""));

    const form = this.shadowRoot.querySelector("form");
    if (!canEdit) return;
    submitOnEnter(form);
    form.addEventListener("submit", (e) => {
      e.preventDefault();
      this.save(form);
    });
    this.shadowRoot.querySelector('[data-action="save"]').addEventListener("click", () => form.requestSubmit());
  }

  async save(form) {
    const { api, i18n, router, workspaces } = this.context;
    const alert = form.querySelector("lds-alert");
    const saved = this.shadowRoot.querySelector("[data-saved]");
    alert.textContent = saved.textContent = "";
    clearFieldErrors(form);
    const body = formValues(form);
    let updated;
    try {
      ({ data: updated } = await busy(this.shadowRoot.querySelector('[data-action="save"]'), () =>
        api.patch(`/w/${encodeURIComponent(this.ws.slug)}`, { body, ifMatch: this.etag })));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else alert.textContent = errorMessage(err, i18n);
      return;
    }
    await workspaces.refresh();
    if (updated.slug !== this.ws.slug) {
      router.navigate(`/w/${updated.slug}/settings/general`, { replace: true });
      return;
    }
    await this.load();
    this.shadowRoot.querySelector("[data-saved]").textContent = i18n.t("account.saved");
  }
}

customElements.define("hzn-settings-general", HznSettingsGeneral);
