// <hzn-reports>: /w/:ws/p/:key/reports — a project's reports
// (docs/reports.md), one at a time, picked from the list beside them
// (wireframes: Report): Sprint (burndown, burnup, sprint report, velocity),
// Flow (cumulative flow, cycle time, throughput), Planning (a release's or
// milestone's progress) and Workload (distribution, created vs resolved).
// Sprint reports open with the sprint's figures. Flow and workload take the
// view bar's filter. Count or estimate, the report and what it's about are
// in the address, so a report's URL is its shareable link.

import "horizon/components/charts.js";
import "horizon/components/hzn-view-bar.js";
import { projectData, issueHref, issueStyles } from "horizon/components/issue-ui.js";
import { loadIterations, loadReleases, planningStyles } from "horizon/components/planning-ui.js";
import { paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const TABS = ["sprint", "flow", "releases", "workload"];
/** The reports in the list, by group; tab is the family a report's data comes from. */
const REPORTS = [
  { id: "burndown", tab: "sprint", label: "reports.burndown" }, { id: "burnup", tab: "sprint", label: "reports.burnup" },
  { id: "sprint", tab: "sprint", label: "reports.sprintReport" }, { id: "velocity", tab: "sprint", label: "reports.velocity" },
  { id: "cfd", tab: "flow", label: "reports.cfd" }, { id: "cycle-time", tab: "flow", label: "reports.cycleLead" },
  { id: "throughput", tab: "flow", label: "reports.throughput" },
  { id: "release", tab: "releases", label: "reports.releaseProgress" }, { id: "milestone", tab: "releases", label: "reports.milestoneProgress" },
  { id: "distribution", tab: "workload", label: "reports.distribution" }, { id: "created-resolved", tab: "workload", label: "reports.createdResolved" },
];
/** The first report of each family, for addresses from before the list (?tab=). */
const FIRST = { sprint: "burndown", flow: "cfd", releases: "release", workload: "distribution" };
const GROUPS = ["assignee", "type", "priority", "label"];
const TONES = { completed: "completed", started: "started", unstarted: "unstarted", abandoned: "abandoned" };

const styles = sheet(`
  :host { display: grid; grid-template-columns: 13rem minmax(0, 1fr); min-block-size: 100%; }
  nav.reports { padding: var(--lds-space-4) var(--lds-space-3); border-inline-end: 1px solid var(--lds-color-border-default); }
  nav.reports h2 { margin: var(--lds-space-4) 0 var(--lds-space-1) var(--lds-space-2); font-size: var(--lds-font-size-2xs); letter-spacing: 0.08em;
    text-transform: uppercase; color: var(--lds-color-fg-muted); }
  nav.reports a { display: flex; align-items: center; gap: var(--lds-space-2); padding: var(--lds-space-1) var(--lds-space-2); border-radius: var(--lds-radius-sm);
    color: var(--lds-color-fg-default); text-decoration: none; font-size: var(--lds-font-size-sm); line-height: 1.5rem; }
  nav.reports a:hover { background: var(--lds-color-bg-subtle); }
  nav.reports a[aria-current="page"] { background: var(--lds-color-bg-subtle); font-weight: var(--lds-font-weight-semibold); }
  .main { display: flex; flex-direction: column; min-inline-size: 0; }
  header.bar { display: flex; align-items: center; justify-content: space-between; gap: var(--lds-space-3);
    min-block-size: 3.25rem; padding: 0 var(--lds-space-5); border-block-end: 1px solid var(--lds-color-border-default); }
  .crumb { display: flex; align-items: center; gap: var(--lds-space-2); }
  .crumb strong { font-weight: var(--lds-font-weight-semibold); }
  .crumb a { color: inherit; text-decoration: none; }
  .crumb lds-icon { color: var(--lds-color-fg-subtle); }
  .swatch { display: inline-block; inline-size: 0.625rem; block-size: 0.625rem; border-radius: var(--lds-radius-xs); }
  .controls { display: flex; align-items: center; gap: var(--lds-space-2); padding: var(--lds-space-2) var(--lds-space-5); box-sizing: border-box;
    min-block-size: 3rem; border-block-end: 1px solid var(--lds-color-border-subtle); }
  .controls lds-select { min-inline-size: 12rem; }
  .grow { flex: 1; }
  .barwrap { display: contents; }
  .barwrap[hidden] { display: none; }
  .panels { display: grid; gap: var(--lds-space-6); padding: var(--lds-space-5); }
  .tiles { display: grid; grid-template-columns: repeat(auto-fit, minmax(11rem, 1fr)); gap: var(--lds-space-3); }
  .tile { padding: var(--lds-space-3) var(--lds-space-4); border: 1px solid var(--lds-color-border-default); border-radius: var(--lds-radius-md); }
  .tile .name { font-size: var(--lds-font-size-xs); color: var(--lds-color-fg-muted); }
  .tile .value { display: block; margin-block: var(--lds-space-1); font-size: var(--lds-font-size-2xl); font-weight: var(--lds-font-weight-semibold);
    font-variant-numeric: tabular-nums; }
  .tile .note { font-size: var(--lds-font-size-xs); color: var(--lds-color-fg-muted); }
  section.panel h2 { margin: 0 0 var(--lds-space-1); font-size: var(--lds-font-size-md); }
  section.panel p.note { margin: 0 0 var(--lds-space-3); color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  table.items { inline-size: 100%; border-collapse: collapse; font-size: var(--lds-font-size-sm); border: 1px solid var(--lds-color-border-default); }
  table.items th { background: var(--lds-color-bg-subtle); font-size: var(--lds-font-size-xs); color: var(--lds-color-fg-muted); font-weight: var(--lds-font-weight-semibold); }
  table.items th, table.items td { padding: var(--lds-space-2) var(--lds-space-3); border-block-end: 1px solid var(--lds-color-border-subtle); text-align: start; }
  table.items td.num, table.items th.num { text-align: end; font-variant-numeric: tabular-nums; }
  table.items a { color: var(--lds-color-fg-muted); text-decoration: none; }
  table.items a:hover { text-decoration: underline; }
  .totals { display: flex; flex-wrap: wrap; gap: var(--lds-space-2) var(--lds-space-5); margin-block-end: var(--lds-space-3); font-size: var(--lds-font-size-sm); }
  .totals strong { font-variant-numeric: tabular-nums; }
  .empty { padding: var(--lds-space-12) var(--lds-space-6); }
  [data-report-pick] { display: none; }
  @media (max-width: 40rem) {
    :host { grid-template-columns: minmax(0, 1fr); }
    nav.reports { display: none; }
    [data-report-pick] { display: inline-flex; }
  }
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
    root.addEventListener("click", (e) => {
      const pick = e.target.closest?.("[data-report]");
      if (pick) {
        e.preventDefault();
        this.pick(pick.dataset.report);
      } else if (e.target.closest?.('[data-action="copy-link"]')) {
        navigator.clipboard?.writeText(location.href).catch(() => {});
        globalThis.LDS?.toast?.({ intent: "success", title: this.context.i18n.t("issues.linkCopied") });
      }
    });
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
    const tab = TABS.includes(q?.get("tab")) ? q.get("tab") : "sprint";
    const report = REPORTS.find((r) => r.id === q?.get("report"))?.id ?? (tab === "releases" && q?.get("milestone") ? "milestone" : FIRST[tab]);
    this.state = {
      report,
      tab: REPORTS.find((r) => r.id === report).tab,
      metric: q?.get("metric") === "estimate" ? "estimate" : "count",
      iteration: q?.get("iteration") ?? (sprints.find((it) => it.state === "active") ?? sprints[sprints.length - 1])?.id ?? "",
      release: q?.get("release") ?? (this.releases.find((r) => r.state === "unreleased") ? `r:${this.releases.find((r) => r.state === "unreleased").id}` : ""),
      group: GROUPS.includes(q?.get("group")) ? q.get("group") : "assignee",
      period: q?.get("period") === "month" ? "month" : "week",
    };
    if (q?.get("milestone")) this.state.release = `m:${q.get("milestone")}`;
    else if (q?.get("release")) this.state.release = `r:${q.get("release")}`;
    if (report === "milestone" && !this.state.release.startsWith("m:")) this.state.release = this.milestones[0] ? `m:${this.milestones[0].id}` : "";
    this.renderFrame();
    this.bar = null;
    this.renderControls();
    this.bar = this.shadowRoot.querySelector("hzn-view-bar");
    await this.bar.setup({ ws: this.ws, data: this.data, layout: "list" });
    this.run();
  }

  renderFrame() {
    const { t } = this.context.i18n;
    const p = this.data.project;
    const groups = [["sprint", "reports.nav.sprint"], ["flow", "reports.nav.flow"], ["releases", "reports.nav.planning"], ["workload", "reports.nav.workload"]];
    this.shadowRoot.innerHTML = String(html`
      <nav class="reports" aria-label="${t("planning.layouts.reports")}">
        <a href="/w/${encodeURIComponent(this.ws)}/p/${encodeURIComponent(p.key)}"><lds-icon name="eye" size="sm"></lds-icon>${t("planning.layouts.overview")}</a>
        ${groups.map(([tab, label]) => html`<h2>${t(label)}</h2>
          ${REPORTS.filter((r) => r.tab === tab).map((r) => html`<a href="?report=${r.id}" data-report="${r.id}">${t(r.label)}</a>`)}`)}
      </nav>
      <div class="main">
        <header class="bar">
          <div class="crumb">
            <span class="swatch" data-colour="${p.colour}"></span><strong>${p.name}</strong>
            <lds-icon name="chevron-right" size="sm"></lds-icon><span>${t("planning.layouts.reports")}</span>
            <lds-icon name="chevron-right" size="sm"></lds-icon><span data-report-name></span>
          </div>
          <lds-button variant="secondary" size="sm" data-action="copy-link"><lds-icon slot="start" name="link" size="sm"></lds-icon>${t("issues.copyLink")}</lds-button>
        </header>
        <div class="controls" data-controls></div>
        <div data-notice></div>
        <div class="panels" data-panels><lds-skeleton></lds-skeleton></div>
      </div>`);
    paintSwatches(this.shadowRoot);
  }

  /** Shows another report of the list. */
  pick(id) {
    const r = REPORTS.find((x) => x.id === id);
    if (!r || id === this.state.report) return;
    const tabChanged = r.tab !== this.state.tab;
    this.state.report = id;
    this.state.tab = r.tab;
    if (id === "milestone" && !this.state.release.startsWith("m:")) this.state.release = this.milestones[0] ? `m:${this.milestones[0].id}` : "";
    if (id === "release" && !this.state.release.startsWith("r:")) {
      const next = this.releases.find((x) => x.state === "unreleased");
      this.state.release = next ? `r:${next.id}` : "";
    }
    this.syncAddress();
    this.renderControls();
    if (tabChanged || r.tab === "releases") this.run();
    else this.render(this.last);
  }

  renderControls() {
    const { t } = this.context.i18n;
    const s = this.state;
    const sprints = this.iterations.filter((it) => it.state === "active" || it.state === "completed");
    for (const a of this.shadowRoot.querySelectorAll("[data-report]")) {
      if (a.dataset.report === s.report) a.setAttribute("aria-current", "page");
      else a.removeAttribute("aria-current");
    }
    this.shadowRoot.querySelector("[data-report-name]").textContent = t(REPORTS.find((r) => r.id === s.report).label);
    const controls = this.shadowRoot.querySelector("[data-controls]");
    // The view bar keeps its filter across reports: it moves, it isn't rebuilt.
    const bar = this.bar ?? this.shadowRoot.querySelector("hzn-view-bar");
    controls.innerHTML = String(html`
      <lds-select size="sm" data-report-pick value="${s.report}" label="${t("planning.layouts.reports")}">
        ${REPORTS.map((r) => html`<option value="${r.id}">${t(r.label)}</option>`)}</lds-select>
      ${s.tab === "sprint" ? html`<lds-select size="sm" data-iteration value="${s.iteration}" label="${t("reports.iteration")}">
        ${sprints.length ? sprints.map((it) => html`<option value="${it.id}">${it.name}</option>`) : html`<option value="">${t("reports.noIterations")}</option>`}</lds-select>` : ""}
      ${s.report === "release" ? html`<lds-select size="sm" data-release value="${s.release}" label="${t("reports.release")}">
        <option value="">${t("reports.pickRelease")}</option>
        ${this.releases.filter((r) => r.state !== "archived").map((r) => html`<option value="r:${r.id}">${r.name}</option>`)}</lds-select>` : ""}
      ${s.report === "milestone" ? html`<lds-select size="sm" data-release value="${s.release}" label="${t("reports.milestone")}">
        <option value="">${t("reports.pickMilestone")}</option>
        ${this.milestones.map((m) => html`<option value="m:${m.id}">${m.name}</option>`)}</lds-select>` : ""}
      ${s.report === "distribution" ? html`<lds-select size="sm" data-group value="${s.group}" label="${t("reports.group")}">
        ${GROUPS.map((g) => html`<option value="${g}">${t(`reports.groups.${g}`)}</option>`)}</lds-select>` : ""}
      ${s.report === "throughput" || s.report === "created-resolved" ? html`<lds-segmented size="sm" data-period value="${s.period}" label="${t("reports.period")}">
        <lds-segment value="week">${t("reports.weeks")}</lds-segment><lds-segment value="month">${t("reports.months")}</lds-segment></lds-segmented>` : ""}
      <lds-segmented size="sm" data-metric value="${s.metric}" label="${t("reports.metric")}">
        <lds-segment value="count">${t("reports.count")}</lds-segment><lds-segment value="estimate">${t("reports.estimate")}</lds-segment>
      </lds-segmented>
      <span class="barwrap" data-barwrap></span>`);
    const wrap = controls.querySelector("[data-barwrap]");
    wrap.hidden = s.tab !== "flow" && s.tab !== "workload";
    wrap.append(bar ?? Object.assign(document.createElement("hzn-view-bar")));
  }

  onChange(e) {
    const el = e.target;
    const v = e.detail?.value ?? "";
    if (el.matches?.("[data-report-pick]")) return this.pick(v);
    const key = ["iteration", "release", "group", "period", "metric"].find((k) => el.matches?.(`[data-${k}]`));
    if (!key) return;
    this.state[key] = v;
    this.syncAddress();
    this.run();
  }

  syncAddress() {
    const s = this.state;
    const url = new URL(location.href);
    for (const k of ["tab", "report", "metric", "iteration", "release", "milestone", "group", "period"]) url.searchParams.delete(k);
    url.searchParams.set("report", s.report);
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
    this.last = out;
    this.render(out);
  }

  /** Renders the chosen report from its family's data. */
  render(out) {
    if (out === undefined) return;
    this[`render_${this.state.tab}`](out);
  }

  /**
   * The panels of the chosen report (a panel lists the reports it belongs
   * to in for), each chart's data set once it's in the page.
   */
  panels(all) {
    const cards = all.filter((c) => !c.for || c.for.includes(this.state.report));
    const box = this.shadowRoot.querySelector("[data-panels]");
    box.innerHTML = String(html`${cards.map((c, k) => html`
      <section class="panel">${c.title ? html`<h2>${c.title}</h2>` : ""}${c.note ? html`<p class="note">${c.note}</p>` : ""}
        ${c.chart ? html`<${c.chart} data-chart="${k}" height="${c.height ?? 320}" label="${c.title}" table-label="${this.context.i18n.t("reports.showTable")}"
          chart-label="${this.context.i18n.t("reports.showChart")}" empty="${this.context.i18n.t("reports.noData")}"></${c.chart}>` : c.body}
      </section>`)}`);
    cards.forEach((c, k) => {
      if (c.chart) box.querySelector(`[data-chart="${k}"]`).data = c.data;
    });
  }

  /** The sprint's figures (wireframes: Report). */
  tiles(tt) {
    const { t } = this.context.i18n;
    const pct = tt.committed ? Math.round((tt.completed / tt.committed) * 100) : 0;
    const tile = (label, value, note) => html`<div class="tile"><span class="name">${label}</span><span class="value">${value}</span><span class="note">${note}</span></div>`;
    return html`<div class="tiles">
      ${tile(t("reports.sprint.committed"), this.fmt(tt.committed), t("reports.tile.atStart"))}
      ${tile(t("reports.sprint.completed"), this.fmt(tt.completed), t("reports.tile.ofCommitted", { percent: pct }))}
      ${tile(t("reports.tile.scopeChange"), `+${this.fmt(tt.added)} / −${this.fmt(tt.removed)}`, t("reports.tile.addedRemoved"))}
      ${tile(t("reports.sprint.rolled_over"), this.fmt(tt.rolled_over), t("reports.tile.open", { count: this.fmt(tt.open) }))}
    </div>`;
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
      { for: ["burndown", "burnup", "sprint"], body: this.tiles(tt) },
      { for: ["burndown"], title: t("reports.burndownTitle"), chart: "hzn-chart-line", data: { labels: bd.days, marker: today, series: [
        { name: t("reports.remaining"), tone: "started", values: bd.remaining },
        { name: t("reports.ideal"), tone: "reference", dashed: true, values: bd.ideal },
      ] } },
      { for: ["burnup"], title: t("reports.burnup"), chart: "hzn-chart-line", data: { labels: bd.days, marker: today, series: [
        { name: t("reports.scope"), tone: "reference", values: bd.scope },
        { name: t("reports.completed"), tone: "completed", values: bd.completed },
      ] } },
      { for: ["burndown", "sprint"], title: t("reports.sprintReport"), body: html`
        <div class="totals">${["committed", "added", "removed", "completed", "abandoned", "rolled_over", "open"].map((k) => html`
          <span>${t(`reports.sprint.${k}`)} <strong>${this.fmt(tt[k])}</strong></span>`)}</div>
        <table class="items"><thead><tr><th>${t("table.col.key")}</th><th>${t("table.col.title")}</th><th>${t("reports.sprint.scope")}</th>
          <th>${t("reports.sprint.ended")}</th><th class="num">${t(`reports.${sp.metric}`)}</th></tr></thead>
        <tbody>${sp.items.map((i) => html`<tr data-item="${i.key}"><td><a href="${issueHref(this.ws, i.key)}">${i.key}</a></td><td>${i.title}</td>
          <td>${t(i.committed ? "reports.sprint.committed" : "reports.sprint.added")}</td><td>${t(`reports.sprint.${i.ended}`)}</td>
          <td class="num">${this.fmt(i.value)}</td></tr>`)}</tbody></table>` },
      { for: ["velocity"], title: t("reports.velocity"), note: vel.items.length ? t("reports.velocityAverage", { value: this.fmt(vel.average) }) : "", chart: "hzn-chart-bar", data: {
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
      { for: ["cfd"], title: t("reports.cfd"), chart: "hzn-chart-area", data: { labels: cfd.days,
        series: cfd.series.map((s) => ({ name: t(`outcome.${s.key}`), tone: TONES[s.key], values: s.values })) } },
      { for: ["cycle-time"], title: t("reports.cycleTime"), note: t("reports.cycleNote", { p50: pct(ct.cycle.p50), p85: pct(ct.cycle.p85), lead: pct(ct.lead.p50) }),
        chart: "hzn-chart-scatter", data: { xName: t("reports.completedOn"), yName: t("reports.cycleDays"),
          points: ct.items.filter((p) => p.cycle_days != null).map((p) => ({ x: p.completed, y: p.cycle_days, name: `${p.key} ${p.title}` })),
          refs: [{ name: "p50", value: ct.cycle.p50, tone: "reference" }, { name: "p85", value: ct.cycle.p85, tone: "annotation" }, { name: "p95", value: ct.cycle.p95, tone: "marker" }] } },
      { for: ["throughput"], title: t("reports.throughput"), note: t("reports.average", { value: this.fmt(tp.average) }), chart: "hzn-chart-bar", data: {
        labels: tp.periods, series: [{ name: t("reports.completed"), tone: "completed", values: tp.series[0].values }] } },
    ]);
  }

  render_releases(p) {
    const { t } = this.context.i18n;
    if (!p) {
      this.shadowRoot.querySelector("[data-panels]").innerHTML = String(html`<lds-empty-state class="empty wide"><lds-icon slot="icon" name="rocket"></lds-icon>
        <h2 slot="title">${t(this.state.report === "milestone" ? "reports.pickMilestone" : "reports.pickRelease")}</h2></lds-empty-state>`);
      return;
    }
    const outcomes = ["completed", "started", "unstarted", "abandoned"];
    this.panels([
      { title: t("reports.scopeByOutcome", { name: p.name }), note: p.date ? t("reports.due", { date: p.date }) : "", chart: "hzn-chart-bar", height: 120, data: {
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
      { for: ["distribution"], title: t("reports.distribution"), note: t("reports.openIssues"), chart: "hzn-chart-bar", data: {
        labels: dist.groups.map((g) => (dist.group === "priority" ? t(`priority.${g.key}`) : name(g))), horizontal: true,
        series: [{ name: t(`reports.${dist.metric}`), tone: "started", values: dist.groups.map((g) => g.value) }] } },
      { for: ["created-resolved"], title: t("reports.createdResolved"), chart: "hzn-chart-bar", data: { labels: cr.periods, series: [
        { name: t("reports.created"), tone: "started", values: cr.series[0].values },
        { name: t("reports.resolved"), tone: "completed", values: cr.series[1].values },
      ] } },
    ]);
  }
}

customElements.define("hzn-reports", HznReports);
