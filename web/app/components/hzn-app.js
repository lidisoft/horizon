// <hzn-app>: the standalone frame (ADR-0018). It creates the services,
// provides them to the views it mounts, and owns the sidebar, the routing
// outlet, the command palette (⌘K) and the shortcut help (?).
//
// It also guards routes: signed-out users land on /sign-in?next=…, and
// public routes (sign-in, password reset) render without the sidebar.

import { createApi } from "horizon/core/api.js";
import { provideContext } from "horizon/core/context.js";
import { html, raw, sheet } from "horizon/core/dom.js";
import { createI18n } from "horizon/core/i18n.js";
import { createRouter } from "horizon/core/router.js";
import { createSession, safeNext } from "horizon/core/session.js";
import { createShortcuts, displayKeys } from "horizon/core/shortcuts.js";
import { createStore } from "horizon/core/store.js";
import catalog from "horizon/i18n/en.json" with { type: "json" };

/**
 * Routes: name, URL pattern and the view element mounted for it. public
 * routes are for signed-out users and have no sidebar.
 */
export const ROUTES = [
  { name: "home", path: "/", view: "hzn-home" },
  { name: "account", path: "/account", view: "hzn-account" },
  { name: "sign-in", path: "/sign-in", view: "hzn-sign-in", public: true },
  { name: "forgot-password", path: "/forgot-password", view: "hzn-forgot-password", public: true },
  { name: "reset-password", path: "/reset-password", view: "hzn-reset-password", public: true },
];
const NOT_FOUND_VIEW = "hzn-not-found";
const THEME_KEY = "horizon.theme";

const styles = sheet(`
  :host { display: block; height: 100vh; }
  .frame { display: flex; height: 100%; }
  .frame[data-layout="public"] lds-nav { display: none; }
  lds-nav { flex: none; }
  .footer { display: grid; gap: var(--lds-space-1); }
  /* position: relative keeps absolutely positioned descendants (visually
     hidden labels) inside the scrolling area instead of stretching the page. */
  main { flex: 1; min-width: 0; overflow: auto; position: relative; }
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
    this.context = { api, store: createStore(), router, i18n, shortcuts, session };
    provideContext(this, this.context);

    const t = i18n.t;
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [styles];
    root.innerHTML = String(html`
      <div class="frame">
        <lds-nav aria-label="${t("nav.label")}">
          <a slot="brand" class="brand" href="/">${t("app.name")}</a>
          <lds-nav-item href="/" data-route="home"><lds-icon slot="start" name="layers"></lds-icon>${t("nav.home")}</lds-nav-item>
          <div slot="footer" class="footer">
            <lds-nav-item href="/account" data-route="account"><lds-avatar slot="start" size="sm"></lds-avatar><span data-user></span></lds-nav-item>
            <lds-button variant="ghost" size="sm" data-action="theme">
              <lds-icon slot="start" name="moon"></lds-icon><span data-label>${t("theme.dark")}</span>
            </lds-button>
            <lds-button variant="ghost" size="sm" data-action="sign-out">
              <lds-icon slot="start" name="log-out"></lds-icon>${t("nav.signOut")}
            </lds-button>
          </div>
        </lds-nav>
        <main id="outlet"></main>
      </div>
      <lds-command id="palette" aria-label="${t("command.label")}" placeholder="${t("command.placeholder")}" empty="${t("command.empty")}">
        <lds-command-group heading="${t("command.group.navigation")}">
          <lds-command-item value="home"><lds-icon slot="start" name="layers"></lds-icon>${t("shortcut.home")}</lds-command-item>
          <lds-command-item value="account"><lds-icon slot="start" name="user"></lds-icon>${t("command.account")}</lds-command-item>
        </lds-command-group>
        <lds-command-group heading="${t("command.group.general")}">
          <lds-command-item value="theme"><lds-icon slot="start" name="moon"></lds-icon>${t("shortcut.theme")}</lds-command-item>
          <lds-command-item value="help"><lds-icon slot="start" name="keyboard"></lds-icon>${t("shortcut.help")}</lds-command-item>
          <lds-command-item value="sign-out"><lds-icon slot="start" name="log-out"></lds-icon>${t("nav.signOut")}</lds-command-item>
        </lds-command-group>
      </lds-command>
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
      home: () => router.navigate(router.href("home")),
      account: () => router.navigate(router.href("account")),
      "sign-out": () => session.logout(),
      theme: () => this.toggleTheme(),
      help: () => this.showHelp(),
      palette: () => this.palette.setAttribute("open", ""),
    };
    shortcuts.register({ id: "palette", keys: "mod+k", description: "shortcut.palette", allowInInputs: true, handler: actions.palette });
    shortcuts.register({ id: "help", keys: "?", description: "shortcut.help", handler: actions.help });
    shortcuts.register({ id: "home", keys: "g h", description: "shortcut.home", handler: actions.home });

    this.palette.addEventListener("lds-select", (event) => {
      const action = actions[event.detail?.value];
      if (!action) return;
      this.palette.removeAttribute("open");
      action();
    });
    root.querySelector('[data-action="theme"]').addEventListener("click", actions.theme);
    root.querySelector('[data-action="sign-out"]').addEventListener("click", actions["sign-out"]);

    this.renderTheme();
    router.subscribe((match) => this.show(match));
    session.subscribe(() => {
      this.renderUser();
      if (router.current) this.show(router.current);
    });
    shortcuts.start();
    this.start();
  }

  /** Learns who is signed in, then starts routing. */
  async start() {
    try {
      await this.context.session.load();
    } catch {
      // Server unreachable or failing: show the page anyway. Protected
      // routes send the user to sign-in, which reports errors as they occur.
    }
    this.context.router.start();
  }

  disconnectedCallback() {
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
    this.frame.dataset.layout = match.route?.public ? "public" : "app";
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
    for (const item of this.shadowRoot.querySelectorAll("lds-nav-item[data-route]")) {
      item.toggleAttribute("active", item.dataset.route === match.route?.name);
    }
  }

  renderUser() {
    const user = this.context.session.user;
    this.shadowRoot.querySelector("[data-user]").textContent = user?.name ?? "";
    this.shadowRoot.querySelector("lds-avatar").setAttribute("name", user?.name ?? "");
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
