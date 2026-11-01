// <hzn-project-home>: /w/:ws/p/:key — leads to the project's issues until
// the overview arrives (phase 6), resolving a former key on the way.

import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const styles = sheet(`
  :host { display: block; padding: var(--lds-space-12) var(--lds-space-6); }
  a { color: var(--lds-color-fg-link); }
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
    router.navigate(`/w/${ws}/p/${p.key}/issues`, { replace: true });
  }
}

customElements.define("hzn-project-home", HznProjectHome);
