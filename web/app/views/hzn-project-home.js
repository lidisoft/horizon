// <hzn-project-home>: /w/:ws/p/:key — a placeholder until issues arrive.
// A former key redirects to the current one.

import { paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const styles = sheet(`
  :host { display: block; padding: var(--lds-space-12) var(--lds-space-6); }
  a { color: var(--lds-color-fg-link); }
  .dot { display: inline-block; width: 0.75rem; height: 0.75rem; border-radius: var(--lds-radius-xs); }
`);

class HznProjectHome extends HTMLElement {
  static observedAttributes = ["ws", "key"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    this.attachShadow({ mode: "open" }).adoptedStyleSheets = [styles];
    this.load();
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.load();
  }

  async load() {
    const { api, i18n, router } = this.context;
    const { t } = i18n;
    const ws = this.getAttribute("ws");
    const key = this.getAttribute("key");
    let p;
    try {
      ({ data: p } = await api.get(`/w/${encodeURIComponent(ws)}/projects/${encodeURIComponent(key)}`));
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`
        <lds-empty-state><lds-icon slot="icon" name="search"></lds-icon>
          <h2 slot="title">${err.status === 404 ? t("projects.notFound") : errorMessage(err, i18n)}</h2>
          <a href="/w/${ws}">${t("notFound.home")}</a>
        </lds-empty-state>`);
      return;
    }
    if (p.key !== key.toUpperCase()) {
      router.navigate(`/w/${ws}/p/${p.key}`, { replace: true });
      return;
    }
    this.shadowRoot.innerHTML = String(html`
      <lds-empty-state>
        <span slot="icon" class="dot" data-colour="${p.colour}"></span>
        <h2 slot="title">${p.name} <span class="muted">${p.key}</span></h2>
        ${p.archived_at ? t("projects.isArchived") : t("projects.empty")}
        <a slot="actions" href="/w/${ws}/settings/projects/${p.key}">${t("projects.settings")}</a>
      </lds-empty-state>`);
    paintSwatches(this.shadowRoot);
  }
}

customElements.define("hzn-project-home", HznProjectHome);
