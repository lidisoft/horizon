// Client cache (docs/frontend.md#state-and-data-flow): entities normalised by
// type and id, query results as lists of ids, and change notifications
// batched per microtask so a burst of updates renders once.

/**
 * @typedef {{ status: "loading"|"ready"|"error", ids: string[], error: any, meta: any }} QueryState
 */

export function createStore() {
  /** @type {Map<string, Map<string, object>>} */
  const entities = new Map();
  /** @type {Map<string, QueryState & { promise?: Promise<void> }>} */
  const queries = new Map();
  /** @type {Map<string, (store: object, event: object) => void>} */
  const eventHandlers = new Map();
  const listeners = new Set();
  let scheduled = false;

  function changed() {
    if (scheduled) return;
    scheduled = true;
    queueMicrotask(() => {
      scheduled = false;
      for (const fn of [...listeners]) fn(store);
    });
  }

  function table(type) {
    let t = entities.get(type);
    if (!t) {
      t = new Map();
      entities.set(type, t);
    }
    return t;
  }

  const store = {
    /** Inserts or replaces entities (each needs an id). */
    put(type, items) {
      const t = table(type);
      for (const item of Array.isArray(items) ? items : [items]) {
        if (item?.id == null) throw new Error(`store.put(${type}): entity without id`);
        t.set(String(item.id), Object.freeze({ ...item }));
      }
      changed();
    },

    /** Merges fields into an existing entity; returns the previous version. */
    patch(type, id, fields) {
      const t = table(type);
      const before = t.get(String(id));
      if (!before) return undefined;
      t.set(String(id), Object.freeze({ ...before, ...fields }));
      changed();
      return before;
    },

    remove(type, id) {
      if (table(type).delete(String(id))) changed();
    },

    get(type, id) {
      return entities.get(type)?.get(String(id));
    },

    all(type) {
      return [...(entities.get(type)?.values() || [])];
    },

    /**
     * Runs fetcher once per key until invalidated; concurrent calls share the
     * request. fetcher returns { type, items, meta? }.
     * @returns {Promise<QueryState>}
     */
    query(key, fetcher) {
      const q = queries.get(key);
      if (q && (q.status === "ready" || q.promise)) {
        return q.promise ? q.promise.then(() => queries.get(key)) : Promise.resolve(q);
      }
      const state = { status: "loading", ids: q?.ids || [], error: null, meta: q?.meta ?? null };
      state.promise = Promise.resolve()
        .then(fetcher)
        .then(({ type, items, meta = null }) => {
          store.put(type, items);
          queries.set(key, { status: "ready", ids: items.map((i) => String(i.id)), error: null, meta });
        })
        .catch((error) => {
          queries.set(key, { status: "error", ids: state.ids, error, meta: state.meta });
        })
        .finally(changed);
      queries.set(key, state);
      changed();
      return state.promise.then(() => queries.get(key));
    },

    /** The current state of a query, without fetching. */
    queryState(key) {
      const q = queries.get(key);
      return q && { status: q.status, ids: q.ids, error: q.error, meta: q.meta };
    },

    /** Marks queries whose key starts with prefix stale; the next query() refetches. */
    invalidate(prefix = "") {
      for (const [key, q] of queries) {
        if (key.startsWith(prefix) && !q.promise) queries.set(key, { ...q, status: "stale" });
      }
      changed();
    },

    /** Registers how a domain event type updates the cache. */
    on(eventType, handler) {
      eventHandlers.set(eventType, handler);
      return () => eventHandlers.delete(eventType);
    },

    /**
     * Applies a domain event (docs/api.md#domain-event-format). Without a
     * handler, queries touching that aggregate type are invalidated.
     */
    apply(event) {
      const handler = eventHandlers.get(event.type);
      if (handler) handler(store, event);
      else store.invalidate(event.aggregate?.type || "");
    },

    /** Calls fn after each batch of changes; returns the unsubscribe function. */
    subscribe(fn) {
      listeners.add(fn);
      return () => listeners.delete(fn);
    },
  };
  return store;
}
