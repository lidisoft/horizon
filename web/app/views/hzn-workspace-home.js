// <hzn-workspace-home>: /w/:ws — My issues: the open issues assigned to
// the user in every project of the workspace, grouped by outcome (started
// first), in rank order (wireframes: Sidebar, Main).

import { issueHref, issueStyles, priorityGlyph, projectData, shortDate, statusGlyph } from "horizon/components/issue-ui.js";
import { paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const GROUPS = ["started", "unstarted"];

const styles = sheet(`
  :host { display: block; }
  header.bar { display: flex; align-items: center; block-size: 3.25rem; padding: 0 var(--lds-space-5);
    border-block-end: 1px solid var(--lds-color-border-default); font-weight: var(--lds-font-weight-semibold); }
  .group { display: flex; align-items: center; gap: var(--lds-space-2); block-size: 2.25rem; padding: 0 var(--lds-space-5);
    background: var(--lds-color-bg-subtle); border-block-end: 1px solid var(--lds-color-border-subtle);
    font-size: var(--lds-font-size-xs); font-weight: var(--lds-font-weight-semibold); }
  .group .count { color: var(--lds-color-fg-muted); font-weight: normal; }
  .row { display: grid; grid-template-columns: 1rem 4.5rem 1rem minmax(0, 1fr) auto 3.5rem; align-items: center;
    gap: var(--lds-space-3); block-size: 2.5rem; padding: 0 var(--lds-space-5); border-block-end: 1px solid var(--lds-color-border-subtle);
    color: inherit; text-decoration: none; }
  .row:hover, .row:focus-visible { background: var(--lds-color-bg-subtle); outline: none; }
  .row .key { font-size: var(--lds-font-size-xs); }
  .title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .project { display: inline-flex; align-items: center; gap: var(--lds-space-1); color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-xs); }
  .swatch { display: inline-block; inline-size: 0.5rem; block-size: 0.5rem; border-radius: var(--lds-radius-xs); }
  .due { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-xs); text-align: end; }
  lds-empty-state { margin-block-start: var(--lds-space-12); }
`);

class HznWorkspaceHome extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    this.attachShadow({ mode: "open" }).adoptedStyleSheets = [issueStyles, styles];
    this.load();
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before !== after) this.load();
  }

  async load() {
    const { api, i18n } = this.context;
    const { t } = i18n;
    const slug = this.getAttribute("ws");
    const enc = encodeURIComponent(slug);
    let items;
    let projects;
    try {
      const [res, list] = await Promise.all([
        api.post(`/w/${enc}/issues/query`, { body: {
          filter: { and: [{ field: "assignee", op: "eq", value: "@me" }, { field: "status.outcome", op: "eq", value: "open" }] },
          sort: [{ field: "rank", dir: "asc" }], limit: 200,
        } }),
        api.get(`/w/${enc}/projects`),
      ]);
      items = res.data.items;
      projects = new Map(list.data.items.map((p) => [p.id, p]));
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`
        <header class="bar">${t("nav.myIssues")}</header>
        <lds-empty-state>
          <lds-icon slot="icon" name="search"></lds-icon>
          <h2 slot="title">${err.status === 404 ? t("workspace.notFound") : errorMessage(err, i18n)}</h2>
          <a href="/">${t("notFound.home")}</a>
        </lds-empty-state>`);
      return;
    }
    // Each project's statuses, for the glyphs and the grouping.
    const keys = [...new Set(items.map((it) => projects.get(it.project_id)?.key).filter(Boolean))];
    const data = new Map(await Promise.all(keys.map(async (k) => [k, await projectData(this.context, slug, k).catch(() => null)])));
    const rows = items.map((it) => {
      const p = projects.get(it.project_id);
      const d = data.get(p?.key);
      return { it, p, d, outcome: d?.statuses.get(it.status_id)?.outcome ?? "unstarted" };
    });
    this.shadowRoot.innerHTML = String(html`
      <header class="bar">${t("nav.myIssues")}</header>
      ${rows.length ? GROUPS.map((g) => {
        const group = rows.filter((r) => r.outcome === g);
        return group.length ? html`
          <section aria-label="${t(`outcome.${g}`)}">
            <div class="group"><span class="glyph status" data-outcome="${g}"><lds-icon name="${g === "started" ? "circle-dot" : "circle"}" size="sm"></lds-icon></span>
              ${t(`outcome.${g}`)}<span class="count">${group.length}</span></div>
            ${group.map(({ it, p, d }) => html`
              <a class="row" href="${issueHref(slug, it.key)}">
                ${priorityGlyph(it.priority, t)}<span class="key">${it.key}</span>${d ? statusGlyph(d, it.status_id) : html`<span></span>`}
                <span class="title">${it.title}</span>
                <span class="project">${p ? html`<span class="swatch" data-colour="${p.colour}"></span>${p.name}` : ""}</span>
                <span class="due">${it.due_date ? shortDate(it.due_date, i18n) : ""}</span>
              </a>`)}
          </section>` : "";
      }) : html`
        <lds-empty-state>
          <lds-icon slot="icon" name="user"></lds-icon>
          <h2 slot="title">${t("myIssues.none")}</h2>
          ${t("myIssues.noneHint")}
        </lds-empty-state>`}`);
    paintSwatches(this.shadowRoot);
  }
}

customElements.define("hzn-workspace-home", HznWorkspaceHome);
