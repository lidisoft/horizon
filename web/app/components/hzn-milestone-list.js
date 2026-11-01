// <hzn-milestone-list ws [project] [manage]>: milestones by date with their
// progress and linked issues: the workspace's and those of visible
// projects, or, with project (a key), the workspace's and that project's.
// People with milestone.manage create, edit, link issues to and delete
// them; manage says the caller holds it on the project.

import "horizon/components/hzn-milestone-dialog.js";
import { issueHref } from "horizon/components/issue-ui.js";
import { localDate, planningStyles, progress } from "horizon/components/planning-ui.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { busy, errorMessage } from "horizon/core/forms.js";

const styles = sheet(`
  :host { display: block; }
  .head { display: flex; align-items: center; justify-content: space-between; gap: var(--lds-space-3); margin-block-end: var(--lds-space-3); }
  h2 { margin: 0; font-size: var(--lds-font-size-md); }
  ol { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--lds-space-2); }
  li { border: 1px solid var(--lds-color-border-default); border-radius: var(--lds-radius-md); background: var(--lds-color-bg-surface); }
  .item { display: grid; grid-template-columns: 6rem minmax(0, 1fr) auto auto auto; align-items: center; gap: var(--lds-space-3);
    padding: var(--lds-space-2) var(--lds-space-3); }
  .date { font-variant-numeric: tabular-nums; color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  .date.past { text-decoration: line-through; }
  .name { font-weight: var(--lds-font-weight-medium); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .desc { grid-column: 2 / -1; margin: 0; color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  .issues { padding: 0 var(--lds-space-3) var(--lds-space-3) calc(6rem + var(--lds-space-6)); display: flex; flex-wrap: wrap; gap: var(--lds-space-2); }
  .issues a { color: var(--lds-color-fg-link); font-size: var(--lds-font-size-sm); }
  .toggle { border: 0; background: none; padding: 0; color: var(--lds-color-fg-link); cursor: pointer; font: inherit; font-size: var(--lds-font-size-sm); }
  .none { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  .muted { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  .confirm { display: flex; justify-content: flex-end; gap: var(--lds-space-3); }
  lds-alert:empty { display: none; }
`);

class HznMilestoneList extends HTMLElement {
  static observedAttributes = ["ws", "project"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    this.expanded = new Map(); // milestone id → linked keys
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [planningStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    // The dialogs stay put: pushes re-render the list under them.
    const { t } = this.context.i18n;
    root.innerHTML = String(html`
      <div data-body></div>
      <lds-modal size="sm" data-confirm>
        <span slot="header" data-confirm-title></span>
        <p>${t("milestones.deleteBody")}</p>
        <div slot="footer" class="confirm">
          <lds-button variant="ghost" data-action="cancel">${t("tokens.cancel")}</lds-button>
          <lds-button variant="danger" data-action="confirm-delete">${t("milestones.delete")}</lds-button>
        </div>
      </lds-modal>
      <hzn-milestone-dialog></hzn-milestone-dialog>`);
    this.onChanged = () => this.load();
    document.addEventListener("hzn-milestone-changed", this.onChanged);
    this.offSocket = this.context.socket.on((m) => {
      if (m.t === "event" && m.d?.aggregate?.type === "milestone") {
        clearTimeout(this.timer);
        this.timer = setTimeout(() => this.load(), 150);
      }
    });
    this.load();
  }

  disconnectedCallback() {
    document.removeEventListener("hzn-milestone-changed", this.onChanged);
    this.offSocket?.();
    clearTimeout(this.timer);
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before !== after && before !== null) this.load();
  }

  get ws() {
    return this.getAttribute("ws");
  }

  async load() {
    const { api, i18n } = this.context;
    const ws = encodeURIComponent(this.ws);
    const project = this.getAttribute("project");
    try {
      const [list, projects] = await Promise.all([
        api.get(project ? `/w/${ws}/projects/${encodeURIComponent(project)}/milestones` : `/w/${ws}/milestones`),
        api.get(`/w/${ws}/projects`),
      ]);
      this.list = list.data;
      this.projects = projects.data.items.filter((p) => !p.archived_at);
      // Linked keys of the expanded ones, again.
      await Promise.all([...this.expanded.keys()].map((id) => this.fetchKeys(id)));
    } catch (err) {
      this.shadowRoot.querySelector("[data-body]").innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    this.render();
  }

  async fetchKeys(id) {
    try {
      const { data } = await this.context.api.get(`/w/${encodeURIComponent(this.ws)}/milestones/${id}/issues`);
      this.expanded.set(id, data.items);
    } catch {
      this.expanded.delete(id);
    }
  }

  /** Whether the caller may create a milestone here: on the workspace, or (manage) on the project. */
  get canCreate() {
    return this.list.can_create || this.hasAttribute("manage");
  }

  render() {
    const { i18n } = this.context;
    const { t } = i18n;
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const projectName = (id) => this.projects.find((p) => p.id === id)?.name;
    this.shadowRoot.querySelector("[data-body]").innerHTML = String(html`
      <div class="head">
        <h2>${t("milestones.title")}</h2>
        ${this.canCreate ? html`<lds-button size="sm" variant="secondary" data-action="new"><lds-icon slot="start" name="flag" size="sm"></lds-icon>${t("milestones.new")}</lds-button>` : ""}
      </div>
      ${this.list.items.length ? html`<ol>${this.list.items.map((m) => html`
        <li data-milestone="${m.id}">
          <div class="item">
            <span class="date ${localDate(m.date) < today ? "past" : ""}">${i18n.date(localDate(m.date), { dateStyle: "medium" })}</span>
            <span class="name">${m.name}</span>
            <lds-badge size="sm" intent="${m.project_id ? "neutral" : "info"}">${m.project_id ? projectName(m.project_id) ?? "" : t("milestones.workspace")}</lds-badge>
            ${progress(m.progress, i18n)}
            ${m.can_manage ? html`
              <lds-menu placement="bottom-end">
                <lds-button slot="trigger" variant="ghost" size="sm" label="${t("milestones.more", { name: m.name })}"><lds-icon name="more-horizontal"></lds-icon></lds-button>
                <lds-menu-item data-action="edit" data-id="${m.id}"><lds-icon slot="start" name="pencil"></lds-icon>${t("milestones.edit")}</lds-menu-item>
                <lds-menu-item data-action="link" data-id="${m.id}"><lds-icon slot="start" name="link"></lds-icon>${t("milestones.link")}</lds-menu-item>
                <hr><lds-menu-item data-action="delete" data-id="${m.id}"><lds-icon slot="start" name="trash-2"></lds-icon>${t("milestones.delete")}</lds-menu-item>
              </lds-menu>` : html`<span></span>`}
            ${m.description ? html`<p class="desc">${m.description}</p>` : ""}
          </div>
          <div class="issues">
            <button type="button" class="toggle" data-action="toggle" data-id="${m.id}" aria-expanded="${String(this.expanded.has(m.id))}">
              ${t(this.expanded.has(m.id) ? "milestones.hideIssues" : "milestones.showIssues", { count: m.progress.issues })}</button>
            ${this.expanded.has(m.id) ? this.expanded.get(m.id).map((k) => html`<a href="${issueHref(this.ws, k)}">${k}</a>`) : ""}
          </div>
        </li>`)}</ol>` : html`<p class="none">${t("milestones.empty")}</p>`}`);
  }

  async onClick(e) {
    const el = e.target.closest?.("[data-action]");
    if (!el) return;
    const m = el.dataset.id && this.list.items.find((x) => x.id === el.dataset.id);
    const dialog = this.shadowRoot.querySelector("hzn-milestone-dialog");
    switch (el.dataset.action) {
      case "new":
        return dialog.create({ ws: this.ws, projects: this.projects, project: this.getAttribute("project")?.toUpperCase() ?? "" });
      case "edit":
        return dialog.edit({ ws: this.ws, milestone: m });
      case "link":
        await this.fetchKeys(m.id);
        return dialog.link({ ws: this.ws, milestone: m, keys: this.expanded.get(m.id) ?? [] });
      case "toggle":
        if (this.expanded.has(m.id)) this.expanded.delete(m.id);
        else await this.fetchKeys(m.id);
        return this.render();
      case "delete": {
        this.deleting = m;
        const modal = this.shadowRoot.querySelector("[data-confirm]");
        modal.querySelector("[data-confirm-title]").textContent = this.context.i18n.t("milestones.deleteTitle", { name: m.name });
        return modal.setAttribute("open", "");
      }
      case "cancel":
        return this.shadowRoot.querySelector("[data-confirm]").removeAttribute("open");
      case "confirm-delete":
        return this.delete(el);
    }
  }

  async delete(button) {
    const { api, i18n } = this.context;
    try {
      await busy(button, () => api.delete(`/w/${encodeURIComponent(this.ws)}/milestones/${this.deleting.id}`));
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      return;
    }
    this.shadowRoot.querySelector("[data-confirm]").removeAttribute("open");
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("milestones.deleted", { name: this.deleting.name }) });
    this.expanded.delete(this.deleting.id);
    this.load();
  }
}

customElements.define("hzn-milestone-list", HznMilestoneList);
