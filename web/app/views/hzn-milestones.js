// <hzn-milestones>: /w/:ws/milestones — every milestone the user sees, the
// workspace's and its projects', by date (docs/domain.md#milestone).

import "horizon/components/hzn-milestone-list.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";

const styles = sheet(`
  :host { display: flex; flex-direction: column; min-block-size: 100%; }
  header.bar { display: flex; align-items: center; min-block-size: 3.25rem; padding: 0 var(--lds-space-5);
    border-block-end: 1px solid var(--lds-color-border-default); font-weight: var(--lds-font-weight-semibold); }
  .content { padding: var(--lds-space-5); max-inline-size: 64rem; }
`);

class HznMilestones extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    this.attachShadow({ mode: "open" }).adoptedStyleSheets = [styles];
    this.render();
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.render();
  }

  render() {
    const { t } = this.context.i18n;
    this.shadowRoot.innerHTML = String(html`
      <header class="bar">${t("milestones.title")}</header>
      <div class="content"><hzn-milestone-list ws="${this.getAttribute("ws")}"></hzn-milestone-list></div>`);
  }
}

customElements.define("hzn-milestones", HznMilestones);
