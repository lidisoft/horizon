// <hzn-home>: the landing view. Until issues and projects exist it reports
// the instance's readiness checks, which proves the frame, the services and
// the server end to end.

import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { displayKeys } from "horizon/core/shortcuts.js";

const styles = sheet(`
  :host { display: block; padding: var(--lds-space-12) var(--lds-space-6); }
  .checks { display: grid; grid-template-columns: max-content max-content; gap: var(--lds-space-2) var(--lds-space-6); margin: 0; justify-content: center; }
  .checks dt { color: var(--lds-color-fg-muted); }
  .checks dd { margin: 0; }
  h3 { margin: var(--lds-space-8) 0 var(--lds-space-3); text-align: center; font-size: var(--lds-font-size-sm); }
  p.status { text-align: center; color: var(--lds-color-fg-muted); }
`);

const CHECKS = ["postgres", "nats", "s3", "schema"];

class HznHome extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const { t } = this.context.i18n;
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [styles];
    root.innerHTML = String(html`
      <lds-empty-state>
        <lds-icon slot="icon" name="rocket"></lds-icon>
        <h2 slot="title">${t("home.title")}</h2>
        ${t("home.body", { palette: displayKeys("mod+k").join(""), help: "?" })}
      </lds-empty-state>
      <section aria-busy="true" aria-labelledby="checks-title">
        <h3 id="checks-title">${t("home.checks")}</h3>
        <p class="status">${t("home.checking")}</p>
      </section>
    `);
    this.loadChecks();
  }

  async loadChecks() {
    const { t } = this.context.i18n;
    const section = this.shadowRoot.querySelector("section");
    let checks = null;
    try {
      // /readyz is a probe outside /api/v1; it answers 503 with the same body.
      const res = await fetch("/readyz", { headers: { Accept: "application/json" } });
      checks = (await res.json()).checks;
    } catch {
      // network failure: reported below
    }
    section.removeAttribute("aria-busy");
    if (!checks) {
      section.querySelector(".status").textContent = t("home.check.unreachable");
      return;
    }
    section.querySelector(".status").replaceWith(document.createRange().createContextualFragment(String(html`
      <dl class="checks">
        ${CHECKS.filter((name) => name in checks).map((name) => {
          const ok = checks[name] === "ok";
          return html`<dt>${t(`check.${name}`)}</dt>
            <dd><lds-badge size="sm" intent="${ok ? "success" : "danger"}" title="${ok ? "" : checks[name]}">${t(ok ? "home.check.ok" : "home.check.failing")}</lds-badge></dd>`;
        })}
      </dl>
    `)));
  }
}

customElements.define("hzn-home", HznHome);
