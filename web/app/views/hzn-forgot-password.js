// <hzn-forgot-password>: asks for a reset link. The answer is the same
// whether or not the email has an account.

import { requestContext } from "horizon/core/context.js";
import { html } from "horizon/core/dom.js";
import { busy, errorMessage, formValues, setAutocomplete, showFieldErrors, submitOnEnter } from "horizon/core/forms.js";
import { authPage, authStyles } from "horizon/views/auth-page.js";

class HznForgotPassword extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const { t } = this.context.i18n;
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [authStyles];
    root.innerHTML = String(authPage(t, t("forgot.title"), html`
      <p class="lead">${t("forgot.lead")}</p>
      <form novalidate>
        <lds-alert intent="danger" role="alert"></lds-alert>
        <lds-field label="${t("field.email")}" required>
          <lds-input name="email" type="email" required></lds-input>
        </lds-field>
        <lds-button type="submit" variant="primary" full-width>${t("forgot.submit")}</lds-button>
      </form>
      <p class="links"><a href="/sign-in">${t("forgot.back")}</a></p>
    `));

    const form = root.querySelector("form");
    setAutocomplete(root.querySelector('[name="email"]'), "username");
    submitOnEnter(form);
    form.addEventListener("submit", (event) => {
      event.preventDefault();
      this.submit(form);
    });
    requestAnimationFrame(() => root.querySelector('[name="email"]').focus());
  }

  async submit(form) {
    const { api, i18n } = this.context;
    const alert = form.querySelector("lds-alert");
    alert.textContent = "";
    const { email = "" } = formValues(form);
    try {
      await busy(form, () => api.post("/auth/password/forgot", { body: { email: email.trim() } }));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else alert.textContent = errorMessage(err, i18n);
      return;
    }
    form.replaceWith(document.createRange().createContextualFragment(String(html`
      <lds-alert intent="success" role="status">${i18n.t("forgot.sent", { email: email.trim() })}</lds-alert>
    `)));
  }
}

customElements.define("hzn-forgot-password", HznForgotPassword);
