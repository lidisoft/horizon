// <hzn-release-dialog>: creates and edits a project's releases.
//   create({ ws, project })           a new release
//   edit({ ws, project, release })    name, description and dates
// On success it fires hzn-release-changed on the document.

import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, showFieldErrors } from "horizon/core/forms.js";

const styles = sheet(`
  form { display: grid; gap: var(--lds-space-4); }
  lds-alert:empty { display: none; }
  .actions { display: flex; justify-content: flex-end; gap: var(--lds-space-3); }
`);

class HznReleaseDialog extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [styles];
    const { t } = this.context.i18n;
    root.innerHTML = String(html`
      <lds-modal size="md">
        <span slot="header" data-heading></span>
        <form novalidate></form>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button variant="primary" data-submit></lds-button>
        </div>
      </lds-modal>`);
    this.modal = root.querySelector("lds-modal");
    this.form = root.querySelector("form");
    this.form.addEventListener("submit", (e) => {
      e.preventDefault();
      this.submit();
    });
    this.form.addEventListener("lds-change", (e) => {
      if (e.target.matches("lds-date-range")) this.dates = { start: e.detail.start, end: e.detail.end };
    });
    root.querySelector("[data-close]").addEventListener("click", () => this.modal.removeAttribute("open"));
    root.querySelector("[data-submit]").addEventListener("click", () => this.form.requestSubmit());
  }

  create({ ws, project }) {
    this.open({ ws, project, release: null });
  }

  edit({ ws, project, release }) {
    this.open({ ws, project, release });
  }

  open({ ws, project, release }) {
    const { t } = this.context.i18n;
    Object.assign(this, { ws, project, release });
    const r = release ?? { name: "", description: "", start_date: "", release_date: "" };
    this.dates = { start: r.start_date ?? "", end: r.release_date ?? "" };
    this.shadowRoot.querySelector("[data-heading]").textContent = t(release ? "releases.edit" : "releases.new");
    this.shadowRoot.querySelector("[data-submit]").textContent = t(release ? "account.save" : "releases.create");
    this.form.innerHTML = String(html`
      <lds-alert intent="danger" role="alert"></lds-alert>
      <lds-field label="${t("releases.name")}" required><lds-input name="name" required maxlength="100" value="${r.name}" placeholder="v1.0"></lds-input></lds-field>
      <lds-field label="${t("releases.description")}"><lds-textarea name="description" rows="3" maxlength="2000" value="${r.description}"></lds-textarea></lds-field>
      <lds-field label="${t("releases.dates")}">
        <lds-date-range start="${this.dates.start}" end="${this.dates.end}" label="${t("releases.dates")}"
          start-label="${t("releases.startDate")}" end-label="${t("releases.releaseDate")}"></lds-date-range>
        <span slot="help">${t("releases.datesHint")}</span>
      </lds-field>`);
    this.modal.setAttribute("open", "");
    requestAnimationFrame(() => this.form.querySelector('[name="name"]')?.focus());
  }

  async submit() {
    const { api, i18n } = this.context;
    const form = this.form;
    const alert = form.querySelector("lds-alert");
    alert.textContent = "";
    clearFieldErrors(form);
    const fd = new FormData(form);
    const body = {
      name: String(fd.get("name") ?? "").trim(),
      description: String(fd.get("description") ?? "").trim(),
      start_date: this.dates.start ?? "",
      release_date: this.dates.end ?? "",
    };
    const base = `/w/${encodeURIComponent(this.ws)}/projects/${encodeURIComponent(this.project)}/releases`;
    let saved;
    try {
      ({ data: saved } = await busy(this.shadowRoot.querySelector("[data-submit]"), () => this.release
        ? api.patch(`${base}/${this.release.id}`, { body, ifMatch: `"${this.release.version}"` })
        : api.post(base, { body })));
    } catch (err) {
      const rest = err.fields?.length ? showFieldErrors(form, err.fields, i18n.t, i18n.has) : [];
      if (!err.fields?.length || rest.length) alert.textContent = errorMessage(err, i18n);
      return;
    }
    this.modal.removeAttribute("open");
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t(this.release ? "account.saved" : "releases.created", { name: saved.name }) });
    document.dispatchEvent(new CustomEvent("hzn-release-changed", { detail: { project: this.project, id: saved.id } }));
  }
}

customElements.define("hzn-release-dialog", HznReleaseDialog);
