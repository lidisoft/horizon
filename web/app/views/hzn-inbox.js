// <hzn-inbox>: /w/:ws/inbox — one row per issue the user follows, bumped by
// each new event (docs/domain.md#notifications). Opening a row reads it;
// rows can be marked read or unread, snoozed and archived. Pushed inbox
// changes refresh it.

import { describeEvent, issueHref, issueStyles } from "horizon/components/issue-ui.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const STATES = ["unread", "all", "snoozed"];
const OUTCOME_ICONS = { unstarted: "circle", started: "circle-dot", completed: "circle-check", abandoned: "circle-x" };

const styles = sheet(`
  :host { display: flex; flex-direction: column; min-block-size: 100%; }
  header.bar { display: flex; align-items: center; gap: var(--lds-space-3); flex-wrap: wrap;
    min-block-size: 3.25rem; padding: var(--lds-space-2) var(--lds-space-5); border-block-end: 1px solid var(--lds-color-border-default); }
  header h1 { margin: 0; font-size: var(--lds-font-size-md); font-weight: var(--lds-font-weight-semibold); }
  .grow { flex: 1; }
  .rows { display: grid; }
  .row { display: grid; grid-template-columns: 0.5rem 1.25rem minmax(0, 1fr) auto; gap: var(--lds-space-1) var(--lds-space-3); align-items: center;
    padding: var(--lds-space-3) var(--lds-space-5); border-block-end: 1px solid var(--lds-color-border-subtle); }
  .row:hover, .row:focus-within { background: var(--lds-color-bg-subtle); }
  .dot { inline-size: 0.5rem; block-size: 0.5rem; border-radius: var(--lds-radius-full); }
  .row[data-unread] .dot { background: var(--lds-color-accent-fg); }
  .main { display: grid; gap: var(--lds-space-1); min-inline-size: 0; color: inherit; text-decoration: none; }
  .main:focus-visible { outline: 2px solid var(--lds-color-focus-ring, var(--lds-color-accent-fg)); outline-offset: 2px; border-radius: var(--lds-radius-xs); }
  .line { display: flex; gap: var(--lds-space-2); align-items: baseline; min-inline-size: 0; }
  .title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .row[data-unread] .title { font-weight: var(--lds-font-weight-semibold); }
  .what { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .side { display: flex; align-items: center; gap: var(--lds-space-2); }
  .when { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-xs); white-space: nowrap; }
  .actions { display: flex; gap: var(--lds-space-1); }
  .more { padding: var(--lds-space-4) var(--lds-space-5); }
  .empty { padding: var(--lds-space-12) var(--lds-space-6); }
`);

class HznInbox extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [issueStyles, styles];
    const state = this.context.router.current?.query.get("state");
    this.state = STATES.includes(state) ? state : "unread";
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("lds-change", (e) => {
      if (e.target.matches?.("[data-state]")) this.setState(e.detail.value);
    });
    this.offSocket = this.context.socket.on((m) => {
      if ((m.t === "inbox" && m.d?.workspace === this.ws) || m.t === "open") {
        clearTimeout(this.timer);
        this.timer = setTimeout(() => this.load(), 200);
      }
    });
    this.load();
  }

  disconnectedCallback() {
    this.offSocket?.();
    clearTimeout(this.timer);
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.load();
  }

  get ws() {
    return this.getAttribute("ws");
  }

  get base() {
    return `/w/${encodeURIComponent(this.ws)}/inbox`;
  }

  setState(state) {
    if (!STATES.includes(state) || state === this.state) return;
    this.state = state;
    this.context.router.navigate(state === "unread" ? `/w/${this.ws}/inbox` : `/w/${this.ws}/inbox?state=${state}`, { replace: true });
    this.load();
  }

  /** Loads the first page again (more pages are dropped). */
  async load() {
    const { api, i18n } = this.context;
    const seq = (this.seq = (this.seq ?? 0) + 1);
    try {
      const { data } = await api.get(this.base, { query: { state: this.state } });
      if (seq !== this.seq) return;
      this.items = data.items;
      this.next = data.next_cursor;
      this.error = "";
    } catch (err) {
      if (seq !== this.seq) return;
      this.items = [];
      this.error = errorMessage(err, i18n);
    }
    this.render();
  }

  async loadMore() {
    const { api, i18n } = this.context;
    const seq = this.seq;
    try {
      const { data } = await api.get(this.base, { query: { state: this.state, cursor: this.next } });
      if (seq !== this.seq) return;
      this.items.push(...data.items);
      this.next = data.next_cursor;
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
    this.render();
  }

  render() {
    const { i18n } = this.context;
    const { t } = i18n;
    const focused = this.shadowRoot.activeElement?.closest?.(".row")?.dataset.id;
    const unread = this.items.some((i) => !i.read);
    this.shadowRoot.innerHTML = String(html`
      <header class="bar">
        <h1>${t("inbox.title")}</h1>
        <lds-segmented size="sm" data-state value="${this.state}" label="${t("inbox.show")}">
          ${STATES.map((s) => html`<lds-segment value="${s}">${t(`inbox.states.${s}`)}</lds-segment>`)}
        </lds-segmented>
        <span class="grow"></span>
        ${this.state !== "snoozed" ? html`
          <lds-button size="sm" variant="ghost" data-action="read-all" ${unread ? "" : html`disabled`}><lds-icon slot="start" name="check"></lds-icon>${t("inbox.readAll")}</lds-button>
          <lds-button size="sm" variant="ghost" data-action="archive-read"><lds-icon slot="start" name="archive"></lds-icon>${t("inbox.archiveRead")}</lds-button>` : ""}
      </header>
      ${this.error ? html`<lds-alert intent="danger">${this.error}</lds-alert>` : ""}
      ${!this.error && !this.items.length ? html`
        <lds-empty-state class="empty"><lds-icon slot="icon" name="inbox"></lds-icon>
          <h2 slot="title">${t(`inbox.empty.${this.state}`)}</h2>${t("inbox.emptyHint")}</lds-empty-state>` : ""}
      <div class="rows" role="list" aria-label="${t("inbox.title")}">${this.items.map((it) => this.row(it))}</div>
      ${this.next ? html`<div class="more"><lds-button size="sm" variant="secondary" data-action="more">${t("inbox.more")}</lds-button></div>` : ""}`);
    if (focused) this.shadowRoot.querySelector(`.row[data-id="${focused}"] .main`)?.focus();
  }

  row(it) {
    const { i18n } = this.context;
    const { t } = i18n;
    const e = it.event;
    const what = e ? `${e.actor ? e.actor.name || t("issue.someone") : t("issue.system")} ${e.type === "issue.commented" ? t("inbox.commented") : describeEvent(i18n, e)}` : "";
    return html`
      <div class="row" role="listitem" data-id="${it.id}" ${it.read ? "" : html`data-unread`}>
        <span class="dot" aria-label="${it.read ? "" : t("inbox.unread")}"></span>
        <span class="glyph status" data-outcome="${it.issue.status.outcome}"><lds-icon name="${OUTCOME_ICONS[it.issue.status.outcome] ?? "circle"}" size="sm" label="${it.issue.status.name}"></lds-icon></span>
        <a class="main" href="${issueHref(this.ws, it.issue.key)}" data-open>
          <span class="line"><span class="key">${it.issue.key}</span><span class="title">${it.issue.title}</span></span>
          <span class="line">
            ${it.reason !== "subscribed" ? html`<lds-badge size="sm" intent="${it.reason === "assigned" ? "info" : "warning"}">${t(`inbox.reason.${it.reason}`)}</lds-badge>` : ""}
            <span class="what">${what}${it.unread_count > 1 ? ` · ${t("inbox.moreUpdates", { count: it.unread_count - 1 })}` : ""}</span>
          </span>
        </a>
        <span class="side">
          <span class="when">${it.snoozed_until ? t("inbox.snoozedUntil", { when: i18n.relative(it.snoozed_until) }) : i18n.relative(it.updated_at)}</span>
          <span class="actions">
            <lds-button size="sm" variant="ghost" data-action="toggle-read" label="${t(it.read ? "inbox.markUnread" : "inbox.markRead")}">
              <lds-icon name="${it.read ? "circle-dot" : "eye"}"></lds-icon></lds-button>
            <lds-menu placement="bottom-end" data-snooze>
              <lds-button slot="trigger" size="sm" variant="ghost" label="${t("inbox.snooze")}"><lds-icon name="history"></lds-icon></lds-button>
              ${it.snoozed_until ? html`<lds-menu-item data-action="wake">${t("inbox.wake")}</lds-menu-item>` : ""}
              ${["hour", "tomorrow", "week"].map((s) => html`<lds-menu-item data-action="snooze" data-until="${s}">${t(`inbox.snoozeFor.${s}`)}</lds-menu-item>`)}
            </lds-menu>
            <lds-button size="sm" variant="ghost" data-action="archive" label="${t("inbox.archive")}"><lds-icon name="archive"></lds-icon></lds-button>
          </span>
        </span>
      </div>`;
  }

  async onClick(e) {
    const open = e.target.closest?.("[data-open]");
    if (open) {
      const id = open.closest(".row").dataset.id;
      if (!this.items.find((i) => i.id === id)?.read) this.patch(id, { read: true }, false);
      return; // the router follows the link
    }
    const el = e.target.closest?.("[data-action]");
    if (!el) return;
    const { api, i18n } = this.context;
    const id = el.closest(".row")?.dataset.id;
    const item = this.items.find((i) => i.id === id);
    try {
      switch (el.dataset.action) {
        case "more":
          return this.loadMore();
        case "read-all":
          await api.post(`${this.base}/read-all`);
          return this.load();
        case "archive-read":
          await api.post(`${this.base}/archive-read`);
          return this.load();
        case "toggle-read":
          return this.patch(id, { read: !item.read });
        case "archive":
          return this.patch(id, { archived: true });
        case "snooze":
          return this.snooze(id, el.dataset.until);
        case "wake":
          return this.patch(id, { snoozed_until: null });
      }
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
  }

  /** Snoozes a row for an hour, until tomorrow 08:00 or next Monday 08:00. */
  snooze(id, until) {
    const d = new Date();
    if (until === "hour") d.setHours(d.getHours() + 1);
    else {
      d.setDate(d.getDate() + (until === "week" ? ((8 - d.getDay()) % 7 || 7) : 1));
      d.setHours(8, 0, 0, 0);
    }
    this.patch(id, { snoozed_until: d.toISOString() });
  }

  /** Changes a row, then reloads (reload: false when navigating away). */
  async patch(id, body, reload = true) {
    const { api, i18n } = this.context;
    try {
      await api.patch(`${this.base}/${id}`, { body });
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      return;
    }
    if (reload) this.load();
  }
}

customElements.define("hzn-inbox", HznInbox);
