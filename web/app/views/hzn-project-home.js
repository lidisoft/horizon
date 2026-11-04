// <hzn-project-home>: /w/:ws/p/:key — the project overview
// (docs/reports.md#project-overview-page): the active sprint with its
// burndown, the next release and milestone, flow health, recent activity
// and the viewer's open issues, from GET /projects/{key}/overview. Pushed
// events refresh it; a former key resolves to the current one.

import "horizon/components/charts.js";
import { changesProjectData, describeEvent, forgetProjects, issueHref, issueStyles, projectData, statusGlyph } from "horizon/components/issue-ui.js";
import { planningStyles, viewSwitch } from "horizon/components/planning-ui.js";
import { paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const OUTCOMES = ["completed", "started", "unstarted", "abandoned"];

const styles = sheet(`
  :host { display: flex; flex-direction: column; min-block-size: 100%; }
  header.bar { display: flex; align-items: center; justify-content: space-between; gap: var(--lds-space-3);
    min-block-size: 3.25rem; padding: 0 var(--lds-space-5); border-block-end: 1px solid var(--lds-color-border-default); }
  .crumb { display: flex; align-items: center; gap: var(--lds-space-2); }
  .crumb strong { font-weight: var(--lds-font-weight-semibold); }
  .swatch { display: inline-block; inline-size: 0.625rem; block-size: 0.625rem; border-radius: var(--lds-radius-xs); }
  .toolbar { display: flex; align-items: center; gap: var(--lds-space-2); padding: var(--lds-space-2) var(--lds-space-5);
    border-block-end: 1px solid var(--lds-color-border-subtle); }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(22rem, 1fr)); gap: var(--lds-space-4); padding: var(--lds-space-5); }
  lds-card h2 { margin: 0 0 var(--lds-space-3); font-size: var(--lds-font-size-md); display: flex; align-items: baseline; justify-content: space-between; gap: var(--lds-space-2); }
  lds-card h2 a { font-size: var(--lds-font-size-sm); font-weight: normal; color: var(--lds-color-fg-link); }
  .muted { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  .row { display: flex; align-items: center; justify-content: space-between; gap: var(--lds-space-3); margin-block: var(--lds-space-2); }
  .big { font-size: var(--lds-font-size-xl); font-weight: var(--lds-font-weight-semibold); font-variant-numeric: tabular-nums; }
  .outcomes { display: flex; block-size: 0.625rem; border-radius: var(--lds-radius-full); overflow: hidden; background: var(--lds-color-bg-subtle);
    gap: var(--lds-chart-segment-gap, 2px); margin-block: var(--lds-space-2) var(--lds-space-1); }
  .outcomes span { flex: var(--n) 1 0; }
  ${OUTCOMES.map((o) => `.outcomes .${o} { background: var(--lds-color-chart-${o}); }`).join("\n  ")}
  .legend { display: flex; flex-wrap: wrap; gap: var(--lds-space-1) var(--lds-space-3); font-size: var(--lds-font-size-xs); color: var(--lds-color-fg-muted); }
  .target + .target { margin-block-start: var(--lds-space-4); }
  ul { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--lds-space-2); font-size: var(--lds-font-size-sm); }
  li { display: flex; gap: var(--lds-space-2); align-items: baseline; }
  li .title { flex: 1; min-inline-size: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  li .when { margin-inline-start: auto; color: var(--lds-color-fg-muted); white-space: nowrap; }
  .wip li { justify-content: space-between; }
  .empty { padding: var(--lds-space-12) var(--lds-space-6); }
`);

class HznProjectHome extends HTMLElement {
  static observedAttributes = ["ws", "key"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    this.attachShadow({ mode: "open" }).adoptedStyleSheets = [issueStyles, planningStyles, styles];
    this.offSocket = this.context.socket.on((m) => this.onPush(m));
    this.load();
  }

  disconnectedCallback() {
    this.offSocket?.();
    clearTimeout(this.timer);
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.load();
  }

  get ws() {
    return this.getAttribute("ws");
  }

  async load() {
    const { api, i18n, router } = this.context;
    const { t } = i18n;
    const key = this.getAttribute("key");
    const seq = (this.seq = (this.seq ?? 0) + 1); // a later load wins over this one
    let overview;
    try {
      this.data = await projectData(this.context, this.ws, key);
      if (this.data.project.key !== key) {
        router.navigate(`/w/${this.ws}/p/${this.data.project.key}`, { replace: true }); // a former key, or another case
        return;
      }
      ({ data: overview } = await api.get(`/w/${encodeURIComponent(this.ws)}/projects/${encodeURIComponent(this.data.project.key)}/overview`));
    } catch (err) {
      if (seq !== this.seq) return;
      this.shadowRoot.innerHTML = String(html`
        <lds-empty-state class="empty"><lds-icon slot="icon" name="search"></lds-icon>
          <h2 slot="title">${err.status === 404 ? t("projects.notFound") : errorMessage(err, i18n)}</h2>
          <a href="/w/${this.ws}">${t("notFound.home")}</a>
        </lds-empty-state>`);
      return;
    }
    if (seq !== this.seq) return;
    this.render(overview);
  }

  /** A bar of outcomes and its legend (the legend says the numbers). */
  outcomes(by) {
    const { t } = this.context.i18n;
    const total = OUTCOMES.reduce((n, o) => n + (by[o] ?? 0), 0);
    const text = OUTCOMES.filter((o) => by[o]).map((o) => `${by[o]} ${t(`outcome.${o}`).toLowerCase()}`).join(" · ");
    return html`
      <div class="outcomes" role="img" aria-label="${text || t("overview.noIssues")}">
        ${OUTCOMES.filter((o) => by[o]).map((o) => html`<span class="${o}" data-n="${by[o]}"></span>`)}
      </div>
      <div class="legend">${total ? text : t("overview.noIssues")}</div>`;
  }

  render(o) {
    const { i18n } = this.context;
    const { t } = i18n;
    const p = this.data.project;
    const base = `/w/${encodeURIComponent(this.ws)}/p/${encodeURIComponent(p.key)}`;
    const sp = o.sprint;
    const thru = o.flow.throughput.reduce((n, v) => n + (v ?? 0), 0);
    const days = (v) => (v == null ? "–" : t("reports.days", { count: v }));
    this.shadowRoot.innerHTML = String(html`
      <header class="bar"><div class="crumb"><span class="swatch" data-colour="${p.colour}"></span><strong>${p.name}</strong></div></header>
      <div class="toolbar">${viewSwitch(t, this.ws, p.key, "")}</div>
      <div class="grid">
        <lds-card data-block="sprint">
          <h2>${t("overview.sprint")}${sp ? html`<a href="${base}/reports?tab=sprint&iteration=${sp.iteration.id}">${t("overview.reports")}</a>` : ""}</h2>
          ${sp ? html`
            <div class="row"><strong>${sp.iteration.name}</strong><span class="muted">${t("board.daysLeft", { count: sp.days_left })}</span></div>
            ${sp.goal ? html`<p class="muted">${sp.goal}</p>` : ""}
            <hzn-sparkline data-spark="burndown" width="320" height="48" label="${t("reports.burndown")}"></hzn-sparkline>
            ${this.outcomes(sp.by_outcome)}`
            : html`<p class="muted">${t("overview.noSprint")}</p>`}
        </lds-card>
        <lds-card data-block="targets">
          <h2>${t("overview.targets")}<a href="${base}/releases">${t("planning.layouts.releases")}</a></h2>
          ${[o.next_release && { ...o.next_release, icon: "rocket", param: "release" }, o.next_milestone && { ...o.next_milestone, icon: "milestone", param: "milestone" }].filter(Boolean).map((x) => html`
            <div class="target">
              <div class="row"><a href="${base}/reports?tab=releases&${x.param}=${x.id}"><lds-icon name="${x.icon}" size="sm"></lds-icon> ${x.name}</a>
                <span class="muted">${x.date ?? ""}</span></div>
              ${this.outcomes(x.by_outcome)}
            </div>`)}
          ${!o.next_release && !o.next_milestone ? html`<p class="muted">${t("overview.noTargets")}</p>` : ""}
        </lds-card>
        <lds-card data-block="flow">
          <h2>${t("overview.flow")}<a href="${base}/reports?tab=flow">${t("overview.reports")}</a></h2>
          <div class="row"><span><span class="big">${thru}</span> <span class="muted">${t("overview.completedWeeks")}</span></span>
            <hzn-sparkline data-spark="throughput" width="120" height="32" label="${t("reports.throughput")}"></hzn-sparkline></div>
          <p class="muted">${t("overview.cycle", { p50: days(o.flow.cycle.p50), p85: days(o.flow.cycle.p85) })}</p>
          <ul class="wip" aria-label="${t("overview.wip")}">${o.flow.wip.length
            ? o.flow.wip.map((w) => html`<li><span>${w.name}</span><strong>${w.count}</strong></li>`)
            : html`<li class="muted">${t("overview.noWip")}</li>`}</ul>
        </lds-card>
        <lds-card data-block="mine">
          <h2>${t("overview.myWork")}</h2>
          <ul>${o.my_work.length ? o.my_work.map((i) => html`<li>${statusGlyph(this.data, i.status_id)}<a class="key" href="${issueHref(this.ws, i.key)}">${i.key}</a>
            <span class="title">${i.title}</span></li>`) : html`<li class="muted">${t("overview.noWork")}</li>`}</ul>
        </lds-card>
        <lds-card data-block="activity">
          <h2>${t("overview.activity")}</h2>
          <ul>${o.activity.length ? o.activity.map((e) => html`<li><a class="key" href="${issueHref(this.ws, e.aggregate.key)}">${e.aggregate.key}</a>
            <span class="title"><strong>${e.actor ? e.actor.name || t("issue.someone") : t("issue.system")}</strong> ${describeEvent(i18n, e)}</span>
            <span class="when">${i18n.relative(e.occurred_at)}</span></li>`) : html`<li class="muted">${t("overview.noActivity")}</li>`}</ul>
        </lds-card>
      </div>`);
    paintSwatches(this.shadowRoot);
    for (const seg of this.shadowRoot.querySelectorAll(".outcomes span")) seg.style.setProperty("--n", seg.dataset.n); // the CSP forbids style attributes
    const spark = (name, d) => {
      const el = this.shadowRoot.querySelector(`[data-spark="${name}"]`);
      if (el) el.data = d;
    };
    if (sp) spark("burndown", { values: sp.remaining, ref: sp.ideal, tone: "started" });
    spark("throughput", { values: o.flow.throughput, tone: "completed" });
  }

  /** Refreshes on events about the project's issues and plans. */
  onPush(m) {
    if (m.t !== "event" && m.t !== "resync") return;
    if (m.t === "event" && changesProjectData(m.d)) forgetProjects();
    const key = m.d?.aggregate?.key ?? "";
    const plans = ["iteration", "release", "milestone"].includes(m.d?.aggregate?.type);
    if (m.t === "resync" || plans || key.startsWith(`${this.data?.project.key}-`) || changesProjectData(m.d ?? {})) {
      clearTimeout(this.timer);
      this.timer = setTimeout(() => this.load(), 500); // the projections follow the event shortly
    }
  }
}

customElements.define("hzn-project-home", HznProjectHome);
