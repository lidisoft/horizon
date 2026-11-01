// WebSocket client (docs/realtime.md): one socket per tab, subscriptions
// that survive reconnects, replay of what was missed while away, and the
// domain events applied to the store.

const PING_EVERY = 25_000;
const MAX_BACKOFF = 30_000;
const SEEN = 500; // event ids remembered for dropping duplicates

/**
 * @param {{ url?: string, store?: { apply(e: object): void, invalidate(p?: string): void },
 *   WebSocket?: typeof WebSocket, timers?: { setTimeout: typeof setTimeout, clearTimeout: typeof clearTimeout,
 *   setInterval: typeof setInterval, clearInterval: typeof clearInterval } }} options
 */
export function createSocket({ url = "/api/v1/ws", store, WebSocket: Impl = globalThis.WebSocket, timers = globalThis } = {}) {
  const topics = new Set();
  const listeners = new Set();
  const seen = [];
  let ws = null;
  let open = false;
  let stopped = true;
  let lastId = "";
  let attempts = 0;
  let retry = null;
  let ping = null;
  let seq = 0;

  function absolute(path) {
    const u = new URL(path, location.href);
    u.protocol = u.protocol === "https:" ? "wss:" : "ws:";
    return u.href;
  }

  function send(t, d) {
    if (open) ws.send(JSON.stringify({ t, id: String(++seq), d }));
  }

  function notify(message) {
    for (const fn of [...listeners]) fn(message);
  }

  function connect() {
    if (stopped || ws) return;
    ws = new Impl(absolute(url));
    ws.onopen = () => {
      open = true;
      attempts = 0;
      if (topics.size) send("sub", { topics: [...topics], ...(lastId ? { since: lastId } : {}) });
      ping = timers.setInterval(() => send("ping"), PING_EVERY);
      notify({ t: "open" });
    };
    ws.onmessage = (m) => {
      let msg;
      try {
        msg = JSON.parse(m.data);
      } catch {
        return;
      }
      if (msg.t === "event") received(msg.d);
      else if (msg.t === "resync") {
        store?.invalidate("");
        notify(msg);
      } else notify(msg);
    };
    ws.onclose = () => {
      const wasOpen = open;
      ws = null;
      open = false;
      timers.clearInterval(ping);
      if (wasOpen) notify({ t: "close" });
      if (stopped) return;
      // Backoff with jitter: 1 s, 2 s, 4 s … up to 30 s.
      const delay = Math.min(MAX_BACKOFF, 1000 * 2 ** attempts++) * (0.5 + Math.random() / 2);
      retry = timers.setTimeout(() => {
        retry = null;
        connect();
      }, delay);
    };
  }

  function received(event) {
    if (!event?.id || seen.includes(event.id)) return;
    seen.push(event.id);
    if (seen.length > SEEN) seen.shift();
    // UUIDv7 ids sort by time.
    if (event.id > lastId) lastId = event.id;
    store?.apply(event);
    notify({ t: "event", d: event });
  }

  return {
    /** Opens the socket (again after stop). */
    start() {
      stopped = false;
      connect();
    },

    /** Closes the socket and forgets the topics, e.g. on sign-out. */
    stop() {
      stopped = true;
      topics.clear();
      lastId = "";
      if (retry) timers.clearTimeout(retry);
      retry = null;
      ws?.close();
    },

    /** Adds topics (ws:<slug>, project:<slug>/<KEY>, issue:<slug>/<KEY-n>). */
    subscribe(names) {
      const fresh = names.filter((n) => !topics.has(n));
      for (const n of fresh) topics.add(n);
      if (fresh.length) send("sub", { topics: fresh });
    },

    unsubscribe(names) {
      const gone = names.filter((n) => topics.delete(n));
      if (gone.length) send("unsub", { topics: gone });
    },

    /** The current topics. */
    topics: () => [...topics],

    /** Calls fn with every message: {t: "event", d}, resync, open, close… */
    on(fn) {
      listeners.add(fn);
      return () => listeners.delete(fn);
    },
  };
}
