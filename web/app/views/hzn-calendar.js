// <hzn-calendar>: /w/:ws/p/:key/calendar[?mode=month|week&date=YYYY-MM-DD]
// — a project's issues on their due dates (wireframes: Calendar), with its
// milestones and release dates, its iterations as strips across the days
// they cover, and its non-working days shaded (holidays named). The week
// view also shows the issues under way each day.
//
// Arrow keys move between days (Page Up/Down by month or week), Enter on
// an issue opens it. Dragging an issue to another day, or Alt+arrows on a
// focused one, moves its dates by that many days, through the same plan
// as the timeline (hzn-reschedule).

import "horizon/components/hzn-reschedule.js";
import "horizon/components/hzn-view-bar.js";
import { Calendar, addDays, daysBetween, isoWeekday } from "horizon/components/calendar.js";
import { changesProjectData, forgetProjects, issueHref, issueStyles, projectData, statusGlyph } from "horizon/components/issue-ui.js";
import { fetchIssues, loadIterations, loadReleases, planningStyles, viewSwitch } from "horizon/components/planning-ui.js";
import { paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const MODES = ["month", "week"];
const PER_DAY = 3; // issues shown in a month cell before "+N more"

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
  .nav { display: flex; align-items: center; gap: var(--lds-space-2); padding: var(--lds-space-2) var(--lds-space-5); }
  .nav .grow { flex: 1; }
  .nav h2 { margin: 0 var(--lds-space-2); font-size: var(--lds-font-size-lg); }
  .cal { flex: 1; overflow: auto; padding: 0 var(--lds-space-5) var(--lds-space-5); }
  .weekdays, .week { display: grid; grid-template-columns: repeat(7, minmax(7rem, 1fr)); }
  .weekdays div { padding: var(--lds-space-1) var(--lds-space-2); font-size: var(--lds-font-size-xs); color: var(--lds-color-fg-muted);
    font-weight: var(--lds-font-weight-semibold); text-transform: uppercase; }
  .week { position: relative; border-block-start: 1px solid var(--lds-color-border-subtle); }
  .strips { grid-column: 1 / -1; display: grid; grid-template-columns: subgrid; gap: 2px 0; padding-block: 2px; }
  .strips:empty { display: none; }
  .strip { font-size: var(--lds-font-size-xs); padding-inline: var(--lds-space-2); border-radius: var(--lds-radius-xs); white-space: nowrap;
    overflow: hidden; text-overflow: ellipsis; background: var(--lds-color-bg-subtle); border: 1px solid var(--lds-color-border-default); }
  .strip[data-state="active"] { background: var(--lds-color-accent-subtle); border-color: var(--lds-color-accent-border); }
  .day { min-block-size: 7rem; padding: var(--lds-space-1); border-inline-start: 1px solid var(--lds-color-border-subtle); outline: none;
    display: flex; flex-direction: column; gap: 2px; }
  .week.tall .day { min-block-size: 22rem; }
  .day:nth-child(7n + 2) { border-inline-start: 0; }
  .day.off { background: var(--lds-color-bg-subtle); }
  .day.holiday { background: var(--lds-color-warning-subtle); }
  .day.other .num { color: var(--lds-color-fg-subtle); }
  .day:focus-visible { box-shadow: inset 0 0 0 2px var(--lds-color-accent-border); }
  .day.drop { box-shadow: inset 0 0 0 2px var(--lds-color-accent-fg); }
  .day .top { display: flex; align-items: baseline; gap: var(--lds-space-1); font-size: var(--lds-font-size-xs); }
  .day .num { font-weight: var(--lds-font-weight-semibold); font-variant-numeric: tabular-nums; }
  .day.today .num { color: var(--lds-color-fg-on-accent, #fff); background: var(--lds-color-accent-fg); border-radius: var(--lds-radius-full);
    padding-inline: 6px; }
  .day .note { color: var(--lds-color-fg-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .marker { font-size: var(--lds-font-size-xs); padding: 1px var(--lds-space-1); border-radius: var(--lds-radius-xs); white-space: nowrap;
    overflow: hidden; text-overflow: ellipsis; }
  .marker.milestone { background: var(--lds-color-info-subtle); }
  .marker.release { background: var(--lds-color-success-subtle); }
  .chip { display: flex; align-items: center; gap: var(--lds-space-1); font-size: var(--lds-font-size-xs); padding: 1px var(--lds-space-1);
    border-radius: var(--lds-radius-xs); background: var(--lds-color-bg-surface); border: 1px solid var(--lds-color-border-default);
    cursor: pointer; outline: none; white-space: nowrap; overflow: hidden; }
  .chip .title { overflow: hidden; text-overflow: ellipsis; }
  .chip:focus-visible { box-shadow: 0 0 0 2px var(--lds-color-accent-border); }
  .chip.under-way { border-style: dashed; color: var(--lds-color-fg-muted); }
  .chip.dragging { opacity: 0.5; }
  .more { all: unset; font-size: var(--lds-font-size-xs); color: var(--lds-color-fg-link); cursor: pointer; }
  .empty { padding: var(--lds-space-12) var(--lds-space-6); }
`);

/** The Monday on or before date. */
function monday(date) {
  return addDays(date, 1 - isoWeekday(date));
}

class HznCalendar extends HTMLElement {
  static observedAttributes = ["ws", "key"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [issueStyles, planningStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("keydown", (e) => this.onKey(e));
    root.addEventListener("lds-change", (e) => {
      if (e.target.matches("[data-mode]")) this.go(this.anchor, e.detail.value);
    });
    root.addEventListener("dragstart", (e) => this.onDragStart(e));
    root.addEventListener("dragover", (e) => this.onDragOver(e));
    root.addEventListener("drop", (e) => this.onDrop(e));
    root.addEventListener("dragend", () => this.endDrag());
    root.addEventListener("hzn-view-change", () => this.reload());
    this.offSocket = this.context.socket.on((m) => this.onPush(m));
    this.onChanged = () => this.refresh();
    document.addEventListener("hzn-issue-changed", this.onChanged);
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
      router.navigate(`${this.base}/p/${this.data.project.key}/calendar${location.search}`, { replace: true }); // a former key
      return;
    }
    this.calendar = Calendar.from(this.data.config.calendar);
    const q = router.current?.query;
    this.mode = MODES.includes(q?.get("mode")) ? q.get("mode") : "month";
    this.anchor = /^\d{4}-\d{2}-\d{2}$/.test(q?.get("date") ?? "") ? q.get("date") : this.calendar.today();
    this.focusDate = this.anchor;
    this.renderFrame();
    this.bar = this.shadowRoot.querySelector("hzn-view-bar");
    await this.bar.setup({ ws: this.ws, data: this.data, layout: "calendar", display: () => ({ ...this.bar.view?.display, mode: this.mode }) });
    if (MODES.includes(this.bar.view?.display?.mode) && !q?.has("mode")) this.mode = this.bar.view.display.mode;
    await this.reload();
  }

  async reload() {
    const seq = (this.seq = (this.seq ?? 0) + 1); // a later reload wins over this one
    const { api, i18n } = this.context;
    const key = this.data.project.key;
    try {
      const [items, iterations, releases, milestones] = await Promise.all([
        fetchIssues(this.context, this.ws, key, this.bar?.filter),
        loadIterations(this.context, this.ws, key).then((r) => r.items).catch(() => []),
        loadReleases(this.context, this.ws, key).then((r) => r.items).catch(() => []),
        api.get(`${this.base}/projects/${encodeURIComponent(key)}/milestones`).then((r) => r.data.items).catch(() => []),
      ]);
      if (seq !== this.seq) return;
      Object.assign(this, { items, iterations, releases, milestones });
    } catch (err) {
      if (seq !== this.seq) return;
      this.shadowRoot.querySelector("[data-cal]").innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    this.render();
  }

  async refresh() {
    const focused = this.shadowRoot.activeElement?.dataset?.chip;
    await this.reload();
    if (focused) this.shadowRoot.querySelector(`[data-chip="${CSS.escape(focused)}"]`)?.focus();
  }

  /** The days shown: whole weeks around the month, or the anchor's week. */
  days() {
    if (this.mode === "week") {
      const from = monday(this.anchor);
      return Array.from({ length: 7 }, (_, k) => addDays(from, k));
    }
    const first = `${this.anchor.slice(0, 8)}01`;
    const from = monday(first);
    const last = addDays(`${addDays(first, 32).slice(0, 8)}01`, -1);
    const to = addDays(monday(last), 6);
    return Array.from({ length: daysBetween(from, to) + 1 }, (_, k) => addDays(from, k));
  }

  /** Moves to date (in mode), keeping the address in step. */
  go(date, mode = this.mode) {
    this.anchor = date;
    this.focusDate = date;
    this.mode = mode;
    const url = new URL(location.href);
    url.searchParams.set("mode", mode);
    url.searchParams.set("date", date);
    this.context.router.navigate(url.pathname + url.search, { replace: true });
    this.render();
    this.bar?.refresh();
    this.shadowRoot.querySelector(`[data-day="${date}"]`)?.focus();
  }

  // --- Rendering ---------------------------------------------------------------------

  renderFrame() {
    const { t } = this.context.i18n;
    const p = this.data.project;
    this.shadowRoot.innerHTML = String(html`
      <header class="bar">
        <div class="crumb">
          <span class="swatch" data-colour="${p.colour}"></span><strong>${p.name}</strong>
          <lds-icon name="chevron-right" size="sm"></lds-icon><span>${t("planning.layouts.calendar")}</span>
        </div>
      </header>
      ${p.archived_at ? html`<lds-alert intent="warning">${t("projects.isArchived")}</lds-alert>` : ""}
      <div class="toolbar">${viewSwitch(t, this.ws, p.key, "calendar")}<hzn-view-bar></hzn-view-bar></div>
      <div class="nav" data-nav></div>
      <div class="cal" data-cal><lds-skeleton></lds-skeleton></div>
      <hzn-reschedule></hzn-reschedule>`);
    paintSwatches(this.shadowRoot);
  }

  render() {
    const { i18n } = this.context;
    const { t } = i18n;
    const days = this.days();
    const today = this.calendar.today();
    const month = this.anchor.slice(0, 7);
    const fmt = (opts) => new Intl.DateTimeFormat(i18n.locale, { timeZone: "UTC", ...opts });
    const title = this.mode === "month"
      ? fmt({ month: "long", year: "numeric" }).format(new Date(`${month}-01T00:00:00Z`))
      : t("calview.weekOf", { date: fmt({ dateStyle: "medium" }).format(new Date(`${days[0]}T00:00:00Z`)) });
    this.shadowRoot.querySelector("[data-nav]").innerHTML = String(html`
      <lds-button variant="ghost" size="sm" data-action="prev" label="${t(`calview.prev.${this.mode}`)}"><lds-icon name="chevron-left" size="sm"></lds-icon></lds-button>
      <lds-button variant="ghost" size="sm" data-action="next" label="${t(`calview.next.${this.mode}`)}"><lds-icon name="chevron-right" size="sm"></lds-icon></lds-button>
      <h2 data-title>${title}</h2>
      <lds-button variant="ghost" size="sm" data-action="today">${t("timeline.today")}</lds-button>
      <span class="grow"></span>
      <lds-segmented size="sm" data-mode value="${this.mode}" label="${t("calview.mode")}">
        ${MODES.map((m) => html`<lds-segment value="${m}">${t(`calview.modes.${m}`)}</lds-segment>`)}
      </lds-segmented>`);

    // What falls on each day.
    const due = new Map();
    const underWay = new Map();
    const put = (map, d, x) => {
      if (!map.has(d)) map.set(d, []);
      map.get(d).push(x);
    };
    for (const i of this.items) {
      const end = i.due_date ?? i.start_date;
      if (end) put(due, end, i);
      if (this.mode === "week" && i.start_date && i.due_date) {
        for (const d of days) if (d >= i.start_date && d < i.due_date) put(underWay, d, i);
      }
    }
    const markers = new Map();
    for (const m of this.milestones) if (m.date) put(markers, m.date, { cls: "milestone", label: `◆ ${m.name}` });
    for (const r of this.releases) if (r.release_date && r.state !== "archived") put(markers, r.release_date, { cls: "release", label: `${t("calview.release")} ${r.name}` });
    const weekdays = days.slice(0, 7).map((d) => fmt({ weekday: "short" }).format(new Date(`${d}T00:00:00Z`)));
    const weeks = [];
    for (let k = 0; k < days.length; k += 7) weeks.push(days.slice(k, k + 7));
    const chip = (i, cls = "") => html`
      <div class="chip ${cls}" role="button" tabindex="-1" data-chip="${i.key}" ${this.canEdit && !i.archived_at && !cls ? html`draggable="true"` : ""}
        aria-label="${i.key} ${i.title}" title="${i.key} ${i.title}">${statusGlyph(this.data, i.status_id)}<span class="key">${i.key}</span><span class="title">${i.title}</span></div>`;
    const box = this.shadowRoot.querySelector("[data-cal]");
    box.innerHTML = String(html`
      <div class="weekdays" aria-hidden="true">${weekdays.map((w) => html`<div>${w}</div>`)}</div>
      <div role="grid" aria-label="${title}">
        ${weeks.map((week) => {
          const strips = this.iterations.filter((it) => it.start_date && it.end_date && it.start_date <= week[6] && it.end_date >= week[0]);
          return html`
          <div class="week${this.mode === "week" ? " tall" : ""}" role="row">
            <div class="strips">${strips.map((it) => html`<div class="strip" data-state="${it.state}" data-from="${it.start_date < week[0] ? week[0] : it.start_date}"
              data-to="${it.end_date > week[6] ? week[6] : it.end_date}" data-week="${week[0]}" title="${it.name}">${it.name}</div>`)}</div>
            ${week.map((d) => {
              const holiday = this.calendar.holiday(d);
              const list = due.get(d) ?? [];
              const shown = this.mode === "month" ? list.slice(0, PER_DAY) : list;
              const cls = [!this.calendar.working(d) ? "off" : "", holiday ? "holiday" : "", d.slice(0, 7) !== month && this.mode === "month" ? "other" : "", d === today ? "today" : ""];
              return html`
              <div class="day ${cls.filter(Boolean).join(" ")}" role="gridcell" tabindex="${d === this.focusDate ? "0" : "-1"}" data-day="${d}"
                aria-label="${fmt({ dateStyle: "full" }).format(new Date(`${d}T00:00:00Z`))}${holiday ? ` · ${holiday}` : ""}${list.length ? ` · ${t("calview.due", { count: list.length })}` : ""}">
                <div class="top"><span class="num">${Number(d.slice(8))}</span>${holiday ? html`<span class="note">${holiday}</span>` : ""}</div>
                ${(markers.get(d) ?? []).map((m) => html`<div class="marker ${m.cls}">${m.label}</div>`)}
                ${shown.map((i) => chip(i))}
                ${(underWay.get(d) ?? []).map((i) => chip(i, "under-way"))}
                ${list.length > shown.length ? html`<button type="button" class="more" data-action="more" data-date="${d}">${t("calview.more", { count: list.length - shown.length })}</button>` : ""}
              </div>`;
            })}
          </div>`;
        })}
      </div>`);
    for (const s of box.querySelectorAll(".strip")) {
      const from = daysBetween(s.dataset.week, s.dataset.from) + 1;
      const to = daysBetween(s.dataset.week, s.dataset.to) + 2;
      s.style.setProperty("grid-column", `${from} / ${to}`); // through the CSSOM: the CSP forbids style attributes
    }
  }

  // --- Interaction -------------------------------------------------------------------

  issue(key) {
    return this.items.find((i) => i.key === key);
  }

  /** Moves an issue's dates by days (both ends; a lone date alone). */
  move(i, days) {
    if (!this.canEdit || i.archived_at || !days) return;
    const start = i.start_date ? addDays(i.start_date, days) : "";
    const due = i.due_date ? addDays(i.due_date, days) : "";
    this.shadowRoot.querySelector("hzn-reschedule").propose({ ws: this.ws, issue: i, start, due });
  }

  onKey(e) {
    const path = e.composedPath();
    const chipEl = path.find((n) => n.dataset?.chip);
    if (chipEl) {
      const i = this.issue(chipEl.dataset.chip);
      if (e.key === "Enter") {
        e.preventDefault();
        return this.context.router.navigate(issueHref(this.ws, i.key));
      }
      if (e.altKey && ["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown"].includes(e.key)) {
        e.preventDefault();
        return this.move(i, { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 }[e.key]);
      }
    }
    const dayEl = path.find((n) => n.dataset?.day);
    if (!dayEl || e.altKey) return;
    const d = dayEl.dataset.day;
    const step = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 }[e.key];
    if (step && !chipEl) {
      e.preventDefault();
      return this.focusDay(addDays(d, step));
    }
    if (e.key === "PageUp" || e.key === "PageDown") {
      e.preventDefault();
      const dir = e.key === "PageUp" ? -1 : 1;
      return this.go(this.mode === "week" ? addDays(d, 7 * dir) : this.shiftMonth(d, dir));
    }
    if (e.key === "Enter" && !chipEl) {
      e.preventDefault();
      dayEl.querySelector("[data-chip]")?.focus();
    }
  }

  /** The same day a month away (or the month's last day). */
  shiftMonth(date, dir) {
    const [y, m, d] = date.split("-").map(Number);
    const first = new Date(Date.UTC(y, m - 1 + dir, 1));
    const last = new Date(Date.UTC(first.getUTCFullYear(), first.getUTCMonth() + 1, 0)).getUTCDate();
    first.setUTCDate(Math.min(d, last));
    return first.toISOString().slice(0, 10);
  }

  focusDay(date) {
    const el = this.shadowRoot.querySelector(`[data-day="${date}"]`);
    if (!el || (this.mode === "month" && el.classList.contains("other"))) return this.go(date);
    for (const x of this.shadowRoot.querySelectorAll("[data-day]")) x.tabIndex = -1;
    el.tabIndex = 0;
    this.focusDate = date;
    el.focus();
  }

  onClick(e) {
    const el = e.target.closest?.("[data-action]");
    switch (el?.dataset.action) {
      case "prev":
        return this.go(this.mode === "week" ? addDays(this.anchor, -7) : this.shiftMonth(this.anchor, -1));
      case "next":
        return this.go(this.mode === "week" ? addDays(this.anchor, 7) : this.shiftMonth(this.anchor, 1));
      case "today":
        return this.go(this.calendar.today());
      case "more":
        return this.go(el.dataset.date, "week");
    }
    const chipEl = e.target.closest?.("[data-chip]");
    if (chipEl) return this.context.router.navigate(issueHref(this.ws, chipEl.dataset.chip));
    const day = e.target.closest?.("[data-day]");
    if (day) this.focusDay(day.dataset.day);
  }

  onDragStart(e) {
    const chipEl = e.target.closest?.("[data-chip][draggable]");
    if (!chipEl) return;
    this.dragKey = chipEl.dataset.chip;
    this.dragFrom = chipEl.closest("[data-day]").dataset.day;
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", this.dragKey);
    chipEl.classList.add("dragging");
  }

  onDragOver(e) {
    if (!this.dragKey) return;
    const day = e.target.closest?.("[data-day]");
    if (!day) return;
    e.preventDefault();
    for (const x of this.shadowRoot.querySelectorAll(".day.drop")) x.classList.remove("drop");
    day.classList.add("drop");
  }

  onDrop(e) {
    if (!this.dragKey) return;
    const day = e.target.closest?.("[data-day]");
    const key = this.dragKey;
    const from = this.dragFrom;
    this.endDrag();
    if (!day) return;
    e.preventDefault();
    this.move(this.issue(key), daysBetween(from, day.dataset.day));
  }

  endDrag() {
    for (const x of this.shadowRoot.querySelectorAll(".drop, .dragging")) x.classList.remove("drop", "dragging");
    this.dragKey = null;
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

customElements.define("hzn-calendar", HznCalendar);
