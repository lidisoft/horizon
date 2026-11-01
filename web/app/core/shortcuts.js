// Keyboard layer (docs/frontend.md): every frequent action has a shortcut,
// and the help overlay (?) is generated from this registry.
//
// Keys are written as space-separated steps; a step is modifiers plus a key:
//   "mod+k"   ⌘K on macOS, Ctrl+K elsewhere
//   "g i"     G then I, within a second
//   "?"       printable keys match as typed, whatever Shift did
//
// Scopes: "global" always applies; push a scope (e.g. "issue-list") while a
// view is active, so its shortcuts apply only there. pause() while a dialog
// such as the command palette has the keyboard: only shortcuts allowed in
// inputs (⌘K) fire, so keys meant for the dialog never act on the page.

const SEQUENCE_TIMEOUT = 1000;

const isMac = /Mac|iPhone|iPad/.test(globalThis.navigator?.platform || "");

/** Display form of a step for lds-kbd, e.g. ["⌘K"] or ["G", "I"]. */
export function displayKeys(keys) {
  return keys.split(" ").map((step) =>
    step
      .split("+")
      .map((part) => {
        if (part === "mod") return isMac ? "⌘" : "Ctrl+";
        if (part === "shift") return isMac ? "⇧" : "Shift+";
        if (part === "alt") return isMac ? "⌥" : "Alt+";
        if (part === "escape") return "Esc";
        return part.length === 1 ? part.toUpperCase() : part[0].toUpperCase() + part.slice(1);
      })
      .join(""),
  );
}

function isEditable(event) {
  const el = event.composedPath()[0];
  if (!(el instanceof Element)) return false;
  return el.isContentEditable || el.matches?.("input, textarea, select, [contenteditable='' i], [contenteditable='true' i]");
}

/** Normalises a keydown event to a step string. */
export function stepOf(event, mac = isMac) {
  const key = event.key.toLowerCase();
  const mods = [];
  if (mac ? event.metaKey : event.ctrlKey) mods.push("mod");
  if (event.altKey) mods.push("alt");
  // Shift is part of printable characters (? is Shift+/); only named keys keep it.
  if (event.shiftKey && key.length > 1) mods.push("shift");
  return [...mods, key].join("+");
}

/**
 * @typedef {object} Shortcut
 * @property {string} id
 * @property {string} keys
 * @property {string} description i18n key
 * @property {string} [scope] default "global"
 * @property {boolean} [allowInInputs] also fire while typing
 * @property {(event: KeyboardEvent) => void} handler
 */

export function createShortcuts({ target = globalThis.window, mac = isMac } = {}) {
  /** @type {Map<string, Shortcut>} */
  const registry = new Map();
  const scopes = ["global"];
  let pauses = 0;
  let buffer = [];
  let timer = null;

  function active() {
    return new Set(["global", scopes[scopes.length - 1]]);
  }

  function onKeyDown(event) {
    if (event.isComposing || event.repeat) return;
    if (["shift", "control", "meta", "alt"].includes(event.key.toLowerCase())) return;
    const editable = isEditable(event);
    const step = stepOf(event, mac);

    clearTimeout(timer);
    buffer.push(step);
    const typed = buffer.join(" ");
    const inScope = [...registry.values()].filter((s) => active().has(s.scope) && ((!editable && !pauses) || s.allowInInputs));

    const exact = inScope.find((s) => s.keys === typed);
    if (exact) {
      buffer = [];
      event.preventDefault();
      exact.handler(event);
      return;
    }
    if (inScope.some((s) => s.keys.startsWith(typed + " "))) {
      timer = setTimeout(() => (buffer = []), SEQUENCE_TIMEOUT);
      return;
    }
    // Not a prefix of anything: retry this key alone (an aborted sequence).
    buffer = [];
    if (typed !== step) {
      const alone = inScope.find((s) => s.keys === step);
      if (alone) {
        event.preventDefault();
        alone.handler(event);
      }
    }
  }

  return {
    start() {
      target.addEventListener("keydown", onKeyDown);
    },

    stop() {
      target.removeEventListener("keydown", onKeyDown);
    },

    /** Registers a shortcut; returns the function that removes it. */
    register(shortcut) {
      const s = { scope: "global", allowInInputs: false, ...shortcut };
      if (registry.has(s.id)) throw new Error(`shortcuts: duplicate id ${s.id}`);
      registry.set(s.id, s);
      return () => registry.delete(s.id);
    },

    /** Makes scope the active view scope until the returned function runs. */
    pushScope(scope) {
      scopes.push(scope);
      return () => {
        const i = scopes.lastIndexOf(scope);
        if (i > 0) scopes.splice(i, 1);
      };
    },

    /** Holds back shortcuts not allowed in inputs until the returned function runs. */
    pause() {
      pauses++;
      buffer = [];
      let done = false;
      return () => {
        if (!done) pauses--;
        done = true;
      };
    },

    /** Shortcuts that currently apply, for the help overlay. */
    list() {
      const scope = active();
      return [...registry.values()].filter((s) => scope.has(s.scope));
    },
  };
}
