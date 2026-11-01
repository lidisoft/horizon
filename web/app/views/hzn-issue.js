// <hzn-issue>: /w/:ws/issues/:issue — the issue's page (wireframes:
// IssueDetail). A former key redirects to the current one.

import "horizon/components/hzn-issue-detail.js";
import { requestContext } from "horizon/core/context.js";
import { sheet } from "horizon/core/dom.js";

const styles = sheet(`:host { display: block; min-block-size: 100%; }`);

class HznIssue extends HTMLElement {
  static observedAttributes = ["ws", "issue"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [styles];
    this.detail = document.createElement("hzn-issue-detail");
    this.detail.setAttribute("mode", "page");
    this.sync();
    root.append(this.detail);
  }

  attributeChangedCallback() {
    if (this.detail) this.sync();
  }

  sync() {
    for (const name of ["ws", "issue"]) {
      const v = this.getAttribute(name);
      if (v && this.detail.getAttribute(name) !== v) this.detail.setAttribute(name, v);
    }
  }
}

customElements.define("hzn-issue", HznIssue);
