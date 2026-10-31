// <hzn-account>: the signed-in user's profile, password and sessions.

import { requestContext } from "horizon/core/context.js";
import { html, ldsLightSheet, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, formValues, setAutocomplete, showFieldErrors, submitOnEnter } from "horizon/core/forms.js";

const styles = sheet(`
  :host { display: block; padding: var(--lds-space-8) var(--lds-space-6); }
  .page { max-width: 44rem; margin: 0 auto; display: grid; gap: var(--lds-space-8); }
  h1 { margin: 0; font-family: var(--lds-font-family-display); font-size: var(--lds-font-size-2xl); }
  h2 { margin: 0 0 var(--lds-space-1); font-size: var(--lds-font-size-lg); }
  .lead { margin: 0 0 var(--lds-space-5); color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  form { display: grid; gap: var(--lds-space-4); max-width: 28rem; }
  .actions { display: flex; gap: var(--lds-space-3); align-items: center; }
  .muted { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  td.end { text-align: end; white-space: nowrap; }
  [data-sessions] { margin-bottom: var(--lds-space-4); }
  lds-alert:empty { display: none; }
`);

class HznAccount extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const { i18n, session } = this.context;
    const { t } = i18n;
    const user = session.user;
    const zones = Intl.supportedValuesOf?.("timeZone") ?? [];
    const browserZone = Intl.DateTimeFormat().resolvedOptions().timeZone;

    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [ldsLightSheet(), styles];
    root.innerHTML = String(html`
      <div class="page">
        <h1>${t("account.title")}</h1>

        <section aria-labelledby="profile-title">
          <h2 id="profile-title">${t("account.profile")}</h2>
          <p class="lead">${user.email}</p>
          <form id="profile" novalidate>
            <lds-alert intent="danger" role="alert"></lds-alert>
            <lds-field label="${t("field.name")}" required>
              <lds-input name="name" required maxlength="200" value="${user.name}"></lds-input>
            </lds-field>
            <lds-field label="${t("field.timezone")}">
              <lds-combobox name="timezone" value="${user.timezone ?? ""}" placeholder="${t("account.timezoneBrowser", { zone: browserZone })}">
                <lds-option value="">${t("account.timezoneBrowser", { zone: browserZone })}</lds-option>
                ${zones.map((z) => html`<lds-option value="${z}">${z.replaceAll("_", " ")}</lds-option>`)}
              </lds-combobox>
            </lds-field>
            <div class="actions">
              <lds-button type="submit" variant="primary">${t("account.save")}</lds-button>
              <span class="muted" role="status" data-saved></span>
            </div>
          </form>
        </section>

        <section aria-labelledby="password-title">
          <h2 id="password-title">${t("account.password")}</h2>
          <p class="lead">${t("account.passwordLead")}</p>
          <form id="password" novalidate>
            <lds-alert intent="danger" role="alert"></lds-alert>
            <lds-field label="${t("field.currentPassword")}" required>
              <lds-input name="current_password" type="password" required></lds-input>
            </lds-field>
            <lds-field label="${t("field.newPassword")}" required>
              <lds-input name="new_password" type="password" required></lds-input>
            </lds-field>
            <div class="actions">
              <lds-button type="submit" variant="secondary">${t("account.changePassword")}</lds-button>
              <span class="muted" role="status" data-saved></span>
            </div>
          </form>
        </section>

        <section aria-labelledby="sessions-title" aria-busy="true">
          <h2 id="sessions-title">${t("account.sessions")}</h2>
          <p class="lead">${t("account.sessionsLead")}</p>
          <div data-sessions></div>
          <div class="actions">
            <lds-button data-action="revoke-others" variant="secondary">${t("account.revokeOthers")}</lds-button>
            <lds-button data-action="sign-out" variant="ghost">${t("account.signOut")}</lds-button>
          </div>
        </section>
      </div>
    `);

    const profile = root.getElementById("profile");
    const password = root.getElementById("password");
    setAutocomplete(password.querySelector('[name="current_password"]'), "current-password");
    setAutocomplete(password.querySelector('[name="new_password"]'), "new-password");
    for (const [form, fn] of [[profile, () => this.saveProfile(profile)], [password, () => this.changePassword(password)]]) {
      submitOnEnter(form);
      form.addEventListener("submit", (event) => {
        event.preventDefault();
        fn();
      });
    }
    root.querySelector('[data-action="revoke-others"]').addEventListener("click", () => this.revokeOthers());
    root.querySelector('[data-action="sign-out"]').addEventListener("click", () => session.logout());
    root.querySelector("[data-sessions]").addEventListener("click", (event) => {
      const id = event.target.closest?.("[data-revoke]")?.dataset.revoke;
      if (id) this.revoke(id);
    });
    this.loadSessions();
  }

  async saveProfile(form) {
    const { api, i18n, session } = this.context;
    const alert = form.querySelector("lds-alert");
    const saved = form.querySelector("[data-saved]");
    alert.textContent = saved.textContent = "";
    clearFieldErrors(form);
    const { name = "", timezone = "" } = formValues(form);
    try {
      const { data } = await busy(form, () =>
        api.patch("/me", { body: { name, timezone }, ifMatch: `"${session.user.version}"` }),
      );
      session.setUser(data);
      saved.textContent = i18n.t("account.saved");
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else alert.textContent = errorMessage(err, i18n);
      if (err.code === "version_conflict") await session.load();
    }
  }

  async changePassword(form) {
    const { api, i18n } = this.context;
    const alert = form.querySelector("lds-alert");
    const saved = form.querySelector("[data-saved]");
    alert.textContent = saved.textContent = "";
    clearFieldErrors(form);
    const body = formValues(form);
    try {
      await busy(form, () => api.post("/me/password", { body }));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else alert.textContent = errorMessage(err, i18n);
      return;
    }
    form.reset();
    saved.textContent = i18n.t("account.passwordChanged");
    this.loadSessions();
  }

  async loadSessions() {
    const { api, i18n } = this.context;
    const { t } = i18n;
    const section = this.shadowRoot.querySelector('[aria-labelledby="sessions-title"]');
    const box = section.querySelector("[data-sessions]");
    let items;
    try {
      ({ data: { items } } = await api.get("/me/sessions"));
    } catch (err) {
      box.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    } finally {
      section.removeAttribute("aria-busy");
    }
    box.innerHTML = String(html`
      <lds-table density="compact">
        <table>
          <thead><tr>
            <th scope="col">${t("account.device")}</th>
            <th scope="col">${t("account.ip")}</th>
            <th scope="col">${t("account.lastSeen")}</th>
            <th scope="col"><span class="lds-visually-hidden">${t("account.actions")}</span></th>
          </tr></thead>
          <tbody>
            ${items.map((s) => html`
              <tr>
                <td>${describeAgent(s.user_agent) || t("account.unknownDevice")}
                  ${s.current ? html` <lds-badge size="sm" intent="info">${t("account.thisDevice")}</lds-badge>` : ""}</td>
                <td>${s.ip || "—"}</td>
                <td title="${i18n.date(s.last_seen_at, { dateStyle: "medium", timeStyle: "short" })}">${i18n.relative(s.last_seen_at)}</td>
                <td class="end">${s.current ? "" : html`<lds-button size="sm" variant="ghost" data-revoke="${s.id}">${t("account.revoke")}</lds-button>`}</td>
              </tr>`)}
          </tbody>
        </table>
      </lds-table>
    `);
  }

  async revoke(id) {
    const { api, i18n } = this.context;
    try {
      await api.delete(`/me/sessions/${encodeURIComponent(id)}`);
    } catch (err) {
      if (err.status !== 404) globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
    this.loadSessions();
  }

  async revokeOthers() {
    const { api, i18n } = this.context;
    try {
      const { data } = await api.post("/me/sessions/revoke-others");
      globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("account.revoked", { count: data.revoked }) });
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
    this.loadSessions();
  }
}

/** "Firefox on Linux" from a user agent string; "" when unknown. */
export function describeAgent(ua = "") {
  const browser = [["Edg/", "Edge"], ["OPR/", "Opera"], ["Firefox/", "Firefox"], ["Chrome/", "Chrome"], ["Safari/", "Safari"]]
    .find(([token]) => ua.includes(token))?.[1];
  const os = [["Windows", "Windows"], ["Android", "Android"], ["iPhone", "iOS"], ["iPad", "iPadOS"], ["Mac OS X", "macOS"], ["CrOS", "ChromeOS"], ["Linux", "Linux"]]
    .find(([token]) => ua.includes(token))?.[1];
  if (browser && os) return `${browser} · ${os}`;
  return browser || os || "";
}

customElements.define("hzn-account", HznAccount);
