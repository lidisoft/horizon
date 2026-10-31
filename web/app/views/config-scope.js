// The scope of the configuration pages: the workspace, or one of its
// projects (?project=KEY). Workspace definitions are inherited by projects;
// a project can switch them off, copy them and add its own (ADR-0011).

import { html } from "horizon/core/dom.js";

/**
 * Loads the workspace, its projects and the scope chosen in the URL.
 * @returns {Promise<{ ws: object, projects: object[], project: object|null, base: string }>}
 *   base is the API prefix of the scope's configuration.
 */
export async function loadScope(context, slug) {
  const { api, router } = context;
  const enc = encodeURIComponent(slug);
  const [ws, projects] = await Promise.all([api.get(`/w/${enc}`), api.get(`/w/${enc}/projects`)]);
  const key = router.current?.query.get("project")?.toUpperCase();
  const project = projects.data.items.find((p) => p.key === key) ?? null;
  const base = project ? `/w/${enc}/projects/${project.key}/config` : `/w/${enc}/config`;
  return { ws: ws.data, projects: projects.data.items.filter((p) => !p.archived_at), project, base };
}

/** The scope picker for the settings header. */
export function scopePicker(t, scope) {
  return html`
    <lds-select size="sm" data-scope value="${scope.project?.key ?? ""}" label="${t("config.scope")}">
      <option value="">${t("config.workspaceScope", { name: scope.ws.name })}</option>
      ${scope.projects.map((p) => html`<option value="${p.key}">${p.name}</option>`)}
    </lds-select>`;
}

/** Navigates to page in the scope the picker selects. */
export function watchScope(root, context, slug, page) {
  root.querySelector("[data-scope]")?.addEventListener("lds-change", (e) => {
    const q = e.detail.value ? `?project=${encodeURIComponent(e.detail.value)}` : "";
    context.router.navigate(`/w/${slug}/settings/${page}${q}`, { replace: true });
  });
}

/** The badge telling where a definition comes from, seen from the scope. */
export function originBadge(t, item, scope) {
  if (!scope.project) return "";
  if (!item.inherited) return html`<lds-badge size="sm" intent="info">${t("config.projectOnly", { name: scope.project.name })}</lds-badge>`;
  if (item.disabled) return html`<lds-badge size="sm">${t("config.switchedOff")}</lds-badge>`;
  return html`<lds-badge size="sm">${t("config.inherited")}</lds-badge>`;
}

/** The icon of a status, by the outcome of its category. */
export function statusIcon(outcome) {
  return { unstarted: "circle", started: "circle-dot", completed: "circle-check", abandoned: "circle-x" }[outcome] ?? "circle-dashed";
}
