// <hzn-roadmap>: /w/:ws/roadmap[?projects=KEY,KEY&state=open|closed|all&q=]
// — the workspace roadmap (docs/frontend.md#roadmap): top-level issues
// that have children, across projects, grouped by project. Each bar is the
// issue's own dates, or (dashed) its children's span, filled with the share
// of its children completed. Milestones and release dates are markers;
// projects, state and words narrow it.

import { issueHref, issueStyles } from "horizon/components/issue-ui.js";
import { addDays, daysBetween, today } from "horizon/components/calendar.js";
import { paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const ZOOMS = { month: 12, quarter: 4, year: 1.6 }; // pixels per day
const STATES = ["open", "all", "closed"];
const ROW = 40;

const styles = sheet(`
  :host { display: flex; flex-direction: column; min-block-size: 100%; }
  header.bar { display: flex; align-items: center; justify-content: space-between; min-block-size: 3.25rem; padding: 0 var(--lds-space-5);
    border-block-end: 1px solid var(--lds-color-border-default); font-weight: var(--lds-font-weight-semibold); }
  .toolbar { display: flex; align-items: center; gap: var(--lds-space-2); padding: var(--lds-space-2) var(--lds-space-5);
    border-block-end: 1px solid var(--lds-color-border-subtle); flex-wrap: wrap; }
  .toolbar lds-combobox { min-inline-size: 14rem; }
  .toolbar .grow { flex: 1; }
  .gantt { flex: 1; display: grid; grid-template-columns: minmax(16rem, 24rem) minmax(0, 1fr); overflow: auto; }
  .names { position: sticky; inset-inline-start: 0; z-index: 3; background: var(--lds-color-bg-canvas); border-inline-end: 1px solid var(--lds-color-border-default); }
  .names .head, .scale { block-size: 1.75rem; position: sticky; top: 0; z-index: 2; background: var(--lds-color-bg-canvas);
    border-block-end: 1px solid var(--lds-color-border-default); }
  .group, .grouplane { block-size: 2rem; display: flex; align-items: center; gap: var(--lds-space-2); padding: 0 var(--lds-space-3);
    background: var(--lds-color-bg-subtle); border-block-end: 1px solid var(--lds-color-border-subtle); font-weight: var(--lds-font-weight-semibold);
    font-size: var(--lds-font-size-sm); }
  .grouplane { padding: 0; }
  .swatch { display: inline-block; inline-size: 0.625rem; block-size: 0.625rem; border-radius: var(--lds-radius-xs); }
  .name { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: var(--lds-space-2); block-size: ${ROW}px;
    padding: 0 var(--lds-space-3); border-block-end: 1px solid var(--lds-color-border-subtle); font-size: var(--lds-font-size-sm); }
  .name a { color: inherit; text-decoration: none; }
  .name .title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .name .count { color: var(--lds-color-fg-muted); font-variant-numeric: tabular-nums; }
  .chart { position: relative; inline-size: var(--width); }
  .scale { position: sticky; }
  .scale span { position: absolute; inset-inline-start: var(--x); inline-size: var(--w); font-size: var(--lds-font-size-xs); color: var(--lds-color-fg-muted);
    padding-inline-start: 4px; border-inline-start: 1px solid var(--lds-color-border-subtle); line-height: 1.75rem; white-space: nowrap; overflow: hidden; }
  .lanes { position: relative; }
  .lane { position: relative; block-size: ${ROW}px; border-block-end: 1px solid var(--lds-color-border-subtle); }
  .lane .none { position: absolute; inset-inline-start: var(--lds-space-2); top: 10px; font-size: var(--lds-font-size-xs); color: var(--lds-color-fg-subtle); }
  .gbar { position: absolute; inset-inline-start: var(--x); inline-size: var(--w); inset-block: 8px; border-radius: var(--lds-radius-sm); overflow: hidden;
    background: var(--lds-color-bg-surface); border: 1px solid var(--lds-color-accent-border); z-index: 2; outline: none; cursor: pointer;
    font-size: var(--lds-font-size-xs); white-space: nowrap; }
  .gbar.derived { border-style: dashed; }
  .gbar .fill { position: absolute; inset-block: 0; inset-inline-start: 0; inline-size: var(--done); background: var(--lds-color-chart-completed, var(--lds-color-success-subtle)); opacity: 0.35; }
  .gbar .label { position: relative; padding-inline: 6px; line-height: 22px; }
  .gbar:focus-visible { box-shadow: 0 0 0 2px var(--lds-color-accent-border); }
  .marker { position: absolute; inset-block: 0; inset-inline-start: var(--x); border-inline-start: 2px dashed var(--lds-color-accent-border); z-index: 1; }
  .marker.release { border-inline-start-color: var(--lds-color-success-fg); }
  .marker.today { border-inline-start: 2px solid var(--lds-color-accent-fg); }
  .marker span { position: absolute; top: 2px; inset-inline-start: 4px; font-size: var(--lds-font-size-xs); white-space: nowrap;
    background: var(--lds-color-bg-surface); padding-inline: 4px; border-radius: var(--lds-radius-xs); }
  .empty { padding: var(--lds-space-12) var(--lds-space-6); }
`);

class HznRoadmap extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [issueStyles, styles];
    root.addEventListener("lds-change", (e) => this.onChange(e));
    root.addEventListener("lds-input", (e) => {
      if (!e.target.matches("[data-q]")) return;
      clearTimeout(this.typing);
      this.typing = setTimeout(() => this.set({ q: e.detail.value.trim() }), 300);
    });
    root.addEventListener("keydown", (e) => {
      const bar = e.composedPath().find((n) => n.dataset?.bar);
      if (bar && e.key === "Enter") this.context.router.navigate(issueHref(this.ws, bar.dataset.bar));
    });
    root.addEventListener("click", (e) => {
      const bar = e.target.closest?.("[data-bar]");
      if (bar) this.context.router.navigate(issueHref(this.ws, bar.dataset.bar));
    });
    this.offSocket = this.context.socket.on((m) => {
      if (m.t === "resync" || (m.t === "event" && ["issue", "milestone", "release", "project"].includes(m.d?.aggregate?.type))) {
        clearTimeout(this.timer);
        this.timer = setTimeout(() => this.reload(), 300);
      }
    });
    this.zoom = "quarter";
    this.load();
  }

  disconnectedCallback() {
    this.offSocket?.();
    clearTimeout(this.timer);
    clearTimeout(this.typing);
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
    try {
      const [{ data: ws }, { data: projects }] = await Promise.all([api.get(this.base), api.get(`${this.base}/projects`)]);
      this.workspace = ws;
      this.projects = projects.items.filter((p) => !p.archived_at);
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    const q = router.current?.query;
    this.state = {
      projects: (q?.get("projects") ?? "").split(",").filter((k) => this.projects.some((p) => p.key === k)),
      status: STATES.includes(q?.get("state")) ? q.get("state") : "open",
      q: q?.get("q") ?? "",
    };
    this.renderFrame();
    await this.reload();
  }

  /** The filter the controls describe. */
  filter() {
    const and = [];
    if (this.state.status !== "all") and.push({ field: "status.outcome", op: "eq", value: this.state.status });
    if (this.state.q) and.push({ field: "text", op: "contains", value: this.state.q });
    return and.length ? { and } : undefined;
  }

  set(change) {
    Object.assign(this.state, change);
    const url = new URL(location.href);
    for (const [k, v] of [["projects", this.state.projects.join(",")], ["state", this.state.status === "open" ? "" : this.state.status], ["q", this.state.q]]) {
      if (v) url.searchParams.set(k, v);
      else url.searchParams.delete(k);
    }
    this.context.router.navigate(url.pathname + url.search, { replace: true });
    this.reload();
  }

  async reload() {
    const seq = (this.seq = (this.seq ?? 0) + 1); // a later reload wins over this one
    const { api, i18n } = this.context;
    try {
      const items = [];
      let cursor = "";
      do {
        const { data } = await api.post(`${this.base}/roadmap`, {
          body: { filter: this.filter(), projects: this.state.projects.length ? this.state.projects : undefined, limit: 500, cursor: cursor || undefined },
        });
        items.push(...data.items);
        cursor = data.next_cursor;
      } while (cursor);
      const shown = new Set(items.map((x) => x.issue.project_id));
      const [milestones, ...releases] = await Promise.all([
        api.get(`${this.base}/milestones`).then((r) => r.data.items).catch(() => []),
        ...this.projects.filter((p) => shown.has(p.id)).map((p) => api.get(`${this.base}/projects/${encodeURIComponent(p.key)}/releases`)
          .then((r) => r.data.items.map((x) => ({ ...x, project: p }))).catch(() => [])),
      ]);
      if (seq !== this.seq) return;
      Object.assign(this, { items, milestones: milestones.filter((m) => !m.project_id || shown.has(m.project_id)), releases: releases.flat() });
    } catch (err) {
      if (seq !== this.seq) return;
      this.shadowRoot.querySelector("[data-gantt]").innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    this.renderChart();
  }

  renderFrame() {
    const { t } = this.context.i18n;
    this.shadowRoot.innerHTML = String(html`
      <header class="bar">${t("roadmap.title")}</header>
      <div class="toolbar">
        <lds-combobox data-projects multiple value="${this.state.projects.join(",")}" label="${t("roadmap.projects")}" placeholder="${t("roadmap.allProjects")}">
          ${this.projects.map((p) => html`<lds-option value="${p.key}">${p.name}</lds-option>`)}
        </lds-combobox>
        <lds-segmented size="sm" data-state value="${this.state.status}" label="${t("roadmap.state")}">
          ${STATES.map((s) => html`<lds-segment value="${s}">${t(`roadmap.states.${s}`)}</lds-segment>`)}
        </lds-segmented>
        <lds-input type="search" size="sm" data-q value="${this.state.q}" placeholder="${t("views.textPlaceholder")}" aria-label="${t("views.text")}">
          <lds-icon slot="prefix" name="search"></lds-icon></lds-input>
        <span class="grow"></span>
        <lds-segmented size="sm" data-zoom value="${this.zoom}" label="${t("timeline.zoom")}">
          ${Object.keys(ZOOMS).map((z) => html`<lds-segment value="${z}">${t(`roadmap.zooms.${z}`)}</lds-segment>`)}
        </lds-segmented>
      </div>
      <div class="gantt" data-gantt><lds-skeleton></lds-skeleton></div>`);
  }

  onChange(e) {
    const el = e.target;
    if (el.matches("[data-projects]")) this.set({ projects: e.detail.values ?? (e.detail.value ? e.detail.value.split(",") : []) });
    else if (el.matches("[data-state]")) this.set({ status: e.detail.value });
    else if (el.matches("[data-zoom]") && ZOOMS[e.detail.value]) {
      this.zoom = e.detail.value;
      this.renderChart();
    }
  }

  x(date) {
    return daysBetween(this.from, date) * ZOOMS[this.zoom];
  }

  renderChart() {
    const { i18n } = this.context;
    const { t } = i18n;
    const box = this.shadowRoot.querySelector("[data-gantt]");
    if (!this.items.length) {
      box.innerHTML = String(html`
        <lds-empty-state class="empty"><lds-icon slot="icon" name="chart-gantt"></lds-icon>
          <h2 slot="title">${t("roadmap.empty")}</h2>${t("roadmap.emptyHint")}
        </lds-empty-state>`);
      return;
    }
    const now = today(this.workspace.time_zone);
    let from = addDays(now, -30);
    let to = addDays(now, 120);
    const see = (d) => {
      if (d && d < from) from = d;
      if (d && d > to) to = d;
    };
    for (const it of this.items) {
      see(it.start_date);
      see(it.due_date);
    }
    for (const m of this.milestones) see(m.date);
    for (const r of this.releases) see(r.release_date);
    from = `${from.slice(0, 8)}01`;
    to = addDays(`${addDays(`${to.slice(0, 8)}01`, 40).slice(0, 8)}01`, -1);
    this.from = from;
    const days = daysBetween(from, to) + 1;
    const px = ZOOMS[this.zoom];
    const months = [];
    for (let d = from; d <= to;) {
      const next = `${addDays(d, 32).slice(0, 8)}01`; // the first of the following month
      months.push({ date: d, to: addDays(next, -1) });
      d = next;
    }
    const monthName = new Intl.DateTimeFormat(i18n.locale, { month: px >= 4 ? "long" : "short", year: px >= 4 ? "numeric" : undefined, timeZone: "UTC" });
    const groups = this.projects.map((p) => ({ p, items: this.items.filter((x) => x.issue.project_id === p.id) })).filter((g) => g.items.length);
    const keyOf = new Map(this.projects.map((p) => [p.id, p.key]));
    const markers = [
      ...this.milestones.filter((m) => m.date).map((m) => ({ cls: "milestone", date: m.date, label: `◆ ${m.project_id ? `${keyOf.get(m.project_id) ?? ""} · ` : ""}${m.name}` })),
      ...this.releases.filter((r) => r.release_date && r.state !== "archived").map((r) => ({ cls: "release", date: r.release_date, label: `${r.project.key} ${r.name}` })),
    ];
    const done = (it) => {
      const counted = it.children.count - it.children.abandoned;
      return counted > 0 ? Math.round((100 * it.children.completed) / counted) : 0;
    };
    box.innerHTML = String(html`
      <div class="names">
        <div class="head"></div>
        ${groups.map((g) => html`
          <div class="group"><span class="swatch" data-colour="${g.p.colour}"></span>${g.p.name}</div>
          ${g.items.map((it) => html`
            <div class="name"><a class="key" href="${issueHref(this.ws, it.issue.key)}">${it.issue.key}</a>
              <span class="title" title="${it.issue.title}">${it.issue.title}</span>
              <span class="count" title="${t("roadmap.progress", { done: it.children.completed, count: it.children.count })}">${it.children.completed}/${it.children.count}</span></div>`)}`)}
      </div>
      <div class="chart" data-chart>
        <div class="scale">${months.map((m) => html`<span data-x="${m.date}" data-to="${m.to}">${monthName.format(new Date(`${m.date}T00:00:00Z`))}</span>`)}</div>
        <div class="lanes">
          ${markers.map((m) => html`<div class="marker ${m.cls}" data-x="${m.date}"><span>${m.label}</span></div>`)}
          <div class="marker today" data-x="${now}" aria-hidden="true"></div>
          ${groups.map((g) => html`
            <div class="grouplane"></div>
            ${g.items.map((it) => html`<div class="lane">${it.start_date || it.due_date
              ? html`<div class="gbar${it.derived ? " derived" : ""}" role="link" tabindex="0" data-bar="${it.issue.key}" data-x="${it.start_date ?? it.due_date}"
                  data-to="${it.due_date ?? it.start_date}" data-done="${done(it)}"
                  aria-label="${t("roadmap.barLabel", { key: it.issue.key, title: it.issue.title, start: it.start_date ?? it.due_date, due: it.due_date ?? it.start_date, done: done(it) })}"
                  title="${it.derived ? t("roadmap.derived") : ""}"><span class="fill"></span><span class="label">${it.issue.title}</span></div>`
              : html`<span class="none">${t("roadmap.noDates")}</span>`}</div>`)}`)}
        </div>
      </div>`);
    paintSwatches(box);
    box.querySelector("[data-chart]").style.setProperty("--width", `${days * px}px`);
    for (const el of box.querySelectorAll("[data-x]")) {
      el.style.setProperty("--x", `${this.x(el.dataset.x)}px`);
      if (el.dataset.to) el.style.setProperty("--w", `${Math.max(2, (daysBetween(el.dataset.x, el.dataset.to) + 1) * px - 1)}px`);
      if (el.dataset.done) el.style.setProperty("--done", `${el.dataset.done}%`);
    }
    if (!this.scrolled) {
      this.scrolled = true;
      box.scrollLeft = Math.max(0, this.x(addDays(now, -30)));
    }
  }
}

customElements.define("hzn-roadmap", HznRoadmap);
