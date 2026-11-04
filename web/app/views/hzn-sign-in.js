// <hzn-sign-in>: sign-in with email and password, the instance's single
// sign-on providers, an emailed sign-in link, or a workspace's SSO found by
// email (docs/security.md#authentication-methods). On success the session
// changes and the frame's route guard sends the user to ?next= (same-origin
// paths only) or home. SSO errors come back as ?sso_error=.

import { requestContext } from "horizon/core/context.js";
import { html } from "horizon/core/dom.js";
import { busy, errorMessage, formValues } from "horizon/core/forms.js";
import { authPage, authStyles } from "horizon/views/auth-page.js";

const MODES = ["password", "magic", "sso"];

class HznSignIn extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [authStyles];
    root.addEventListener("click", (e) => {
      const mode = e.target.closest?.("[data-mode]")?.dataset.mode;
      const provider = e.target.closest?.("[data-provider]")?.dataset.provider;
      if (mode) {
        e.preventDefault();
        this.show(mode);
      } else if (provider) {
        location.assign(this.startURL(provider));
      }
    });
    this.providers = [];
    const query = this.context.router.current?.query;
    this.ssoError = query?.get("sso_error") ?? "";
    this.show("password");
    this.context.api.get("/auth/oidc/providers").then(({ data }) => {
      this.providers = data.items;
      this.renderProviders();
    }, () => {});
  }

  /** The instance's providers, above the form (filled in when known). */
  renderProviders() {
    const { t } = this.context.i18n;
    const box = this.shadowRoot.querySelector("[data-providers]");
    if (!box) return;
    box.innerHTML = this.providers.length ? String(html`
      <div class="providers">${this.providers.map((p) => html`
        <lds-button variant="secondary" full-width data-provider="${p.slug}">${t("signIn.continueWith", { name: p.name })}</lds-button>`)}
      </div>
      <p class="or">${t("signIn.or")}</p>`) : "";
  }

  /** Where to land after signing in: ?next= when it is a same-origin path. */
  get next() {
    const next = this.context.router.current?.query.get("next") ?? "/";
    return next.startsWith("/") && !next.startsWith("//") ? next : "/";
  }

  startURL(slug) {
    return `/api/v1/auth/oidc/${encodeURIComponent(slug)}/start?next=${encodeURIComponent(this.next)}`;
  }

  show(mode) {
    const { t, has } = this.context.i18n;
    this.mode = MODES.includes(mode) ? mode : "password";
    const error = this.ssoError ? (has(`error.${this.ssoError}`) ? t(`error.${this.ssoError}`) : t("error.sso_failed")) : "";
    const others = MODES.filter((m) => m !== this.mode).map((m) => html`<a href="#" data-mode="${m}">${t(`signIn.mode.${m}`)}</a>`);
    let body;
    switch (this.mode) {
      case "magic":
        body = html`
          <p class="lead">${t("signIn.magicLead")}</p>
          <form novalidate data-form="magic">
            <lds-alert intent="danger" role="alert"></lds-alert>
            <lds-field label="${t("field.email")}" required><lds-input name="email" type="email" autocomplete="username" required></lds-input></lds-field>
            <lds-button type="submit" variant="primary" full-width>${t("signIn.sendLink")}</lds-button>
          </form>`;
        break;
      case "sso":
        body = html`
          <p class="lead">${t("signIn.ssoLead")}</p>
          <form novalidate data-form="sso">
            <lds-alert intent="danger" role="alert"></lds-alert>
            <lds-field label="${t("field.email")}" required><lds-input name="email" type="email" autocomplete="username" required></lds-input></lds-field>
            <lds-button type="submit" variant="primary" full-width>${t("signIn.ssoContinue")}</lds-button>
          </form>`;
        break;
      default:
        body = html`
          <form novalidate data-form="password">
            <lds-alert intent="danger" role="alert"></lds-alert>
            <lds-field label="${t("field.email")}" required>
              <lds-input name="email" type="email" autocomplete="username" required></lds-input>
            </lds-field>
            <lds-field label="${t("field.password")}" required>
              <lds-input name="password" type="password" autocomplete="current-password" required></lds-input>
            </lds-field>
            <lds-button type="submit" variant="primary" full-width>${t("signIn.submit")}</lds-button>
          </form>
          <p class="links"><a href="/forgot-password">${t("signIn.forgot")}</a></p>`;
    }
    const root = this.shadowRoot;
    root.innerHTML = String(authPage(t, t("signIn.title"), html`
      ${error ? html`<lds-alert intent="danger" role="alert" data-sso-error>${error}</lds-alert>` : ""}
      <div data-providers></div>${body}
      <p class="links modes">${others.flatMap((a, i) => (i ? [html` · `, a] : [a]))}</p>`));
    this.renderProviders();
    const form = root.querySelector("form");
    const email = form.querySelector('[name="email"]');
    form.addEventListener("submit", (event) => {
      event.preventDefault();
      this.submit(form);
    });
    requestAnimationFrame(() => email.focus());
  }

  async submit(form) {
    const { api, i18n, session } = this.context;
    const alert = form.querySelector("lds-alert");
    const { email = "", password = "" } = formValues(form);
    if (!email.trim() || (this.mode === "password" && !password)) {
      alert.textContent = i18n.t("signIn.missing");
      return;
    }
    alert.textContent = "";
    try {
      if (this.mode === "magic") {
        await busy(form, () => api.post("/auth/magic-link", { body: { email: email.trim() } }));
        form.outerHTML = String(html`<lds-alert intent="success" role="status" data-sent>${i18n.t("signIn.linkSent", { email: email.trim() })}</lds-alert>`);
      } else if (this.mode === "sso") {
        const { data } = await busy(form, () => api.post("/auth/oidc/discover", { body: { email: email.trim() } }));
        location.assign(this.startURL(data.slug));
      } else {
        await busy(form, () => session.login(email.trim(), password));
      }
    } catch (err) {
      alert.textContent = errorMessage(err, i18n);
      if (err.code === "sso_required") {
        this.ssoError = "sso_required";
        this.show("sso");
        this.shadowRoot.querySelector('[name="email"]').setAttribute("value", email.trim());
        return;
      }
      form.querySelector('[name="password"]')?.focus();
    }
  }
}

customElements.define("hzn-sign-in", HznSignIn);
