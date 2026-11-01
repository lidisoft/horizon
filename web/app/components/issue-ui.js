// Shared bits of the issue screens: a project's data (its resolved
// configuration and the people who see it, cached per project and dropped
// when configuration changes), status and priority glyphs, people, labels
// and dates (wireframes: StatusIcon, PriorityIcon, Main, IssueDetail).

import { html, sheet } from "horizon/core/dom.js";

export const PRIORITIES = ["urgent", "high", "medium", "low", "none"];
const PRIORITY_ICONS = { urgent: "octagon-alert", high: "signal-high", medium: "signal-medium", low: "signal-low", none: "minus" };
const OUTCOME_ICONS = { backlog: "circle-dashed", unstarted: "circle", started: "circle-dot", completed: "circle-check", abandoned: "circle-x" };

/** Issue keys as people type them: HZN-42, hzn-42. */
export const ISSUE_KEY = /^[A-Za-z][A-Za-z0-9]{1,9}-[1-9][0-9]*$/;

export const issueStyles = sheet(`
  .glyph { display: inline-flex; inline-size: 1rem; block-size: 1rem; flex: none; align-items: center; justify-content: center; }
  .status[data-outcome="backlog"], .status[data-outcome="abandoned"] { color: var(--lds-color-fg-subtle); }
  .status[data-outcome="unstarted"] { color: var(--lds-color-fg-muted); }
  .status[data-outcome="started"] { color: var(--lds-color-accent-fg); }
  .status[data-outcome="completed"] { color: var(--lds-color-success-fg); }
  .priority { color: var(--lds-color-fg-default); }
  .priority[data-priority="urgent"] { color: var(--lds-color-danger-fg); }
  .priority[data-priority="none"] { color: var(--lds-color-fg-subtle); }
  .key { color: var(--lds-color-fg-muted); font-variant-numeric: tabular-nums; white-space: nowrap; }
  .label { display: inline-flex; align-items: center; gap: var(--lds-space-1); padding: 0 var(--lds-space-2);
    border: 1px solid var(--lds-color-border-default); border-radius: var(--lds-radius-full);
    font-size: var(--lds-font-size-xs); line-height: 1.25rem; white-space: nowrap; }
  .label[data-disabled] { color: var(--lds-color-fg-subtle); }
  .dot { display: inline-block; inline-size: 0.5rem; block-size: 0.5rem; border-radius: var(--lds-radius-full); flex: none; }
  .muted { color: var(--lds-color-fg-muted); }
  lds-alert:empty { display: none; }
`);

const projects = new Map();

/**
 * The project's data for issue screens, fetched once per project: the
 * project, its resolved configuration indexed by id, and its people.
 */
export function projectData(context, ws, key) {
  const id = `${ws}/${String(key).toUpperCase()}`;
  let p = projects.get(id);
  if (!p) {
    const base = `/w/${encodeURIComponent(ws)}/projects/${encodeURIComponent(key)}`;
    const { api } = context;
    p = Promise.all([api.get(base), api.get(`${base}/config/resolved`), api.get(`${base}/people`)])
      .then(([project, config, people]) => indexProject(project.data, config.data, people.data.items));
    p.catch(() => projects.delete(id));
    projects.set(id, p);
  }
  return p;
}

/** Drops cached project data, after a configuration or project change. */
export function forgetProjects() {
  projects.clear();
}

/** Events that change what projectData holds. */
export function changesProjectData(event) {
  const type = event.aggregate?.type;
  return type && type !== "issue" && type !== "role_grant";
}

function indexProject(project, config, people) {
  const categories = new Map(config.categories.map((c) => [c.id, c]));
  const unstarted = config.categories.filter((c) => c.outcome === "unstarted").sort((a, b) => a.position - b.position);
  const statuses = new Map();
  for (const w of config.workflows) {
    w.statuses.forEach((s, i) => {
      const c = categories.get(s.category_id);
      statuses.set(s.id, { ...s, workflow: w.id, outcome: c?.outcome ?? "unstarted", position: (c?.position ?? 0) * 1000 + i,
        backlog: unstarted.length > 1 && c?.id === unstarted[0].id });
    });
  }
  const workflows = new Map(config.workflows.map((w) => [w.id, w]));
  const types = new Map(config.issue_types.map((t) => [t.id, t]));
  return {
    project, config, categories, statuses, workflows, types,
    labels: new Map(config.labels.map((l) => [l.id, l])),
    fields: new Map(config.fields.map((f) => [f.id, f])),
    people: new Map(people.map((p) => [p.id, p])),
    peopleList: people,
    /** The statuses an issue of the type can be in, in workflow order. */
    statusesOf(typeId) {
      return workflows.get(types.get(typeId)?.workflow_id)?.statuses.map((s) => statuses.get(s.id)) ?? [];
    },
    /** The custom fields of the type, with whether they're required. */
    fieldsOf(typeId) {
      return (config.type_fields[typeId] ?? []).map((tf) => ({ ...this.fields.get(tf.field_id), required: tf.required })).filter((f) => f.id);
    },
  };
}

export function statusGlyph(data, statusId) {
  const s = data.statuses.get(statusId);
  const outcome = s?.backlog ? "backlog" : s?.outcome ?? "unstarted";
  return html`<span class="glyph status" data-outcome="${outcome}"><lds-icon name="${OUTCOME_ICONS[outcome]}" size="sm" label="${s?.name ?? ""}"></lds-icon></span>`;
}

export function priorityGlyph(priority, t) {
  const p = PRIORITY_ICONS[priority] ? priority : "none";
  return html`<span class="glyph priority" data-priority="${p}"><lds-icon name="${PRIORITY_ICONS[p]}" size="sm" label="${t(`priority.${p}`)}"></lds-icon></span>`;
}

export function priorityIcon(priority) {
  return PRIORITY_ICONS[priority] ?? PRIORITY_ICONS.none;
}

/** An avatar for a person of the project, or nothing. */
export function avatar(data, id, size = "sm") {
  if (!id) return "";
  const name = data.people.get(id)?.name ?? "?";
  return html`<lds-avatar size="${size}" name="${name}" title="${name}"></lds-avatar>`;
}

export function personName(data, id, t) {
  return id ? data.people.get(id)?.name ?? t("issue.someone") : t("issue.unassigned");
}

/** Label chips; switched-off labels stay, greyed. Paint with paintSwatches. */
export function labelChips(data, ids) {
  return ids
    .map((id) => data.labels.get(id))
    .filter(Boolean)
    .map((l) => html`<span class="label" ${l.disabled ? html`data-disabled` : ""}><span class="dot" data-colour="${l.colour}"></span>${l.name}</span>`);
}

/** "Nov 4" for a date-only string (no time zone shift). */
export function shortDate(value, i18n) {
  if (!value) return "";
  const [y, m, d] = value.split("-").map(Number);
  return i18n.date(new Date(y, m - 1, d), { month: "short", day: "numeric" });
}

/** The issue's page URL. */
export function issueHref(ws, key) {
  return `/w/${encodeURIComponent(ws)}/issues/${encodeURIComponent(key)}`;
}
