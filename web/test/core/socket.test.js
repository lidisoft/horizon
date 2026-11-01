import { createSocket } from "horizon/core/socket.js";
import { assert, describe, test } from "horizon-test/runner.js";

// FakeSocket records what the client sends; the test plays the server.
class FakeSocket {
  static all = [];
  constructor(url) {
    this.url = url;
    this.sent = [];
    FakeSocket.all.push(this);
  }
  send(data) {
    this.sent.push(JSON.parse(data));
  }
  close() {
    this.onclose?.();
  }
  open() {
    this.onopen?.();
  }
  push(msg) {
    this.onmessage?.({ data: JSON.stringify(msg) });
  }
}

function fakeTimers() {
  const pending = [];
  return {
    pending,
    setTimeout: (fn, ms) => pending.push({ fn, ms }) && pending.length,
    clearTimeout: () => {},
    setInterval: () => 0,
    clearInterval: () => {},
    fire() {
      for (const t of pending.splice(0)) t.fn();
    },
  };
}

const event = (id, type = "issue.created") => ({ id, type, aggregate: { type: "issue", id: "i", key: "HZN-1", version: 1 } });

describe("socket", () => {
  test("subscribes on open, applies events once, and replays after a reconnect", () => {
    FakeSocket.all = [];
    const applied = [];
    const invalidated = [];
    const store = { apply: (e) => applied.push(e.id), invalidate: (p) => invalidated.push(p) };
    const timers = fakeTimers();
    const s = createSocket({ url: "/api/v1/ws", store, WebSocket: FakeSocket, timers });
    s.subscribe(["ws:acme"]);
    s.start();
    const first = FakeSocket.all[0];
    assert.ok(first.url.endsWith("/api/v1/ws") && first.url.startsWith("ws"));
    first.open();
    assert.deepEqual(first.sent[0].t, "sub");
    assert.deepEqual(first.sent[0].d, { topics: ["ws:acme"] });

    s.subscribe(["ws:acme", "issue:acme/HZN-1"]);
    assert.deepEqual(first.sent[1].d, { topics: ["issue:acme/HZN-1"] });

    first.push({ t: "event", d: event("01a") });
    first.push({ t: "event", d: event("01b") });
    first.push({ t: "event", d: event("01a") });
    assert.deepEqual(applied, ["01a", "01b"]);

    // The server goes away: the client retries, then asks for what it missed.
    first.close();
    assert.equal(timers.pending.length, 1);
    timers.fire();
    const second = FakeSocket.all[1];
    second.open();
    assert.deepEqual(second.sent[0].d, { topics: ["ws:acme", "issue:acme/HZN-1"], since: "01b" });

    second.push({ t: "resync", d: { topics: ["ws:acme"] } });
    assert.deepEqual(invalidated, [""]);

    s.unsubscribe(["issue:acme/HZN-1"]);
    assert.deepEqual(second.sent[1], { t: "unsub", id: second.sent[1].id, d: { topics: ["issue:acme/HZN-1"] } });

    s.stop();
    assert.equal(timers.pending.length, 0);
    assert.deepEqual(s.topics(), []);
  });
});
