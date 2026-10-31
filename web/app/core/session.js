// The signed-in user and the CSRF token (docs/security.md). The session
// cookie itself is HttpOnly: the client only learns whether it is valid by
// asking /me.

/**
 * @typedef {{ status: "unknown"|"signed-in"|"signed-out", user: object|null, csrf: string|null }} SessionState
 */

/** Paths a "next" parameter may send the user back to: same-origin only. */
export function safeNext(next, fallback = "/") {
  return typeof next === "string" && next.startsWith("/") && !next.startsWith("//") && !next.startsWith("/\\")
    ? next
    : fallback;
}

/**
 * @param {object} options
 * @param {() => { get: Function, post: Function }} options.api returns the
 *   API client (a getter: the client itself is built with this session's hooks)
 */
export function createSession({ api }) {
  /** @type {SessionState} */
  let state = { status: "unknown", user: null, csrf: null };
  const listeners = new Set();

  function set(next) {
    const changed = next.status !== state.status || next.user !== state.user;
    state = next;
    if (changed) for (const fn of [...listeners]) fn(state);
  }

  function signedIn(data) {
    set({ status: "signed-in", user: data.user, csrf: data.csrf_token });
  }

  const session = {
    get status() {
      return state.status;
    },
    get user() {
      return state.user;
    },
    get csrf() {
      return state.csrf;
    },

    /** Asks the server who is signed in. */
    async load() {
      try {
        const { data } = await api().get("/me");
        signedIn(data);
      } catch (err) {
        if (err?.status !== 401) throw err;
        set({ status: "signed-out", user: null, csrf: null });
      }
      return state;
    },

    async login(email, password) {
      const { data } = await api().post("/auth/login", { body: { email, password } });
      signedIn(data);
      return state;
    },

    async logout() {
      try {
        await api().post("/auth/logout");
      } finally {
        set({ status: "signed-out", user: null, csrf: null });
      }
    },

    /** Records the session ending server-side (a 401 on any request). */
    ended() {
      if (state.status === "signed-in") set({ status: "signed-out", user: null, csrf: null });
    },

    /** Replaces the user after a profile change. */
    setUser(user) {
      set({ ...state, user });
    },

    /** Calls fn with the new state when the user signs in, out or changes. */
    subscribe(fn) {
      listeners.add(fn);
      return () => listeners.delete(fn);
    },
  };
  return session;
}
