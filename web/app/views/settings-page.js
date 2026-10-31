// Shared layout of the workspace settings pages (wireframes: SettingsNav):
// a settings sidebar in place of the app's, and a header with the
// breadcrumb, the workspace and the page's actions.

import { html, sheet } from "horizon/core/dom.js";

export const settingsStyles = sheet(`
  :host { display: flex; min-height: 100%; }
  nav.settings { flex: none; width: 15rem; box-sizing: border-box; padding: var(--lds-space-6) var(--lds-space-4);
    background: var(--lds-color-bg-subtle); border-inline-end: 1px solid var(--lds-color-border-subtle); }
  nav.settings .back { display: flex; align-items: center; gap: var(--lds-space-2); font-weight: var(--lds-font-weight-semibold);
    color: var(--lds-color-fg-default); text-decoration: none; margin-bottom: var(--lds-space-6); }
  nav.settings h2 { font-size: var(--lds-font-size-2xs); letter-spacing: 0.08em; text-transform: uppercase;
    color: var(--lds-color-fg-muted); margin: 0 0 var(--lds-space-2) var(--lds-space-2); }
  /* LDS v0.0.5 clips label descenders (g, p, y): overflow hidden with
     line-height 1. Remove once LDS fixes lds-nav-item. */
  lds-nav-item::part(label) { line-height: 1.25; }
  .page { flex: 1; min-width: 0; }
  header.bar { display: flex; align-items: center; gap: var(--lds-space-3); padding: var(--lds-space-4) var(--lds-space-6);
    border-bottom: 1px solid var(--lds-color-border-subtle); }
  header.bar .actions { margin-inline-start: auto; display: flex; gap: var(--lds-space-3); align-items: center; }
  .content { padding: var(--lds-space-5) var(--lds-space-6) var(--lds-space-8); }
  .muted { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  lds-alert:empty { display: none; }
`);

const PAGES = [
  { name: "general", icon: "settings", label: "settings.general" },
  { name: "members", icon: "users", label: "settings.members" },
];

/** The settings frame around body; actions go to the header's end. */
export function settingsPage(t, ws, active, body, actions = "") {
  const title = t(PAGES.find((p) => p.name === active).label);
  return html`
    <nav class="settings" aria-label="${t("settings.title")}">
      <a class="back" href="/w/${ws.slug}"><lds-icon name="chevron-left"></lds-icon>${t("settings.title")}</a>
      <h2>${t("settings.workspace")}</h2>
      ${PAGES.map((p) => html`<lds-nav-item href="/w/${ws.slug}/settings/${p.name}" ${p.name === active ? html`active aria-current="page"` : ""}>
        <lds-icon slot="start" name="${p.icon}"></lds-icon>${t(p.label)}</lds-nav-item>`)}
    </nav>
    <div class="page">
      <header class="bar">
        <lds-breadcrumb>
          <lds-breadcrumb-item href="/w/${ws.slug}/settings/general">${t("settings.title")}</lds-breadcrumb-item>
          <lds-breadcrumb-item current>${title}</lds-breadcrumb-item>
        </lds-breadcrumb>
        <lds-badge size="sm">${ws.name}</lds-badge>
        <div class="actions">${actions}</div>
      </header>
      <div class="content">${body}</div>
    </div>
  `;
}
