// The client side of the step protocol (docs/realtime.md#protocol): local
// steps go to the server in batches against the confirmed version; batches
// from others arrive over the socket and the unconfirmed local steps are
// rebased over them (horizon/editor/transform.js); a rejected batch is
// rebased the same way and sent again. Joining (again, after a reconnect)
// catches up from the confirmed version.

import { schema } from "horizon/editor/model.js";
import { Mapping, rebase, stepFromJSON } from "horizon/editor/transform.js";

const SEND_DELAY = 50; // ms: steps typed meanwhile go in the same batch
const CURSOR_DELAY = 100; // ms between cursor updates
const CURSOR_REPEAT = 20_000; // ms: an idle cursor is sent again, so it doesn't expire
const KEEP_MAPS = 500; // confirmed steps' maps kept, to bring others' cursors forward
const OFFLINE_READONLY = 30_000; // ms without a connection before editing stops

let clients = 0;

export class Collab {
  /**
   * socket: the app's socket; ws, issue: the description's issue; editor:
   * { reset(doc, version), receive(doc, mapping), setOffline(bool),
   * remoteCursor(conn, cursor) }; onError(code).
   */
  constructor({ socket, ws, issue, editor, onError = () => {} }) {
    Object.assign(this, { socket, ws, issue, editor, onError });
    this.clientId = `${Date.now().toString(36)}-${(++clients).toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
    this.version = null; // confirmed
    this.confirmed = null; // the document at that version
    this.unconfirmed = []; // local steps, applied after it
    this.inflight = 0; // of them, those sent
    this.ready = false;
    this.maps = []; // [version before, map] of confirmed steps, oldest first
    this.pendingCursors = new Map(); // conn → cursor ahead of our version
  }

  start() {
    this.off = this.socket.on((m) => this.onMessage(m));
    this.join();
  }

  stop() {
    clearTimeout(this.timer);
    clearTimeout(this.offlineTimer);
    clearTimeout(this.cursorTimer);
    clearInterval(this.cursorRepeat);
    if (this.unconfirmed.length && !this.inflight) this.send();
    this.off?.();
    this.socket.send("doc.leave", { ws: this.ws, issue: this.issue });
  }

  join() {
    const d = { ws: this.ws, issue: this.issue };
    if (this.version !== null) d.version = this.version;
    this.socket.send("doc.join", d);
  }

  /** Steps the user just made (already in the editor's document). */
  local(steps) {
    this.unconfirmed.push(...steps);
    this.schedule();
  }

  schedule() {
    if (this.inflight || !this.ready || !this.unconfirmed.length || this.timer) return;
    this.timer = setTimeout(() => {
      this.timer = null;
      this.send();
    }, SEND_DELAY);
  }

  send() {
    if (this.inflight || !this.ready || !this.unconfirmed.length) return;
    const steps = this.unconfirmed.slice();
    if (!this.socket.send("doc.steps", { ws: this.ws, issue: this.issue, version: this.version, client: this.clientId, steps: steps.map((s) => s.toJSON()) })) return;
    this.inflight = steps.length;
  }

  onMessage(m) {
    if (m.t === "open") return this.reconnected();
    if (m.t === "close") return this.disconnected();
    if (m.d?.issue !== this.issue) return;
    switch (m.t) {
      case "doc.joined":
        return this.joined(m.d);
      case "doc.steps":
        return this.receive(m.d);
      case "doc.ack":
        if (m.d.version > this.version) this.confirm(m.d.version);
        return;
      case "doc.reject":
        this.inflight = 0;
        for (const b of m.d.steps ?? []) this.receive(b);
        return this.schedule();
      case "doc.error":
        return this.failed(m.d.code);
      case "doc.cursor":
        return this.remoteCursor(m.d);
    }
  }

  /** Records the maps of confirmed steps from version on. */
  remember(version, steps) {
    steps.forEach((s, i) => this.maps.push([version + i, s.getMap()]));
    if (this.maps.length > KEEP_MAPS) this.maps.splice(0, this.maps.length - KEEP_MAPS);
  }

  /** A local position as a position in the confirmed document. */
  toConfirmed(pos) {
    const inverted = [];
    for (let i = this.unconfirmed.length - 1; i >= 0; i--) inverted.push(this.unconfirmed[i].getMap().invert());
    return new Mapping(inverted).map(pos);
  }

  /** A position in the document at version, as a local position (null when ahead of us or too old). */
  fromVersion(pos, version) {
    if (version > this.version) return null;
    const first = this.maps.findIndex(([v]) => v === version);
    if (version < this.version && first < 0) return null;
    const maps = version < this.version ? this.maps.slice(first).map(([, m]) => m) : [];
    for (const s of this.unconfirmed) maps.push(s.getMap());
    return new Mapping(maps).map(pos);
  }

  /** Sends where the user's selection is, a moment later. */
  cursor(anchor, head) {
    this.lastCursor = { anchor, head };
    if (this.cursorTimer || !this.ready) return;
    this.cursorTimer = setTimeout(() => {
      this.cursorTimer = null;
      this.sendCursor();
    }, CURSOR_DELAY);
    if (!this.cursorRepeat) this.cursorRepeat = setInterval(() => this.sendCursor(), CURSOR_REPEAT);
  }

  sendCursor() {
    const c = this.lastCursor;
    if (!c || !this.ready) return;
    this.socket.send("doc.cursor", { ws: this.ws, issue: this.issue, version: this.version, anchor: this.toConfirmed(c.anchor), head: this.toConfirmed(c.head) });
  }

  remoteCursor(d) {
    if (d.gone) {
      this.pendingCursors.delete(d.conn);
      return this.editor.remoteCursor(d.conn, null);
    }
    if (d.version > this.version) {
      this.pendingCursors.set(d.conn, d); // shown once we have those steps
      return;
    }
    const anchor = this.fromVersion(d.anchor, d.version);
    const head = this.fromVersion(d.head, d.version);
    if (anchor === null || head === null) return;
    this.editor.remoteCursor(d.conn, { user: d.user, name: d.name, anchor, head });
  }

  /** Cursors that waited for steps we now have. */
  flushCursors() {
    for (const [conn, d] of this.pendingCursors) {
      if (d.version <= this.version) {
        this.pendingCursors.delete(conn);
        this.remoteCursor(d);
      }
    }
  }

  joined(d) {
    if (d.doc) {
      // From scratch: what wasn't confirmed can't be carried over.
      this.confirmed = schema.nodeFromJSON(d.doc);
      this.version = d.version;
      if (this.unconfirmed.length) this.onError("lost");
      this.unconfirmed = [];
      this.inflight = 0;
      this.maps = [];
      this.editor.reset(this.confirmed.toJSON(), this.version);
    }
    for (const b of d.steps ?? []) this.receive(b);
    this.inflight = 0; // a batch sent before is either among those, or to send again
    this.ready = true;
    this.editor.setOffline(false);
    this.schedule();
  }

  /** A batch accepted by the server: ours (confirmation) or someone else's. */
  receive(b) {
    if (this.version === null) return;
    if (b.version + b.steps.length <= this.version) return; // had it
    if (b.version !== this.version) return this.join(); // missed some: catch up
    if (b.client === this.clientId) return this.confirm(b.version + b.steps.length);
    let remote;
    let confirmed = this.confirmed;
    try {
      remote = b.steps.map((s) => stepFromJSON(s));
      for (const s of remote) confirmed = s.apply(confirmed);
    } catch {
      return this.failed("invalid_steps");
    }
    const { doc, steps, mapping } = rebase(this.confirmed, this.unconfirmed, remote);
    this.remember(this.version, remote);
    this.confirmed = confirmed;
    this.version += remote.length;
    this.unconfirmed = steps;
    this.editor.receive(doc, mapping);
    this.flushCursors();
  }

  /** Our first steps are in, up to version. */
  confirm(version) {
    const n = version - this.version;
    for (const s of this.unconfirmed.slice(0, n)) this.confirmed = s.apply(this.confirmed);
    this.remember(this.version, this.unconfirmed.slice(0, n));
    this.unconfirmed = this.unconfirmed.slice(n);
    this.version = version;
    this.inflight = 0;
    this.schedule();
    this.flushCursors();
  }

  failed(code) {
    // Out of step with the server: start over from its document.
    this.onError(code);
    this.version = null;
    this.unconfirmed = [];
    this.inflight = 0;
    this.ready = false;
    if (code !== "forbidden" && code !== "issue_not_found" && code !== "issue_archived") this.join();
  }

  reconnected() {
    clearTimeout(this.offlineTimer);
    this.inflight = 0;
    this.join();
  }

  disconnected() {
    this.ready = false;
    clearTimeout(this.offlineTimer);
    this.offlineTimer = setTimeout(() => this.editor.setOffline(true), OFFLINE_READONLY);
  }
}
