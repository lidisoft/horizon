// <hzn-workspace-home>: /w/:ws. A placeholder until projects arrive: it
// names the workspace and points to its settings.

import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const styles = sheet(`
  :host { display: block; padding: var(--lds-space-12) var(--lds-space-6); }
  a { color: var(--lds-color-fg-link); }
`);

class HznWorkspaceHome extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    this.attachShadow({ mode: "open" }).adoptedStyleSheets = [styles];
    this.load();
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before !== after) this.load();
  }

  async load() {
    const { api, i18n } = this.context;
    const { t } = i18n;
    const slug = this.getAttribute("ws");
    let ws;
    try {
      ({ data: ws } = await api.get(`/w/${encodeURIComponent(slug)}`));
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`
        <lds-empty-state>
          <lds-icon slot="icon" name="search"></lds-icon>
          <h2 slot="title">${err.status === 404 ? t("workspace.notFound") : errorMessage(err, i18n)}</h2>
          <a href="/">${t("notFound.home")}</a>
        </lds-empty-state>`);
      return;
    }
    this.shadowRoot.innerHTML = String(html`
      <lds-empty-state>
        <lds-icon slot="icon" name="layers"></lds-icon>
        <h2 slot="title">${ws.name}</h2>
        ${t("workspace.empty")}
        <a slot="actions" href="/w/${ws.slug}/settings/members">${t("workspace.toMembers")}</a>
      </lds-empty-state>`);
  }
}

customElements.define("hzn-workspace-home", HznWorkspaceHome);
