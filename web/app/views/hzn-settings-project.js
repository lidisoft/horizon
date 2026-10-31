// <hzn-settings-project>: /w/:ws/settings/projects/:key — one project's
// name, colour, key, description, lead and visibility, and archiving
// (wireframes: General, project scope). Read-only without project.manage.

import { COLOURS, paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, formValues, showFieldErrors, submitOnEnter } from "horizon/core/forms.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

const styles = sheet(`
  form { display: grid; gap: var(--lds-space-4); max-width: 44rem; }
  h3 { margin: 0; font-size: var(--lds-font-size-md); }
  .row { display: grid; grid-template-columns: 1fr 10rem; gap: var(--lds-space-4); align-items: end; }
  .danger { max-width: 44rem; margin-top: var(--lds-space-8); padding: var(--lds-space-4) var(--lds-space-5);
    border: 1px solid var(--lds-color-border-default); border-radius: var(--lds-radius-md); }
  .danger h3 { margin-bottom: var(--lds-space-2); }
  .danger p { margin: 0 0 var(--lds-space-3); }
  .dot { display: inline-block; width: 0.625rem; height: 0.625rem; border-radius: var(--lds-radius-xs); }
`);

class HznSettingsProject extends HTMLElement {
  static observedAttributes = ["ws", "key"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    this.attachShadow({ mode: "open" }).adoptedStyleSheets = [settingsStyles, styles];
    this.load();
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.load();
  }

  async load() {
    const { api, i18n, router } = this.context;
    const ws = encodeURIComponent(this.getAttribute("ws"));
    const key = this.getAttribute("key");
    try {
      const [w, p, members] = await Promise.all([
        api.get(`/w/${ws}`), api.get(`/w/${ws}/projects/${encodeURIComponent(key)}`), api.get(`/w/${ws}/members`).catch(() => null),
      ]);
      this.ws = w.data;
      this.project = p.data;
      this.etag = p.etag;
      this.members = members?.data.items ?? [];
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    if (this.project.key !== key.toUpperCase()) {
      router.navigate(`/w/${this.ws.slug}/settings/projects/${this.project.key}`, { replace: true }); // a former key
      return;
    }
    this.render();
  }

  render() {
    const { t } = this.context.i18n;
    const p = this.project;
    const canEdit = p.can.includes("project.manage");
    const ro = canEdit ? "" : html`readonly`;
    const off = canEdit ? "" : html`disabled`;
    const body = html`
      <form novalidate>
        <h3>${t("projects.project")}</h3>
        <lds-alert intent="danger" role="alert"></lds-alert>
        ${canEdit ? "" : html`<p class="muted">${t("projects.readOnly")}</p>`}
        ${p.archived_at ? html`<lds-alert intent="warning">${t("projects.isArchived")}</lds-alert>` : ""}
        <div class="row">
          <lds-field label="${t("projects.name")}" required><lds-input name="name" required maxlength="80" value="${p.name}" ${ro}></lds-input></lds-field>
          <lds-field label="${t("projects.colour")}">
            <lds-select name="colour" value="${p.colour}" ${off}>${COLOURS.map((c) => html`<option value="${c}">${t(`colour.${c}`)}</option>`)}</lds-select>
          </lds-field>
        </div>
        <lds-field label="${t("projects.key")}" required>
          <lds-input name="key" required maxlength="10" value="${p.key}" ${ro}></lds-input>
          <span slot="help">${t("projects.keyHelp")}</span>
        </lds-field>
        <lds-alert intent="info" data-key-note hidden></lds-alert>
        ${p.aliases.length ? html`<p class="muted">${t("projects.aliases", { keys: p.aliases.join(", ") })}</p>` : ""}
        <lds-field label="${t("projects.description")}">
          <lds-textarea name="description" rows="3" maxlength="2000" value="${p.description}" ${ro}></lds-textarea>
        </lds-field>
        <lds-field label="${t("projects.lead")}">
          <lds-combobox name="lead_id" value="${p.lead_id ?? ""}" placeholder="${t("projects.noLead")}" ${off}>
            <lds-option value="">${t("projects.noLead")}</lds-option>
            ${this.members.filter((m) => m.status === "active").map((m) => html`
              <lds-option value="${m.user_id}" keywords="${m.email}"><lds-avatar slot="start" size="sm" name="${m.name}"></lds-avatar>${m.name}</lds-option>`)}
          </lds-combobox>
        </lds-field>
        <lds-field label="${t("projects.visibility")}">
          <lds-radio-group name="visibility" value="${p.visibility}" ${off}>
            <button type="button" value="workspace">${t("projects.visibility.workspaceLong")}</button>
            <button type="button" value="private">${t("projects.visibility.privateLong", { name: p.name })}</button>
          </lds-radio-group>
        </lds-field>
      </form>
      ${canEdit ? html`
        <section class="danger" aria-labelledby="archive-title">
          <h3 id="archive-title">${t(p.archived_at ? "projects.restoreTitle" : "projects.archiveTitle")}</h3>
          <p class="muted">${t(p.archived_at ? "projects.restoreBody" : "projects.archiveBody", { name: p.name })}</p>
          <lds-button variant="secondary" data-action="${p.archived_at ? "unarchive" : "archive"}">${t(p.archived_at ? "projects.restore" : "projects.archive")}</lds-button>
        </section>` : ""}`;
    const actions = canEdit ? html`<span class="muted" role="status" data-saved></span><lds-button data-action="save">${t("account.save")}</lds-button>` : "";
    this.shadowRoot.innerHTML = String(settingsPage(t, this.ws, "projects", body, actions,
      html`<span class="dot" data-colour="${p.colour}"></span> ${p.name}`));
    paintSwatches(this.shadowRoot);
    if (!canEdit) return;

    const form = this.shadowRoot.querySelector("form");
    submitOnEnter(form);
    form.addEventListener("submit", (e) => {
      e.preventDefault();
      this.save(form);
    });
    form.querySelector('[name="key"]').addEventListener("lds-input", (e) => {
      const note = this.shadowRoot.querySelector("[data-key-note]");
      const next = e.detail.value.trim().toUpperCase();
      note.hidden = !next || next === p.key;
      note.textContent = t("projects.keyChange", { old: p.key, key: next });
    });
    this.shadowRoot.querySelector('[data-action="save"]').addEventListener("click", () => form.requestSubmit());
    this.shadowRoot.querySelector("[data-action=archive], [data-action=unarchive]").addEventListener("click", (e) => this.archive(e.currentTarget));
  }

  async save(form) {
    const { api, i18n, router, projects } = this.context;
    const alert = form.querySelector("lds-alert");
    alert.textContent = "";
    clearFieldErrors(form);
    const body = formValues(form);
    body.lead_id = body.lead_id || "00000000-0000-0000-0000-000000000000";
    let updated;
    try {
      ({ data: updated } = await busy(this.shadowRoot.querySelector('[data-action="save"]'), () =>
        api.patch(`/w/${encodeURIComponent(this.ws.slug)}/projects/${this.project.key}`, { body, ifMatch: this.etag })));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else alert.textContent = errorMessage(err, i18n);
      return;
    }
    await projects.refresh();
    if (updated.key !== this.project.key) {
      router.navigate(`/w/${this.ws.slug}/settings/projects/${updated.key}`, { replace: true });
      return;
    }
    await this.load();
    this.shadowRoot.querySelector("[data-saved]").textContent = i18n.t("account.saved");
  }

  async archive(button) {
    const { api, i18n, projects } = this.context;
    try {
      await busy(button, () => api.post(`/w/${encodeURIComponent(this.ws.slug)}/projects/${this.project.key}/${button.dataset.action}`));
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      return;
    }
    await projects.refresh();
    this.load();
  }
}

customElements.define("hzn-settings-project", HznSettingsProject);
