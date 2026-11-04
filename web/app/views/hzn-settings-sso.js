// <hzn-settings-sso>: /w/:ws/settings/authentication — the workspace's
// OpenID Connect providers and whether it requires single sign-on
// (workspace.manage; docs/security.md#authentication-methods).

import { requestContext } from "horizon/core/context.js";
import { html, ldsLightSheet, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, showFieldErrors } from "horizon/core/forms.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

const styles = sheet(`
  section + section { margin-block-start: var(--lds-space-8); }
  h3 { font-size: var(--lds-font-size-md); margin: 0 0 var(--lds-space-2); }
  .lead { margin: 0 0 var(--lds-space-4); color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); max-inline-size: 44rem; }
  .mono { font-family: var(--lds-font-family-mono); font-size: var(--lds-font-size-xs); overflow-wrap: anywhere; }
  lds-modal form { display: grid; gap: var(--lds-space-4); }
  .redirect { display: grid; gap: var(--lds-space-1); font-size: var(--lds-font-size-sm); }
  .redirect code { font-family: var(--lds-font-family-mono); padding: var(--lds-space-2) var(--lds-space-3); background: var(--lds-color-bg-subtle);
    border-radius: var(--lds-radius-sm); overflow-wrap: anywhere; user-select: all; }
  td.actions { text-align: end; white-space: nowrap; }
`);

class HznSettingsSso extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [ldsLightSheet(), settingsStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("lds-change", (e) => {
      if (e.target.matches?.("[data-enforce]")) this.setEnforced(e.detail.checked);
    });
    root.addEventListener("lds-input", (e) => {
      if (e.target.matches?.('[name="slug"]')) this.shadowRoot.querySelector("[data-redirect]").textContent = this.redirect(e.detail.value);
    });
    this.load();
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.load();
  }

  get base() {
    return `/w/${encodeURIComponent(this.getAttribute("ws"))}`;
  }

  redirect(slug) {
    return `${location.origin}/api/v1/auth/oidc/${slug || "…"}/callback`;
  }

  async load() {
    const { api, i18n } = this.context;
    try {
      const [ws, providers, sso, roles] = await Promise.all([
        api.get(this.base), api.get(`${this.base}/sso/providers`), api.get(`${this.base}/sso`), api.get(`${this.base}/roles`),
      ]);
      this.ws = ws.data;
      this.providers = providers.data.items;
      this.enforced = sso.data.enforced;
      this.roles = roles.data.items;
    } catch (err) {
      const msg = html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`;
      this.shadowRoot.innerHTML = String(this.ws ? settingsPage(i18n.t, this.ws, "authentication", msg) : msg);
      return;
    }
    this.render();
  }

  render() {
    const { t } = this.context.i18n;
    const role = (id) => this.roles.find((r) => r.id === id)?.name;
    const body = html`
      <section>
        <h3>${t("sso.require")}</h3>
        <p class="lead">${t("sso.requireHint")}</p>
        <lds-switch data-enforce ${this.enforced ? html`checked` : ""}>${t("sso.requireSwitch")}</lds-switch>
      </section>
      <section>
        <h3>${t("sso.providers")}</h3>
        <p class="lead">${t("sso.providersHint")}</p>
        ${this.providers.length ? html`
          <lds-table density="compact">
            <table>
              <thead><tr><th scope="col">${t("sso.name")}</th><th scope="col">${t("sso.issuer")}</th><th scope="col">${t("sso.domains")}</th>
                <th scope="col">${t("sso.provision")}</th><th scope="col">${t("sso.status")}</th><th scope="col"></th></tr></thead>
              <tbody>${this.providers.map((p) => html`
                <tr data-id="${p.id}">
                  <td>${p.name}<div class="mono">${p.slug}</div></td><td><span class="mono">${p.issuer}</span></td>
                  <td>${p.email_domains.length ? p.email_domains.join(", ") : t("sso.anyDomain")}</td>
                  <td>${p.provision_role_id ? role(p.provision_role_id) ?? "—" : t("sso.noProvision")}</td>
                  <td>${p.enabled ? html`<lds-badge size="sm" intent="success">${t("sso.enabled")}</lds-badge>` : html`<lds-badge size="sm">${t("sso.disabled")}</lds-badge>`}</td>
                  <td class="actions"><lds-button size="sm" variant="ghost" data-action="edit" label="${t("sso.editNamed", { name: p.name })}">${t("issues.edit")}</lds-button></td>
                </tr>`)}</tbody>
            </table>
          </lds-table>`
          : html`<lds-empty-state><lds-icon slot="icon" name="lock"></lds-icon><h2 slot="title">${t("sso.none")}</h2>${t("sso.noneHint")}</lds-empty-state>`}
      </section>
      ${this.dialog()}`;
    const actions = html`<lds-button data-action="new"><lds-icon slot="start" name="plus"></lds-icon>${t("sso.new")}</lds-button>`;
    this.shadowRoot.innerHTML = String(settingsPage(t, this.ws, "authentication", body, actions));
  }

  dialog() {
    const { t } = this.context.i18n;
    const p = this.editing;
    return html`
      <lds-modal data-dialog="edit" size="md">
        <span slot="header">${p ? t("sso.edit") : t("sso.new")}</span>
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <lds-field label="${t("sso.name")}" required><lds-input name="name" required maxlength="100" value="${p?.name ?? ""}"></lds-input></lds-field>
          <lds-field label="${t("sso.slug")}" required hint="${t("sso.slugHint")}">
            <lds-input name="slug" required maxlength="40" value="${p?.slug ?? ""}" ${p ? html`disabled` : ""}></lds-input></lds-field>
          <div class="redirect"><strong>${t("sso.redirect")}</strong><code data-redirect>${this.redirect(p?.slug)}</code></div>
          <lds-field label="${t("sso.issuer")}" required hint="${t("sso.issuerHint")}"><lds-input name="issuer" type="url" required value="${p?.issuer ?? ""}"></lds-input></lds-field>
          <lds-field label="${t("sso.clientID")}" required><lds-input name="client_id" required value="${p?.client_id ?? ""}"></lds-input></lds-field>
          <lds-field label="${t("sso.clientSecret")}" ${p ? "" : html`required`} hint="${p ? t("sso.secretKeep") : ""}">
            <lds-input name="client_secret" type="password" ${p ? html`placeholder="••••••••"` : html`required`}></lds-input></lds-field>
          <lds-field label="${t("sso.domains")}" hint="${t("sso.domainsHint")}"><lds-input name="email_domains" value="${(p?.email_domains ?? []).join(", ")}" placeholder="acme.com"></lds-input></lds-field>
          <lds-field label="${t("sso.provision")}" hint="${t("sso.provisionHint")}">
            <lds-select name="provision_role_id" value="${p?.provision_role_id ?? ""}">
              <option value="">${t("sso.noProvision")}</option>
              ${this.roles.map((r) => html`<option value="${r.id}">${r.name}</option>`)}
            </lds-select>
          </lds-field>
          <lds-switch name="enabled" ${p && !p.enabled ? "" : html`checked`}>${t("sso.enabled")}</lds-switch>
        </form>
        <div slot="footer" class="actions">
          ${p ? html`<lds-button variant="ghost" data-action="delete">${t("sso.delete")}</lds-button>` : ""}
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button data-action="save">${p ? t("sso.save") : t("sso.create")}</lds-button>
        </div>
      </lds-modal>`;
  }

  async setEnforced(on) {
    const { api, i18n } = this.context;
    try {
      const { data } = await api.put(`${this.base}/sso`, { body: { enforced: on } });
      this.enforced = data.enforced;
      globalThis.LDS?.toast?.({ intent: "success", title: i18n.t(on ? "sso.requiredOn" : "sso.requiredOff") });
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      this.render(); // back to the saved state
    }
  }

  async onClick(e) {
    const target = e.target.closest?.("[data-action], [data-close]");
    if (!target) return;
    if (target.hasAttribute("data-close")) {
      target.closest("lds-modal").removeAttribute("open");
      return;
    }
    const { api, i18n } = this.context;
    switch (target.dataset.action) {
      case "new":
      case "edit":
        this.editing = target.dataset.action === "edit" ? this.providers.find((p) => p.id === target.closest("tr").dataset.id) : null;
        this.render();
        this.shadowRoot.querySelector('[data-dialog="edit"]').setAttribute("open", "");
        return;
      case "save":
        return this.save(target);
      case "delete":
        if (!confirm(i18n.t("sso.deleteConfirm", { name: this.editing.name }))) return;
        try {
          await api.delete(`${this.base}/sso/providers/${this.editing.id}`);
        } catch (err) {
          globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
          return;
        }
        this.editing = null;
        return this.load();
    }
  }

  async save(button) {
    const { api, i18n } = this.context;
    const modal = this.shadowRoot.querySelector('[data-dialog="edit"]');
    const form = modal.querySelector("form");
    clearFieldErrors(form);
    form.querySelector("lds-alert").textContent = "";
    const fd = new FormData(form);
    const value = (name) => String(fd.get(name) ?? "").trim();
    const body = {
      name: value("name"), issuer: value("issuer"), client_id: value("client_id"),
      email_domains: value("email_domains").split(/[\s,]+/).filter(Boolean),
      enabled: form.querySelector('[name="enabled"]').hasAttribute("checked"),
    };
    if (value("client_secret")) body.client_secret = value("client_secret");
    const role = value("provision_role_id");
    if (role) body.provision_role_id = role;
    else if (this.editing) body.no_provision = true;
    try {
      await busy(button, () => (this.editing
        ? api.patch(`${this.base}/sso/providers/${this.editing.id}`, { body, ifMatch: `"${this.editing.version}"` })
        : api.post(`${this.base}/sso/providers`, { body: { ...body, slug: value("slug") } })));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else form.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      return;
    }
    modal.removeAttribute("open");
    this.editing = null;
    await this.load();
  }
}

customElements.define("hzn-settings-sso", HznSettingsSso);
