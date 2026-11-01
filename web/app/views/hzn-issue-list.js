// <hzn-issue-list>: /w/:ws/p/:key/issues — a project's issues grouped by
// status, in rank order (wireframes: Main, IssuePeek). A row opens the peek
// panel beside the list; o or a double click opens the issue's page. j/k
// move between rows, Enter peeks, c creates. Pushed events refresh it.
// The view bar filters it and picks saved views (?view=, ?filter=).

import "horizon/components/hzn-issue-create.js";
import "horizon/components/hzn-issue-detail.js";
import "horizon/components/hzn-view-bar.js";
import {
  avatar, changesProjectData, forgetProjects, issueHref, issueStyles, labelChips, priorityGlyph, projectData, shortDate, statusGlyph,
} from "horizon/components/issue-ui.js";
import { fetchIssues, planningStyles, viewSwitch } from "horizon/components/planning-ui.js";
import { paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const styles = sheet(`
  :host { display: flex; flex-direction: column; min-block-size: 100%; }
  header.bar { display: flex; align-items: center; justify-content: space-between; gap: var(--lds-space-3);
    min-block-size: 3.25rem; padding: 0 var(--lds-space-5); border-block-end: 1px solid var(--lds-color-border-default); }
  .crumb { display: flex; align-items: center; gap: var(--lds-space-2); }
  .crumb strong { font-weight: var(--lds-font-weight-semibold); }
  .crumb lds-icon { color: var(--lds-color-fg-subtle); }
  .swatch { display: inline-block; inline-size: 0.625rem; block-size: 0.625rem; border-radius: var(--lds-radius-xs); }
  .toolbar { display: flex; align-items: center; gap: var(--lds-space-2); padding: var(--lds-space-2) var(--lds-space-5);
    border-block-end: 1px solid var(--lds-color-border-subtle); }
  .toolbar .grow { flex: 1; }
  .group { display: flex; align-items: center; gap: var(--lds-space-2); block-size: 2.25rem; padding: 0 var(--lds-space-5);
    background: var(--lds-color-bg-subtle); border-block-end: 1px solid var(--lds-color-border-subtle);
    font-size: var(--lds-font-size-sm); font-weight: var(--lds-font-weight-semibold); }
  .group .count { color: var(--lds-color-fg-muted); font-weight: normal; }
  .row { display: grid; grid-template-columns: 1.25rem 4.5rem 1.25rem minmax(0, 1fr) auto 2rem 3.5rem 1.75rem;
    align-items: center; gap: var(--lds-space-3); block-size: 2.5rem; padding: 0 var(--lds-space-5);
    border-block-end: 1px solid var(--lds-color-border-subtle); cursor: pointer; outline: none; }
  .row:hover { background: var(--lds-color-bg-subtle); }
  .row:focus-visible, .row[aria-current="true"] { background: var(--lds-color-accent-subtle); }
  .title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .labels { display: flex; gap: var(--lds-space-1); justify-content: flex-end; }
  .num, .due { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); text-align: end; font-variant-numeric: tabular-nums; }
  .empty { padding: var(--lds-space-12) var(--lds-space-6); }
  lds-drawer { --lds-drawer-width: min(34rem, 100vw); }
  [slot="header"] { display: flex; align-items: center; gap: var(--lds-space-2); font-size: var(--lds-font-size-sm); }
  @media (max-width: 48rem) {
    .row { grid-template-columns: 1.25rem 4.5rem 1.25rem minmax(0, 1fr) 1.75rem; }
    .labels, .num, .due { display: none; }
  }
`);

class HznIssueList extends HTMLElement {
  static observedAttributes = ["ws", "key"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [issueStyles, planningStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("dblclick", (e) => {
      const row = e.target.closest?.(".row");
      if (row) this.open(row.dataset.key);
    });
    const { shortcuts, socket } = this.context;
    this.offScope = shortcuts.pushScope("issue-list");
    this.offKeys = [
      shortcuts.register({ id: "list-down", keys: "j", scope: "issue-list", description: "shortcut.next", handler: () => this.move(1) }),
      shortcuts.register({ id: "list-up", keys: "k", scope: "issue-list", description: "shortcut.previous", handler: () => this.move(-1) }),
      shortcuts.register({ id: "list-peek", keys: "enter", scope: "issue-list", description: "shortcut.peek", handler: () => this.peek(this.current) }),
      shortcuts.register({ id: "list-open", keys: "o", scope: "issue-list", description: "shortcut.open", handler: () => this.current && this.open(this.current) }),
    ];
    this.offSocket = socket.on((m) => this.onPush(m));
    this.onCreated = (e) => {
      if (e.detail?.key?.startsWith(`${this.data?.project.key}-`)) this.reload();
    };
    document.addEventListener("hzn-issue-changed", this.onCreated);
    root.addEventListener("hzn-view-change", () => this.reload());
    this.load();
  }

  disconnectedCallback() {
    this.offScope?.();
    for (const off of this.offKeys ?? []) off();
    this.offSocket?.();
    document.removeEventListener("hzn-issue-changed", this.onCreated);
    clearTimeout(this.timer);
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.load();
  }

  get ws() {
    return this.getAttribute("ws");
  }

  async load() {
    const { i18n, router } = this.context;
    const key = this.getAttribute("key");
    try {
      this.data = await projectData(this.context, this.ws, key);
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`
        <lds-empty-state class="empty"><lds-icon slot="icon" name="search"></lds-icon>
          <h2 slot="title">${err.status === 404 ? i18n.t("projects.notFound") : errorMessage(err, i18n)}</h2>
          <a href="/w/${this.ws}">${i18n.t("notFound.home")}</a>
        </lds-empty-state>`);
      return;
    }
    if (this.data.project.key !== key.toUpperCase()) {
      router.navigate(`/w/${this.ws}/p/${this.data.project.key}/issues`, { replace: true }); // a former key
      return;
    }
    this.renderFrame();
    this.bar = this.shadowRoot.querySelector("hzn-view-bar");
    await this.bar.setup({ ws: this.ws, data: this.data, layout: "list" });
    await this.reload();
  }

  /** Fetches every page of the list: the project's issues, or those the filter matches. */
  async reload() {
    const seq = (this.seq = (this.seq ?? 0) + 1); // a later reload wins over this one
    const { i18n } = this.context;
    let items;
    try {
      items = await fetchIssues(this.context, this.ws, this.data.project.key, this.bar?.filter);
    } catch (err) {
      if (seq !== this.seq) return;
      this.shadowRoot.querySelector("[data-list]").innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    if (seq !== this.seq) return;
    this.items = items;
    this.renderList();
  }

  renderFrame() {
    const { t } = this.context.i18n;
    const p = this.data.project;
    const canCreate = p.can.includes("issue.create") && !p.archived_at;
    this.shadowRoot.innerHTML = String(html`
      <header class="bar">
        <div class="crumb">
          <span class="swatch" data-colour="${p.colour}"></span><strong>${p.name}</strong>
          <lds-icon name="chevron-right" size="sm"></lds-icon><span>${t("issues.title")}</span>
        </div>
        ${canCreate ? html`<lds-button variant="primary" size="sm" data-action="create"><lds-icon slot="start" name="plus" size="sm"></lds-icon>${t("issues.new")}</lds-button>` : ""}
      </header>
      ${p.archived_at ? html`<lds-alert intent="warning">${t("projects.isArchived")}</lds-alert>` : ""}
      <div class="toolbar">${viewSwitch(t, this.ws, p.key, "issues")}<hzn-view-bar></hzn-view-bar></div>
      <div role="list" aria-label="${t("issues.byStatus")}" data-list><lds-skeleton></lds-skeleton></div>
      <lds-drawer non-modal size="md" data-peek>
        <span slot="header" data-peek-title></span>
        <lds-button slot="actions" variant="ghost" size="sm" data-action="open" label="${t("issues.openPage")}"><lds-icon name="external-link" size="sm"></lds-icon></lds-button>
        <lds-button slot="actions" variant="ghost" size="sm" data-action="copy" label="${t("issues.copyLink")}"><lds-icon name="link" size="sm"></lds-icon></lds-button>
      </lds-drawer>
      <hzn-issue-create></hzn-issue-create>`);
    paintSwatches(this.shadowRoot);
    this.shadowRoot.querySelector("[data-peek]").addEventListener("lds-close", () => {
      this.peeked = null;
      this.markRows();
    });
  }

  renderList() {
    const { i18n } = this.context;
    const { t } = i18n;
    const box = this.shadowRoot.querySelector("[data-list]");
    if (!this.items.length) {
      box.innerHTML = String(html`
        <lds-empty-state class="empty"><lds-icon slot="icon" name="${this.bar?.filter ? "filter" : "list"}"></lds-icon>
          <h2 slot="title">${this.bar?.filter ? t("views.noMatch") : t("issues.empty")}</h2>${this.bar?.filter ? "" : t("issues.emptyHint")}
        </lds-empty-state>`);
      this.rows = [];
      return;
    }
    const groups = new Map();
    for (const i of this.items) {
      if (!groups.has(i.status_id)) groups.set(i.status_id, []);
      groups.get(i.status_id).push(i);
    }
    const order = [...groups.keys()].sort((a, b) => (this.data.statuses.get(a)?.position ?? 0) - (this.data.statuses.get(b)?.position ?? 0));
    box.innerHTML = String(html`${order.map((statusId) => {
      const s = this.data.statuses.get(statusId);
      const rows = groups.get(statusId);
      return html`
        <section aria-label="${s?.name ?? ""}">
          <div class="group">${statusGlyph(this.data, statusId)}<span>${s?.name ?? "?"}</span> <span class="count">${rows.length}</span></div>
          ${rows.map((i) => html`
            <div class="row" role="listitem" tabindex="-1" data-key="${i.key}" aria-label="${i.key} ${i.title}">
              ${priorityGlyph(i.priority, t)}
              <span class="key">${i.key}</span>
              ${statusGlyph(this.data, i.status_id)}
              <span class="title">${i.title}</span>
              <span class="labels">${labelChips(this.data, i.labels)}</span>
              <span class="num">${i.estimate ?? ""}</span>
              <span class="due">${shortDate(i.due_date, i18n)}</span>
              <span>${avatar(this.data, i.assignee_id)}</span>
            </div>`)}
        </section>`;
    })}`);
    paintSwatches(box);
    this.rows = [...box.querySelectorAll(".row")];
    if (!this.rows.some((r) => r.dataset.key === this.current)) this.current = null;
    this.markRows();
  }

  markRows() {
    for (const r of this.rows ?? []) {
      r.setAttribute("aria-current", String(r.dataset.key === (this.peeked ?? this.current)));
      r.tabIndex = r.dataset.key === this.current ? 0 : -1;
    }
  }

  /** Moves the current row by delta (j/k), keeping the peek in step. */
  move(delta) {
    if (!this.rows?.length) return;
    const i = this.rows.findIndex((r) => r.dataset.key === this.current);
    const next = this.rows[Math.max(0, Math.min(this.rows.length - 1, i < 0 ? 0 : i + delta))];
    this.current = next.dataset.key;
    this.markRows();
    next.focus();
    next.scrollIntoView({ block: "nearest" });
    if (this.peeked) this.peek(this.current);
  }

  onClick(e) {
    const action = e.target.closest?.("[data-action]")?.dataset.action;
    if (action === "create") return this.create();
    if (action === "open") return this.open(this.peeked);
    if (action === "copy") return this.copy(this.peeked);
    const row = e.target.closest?.(".row");
    if (row) {
      this.current = row.dataset.key;
      this.peek(row.dataset.key);
    }
  }

  peek(key) {
    if (!key) return;
    const drawer = this.shadowRoot.querySelector("[data-peek]");
    this.peeked = key;
    this.markRows();
    let detail = drawer.querySelector("hzn-issue-detail");
    if (!detail) {
      detail = document.createElement("hzn-issue-detail");
      detail.setAttribute("mode", "peek");
      detail.setAttribute("ws", this.ws);
      detail.addEventListener("hzn-issue-loaded", (e) => {
        drawer.querySelector("[data-peek-title]").textContent = e.detail.parent ? `${e.detail.parent} · ${e.detail.key}` : e.detail.key;
      });
      drawer.append(detail);
    }
    detail.setAttribute("issue", key);
    drawer.setAttribute("open", "");
    // The drawer takes focus as it opens; a peek leaves it on the list, so
    // j/k keep moving and Enter doesn't press a button in the panel.
    this.rows?.find((r) => r.dataset.key === key)?.focus();
  }

  open(key) {
    if (key) this.context.router.navigate(issueHref(this.ws, key));
  }

  async copy(key) {
    if (!key) return;
    await navigator.clipboard?.writeText(new URL(issueHref(this.ws, key), location.href).href).catch(() => {});
    globalThis.LDS?.toast?.({ intent: "success", title: this.context.i18n.t("issues.linkCopied") });
  }

  create() {
    if (!this.data?.project.can.includes("issue.create")) return;
    this.shadowRoot.querySelector("hzn-issue-create").show({ ws: this.ws, project: this.data.project.key });
  }

  /** Refreshes on pushed events about this project's issues. */
  onPush(m) {
    if (m.t !== "event" && m.t !== "resync") return;
    if (m.t === "event" && changesProjectData(m.d)) {
      forgetProjects();
      clearTimeout(this.timer);
      this.timer = setTimeout(() => this.load(), 200);
      return;
    }
    const key = m.d?.aggregate?.key ?? "";
    if (m.t === "resync" || key.startsWith(`${this.data?.project.key}-`) || m.d?.type === "issue.moved") {
      clearTimeout(this.timer);
      this.timer = setTimeout(() => this.reload(), 150);
    }
  }
}

customElements.define("hzn-issue-list", HznIssueList);
