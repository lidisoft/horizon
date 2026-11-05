// Shared bits of the planning screens (wireframes: Backlog, Board,
// ViewSwitch): the project's layouts, iteration states and dates, and a
// project's iterations and releases.

import { html, sheet } from "horizon/core/dom.js";

export const planningStyles = sheet(`
  nav.views { display: inline-flex; flex: none; gap: 2px; padding: 2px; border-radius: var(--lds-radius-md);
    background: var(--lds-segmented-bg, var(--lds-color-bg-subtle)); }
  nav.views a { display: inline-flex; align-items: center; gap: var(--lds-space-2); block-size: 1.75rem; padding: 0 var(--lds-space-3);
    border-radius: var(--lds-radius-sm); color: var(--lds-color-fg-muted); text-decoration: none; font-size: var(--lds-font-size-sm);
    white-space: nowrap; }
  nav.views a:hover { color: var(--lds-color-fg-default); }
  nav.views a:focus-visible { outline: 2px solid var(--lds-color-accent-default); outline-offset: -2px; }
  nav.views a[aria-current="page"] { background: var(--lds-color-bg-canvas); color: var(--lds-color-fg-default);
    font-weight: var(--lds-font-weight-medium); box-shadow: var(--lds-shadow-xs, 0 1px 2px rgb(0 0 0 / 0.08)); }
  @media (max-width: 72rem) { nav.views a span { display: none; } }
  .progress { display: inline-flex; align-items: center; gap: var(--lds-space-2); white-space: nowrap; }
  .progress lds-progress { inline-size: 6rem; }
`);

/** The layouts of a project's issues, in the switcher's order, with their icons. */
export const LAYOUTS = [
  ["issues", "list"], ["board", "kanban"], ["backlog", "layers"], ["table", "table"], ["timeline", "chart-gantt"], ["calendar", "calendar-days"],
];

/** The layout switcher (wireframes: ViewSwitch): links styled as a segmented control. */
export function viewSwitch(t, ws, key, active) {
  return html`
    <nav class="views" aria-label="${t("planning.layout")}">
      ${LAYOUTS.map(([l, icon]) => html`<a href="/w/${encodeURIComponent(ws)}/p/${encodeURIComponent(key)}/${l}" ${l === active ? html`aria-current="page"` : ""}
        title="${t(`planning.layouts.${l}`)}"><lds-icon name="${icon}" size="sm"></lds-icon><span>${t(`planning.layouts.${l}`)}</span></a>`)}
    </nav>`;
}

const STATE_INTENTS = { active: "info", scheduled: "neutral", planned: "neutral", completed: "success", archived: "neutral" };

/** The state badge of an iteration. */
export function stateBadge(t, state) {
  return html`<lds-badge size="sm" intent="${STATE_INTENTS[state] ?? "neutral"}">${t(`iterations.state.${state}`)}</lds-badge>`;
}

/** A date-only string as a local date. */
export function localDate(value) {
  const [y, m, d] = value.split("-").map(Number);
  return new Date(y, m - 1, d);
}

/** "Nov 9 – Nov 20", or "" without dates. */
export function dateSpan(it, i18n) {
  if (!it.start_date || !it.end_date) return "";
  const f = (v) => i18n.date(localDate(v), { month: "short", day: "numeric" });
  return `${f(it.start_date)} – ${f(it.end_date)}`;
}

/** Whole days from today to an iteration's end, in the user's time zone (negative once over). */
export function daysLeft(it, now = new Date()) {
  if (!it.end_date) return null;
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  return Math.round((localDate(it.end_date) - today) / 86_400_000);
}

/** Iterations issues can still be loaded into. */
export function isOpen(it) {
  return it.state === "planned" || it.state === "scheduled" || it.state === "active";
}

/** Points as people read them: 3, 2.5. */
export function points(n, i18n) {
  return i18n.number(Math.round(n * 10) / 10);
}

/** The project's iterations (GET …/iterations). */
export async function loadIterations(context, ws, key) {
  const { data } = await context.api.get(`/w/${encodeURIComponent(ws)}/projects/${encodeURIComponent(key)}/iterations`);
  return data;
}

/** The project's releases (GET …/releases). */
export async function loadReleases(context, ws, key) {
  const { data } = await context.api.get(`/w/${encodeURIComponent(ws)}/projects/${encodeURIComponent(key)}/releases`);
  return data;
}

/**
 * Progress of a set of issues (stats from the API): completed points over
 * estimated points when estimated, else completed issues over issues.
 */
export function progress(stats, i18n) {
  const { t } = i18n;
  const done = stats.by_outcome?.completed ?? { issues: 0, points: 0 };
  const byPoints = stats.points > 0;
  const value = byPoints ? done.points : done.issues;
  const max = byPoints ? stats.points : stats.issues;
  const text = byPoints
    ? t("planning.progressPoints", { done: points(done.points, i18n), total: points(stats.points, i18n), count: done.issues, issues: stats.issues })
    : t("planning.progressIssues", { done: done.issues, count: stats.issues });
  return html`<span class="progress"><lds-progress value="${max ? Math.round((value / max) * 100) : 0}" max="100" label="${text}"></lds-progress><span class="muted">${text}</span></span>`;
}

/**
 * Every issue of a project in rank order, or those a filter (a query AST)
 * matches in the sort given, a page of 500 at a time.
 */
export async function fetchIssues(context, ws, key, filter, sort) {
  const { api } = context;
  const items = [];
  let cursor = "";
  do {
    const { data } = filter || sort?.length
      ? await api.post(`/w/${encodeURIComponent(ws)}/issues/query`, {
        body: { projects: [key], filter: filter ?? undefined, sort: sort?.length ? sort : [{ field: "rank" }], limit: 500, cursor: cursor || undefined },
      })
      : await api.get(`/w/${encodeURIComponent(ws)}/projects/${encodeURIComponent(key)}/issues`, { query: { limit: 500, cursor: cursor || undefined } });
    items.push(...data.items);
    cursor = data.next_cursor;
  } while (cursor);
  return items;
}
