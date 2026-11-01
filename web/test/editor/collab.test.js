// The client side of the step protocol against a fake server that behaves
// like internal/issue's: a batch applies only to the current version, else
// the client gets what it missed. Messages are delivered in a shuffled but
// per-connection ordered way, so batches cross.

import { Collab } from "horizon/editor/collab.js";
import { insertText } from "horizon/editor/commands.js";
import { schema } from "horizon/editor/model.js";
import { EditorState, Selection } from "horizon/editor/state.js";
import { stepFromJSON } from "horizon/editor/transform.js";
import { assert, describe, test } from "horizon-test/runner.js";

function fakeServer(docJSON) {
  const server = { doc: schema.nodeFromJSON(docJSON), version: 0, batches: [], sockets: [] };
  server.socket = (rand) => {
    const listeners = new Set();
    const inbox = []; // to the client, in order
    const sock = {
      open: true,
      on: (fn) => (listeners.add(fn), () => listeners.delete(fn)),
      send(t, d) {
        // The server answers at once; the client receives later.
        if (t === "doc.join") {
          const since = d.version ?? -1;
          const missing = server.batches.filter((b) => b.version >= since);
          inbox.push({ t: "doc.joined", d: since < 0 ? { issue: d.issue, version: server.version, doc: server.doc.toJSON(), steps: [] } : { issue: d.issue, version: server.version, steps: missing } });
        } else if (t === "doc.steps") {
          if (d.version !== server.version) {
            inbox.push({ t: "doc.reject", d: { issue: d.issue, version: server.version, steps: server.batches.filter((b) => b.version >= d.version) } });
          } else {
            for (const s of d.steps) server.doc = stepFromJSON(s).apply(server.doc);
            const batch = { version: server.version, steps: d.steps, client: d.client };
            server.batches.push(batch);
            server.version += d.steps.length;
            inbox.push({ t: "doc.ack", d: { issue: d.issue, version: server.version } });
            for (const other of server.sockets) other.inbox.push({ t: "doc.steps", d: { issue: d.issue, ...batch } });
          }
        }
        return true;
      },
      inbox,
      deliver() {
        const m = inbox.shift();
        if (m) for (const fn of [...listeners]) fn(m);
        return !!m;
      },
    };
    server.sockets.push(sock);
    return sock;
  };
  return server;
}

/** An editor stand-in: a state that commands and the collab drive. */
function fakeEditor() {
  const ed = {
    state: null,
    collab: null,
    reset(json) {
      const doc = schema.nodeFromJSON(json);
      ed.state = new EditorState(doc, Selection.atEnd(doc));
    },
    receive(doc, mapping) {
      ed.state = new EditorState(doc, ed.state.selection.map(mapping));
    },
    setOffline() {},
    type(text, atStart = false) {
      if (atStart) ed.state = new EditorState(ed.state.doc, Selection.atStart(ed.state.doc));
      insertText(text)(ed.state, (tr) => {
        ed.state = ed.state.apply(tr);
        ed.collab.local(tr.steps);
      });
    },
  };
  return ed;
}

describe("collaboration", () => {
  test("two clients typing at once converge with the server", async () => {
    const server = fakeServer({ type: "doc", content: [{ type: "paragraph", content: [{ type: "text", text: "middle" }] }] });
    let seed = 7;
    const rand = () => ((seed = (seed * 16807) % 2147483647) / 2147483647);
    const clients = [0, 1].map(() => {
      const sock = server.socket(rand);
      const ed = fakeEditor();
      ed.collab = new Collab({ socket: sock, ws: "acme", issue: "HZN-1", editor: ed });
      ed.collab.start();
      return { sock, ed };
    });
    const pump = async (rounds) => {
      for (let i = 0; i < rounds; i++) {
        const c = clients[Math.floor(rand() * 2)];
        c.sock.deliver();
        await new Promise((r) => setTimeout(r, 0));
      }
    };
    await pump(10);
    for (let i = 0; i < 20; i++) {
      clients[0].ed.type("a");
      clients[1].ed.type("b", i === 0);
      await pump(4);
      await new Promise((r) => setTimeout(r, 60)); // the send delay
    }
    // Drain everything.
    for (let i = 0; i < 200; i++) {
      await new Promise((r) => setTimeout(r, 60));
      let busy = false;
      for (const c of clients) while (c.sock.deliver()) busy = true;
      if (!busy && clients.every((c) => !c.ed.collab.unconfirmed.length)) break;
    }
    const text = (ed) => ed.state.doc.textContent();
    const want = server.doc.textContent();
    assert.equal(text(clients[0].ed), want);
    assert.equal(text(clients[1].ed), want);
    assert.equal(want.split("a").length - 1, 20);
    assert.equal(want.split("b").length - 1, 20);
    assert.ok(want.includes("middle"));
  }, { timeout: 30000 });
});
