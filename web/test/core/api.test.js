import { ApiError, NetworkError, createApi } from "horizon/core/api.js";
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
});
