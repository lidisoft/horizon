import { ApiError, NetworkError, createApi, newIdempotencyKey } from "horizon/core/api.js";
import { assert, describe, test } from "horizon-test/runner.js";

/** A fetch double that records calls and answers with the next response. */
function fakeFetch(...responses) {
  const calls = [];
  const fetch = async (url, init) => {
    calls.push({ url: String(url), ...init, headers: { ...init.headers } });
    const r = responses.shift();
    if (r instanceof Error) throw r;
    return new Response(r.body === undefined ? null : JSON.stringify(r.body), {
      status: r.status ?? 200,
      headers: { "Content-Type": r.type ?? "application/json", ...(r.headers || {}) },
    });
  };
  return { fetch, calls };
}

describe("api", () => {
  test("GET sends no idempotency key and returns data and ETag", async () => {
    const { fetch, calls } = fakeFetch({ body: { key: "HZN-1" }, headers: { ETag: '"3"' } });
    const api = createApi({ fetch });
    const res = await api.get("/w/acme/issues/HZN-1", { query: { expand: "parent", skip: undefined } });
    assert.deepEqual(res.data, { key: "HZN-1" });
    assert.equal(res.etag, '"3"');
    assert.equal(calls[0].method, "GET");
    assert.match(calls[0].url, /\/api\/v1\/w\/acme\/issues\/HZN-1\?expand=parent$/);
    assert.equal(calls[0].headers["Idempotency-Key"], undefined);
  });

  test("mutations send JSON, an idempotency key, If-Match and the CSRF token", async () => {
    const { fetch, calls } = fakeFetch({ status: 204, body: undefined });
    const api = createApi({ fetch, csrfToken: () => "csrf-1" });
    const res = await api.post("/w/acme/issues/HZN-1/assign", { body: { assignee: "u1" }, ifMatch: '"3"' });
    assert.equal(res.data, null);
    const h = calls[0].headers;
    assert.match(h["Idempotency-Key"], /^[0-9a-f-]{36}$/);
    assert.equal(h["If-Match"], '"3"');
    assert.equal(h["X-CSRF-Token"], "csrf-1");
    assert.equal(calls[0].body, '{"assignee":"u1"}');
  });

  test("a retry can reuse its idempotency key", async () => {
    const { fetch, calls } = fakeFetch({ body: {} }, { body: {} });
    const api = createApi({ fetch });
    await api.post("/x", { body: {}, idempotencyKey: "k-1" });
    await api.post("/x", { body: {}, idempotencyKey: "k-1" });
    assert.equal(calls[0].headers["Idempotency-Key"], "k-1");
    assert.equal(calls[1].headers["Idempotency-Key"], "k-1");
  });

  test("problem details become an ApiError with code and fields", async () => {
    const problem = { type: "validation_failed", title: "Invalid issue", status: 422, errors: [{ field: "title", code: "required" }] };
    const { fetch } = fakeFetch({ status: 422, body: problem, type: "application/problem+json" });
    const err = await assert.rejects(createApi({ fetch }).post("/x", { body: {} }), ApiError);
    assert.equal(err.status, 422);
    assert.equal(err.code, "validation_failed");
    assert.deepEqual(err.fields, [{ field: "title", code: "required" }]);
  });

  test("about:blank problems get an http_<status> code", async () => {
    const { fetch } = fakeFetch({ status: 404, body: { type: "about:blank", title: "Not Found", status: 404 }, type: "application/problem+json" });
    const err = await assert.rejects(createApi({ fetch }).get("/nope"), ApiError);
    assert.equal(err.code, "http_404");
  });

  test("network failures become NetworkError; aborts stay AbortError", async () => {
    const { fetch } = fakeFetch(new TypeError("Failed to fetch"), new DOMException("aborted", "AbortError"));
    const api = createApi({ fetch });
    await assert.rejects(api.get("/x"), NetworkError);
    const err = await assert.rejects(api.get("/x"));
    assert.equal(err.name, "AbortError");
  });
  test("a 401 unauthenticated tells the session; other 401s don't", async () => {
    const ended = [];
    const { fetch } = fakeFetch(
      { status: 401, body: { type: "unauthenticated", status: 401 } },
      { status: 401, body: { type: "invalid_credentials", status: 401 } },
    );
    const api = createApi({ fetch, onUnauthenticated: (err) => ended.push(err.code) });
    await assert.rejects(api.get("/me"), ApiError);
    await assert.rejects(api.post("/auth/login", { body: {} }), ApiError);
    assert.deepEqual(ended, ["unauthenticated"]);
  });

  test("a stale CSRF token is refreshed and the mutation retried once, same key", async () => {
    let token = "old";
    const { fetch, calls } = fakeFetch(
      { status: 403, body: { type: "csrf_invalid", status: 403 } },
      { status: 204, body: undefined },
    );
    const api = createApi({ fetch, csrfToken: () => token, refreshCsrf: async () => (token = "new") });
    await api.patch("/me", { body: { name: "Ada" } });
    assert.equal(calls.length, 2);
    assert.equal(calls[0].headers["X-CSRF-Token"], "old");
    assert.equal(calls[1].headers["X-CSRF-Token"], "new");
    assert.equal(calls[0].headers["Idempotency-Key"], calls[1].headers["Idempotency-Key"]);
  });

  test("a CSRF failure after the refresh is not retried again", async () => {
    const refused = { status: 403, body: { type: "csrf_invalid", status: 403 } };
    const { fetch, calls } = fakeFetch(refused, refused);
    const api = createApi({ fetch, refreshCsrf: async () => {} });
    const err = await assert.rejects(api.post("/x", { body: {} }), ApiError);
    assert.equal(err.code, "csrf_invalid");
    assert.equal(calls.length, 2);
  });
  test("idempotency keys are UUIDv4 even without crypto.randomUUID (plain HTTP off localhost)", async () => {
    const uuid4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
    crypto.randomUUID = undefined; // shadows Crypto.prototype.randomUUID, as in an insecure context
    try {
      const keys = new Set(Array.from({ length: 50 }, newIdempotencyKey));
      assert.equal(keys.size, 50);
      for (const k of keys) assert.match(k, uuid4);
      const { fetch, calls } = fakeFetch({ status: 204, body: undefined });
      await createApi({ fetch }).post("/x", { body: {} });
      assert.match(calls[0].headers["Idempotency-Key"], uuid4);
    } finally {
      delete crypto.randomUUID;
    }
    assert.equal(typeof crypto.randomUUID, "function");
  });
});
