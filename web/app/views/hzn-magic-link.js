// <hzn-magic-link>: signs in with the token of an emailed link
// (/magic-link#token=…), in the browser that asked for it. The token is
// read once and dropped from the address bar.

import { requestContext } from "horizon/core/context.js";
import { html } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";
import { authPage, authStyles } from "horizon/views/auth-page.js";

class HznMagicLink extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const token = new URLSearchParams(location.hash.slice(1)).get("token") || "";
    if (location.hash) history.replaceState(history.state, "", location.pathname + location.search);
    this.attachShadow({ mode: "open" }).adoptedStyleSheets = [authStyles];
    this.verify(token);
  }

  async verify(token) {
    const { api, i18n, session, router } = this.context;
    const { t } = i18n;
    this.shadowRoot.innerHTML = String(authPage(t, t("magic.title"), html`<p class="lead" role="status">${t("magic.checking")}</p>`));
    try {
      if (!token) throw Object.assign(new Error("no token"), { code: "invalid_input", fields: [{ field: "token", code: "invalid_or_expired" }] });
      const { data } = await api.post("/auth/magic-link/verify", { body: { token } });
      session.adopt(data);
      router.navigate("/", { replace: true });
    } catch (err) {
      const msg = err.fields?.some((f) => f.field === "token") ? t("magic.invalid") : errorMessage(err, i18n);
      this.shadowRoot.innerHTML = String(authPage(t, t("magic.title"), html`
        <lds-alert intent="danger" role="alert">${msg}</lds-alert>
        <p class="links"><a href="/sign-in">${t("magic.again")}</a></p>`));
    }
  }
}

customElements.define("hzn-magic-link", HznMagicLink);
