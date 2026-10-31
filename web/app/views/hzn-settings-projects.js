// <hzn-settings-projects>: /w/:ws/settings/projects[?new=1] — the projects
// the caller sees, and creating one (project.create).

import { COLOURS, paintSwatches, suggestKey } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, ldsLightSheet, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, formValues, showFieldErrors } from "horizon/core/forms.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

const styles = sheet(`
  .dot { display: inline-block; width: 0.625rem; height: 0.625rem; border-radius: var(--lds-radius-xs); margin-inline-end: var(--lds-space-2); }
  .key { font-family: var(--lds-font-family-mono); font-size: var(--lds-font-size-xs); }
  tbody tr { cursor: pointer; }
  a.name { color: inherit; text-decoration: none; font-weight: var(--lds-font-weight-medium); }
  lds-modal form { display: grid; gap: var(--lds-space-4); }
  .row { display: grid; grid-template-columns: 1fr 10rem; gap: var(--lds-space-4); }
`);

class HznSettingsProjects extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [ldsLightSheet(), settingsStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    this.load();
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.load();
  }

  async load() {
    const { api, i18n } = this.context;
    const slug = encodeURIComponent(this.getAttribute("ws"));
    try {
      const [ws, projects, members] = await Promise.all([api.get(`/w/${slug}`), api.get(`/w/${slug}/projects`), api.get(`/w/${slug}/members`).catch(() => null)]);
      this.ws = ws.data;
      this.projects = projects.data.items;
      this.members = members?.data.items ?? [];
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    this.canCreate = this.ws.can.includes("project.create");
    this.render();
    if (this.canCreate && this.context.router.current?.query.get("new") === "1") this.openNew();
  }

  render() {
    const { t } = this.context.i18n;
    const lead = new Map(this.members.map((m) => [m.user_id, m.name]));
    const body = this.projects.length ? html`
      <lds-table density="compact">
        <table>
          <thead><tr>
            <th scope="col">${t("projects.name")}</th>
            <th scope="col">${t("projects.key")}</th>
            <th scope="col">${t("projects.visibility")}</th>
            <th scope="col">${t("projects.lead")}</th>
            <th scope="col">${t("projects.status")}</th>
          </tr></thead>
          <tbody>${this.projects.map((p) => html`
            <tr data-key="${p.key}">
              <td><span class="dot" data-colour="${p.colour}"></span><a class="name" href="/w/${this.ws.slug}/settings/projects/${p.key}">${p.name}</a></td>
              <td><span class="key">${p.key}</span>${p.aliases.length ? html` <span class="muted">${t("projects.formerly", { keys: p.aliases.join(", ") })}</span>` : ""}</td>
              <td>${t(`projects.visibility.${p.visibility}`)}</td>
              <td>${lead.get(p.lead_id) ?? "—"}</td>
              <td>${p.archived_at ? html`<lds-badge size="sm">${t("projects.archived")}</lds-badge>` : html`<lds-badge size="sm" intent="success">${t("projects.active")}</lds-badge>`}</td>
            </tr>`)}</tbody>
        </table>
      </lds-table>` : html`
      <lds-empty-state><lds-icon slot="icon" name="layers"></lds-icon><h2 slot="title">${t("projects.none")}</h2>${t(this.canCreate ? "projects.noneCreate" : "projects.noneMember")}</lds-empty-state>`;
    const actions = this.canCreate ? html`<lds-button data-action="new"><lds-icon slot="start" name="plus"></lds-icon>${t("projects.new")}</lds-button>` : "";
    this.shadowRoot.innerHTML = String(settingsPage(t, this.ws, "projects", html`${body}${this.canCreate ? this.dialog() : ""}`, actions));
    paintSwatches(this.shadowRoot);
  }

  dialog() {
    const { t } = this.context.i18n;
    return html`
      <lds-modal data-dialog="new" size="md">
        <span slot="header">${t("projects.new")}</span>
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <div class="row">
            <lds-field label="${t("projects.name")}" required><lds-input name="name" required maxlength="80"></lds-input></lds-field>
            <lds-field label="${t("projects.key")}" required><lds-input name="key" required maxlength="10"></lds-input></lds-field>
          </div>
          <p class="muted">${t("projects.keyHelp")}</p>
          <div class="row">
            <lds-field label="${t("projects.visibility")}">
              <lds-select name="visibility" value="workspace">
                <option value="workspace">${t("projects.visibility.workspace")}</option>
                <option value="private">${t("projects.visibility.private")}</option>
              </lds-select>
            </lds-field>
            <lds-field label="${t("projects.colour")}">
              <lds-select name="colour" value="blue">${COLOURS.map((c) => html`<option value="${c}">${t(`colour.${c}`)}</option>`)}</lds-select>
            </lds-field>
          </div>
        </form>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button data-action="create">${t("projects.create")}</lds-button>
        </div>
      </lds-modal>`;
  }

  openNew() {
    const modal = this.shadowRoot.querySelector('[data-dialog="new"]');
    const form = modal.querySelector("form");
    const name = form.querySelector('[name="name"]');
    const key = form.querySelector('[name="key"]');
    let keyEdited = false;
    key.addEventListener("lds-input", () => (keyEdited = true));
    name.addEventListener("lds-input", (e) => {
      if (!keyEdited) key.setAttribute("value", suggestKey(e.detail.value));
    });
    modal.setAttribute("open", "");
  }

  onClick(e) {
    const target = e.target.closest?.("[data-action], [data-close], tr[data-key]");
    if (!target) return;
    if (target.hasAttribute("data-close")) target.closest("lds-modal").removeAttribute("open");
    else if (target.dataset.action === "new") this.openNew();
    else if (target.dataset.action === "create") this.create();
    else if (target.dataset.key && !e.target.closest("a")) this.context.router.navigate(`/w/${this.ws.slug}/settings/projects/${target.dataset.key}`);
  }

  async create() {
    const { api, i18n, router, projects } = this.context;
    const modal = this.shadowRoot.querySelector('[data-dialog="new"]');
    const form = modal.querySelector("form");
    clearFieldErrors(form);
    form.querySelector("lds-alert").textContent = "";
    let created;
    try {
      ({ data: created } = await busy(modal.querySelector('[data-action="create"]'), () =>
        api.post(`/w/${encodeURIComponent(this.ws.slug)}/projects`, { body: formValues(form) })));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else form.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      return;
    }
    modal.removeAttribute("open");
    await projects.refresh();
    router.navigate(`/w/${this.ws.slug}/p/${created.key}`);
  }
}

customElements.define("hzn-settings-projects", HznSettingsProjects);
