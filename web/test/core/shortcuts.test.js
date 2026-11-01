import { createShortcuts, displayKeys, stepOf } from "horizon/core/shortcuts.js";
import { assert, describe, test } from "horizon-test/runner.js";

const key = (k, init = {}) => new KeyboardEvent("keydown", { key: k, cancelable: true, ...init });

function setup() {
  const target = new EventTarget();
  const shortcuts = createShortcuts({ target, mac: false });
  shortcuts.start();
  const fired = [];
  const add = (id, keys, extra = {}) => shortcuts.register({ id, keys, description: id, handler: () => fired.push(id), ...extra });
  return { target, shortcuts, fired, add };
}

describe("shortcuts", () => {
  test("stepOf normalises modifiers per platform", () => {
    assert.equal(stepOf(key("k", { ctrlKey: true }), false), "mod+k");
    assert.equal(stepOf(key("k", { metaKey: true }), true), "mod+k");
    assert.equal(stepOf(key("k", { metaKey: true }), false), "k");
    assert.equal(stepOf(key("?", { shiftKey: true }), false), "?");
    assert.equal(stepOf(key("Enter", { shiftKey: true }), false), "shift+enter");
  });

  test("single keys, chords and sequences fire their handlers", () => {
    const { target, fired, add } = setup();
    add("palette", "mod+k");
    add("help", "?");
    add("inbox", "g i");
    target.dispatchEvent(key("k", { ctrlKey: true }));
    target.dispatchEvent(key("?", { shiftKey: true }));
    target.dispatchEvent(key("g"));
    target.dispatchEvent(key("i"));
    assert.deepEqual(fired, ["palette", "help", "inbox"]);
  });

  test("an aborted sequence still lets the last key fire alone", () => {
    const { target, fired, add } = setup();
    add("inbox", "g i");
    add("create", "c");
    target.dispatchEvent(key("g"));
    target.dispatchEvent(key("c"));
    assert.deepEqual(fired, ["create"]);
  });

  test("typing in fields is ignored unless allowed", () => {
    const field = document.createElement("input");
    document.getElementById("fixtures").append(field);
    const shortcuts = createShortcuts({ target: field, mac: false });
    const fired = [];
    shortcuts.register({ id: "create", keys: "c", description: "c", handler: () => fired.push("create") });
    shortcuts.register({ id: "palette", keys: "mod+k", description: "k", allowInInputs: true, handler: () => fired.push("palette") });
    shortcuts.start();
    try {
      field.dispatchEvent(key("c", { bubbles: true, composed: true }));
      field.dispatchEvent(key("k", { ctrlKey: true, bubbles: true, composed: true }));
      assert.deepEqual(fired, ["palette"]);
    } finally {
      shortcuts.stop();
      field.remove();
    }
  });

  test("pause holds back all but the shortcuts allowed in inputs", () => {
    const { target, shortcuts, fired, add } = setup();
    add("comment", "r");
    add("palette", "mod+k", { allowInInputs: true });
    const resume = shortcuts.pause();
    target.dispatchEvent(key("r"));
    target.dispatchEvent(key("k", { ctrlKey: true }));
    resume();
    resume(); // idempotent
    target.dispatchEvent(key("r"));
    assert.deepEqual(fired, ["palette", "comment"]);
  });

  test("scopes limit shortcuts to the active view", () => {
    const { target, shortcuts, fired, add } = setup();
    add("global", "?");
    add("list-only", "x", { scope: "issue-list" });
    target.dispatchEvent(key("x"));
    const pop = shortcuts.pushScope("issue-list");
    target.dispatchEvent(key("x"));
    assert.deepEqual(shortcuts.list().map((s) => s.id).sort(), ["global", "list-only"]);
    pop();
    target.dispatchEvent(key("x"));
    assert.deepEqual(fired, ["list-only"]);
  });

  test("duplicate ids are refused and register returns an unregister function", () => {
    const { target, fired, add } = setup();
    const off = add("help", "?");
    assert.throws(() => add("help", "h"), /duplicate/);
    off();
    target.dispatchEvent(key("?", { shiftKey: true }));
    assert.deepEqual(fired, []);
  });

  test("displayKeys renders one entry per step", () => {
    assert.equal(displayKeys("g i").join(","), "G,I");
    assert.equal(displayKeys("escape")[0], "Esc");
  });
});
