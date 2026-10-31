// History API router. Routes map URL patterns to view elements:
//
//   { name: "issue", path: "/w/:workspace/issues/:key", view: "hzn-issue-detail" }
//
// Same-origin link clicks are handled without a page load, including links
// inside shadow roots (found through composedPath).

/**
 * @typedef {{ name: string, path: string, view: string }} Route
 * @typedef {{ route: Route|null, params: Record<string,string>, query: URLSearchParams, path: string }} Match
 */

function compile(path) {
  const names = [];
  const pattern = path
    .split("/")
    .map((part) => {
      if (part.startsWith(":")) {
        names.push(part.slice(1));
        return "([^/]+)";
      }
      return part.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    })
    .join("/");
  return { regex: new RegExp(`^${pattern}/?$`), names };
}

/**
 * @param {Route[]} routes first match wins
 * @param {object} [options]
 * @param {Window} [options.window] for tests
 */
export function createRouter(routes, { window: win = globalThis.window } = {}) {
  const compiled = routes.map((route) => ({ route, ...compile(route.path) }));
  const listeners = new Set();
  let current = null;

  /** @returns {Match} */
  function match(url) {
    const u = new URL(url, win.location.origin);
    for (const { route, regex, names } of compiled) {
      const m = regex.exec(u.pathname);
      if (m) {
        const params = {};
        names.forEach((n, i) => (params[n] = decodeURIComponent(m[i + 1])));
        return { route, params, query: u.searchParams, path: u.pathname + u.search };
      }
    }
    return { route: null, params: {}, query: u.searchParams, path: u.pathname + u.search };
  }

  function update() {
    current = match(win.location.href);
    for (const fn of [...listeners]) fn(current);
  }

  function onClick(event) {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
      return;
    }
    const a = event.composedPath().find((n) => n instanceof win.HTMLAnchorElement && n.hasAttribute("href"));
    if (!a || (a.target && a.target !== "_self") || a.hasAttribute("download")) return;
    const url = new URL(a.href, win.location.href);
    if (url.origin !== win.location.origin) return;
    if (url.pathname === win.location.pathname && url.search === win.location.search && url.hash) return;
    if (!match(url).route) return; // server pages such as /swagger/
    event.preventDefault();
    router.navigate(url.pathname + url.search + url.hash);
  }

  const router = {
    /** Starts listening to history and link clicks. */
    start() {
      win.addEventListener("popstate", update);
      win.document.addEventListener("click", onClick);
      update();
      return router;
    },

    stop() {
      win.removeEventListener("popstate", update);
      win.document.removeEventListener("click", onClick);
    },

    /** @returns {Match} */
    get current() {
      return current;
    },

    match,

    navigate(path, { replace = false } = {}) {
      if (replace) win.history.replaceState(null, "", path);
      else win.history.pushState(null, "", path);
      update();
      win.scrollTo?.(0, 0);
    },

    /** Builds a path from a route name and parameters. */
    href(name, params = {}) {
      const route = routes.find((r) => r.name === name);
      if (!route) throw new Error(`router: no route named ${name}`);
      return route.path.replace(/:([A-Za-z_]\w*)/g, (_, k) => {
        if (params[k] == null) throw new Error(`router: ${name} needs ${k}`);
        return encodeURIComponent(params[k]);
      });
    },

    /** Calls fn with the new match on every navigation. */
    subscribe(fn) {
      listeners.add(fn);
      return () => listeners.delete(fn);
    },
  };
  return router;
}
