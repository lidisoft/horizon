import { ApiError } from "horizon/core/api.js";
import { createSession, safeNext } from "horizon/core/session.js";
import { assert, describe, test } from "horizon-test/runner.js";

const ada = { id: "u1", name: "Ada", version: 1 };

/** An API double answering each path with a function of the request. */
function fakeApi(routes) {
  const calls = [];
  const call = (method) => async (path, options = {}) => {
    calls.push({ method, path, body: options.body });
    const answer = routes[`${method} ${path}`];
    if (!answer) throw new Error(`unexpected ${method} ${path}`);
    return answer(options);
  };
  return { api: { get: call("GET"), post: call("POST") }, calls };
}

const unauthenticated = () => {
  throw new ApiError(401, { type: "unauthenticated" });
};

describe("session", () => {
  test("load: signed in from /me", async () => {
    const { api } = fakeApi({ "GET /me": () => ({ data: { user: ada, csrf_token: "c1" } }) });
    const session = createSession({ api: () => api });
    assert.equal(session.status, "unknown");
    await session.load();
    assert.equal(session.status, "signed-in");
    assert.equal(session.user, ada);
    assert.equal(session.csrf, "c1");
  });

  test("load: a 401 means signed out; other failures are thrown", async () => {
    const { api } = fakeApi({ "GET /me": unauthenticated });
    const session = createSession({ api: () => api });
    await session.load();
    assert.equal(session.status, "signed-out");

    const broken = fakeApi({ "GET /me": () => Promise.reject(new ApiError(503, {})) });
    await assert.rejects(createSession({ api: () => broken.api }).load(), ApiError);
  });

  test("login, then logout, notifying subscribers once per change", async () => {
    const { api, calls } = fakeApi({
      "POST /auth/login": () => ({ data: { user: ada, csrf_token: "c2" } }),
      "POST /auth/logout": () => ({ data: null }),
    });
    const session = createSession({ api: () => api });
    const seen = [];
    session.subscribe((s) => seen.push(s.status));
    await session.login("ada@example.com", "pw");
    assert.deepEqual(calls[0].body, { email: "ada@example.com", password: "pw" });
    assert.equal(session.csrf, "c2");
    await session.logout();
    assert.equal(session.user, null);
    assert.deepEqual(seen, ["signed-in", "signed-out"]);
  });

  test("logout signs out locally even when the server call fails", async () => {
    const { api } = fakeApi({
      "POST /auth/login": () => ({ data: { user: ada, csrf_token: "c" } }),
      "POST /auth/logout": () => Promise.reject(new Error("offline")),
    });
    const session = createSession({ api: () => api });
    await session.login("a", "b");
    await assert.rejects(session.logout(), Error);
    assert.equal(session.status, "signed-out");
  });

  test("ended only acts on a signed-in session", async () => {
    const { api } = fakeApi({ "GET /me": () => ({ data: { user: ada, csrf_token: "c" } }) });
    const session = createSession({ api: () => api });
    const seen = [];
    session.subscribe((s) => seen.push(s.status));
    session.ended();
    await session.load();
    session.ended();
    session.ended();
    assert.deepEqual(seen, ["signed-in", "signed-out"]);
  });

  test("setUser notifies with the new user", async () => {
    const { api } = fakeApi({ "GET /me": () => ({ data: { user: ada, csrf_token: "c" } }) });
    const session = createSession({ api: () => api });
    await session.load();
    let seen = null;
    session.subscribe((s) => (seen = s.user));
    session.setUser({ ...ada, name: "Ada King" });
    assert.equal(seen.name, "Ada King");
    assert.equal(session.csrf, "c");
  });

  test("safeNext keeps same-origin paths only", () => {
    assert.equal(safeNext("/account?tab=1#x"), "/account?tab=1#x");
    for (const bad of [null, "", "account", "//evil.example", "/\\evil.example", "https://evil.example/"]) {
      assert.equal(safeNext(bad), "/", String(bad));
    }
    assert.equal(safeNext("//x", "/home"), "/home");
  });
});
