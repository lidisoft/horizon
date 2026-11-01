// Shared bits of the planning screens (wireframes: Backlog, Board,
// ViewSwitch): the project's layouts, iteration states and dates, and a
// project's iterations and releases.

import { html, sheet } from "horizon/core/dom.js";

export const planningStyles = sheet(`
  nav.views { display: flex; gap: var(--lds-space-1); }
  nav.views a { padding: var(--lds-space-1) var(--lds-space-3); border-radius: var(--lds-radius-sm); color: var(--lds-color-fg-muted);
    text-decoration: none; font-size: var(--lds-font-size-sm); }
  nav.views a:hover { background: var(--lds-color-bg-subtle); color: var(--lds-color-fg-default); }
  nav.views a[aria-current="page"] { background: var(--lds-color-bg-subtle); color: var(--lds-color-fg-default);
    font-weight: var(--lds-font-weight-medium); }
`);

/** The layouts of a project's issues, in the switcher's order. */
export const LAYOUTS = ["issues", "backlog"];

/** The layout switcher (wireframes: ViewSwitch). */
export function viewSwitch(t, ws, key, active) {
  return html`
    <nav class="views" aria-label="${t("planning.layout")}">
      ${LAYOUTS.map((l) => html`<a href="/w/${encodeURIComponent(ws)}/p/${encodeURIComponent(key)}/${l}" ${l === active ? html`aria-current="page"` : ""}>${t(`planning.layouts.${l}`)}</a>`)}
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
