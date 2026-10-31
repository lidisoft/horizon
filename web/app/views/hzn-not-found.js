// <hzn-not-found path="/nowhere">: shown for URLs no route matches.

import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";

const styles = sheet(`:host { display: block; padding: var(--lds-space-12) var(--lds-space-6); }`);

class HznNotFound extends HTMLElement {
  static observedAttributes = ["path"];

  connectedCallback() {
    if (!this.shadowRoot) {
      this.context = requestContext(this);
      this.attachShadow({ mode: "open" }).adoptedStyleSheets = [styles];
    }
    this.render();
  }

  attributeChangedCallback() {
    if (this.shadowRoot) this.render();
  }

  render() {
    const { i18n, router } = this.context;
    this.shadowRoot.innerHTML = String(html`
      <lds-empty-state>
        <lds-icon slot="icon" name="search"></lds-icon>
        <h2 slot="title">${i18n.t("notFound.title")}</h2>
        ${i18n.t("notFound.body", { path: this.getAttribute("path") || "" })}
        <div slot="actions"><a href="${router.href("home")}">${i18n.t("notFound.home")}</a></div>
      </lds-empty-state>
    `);
  }
}

customElements.define("hzn-not-found", HznNotFound);
