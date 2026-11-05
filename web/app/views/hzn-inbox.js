// <hzn-inbox>: /w/:ws/inbox — one row per issue the user follows, bumped by
// each new event (docs/domain.md#notifications), beside the selected
// issue (wireframes: Inbox). Selecting a row reads it and shows the issue;
// it can be marked unread, snoozed, archived (Done) or opened. Pushed inbox
// changes refresh it.

import "horizon/components/hzn-issue-detail.js";
import { describeEvent, issueHref, issueStyles } from "horizon/components/issue-ui.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const STATES = ["unread", "all", "snoozed"];
const OUTCOME_ICONS = { unstarted: "circle", started: "circle-dot", completed: "circle-check", abandoned: "circle-x" };

const styles = sheet(`
  :host { display: grid; grid-template-columns: minmax(18rem, 25rem) minmax(0, 1fr); block-size: 100%; min-block-size: 0; }
  .list { display: flex; flex-direction: column; min-block-size: 0; border-inline-end: 1px solid var(--lds-color-border-default); }
  .pane { display: flex; flex-direction: column; min-inline-size: 0; min-block-size: 0; }
  header.bar { display: flex; align-items: center; gap: var(--lds-space-2); flex: none; box-sizing: border-box;
    block-size: 3.25rem; padding: 0 var(--lds-space-4); border-block-end: 1px solid var(--lds-color-border-default); }
  header h1 { margin: 0; font-size: var(--lds-font-size-sm); font-weight: var(--lds-font-weight-semibold); }
  header .what { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .grow { flex: 1; }
  .rows { display: grid; align-content: start; overflow: auto; flex: 1; }
  .row { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: var(--lds-space-3); align-items: start; position: relative;
    padding: var(--lds-space-3) var(--lds-space-4); border-block-end: 1px solid var(--lds-color-border-subtle); }
  .row:hover { background: var(--lds-color-bg-subtle); }
  .row[aria-current="true"] { background: var(--lds-color-bg-muted, var(--lds-color-bg-subtle)); }
  .main { display: grid; gap: 2px; min-inline-size: 0; padding: 0; border: 0; background: none; color: inherit; font: inherit; text-align: start; cursor: pointer; }
  .main::after { content: ""; position: absolute; inset: 0; }
  .main:focus-visible { outline: none; }
  .row:has(.main:focus-visible) { box-shadow: inset 0 0 0 2px var(--lds-color-accent-default); }
  .line { display: flex; gap: var(--lds-space-2); align-items: center; min-inline-size: 0; }
  .line .key { font-size: var(--lds-font-size-xs); }
  .title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .row[data-unread] .title { font-weight: var(--lds-font-weight-semibold); }
  .row .what { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-xs); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .when { margin-inline-start: auto; color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-xs); white-space: nowrap; }
  .dot { inline-size: 0.5rem; block-size: 0.5rem; border-radius: var(--lds-radius-full); flex: none; }
  .row[data-unread] .dot { background: var(--lds-color-accent-fg); }
  .actions { display: flex; gap: var(--lds-space-2); }
  .detail { flex: 1; overflow: auto; padding: var(--lds-space-6) var(--lds-space-8); }
  .detail hzn-issue-detail { max-inline-size: 48rem; }
  .more { padding: var(--lds-space-4); }
  .empty { padding: var(--lds-space-12) var(--lds-space-6); }
  @media (max-width: 40rem) {
    :host { grid-template-columns: minmax(0, 1fr); }
    .pane { display: none; }
  }
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
      // The row being read stays in place under Unread until another is picked.
      const before = this.items?.findIndex((i) => i.id === this.selected) ?? -1;
      if (before >= 0 && !data.items.some((i) => i.id === this.selected) && this.state === "unread") {
        data.items.splice(Math.min(before, data.items.length), 0, { ...this.items[before], read: true });
      }
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
    const selected = this.items.find((i) => i.id === this.selected) ?? null;
    if (!this.shadowRoot.querySelector(".list")) {
      this.shadowRoot.innerHTML = String(html`<section class="list"></section>
        <section class="pane" aria-label="${t("inbox.selected")}"><div data-pane-bar></div><div class="detail" data-detail></div></section>`);
    }
    this.shadowRoot.querySelector(".list").innerHTML = String(html`
        <header class="bar">
          <h1>${t("inbox.title")}</h1>
          <span class="grow"></span>
          <lds-segmented size="sm" data-state value="${this.state}" label="${t("inbox.show")}">
            ${STATES.map((s) => html`<lds-segment value="${s}">${t(`inbox.states.${s}`)}</lds-segment>`)}
          </lds-segmented>
          ${this.state !== "snoozed" ? html`
            <lds-menu placement="bottom-end">
              <lds-button slot="trigger" size="sm" variant="ghost" label="${t("issues.more")}"><lds-icon name="more-horizontal"></lds-icon></lds-button>
              <lds-menu-item data-action="read-all" ${unread ? "" : html`disabled`}><lds-icon slot="start" name="check"></lds-icon>${t("inbox.readAll")}</lds-menu-item>
              <lds-menu-item data-action="archive-read"><lds-icon slot="start" name="archive"></lds-icon>${t("inbox.archiveRead")}</lds-menu-item>
            </lds-menu>` : ""}
        </header>
        ${this.error ? html`<lds-alert intent="danger">${this.error}</lds-alert>` : ""}
        ${!this.error && !this.items.length ? html`
          <lds-empty-state class="empty"><lds-icon slot="icon" name="inbox"></lds-icon>
            <h2 slot="title">${t(`inbox.empty.${this.state}`)}</h2>${t("inbox.emptyHint")}</lds-empty-state>` : ""}
        <div class="rows" role="list" aria-label="${t("inbox.title")}">
          ${this.items.map((it) => this.row(it))}
          ${this.next ? html`<div class="more"><lds-button size="sm" variant="secondary" data-action="more">${t("inbox.more")}</lds-button></div>` : ""}
        </div>`);
    const bar = String(selected ? html`
          <header class="bar" data-id="${selected.id}">
            <span class="what"><span class="key">${selected.issue.key}</span> · ${this.what(selected)}</span>
            <span class="grow"></span>
            <span class="actions">
              <lds-menu placement="bottom-end" data-snooze>
                <lds-button slot="trigger" size="sm" variant="secondary"><lds-icon slot="start" name="history" size="sm"></lds-icon>${t("inbox.snooze")}</lds-button>
                ${selected.snoozed_until ? html`<lds-menu-item data-action="wake">${t("inbox.wake")}</lds-menu-item>` : ""}
                ${["hour", "tomorrow", "week"].map((s) => html`<lds-menu-item data-action="snooze" data-until="${s}">${t(`inbox.snoozeFor.${s}`)}</lds-menu-item>`)}
              </lds-menu>
              <lds-button size="sm" variant="secondary" data-action="toggle-read">${t(selected.read ? "inbox.markUnread" : "inbox.markRead")}</lds-button>
              <lds-button size="sm" variant="secondary" data-action="archive"><lds-icon slot="start" name="check" size="sm"></lds-icon>${t("inbox.done")}</lds-button>
              <lds-button size="sm" variant="primary" data-open="${issueHref(this.ws, selected.issue.key)}">${t("inbox.openIssue")}</lds-button>
            </span>
          </header>` : "");
    // Unchanged controls stay, so a press isn't lost to a refresh.
    const barBox = this.shadowRoot.querySelector("[data-pane-bar]");
    if (barBox.dataset.html !== bar) {
      barBox.dataset.html = bar;
      barBox.innerHTML = bar;
    }
    // The issue stays mounted while it is the one selected: rows refresh
    // around it as the inbox changes.
    const detail = this.shadowRoot.querySelector("[data-detail]");
    const key = selected?.issue.key ?? "";
    if (detail.dataset.key !== key) {
      detail.dataset.key = key;
      detail.innerHTML = String(selected
        ? html`<hzn-issue-detail mode="peek" ws="${this.ws}" issue="${key}"></hzn-issue-detail>`
        : html`<lds-empty-state class="empty"><lds-icon slot="icon" name="inbox"></lds-icon>
            <h2 slot="title">${t("inbox.pick")}</h2>${t("inbox.pickHint")}</lds-empty-state>`);
    }
    if (focused) this.shadowRoot.querySelector(`.row[data-id="${focused}"] .main`)?.focus();
  }

  /** Who did what last on the row's issue. */
  what(it) {
    const { i18n } = this.context;
    const { t } = i18n;
    const e = it.event;
    if (!e) return "";
    return `${e.actor ? e.actor.name || t("issue.someone") : t("issue.system")} ${e.type === "issue.commented" ? t("inbox.commented") : describeEvent(i18n, e)}`;
  }

  row(it) {
    const { i18n } = this.context;
    const { t } = i18n;
    return html`
      <div class="row" role="listitem" data-id="${it.id}" ${it.read ? "" : html`data-unread`} ${it.id === this.selected ? html`aria-current="true"` : ""}>
        <lds-avatar size="sm" name="${it.event?.actor?.name ?? ""}"></lds-avatar>
        <button type="button" class="main" data-select>
          <span class="line">
            <span class="glyph status" data-outcome="${it.issue.status.outcome}"><lds-icon name="${OUTCOME_ICONS[it.issue.status.outcome] ?? "circle"}" size="sm" label="${it.issue.status.name}"></lds-icon></span>
            <span class="key">${it.issue.key}</span>
            <span class="when">${it.snoozed_until ? t("inbox.snoozedUntil", { when: i18n.relative(it.snoozed_until) }) : i18n.relative(it.updated_at)}</span>
            <span class="dot" aria-label="${it.read ? "" : t("inbox.unread")}"></span>
          </span>
          <span class="title">${it.issue.title}</span>
          <span class="line">
            ${it.reason !== "subscribed" ? html`<lds-badge size="sm" intent="${it.reason === "assigned" ? "info" : "warning"}">${t(`inbox.reason.${it.reason}`)}</lds-badge>` : ""}
            <span class="what">${this.what(it)}${it.unread_count > 1 ? ` · ${t("inbox.moreUpdates", { count: it.unread_count - 1 })}` : ""}</span>
          </span>
        </button>
      </div>`;
  }

  /** Shows a row's issue beside the list and reads the row. */
  select(id) {
    const item = this.items.find((i) => i.id === id);
    // Too narrow for the pane: the issue's page instead.
    if (item && matchMedia("(max-width: 40rem)").matches) {
      if (!item.read) this.patch(id, { read: true }, false);
      return this.context.router.navigate(issueHref(this.ws, item.issue.key));
    }
    this.selected = id;
    this.render();
    if (item && !item.read) this.patch(id, { read: true });
  }

  async onClick(e) {
    const pick = e.target.closest?.("[data-select]");
    if (pick) return this.select(pick.closest(".row").dataset.id);
    const open = e.target.closest?.("[data-open]");
    if (open) return this.context.router.navigate(open.dataset.open);
    const el = e.target.closest?.("[data-action]");
    if (!el) return;
    const { api, i18n } = this.context;
    const id = el.closest(".row, header[data-id]")?.dataset.id;
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
