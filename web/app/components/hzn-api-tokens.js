// <hzn-api-tokens>: the signed-in user's personal access tokens: list,
// create (the secret is shown once) and revoke after confirmation.

import { requestContext } from "horizon/core/context.js";
import { html, ldsLightSheet, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, formValues, showFieldErrors } from "horizon/core/forms.js";

const styles = sheet(`
  :host { display: grid; gap: var(--lds-space-5); }
  form { display: grid; gap: var(--lds-space-4); max-width: 28rem; }
  .row { display: grid; grid-template-columns: 1fr 1fr; gap: var(--lds-space-4); }
  .actions { display: flex; gap: var(--lds-space-3); align-items: center; }
  .hint { font-family: var(--lds-font-family-mono); font-size: var(--lds-font-size-xs); color: var(--lds-color-fg-muted); }
  .muted { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); margin: 0; }
  td.end { text-align: end; white-space: nowrap; }
  lds-alert:empty, [data-created]:empty { display: none; }
`);

const SCOPES = ["read", "write", "admin"];
const SCOPE_INTENT = { read: "neutral", write: "info", admin: "warning" };
const EXPIRY_DAYS = ["30", "90", "365", "never"];

class HznApiTokens extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const { t } = this.context.i18n;
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [ldsLightSheet(), styles];
    root.innerHTML = String(html`
      <div data-created></div>
      <div data-list aria-busy="true"></div>
      <form novalidate>
        <lds-alert intent="danger" role="alert"></lds-alert>
        <lds-field label="${t("tokens.name")}" required>
          <lds-input name="name" required maxlength="100" placeholder="${t("tokens.namePlaceholder")}"></lds-input>
        </lds-field>
        <div class="row">
          <lds-field label="${t("tokens.scope")}">
            <lds-select name="scope" value="read">
              ${SCOPES.map((s) => html`<option value="${s}">${t(`tokens.scope.${s}`)}</option>`)}
            </lds-select>
          </lds-field>
          <lds-field label="${t("tokens.expires")}">
            <lds-select name="expires" value="90">
              ${EXPIRY_DAYS.map((d) => html`<option value="${d}">${t(`tokens.expiry.${d}`)}</option>`)}
            </lds-select>
          </lds-field>
        </div>
        <p class="muted" data-scope-help>${t("tokens.scopeHelp.read")}</p>
        <div class="actions">
          <lds-button type="submit" variant="secondary">${t("tokens.create")}</lds-button>
        </div>
      </form>
      <lds-modal size="sm" data-confirm>
        <span slot="header">${t("tokens.revokeTitle")}</span>
        <p class="muted" data-confirm-body></p>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-confirm-cancel>${t("tokens.cancel")}</lds-button>
          <lds-button variant="danger" data-confirm-ok>${t("tokens.revoke")}</lds-button>
        </div>
      </lds-modal>
    `);

    const form = root.querySelector("form");
    form.addEventListener("submit", (event) => {
      event.preventDefault();
      this.create(form);
    });
    form.querySelector('[name="scope"]').addEventListener("lds-change", (event) => {
      form.querySelector("[data-scope-help]").textContent = t(`tokens.scopeHelp.${event.detail.value}`);
    });
    root.querySelector("[data-list]").addEventListener("click", (event) => {
      const button = event.target.closest?.("[data-revoke]");
      if (button) this.confirmRevoke(button.dataset.revoke, button.dataset.name);
    });
    const modal = root.querySelector("[data-confirm]");
    root.querySelector("[data-confirm-cancel]").addEventListener("click", () => modal.removeAttribute("open"));
    root.querySelector("[data-confirm-ok]").addEventListener("click", () => this.revoke());
    this.load();
  }

  async load() {
    const { api, i18n } = this.context;
    const { t } = i18n;
    const box = this.shadowRoot.querySelector("[data-list]");
    let items;
    try {
      ({ data: { items } } = await api.get("/me/tokens"));
    } catch (err) {
      box.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    } finally {
      box.removeAttribute("aria-busy");
    }
    if (!items.length) {
      box.innerHTML = String(html`<p class="muted">${t("tokens.none")}</p>`);
      return;
    }
    const now = Date.now();
    const when = (v) => html`<span title="${i18n.date(v, { dateStyle: "medium", timeStyle: "short" })}">${i18n.relative(v)}</span>`;
    box.innerHTML = String(html`
      <lds-table density="compact">
        <table>
          <thead><tr>
            <th scope="col">${t("tokens.name")}</th>
            <th scope="col">${t("tokens.scope")}</th>
            <th scope="col">${t("tokens.lastUsed")}</th>
            <th scope="col">${t("tokens.expires")}</th>
            <th scope="col"><span class="lds-visually-hidden">${t("account.actions")}</span></th>
          </tr></thead>
          <tbody>
            ${items.map((tok) => {
              const expired = tok.expires_at && new Date(tok.expires_at).getTime() <= now;
              return html`
                <tr>
                  <td>${tok.name} <br><span class="hint">${tok.hint}…</span></td>
                  <td><lds-badge size="sm" intent="${SCOPE_INTENT[tok.scope]}">${t(`tokens.scope.${tok.scope}`)}</lds-badge></td>
                  <td>${tok.last_used_at ? when(tok.last_used_at) : t("tokens.never")}</td>
                  <td>${expired
                    ? html`<lds-badge size="sm" intent="danger">${t("tokens.expired")}</lds-badge>`
                    : tok.expires_at ? when(tok.expires_at) : t("tokens.noExpiry")}</td>
                  <td class="end"><lds-button size="sm" variant="ghost" data-revoke="${tok.id}" data-name="${tok.name}">${t("tokens.revoke")}</lds-button></td>
                </tr>`;
            })}
          </tbody>
        </table>
      </lds-table>
    `);
  }

  async create(form) {
    const { api, i18n } = this.context;
    const { t } = i18n;
    const alert = form.querySelector("lds-alert");
    alert.textContent = "";
    clearFieldErrors(form);
    const { name = "", scope = "read", expires = "90" } = formValues(form);
    const body = { name, scope };
    if (expires !== "never") body.expires_at = new Date(Date.now() + Number(expires) * 86400e3).toISOString();
    let created;
    try {
      ({ data: created } = await busy(form, () => api.post("/me/tokens", { body })));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, t, i18n.has);
      else alert.textContent = errorMessage(err, i18n);
      return;
    }
    form.querySelector('[name="name"]').setAttribute("value", "");
    form.reset();
    const box = this.shadowRoot.querySelector("[data-created]");
    box.innerHTML = String(html`
      <lds-alert intent="success" role="status">
        <span slot="title">${t("tokens.created", { name: created.name })}</span>
        ${t("tokens.copyNow")}
      </lds-alert>
    `);
    const code = document.createElement("lds-code");
    code.textContent = created.token;
    box.append(code);
    this.load();
  }

  confirmRevoke(id, name) {
    this.pending = id;
    const modal = this.shadowRoot.querySelector("[data-confirm]");
    modal.querySelector("[data-confirm-body]").textContent = this.context.i18n.t("tokens.revokeBody", { name });
    modal.setAttribute("open", "");
  }

  async revoke() {
    const { api, i18n } = this.context;
    const modal = this.shadowRoot.querySelector("[data-confirm]");
    modal.removeAttribute("open");
    try {
      await api.delete(`/me/tokens/${encodeURIComponent(this.pending)}`);
    } catch (err) {
      if (err.status !== 404) globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
    this.shadowRoot.querySelector("[data-created]").replaceChildren();
    this.load();
  }
}

customElements.define("hzn-api-tokens", HznApiTokens);
