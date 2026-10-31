// fetch wrapper for the JSON API (docs/api.md#conventions): JSON bodies,
// an Idempotency-Key on every mutation, ETag / If-Match for optimistic
// concurrency, the CSRF header for cookie sessions, and RFC 9457 problem
// details turned into ApiError.

const SAFE = new Set(["GET", "HEAD"]);

/** An API failure. code is the problem type, stable and translatable. */
export class ApiError extends Error {
  constructor(status, problem = {}) {
    super(problem.detail || problem.title || `HTTP ${status}`);
    this.name = "ApiError";
    this.status = status;
    this.code = problem.type && problem.type !== "about:blank" ? problem.type : `http_${status}`;
    this.title = problem.title || "";
    this.fields = problem.errors || [];
    this.problem = problem;
  }

  /** The current representation, sent with 409 Conflict. */
  get current() {
    return this.problem.current;
  }
}

/** A request that never reached the server (offline, DNS, aborted). */
export class NetworkError extends Error {
  constructor(cause) {
    super("network error", { cause });
    this.name = "NetworkError";
  }
}

/**
 * Creates an API client.
 *
 * @param {object} [options]
 * @param {string} [options.base] path prefix, /api/v1
 * @param {typeof fetch} [options.fetch] for tests
 * @param {() => string|null} [options.csrfToken] header value for cookie sessions
 */
export function createApi({ base = "/api/v1", fetch = globalThis.fetch.bind(globalThis), csrfToken = () => null } = {}) {
  /**
   * @param {string} method
   * @param {string} path e.g. /w/acme/issues/HZN-42
   * @param {object} [options]
   * @param {any} [options.body] JSON-encoded
   * @param {Record<string, any>} [options.query] undefined and null are skipped
   * @param {string} [options.ifMatch] ETag from a previous response
   * @param {string} [options.idempotencyKey] reuse it when retrying the same mutation
   * @param {AbortSignal} [options.signal]
   * @returns {Promise<{ data: any, etag: string|null, status: number }>}
   */
  async function request(method, path, { body, query, ifMatch, idempotencyKey, signal } = {}) {
    const url = new URL(base + path, location.origin);
    for (const [k, v] of Object.entries(query || {})) {
      if (v !== undefined && v !== null) url.searchParams.set(k, String(v));
    }

    const headers = { Accept: "application/json" };
    if (body !== undefined) headers["Content-Type"] = "application/json";
    if (!SAFE.has(method)) {
      headers["Idempotency-Key"] = idempotencyKey || crypto.randomUUID();
      const csrf = csrfToken();
      if (csrf) headers["X-CSRF-Token"] = csrf;
    }
    if (ifMatch) headers["If-Match"] = ifMatch;

    let res;
    try {
      res = await fetch(url, {
        method,
        headers,
        body: body === undefined ? undefined : JSON.stringify(body),
        credentials: "same-origin",
        signal,
      });
    } catch (err) {
      if (err?.name === "AbortError") throw err;
      throw new NetworkError(err);
    }

    const type = res.headers.get("Content-Type") || "";
    const data = res.status === 204 || !/json/.test(type) ? null : await res.json();
    if (!res.ok) throw new ApiError(res.status, data || { title: res.statusText });
    return { data, etag: res.headers.get("ETag"), status: res.status };
  }

  const verb = (method) => (path, options) => request(method, path, options);
  return {
    request,
    get: verb("GET"),
    post: verb("POST"),
    put: verb("PUT"),
    patch: verb("PATCH"),
    delete: verb("DELETE"),
  };
}
