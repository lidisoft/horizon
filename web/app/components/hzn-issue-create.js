// <hzn-issue-create>: the New issue dialog. show({ ws, project?, parent? })
// opens it; without a project it asks for one. On success it fires
// hzn-issue-changed on the document with the new key.

import { PRIORITIES, projectData } from "horizon/components/issue-ui.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, showFieldErrors, submitOnEnter } from "horizon/core/forms.js";

const styles = sheet(`
  form { display: grid; gap: var(--lds-space-4); }
  lds-alert:empty { display: none; }
  .row { display: grid; grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr)); gap: var(--lds-space-3); }
  .actions { display: flex; justify-content: flex-end; gap: var(--lds-space-3); }
`);

class HznIssueCreate extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [styles];
    const { t } = this.context.i18n;
    root.innerHTML = String(html`
      <lds-modal size="md">
        <span slot="header">${t("issues.new")}</span>
        <form novalidate></form>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button variant="primary" data-action="create">${t("issues.create")}</lds-button>
        </div>
      </lds-modal>`);
    this.modal = root.querySelector("lds-modal");
    this.form = root.querySelector("form");
    submitOnEnter(this.form);
    this.form.addEventListener("submit", (e) => {
      e.preventDefault();
      this.submit();
    });
    root.querySelector("[data-close]").addEventListener("click", () => this.modal.removeAttribute("open"));
    root.querySelector('[data-action="create"]').addEventListener("click", () => this.form.requestSubmit());
    this.form.addEventListener("lds-change", (e) => {
      if (e.target.getAttribute("name") === "project") this.fill(e.target.getAttribute("value"));
    });
  }

  /** Opens the dialog; options: ws, project (key), parent (key). */
  async show({ ws, project, parent = "" }) {
    const { api, i18n } = this.context;
    this.ws = ws;
    this.parent = parent;
    this.fixedProject = project;
    this.projects = null;
    if (!project) {
      try {
        const { data } = await api.get(`/w/${encodeURIComponent(ws)}/projects`);
        this.projects = data.items.filter((p) => !p.archived_at);
      } catch (err) {
        globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
        return;
      }
      project = this.projects[0]?.key;
    }
    await this.fill(project);
    this.modal.setAttribute("open", "");
    requestAnimationFrame(() => this.form.querySelector('[name="title"]')?.focus());
  }

  /** Renders the form for a project, keeping the typed title. */
  async fill(projectKey) {
    const { i18n } = this.context;
    const { t } = i18n;
    const title = this.form.querySelector('[name="title"]') ? String(new FormData(this.form).get("title") ?? "") : "";
    if (!projectKey) {
      this.form.innerHTML = String(html`<lds-alert intent="info">${t("issues.noProject")}</lds-alert>`);
      return;
    }
    try {
      this.data = await projectData(this.context, this.ws, projectKey);
    } catch (err) {
      this.form.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    const d = this.data;
    const types = d.config.issue_types;
    // A type that fits: one taking any parent, else one that can stand alone
    // (or, under a parent, one that accepts it).
    const firstType = types.find((x) => x.parent_rule === "any") ?? types.find((x) => x.parent_rule !== (this.parent ? "none" : "required")) ?? types[0];
    const scale = d.config.estimation;
    this.form.innerHTML = String(html`
      <lds-alert intent="danger" role="alert"></lds-alert>
      ${this.projects ? html`
        <lds-field label="${t("issues.project")}">
          <lds-select name="project" value="${projectKey}">${this.projects.map((p) => html`<option value="${p.key}">${p.name} (${p.key})</option>`)}</lds-select>
        </lds-field>` : ""}
      <div class="row">
        <lds-field label="${t("issues.type")}" required>
          <lds-select name="type_id" value="${firstType?.id ?? ""}">${types.map((x) => html`<option value="${x.id}">${x.name}</option>`)}</lds-select>
        </lds-field>
        <lds-field label="${t("issues.priority")}">
          <lds-select name="priority" value="none">${PRIORITIES.map((p) => html`<option value="${p}">${t(`priority.${p}`)}</option>`)}</lds-select>
        </lds-field>
      </div>
      <lds-field label="${t("issues.titleField")}" required><lds-input name="title" required maxlength="255" value="${title}"></lds-input></lds-field>
      <lds-field label="${t("issues.description")}"><lds-textarea name="description" rows="4"></lds-textarea></lds-field>
      <div class="row">
        <lds-field label="${t("issues.assignee")}">
          <lds-combobox name="assignee_id" placeholder="${t("issue.unassigned")}" label="${t("issues.assignee")}">
            ${d.peopleList.map((p) => html`<lds-option value="${p.id}"><lds-avatar slot="start" size="sm" name="${p.name}"></lds-avatar>${p.name}</lds-option>`)}
          </lds-combobox>
        </lds-field>
        ${scale.kind !== "none" ? html`
          <lds-field label="${t("issues.estimate")}">
            <lds-select name="estimate" value=""><option value="">${t("issues.noEstimate")}</option>${scale.values.map((v) => html`<option value="${v.label}">${v.label}</option>`)}</lds-select>
          </lds-field>` : ""}
      </div>
      <lds-field label="${t("issues.labels")}">
        <lds-combobox name="labels" multiple label="${t("issues.labels")}">
          ${d.config.labels.filter((l) => !l.disabled).map((l) => html`<lds-option value="${l.id}">${l.name}</lds-option>`)}
        </lds-combobox>
      </lds-field>
      <lds-field label="${t("issues.parent")}"><lds-input name="parent" value="${this.parent}" placeholder="${d.project.key}-1"></lds-input></lds-field>`);
  }

  async submit() {
    const { api, i18n } = this.context;
    const form = this.form;
    const alert = form.querySelector("lds-alert");
    if (!alert) return;
    alert.textContent = "";
    clearFieldErrors(form);
    const fd = new FormData(form);
    const value = (name) => String(fd.get(name) ?? "").trim();
    const body = {
      project: this.data.project.key,
      type_id: value("type_id"),
      title: value("title"),
      description: String(fd.get("description") ?? ""),
      priority: value("priority") || "none",
      parent: value("parent"),
    };
    if (value("assignee_id")) body.assignee_id = value("assignee_id");
    if (value("estimate")) body.estimate = value("estimate");
    const labels = fd.getAll("labels").map(String).filter(Boolean);
    if (labels.length) body.labels = labels;
    let created;
    try {
      ({ data: created } = await busy(this.shadowRoot.querySelector('[data-action="create"]'), () =>
        api.post(`/w/${encodeURIComponent(this.ws)}/issues`, { body })));
    } catch (err) {
      const rest = err.fields?.length ? showFieldErrors(form, err.fields, i18n.t, i18n.has) : [];
      if (!err.fields?.length || rest.length) alert.textContent = errorMessage(err, i18n);
      return;
    }
    this.modal.removeAttribute("open");
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("issues.created", { key: created.key }) });
    document.dispatchEvent(new CustomEvent("hzn-issue-changed", { detail: { key: created.key } }));
  }
}

customElements.define("hzn-issue-create", HznIssueCreate);
