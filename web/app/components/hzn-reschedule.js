// <hzn-reschedule>: changes an issue's dates the timeline's way
// (docs/frontend.md#timeline). propose({ ws, issue, start, due }) asks the
// server for the plan; with nothing else to move it saves at once,
// otherwise a dialog lists the issues that would move and by how many
// working days, and offers to move them all, the issue alone, or nothing.
// It resolves to true when something was saved, and fires
// hzn-issue-changed on the document.

import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const styles = sheet(`
  .moves { display: grid; gap: var(--lds-space-2); margin: var(--lds-space-3) 0 0; padding: 0; list-style: none; }
  .moves li { display: grid; grid-template-columns: 5rem minmax(0, 1fr) auto; gap: var(--lds-space-3); align-items: baseline; }
  .moves .shift { font-variant-numeric: tabular-nums; color: var(--lds-color-fg-muted); }
  .moves .blocked { color: var(--lds-color-danger-fg); }
  .actions { display: flex; justify-content: flex-end; gap: var(--lds-space-2); flex-wrap: wrap; }
`);

class HznReschedule extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [styles];
    root.innerHTML = String(html`
      <lds-modal size="md" data-plan>
        <span slot="header" data-plan-title></span>
        <div data-plan-body></div>
        <div slot="footer" class="actions" data-plan-actions></div>
      </lds-modal>`);
    this.modal = root.querySelector("[data-plan]");
    root.addEventListener("click", (e) => {
      const action = e.target.closest?.("[data-action]")?.dataset.action;
      if (action === "plan-cancel") this.finish(false);
      else if (action === "plan-alone") this.apply(false);
      else if (action === "plan-all") this.apply(true);
    });
    this.modal.addEventListener("lds-close", () => this.pending && this.finish(false));
  }

  path(ws, key) {
    return `/w/${encodeURIComponent(ws)}/issues/${encodeURIComponent(key)}`;
  }

  /** start or due "" clears that date. */
  propose({ ws, issue, start, due }) {
    const { api, i18n } = this.context;
    const body = { start_date: start, due_date: due };
    const ifMatch = `"${issue.version}"`;
    return new Promise((resolve) => {
      (async () => {
        try {
          const { data: plan } = await api.post(`${this.path(ws, issue.key)}/schedule`, { body, ifMatch });
          if (!plan.moves.length) {
            await api.post(`${this.path(ws, issue.key)}/schedule`, { body: { ...body, confirm: true }, ifMatch });
            this.changed(issue.key);
            return resolve(true);
          }
          this.pending = { ws, issue, body, plan, resolve };
          this.show();
        } catch (err) {
          globalThis.LDS?.toast?.({ intent: "danger", title: err.code === "version_conflict" ? i18n.t("issues.conflict") : errorMessage(err, i18n) });
          this.changed(issue.key);
          resolve(false);
        }
      })();
    });
  }

  show() {
    const { t } = this.context.i18n;
    const { issue, plan } = this.pending;
    const movable = plan.moves.filter((m) => !m.blocked).length;
    this.shadowRoot.querySelector("[data-plan-title]").textContent = t("timeline.planTitle", { key: issue.key });
    this.shadowRoot.querySelector("[data-plan-body]").innerHTML = String(html`
      <p>${t("timeline.planIntro", { key: issue.key, count: plan.moves.length })}</p>
      <ul class="moves">${plan.moves.map((m) => html`
        <li data-move="${m.key}"><strong>${m.key}</strong><span>${m.title}</span>
          ${m.blocked ? html`<span class="blocked">${t(`timeline.blocked.${m.blocked}`)}</span>`
            : html`<span class="shift">${t("timeline.shift", { count: m.shift })} · ${m.to.start_date ?? ""}${m.to.start_date ? " → " : ""}${m.to.due_date ?? ""}</span>`}
        </li>`)}</ul>`);
    this.shadowRoot.querySelector("[data-plan-actions]").innerHTML = String(html`
      <lds-button variant="ghost" data-action="plan-cancel">${t("tokens.cancel")}</lds-button>
      <lds-button variant="secondary" data-action="plan-alone">${t("timeline.moveAlone", { key: issue.key })}</lds-button>
      ${movable ? html`<lds-button variant="primary" data-action="plan-all">${t("timeline.moveAll", { count: movable + 1 })}</lds-button>` : ""}`);
    this.modal.setAttribute("open", "");
  }

  async apply(all) {
    const { api, i18n } = this.context;
    const { ws, issue, body } = this.pending;
    const ifMatch = `"${issue.version}"`;
    let saved = false;
    try {
      if (all) {
        const { data: plan } = await api.post(`${this.path(ws, issue.key)}/schedule`, { body: { ...body, confirm: true }, ifMatch });
        const failed = plan.moves.filter((m) => m.result !== "moved").length;
        if (failed) globalThis.LDS?.toast?.({ intent: "warning", title: i18n.t("timeline.notMoved", { count: failed }) });
      } else {
        await api.patch(this.path(ws, issue.key), { body, ifMatch });
      }
      saved = true;
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: err.code === "version_conflict" ? i18n.t("issues.conflict") : errorMessage(err, i18n) });
    }
    this.finish(saved);
  }

  finish(saved) {
    const p = this.pending;
    this.pending = null;
    this.modal.removeAttribute("open");
    if (!p) return;
    this.changed(p.issue.key);
    p.resolve(saved);
  }

  changed(key) {
    document.dispatchEvent(new CustomEvent("hzn-issue-changed", { detail: { key } }));
  }
}

customElements.define("hzn-reschedule", HznReschedule);
