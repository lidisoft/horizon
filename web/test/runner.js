// A small test runner for web/app modules (ADR-0006: no npm, no Jest). Test
// files import it as "horizon-test/runner.js" and register tests; the page's
// entry module runs them. Go reads window.__horizonTests through chromedp
// (web/browser_test.go); opened by hand, the page shows a report.

const registered = [];
let prefix = "";

/** Groups tests under a name. */
export function describe(name, fn) {
  const saved = prefix;
  prefix = saved ? `${saved} › ${name}` : name;
  try {
    fn();
  } finally {
    prefix = saved;
  }
}

/** Registers a test; fn may be async. */
export function test(name, fn, { timeout = 5000 } = {}) {
  registered.push({ name: prefix ? `${prefix} › ${name}` : name, fn, timeout });
}

export class AssertionError extends Error {
  constructor(message) {
    super(message);
    this.name = "AssertionError";
  }
}

const show = (v) => {
  try {
    return typeof v === "string" ? JSON.stringify(v) : JSON.stringify(v) ?? String(v);
  } catch {
    return String(v);
  }
};

function deepEqual(a, b) {
  if (Object.is(a, b)) return true;
  if (a instanceof Date && b instanceof Date) return a.getTime() === b.getTime();
  if (typeof a !== "object" || typeof b !== "object" || a === null || b === null) return false;
  if (Array.isArray(a) !== Array.isArray(b)) return false;
  const ka = Object.keys(a);
  const kb = Object.keys(b);
  return ka.length === kb.length && ka.every((k) => Object.hasOwn(b, k) && deepEqual(a[k], b[k]));
}

function matches(err, expected) {
  if (expected === undefined) return true;
  if (typeof expected === "function") return err instanceof expected;
  if (expected instanceof RegExp) return expected.test(String(err?.message ?? err));
  return false;
}

export const assert = {
  ok(value, message = `expected a truthy value, got ${show(value)}`) {
    if (!value) throw new AssertionError(message);
  },
  equal(actual, expected, message) {
    if (!Object.is(actual, expected)) {
      throw new AssertionError(message || `expected ${show(expected)}, got ${show(actual)}`);
    }
  },
  deepEqual(actual, expected, message) {
    if (!deepEqual(actual, expected)) {
      throw new AssertionError(message || `expected ${show(expected)}, got ${show(actual)}`);
    }
  },
  match(actual, regex, message) {
    if (!regex.test(String(actual))) throw new AssertionError(message || `${show(actual)} does not match ${regex}`);
  },
  /** Expects fn to throw; expected is an Error class or a RegExp on the message. */
  throws(fn, expected) {
    try {
      fn();
    } catch (err) {
      if (!matches(err, expected)) throw new AssertionError(`threw the wrong error: ${err}`);
      return err;
    }
    throw new AssertionError("expected a throw");
  },
  async rejects(promise, expected) {
    try {
      await (typeof promise === "function" ? promise() : promise);
    } catch (err) {
      if (!matches(err, expected)) throw new AssertionError(`rejected with the wrong error: ${err}`);
      return err;
    }
    throw new AssertionError("expected a rejection");
  },
};

/** Resolves after pending microtasks (store notifications are batched). */
export const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

function withTimeout(promise, ms, name) {
  let timer;
  return Promise.race([
    promise.finally(() => clearTimeout(timer)),
    new Promise((_, reject) => {
      timer = setTimeout(() => reject(new Error(`${name}: timed out after ${ms} ms`)), ms);
    }),
  ]);
}

/**
 * Runs the registered tests in order. filter keeps the tests whose name
 * contains it (?filter=store in the page URL).
 */
export async function run({ filter = new URLSearchParams(location.search).get("filter") || "" } = {}) {
  const results = [];
  for (const t of registered) {
    if (filter && !t.name.includes(filter)) continue;
    const started = performance.now();
    try {
      await withTimeout(Promise.resolve().then(t.fn), t.timeout, t.name);
      results.push({ name: t.name, ok: true, ms: Math.round(performance.now() - started) });
    } catch (err) {
      results.push({ name: t.name, ok: false, ms: Math.round(performance.now() - started), error: err?.stack || String(err) });
    }
  }
  report(results);
  window.__horizonTests = { done: true, results };
  return results;
}

function report(results) {
  const failed = results.filter((r) => !r.ok);
  const summary = document.createElement("h1");
  summary.textContent = `${results.length - failed.length} passed, ${failed.length} failed`;
  summary.className = failed.length ? "failed" : "passed";
  const list = document.createElement("ol");
  for (const r of results) {
    const li = document.createElement("li");
    li.className = r.ok ? "passed" : "failed";
    li.textContent = `${r.ok ? "✓" : "✗"} ${r.name} (${r.ms} ms)`;
    if (!r.ok) {
      const pre = document.createElement("pre");
      pre.textContent = r.error;
      li.append(pre);
    }
    list.append(li);
  }
  document.getElementById("report").replaceChildren(summary, list);
}
