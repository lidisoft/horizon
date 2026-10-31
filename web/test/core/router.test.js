import { createRouter } from "horizon/core/router.js";
import { assert, describe, test } from "horizon-test/runner.js";

const ROUTES = [
  { name: "home", path: "/", view: "hzn-home" },
  { name: "issue", path: "/w/:workspace/issues/:key", view: "hzn-issue" },
];

/** A window double: real location parsing, recorded history, a DOM root for clicks. */
function fakeWindow(start = location.origin + "/") {
  const root = document.createElement("div");
  document.getElementById("fixtures").append(root);
  const listeners = {};
  const win = {
    location: new URL(start),
    pushed: [],
    history: {
      pushState(_, __, path) {
        win.location = new URL(path, win.location);
        win.pushed.push(path);
      },
      replaceState(_, __, path) {
        win.location = new URL(path, win.location);
      },
    },
    document: root,
    HTMLAnchorElement,
    addEventListener: (t, fn) => (listeners[t] = fn),
    removeEventListener: (t) => delete listeners[t],
    pop(path) {
      win.location = new URL(path, win.location);
      listeners.popstate?.();
    },
    cleanup: () => root.remove(),
  };
  return win;
}

describe("router", () => {
  test("matches routes, decodes params and keeps the query", () => {
    const r = createRouter(ROUTES, { window: fakeWindow() });
    const m = r.match("/w/acme/issues/HZN-42?tab=activity");
    assert.equal(m.route.name, "issue");
    assert.deepEqual(m.params, { workspace: "acme", key: "HZN-42" });
    assert.equal(m.query.get("tab"), "activity");
    assert.equal(r.match("/w/acme/issues/HZN-42/").route.name, "issue");
    assert.equal(r.match("/w/a%20b/issues/X-1").params.workspace, "a b");
    assert.equal(r.match("/nowhere").route, null);
  });

  test("href builds paths and needs every parameter", () => {
    const r = createRouter(ROUTES, { window: fakeWindow() });
    assert.equal(r.href("issue", { workspace: "a b", key: "HZN-1" }), "/w/a%20b/issues/HZN-1");
    assert.throws(() => r.href("issue", { workspace: "acme" }), /needs key/);
    assert.throws(() => r.href("nope"), /no route/);
  });

  test("navigate and popstate notify subscribers", () => {
    const win = fakeWindow();
    const r = createRouter(ROUTES, { window: win });
    const seen = [];
    r.subscribe((m) => seen.push(m.route?.name ?? "none"));
    r.start();
    r.navigate("/w/acme/issues/HZN-1");
    win.pop("/");
    win.pop("/missing");
    r.stop();
    win.cleanup();
    assert.deepEqual(seen, ["home", "issue", "home", "none"]);
    assert.deepEqual(win.pushed, ["/w/acme/issues/HZN-1"]);
  });

  test("intercepts plain clicks on app links, including inside shadow roots", () => {
    const win = fakeWindow();
    const r = createRouter(ROUTES, { window: win }).start();
    const host = document.createElement("div");
    const shadow = host.attachShadow({ mode: "open" });
    shadow.innerHTML = `
      <a id="app" href="/w/acme/issues/HZN-7">app</a>
      <a id="server" href="/swagger/">server</a>
      <a id="blank" href="/" target="_blank">new tab</a>
      <a id="external" href="https://example.com/">external</a>`;
    win.document.append(host);
    // Runs after the router's listener: records whether it handled the
    // click, then stops the real navigation so the test page stays.
    let handled = null;
    win.document.addEventListener("click", (e) => {
      handled = e.defaultPrevented;
      e.preventDefault();
    });
    const click = (id, init = {}) => {
      handled = null;
      shadow.getElementById(id).dispatchEvent(new MouseEvent("click", { bubbles: true, composed: true, cancelable: true, button: 0, ...init }));
      return handled;
    };
    try {
      assert.ok(click("app"), "app link handled");
      assert.equal(win.location.pathname, "/w/acme/issues/HZN-7");
      assert.ok(!click("app", { metaKey: true }), "modifier clicks left to the browser");
      assert.ok(!click("server"), "routes the SPA doesn't know are left to the server");
      assert.ok(!click("blank"));
      assert.ok(!click("external"));
    } finally {
      r.stop();
      win.cleanup();
    }
  });
});
