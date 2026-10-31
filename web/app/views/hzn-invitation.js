// <hzn-invitation>: /invitation#token=… — joins a workspace from an
// emailed invitation. Signed in with the invited email, it's one click;
// with an existing account, the user signs in first (the token waits in
// session storage, never in a URL); otherwise a name and password create
// the account.

import { requestContext } from "horizon/core/context.js";
import { html } from "horizon/core/dom.js";
import { busy, errorMessage, formValues, setAutocomplete, showFieldErrors, submitOnEnter } from "horizon/core/forms.js";
import { authPage, authStyles } from "horizon/views/auth-page.js";

const TOKEN_KEY = "horizon.invitation";
const MIN_LENGTH = 12; // identity.MinPasswordLength

function stash(token) {
  try {
    if (token) sessionStorage.setItem(TOKEN_KEY, token);
    else sessionStorage.removeItem(TOKEN_KEY);
  } catch {
    // storage blocked: the user opens the emailed link again after signing in
  }
}

function stashed() {
  try {
    return sessionStorage.getItem(TOKEN_KEY) || "";
  } catch {
    return "";
  }
}

class HznInvitation extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    this.token = new URLSearchParams(location.hash.slice(1)).get("token") || stashed();
    if (location.hash) history.replaceState(history.state, "", location.pathname + location.search);
    this.attachShadow({ mode: "open" }).adoptedStyleSheets = [authStyles];
    this.unsubscribe = this.context.session.subscribe(() => this.load());
    this.load();
  }

  disconnectedCallback() {
    this.unsubscribe?.();
  }

  show(title, body) {
    this.shadowRoot.innerHTML = String(authPage(this.context.i18n.t, title, body));
  }

  async load() {
    const { api, i18n, session } = this.context;
    const { t } = i18n;
    if (!this.token) {
      this.show(t("invitation.title"), html`<lds-alert intent="danger" role="alert">${t("error.invitation_invalid")}</lds-alert>`);
      return;
    }
    let preview;
    try {
      ({ data: preview } = await api.post("/invitations/lookup", { body: { token: this.token } }));
    } catch (err) {
      stash("");
      this.show(t("invitation.title"), html`<lds-alert intent="danger" role="alert">${errorMessage(err, i18n)}</lds-alert>
        <p class="links"><a href="/">${t("notFound.home")}</a></p>`);
      return;
    }
    this.preview = preview;
    const title = t("invitation.join", { workspace: preview.workspace.name });
    const lead = html`<p class="lead">${preview.inviter_name
      ? t("invitation.leadFrom", { inviter: preview.inviter_name, email: preview.email })
      : t("invitation.lead", { email: preview.email })}</p>`;
    const user = session.user;

    if (user && user.email.toLowerCase() === preview.email.toLowerCase()) {
      this.show(title, html`${lead}
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <lds-button type="submit" full-width>${t("invitation.accept")}</lds-button>
        </form>`);
    } else if (user) {
      this.show(title, html`${lead}
        <lds-alert intent="warning" role="alert">${t("invitation.otherAccount", { email: user.email, invited: preview.email })}</lds-alert>
        <p class="links"><lds-button variant="secondary" data-action="sign-out">${t("invitation.signOut")}</lds-button></p>`);
      this.shadowRoot.querySelector('[data-action="sign-out"]').addEventListener("click", () => {
        stash(this.token);
        session.logout();
      });
      return;
    } else if (preview.account_exists) {
      this.show(title, html`${lead}
        <p class="lead">${t("invitation.signInFirst")}</p>
        <lds-button full-width data-action="sign-in">${t("invitation.signIn")}</lds-button>`);
      this.shadowRoot.querySelector('[data-action="sign-in"]').addEventListener("click", () => {
        stash(this.token);
        this.context.router.navigate(`/sign-in?next=${encodeURIComponent("/invitation")}`);
      });
      return;
    } else {
      this.show(title, html`${lead}
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <lds-field label="${t("field.name")}" required><lds-input name="name" required maxlength="200"></lds-input></lds-field>
          <lds-field label="${t("field.password")}" required>
            <lds-input name="password" type="password" required></lds-input>
            <span slot="help">${t("reset.lead", { min: MIN_LENGTH })}</span>
          </lds-field>
          <lds-button type="submit" full-width>${t("invitation.create")}</lds-button>
        </form>`);
      setAutocomplete(this.shadowRoot.querySelector('[name="name"]'), "name");
      setAutocomplete(this.shadowRoot.querySelector('[name="password"]'), "new-password");
    }
    const form = this.shadowRoot.querySelector("form");
    submitOnEnter(form);
    form.addEventListener("submit", (e) => {
      e.preventDefault();
      this.accept(form);
    });
  }

  async accept(form) {
    const { api, i18n, session, workspaces, router } = this.context;
    const alert = form.querySelector("lds-alert");
    alert.textContent = "";
    let joined;
    try {
      ({ data: joined } = await busy(form, () => api.post("/invitations/accept", { body: { token: this.token, ...formValues(form) } })));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else alert.textContent = errorMessage(err, i18n);
      return;
    }
    stash("");
    this.unsubscribe?.();
    if (session.status !== "signed-in") await session.load(); // the account was just created and signed in
    await workspaces.refresh();
    router.navigate(`/w/${joined.workspace.slug}`, { replace: true });
  }
}

customElements.define("hzn-invitation", HznInvitation);
