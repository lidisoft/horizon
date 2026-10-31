// <hzn-sign-in>: email and password sign-in. On success the session
// changes and the frame's route guard sends the user to ?next= (same-origin
// paths only) or home.

import { requestContext } from "horizon/core/context.js";
import { html } from "horizon/core/dom.js";
import { busy, errorMessage, formValues, setAutocomplete, submitOnEnter } from "horizon/core/forms.js";
import { authPage, authStyles } from "horizon/views/auth-page.js";

class HznSignIn extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const { t } = this.context.i18n;
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [authStyles];
    root.innerHTML = String(authPage(t, t("signIn.title"), html`
      <form novalidate>
        <lds-alert intent="danger" role="alert"></lds-alert>
        <lds-field label="${t("field.email")}" required>
          <lds-input name="email" type="email" required></lds-input>
        </lds-field>
        <lds-field label="${t("field.password")}" required>
          <lds-input name="password" type="password" required></lds-input>
        </lds-field>
        <lds-button type="submit" variant="primary" full-width>${t("signIn.submit")}</lds-button>
      </form>
      <p class="links"><a href="/forgot-password">${t("signIn.forgot")}</a></p>
    `));

    const form = root.querySelector("form");
    setAutocomplete(root.querySelector('[name="email"]'), "username");
    setAutocomplete(root.querySelector('[name="password"]'), "current-password");
    submitOnEnter(form);
    form.addEventListener("submit", (event) => {
      event.preventDefault();
      this.submit(form);
    });
    requestAnimationFrame(() => root.querySelector('[name="email"]').focus());
  }

  async submit(form) {
    const { i18n, session } = this.context;
    const alert = form.querySelector("lds-alert");
    const { email = "", password = "" } = formValues(form);
    if (!email.trim() || !password) {
      alert.textContent = i18n.t("signIn.missing");
      return;
    }
    alert.textContent = "";
    try {
      await busy(form, () => session.login(email.trim(), password));
    } catch (err) {
      alert.textContent = errorMessage(err, i18n);
      form.querySelector('[name="password"]').focus();
    }
  }
}

customElements.define("hzn-sign-in", HznSignIn);
