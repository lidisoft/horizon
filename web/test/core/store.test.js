import { createStore } from "horizon/core/store.js";
import { assert, describe, flush, test } from "horizon-test/runner.js";

describe("store", () => {
  test("put, get, patch, all and remove", () => {
    const s = createStore();
    s.put("issue", [{ id: 1, title: "A" }, { id: "2", title: "B" }]);
    assert.equal(s.get("issue", "1").title, "A");
    assert.equal(s.get("issue", 2).title, "B");
    const before = s.patch("issue", 1, { title: "A2" });
    assert.equal(before.title, "A");
    assert.equal(s.get("issue", 1).title, "A2");
    assert.ok(Object.isFrozen(s.get("issue", 1)));
    assert.equal(s.patch("issue", 99, {}), undefined);
    s.remove("issue", 2);
    assert.deepEqual(s.all("issue").map((i) => i.id), [1]);
    assert.throws(() => s.put("issue", { title: "no id" }), /without id/);
  });

  test("subscribers are notified once per batch", async () => {
    const s = createStore();
    let calls = 0;
    const off = s.subscribe(() => calls++);
    s.put("issue", { id: 1 });
    s.put("issue", { id: 2 });
    s.patch("issue", 1, { x: 1 });
    await flush();
    assert.equal(calls, 1);
    off();
    s.put("issue", { id: 3 });
    await flush();
    assert.equal(calls, 1);
  });

  test("query dedupes concurrent fetches and caches until invalidated", async () => {
    const s = createStore();
    let fetches = 0;
    const fetcher = async () => {
      fetches++;
      return { type: "issue", items: [{ id: 1 }, { id: 2 }], meta: { next: null } };
    };
    const [a, b] = await Promise.all([s.query("issues:open", fetcher), s.query("issues:open", fetcher)]);
    assert.equal(fetches, 1);
    assert.deepEqual(a.ids, ["1", "2"]);
    assert.equal(b.status, "ready");
    await s.query("issues:open", fetcher);
    assert.equal(fetches, 1);

    s.invalidate("issues:");
    assert.equal(s.queryState("issues:open").status, "stale");
    await s.query("issues:open", fetcher);
    assert.equal(fetches, 2);
  });

  test("a failed query keeps its previous ids and retries on the next call", async () => {
    const s = createStore();
    await s.query("q", async () => ({ type: "issue", items: [{ id: 1 }] }));
    s.invalidate();
    const failed = await s.query("q", async () => {
      throw new Error("offline");
    });
    assert.equal(failed.status, "error");
    assert.deepEqual(failed.ids, ["1"]);
    const ok = await s.query("q", async () => ({ type: "issue", items: [{ id: 2 }] }));
    assert.deepEqual(ok.ids, ["2"]);
  });

  test("apply runs the event's handler, or invalidates its aggregate type", async () => {
    const s = createStore();
    await s.query("issue:list", async () => ({ type: "issue", items: [{ id: 1, title: "A" }] }));
    await s.query("iteration:list", async () => ({ type: "iteration", items: [] }));
    s.on("issue.retitled", (store, e) => store.patch("issue", e.aggregate.id, { title: e.payload.title }));

    s.apply({ type: "issue.retitled", aggregate: { type: "issue", id: 1 }, payload: { title: "B" } });
    assert.equal(s.get("issue", 1).title, "B");
    assert.equal(s.queryState("issue:list").status, "ready");

    s.apply({ type: "iteration.loaded", aggregate: { type: "iteration", id: 7 }, payload: {} });
    assert.equal(s.queryState("iteration:list").status, "stale");
    assert.equal(s.queryState("issue:list").status, "ready");
  });
});
