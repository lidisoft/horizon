// <hzn-reports>: /w/:ws/p/:key/reports — a project's reports
// (docs/reports.md): Sprint (burndown, burnup, sprint report, velocity),
// Flow (cumulative flow, cycle time, throughput), Releases (a release's or
// milestone's progress) and Workload (distribution, created vs resolved).
// Flow and workload take the view bar's filter. Count or estimate, the
// tab and what it's about are in the address, so a report's URL is its
// shareable link.

import "horizon/components/charts.js";
import "horizon/components/hzn-view-bar.js";
import { projectData, issueHref, issueStyles } from "horizon/components/issue-ui.js";
import { loadIterations, loadReleases, planningStyles, viewSwitch } from "horizon/components/planning-ui.js";
import { paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const TABS = ["sprint", "flow", "releases", "workload"];
const GROUPS = ["assignee", "type", "priority", "label"];
const TONES = { completed: "completed", started: "started", unstarted: "unstarted", abandoned: "abandoned" };

const styles = sheet(`
  :host { display: flex; flex-direction: column; min-block-size: 100%; }
  header.bar { display: flex; align-items: center; justify-content: space-between; gap: var(--lds-space-3);
    min-block-size: 3.25rem; padding: 0 var(--lds-space-5); border-block-end: 1px solid var(--lds-color-border-default); }
  .crumb { display: flex; align-items: center; gap: var(--lds-space-2); }
  .crumb strong { font-weight: var(--lds-font-weight-semibold); }
  .crumb lds-icon { color: var(--lds-color-fg-subtle); }
  .swatch { display: inline-block; inline-size: 0.625rem; block-size: 0.625rem; border-radius: var(--lds-radius-xs); }
  .toolbar { display: flex; align-items: center; gap: var(--lds-space-2); padding: var(--lds-space-2) var(--lds-space-5);
    border-block-end: 1px solid var(--lds-color-border-subtle); flex-wrap: wrap; }
  .controls { display: flex; align-items: center; gap: var(--lds-space-3); padding: var(--lds-space-3) var(--lds-space-5); flex-wrap: wrap; }
  .controls lds-select { min-inline-size: 12rem; }
  .grow { flex: 1; }
  .barwrap { display: contents; }
  .barwrap[hidden] { display: none; }
  .panels { display: grid; grid-template-columns: repeat(auto-fit, minmax(28rem, 1fr)); gap: var(--lds-space-4); padding: 0 var(--lds-space-5) var(--lds-space-5); }
  lds-card { display: block; }
  lds-card h2 { margin: 0 0 var(--lds-space-1); font-size: var(--lds-font-size-md); }
  lds-card p.note { margin: 0 0 var(--lds-space-3); color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  .wide { grid-column: 1 / -1; }
  table.items { inline-size: 100%; border-collapse: collapse; font-size: var(--lds-font-size-sm); }
  table.items th, table.items td { padding: var(--lds-space-1) var(--lds-space-2); border-block-end: 1px solid var(--lds-color-border-subtle); text-align: start; }
  table.items td.num, table.items th.num { text-align: end; font-variant-numeric: tabular-nums; }
  .totals { display: flex; flex-wrap: wrap; gap: var(--lds-space-2) var(--lds-space-5); margin-block-end: var(--lds-space-3); font-size: var(--lds-font-size-sm); }
  .totals strong { font-variant-numeric: tabular-nums; }
  .empty { padding: var(--lds-space-12) var(--lds-space-6); }
`);

class HznReports extends HTMLElement {
  static observedAttributes = ["ws", "key"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [issueStyles, planningStyles, styles];
    root.addEventListener("lds-change", (e) => this.onChange(e));
    root.addEventListener("hzn-view-change", () => this.run());
    this.load();
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.load();
  }

  get ws() {
    return this.getAttribute("ws");
  }

  get base() {
    return `/w/${encodeURIComponent(this.ws)}`;
  }

  async load() {
    const { api, i18n, router } = this.context;
    const key = this.getAttribute("key");
    try {
      this.data = await projectData(this.context, this.ws, key);
      const [iterations, releases, milestones] = await Promise.all([
        loadIterations(this.context, this.ws, key).then((r) => r.items),
        loadReleases(this.context, this.ws, key).then((r) => r.items),
        api.get(`${this.base}/projects/${encodeURIComponent(key)}/milestones`).then((r) => r.data.items),
      ]);
      Object.assign(this, { iterations, releases, milestones });
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`
        <lds-empty-state class="empty"><lds-icon slot="icon" name="search"></lds-icon>
          <h2 slot="title">${err.status === 404 ? i18n.t("projects.notFound") : errorMessage(err, i18n)}</h2>
        </lds-empty-state>`);
      return;
    }
    const q = router.current?.query;
    const sprints = this.iterations.filter((it) => it.state === "active" || it.state === "completed");
    this.state = {
      tab: TABS.includes(q?.get("tab")) ? q.get("tab") : "sprint",
      metric: q?.get("metric") === "estimate" ? "estimate" : "count",
      iteration: q?.get("iteration") ?? (sprints.find((it) => it.state === "active") ?? sprints[sprints.length - 1])?.id ?? "",
      release: q?.get("release") ?? (this.releases.find((r) => r.state === "unreleased") ? `r:${this.releases.find((r) => r.state === "unreleased").id}` : ""),
      group: GROUPS.includes(q?.get("group")) ? q.get("group") : "assignee",
      period: q?.get("period") === "month" ? "month" : "week",
    };
    if (q?.get("milestone")) this.state.release = `m:${q.get("milestone")}`;
    else if (q?.get("release")) this.state.release = `r:${q.get("release")}`;
    this.renderFrame();
    this.bar = this.shadowRoot.querySelector("hzn-view-bar");
    await this.bar.setup({ ws: this.ws, data: this.data, layout: "list" });
    this.renderControls();
    this.run();
  }

  renderFrame() {
    const { t } = this.context.i18n;
    const p = this.data.project;
    this.shadowRoot.innerHTML = String(html`
      <header class="bar">
        <div class="crumb">
          <span class="swatch" data-colour="${p.colour}"></span><strong>${p.name}</strong>
          <lds-icon name="chevron-right" size="sm"></lds-icon><span>${t("planning.layouts.reports")}</span>
        </div>
      </header>
      <div class="toolbar">${viewSwitch(t, this.ws, p.key, "reports")}<span class="barwrap" data-barwrap><hzn-view-bar></hzn-view-bar></span></div>
      <div class="controls" data-controls></div>
      <div data-notice></div>
      <div class="panels" data-panels><lds-skeleton></lds-skeleton></div>`);
    paintSwatches(this.shadowRoot);
  }

  renderControls() {
    const { t } = this.context.i18n;
    const s = this.state;
    const sprints = this.iterations.filter((it) => it.state === "active" || it.state === "completed");
    this.shadowRoot.querySelector("[data-barwrap]").hidden = s.tab !== "flow" && s.tab !== "workload";
    this.shadowRoot.querySelector("[data-controls]").innerHTML = String(html`
      <lds-segmented size="sm" data-tab value="${s.tab}" label="${t("reports.tab")}">
        ${TABS.map((x) => html`<lds-segment value="${x}">${t(`reports.tabs.${x}`)}</lds-segment>`)}
      </lds-segmented>
      ${s.tab === "sprint" ? html`<lds-select size="sm" data-iteration value="${s.iteration}" label="${t("reports.iteration")}">
        ${sprints.length ? sprints.map((it) => html`<option value="${it.id}">${it.name}</option>`) : html`<option value="">${t("reports.noIterations")}</option>`}</lds-select>` : ""}
      ${s.tab === "releases" ? html`<lds-select size="sm" data-release value="${s.release}" label="${t("reports.release")}">
        <option value="">${t("reports.pickRelease")}</option>
        ${this.releases.filter((r) => r.state !== "archived").map((r) => html`<option value="r:${r.id}">${r.name}</option>`)}
        ${this.milestones.map((m) => html`<option value="m:${m.id}">◆ ${m.name}</option>`)}</lds-select>` : ""}
      ${s.tab === "workload" ? html`<lds-select size="sm" data-group value="${s.group}" label="${t("reports.group")}">
        ${GROUPS.map((g) => html`<option value="${g}">${t(`reports.groups.${g}`)}</option>`)}</lds-select>` : ""}
      ${s.tab === "flow" || s.tab === "workload" ? html`<lds-segmented size="sm" data-period value="${s.period}" label="${t("reports.period")}">
        <lds-segment value="week">${t("reports.weeks")}</lds-segment><lds-segment value="month">${t("reports.months")}</lds-segment></lds-segmented>` : ""}
      <span class="grow"></span>
      <lds-segmented size="sm" data-metric value="${s.metric}" label="${t("reports.metric")}">
        <lds-segment value="count">${t("reports.count")}</lds-segment><lds-segment value="estimate">${t("reports.estimate")}</lds-segment>
      </lds-segmented>`);
  }

  onChange(e) {
    const el = e.target;
    const v = e.detail?.value ?? "";
    const key = ["tab", "iteration", "release", "group", "period", "metric"].find((k) => el.matches?.(`[data-${k}]`));
    if (!key) return;
    this.state[key] = v;
    this.syncAddress();
    if (key === "tab") this.renderControls();
    this.run();
  }

  syncAddress() {
    const s = this.state;
    const url = new URL(location.href);
    for (const k of ["tab", "metric", "iteration", "release", "milestone", "group", "period"]) url.searchParams.delete(k);
    url.searchParams.set("tab", s.tab);
    if (s.metric === "estimate") url.searchParams.set("metric", "estimate");
    if (s.tab === "sprint" && s.iteration) url.searchParams.set("iteration", s.iteration);
    if (s.tab === "releases" && s.release) url.searchParams.set(s.release.startsWith("m:") ? "milestone" : "release", s.release.slice(2));
    if (s.tab === "workload") url.searchParams.set("group", s.group);
    if ((s.tab === "flow" || s.tab === "workload") && s.period === "month") url.searchParams.set("period", "month");
    this.context.router.navigate(url.pathname + url.search, { replace: true });
  }

  /** Runs a report; the filter applies to flow and workload ones. */
  report(name, extra = {}) {
    const query = { projects: this.data.project.key, metric: this.state.metric, ...extra };
    if (this.bar?.filter && (this.state.tab === "flow" || this.state.tab === "workload")) query.filter = JSON.stringify(this.bar.filter);
    return this.context.api.get(`${this.base}/reports/${name}`, { query }).then((r) => r.data);
  }

  async run() {
    const { i18n } = this.context;
    const seq = (this.seq = (this.seq ?? 0) + 1); // a later run wins over this one
    const s = this.state;
    let out;
    try {
      switch (s.tab) {
        case "sprint":
          out = s.iteration ? await Promise.all([this.report("burndown", { iteration: s.iteration }), this.report("sprint", { iteration: s.iteration }), this.report("velocity")]) : null;
          break;
        case "flow":
          out = await Promise.all([this.report("cfd"), this.report("cycle-time"), this.report("throughput", { period: s.period })]);
          break;
        case "releases":
          out = s.release ? await this.report("progress", { [s.release.startsWith("m:") ? "milestone" : "release"]: s.release.slice(2) }) : null;
          break;
        default:
          out = await Promise.all([this.report("distribution", { group: s.group }), this.report("created-resolved", { period: s.period })]);
      }
    } catch (err) {
      if (seq !== this.seq) return;
      this.shadowRoot.querySelector("[data-panels]").innerHTML = String(html`<lds-alert intent="danger" class="wide">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    if (seq !== this.seq) return;
    const meta = Array.isArray(out) ? out[0] : out;
    this.shadowRoot.querySelector("[data-notice]").innerHTML = String(meta?.metric_fallback
      ? html`<lds-alert intent="info" class="controls">${i18n.t(`reports.fallback.${meta.metric_fallback}`)}</lds-alert>` : "");
    this[`render_${s.tab}`](out);
  }

  /** A card with a chart element, its data set once it's in the page. */
  panels(cards) {
    const box = this.shadowRoot.querySelector("[data-panels]");
    box.innerHTML = String(html`${cards.map((c, k) => html`
      <lds-card class="${c.wide ? "wide" : ""}"><h2>${c.title}</h2>${c.note ? html`<p class="note">${c.note}</p>` : ""}
        ${c.chart ? html`<${c.chart} data-chart="${k}" label="${c.title}" table-label="${this.context.i18n.t("reports.showTable")}"
          chart-label="${this.context.i18n.t("reports.showChart")}" empty="${this.context.i18n.t("reports.noData")}"></${c.chart}>` : c.body}
      </lds-card>`)}`);
    cards.forEach((c, k) => {
      if (c.chart) box.querySelector(`[data-chart="${k}"]`).data = c.data;
    });
  }

  fmt(v) {
    return new Intl.NumberFormat(this.context.i18n.locale, { maximumFractionDigits: 1 }).format(v ?? 0);
  }

  render_sprint(out) {
    const { t } = this.context.i18n;
    if (!out) {
      this.shadowRoot.querySelector("[data-panels]").innerHTML = String(html`<lds-empty-state class="empty wide"><lds-icon slot="icon" name="repeat"></lds-icon>
        <h2 slot="title">${t("reports.noIterations")}</h2>${t("reports.noIterationsHint")}</lds-empty-state>`);
      return;
    }
    const [bd, sp, vel] = out;
    const today = bd.days.findLastIndex((_, i) => bd.remaining[i] != null);
    const tt = sp.totals;
    this.panels([
      { title: t("reports.burndown"), chart: "hzn-chart-line", data: { labels: bd.days, marker: today, series: [
        { name: t("reports.remaining"), tone: "started", values: bd.remaining },
        { name: t("reports.ideal"), tone: "reference", dashed: true, values: bd.ideal },
      ] } },
      { title: t("reports.burnup"), chart: "hzn-chart-line", data: { labels: bd.days, marker: today, series: [
        { name: t("reports.scope"), tone: "reference", values: bd.scope },
        { name: t("reports.completed"), tone: "completed", values: bd.completed },
      ] } },
      { title: t("reports.sprintReport"), wide: true, body: html`
        <div class="totals">${["committed", "added", "removed", "completed", "abandoned", "rolled_over", "open"].map((k) => html`
          <span>${t(`reports.sprint.${k}`)} <strong>${this.fmt(tt[k])}</strong></span>`)}</div>
        <table class="items"><thead><tr><th>${t("table.col.key")}</th><th>${t("table.col.title")}</th><th>${t("reports.sprint.scope")}</th>
          <th>${t("reports.sprint.ended")}</th><th class="num">${t(`reports.${sp.metric}`)}</th></tr></thead>
        <tbody>${sp.items.map((i) => html`<tr data-item="${i.key}"><td><a href="${issueHref(this.ws, i.key)}">${i.key}</a></td><td>${i.title}</td>
          <td>${t(i.committed ? "reports.sprint.committed" : "reports.sprint.added")}</td><td>${t(`reports.sprint.${i.ended}`)}</td>
          <td class="num">${this.fmt(i.value)}</td></tr>`)}</tbody></table>` },
      { title: t("reports.velocity"), wide: true, note: vel.items.length ? t("reports.velocityAverage", { value: this.fmt(vel.average) }) : "", chart: "hzn-chart-bar", data: {
        labels: vel.items.map((p) => p.iteration.name), series: [
          { name: t("reports.sprint.committed"), tone: "reference", values: vel.items.map((p) => p.committed) },
          { name: t("reports.completed"), tone: "completed", values: vel.items.map((p) => p.completed) },
        ] } },
    ]);
  }

  render_flow([cfd, ct, tp]) {
    const { t } = this.context.i18n;
    const pct = (p) => (p == null ? "–" : t("reports.days", { count: p }));
    this.panels([
      { title: t("reports.cfd"), wide: true, chart: "hzn-chart-area", data: { labels: cfd.days,
        series: cfd.series.map((s) => ({ name: t(`outcome.${s.key}`), tone: TONES[s.key], values: s.values })) } },
      { title: t("reports.cycleTime"), note: t("reports.cycleNote", { p50: pct(ct.cycle.p50), p85: pct(ct.cycle.p85), lead: pct(ct.lead.p50) }),
        chart: "hzn-chart-scatter", data: { xName: t("reports.completedOn"), yName: t("reports.cycleDays"),
          points: ct.items.filter((p) => p.cycle_days != null).map((p) => ({ x: p.completed, y: p.cycle_days, name: `${p.key} ${p.title}` })),
          refs: [{ name: "p50", value: ct.cycle.p50, tone: "reference" }, { name: "p85", value: ct.cycle.p85, tone: "annotation" }, { name: "p95", value: ct.cycle.p95, tone: "marker" }] } },
      { title: t("reports.throughput"), note: t("reports.average", { value: this.fmt(tp.average) }), chart: "hzn-chart-bar", data: {
        labels: tp.periods, series: [{ name: t("reports.completed"), tone: "completed", values: tp.series[0].values }] } },
    ]);
  }

  render_releases(p) {
    const { t } = this.context.i18n;
    if (!p) {
      this.shadowRoot.querySelector("[data-panels]").innerHTML = String(html`<lds-empty-state class="empty wide"><lds-icon slot="icon" name="rocket"></lds-icon>
        <h2 slot="title">${t("reports.pickRelease")}</h2></lds-empty-state>`);
      return;
    }
    const outcomes = ["completed", "started", "unstarted", "abandoned"];
    this.panels([
      { title: t("reports.scopeByOutcome", { name: p.name }), note: p.date ? t("reports.due", { date: p.date }) : "", chart: "hzn-chart-bar", data: {
        labels: [p.name], stacked: true, horizontal: true,
        series: outcomes.map((o) => ({ name: t(`outcome.${o}`), tone: o, values: [p.by_outcome[o]] })) } },
      { title: t("reports.burnup"), chart: "hzn-chart-line", data: { labels: p.days, series: [
        { name: t("reports.scope"), tone: "reference", values: p.scope },
        { name: t("reports.completed"), tone: "completed", values: p.completed },
      ] } },
    ]);
  }

  render_workload([dist, cr]) {
    const { t } = this.context.i18n;
    const name = (g) => g.name || t(dist.group === "assignee" ? "issue.unassigned" : "reports.none");
    this.panels([
      { title: t("reports.distribution"), note: t("reports.openIssues"), chart: "hzn-chart-bar", data: {
        labels: dist.groups.map((g) => (dist.group === "priority" ? t(`priority.${g.key}`) : name(g))), horizontal: true,
        series: [{ name: t(`reports.${dist.metric}`), tone: "started", values: dist.groups.map((g) => g.value) }] } },
      { title: t("reports.createdResolved"), chart: "hzn-chart-bar", data: { labels: cr.periods, series: [
        { name: t("reports.created"), tone: "started", values: cr.series[0].values },
        { name: t("reports.resolved"), tone: "completed", values: cr.series[1].values },
      ] } },
    ]);
  }
}

customElements.define("hzn-reports", HznReports);
