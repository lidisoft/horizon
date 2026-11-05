// <hzn-app>: the standalone frame (ADR-0018). It creates the services,
// provides them to the views it mounts, and owns the sidebar, the routing
// outlet, the command palette (⌘K) and the shortcut help (?).
//
// It also guards routes: signed-out users land on /sign-in?next=…, and
// public routes (sign-in, password reset, invitations) render without the
// sidebar. Settings routes bring their own navigation instead of it.
//
// The sidebar follows the Sidebar wireframe: the workspace switcher and the
// account menu, search (⌘K) and New issue, the inbox and my issues, each
// project with its pages, the saved views, then members and settings. The
// switcher lists the user's workspaces; the last one opened is remembered,
// and "/" goes there.

import "horizon/components/hzn-issue-create.js";
import { ISSUE_KEY, issueHref } from "horizon/components/issue-ui.js";
import { paintSwatches } from "horizon/components/project-colour.js";
import { createApi } from "horizon/core/api.js";
import { provideContext } from "horizon/core/context.js";
import { html, raw, sheet } from "horizon/core/dom.js";
import { createI18n } from "horizon/core/i18n.js";
import { createRouter } from "horizon/core/router.js";
import { createSession, safeNext } from "horizon/core/session.js";
import { createShortcuts, displayKeys } from "horizon/core/shortcuts.js";
import { createSocket } from "horizon/core/socket.js";
import { createStore } from "horizon/core/store.js";
import catalog from "horizon/i18n/en.json" with { type: "json" };

/**
 * Routes: name, URL pattern and the view element mounted for it. public
 * routes work signed out and have no sidebar; settings routes have their own.
 */
export const ROUTES = [
  { name: "home", path: "/", view: "hzn-home" },
  { name: "account", path: "/account", view: "hzn-account" },
  { name: "workspace", path: "/w/:ws", view: "hzn-workspace-home" },
  { name: "settings-general", path: "/w/:ws/settings/general", view: "hzn-settings-general", layout: "settings" },
  { name: "settings-members", path: "/w/:ws/settings/members", view: "hzn-settings-members", layout: "settings" },
  { name: "settings-roles", path: "/w/:ws/settings/roles", view: "hzn-settings-roles", layout: "settings" },
  { name: "settings-authentication", path: "/w/:ws/settings/authentication", view: "hzn-settings-sso", layout: "settings" },
  { name: "settings-projects", path: "/w/:ws/settings/projects", view: "hzn-settings-projects", layout: "settings" },
  { name: "settings-project", path: "/w/:ws/settings/projects/:key", view: "hzn-settings-project", layout: "settings" },
  { name: "project", path: "/w/:ws/p/:key", view: "hzn-project-home" },
  { name: "project-issues", path: "/w/:ws/p/:key/issues", view: "hzn-issue-list" },
  { name: "project-board", path: "/w/:ws/p/:key/board", view: "hzn-board" },
  { name: "project-table", path: "/w/:ws/p/:key/table", view: "hzn-table" },
  { name: "project-timeline", path: "/w/:ws/p/:key/timeline", view: "hzn-timeline" },
  { name: "project-calendar", path: "/w/:ws/p/:key/calendar", view: "hzn-calendar" },
  { name: "project-reports", path: "/w/:ws/p/:key/reports", view: "hzn-reports" },
  { name: "project-backlog", path: "/w/:ws/p/:key/backlog", view: "hzn-backlog" },
  { name: "issue", path: "/w/:ws/issues/:issue", view: "hzn-issue" },
  { name: "search", path: "/w/:ws/search", view: "hzn-search" },
  { name: "inbox", path: "/w/:ws/inbox", view: "hzn-inbox" },
  { name: "milestones", path: "/w/:ws/milestones", view: "hzn-milestones" },
  { name: "roadmap", path: "/w/:ws/roadmap", view: "hzn-roadmap" },
  { name: "project-releases", path: "/w/:ws/p/:key/releases", view: "hzn-releases" },
  { name: "settings-issue-types", path: "/w/:ws/settings/issue-types", view: "hzn-settings-issue-types", layout: "settings" },
  { name: "settings-workflows", path: "/w/:ws/settings/workflows", view: "hzn-settings-workflows", layout: "settings" },
  { name: "settings-categories", path: "/w/:ws/settings/categories", view: "hzn-settings-categories", layout: "settings" },
  { name: "settings-labels", path: "/w/:ws/settings/labels", view: "hzn-settings-labels", layout: "settings" },
  { name: "settings-estimation", path: "/w/:ws/settings/estimation", view: "hzn-settings-estimation", layout: "settings" },
  { name: "settings-iterations", path: "/w/:ws/settings/iterations", view: "hzn-settings-iterations", layout: "settings" },
  { name: "settings-calendar", path: "/w/:ws/settings/calendar", view: "hzn-settings-calendar", layout: "settings" },
  { name: "settings-webhooks", path: "/w/:ws/settings/webhooks", view: "hzn-settings-webhooks", layout: "settings" },
  { name: "settings-webhook", path: "/w/:ws/settings/webhooks/:id", view: "hzn-settings-webhooks", layout: "settings" },
  { name: "settings-git", path: "/w/:ws/settings/git", view: "hzn-settings-git", layout: "settings" },
  { name: "invitation", path: "/invitation", view: "hzn-invitation", public: true },
  { name: "sign-in", path: "/sign-in", view: "hzn-sign-in", public: true },
  { name: "forgot-password", path: "/forgot-password", view: "hzn-forgot-password", public: true },
  { name: "reset-password", path: "/reset-password", view: "hzn-reset-password", public: true },
  { name: "magic-link", path: "/magic-link", view: "hzn-magic-link", public: true },
];
const NOT_FOUND_VIEW = "hzn-not-found";
/** A project's pages in the sidebar, under its name (wireframes: Sidebar). */
const PROJECT_PAGES = [
  { route: "project", path: "", icon: "eye", label: "planning.layouts.overview" },
  { route: "project-issues", path: "/issues", icon: "list", label: "nav.issues" },
  { route: "project-board", path: "/board", icon: "kanban", label: "planning.layouts.board" },
  { route: "project-backlog", path: "/backlog", icon: "layers", label: "planning.layouts.backlog" },
  { route: "project-releases", path: "/releases", icon: "rocket", label: "planning.layouts.releases" },
  { route: "project-reports", path: "/reports", icon: "chart-column", label: "planning.layouts.reports" },
];
/** Where a saved view's layout lives under /w/:ws/p/:key/. */
const LAYOUT_PATHS = { list: "issues", board: "board", table: "table", timeline: "timeline", calendar: "calendar" };
const OUTCOME_ICONS = { unstarted: "circle", started: "circle-dot", completed: "circle-check", abandoned: "circle-x" };
const THEME_KEY = "horizon.theme";
const WORKSPACE_KEY = "horizon.workspace";

/** The workspace the user opened last, if stored. */
export function rememberedWorkspace() {
  try {
    return localStorage.getItem(WORKSPACE_KEY);
  } catch {
    return null;
  }
}

function rememberWorkspace(slug) {
  try {
    localStorage.setItem(WORKSPACE_KEY, slug);
  } catch {
    // storage blocked: "/" picks the first workspace instead
  }
}

const styles = sheet(`
  :host { display: block; height: 100vh; }
  .frame { display: flex; height: 100%; }
  .frame:is([data-layout="public"], [data-layout="settings"]) lds-nav { display: none; }
  lds-nav { flex: none; --lds-nav-width: 15rem; }
  .top { display: flex; flex-direction: column; gap: var(--lds-space-3); inline-size: 100%; }
  .top .row { display: flex; align-items: center; justify-content: space-between; gap: var(--lds-space-2); min-inline-size: 0; }
  .switcher { min-inline-size: 0; }
  .switcher lds-button { max-inline-size: 100%; }
  .switcher [data-ws-name] { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: var(--lds-font-weight-semibold); }
  .tile { display: inline-flex; inline-size: 1.375rem; block-size: 1.375rem; flex: none; align-items: center; justify-content: center;
    border-radius: var(--lds-radius-md); background: var(--lds-color-accent-default); color: var(--lds-color-fg-on-accent);
    font-family: var(--lds-font-family-display); font-weight: var(--lds-font-weight-bold); font-size: var(--lds-font-size-xs); }
  button.me { display: inline-flex; padding: 0; border: 0; border-radius: var(--lds-radius-full); background: none; cursor: pointer; }
  button.me:focus-visible { outline: 2px solid var(--lds-color-focus-ring, var(--lds-color-accent-default)); outline-offset: 2px; }
  .quick { display: flex; gap: var(--lds-space-2); }
  .quick[hidden] { display: none; }
  button.search { flex: 1; display: flex; align-items: center; gap: var(--lds-space-2); block-size: 2rem; min-inline-size: 0;
    padding: 0 var(--lds-space-2) 0 var(--lds-space-3); border: 1px solid var(--lds-color-border-default); border-radius: var(--lds-radius-md);
    background: var(--lds-color-bg-canvas); color: var(--lds-color-fg-muted); font: inherit; font-size: var(--lds-font-size-sm); cursor: pointer; }
  button.search:hover { border-color: var(--lds-color-border-strong, var(--lds-color-border-default)); color: var(--lds-color-fg-default); }
  button.search span { flex: 1; text-align: start; }
  .section { margin: var(--lds-space-4) 0 var(--lds-space-1) var(--lds-space-2); display: flex; align-items: center; justify-content: space-between;
    font-size: var(--lds-font-size-2xs); letter-spacing: 0.08em; text-transform: uppercase; color: var(--lds-color-fg-muted); }
  .dot { inline-size: 0.625rem; block-size: 0.625rem; border-radius: var(--lds-radius-xs); }
  .project-key, .count { font-size: var(--lds-font-size-xs); color: var(--lds-color-fg-muted); }
  .count[hidden] { display: none; }
  /* position: relative keeps absolutely positioned descendants (visually
     hidden labels) inside the scrolling area instead of stretching the page.
     Views read at the wireframes' 14px. */
  main { flex: 1; min-width: 0; overflow: auto; position: relative; font-size: var(--lds-font-size-sm); }
  .brand {
    font-family: var(--lds-font-family-display);
    font-weight: var(--lds-font-weight-semibold);
    color: inherit;
    text-decoration: none;
  }
  .help { display: grid; grid-template-columns: 1fr auto; gap: var(--lds-space-3) var(--lds-space-8); margin: 0; }
  .help dt { color: var(--lds-color-fg-default); }
  .help dd { display: flex; align-items: center; gap: var(--lds-space-1); margin: 0; }
  .then { font-size: var(--lds-font-size-xs); color: var(--lds-color-fg-muted); }
`);

/** Theme the user chose, else the operating system's. */
export function currentTheme() {
  return document.documentElement.getAttribute("data-lds-theme") ||
    (matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
}

/** Applies the stored theme before the first render. */
export function applyStoredTheme() {
  let stored = null;
  try {
    stored = localStorage.getItem(THEME_KEY);
  } catch {
    // storage blocked: follow the operating system
  }
  if (stored === "dark" || stored === "light") globalThis.LDS?.setTheme(stored);
}

export class HznApp extends HTMLElement {
  connectedCallback() {
    if (this.context) return;
    const i18n = createI18n(catalog);
    const router = createRouter(ROUTES);
    const shortcuts = createShortcuts();
    const session = createSession({ api: () => api });
    const api = createApi({
      csrfToken: () => session.csrf,
      onUnauthenticated: () => session.ended(),
      refreshCsrf: () => session.load().then(() => {}),
    });
    this.workspaces = [];
    const store = createStore();
    // Domain events pushed for the current workspace (docs/realtime.md).
    const socket = createSocket({ store });
    this.context = {
      api, store, socket, router, i18n, shortcuts, session, commands: createCommands(),
      // Views call refresh after creating, renaming or joining a workspace.
      workspaces: { list: () => this.workspaces, refresh: () => this.loadWorkspaces() },
      // Views call refresh after creating, renaming or archiving a project.
      projects: { refresh: () => this.loadProjects() },
    };
    provideContext(this, this.context);

    const t = i18n.t;
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [styles];
    root.innerHTML = String(html`
      <div class="frame">
        <lds-nav aria-label="${t("nav.label")}">
          <div slot="brand" class="top">
            <div class="row">
              <a class="brand" href="/" data-brand>${t("app.name")}</a>
              <lds-menu placement="bottom-start" class="switcher" hidden data-switcher>
                <lds-button slot="trigger" variant="ghost" size="sm" label="${t("nav.switchWorkspace")}">
                  <span slot="start" class="tile" data-ws-tile></span><span data-ws-name></span><lds-icon slot="end" name="chevron-down" size="sm"></lds-icon>
                </lds-button>
              </lds-menu>
              <lds-menu placement="bottom-end" data-account>
                <button slot="trigger" type="button" class="me" aria-label="${t("nav.account")}"><lds-avatar size="sm" data-me></lds-avatar></button>
                <lds-menu-item href="/account" data-route="account"><lds-icon slot="start" name="user"></lds-icon><span data-user></span></lds-menu-item>
                <hr>
                <lds-menu-item data-action="theme"><lds-icon slot="start" name="moon"></lds-icon><span data-label>${t("theme.dark")}</span></lds-menu-item>
                <lds-menu-item data-action="sign-out"><lds-icon slot="start" name="log-out"></lds-icon>${t("nav.signOut")}</lds-menu-item>
              </lds-menu>
            </div>
            <div class="quick" data-quick hidden>
              <button type="button" class="search" data-action="palette">
                <lds-icon name="search" size="sm"></lds-icon><span>${t("nav.search")}</span><lds-kbd>${displayKeys("mod+k").join("")}</lds-kbd>
              </button>
              <lds-tooltip content="${t("issues.new")}">
                <lds-button variant="secondary" size="sm" label="${t("issues.new")}" data-action="create-issue"><lds-icon name="plus" size="sm"></lds-icon></lds-button>
              </lds-tooltip>
            </div>
          </div>
          <lds-nav-item href="/" data-route="inbox" data-nav="inbox" hidden><lds-icon slot="start" name="inbox"></lds-icon>${t("nav.inbox")}<span slot="end" class="count" data-unread hidden></span></lds-nav-item>
          <lds-nav-item href="/" data-route="workspace" data-nav="my-issues" hidden><lds-icon slot="start" name="user"></lds-icon>${t("nav.myIssues")}</lds-nav-item>
          <lds-nav-item href="/" data-route="roadmap" data-nav="roadmap" hidden><lds-icon slot="start" name="chart-gantt"></lds-icon>${t("nav.roadmap")}</lds-nav-item>
          <lds-nav-item href="/" data-route="milestones" data-nav="milestones" hidden><lds-icon slot="start" name="flag"></lds-icon>${t("nav.milestones")}</lds-nav-item>
          <div data-projects></div>
          <div data-views></div>
          <lds-nav-item slot="footer" href="/" data-route="settings-members" data-nav="members" hidden><lds-icon slot="start" name="users"></lds-icon>${t("nav.members")}</lds-nav-item>
          <lds-nav-item slot="footer" href="/" data-route="settings" data-nav="settings" hidden><lds-icon slot="start" name="settings"></lds-icon>${t("nav.settings")}</lds-nav-item>
        </lds-nav>
        <main id="outlet"></main>
      </div>
      <lds-command id="palette" aria-label="${t("command.label")}" placeholder="${t("command.placeholder")}" empty="${t("command.empty")}">
        <div data-dynamic></div>
        <lds-command-group heading="${t("command.group.issues")}" data-goto hidden></lds-command-group>
        <lds-command-group heading="${t("command.group.navigation")}">
          <lds-command-item value="create-issue" keywords="new"><lds-icon slot="start" name="plus"></lds-icon>${t("command.createIssue")}<lds-kbd slot="end">C</lds-kbd></lds-command-item>
          <lds-command-item value="home"><lds-icon slot="start" name="layers"></lds-icon>${t("shortcut.home")}</lds-command-item>
          <lds-command-item value="account"><lds-icon slot="start" name="user"></lds-icon>${t("command.account")}</lds-command-item>
        </lds-command-group>
        <lds-command-group heading="${t("command.group.general")}">
          <lds-command-item value="theme"><lds-icon slot="start" name="moon"></lds-icon>${t("shortcut.theme")}</lds-command-item>
          <lds-command-item value="help"><lds-icon slot="start" name="keyboard"></lds-icon>${t("shortcut.help")}</lds-command-item>
          <lds-command-item value="sign-out"><lds-icon slot="start" name="log-out"></lds-icon>${t("nav.signOut")}</lds-command-item>
        </lds-command-group>
      </lds-command>
      <hzn-issue-create></hzn-issue-create>
      <lds-modal id="help" size="sm" dismissible>
        <span slot="header">${t("shortcuts.title")}</span>
        <dl class="help"></dl>
      </lds-modal>
    `);
    this.frame = root.querySelector(".frame");
    this.outlet = root.getElementById("outlet");
    this.palette = root.getElementById("palette");
    this.help = root.getElementById("help");

    const actions = {
      home: () => router.navigate(this.currentSlug ? `/w/${this.currentSlug}` : "/"),
      inbox: () => this.currentSlug && router.navigate(`/w/${this.currentSlug}/inbox`),
      account: () => router.navigate(router.href("account")),
      "sign-out": () => session.logout(),
      theme: () => this.toggleTheme(),
      help: () => this.showHelp(),
      palette: () => this.openPalette(),
      "create-issue": () => this.createIssue(),
    };
    shortcuts.register({ id: "palette", keys: "mod+k", description: "shortcut.palette", allowInInputs: true, handler: actions.palette });
    shortcuts.register({ id: "help", keys: "?", description: "shortcut.help", handler: actions.help });
    shortcuts.register({ id: "home", keys: "g h", description: "shortcut.home", handler: actions.home });
    shortcuts.register({ id: "inbox", keys: "g i", description: "shortcut.inbox", handler: actions.inbox });
    shortcuts.register({ id: "create-issue", keys: "c", description: "shortcut.createIssue", handler: actions["create-issue"] });

    // Commands of the open issue, and "Open <KEY>" for a typed issue key.
    // While it is open, keys belong to it, not to the page's shortcuts.
    this.palette.addEventListener("lds-open", (event) => event.target === this.palette && this.renderCommands());
    new MutationObserver(() => {
      const open = this.palette.hasAttribute("open");
      if (open && !this.resumeShortcuts) this.resumeShortcuts = shortcuts.pause();
      if (!open && this.resumeShortcuts) {
        this.resumeShortcuts();
        this.resumeShortcuts = null;
      }
    }).observe(this.palette, { attributes: true, attributeFilter: ["open"] });
    this.palette.addEventListener("input", (event) => this.searchPalette(event.composedPath()[0]?.value ?? ""));

    this.palette.addEventListener("lds-select", (event) => {
      const value = event.detail?.value ?? "";
      if (value.startsWith("cmd:") || value.startsWith("goto:") || value.startsWith("search:")) {
        this.palette.removeAttribute("open");
        if (value.startsWith("goto:")) router.navigate(issueHref(this.currentSlug, value.slice(5)));
        else if (value.startsWith("search:")) router.navigate(`/w/${this.currentSlug}/search?q=${encodeURIComponent(value.slice(7))}`);
        else this.commandRuns.get(value)?.();
        return;
      }
      const action = actions[value];
      if (!action) return;
      this.palette.removeAttribute("open");
      action();
    });
    for (const name of ["theme", "sign-out", "palette", "create-issue"]) {
      root.querySelector(`[data-action="${name}"]`).addEventListener("click", actions[name]);
    }

    this.renderTheme();
    router.subscribe((match) => this.show(match));
    socket.on((m) => {
      // Another tab or person created, renamed or archived a project.
      if (m.t === "event" && m.d.aggregate?.type === "project") this.loadProjects();
      // The inbox changed (here or in another tab), or the socket came back.
      if ((m.t === "inbox" && m.d?.workspace === this.currentSlug) || m.t === "open") this.loadUnread();
    });
    session.subscribe(async (state) => {
      this.renderUser();
      this.syncSocket();
      // The workspaces come first: "/" picks one of them.
      if (state.status === "signed-in") await this.loadWorkspaces();
      else this.workspaces = [];
      if (router.current) this.show(router.current);
    });
    shortcuts.start();
    this.start();
  }

  /** Learns who is signed in, then starts routing. */
  async start() {
    try {
      await this.context.session.load();
      if (this.context.session.status === "signed-in") await this.loadWorkspaces();
      this.syncSocket();
    } catch {
      // Server unreachable or failing: show the page anyway. Protected
      // routes send the user to sign-in, which reports errors as they occur.
    }
    this.context.router.start();
  }

  /**
   * Opens ⌘K with its input focused straight away (lds-command v0.0.5
   * focuses it a frame later).
   */
  openPalette() {
    this.palette.setAttribute("open", "");
    this.palette.shadowRoot?.querySelector("input")?.focus();
  }

  /** Renders the commands views registered, for the palette about to open. */
  renderCommands() {
    const { commands } = this.context;
    this.commandRuns = new Map();
    const box = this.palette.querySelector("[data-dynamic]");
    box.innerHTML = String(html`${commands.list().map(([id, group]) => html`
      <lds-command-group heading="${group.heading}">${group.items.map((item, i) => {
        const value = `cmd:${id}:${i}`;
        this.commandRuns.set(value, item.run);
        return html`<lds-command-item value="${value}"><lds-icon slot="start" name="${item.icon}"></lds-icon>${item.label}
          ${item.keys ? html`<lds-kbd slot="end">${displayKeys(item.keys).join(" ")}</lds-kbd>` : ""}</lds-command-item>`;
      })}</lds-command-group>`)}`);
    this.searchPalette("");
  }

  /**
   * Issues for what is typed: a typed key at once, then the search's
   * matches. Items carry the query as keywords so the palette's own
   * filter keeps them.
   */
  searchPalette(query) {
    const { api, i18n } = this.context;
    const group = this.palette.querySelector("[data-goto]");
    const q = query.trim();
    clearTimeout(this.searchTimer);
    const seq = (this.searchSeq = (this.searchSeq ?? 0) + 1);
    const key = ISSUE_KEY.test(q) ? q.toUpperCase() : "";
    const render = (items) => {
      group.hidden = !this.currentSlug || !q;
      const keywords = `${q} ${q.toLowerCase()}`;
      group.innerHTML = group.hidden ? "" : String(html`
        ${key && !items.some((r) => r.key === key) ? html`<lds-command-item value="goto:${key}" keywords="${keywords}">
          <lds-icon slot="start" name="search"></lds-icon>${i18n.t("command.openIssue", { key })}</lds-command-item>` : ""}
        ${items.map((r) => html`<lds-command-item value="goto:${r.key}" keywords="${keywords}">
          <lds-icon slot="start" name="${OUTCOME_ICONS[r.status.outcome] ?? "circle"}"></lds-icon>${r.key} ${r.title}</lds-command-item>`)}
        <lds-command-item value="search:${q}" keywords="${keywords}"><lds-icon slot="start" name="search"></lds-icon>${i18n.t("command.searchFor", { q })}</lds-command-item>`);
    };
    render([]);
    if (!q || !this.currentSlug) return;
    // A failed search is tried once more; then the typed key and "Search
    // for" stay. The last error is kept for diagnosis.
    const search = async (attempt) => {
      try {
        const { data } = await api.get(`/w/${encodeURIComponent(this.currentSlug)}/search`, { query: { q, limit: 8 } });
        if (seq !== this.searchSeq) return; // typed on meanwhile
        render(data.items);
      } catch (err) {
        this.lastSearchError = String(err?.message ?? err);
        if (attempt === 0 && seq === this.searchSeq) this.searchTimer = setTimeout(() => search(1), 300);
      }
    };
    this.searchTimer = setTimeout(() => search(0), 150);
  }

  /** The New issue dialog, in the current project if any. */
  createIssue() {
    if (!this.currentSlug || this.context.session.status !== "signed-in") return;
    this.shadowRoot.querySelector("hzn-issue-create").show({ ws: this.currentSlug, project: this.currentProject });
  }

  /** Keeps the socket open while signed in, watching the current workspace. */
  syncSocket() {
    const { socket, session } = this.context;
    if (session.status !== "signed-in") {
      socket.stop();
      return;
    }
    socket.start();
    const want = this.currentSlug ? [`ws:${this.currentSlug}`] : [];
    socket.unsubscribe(socket.topics().filter((t) => !want.includes(t)));
    socket.subscribe(want);
  }

  disconnectedCallback() {
    this.context?.socket.stop();
    this.context?.shortcuts.stop();
    this.context?.router.stop();
  }

  /**
   * Mounts the view for a route, reusing it when only its parameters change.
   * Redirects signed-out users to sign-in, and signed-in users away from it.
   */
  show(match) {
    const { session, router } = this.context;
    const signedIn = session.status === "signed-in";
    if (!signedIn && !match.route?.public) {
      router.navigate(`${router.href("sign-in")}?next=${encodeURIComponent(match.path)}`, { replace: true });
      return;
    }
    if (signedIn && match.route?.name === "sign-in") {
      router.navigate(safeNext(match.query.get("next")), { replace: true });
      return;
    }
    this.frame.dataset.layout = match.route?.public ? "public" : match.route?.layout || "app";
    if (match.params.ws) {
      const changed = this.currentSlug !== match.params.ws;
      this.currentSlug = match.params.ws;
      rememberWorkspace(match.params.ws);
      this.renderSwitcher();
      if (changed) {
        this.loadProjects();
        this.loadUnread();
        this.syncSocket();
      }
    }
    // An issue's page belongs to its project (by its key's prefix).
    this.currentProject = (match.params.key ?? match.params.issue?.replace(/-\d+$/, ""))?.toUpperCase();
    this.renderViews();
    this.markProject();
    const tag = match.route ? match.route.view : NOT_FOUND_VIEW;
    const params = match.route ? match.params : { path: match.path };
    let view = this.outlet.firstElementChild;
    if (!view || view.localName !== tag) {
      view = document.createElement(tag);
      for (const [k, v] of Object.entries(params)) view.setAttribute(k, v);
      this.outlet.replaceChildren(view);
    } else {
      for (const [k, v] of Object.entries(params)) view.setAttribute(k, v);
    }
    const name = match.route?.name;
    const section = name !== "settings-members" && name?.startsWith("settings-") ? "settings" : name;
    if (match.route?.name !== "issue") this.context.commands.clear("issue");
    for (const item of this.shadowRoot.querySelectorAll("lds-nav-item[data-route]")) {
      item.toggleAttribute("active", item.dataset.route === section);
    }
  }

  /** Reloads the user's workspaces for the switcher. */
  async loadWorkspaces() {
    try {
      const { data } = await this.context.api.get("/w");
      this.workspaces = data.items;
    } catch {
      this.workspaces = [];
    }
    this.renderSwitcher();
    return this.workspaces;
  }

  /** Shows the current workspace's unread inbox rows on the sidebar. */
  async loadUnread() {
    const slug = this.currentSlug;
    const badge = this.shadowRoot.querySelector("[data-unread]");
    let unread = 0;
    if (slug && this.context.session.status === "signed-in") {
      try {
        ({ data: { unread } } = await this.context.api.get(`/w/${encodeURIComponent(slug)}/inbox/count`));
      } catch {
        // not a member, or offline: no badge
      }
    }
    if (slug !== this.currentSlug) return;
    badge.textContent = unread > 99 ? "99+" : String(unread);
    badge.hidden = !unread;
    badge.closest("lds-nav-item").setAttribute("aria-label", unread ? this.context.i18n.t("nav.inboxUnread", { count: unread }) : this.context.i18n.t("nav.inbox"));
  }

  /** Loads the current workspace's projects and saved views for the sidebar. */
  async loadProjects() {
    const slug = this.currentSlug;
    const box = this.shadowRoot.querySelector("[data-projects]");
    const viewsBox = this.shadowRoot.querySelector("[data-views]");
    if (!slug) {
      box.replaceChildren();
      viewsBox.replaceChildren();
      return;
    }
    let projects = [];
    let can = [];
    let views = [];
    try {
      const enc = encodeURIComponent(slug);
      const [list, ws, saved] = await Promise.all([
        this.context.api.get(`/w/${enc}/projects`), this.context.api.get(`/w/${enc}`),
        this.context.api.get(`/w/${enc}/views`).catch(() => ({ data: { items: [] } })),
      ]);
      projects = list.data.items.filter((p) => !p.archived_at);
      can = ws.data.can;
      views = saved.data.items;
    } catch {
      // not a member (yet), or offline: no project list
    }
    if (slug !== this.currentSlug) return; // the user moved on meanwhile
    this.projects = projects;
    this.views = views;
    const { t } = this.context.i18n;
    box.innerHTML = String(html`
      ${projects.length || can.includes("project.create") ? html`
        <div class="section">${t("nav.projects")}
          ${can.includes("project.create") ? html`<lds-button size="sm" variant="ghost" label="${t("projects.new")}" data-new-project><lds-icon name="plus" size="sm"></lds-icon></lds-button>` : ""}
        </div>` : ""}
      ${projects.map((p) => html`
        <lds-nav-group label="${p.name}" href="/w/${slug}/p/${p.key}" data-project="${p.key}">
          <span slot="start" class="dot" data-colour="${p.colour}"></span><span slot="end" class="project-key">${p.key}</span>
          ${PROJECT_PAGES.map((page) => html`<lds-nav-item href="/w/${slug}/p/${p.key}${page.path}" data-page="${page.route}">
            <lds-icon slot="start" name="${page.icon}"></lds-icon>${t(page.label)}</lds-nav-item>`)}
        </lds-nav-group>`)}`);
    paintSwatches(box);
    box.querySelector("[data-new-project]")?.addEventListener("click", () => this.context.router.navigate(`/w/${slug}/settings/projects?new=1`));
    this.renderViews();
    this.markProject();
  }

  /**
   * The saved views, each opening its layout in its project (a workspace
   * view: in the current project, else the first).
   */
  renderViews() {
    const slug = this.currentSlug;
    const box = this.shadowRoot.querySelector("[data-views]");
    const byID = new Map((this.projects ?? []).map((p) => [p.id, p]));
    const fallback = (this.projects ?? []).find((p) => p.key === this.currentProject) ?? this.projects?.[0];
    const links = (this.views ?? []).flatMap((v) => {
      const p = v.project_id ? byID.get(v.project_id) : fallback;
      return p && LAYOUT_PATHS[v.layout] ? [{ v, href: `/w/${slug}/p/${p.key}/${LAYOUT_PATHS[v.layout]}?view=${v.id}` }] : [];
    });
    box.innerHTML = links.length ? String(html`
      <lds-nav-group label="${this.context.i18n.t("nav.views")}" open>
        ${links.map(({ v, href }) => html`<lds-nav-item href="${href}" data-view="${v.id}"><lds-icon slot="start" name="bookmark"></lds-icon>${v.name}</lds-nav-item>`)}
      </lds-nav-group>`) : "";
  }

  markProject() {
    const route = this.context.router.current?.route?.name;
    const viewID = this.context.router.current?.query?.get("view");
    for (const group of this.shadowRoot.querySelectorAll("lds-nav-group[data-project]")) {
      const here = group.dataset.project === this.currentProject;
      if (here) group.setAttribute("open", "");
      for (const item of group.querySelectorAll("lds-nav-item")) {
        item.toggleAttribute("active", here && !viewID && (item.dataset.page === route || (route === "issue" && item.dataset.page === "project-issues")));
      }
    }
    for (const item of this.shadowRoot.querySelectorAll("lds-nav-item[data-view]")) item.toggleAttribute("active", item.dataset.view === viewID);
  }

  renderSwitcher() {
    const { t } = this.context.i18n;
    const root = this.shadowRoot;
    const slug = this.currentSlug && this.workspaces.some((w) => w.slug === this.currentSlug)
      ? this.currentSlug
      : this.workspaces[0]?.slug;
    const current = this.workspaces.find((w) => w.slug === slug);
    const menu = root.querySelector("[data-switcher]");
    menu.hidden = !current;
    root.querySelector("[data-brand]").hidden = !!current;
    for (const item of menu.querySelectorAll("lds-menu-item, hr")) item.remove();
    if (current) {
      root.querySelector("[data-ws-name]").textContent = current.name;
      menu.insertAdjacentHTML("beforeend", String(html`
        ${this.workspaces.map((w) => html`<lds-menu-item href="/w/${w.slug}">${w.slug === current.slug ? html`<lds-icon slot="start" name="check"></lds-icon>` : ""}${w.name}</lds-menu-item>`)}
        <hr>
        <lds-menu-item href="/w/${current.slug}/settings/general"><lds-icon slot="start" name="settings"></lds-icon>${t("nav.workspaceSettings")}</lds-menu-item>
      `));
    }
    const base = current ? `/w/${current.slug}` : "";
    for (const [nav, path] of Object.entries({ inbox: "/inbox", "my-issues": "", roadmap: "/roadmap", milestones: "/milestones", members: "/settings/members", settings: "/settings/general" })) {
      const item = root.querySelector(`[data-nav="${nav}"]`);
      item.hidden = !current;
      if (current) item.setAttribute("href", base + path);
    }
    root.querySelector("[data-quick]").hidden = !current;
    root.querySelector("[data-ws-tile]").textContent = current ? [...current.name.trim()][0]?.toUpperCase() ?? "" : "";
  }

  renderUser() {
    const user = this.context.session.user;
    this.shadowRoot.querySelector("[data-user]").textContent = user?.name ?? "";
    this.shadowRoot.querySelector("[data-me]").setAttribute("name", user?.name ?? "");
  }

  toggleTheme() {
    const next = currentTheme() === "dark" ? "light" : "dark";
    globalThis.LDS?.setTheme(next);
    try {
      localStorage.setItem(THEME_KEY, next);
    } catch {
      // storage blocked: the choice lasts for this page only
    }
    this.renderTheme();
  }

  renderTheme() {
    const dark = currentTheme() === "dark";
    const button = this.shadowRoot.querySelector('[data-action="theme"]');
    button.querySelector("lds-icon").setAttribute("name", dark ? "sun" : "moon");
    button.querySelector("[data-label]").textContent = this.context.i18n.t(dark ? "theme.light" : "theme.dark");
  }

  showHelp() {
    const { i18n, shortcuts } = this.context;
    const then = String(html`<span class="then">${i18n.t("shortcuts.then")}</span>`);
    this.help.querySelector(".help").innerHTML = shortcuts
      .list()
      .map((s) => {
        // Spaces between keys keep the text readable ("G then H") for
        // assistive technology; flex layout ignores them.
        const keys = displayKeys(s.keys).map((k) => String(html`<lds-kbd>${k}</lds-kbd>`)).join(` ${then} `);
        return String(html`<dt>${i18n.t(s.description)}</dt><dd>${raw(keys)}</dd>`);
      })
      .join("");
    this.help.setAttribute("open", "");
  }
}

/** Commands views add to the palette while they're shown, by group id. */
function createCommands() {
  const groups = new Map();
  return {
    /** Sets a group ({ heading, items: [{ label, icon, keys?, run }] }); returns its remover. */
    set(id, group) {
      groups.set(id, group);
      return () => groups.get(id) === group && groups.delete(id);
    },
    clear(id) {
      groups.delete(id);
    },
    list() {
      return [...groups.entries()];
    },
  };
}
