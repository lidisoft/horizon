// <hzn-timeline>: /w/:ws/p/:key/timeline — a Gantt of a project's issues
// (wireframes: Timeline). Bars run from start to due date; an issue with a
// due date only is a one-day bar. Arrows are precedes links. The project's
// non-working days are shaded, its iterations form a band above, and
// milestones and release dates are vertical markers.
//
// Dragging a bar moves it, dragging an end changes that date (Alt+←/→ and
// Alt+Shift+←/→ on a focused bar do the same by a working day). Before
// anything is saved, the server's plan says which issues the bar precedes
// would have to move and by how many working days: with none, the change
// is saved; with some, a dialog asks to move them all, the issue alone, or
// nothing (docs/frontend.md#timeline).

import "horizon/components/hzn-view-bar.js";
import { Calendar, addDays, daysBetween } from "horizon/components/calendar.js";
import { changesProjectData, forgetProjects, issueHref, issueStyles, projectData, statusGlyph } from "horizon/components/issue-ui.js";
import { fetchIssues, loadIterations, loadReleases, planningStyles, viewSwitch } from "horizon/components/planning-ui.js";
import { paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const ZOOMS = { week: 36, month: 14, quarter: 5 }; // pixels per day
const ROW = 36; // px, matches .row block-size
const BAND = 28; // the iterations band

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
  .gantt { flex: 1; display: grid; grid-template-columns: minmax(14rem, 22rem) minmax(0, 1fr); overflow: auto; position: relative; }
  .names { position: sticky; inset-inline-start: 0; z-index: 3; background: var(--lds-color-bg-canvas);
    border-inline-end: 1px solid var(--lds-color-border-default); }
  .names .head { block-size: calc(3rem + ${BAND}px); border-block-end: 1px solid var(--lds-color-border-default);
    position: sticky; top: 0; background: var(--lds-color-bg-canvas); z-index: 1; }
  .name { display: flex; align-items: center; gap: var(--lds-space-2); block-size: ${ROW}px; padding: 0 var(--lds-space-3);
    border-block-end: 1px solid var(--lds-color-border-subtle); font-size: var(--lds-font-size-sm); white-space: nowrap; overflow: hidden; }
  .name a { color: inherit; text-decoration: none; }
  .name .title { overflow: hidden; text-overflow: ellipsis; }
  .chart { position: relative; inline-size: var(--width); }
  .scale { position: sticky; top: 0; z-index: 2; background: var(--lds-color-bg-canvas); border-block-end: 1px solid var(--lds-color-border-default); }
  .months, .days { position: relative; block-size: 1.5rem; }
  .months span, .days span { position: absolute; inset-inline-start: var(--x); inline-size: var(--w); font-size: var(--lds-font-size-xs);
    color: var(--lds-color-fg-muted); padding-inline-start: 4px; white-space: nowrap; overflow: hidden; line-height: 1.5rem; }
  .months span { border-inline-start: 1px solid var(--lds-color-border-subtle); font-weight: var(--lds-font-weight-semibold); }
  .days span.today { color: var(--lds-color-accent-fg); font-weight: var(--lds-font-weight-semibold); }
  .band { position: relative; block-size: ${BAND}px; }
  .band span { position: absolute; inset-inline-start: var(--x); inline-size: var(--w); inset-block: 4px; border-radius: var(--lds-radius-sm);
    background: var(--lds-color-bg-subtle); border: 1px solid var(--lds-color-border-default); font-size: var(--lds-font-size-xs);
    padding-inline: 6px; line-height: 18px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
  .band span[data-state="active"] { background: var(--lds-color-accent-subtle); border-color: var(--lds-color-accent-border); }
  .lanes { position: relative; }
  .off { position: absolute; inset-block: 0; inset-inline-start: var(--x); inline-size: var(--w); background: var(--lds-color-bg-subtle); }
  .off.holiday { background: var(--lds-color-warning-subtle); }
  .marker { position: absolute; inset-block: 0; inset-inline-start: var(--x); border-inline-start: 2px dashed var(--lds-color-chart-marker, var(--lds-color-accent-border)); z-index: 1; }
  .marker.today { border-inline-start: 2px solid var(--lds-color-accent-fg); }
  .marker.release { border-inline-start-color: var(--lds-color-success-fg); }
  .marker span { position: absolute; top: 2px; inset-inline-start: 4px; font-size: var(--lds-font-size-xs); white-space: nowrap;
    background: var(--lds-color-bg-surface); padding-inline: 4px; border-radius: var(--lds-radius-xs); }
  .lane { position: relative; block-size: ${ROW}px; border-block-end: 1px solid var(--lds-color-border-subtle); }
  .lane.unscheduled { cursor: copy; }
  .gbar { position: absolute; inset-inline-start: var(--x); inline-size: var(--w); inset-block: 7px; border-radius: var(--lds-radius-sm);
    background: var(--lds-color-accent-subtle); border: 1px solid var(--lds-color-accent-border); cursor: grab; z-index: 2; outline: none;
    display: flex; align-items: center; font-size: var(--lds-font-size-xs); padding-inline: 6px; overflow: hidden; white-space: nowrap; touch-action: none; }
  .gbar[data-outcome="completed"] { background: var(--lds-color-success-subtle); border-color: var(--lds-color-success-border, var(--lds-color-border-strong)); }
  .gbar[data-outcome="abandoned"] { opacity: 0.5; }
  .gbar.point { border-style: dashed; }
  .gbar:focus-visible, .gbar[aria-current="true"] { box-shadow: 0 0 0 2px var(--lds-color-accent-border); }
  .gbar.dragging { opacity: 0.7; cursor: grabbing; }
  .gbar .grip { position: absolute; inset-block: 0; inline-size: 6px; cursor: ew-resize; }
  .gbar .grip.start { inset-inline-start: 0; }
  .gbar .grip.end { inset-inline-end: 0; }
  svg.links { position: absolute; inset: 0; pointer-events: none; z-index: 1; overflow: visible; }
  svg.links path { fill: none; stroke: var(--lds-color-fg-subtle); stroke-width: 1.5; }
  svg.links path.late { stroke: var(--lds-color-danger-fg); }
  .empty { padding: var(--lds-space-12) var(--lds-space-6); }
  .moves { display: grid; gap: var(--lds-space-2); margin: var(--lds-space-3) 0 0; padding: 0; list-style: none; }
  .moves li { display: grid; grid-template-columns: 5rem minmax(0, 1fr) auto; gap: var(--lds-space-3); align-items: baseline; }
  .moves .shift { font-variant-numeric: tabular-nums; color: var(--lds-color-fg-muted); }
  .moves .blocked { color: var(--lds-color-danger-fg); }
  .actions { display: flex; justify-content: flex-end; gap: var(--lds-space-2); flex-wrap: wrap; }
`);

function storedZoom() {
  try {
    return localStorage.getItem("horizon.timeline.zoom");
  } catch {
    return null;
  }
}

class HznTimeline extends HTMLElement {
  static observedAttributes = ["ws", "key"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [issueStyles, planningStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("keydown", (e) => this.onKey(e));
    root.addEventListener("pointerdown", (e) => this.onPointerDown(e));
    root.addEventListener("lds-change", (e) => {
      if (e.target.matches("[data-zoom]")) this.setZoom(e.detail.value);
    });
    root.addEventListener("hzn-view-change", () => this.reload());
    this.offSocket = this.context.socket.on((m) => this.onPush(m));
    this.onChanged = () => this.refresh();
    document.addEventListener("hzn-issue-changed", this.onChanged);
    this.zoom = ZOOMS[storedZoom()] ? storedZoom() : "month";
    this.load();
  }

  disconnectedCallback() {
    this.offSocket?.();
    document.removeEventListener("hzn-issue-changed", this.onChanged);
    clearTimeout(this.timer);
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

  get px() {
    return ZOOMS[this.zoom];
  }

  get canEdit() {
    return (this.data?.project.can ?? []).includes("issue.edit") && !this.data.project.archived_at;
  }

  async load() {
    const { i18n, router } = this.context;
    const key = this.getAttribute("key");
    try {
      this.data = await projectData(this.context, this.ws, key);
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`
        <lds-empty-state class="empty"><lds-icon slot="icon" name="search"></lds-icon>
          <h2 slot="title">${err.status === 404 ? i18n.t("projects.notFound") : errorMessage(err, i18n)}</h2>
          <a href="${this.base}">${i18n.t("notFound.home")}</a>
        </lds-empty-state>`);
      return;
    }
    if (this.data.project.key !== key.toUpperCase()) {
      router.navigate(`${this.base}/p/${this.data.project.key}/timeline${location.search}`, { replace: true }); // a former key
      return;
    }
    this.calendar = Calendar.from(this.data.config.calendar);
    this.renderFrame();
    this.bar = this.shadowRoot.querySelector("hzn-view-bar");
    await this.bar.setup({ ws: this.ws, data: this.data, layout: "timeline", display: () => ({ ...this.bar.view?.display, zoom: this.zoom }) });
    if (ZOOMS[this.bar.view?.display?.zoom]) this.zoom = this.bar.view.display.zoom;
    await this.reload();
  }

  async reload() {
    const { api, i18n } = this.context;
    const key = this.data.project.key;
    const p = `${this.base}/projects/${encodeURIComponent(key)}`;
    try {
      const [items, deps, iterations, releases, milestones] = await Promise.all([
        fetchIssues(this.context, this.ws, key, this.bar?.filter),
        api.get(`${p}/dependencies`).then((r) => r.data.items),
        loadIterations(this.context, this.ws, key).then((r) => r.items).catch(() => []),
        loadReleases(this.context, this.ws, key).then((r) => r.items).catch(() => []),
        api.get(`${p}/milestones`).then((r) => r.data.items).catch(() => []),
      ]);
      Object.assign(this, { items, deps, iterations, releases, milestones });
    } catch (err) {
      this.shadowRoot.querySelector("[data-gantt]").innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    this.renderChart();
  }

  /** Reloads, keeping the focus on the bar just moved. */
  async refresh() {
    await this.reload();
    if (this.focusKey) this.shadowRoot.querySelector(`[data-bar="${CSS.escape(this.focusKey)}"]`)?.focus();
  }

  /** An issue's bar span, or null when it has no dates. */
  span(i) {
    const start = i.start_date ?? i.due_date;
    const due = i.due_date ?? i.start_date;
    return start ? { start, due, point: !i.start_date || !i.due_date } : null;
  }

  /** The days shown: around the issues, markers and today, with margins. */
  range() {
    const today = this.calendar.today();
    let from = addDays(today, -14);
    let to = addDays(today, 60);
    const see = (d) => {
      if (!d) return;
      if (d < from) from = addDays(d, -7);
      if (d > to) to = addDays(d, 14);
    };
    for (const i of this.items) {
      const s = this.span(i);
      if (s) {
        see(s.start);
        see(s.due);
      }
    }
    for (const it of this.iterations) {
      see(it.start_date);
      see(it.end_date);
    }
    for (const m of this.milestones) see(m.date);
    for (const r of this.releases) see(r.release_date);
    // Start on a Monday, so weeks line up.
    from = addDays(from, -((new Date(`${from}T00:00:00Z`).getUTCDay() + 6) % 7));
    return { from, days: daysBetween(from, to) + 1 };
  }

  x(date) {
    return daysBetween(this.from, date) * this.px;
  }

  // --- Rendering ---------------------------------------------------------------------

  renderFrame() {
    const { t } = this.context.i18n;
    const p = this.data.project;
    this.shadowRoot.innerHTML = String(html`
      <header class="bar">
        <div class="crumb">
          <span class="swatch" data-colour="${p.colour}"></span><strong>${p.name}</strong>
          <lds-icon name="chevron-right" size="sm"></lds-icon><span>${t("planning.layouts.timeline")}</span>
        </div>
      </header>
      ${p.archived_at ? html`<lds-alert intent="warning">${t("projects.isArchived")}</lds-alert>` : ""}
      <div class="toolbar">
        ${viewSwitch(t, this.ws, p.key, "timeline")}
        <hzn-view-bar></hzn-view-bar>
        <lds-segmented size="sm" data-zoom value="${this.zoom}" label="${t("timeline.zoom")}">
          ${Object.keys(ZOOMS).map((z) => html`<lds-segment value="${z}">${t(`timeline.zooms.${z}`)}</lds-segment>`)}
        </lds-segmented>
        <lds-button variant="ghost" size="sm" data-action="today">${t("timeline.today")}</lds-button>
      </div>
      <div class="gantt" data-gantt><lds-skeleton></lds-skeleton></div>
      <lds-modal size="md" data-plan>
        <span slot="header" data-plan-title></span>
        <div data-plan-body></div>
        <div slot="footer" class="actions" data-plan-actions></div>
      </lds-modal>`);
    paintSwatches(this.shadowRoot);
  }

  renderChart() {
    const { i18n } = this.context;
    const { t } = i18n;
    const box = this.shadowRoot.querySelector("[data-gantt]");
    if (!this.items.length) {
      box.innerHTML = String(html`
        <lds-empty-state class="empty"><lds-icon slot="icon" name="${this.bar?.filter ? "filter" : "chart-gantt"}"></lds-icon>
          <h2 slot="title">${this.bar?.filter ? t("views.noMatch") : t("issues.empty")}</h2>
        </lds-empty-state>`);
      return;
    }
    const { from, days } = this.range();
    this.from = from;
    const today = this.calendar.today();
    // Scheduled issues first (by start), then those without dates.
    const rows = [...this.items].sort((a, b) => {
      const sa = this.span(a)?.start ?? "9999";
      const sb = this.span(b)?.start ?? "9999";
      return sa.localeCompare(sb);
    });
    this.rows = rows;
    const dates = Array.from({ length: days }, (_, k) => addDays(from, k));
    const last = dates[dates.length - 1];
    const monthName = new Intl.DateTimeFormat(i18n.locale, { month: "long", year: "numeric", timeZone: "UTC" });
    const months = dates.filter((d) => d === from || d.endsWith("-01")).map((d, k, all) => ({
      date: d, to: all[k + 1] ? addDays(all[k + 1], -1) : last, label: monthName.format(new Date(`${d}T00:00:00Z`)),
    }));
    // Every day when there's room, else Mondays (and every other one at the widest zoom).
    const step = this.px >= 14 ? 1 : this.px >= 5 ? 7 : 14;
    const dayLabels = dates.filter((d, k) => k % step === 0);
    const off = dates.filter((d) => !this.calendar.working(d));
    const markers = [
      ...this.milestones.filter((m) => m.date).map((m) => ({ cls: "milestone", date: m.date, label: `◆ ${m.name}` })),
      ...this.releases.filter((r) => r.release_date && r.state !== "archived").map((r) => ({ cls: "release", date: r.release_date, label: `${r.name}` })),
    ];
    const outcome = (i) => this.data.statuses.get(i.status_id)?.outcome ?? "unstarted";
    box.innerHTML = String(html`
      <div class="names">
        <div class="head"></div>
        ${rows.map((i) => html`
          <div class="name" data-key="${i.key}">${statusGlyph(this.data, i.status_id)}
            <a class="key" href="${issueHref(this.ws, i.key)}">${i.key}</a><span class="title" title="${i.title}">${i.title}</span></div>`)}
      </div>
      <div class="chart" data-chart>
        <div class="scale">
          <div class="months">${months.map((m) => html`<span data-x="${m.date}" data-to="${m.to}">${m.label}</span>`)}</div>
          <div class="days">${dayLabels.map((d) => html`<span data-x="${d}" data-to="${addDays(d, step - 1)}" class="${d === today ? "today" : ""}">${Number(d.slice(8))}</span>`)}</div>
          <div class="band" aria-label="${t("timeline.iterations")}">${this.iterations.filter((it) => it.start_date && it.end_date).map((it) => html`
            <span data-x="${it.start_date}" data-to="${it.end_date}" data-state="${it.state}" title="${it.name}">${it.name}</span>`)}</div>
        </div>
        <div class="lanes" data-lanes>
          ${off.map((d) => html`<div class="off${this.calendar.holiday(d) ? " holiday" : ""}" data-x="${d}" data-to="${d}" title="${this.calendar.holiday(d) ?? ""}"></div>`)}
          ${markers.map((m) => html`<div class="marker ${m.cls}" data-x="${m.date}"><span>${m.label}</span></div>`)}
          <div class="marker today" data-x="${today}" aria-hidden="true"></div>
          ${rows.map((i) => {
            const s = this.span(i);
            if (!s) return html`<div class="lane unscheduled" data-key="${i.key}" title="${this.canEdit ? t("timeline.clickToSchedule") : ""}"></div>`;
            return html`<div class="lane" data-key="${i.key}">
              <div class="gbar${s.point ? " point" : ""}" role="button" tabindex="0" data-bar="${i.key}" data-outcome="${outcome(i)}" data-x="${s.start}" data-to="${s.due}"
                aria-label="${t("timeline.barLabel", { key: i.key, title: i.title, start: s.start, due: s.due })}">
                ${this.canEdit && !i.archived_at ? html`<span class="grip start" data-grip="start"></span><span class="grip end" data-grip="end"></span>` : ""}
                ${i.title}
              </div></div>`;
          })}
          <svg class="links" data-links aria-hidden="true"></svg>
        </div>
      </div>`);
    const chart = box.querySelector("[data-chart]");
    chart.style.setProperty("--width", `${days * this.px}px`);
    // Positions through the CSSOM: the CSP forbids style attributes.
    for (const el of box.querySelectorAll("[data-x]")) {
      el.style.setProperty("--x", `${this.x(el.dataset.x)}px`);
      if (el.dataset.to) el.style.setProperty("--w", `${Math.max(1, daysBetween(el.dataset.x, el.dataset.to) + 1) * this.px - 1}px`);
    }
    this.drawLinks();
    if (!this.scrolled) {
      this.scrolled = true;
      this.scrollToDate(addDays(today, -7));
    }
  }

  /** Arrows from each predecessor's end to its successor's start. */
  drawLinks() {
    const svg = this.shadowRoot.querySelector("[data-links]");
    if (!svg) return;
    const rowOf = new Map(this.rows.map((i, k) => [i.key, k]));
    const byKey = new Map(this.rows.map((i) => [i.key, i]));
    const paths = [];
    for (const d of this.deps) {
      const a = byKey.get(d.from);
      const b = byKey.get(d.to);
      const sa = a && this.span(a);
      const sb = b && this.span(b);
      if (!sa || !sb) continue;
      const x1 = this.x(sa.due) + this.px;
      const y1 = rowOf.get(d.from) * ROW + ROW / 2;
      const x2 = this.x(sb.start);
      const y2 = rowOf.get(d.to) * ROW + ROW / 2;
      const late = sb.start < this.calendar.next(this.calendar.add(sa.due, 1 + (d.lag_days ?? 0)), 1);
      const mid = Math.max(x1 + 6, Math.min(x2 - 6, x1 + 12));
      paths.push(`<path class="${late ? "late" : ""}" d="M${x1} ${y1} H${mid} V${y2} H${x2 - 2} m-5 -4 l5 4 l-5 4" data-from="${d.from}" data-to="${d.to}"></path>`);
    }
    svg.innerHTML = paths.join("");
  }

  scrollToDate(date) {
    const gantt = this.shadowRoot.querySelector("[data-gantt]");
    if (gantt && this.from) gantt.scrollLeft = Math.max(0, this.x(date));
  }

  setZoom(zoom) {
    if (!ZOOMS[zoom]) return;
    const gantt = this.shadowRoot.querySelector("[data-gantt]");
    const center = this.from ? addDays(this.from, Math.round((gantt.scrollLeft + gantt.clientWidth / 2) / this.px)) : null;
    this.zoom = zoom;
    try {
      localStorage.setItem("horizon.timeline.zoom", zoom);
    } catch {
      // private mode
    }
    this.renderChart();
    if (center) gantt.scrollLeft = Math.max(0, this.x(center) - gantt.clientWidth / 2);
    this.bar?.refresh();
  }

  // --- Moving ------------------------------------------------------------------------

  issue(key) {
    return this.items.find((i) => i.key === key);
  }

  /** Dragging: the body moves the bar, a grip moves that end, by whole days. */
  onPointerDown(e) {
    if (e.button !== 0 || !this.canEdit) return;
    const path = e.composedPath();
    const barEl = path.find((n) => n.dataset?.bar);
    const lane = path.find((n) => n.classList?.contains("unscheduled"));
    if (lane) return this.scheduleAt(lane, e);
    if (!barEl) return;
    const i = this.issue(barEl.dataset.bar);
    if (!i || i.archived_at) return;
    const grip = path.find((n) => n.dataset?.grip)?.dataset.grip ?? "move";
    const s = this.span(i);
    const startX = e.clientX;
    barEl.setPointerCapture?.(e.pointerId);
    barEl.classList.add("dragging");
    let delta = 0;
    const move = (ev) => {
      delta = Math.round((ev.clientX - startX) / this.px);
      const start = grip === "end" ? s.start : addDays(s.start, delta);
      let due = grip === "start" ? s.due : addDays(s.due, delta);
      if (due < start) due = start;
      barEl.style.setProperty("--x", `${this.x(start)}px`);
      barEl.style.setProperty("--w", `${(daysBetween(start, due) + 1) * this.px - 1}px`);
    };
    const up = () => {
      barEl.removeEventListener("pointermove", move);
      barEl.removeEventListener("pointerup", up);
      barEl.removeEventListener("pointercancel", up);
      barEl.classList.remove("dragging");
      if (!delta) return this.renderChart();
      this.shift(i, grip, delta, false);
    };
    barEl.addEventListener("pointermove", move);
    barEl.addEventListener("pointerup", up);
    barEl.addEventListener("pointercancel", up);
  }

  /** A click on an unscheduled issue's lane gives it that working day. */
  scheduleAt(lane, e) {
    const i = this.issue(lane.dataset.key);
    if (!i) return;
    const box = lane.getBoundingClientRect();
    const day = this.calendar.next(addDays(this.from, Math.floor((e.clientX - box.left) / this.px)), 1);
    this.propose(i, day, day);
  }

  /** Moves a bar, or one of its ends, by delta days (working days with the keyboard). */
  shift(i, grip, delta, working) {
    const s = this.span(i);
    const by = (d) => (working ? this.calendar.add(d, delta) : addDays(d, delta));
    let start = grip === "end" ? s.start : by(s.start);
    let due = grip === "start" ? s.due : by(s.due);
    if (due < start) [start, due] = grip === "start" ? [due, due] : [start, start];
    // An issue with a due date only keeps having one.
    this.propose(i, i.start_date || grip === "start" ? start : "", due);
  }

  /** Asks the server for the plan; saves at once when nothing else moves. */
  async propose(i, start, due) {
    const { api, i18n } = this.context;
    const path = `${this.base}/issues/${encodeURIComponent(i.key)}/schedule`;
    const body = { start_date: start, due_date: due };
    try {
      const { data: plan } = await api.post(path, { body, ifMatch: `"${i.version}"` });
      if (!plan.moves.length) {
        await api.post(path, { body: { ...body, confirm: true }, ifMatch: `"${i.version}"` });
        return this.done(i.key);
      }
      this.pending = { i, body, plan };
      this.showPlan();
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: err.code === "version_conflict" ? i18n.t("issues.conflict") : errorMessage(err, i18n) });
      this.reload();
    }
  }

  showPlan() {
    const { t } = this.context.i18n;
    const { i, plan } = this.pending;
    const modal = this.shadowRoot.querySelector("[data-plan]");
    const movable = plan.moves.filter((m) => !m.blocked).length;
    modal.querySelector("[data-plan-title]").textContent = t("timeline.planTitle", { key: i.key });
    modal.querySelector("[data-plan-body]").innerHTML = String(html`
      <p>${t("timeline.planIntro", { key: i.key, count: plan.moves.length })}</p>
      <ul class="moves">${plan.moves.map((m) => html`
        <li data-move="${m.key}"><strong>${m.key}</strong><span>${m.title}</span>
          ${m.blocked ? html`<span class="blocked">${t(`timeline.blocked.${m.blocked}`)}</span>`
            : html`<span class="shift">${t("timeline.shift", { count: m.shift })} · ${m.to.start_date ?? ""}${m.to.start_date ? " → " : ""}${m.to.due_date ?? ""}</span>`}
        </li>`)}</ul>`);
    modal.querySelector("[data-plan-actions]").innerHTML = String(html`
      <lds-button variant="ghost" data-action="plan-cancel">${t("tokens.cancel")}</lds-button>
      <lds-button variant="secondary" data-action="plan-alone">${t("timeline.moveAlone", { key: i.key })}</lds-button>
      ${movable ? html`<lds-button variant="primary" data-action="plan-all">${t("timeline.moveAll", { count: movable + 1 })}</lds-button>` : ""}`);
    modal.setAttribute("open", "");
  }

  async applyPlan(all) {
    const { api, i18n } = this.context;
    const { i, body } = this.pending;
    this.pending = null;
    this.shadowRoot.querySelector("[data-plan]").removeAttribute("open");
    try {
      if (all) {
        const { data: plan } = await api.post(`${this.base}/issues/${encodeURIComponent(i.key)}/schedule`, { body: { ...body, confirm: true }, ifMatch: `"${i.version}"` });
        const failed = plan.moves.filter((m) => m.result !== "moved").length;
        if (failed) globalThis.LDS?.toast?.({ intent: "warning", title: i18n.t("timeline.notMoved", { count: failed }) });
      } else {
        await api.patch(`${this.base}/issues/${encodeURIComponent(i.key)}`, { body, ifMatch: `"${i.version}"` });
      }
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: err.code === "version_conflict" ? i18n.t("issues.conflict") : errorMessage(err, i18n) });
    }
    this.done(i.key);
  }

  done(key) {
    this.focusKey = key;
    document.dispatchEvent(new CustomEvent("hzn-issue-changed", { detail: { key } }));
  }

  onKey(e) {
    const bar = e.composedPath().find((n) => n.dataset?.bar);
    if (!bar) return;
    const i = this.issue(bar.dataset.bar);
    if (e.key === "Enter") {
      e.preventDefault();
      return this.context.router.navigate(issueHref(this.ws, i.key));
    }
    if (!e.altKey || (e.key !== "ArrowLeft" && e.key !== "ArrowRight") || !this.canEdit) return;
    e.preventDefault();
    this.focusKey = i.key;
    this.shift(i, e.shiftKey ? "end" : "move", e.key === "ArrowLeft" ? -1 : 1, true);
  }

  onClick(e) {
    const action = e.target.closest?.("[data-action]")?.dataset.action;
    switch (action) {
      case "today":
        return this.scrollToDate(addDays(this.calendar.today(), -7));
      case "plan-cancel":
        this.pending = null;
        this.shadowRoot.querySelector("[data-plan]").removeAttribute("open");
        return this.renderChart();
      case "plan-alone":
        return this.applyPlan(false);
      case "plan-all":
        return this.applyPlan(true);
    }
  }

  /** Refreshes on pushed events about this project's issues and plans. */
  onPush(m) {
    if (m.t !== "event" && m.t !== "resync") return;
    if (m.t === "event" && changesProjectData(m.d)) {
      forgetProjects();
      clearTimeout(this.timer);
      this.timer = setTimeout(() => this.load(), 200);
      return;
    }
    const key = m.d?.aggregate?.key ?? "";
    const type = m.d?.aggregate?.type;
    if (m.t === "resync" || key.startsWith(`${this.data?.project.key}-`) || ["iteration", "release", "milestone"].includes(type)) {
      clearTimeout(this.timer);
      this.timer = setTimeout(() => this.refresh(), 150);
    }
  }
}

customElements.define("hzn-timeline", HznTimeline);
