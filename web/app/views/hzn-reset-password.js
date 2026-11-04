// <hzn-reset-password>: chooses a new password with the token from an
// emailed or admin-issued link (/reset-password#token=…). The token is read
// once and dropped from the address bar, so it stays out of history.

import { requestContext } from "horizon/core/context.js";
import { html } from "horizon/core/dom.js";
import { busy, errorMessage, formValues, showFieldErrors } from "horizon/core/forms.js";
import { authPage, authStyles } from "horizon/views/auth-page.js";

const MIN_LENGTH = 12; // identity.MinPasswordLength

class HznResetPassword extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const { t } = this.context.i18n;
    this.token = new URLSearchParams(location.hash.slice(1)).get("token") || "";
    if (location.hash) history.replaceState(history.state, "", location.pathname + location.search);

    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [authStyles];
    if (!this.token) {
      root.innerHTML = String(authPage(t, t("reset.title"), html`
        <lds-alert intent="danger" role="alert">${t("error.token.invalid_or_expired")}</lds-alert>
        <p class="links"><a href="/forgot-password">${t("reset.again")}</a></p>
      `));
      return;
    }
    root.innerHTML = String(authPage(t, t("reset.title"), html`
      <p class="lead">${t("reset.lead", { min: MIN_LENGTH })}</p>
      <form novalidate>
        <lds-alert intent="danger" role="alert"></lds-alert>
        <lds-field label="${t("field.newPassword")}" required>
          <lds-input name="password" type="password" autocomplete="new-password" required></lds-input>
        </lds-field>
        <lds-field label="${t("field.confirmPassword")}" required>
          <lds-input name="confirm" type="password" autocomplete="new-password" required></lds-input>
        </lds-field>
        <lds-button type="submit" variant="primary" full-width>${t("reset.submit")}</lds-button>
      </form>
    `));

    const form = root.querySelector("form");
    form.addEventListener("submit", (event) => {
      event.preventDefault();
      this.submit(form);
    });
    requestAnimationFrame(() => root.querySelector('[name="password"]').focus());
  }

  async submit(form) {
    const { api, i18n } = this.context;
    const { t, has } = i18n;
    const alert = form.querySelector("lds-alert");
    alert.textContent = "";
    const { password = "", confirm = "" } = formValues(form);
    if (password !== confirm) {
      showFieldErrors(form, [{ field: "confirm", code: "mismatch" }], t, has);
      return;
    }
    try {
      await busy(form, () => api.post("/auth/password/reset", { body: { token: this.token, password } }));
    } catch (err) {
      const rest = err.fields?.length ? showFieldErrors(form, err.fields, t, has) : [];
      if (!err.fields?.length) alert.textContent = errorMessage(err, i18n);
      else if (rest.length) alert.textContent = t(`error.${rest[0].field}.${rest[0].code}`);
      return;
    }
    form.replaceWith(document.createRange().createContextualFragment(String(html`
      <lds-alert intent="success" role="status">${t("reset.done")}</lds-alert>
      <p class="links"><a href="/sign-in">${t("reset.signIn")}</a></p>
    `)));
  }
}

customElements.define("hzn-reset-password", HznResetPassword);
