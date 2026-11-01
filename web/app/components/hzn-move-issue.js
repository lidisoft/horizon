// <hzn-move-issue>: the Move to another project dialog (wireframes:
// MoveIssue). show({ ws, issue, children }) resolves with the moved issue,
// or null when cancelled. The server's preview gives the automatic matches;
// types and statuses without one must be chosen, field values and labels
// without one are dropped.

import { projectData } from "horizon/components/issue-ui.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { busy, errorMessage } from "horizon/core/forms.js";

const styles = sheet(`
  .body { display: grid; gap: var(--lds-space-4); }
  lds-alert:empty { display: none; }
  p { margin: 0; }
  table { inline-size: 100%; border-collapse: collapse; font-size: var(--lds-font-size-sm); }
  th { text-align: start; font-weight: normal; color: var(--lds-color-fg-muted); padding: var(--lds-space-2); }
  td { padding: var(--lds-space-2); border-block-start: 1px solid var(--lds-color-border-subtle); vertical-align: middle; }
  tr[data-gap] td { background: var(--lds-color-warning-subtle); }
  .ok { display: inline-flex; gap: var(--lds-space-1); align-items: center; }
  .ok lds-icon { color: var(--lds-color-success-fg); }
  .count { color: var(--lds-color-fg-muted); }
  .actions { display: flex; justify-content: flex-end; align-items: center; gap: var(--lds-space-3); }
  .actions .hint { margin-inline-end: auto; color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-xs); }
`);

class HznMoveIssue extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [styles];
    root.innerHTML = String(html`<lds-modal size="lg"><span slot="header" data-title></span><div class="body" data-body></div>
      <div slot="footer" class="actions" data-footer></div></lds-modal>`);
    this.modal = root.querySelector("lds-modal");
    this.modal.addEventListener("lds-close", () => this.finish(null));
    root.addEventListener("lds-change", (e) => this.onChange(e));
    root.addEventListener("click", (e) => {
      const action = e.target.closest?.("[data-action]")?.dataset.action;
      if (action === "cancel") this.modal.removeAttribute("open");
      if (action === "move") this.submit(e.target.closest("[data-action]"));
    });
  }

  /** Opens the dialog for issue; resolves with the moved issue or null. */
  async show({ ws, issue, children }) {
    const { api, i18n } = this.context;
    this.ws = ws;
    this.issue = issue;
    this.childCount = children;
    this.choice = children ? "move" : "";
    this.mapping = { types: {}, statuses: {}, fields: {} };
    const source = issue.key.slice(0, issue.key.lastIndexOf("-"));
    try {
      const { data } = await api.get(`/w/${encodeURIComponent(ws)}/projects`);
      this.projects = data.items.filter((p) => !p.archived_at && p.key !== source);
      this.source = data.items.find((p) => p.key === source);
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      return null;
    }
    this.shadowRoot.querySelector("[data-title]").textContent = i18n.t("move.title", { key: issue.key });
    this.target = this.projects[0]?.key ?? "";
    this.modal.setAttribute("open", "");
    await this.preview();
    return new Promise((resolve) => (this.resolve = resolve));
  }

  finish(value) {
    this.resolve?.(value);
    this.resolve = null;
  }

  async preview() {
    const { api, i18n } = this.context;
    const body = this.shadowRoot.querySelector("[data-body]");
    if (!this.target) {
      body.innerHTML = String(html`<lds-alert intent="info">${i18n.t("move.noProject")}</lds-alert>`);
      this.renderFooter(false);
      return;
    }
    try {
      const [p, data] = await Promise.all([
        api.get(`/w/${encodeURIComponent(this.ws)}/issues/${encodeURIComponent(this.issue.key)}/move/preview`, { query: { project: this.target, children: this.choice || "move" } }),
        projectData(this.context, this.ws, this.target),
      ]);
      this.plan = p.data;
      this.targetData = data;
    } catch (err) {
      body.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      this.renderFooter(false);
      return;
    }
    this.render();
  }

  render() {
    const { t } = this.context.i18n;
    const plan = this.plan;
    const target = this.projects.find((p) => p.key === this.target);
    const d = this.targetData;
    const statuses = [...d.statuses.values()];
    const row = (kind, m, control) => html`
      <tr ${!m.match && kind !== "label" && kind !== "field" ? html`data-gap` : ""}>
        <td>${t(`move.${kind}`)}</td><td>${m.name} <span class="count">${t("move.issues", { count: m.issues })}</span></td>
        <td><lds-icon name="chevron-right" size="sm"></lds-icon></td><td>${control}</td>
      </tr>`;
    const matched = (id, name) => html`<span class="ok"><lds-icon name="check" size="sm"></lds-icon>${name ?? id}</span>`;
    const dropped = plan.fields.filter((f) => !f.match && !this.mapping.fields[f.id]).length + plan.labels.filter((l) => !l.match).length;
    this.shadowRoot.querySelector("[data-body]").innerHTML = String(html`
      <lds-field label="${t("move.to")}">
        <lds-select data-target value="${this.target}">${this.projects.map((p) => html`<option value="${p.key}">${p.name}</option>`)}</lds-select>
      </lds-field>
      <p>${t("move.keeps", { key: this.issue.key, project: target?.name ?? "" })}</p>
      ${this.childCount ? html`
        <lds-field label="${t("move.subIssues")}">
          <lds-segmented data-children value="${this.choice}" label="${t("move.subIssues")}">
            <lds-segment value="move">${t("move.moveAll", { count: this.childCount })}</lds-segment>
            <lds-segment value="leave">${t("move.leave", { project: this.source?.name ?? "" })}</lds-segment>
          </lds-segmented>
        </lds-field>` : ""}
      <table>
        <thead><tr><th>${t("move.property")}</th><th>${t("move.inSource", { project: this.source?.name ?? "" })}</th><th></th><th>${t("move.inTarget", { project: target?.name ?? "" })}</th></tr></thead>
        <tbody>
          ${plan.types.map((m) => row("type", m, m.match ? matched(m.match, d.types.get(m.match)?.name) : html`
            <lds-select size="sm" data-map="types" data-id="${m.id}" value="${this.mapping.types[m.id] ?? ""}">
              <option value="">${t("move.choose")}</option>${d.config.issue_types.map((x) => html`<option value="${x.id}">${x.name}</option>`)}
            </lds-select>`))}
          ${plan.statuses.map((m) => row("status", m, m.match ? matched(m.match, d.statuses.get(m.match)?.name) : html`
            <lds-select size="sm" data-map="statuses" data-id="${m.id}" value="${this.mapping.statuses[m.id] ?? ""}">
              <option value="">${t("move.choose")}</option>${statuses.map((s) => html`<option value="${s.id}">${d.workflows.get(s.workflow)?.name}: ${s.name}</option>`)}
            </lds-select>`))}
          ${plan.fields.map((m) => row("field", m, m.match ? matched(m.match, d.fields.get(m.match)?.name) : html`
            <lds-select size="sm" data-map="fields" data-id="${m.id}" value="${this.mapping.fields[m.id] ?? ""}">
              <option value="">${t("move.dropValue")}</option>${[...d.fields.values()].map((f) => html`<option value="${f.id}">${f.name}</option>`)}
            </lds-select>`))}
          ${plan.labels.map((m) => row("label", m, m.match ? matched(m.match, d.labels.get(m.match)?.name) : html`<span class="count">${t("move.dropLabel")}</span>`))}
          <tr><td>${t("move.planning")}</td><td colspan="2"></td><td class="count">${t("move.cleared")}</td></tr>
        </tbody>
      </table>
      ${dropped || plan.estimates_cleared ? html`
        <lds-alert intent="warning">${t("move.dropped", { count: dropped })}${plan.estimates_cleared ? ` ${t("move.estimates", { count: plan.estimates_cleared })}` : ""}</lds-alert>` : ""}
      <lds-alert intent="danger" role="alert" data-error></lds-alert>`);
    this.renderFooter(true);
  }

  renderFooter(ready) {
    const { t } = this.context.i18n;
    const target = this.projects?.find((p) => p.key === this.target);
    const count = this.plan && this.choice !== "leave" ? this.plan.issues : 1;
    this.shadowRoot.querySelector("[data-footer]").innerHTML = String(html`
      <span class="hint">${t("move.permissions", { from: this.source?.name ?? "", to: target?.name ?? "" })}</span>
      <lds-button variant="ghost" data-action="cancel">${t("tokens.cancel")}</lds-button>
      <lds-button variant="primary" data-action="move" ${ready ? "" : html`disabled`}>${t("move.submit", { count, project: target?.name ?? "" })}</lds-button>`);
  }

  onChange(e) {
    const el = e.target;
    if (el.hasAttribute("data-target")) {
      this.target = e.detail.value;
      this.mapping = { types: {}, statuses: {}, fields: {} };
      this.preview();
    } else if (el.hasAttribute("data-children")) {
      this.choice = e.detail.value;
      this.preview();
    } else if (el.dataset.map) {
      const m = this.mapping[el.dataset.map];
      if (e.detail.value) m[el.dataset.id] = e.detail.value;
      else delete m[el.dataset.id];
    }
  }

  async submit(button) {
    const { api, i18n } = this.context;
    const fields = {};
    for (const f of this.plan.fields) {
      if (!f.match) fields[f.id] = this.mapping.fields[f.id] ?? null;
    }
    const body = { project: this.target, mapping: { types: this.mapping.types, statuses: this.mapping.statuses, fields } };
    if (this.choice) body.children = this.choice;
    let moved;
    try {
      ({ data: moved } = await busy(button, () => api.post(`/w/${encodeURIComponent(this.ws)}/issues/${encodeURIComponent(this.issue.key)}/move`, { body })));
    } catch (err) {
      const box = this.shadowRoot.querySelector("[data-error]");
      box.textContent = err.code === "mapping_required" ? i18n.t("move.mapAll") : errorMessage(err, i18n);
      return;
    }
    const resolve = this.resolve;
    this.resolve = null;
    this.modal.removeAttribute("open");
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("move.done", { from: this.issue.key, to: moved.key }) });
    resolve?.(moved);
  }
}

customElements.define("hzn-move-issue", HznMoveIssue);
