// fetch wrapper for the JSON API (docs/api.md#conventions): JSON bodies,
// an Idempotency-Key on every mutation, ETag / If-Match for optimistic
// concurrency, the CSRF header for cookie sessions, and RFC 9457 problem
// details turned into ApiError.
//
// Session hooks (docs/security.md#authentication-methods): a 401
// "unauthenticated" calls onUnauthenticated (the session ended); a 403
// "csrf_invalid" (the session changed in another tab) calls refreshCsrf and
// retries once with the same idempotency key.

const SAFE = new Set(["GET", "HEAD"]);

/**
 * A random UUIDv4 for Idempotency-Key. crypto.randomUUID exists only in
 * secure contexts (HTTPS or localhost); getRandomValues works everywhere,
 * including plain HTTP on a LAN address.
 */
export function newIdempotencyKey() {
  if (typeof crypto.randomUUID === "function") return crypto.randomUUID();
  const b = crypto.getRandomValues(new Uint8Array(16));
  b[6] = (b[6] & 0x0f) | 0x40; // version 4
  b[8] = (b[8] & 0x3f) | 0x80; // RFC 9562 variant
  const h = Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

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
 * @param {(error: ApiError) => void} [options.onUnauthenticated]
 * @param {() => Promise<void>} [options.refreshCsrf]
 */
export function createApi({
  base = "/api/v1",
  fetch = globalThis.fetch.bind(globalThis),
  csrfToken = () => null,
  onUnauthenticated = () => {},
  refreshCsrf = null,
} = {}) {
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
  async function request(method, path, options = {}) {
    // One idempotency key per logical mutation, kept across the CSRF retry.
    if (!SAFE.has(method) && !options.idempotencyKey) options = { ...options, idempotencyKey: newIdempotencyKey() };
    try {
      return await send(method, path, options);
    } catch (err) {
      if (err instanceof ApiError && err.code === "csrf_invalid" && refreshCsrf && !options.retried) {
        await refreshCsrf();
        return request(method, path, { ...options, retried: true });
      }
      if (err instanceof ApiError && err.code === "unauthenticated") onUnauthenticated(err);
      throw err;
    }
  }

  async function send(method, path, { body, query, ifMatch, idempotencyKey, signal } = {}) {
    const url = new URL(base + path, location.origin);
    for (const [k, v] of Object.entries(query || {})) {
      if (v !== undefined && v !== null) url.searchParams.set(k, String(v));
    }

    const headers = { Accept: "application/json" };
    if (body !== undefined) headers["Content-Type"] = "application/json";
    if (!SAFE.has(method)) {
      headers["Idempotency-Key"] = idempotencyKey;
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
