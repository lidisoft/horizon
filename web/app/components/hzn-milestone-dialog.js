// <hzn-milestone-dialog>: creates, edits and links milestones.
//   create({ ws, projects, project? })     a new milestone, of a project or of the workspace
//   edit({ ws, milestone })                name, date and description
//   link({ ws, milestone, keys })          the issues it tracks, as keys
// On success it fires hzn-milestone-changed on the document.

import { ISSUE_KEY } from "horizon/components/issue-ui.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, showFieldErrors } from "horizon/core/forms.js";

const styles = sheet(`
  form { display: grid; gap: var(--lds-space-4); }
  lds-alert:empty { display: none; }
  .actions { display: flex; justify-content: flex-end; gap: var(--lds-space-3); }
`);

/** Issue keys in text: HZN-1, OPS-7 (commas, spaces or lines between). */
export function parseKeys(text) {
  return [...new Set(String(text).split(/[\s,;]+/).map((k) => k.trim().toUpperCase()).filter((k) => ISSUE_KEY.test(k)))];
}

class HznMilestoneDialog extends HTMLElement {
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
      if (e.target.matches("[data-date]")) this.date = e.detail.value;
      if (e.target.matches("[data-project]")) this.project = e.detail.value;
    });
    root.querySelector("[data-close]").addEventListener("click", () => this.modal.removeAttribute("open"));
    root.querySelector("[data-submit]").addEventListener("click", () => this.form.requestSubmit());
  }

  create({ ws, projects, project = "" }) {
    this.open("create", { ws, milestone: null, projects, project });
  }

  edit({ ws, milestone }) {
    this.open("edit", { ws, milestone });
  }

  open(mode, { ws, milestone, projects = [], project = "" }) {
    const { t } = this.context.i18n;
    Object.assign(this, { mode, ws, milestone, project });
    const m = milestone ?? { name: "", description: "", date: "" };
    this.date = m.date;
    this.shadowRoot.querySelector("[data-heading]").textContent = t(milestone ? "milestones.edit" : "milestones.new");
    this.shadowRoot.querySelector("[data-submit]").textContent = t(milestone ? "account.save" : "milestones.create");
    this.form.innerHTML = String(html`
      <lds-alert intent="danger" role="alert"></lds-alert>
      ${milestone ? "" : html`
        <lds-field label="${t("milestones.scope")}">
          <lds-select data-project value="${project}">
            ${projects.map((p) => html`<option value="${p.key}">${p.name}</option>`)}
            <option value="">${t("milestones.workspace")}</option>
          </lds-select>
        </lds-field>`}
      <lds-field label="${t("milestones.name")}" required><lds-input name="name" required maxlength="100" value="${m.name}"></lds-input></lds-field>
      <lds-field label="${t("milestones.date")}" required>
        <lds-date-picker data-date name="date" value="${m.date}" aria-label="${t("milestones.date")}"></lds-date-picker>
      </lds-field>
      <lds-field label="${t("milestones.description")}"><lds-textarea name="description" rows="3" maxlength="2000" value="${m.description}"></lds-textarea></lds-field>`);
    this.modal.setAttribute("open", "");
    requestAnimationFrame(() => this.form.querySelector('[name="name"]')?.focus());
  }

  /** Opens the dialog to change the issues a milestone tracks. */
  link({ ws, milestone, keys }) {
    const { t } = this.context.i18n;
    Object.assign(this, { mode: "link", ws, milestone, keys });
    this.shadowRoot.querySelector("[data-heading]").textContent = t("milestones.linkTitle", { name: milestone.name });
    this.shadowRoot.querySelector("[data-submit]").textContent = t("account.save");
    this.form.innerHTML = String(html`
      <lds-alert intent="danger" role="alert"></lds-alert>
      <lds-field label="${t("milestones.issues")}">
        <lds-textarea name="keys" rows="4" value="${keys.join(", ")}" placeholder="HZN-41, HZN-44"></lds-textarea>
        <span slot="help">${t(milestone.project_id ? "milestones.linkHintProject" : "milestones.linkHint")}</span>
      </lds-field>`);
    this.modal.setAttribute("open", "");
    requestAnimationFrame(() => this.form.querySelector('[name="keys"]')?.focus());
  }

  async submit() {
    const { api, i18n } = this.context;
    const form = this.form;
    const alert = form.querySelector("lds-alert");
    alert.textContent = "";
    clearFieldErrors(form);
    const fd = new FormData(form);
    const base = `/w/${encodeURIComponent(this.ws)}/milestones`;
    let request;
    if (this.mode === "link") {
      const next = parseKeys(fd.get("keys") ?? "");
      const add = next.filter((k) => !this.keys.includes(k));
      const remove = this.keys.filter((k) => !next.includes(k));
      request = () => api.post(`${base}/${this.milestone.id}/issues`, { body: { add, remove } });
    } else {
      const body = {
        name: String(fd.get("name") ?? "").trim(),
        description: String(fd.get("description") ?? "").trim(),
        date: this.date ?? "",
      };
      if (this.mode === "create") {
        body.project = this.project ?? "";
        request = () => api.post(base, { body });
      } else {
        request = () => api.patch(`${base}/${this.milestone.id}`, { body, ifMatch: `"${this.milestone.version}"` });
      }
    }
    let saved;
    try {
      ({ data: saved } = await busy(this.shadowRoot.querySelector("[data-submit]"), request));
    } catch (err) {
      const rest = err.fields?.length ? showFieldErrors(form, err.fields, i18n.t, i18n.has) : [];
      if (!err.fields?.length || rest.length) alert.textContent = err.fields?.some((f) => f.field === "add") ? i18n.t("milestones.unknownIssues") : errorMessage(err, i18n);
      return;
    }
    this.modal.removeAttribute("open");
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t(this.mode === "create" ? "milestones.created" : "account.saved", { name: saved.name }) });
    document.dispatchEvent(new CustomEvent("hzn-milestone-changed", { detail: { id: saved.id } }));
  }
}

customElements.define("hzn-milestone-dialog", HznMilestoneDialog);
