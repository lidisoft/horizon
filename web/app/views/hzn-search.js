// <hzn-search>: /w/:ws/search?q= — issues matching words in titles,
// descriptions and comments, keys and near titles (docs/adr/0020). The
// query stays in the URL; matched words are highlighted in snippets.

import { issueHref, issueStyles } from "horizon/components/issue-ui.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";

const OUTCOME_ICONS = { unstarted: "circle", started: "circle-dot", completed: "circle-check", abandoned: "circle-x" };

const styles = sheet(`
  :host { display: block; }
  header { display: flex; align-items: center; gap: var(--lds-space-4); padding: var(--lds-space-4) var(--lds-space-5);
    border-block-end: 1px solid var(--lds-color-border-default); flex-wrap: wrap; }
  header lds-input { flex: 1; min-inline-size: 16rem; }
  .results { display: grid; }
  .result { display: grid; grid-template-columns: 1.25rem 4.5rem minmax(0, 1fr) auto; gap: var(--lds-space-1) var(--lds-space-3);
    align-items: baseline; padding: var(--lds-space-3) var(--lds-space-5); border-block-end: 1px solid var(--lds-color-border-subtle);
    color: inherit; text-decoration: none; }
  .result:hover, .result:focus-visible { background: var(--lds-color-bg-subtle); outline: none; }
  .result .title { font-weight: var(--lds-font-weight-medium); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .result .where { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-xs); }
  .result .snippet { grid-column: 3 / -1; color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  mark { background: var(--lds-color-warning-subtle); color: inherit; border-radius: var(--lds-radius-xs); }
  .empty { padding: var(--lds-space-12) var(--lds-space-6); }
  .count { padding: var(--lds-space-3) var(--lds-space-5); color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
`);

/** Escaped snippet text with the server's markers (U+0002/U+0003) as <mark>. */
export function highlight(snippet) {
  return snippet.split(/[\u0002\u0003]/).map((part, i) => (i % 2 ? html`<mark>${part}</mark>` : html`${part}`));
}

class HznSearch extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const { t } = this.context.i18n;
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [issueStyles, styles];
    const query = this.context.router.current?.query;
    this.q = query?.get("q") ?? "";
    this.archived = query?.get("archived") === "true";
    root.innerHTML = String(html`
      <header>
        <lds-input type="search" data-q value="${this.q}" placeholder="${t("search.placeholder")}" aria-label="${t("search.label")}">
          <lds-icon slot="prefix" name="search"></lds-icon></lds-input>
        <lds-switch data-archived ${this.archived ? html`checked` : ""}>${t("search.archived")}</lds-switch>
      </header>
      <div data-results></div>`);
    root.querySelector("[data-q]").addEventListener("lds-input", (e) => {
      this.q = e.detail.value;
      clearTimeout(this.timer);
      this.timer = setTimeout(() => this.run(), 200);
    });
    root.querySelector("[data-archived]").addEventListener("lds-change", (e) => {
      this.archived = e.detail.checked;
      this.run();
    });
    requestAnimationFrame(() => root.querySelector("[data-q]").focus());
    this.run();
  }

  disconnectedCallback() {
    clearTimeout(this.timer);
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.run();
  }

  async run() {
    const { api, i18n, router } = this.context;
    const { t } = i18n;
    const ws = this.getAttribute("ws");
    const params = new URLSearchParams();
    if (this.q.trim()) params.set("q", this.q.trim());
    if (this.archived) params.set("archived", "true");
    const url = `/w/${ws}/search${params.size ? `?${params}` : ""}`;
    if (location.pathname + location.search !== url) router.navigate(url, { replace: true });
    const box = this.shadowRoot.querySelector("[data-results]");
    if (!this.q.trim()) {
      box.innerHTML = String(html`<lds-empty-state class="empty"><lds-icon slot="icon" name="search"></lds-icon>
        <h2 slot="title">${t("search.prompt")}</h2>${t("search.hint")}</lds-empty-state>`);
      return;
    }
    const seq = (this.seq = (this.seq ?? 0) + 1);
    let data;
    try {
      ({ data } = await api.get(`/w/${encodeURIComponent(ws)}/search`, { query: { q: this.q.trim(), limit: 50, archived: this.archived || undefined } }));
    } catch (err) {
      box.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    if (seq !== this.seq) return; // a later query answered first
    if (!data.items.length) {
      box.innerHTML = String(html`<lds-empty-state class="empty"><lds-icon slot="icon" name="search"></lds-icon>
        <h2 slot="title">${t("search.none", { q: data.query })}</h2>${t("search.noneHint")}</lds-empty-state>`);
      return;
    }
    box.innerHTML = String(html`
      <div class="count" role="status">${t("search.count", { count: data.items.length })}</div>
      <div class="results">${data.items.map((r) => html`
        <a class="result" href="${issueHref(ws, r.key)}">
          <span class="glyph status" data-outcome="${r.status.outcome}"><lds-icon name="${OUTCOME_ICONS[r.status.outcome] ?? "circle"}" size="sm" label="${r.status.name}"></lds-icon></span>
          <span class="key">${r.key}</span>
          <span class="title">${r.title}</span>
          <span class="where">${t(`search.match.${r.match}`)}${r.archived ? ` · ${t("issues.archived")}` : ""}</span>
          ${r.snippet ? html`<span class="snippet">${highlight(r.snippet)}</span>` : ""}
        </a>`)}</div>`);
  }
}

customElements.define("hzn-search", HznSearch);
