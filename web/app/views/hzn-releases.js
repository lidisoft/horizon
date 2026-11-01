// <hzn-releases>: /w/:ws/p/:key/releases — the project's releases with the
// progress of the issues targeted to each, then its milestones and the
// workspace's. People with release.manage create, edit, release, archive
// and delete releases; issues are targeted from the issue's Release
// property.

import "horizon/components/hzn-milestone-list.js";
import "horizon/components/hzn-release-dialog.js";
import { issueHref, issueStyles, projectData, statusGlyph } from "horizon/components/issue-ui.js";
import { loadReleases, localDate, planningStyles, progress, viewSwitch } from "horizon/components/planning-ui.js";
import { paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { busy, errorMessage } from "horizon/core/forms.js";

const STATE_INTENTS = { unreleased: "info", released: "success", archived: "neutral" };

const styles = sheet(`
  :host { display: flex; flex-direction: column; min-block-size: 100%; }
  header.bar { display: flex; align-items: center; justify-content: space-between; gap: var(--lds-space-3);
    min-block-size: 3.25rem; padding: 0 var(--lds-space-5); border-block-end: 1px solid var(--lds-color-border-default); }
  .crumb { display: flex; align-items: center; gap: var(--lds-space-2); }
  .crumb strong { font-weight: var(--lds-font-weight-semibold); }
  .crumb lds-icon { color: var(--lds-color-fg-subtle); }
  .swatch { display: inline-block; inline-size: 0.625rem; block-size: 0.625rem; border-radius: var(--lds-radius-xs); }
  .toolbar { display: flex; align-items: center; gap: var(--lds-space-3); padding: var(--lds-space-2) var(--lds-space-5);
    border-block-end: 1px solid var(--lds-color-border-subtle); }
  .toolbar .grow { flex: 1; }
  .content { display: grid; gap: var(--lds-space-8); padding: var(--lds-space-5); max-inline-size: 64rem; }
  h2 { margin: 0 0 var(--lds-space-3); font-size: var(--lds-font-size-md); }
  ol { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--lds-space-2); }
  li { border: 1px solid var(--lds-color-border-default); border-radius: var(--lds-radius-md); background: var(--lds-color-bg-surface); }
  .item { display: grid; grid-template-columns: minmax(0, 1fr) auto auto auto auto; align-items: center; gap: var(--lds-space-3);
    padding: var(--lds-space-2) var(--lds-space-3); }
  .name { font-weight: var(--lds-font-weight-semibold); }
  .dates { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); white-space: nowrap; }
  .desc { grid-column: 1 / -1; margin: 0; color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  .issues { padding: 0 var(--lds-space-3) var(--lds-space-2); }
  .issue { display: grid; grid-template-columns: 1.25rem 4.5rem minmax(0, 1fr); gap: var(--lds-space-2); align-items: center;
    padding: var(--lds-space-1) 0; color: inherit; text-decoration: none; font-size: var(--lds-font-size-sm); }
  .issue:hover .title { text-decoration: underline; }
  .title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .toggle { border: 0; background: none; padding: 0; color: var(--lds-color-fg-link); cursor: pointer; font: inherit; font-size: var(--lds-font-size-sm); }
  .none { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  .empty { padding: var(--lds-space-12) var(--lds-space-6); }
  .confirm { display: flex; justify-content: flex-end; gap: var(--lds-space-3); }
`);

class HznReleases extends HTMLElement {
  static observedAttributes = ["ws", "key"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    this.expanded = new Set();
    this.archived = false;
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [issueStyles, planningStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("lds-change", (e) => {
      if (e.target.matches("[data-archived]")) {
        this.archived = e.detail.checked;
        this.refresh();
      }
    });
    this.onChanged = () => this.refresh();
    document.addEventListener("hzn-release-changed", this.onChanged);
    this.offSocket = this.context.socket.on((m) => {
      const ours = m.d?.payload?.project_id === this.data?.project.id || (m.d?.aggregate?.key ?? "").startsWith(`${this.data?.project.key}-`);
      if (m.t === "resync" || (m.t === "event" && ours && ["release", "issue"].includes(m.d?.aggregate?.type))) {
        clearTimeout(this.timer);
        this.timer = setTimeout(() => this.refresh(), 150);
      }
    });
    this.load();
  }

  disconnectedCallback() {
    document.removeEventListener("hzn-release-changed", this.onChanged);
    this.offSocket?.();
    clearTimeout(this.timer);
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.load();
  }

  get ws() {
    return this.getAttribute("ws");
  }

  get canManage() {
    return this.data.project.can.includes("release.manage") && !this.data.project.archived_at;
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
      router.navigate(`/w/${this.ws}/p/${this.data.project.key}/releases`, { replace: true }); // a former key
      return;
    }
    this.renderFrame();
    await this.refresh();
  }

  async refresh() {
    if (!this.data) return;
    const { api, i18n } = this.context;
    const key = this.data.project.key;
    try {
      const items = [];
      const [list] = await Promise.all([
        this.archived
          ? api.get(`/w/${encodeURIComponent(this.ws)}/projects/${key}/releases`, { query: { archived: true } }).then((r) => r.data)
          : loadReleases(this.context, this.ws, key),
        (async () => {
          let cursor = "";
          do {
            const { data } = await api.get(`/w/${encodeURIComponent(this.ws)}/projects/${key}/issues`, { query: { limit: 500, cursor: cursor || undefined, archived: true } });
            items.push(...data.items.filter((i) => i.release_id));
            cursor = data.next_cursor;
          } while (cursor);
        })(),
      ]);
      this.list = list;
      this.items = items;
    } catch (err) {
      this.shadowRoot.querySelector("[data-releases]").innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    this.renderReleases();
  }

  renderFrame() {
    const { t } = this.context.i18n;
    const p = this.data.project;
    this.shadowRoot.innerHTML = String(html`
      <header class="bar">
        <div class="crumb">
          <span class="swatch" data-colour="${p.colour}"></span><strong>${p.name}</strong>
          <lds-icon name="chevron-right" size="sm"></lds-icon><span>${t("releases.title")}</span>
        </div>
        ${this.canManage ? html`<lds-button variant="primary" size="sm" data-action="new"><lds-icon slot="start" name="plus" size="sm"></lds-icon>${t("releases.new")}</lds-button>` : ""}
      </header>
      ${p.archived_at ? html`<lds-alert intent="warning">${t("projects.isArchived")}</lds-alert>` : ""}
      <div class="toolbar">${viewSwitch(t, this.ws, p.key, "releases")}<span class="grow"></span>
        <lds-switch data-archived ${this.archived ? html`checked` : ""}>${t("releases.showArchived")}</lds-switch></div>
      <div class="content">
        <section aria-labelledby="releases-title"><h2 id="releases-title">${t("releases.title")}</h2><div data-releases><lds-skeleton></lds-skeleton></div></section>
        <hzn-milestone-list ws="${this.ws}" project="${p.key}" ${p.can.includes("milestone.manage") && !p.archived_at ? html`manage` : ""}></hzn-milestone-list>
      </div>
      <lds-modal size="sm" data-confirm>
        <span slot="header" data-confirm-title></span>
        <p data-confirm-body></p>
        <div slot="footer" class="confirm">
          <lds-button variant="ghost" data-action="cancel">${t("tokens.cancel")}</lds-button>
          <lds-button variant="danger" data-action="confirm-delete">${t("releases.delete")}</lds-button>
        </div>
      </lds-modal>
      <hzn-release-dialog></hzn-release-dialog>`);
    paintSwatches(this.shadowRoot);
  }

  dates(r) {
    const { i18n } = this.context;
    const { t } = i18n;
    const f = (v) => i18n.date(localDate(v), { dateStyle: "medium" });
    if (r.state === "released" && r.release_date) return t("releases.releasedOn", { date: f(r.release_date) });
    const parts = [];
    if (r.start_date) parts.push(t("releases.starts", { date: f(r.start_date) }));
    if (r.release_date) parts.push(t("releases.due", { date: f(r.release_date) }));
    return parts.join(" · ");
  }

  renderReleases() {
    const { i18n } = this.context;
    const { t } = i18n;
    const box = this.shadowRoot.querySelector("[data-releases]");
    if (!this.list.items.length) {
      box.innerHTML = String(html`<p class="none">${t("releases.empty")}</p>`);
      return;
    }
    box.innerHTML = String(html`<ol>${this.list.items.map((r) => {
      const issues = this.items.filter((i) => i.release_id === r.id);
      return html`
        <li data-release="${r.id}" aria-label="${r.name}">
          <div class="item">
            <span><span class="name">${r.name}</span> <span class="dates">${this.dates(r)}</span></span>
            <lds-badge size="sm" intent="${STATE_INTENTS[r.state]}">${t(`releases.state.${r.state}`)}</lds-badge>
            ${progress(r.progress, i18n)}
            ${this.canManage && r.state === "unreleased" ? html`<lds-button size="sm" variant="secondary" data-action="release" data-id="${r.id}">${t("releases.release")}</lds-button>` : html`<span></span>`}
            ${this.canManage ? html`
              <lds-menu placement="bottom-end">
                <lds-button slot="trigger" variant="ghost" size="sm" label="${t("releases.more", { name: r.name })}"><lds-icon name="more-horizontal"></lds-icon></lds-button>
                ${r.state !== "archived" ? html`
                  <lds-menu-item data-action="edit" data-id="${r.id}"><lds-icon slot="start" name="pencil"></lds-icon>${t("releases.edit")}</lds-menu-item>
                  <lds-menu-item data-action="archive" data-id="${r.id}"><lds-icon slot="start" name="archive"></lds-icon>${t("releases.archive")}</lds-menu-item>` : ""}
                <hr><lds-menu-item data-action="delete" data-id="${r.id}"><lds-icon slot="start" name="trash-2"></lds-icon>${t("releases.delete")}</lds-menu-item>
              </lds-menu>` : html`<span></span>`}
            ${r.description ? html`<p class="desc">${r.description}</p>` : ""}
          </div>
          <div class="issues">
            <button type="button" class="toggle" data-action="toggle" data-id="${r.id}" aria-expanded="${String(this.expanded.has(r.id))}">
              ${t(this.expanded.has(r.id) ? "releases.hideIssues" : "releases.showIssues", { count: issues.length })}</button>
            ${this.expanded.has(r.id) ? issues.map((i) => html`
              <a class="issue" href="${issueHref(this.ws, i.key)}">${statusGlyph(this.data, i.status_id)}<span class="key">${i.key}</span><span class="title">${i.title}</span></a>`) : ""}
          </div>
        </li>`;
    })}</ol>`);
  }

  onClick(e) {
    const el = e.target.closest?.("[data-action]");
    if (!el) return;
    const r = el.dataset.id && this.list?.items.find((x) => x.id === el.dataset.id);
    const dialog = this.shadowRoot.querySelector("hzn-release-dialog");
    const project = this.data.project.key;
    switch (el.dataset.action) {
      case "new":
        return dialog.create({ ws: this.ws, project });
      case "edit":
        return dialog.edit({ ws: this.ws, project, release: r });
      case "release":
        return this.command(el, r, "release", "releases.released");
      case "archive":
        return this.command(el, r, "archive", "releases.archived");
      case "toggle":
        if (this.expanded.has(r.id)) this.expanded.delete(r.id);
        else this.expanded.add(r.id);
        return this.renderReleases();
      case "delete": {
        const { t } = this.context.i18n;
        this.deleting = r;
        const modal = this.shadowRoot.querySelector("[data-confirm]");
        modal.querySelector("[data-confirm-title]").textContent = t("releases.deleteTitle", { name: r.name });
        modal.querySelector("[data-confirm-body]").textContent = t("releases.deleteBody", { count: r.progress.issues });
        return modal.setAttribute("open", "");
      }
      case "cancel":
        return this.shadowRoot.querySelector("[data-confirm]").removeAttribute("open");
      case "confirm-delete":
        return this.delete(el);
    }
  }

  base() {
    return `/w/${encodeURIComponent(this.ws)}/projects/${this.data.project.key}/releases`;
  }

  async command(button, r, path, done) {
    const { api, i18n } = this.context;
    try {
      await busy(button, () => api.post(`${this.base()}/${r.id}/${path}`));
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      return;
    }
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t(done, { name: r.name }) });
    this.refresh();
  }

  async delete(button) {
    const { api, i18n } = this.context;
    try {
      await busy(button, () => api.delete(`${this.base()}/${this.deleting.id}`));
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      return;
    }
    this.shadowRoot.querySelector("[data-confirm]").removeAttribute("open");
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("releases.deleted", { name: this.deleting.name }) });
    this.refresh();
  }
}

customElements.define("hzn-releases", HznReleases);
