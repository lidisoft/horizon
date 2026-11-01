// <hzn-issue-detail ws issue mode>: an issue with its properties,
// description, sub-issues, links, attachments, activity and comments
// (wireframes: IssueDetail for mode="page", IssuePeek for mode="peek").
// Every property saves as it changes, with the issue's version as If-Match;
// pushed events about the issue reload it. Fires hzn-issue-loaded with
// { key, parent } and hzn-issue-gone when it is deleted or moved away.

import "horizon/components/hzn-issue-create.js";
import "horizon/components/hzn-move-issue.js";
import {
  PRIORITIES, avatar, issueHref, issueStyles, priorityIcon, projectData, statusGlyph,
} from "horizon/components/issue-ui.js";
import { isOpen } from "horizon/components/planning-ui.js";
import { docStyles, isPlain, renderDoc } from "horizon/editor/render.js";
import { paintSwatches } from "horizon/components/project-colour.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { busy, errorMessage } from "horizon/core/forms.js";

const LINK_TYPES = ["relates_to", "blocks", "blocked_by", "duplicates", "duplicated_by", "precedes", "follows"];

const styles = sheet(`
  :host { display: block; }
  :host([mode="page"]) .layout { display: grid; grid-template-columns: minmax(0, 1fr) 18rem; min-block-size: 100%; }
  :host([mode="page"]) main { padding: var(--lds-space-8) var(--lds-space-10); max-inline-size: 52rem; }
  :host([mode="page"]) aside { padding: var(--lds-space-5); border-inline-start: 1px solid var(--lds-color-border-default); }
  :host([mode="page"]) aside .props { display: grid; gap: var(--lds-space-4); }
  :host([mode="peek"]) .props { display: grid; grid-template-columns: 7rem minmax(0, 1fr); gap: var(--lds-space-2) var(--lds-space-3); align-items: center; }
  :host([mode="peek"]) .props > label { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  :host([mode="peek"]) .props lds-field::part(label) { display: none; }
  .bar { display: flex; align-items: center; justify-content: space-between; gap: var(--lds-space-3);
    min-block-size: 3.25rem; padding: 0 var(--lds-space-5); border-block-end: 1px solid var(--lds-color-border-default); }
  .crumbs { display: flex; align-items: center; gap: var(--lds-space-2); min-inline-size: 0; font-size: var(--lds-font-size-sm); }
  .crumbs a { color: var(--lds-color-fg-muted); text-decoration: none; }
  .crumbs a:hover { color: var(--lds-color-fg-default); }
  .crumbs lds-icon { color: var(--lds-color-fg-subtle); }
  .kind { display: flex; align-items: center; gap: var(--lds-space-2); }
  .kind .grow { flex: 1; }
  h1 { margin: var(--lds-space-2) 0 var(--lds-space-4); font-family: var(--lds-font-family-display); font-size: var(--lds-font-size-2xl);
    line-height: 1.25; cursor: text; overflow-wrap: anywhere; }
  :host([mode="peek"]) h1 { font-size: var(--lds-font-size-xl); margin-block-start: 0; }
  h2 { margin: var(--lds-space-6) 0 var(--lds-space-2); font-size: var(--lds-font-size-md); display: flex; align-items: center; gap: var(--lds-space-3); }
  h2 .grow { flex: 1; }
  .list { display: grid; border: 1px solid var(--lds-color-border-default); border-radius: var(--lds-radius-md); }
  .list:empty { display: none; }
  .item { display: grid; grid-template-columns: auto auto minmax(0, 1fr) auto; align-items: center; gap: var(--lds-space-3);
    padding: var(--lds-space-2) var(--lds-space-3); border-block-end: 1px solid var(--lds-color-border-subtle); }
  .item:last-child { border-block-end: none; }
  .item a { color: inherit; text-decoration: none; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .item a:hover { text-decoration: underline; }
  .item .type { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); min-inline-size: 6rem; }
  .linker { display: flex; gap: var(--lds-space-2); align-items: end; margin-block-start: var(--lds-space-2); }
  .linker lds-input { flex: 1; }
  .feed { display: grid; gap: var(--lds-space-3); }
  .event { display: flex; gap: var(--lds-space-2); align-items: baseline; font-size: var(--lds-font-size-sm); color: var(--lds-color-fg-muted); }
  .event strong { color: var(--lds-color-fg-default); font-weight: var(--lds-font-weight-medium); }
  .comment { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: var(--lds-space-3); }
  .comment .box { border: 1px solid var(--lds-color-border-default); border-radius: var(--lds-radius-md); padding: var(--lds-space-3); }
  .comment .who { display: flex; gap: var(--lds-space-2); align-items: baseline; font-size: var(--lds-font-size-sm); }
  .comment .body { white-space: pre-wrap; overflow-wrap: anywhere; margin-block-start: var(--lds-space-1); }
  .comment .tools { margin-inline-start: auto; display: flex; gap: var(--lds-space-1); }
  .composer { display: grid; gap: var(--lds-space-2); margin-block-start: var(--lds-space-4); }
  .composer .actions { display: flex; justify-content: flex-end; }
  .description { white-space: pre-wrap; }
  .progress { display: flex; align-items: center; gap: var(--lds-space-2); font-size: var(--lds-font-size-sm); font-weight: normal; color: var(--lds-color-fg-muted); }
  .progress lds-progress { inline-size: 7rem; }
  .labels { display: flex; flex-wrap: wrap; gap: var(--lds-space-1); }
  .section-title { margin: var(--lds-space-6) 0 var(--lds-space-2); font-size: var(--lds-font-size-xs); letter-spacing: 0.06em;
    text-transform: uppercase; color: var(--lds-color-fg-muted); }
  .dates { display: grid; gap: var(--lds-space-2); }
  input[type="file"] { display: none; }
  .actions-row { display: flex; justify-content: flex-end; gap: var(--lds-space-3); }
  .peek-body { display: grid; gap: var(--lds-space-2); }
  @media (max-width: 64rem) {
    :host([mode="page"]) .layout { grid-template-columns: minmax(0, 1fr); }
    :host([mode="page"]) aside { border-inline-start: none; border-block-start: 1px solid var(--lds-color-border-default); }
    :host([mode="page"]) main { padding: var(--lds-space-5); }
  }
`);

class HznIssueDetail extends HTMLElement {
  static observedAttributes = ["ws", "issue"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [issueStyles, styles, docStyles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("lds-change", (e) => this.onChange(e));
    root.addEventListener("keydown", (e) => this.onKey(e));
    // The comment being written survives re-renders.
    root.addEventListener("lds-input", (e) => {
      if (e.target.closest?.("[data-composer]")) this.draft = e.detail.value;
    });
    // Pushed changes wait until the control being edited loses focus.
    root.addEventListener("focusout", () => {
      if (this.stale) setTimeout(() => !this.editing() && this.stale && this.load(), 0);
    });
    this.offSocket = this.context.socket.on((m) => this.onPush(m));
    if (this.mode === "page") this.bindKeys();
    this.load();
  }

  disconnectedCallback() {
    this.offSocket?.();
    this.offScope?.();
    for (const off of this.offKeys ?? []) off();
    this.offCommands?.();
    clearTimeout(this.timer);
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before !== after && this.isConnected) this.load();
  }

  get mode() {
    return this.getAttribute("mode") || "page";
  }

  get ws() {
    return this.getAttribute("ws");
  }

  get base() {
    return `/w/${encodeURIComponent(this.ws)}/issues/${encodeURIComponent(this.issue?.key ?? this.getAttribute("issue"))}`;
  }

  /** Shortcuts on the issue page (the peek leaves the list's in place). */
  bindKeys() {
    const { shortcuts } = this.context;
    this.offScope = shortcuts.pushScope("issue");
    const focus = (name) => () => this.shadowRoot.querySelector(`[data-prop="${name}"]`)?.focus();
    this.offKeys = [
      shortcuts.register({ id: "issue-status", keys: "s", scope: "issue", description: "shortcut.status", handler: focus("status") }),
      shortcuts.register({ id: "issue-assign", keys: "a", scope: "issue", description: "shortcut.assign", handler: focus("assignee") }),
      shortcuts.register({ id: "issue-priority", keys: "p", scope: "issue", description: "shortcut.priority", handler: focus("priority") }),
      shortcuts.register({ id: "issue-labels", keys: "l", scope: "issue", description: "shortcut.labels", handler: focus("labels") }),
      shortcuts.register({ id: "issue-title", keys: "e", scope: "issue", description: "shortcut.editTitle", handler: () => this.editTitle() }),
      shortcuts.register({ id: "issue-comment", keys: "r", scope: "issue", description: "shortcut.comment", handler: () => this.shadowRoot.querySelector("[data-composer] lds-textarea")?.focus() }),
      shortcuts.register({ id: "issue-move", keys: "m", scope: "issue", description: "shortcut.move", handler: () => this.move() }),
      shortcuts.register({ id: "issue-copy", keys: "mod+shift+c", scope: "issue", description: "shortcut.copyLink", handler: () => this.copyLink() }),
    ];
  }

  async load() {
    const key = this.getAttribute("issue");
    if (!key || !this.ws) return;
    const { api, i18n, router } = this.context;
    const enc = encodeURIComponent(this.ws);
    let issue, etag;
    try {
      ({ data: issue, etag } = await api.get(`/w/${enc}/issues/${encodeURIComponent(key)}`));
    } catch (err) {
      this.issue = null;
      this.shadowRoot.innerHTML = String(html`
        <lds-empty-state><lds-icon slot="icon" name="search"></lds-icon>
          <h2 slot="title">${err.status === 404 ? i18n.t("issues.notFound") : errorMessage(err, i18n)}</h2>
          <a href="/w/${this.ws}">${i18n.t("notFound.home")}</a>
        </lds-empty-state>`);
      return;
    }
    if (this.mode === "page" && issue.key !== key.toUpperCase()) {
      router.navigate(issueHref(this.ws, issue.key), { replace: true }); // a former key
      return;
    }
    const base = `/w/${enc}/issues/${encodeURIComponent(issue.key)}`;
    const projectKey = issue.key.slice(0, issue.key.lastIndexOf("-"));
    try {
      const plans = `/w/${enc}/projects/${encodeURIComponent(projectKey)}`;
      const [data, children, links, comments, attachments, activity, transitions, iterations, releases] = await Promise.all([
        projectData(this.context, this.ws, projectKey),
        api.get(`${base}/children`), api.get(`${base}/links`), api.get(`${base}/comments`),
        api.get(`${base}/attachments`), api.get(`${base}/activity`), api.get(`${base}/transitions`),
        api.get(`${plans}/iterations`), api.get(`${plans}/releases`),
      ]);
      this.iterations = iterations.data.items;
      this.releases = releases.data.items;
      this.data = data;
      this.subIssues = children.data.items;
      this.links = links.data.items;
      this.comments = comments.data.items;
      this.attachments = attachments.data.items;
      this.activity = activity.data.items;
      this.transitions = transitions.data.items;
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    this.issue = issue;
    this.etag = etag;
    this.parent = issue.parent ?? null;
    // Never re-render under someone typing: a draft would lose its focus,
    // and the next keys would go to the page's shortcuts.
    if (this.editing()) {
      this.stale = true;
      return;
    }
    this.stale = false;
    this.render();
    this.registerCommands();
    this.dispatchEvent(new CustomEvent("hzn-issue-loaded", { detail: { key: issue.key, parent: this.parent?.key ?? "" }, bubbles: true, composed: true }));
  }

  get can() {
    return this.data.project.can;
  }

  render() {
    const focused = this.shadowRoot.activeElement;
    const keep = focused?.dataset?.prop ? `[data-prop="${focused.dataset.prop}"]` : focused?.dataset?.field ? `[data-field="${focused.dataset.field}"]` : null;
    this.paint();
    if (keep) this.shadowRoot.querySelector(keep)?.focus();
  }

  paint() {
    const { i18n } = this.context;
    const { t } = i18n;
    const i = this.issue;
    const d = this.data;
    const type = d.types.get(i.type_id);
    const archived = !!i.archived_at;
    const header = html`
      <div class="kind">
        <lds-badge size="sm">${type?.name ?? ""}</lds-badge><span class="key">${i.key}</span>
        ${archived ? html`<lds-badge size="sm" intent="warning">${t("issues.archived")}</lds-badge>` : ""}
        ${this.mode === "peek" ? html`<span class="grow"></span>${this.menu()}` : ""}
      </div>
      <h1 data-title tabindex="0" title="${t("issues.editTitle")}">${i.title}</h1>`;
    const body = html`
      ${this.mode === "peek" ? html`<div class="props">${this.properties()}</div><lds-divider></lds-divider>` : ""}
      ${this.descriptionBlock()}
      ${this.childrenBlock()}
      ${this.linksBlock()}
      ${this.attachmentsBlock()}
      ${this.activityBlock()}`;
    if (this.mode === "peek") {
      this.shadowRoot.innerHTML = String(html`<div class="peek-body">${header}${body}</div>${this.dialogs()}`);
    } else {
      this.shadowRoot.innerHTML = String(html`
        <header class="bar">
          <nav class="crumbs" aria-label="${t("issues.breadcrumb")}">
            <a href="/w/${this.ws}/p/${d.project.key}/issues">${d.project.name}</a>
            ${this.parent ? html`<lds-icon name="chevron-right" size="sm"></lds-icon><a href="${issueHref(this.ws, this.parent.key)}">${this.parent.key} ${this.parent.name}</a>` : ""}
            <lds-icon name="chevron-right" size="sm"></lds-icon><strong>${i.key}</strong>
          </nav>
          ${this.menu()}
        </header>
        <div class="layout">
          <main>${header}${body}</main>
          <aside><div class="props">${this.properties()}</div></aside>
        </div>
        ${this.dialogs()}`);
    }
    paintSwatches(this.shadowRoot);
  }

  menu() {
    const { t } = this.context.i18n;
    const archived = !!this.issue.archived_at;
    return html`
      <lds-menu placement="bottom-end">
        <lds-button slot="trigger" variant="ghost" size="sm" label="${t("issues.more")}"><lds-icon name="more-horizontal"></lds-icon></lds-button>
        <lds-menu-item data-action="copy"><lds-icon slot="start" name="link"></lds-icon>${t("issues.copyLink")}</lds-menu-item>
        ${this.can.includes("issue.move") && !archived ? html`<lds-menu-item data-action="move"><lds-icon slot="start" name="folder"></lds-icon>${t("issues.move")}</lds-menu-item>` : ""}
        ${this.can.includes("issue.archive") ? html`<lds-menu-item data-action="${archived ? "unarchive" : "archive"}"><lds-icon slot="start" name="archive"></lds-icon>${t(archived ? "issues.unarchive" : "issues.archive")}</lds-menu-item>` : ""}
        ${this.can.includes("issue.delete") ? html`<hr><lds-menu-item data-action="delete"><lds-icon slot="start" name="trash-2"></lds-icon>${t("issues.delete")}</lds-menu-item>` : ""}
      </lds-menu>`;
  }

  /** The property controls; each saves on change. */
  properties() {
    const { i18n } = this.context;
    const { t } = i18n;
    const i = this.issue;
    const d = this.data;
    const off = (perm) => (i.archived_at || !this.can.includes(perm) ? html`disabled` : "");
    const current = d.statuses.get(i.status_id);
    const targets = this.transitions.filter((x) => !x.blocked);
    const scale = d.config.estimation;
    const types = d.config.issue_types;
    const prop = (label, control) => (this.mode === "peek" ? html`<label>${label}</label>${control}` : html`<lds-field label="${label}">${control}</lds-field>`);
    const fields = d.fieldsOf(i.type_id);
    const values = i.fields ?? {};
    return html`
      ${prop(t("issues.status"), html`
        <lds-select data-prop="status" value="${i.status_id}" ${off("issue.transition")}>
          <option value="${i.status_id}">${current?.name ?? "?"}</option>
          ${targets.map((x) => html`<option value="${x.to_status_id}">${x.name} → ${d.statuses.get(x.to_status_id)?.name ?? ""}</option>`)}
        </lds-select>`)}
      ${prop(t("issues.priority"), html`
        <lds-select data-prop="priority" value="${i.priority}" ${off("issue.edit")}>
          ${PRIORITIES.map((p) => html`<option value="${p}">${t(`priority.${p}`)}</option>`)}
        </lds-select>`)}
      ${prop(t("issues.assignee"), html`
        <lds-combobox data-prop="assignee" value="${i.assignee_id ?? ""}" placeholder="${t("issue.unassigned")}" label="${t("issues.assignee")}" ${off("issue.assign")}>
          <lds-option value="">${t("issue.unassigned")}</lds-option>
          ${d.peopleList.map((p) => html`<lds-option value="${p.id}"><lds-avatar slot="start" size="sm" name="${p.name}"></lds-avatar>${p.name}</lds-option>`)}
        </lds-combobox>`)}
      ${prop(t("issues.labels"), html`
        <lds-combobox data-prop="labels" multiple value="${i.labels.join(",")}" label="${t("issues.labels")}" ${off("issue.edit")}>
          ${d.config.labels.filter((l) => !l.disabled || i.labels.includes(l.id)).map((l) => html`<lds-option value="${l.id}">${l.name}</lds-option>`)}
        </lds-combobox>`)}
      ${scale.kind !== "none" ? prop(t("issues.estimate"), scale.values.length <= 6 ? html`
        <lds-segmented data-prop="estimate" size="sm" value="${i.estimate ?? ""}" label="${t("issues.estimate")}" ${off("issue.estimate")}>
          ${scale.values.map((v) => html`<lds-segment value="${v.label}">${v.label}</lds-segment>`)}
          <lds-segment value="">–</lds-segment>
        </lds-segmented>` : html`
        <lds-combobox data-prop="estimate" value="${i.estimate ?? ""}" placeholder="${t("issues.noEstimate")}" label="${t("issues.estimate")}" ${off("issue.estimate")}>
          <lds-option value="">${t("issues.noEstimate")}</lds-option>
          ${scale.values.map((v) => html`<lds-option value="${v.label}">${v.label}</lds-option>`)}
        </lds-combobox>`) : ""}
      ${prop(t("issues.iteration"), html`
        <lds-combobox data-prop="iteration" value="${i.iteration_id ?? ""}" placeholder="${t("issues.noIteration")}" label="${t("issues.iteration")}" ${off("iteration.manage")}>
          <lds-option value="">${t("issues.noIteration")}</lds-option>
          ${this.iterations.filter((it) => isOpen(it) || it.id === i.iteration_id).map((it) => html`<lds-option value="${it.id}">${it.name}<span slot="end">${t(`iterations.state.${it.state}`)}</span></lds-option>`)}
        </lds-combobox>`)}
      ${prop(t("issues.release"), html`
        <lds-combobox data-prop="release" value="${i.release_id ?? ""}" placeholder="${t("issues.noRelease")}" label="${t("issues.release")}" ${off("issue.edit")}>
          <lds-option value="">${t("issues.noRelease")}</lds-option>
          ${this.releases.filter((r) => r.state !== "archived" || r.id === i.release_id).map((r) => html`<lds-option value="${r.id}">${r.name}</lds-option>`)}
        </lds-combobox>`)}
      ${prop(t("issues.dates"), html`
        <div class="dates">
          <lds-date-picker data-prop="start_date" value="${i.start_date ?? ""}" aria-label="${t("issues.startDate")}" ${off("issue.edit")}></lds-date-picker>
          <lds-date-picker data-prop="due_date" value="${i.due_date ?? ""}" aria-label="${t("issues.dueDate")}" ${off("issue.edit")}></lds-date-picker>
        </div>`)}
      ${prop(t("issues.type"), html`
        <lds-select data-prop="type" value="${i.type_id}" ${off("issue.edit")}>${types.map((x) => html`<option value="${x.id}">${x.name}</option>`)}</lds-select>`)}
      ${prop(t("issues.parent"), html`
        <lds-input data-prop="parent" value="${this.parent?.key ?? ""}" placeholder="${d.project.key}-1" ${off("issue.edit")}></lds-input>`)}
      ${fields.length ? html`<div class="section-title" ${this.mode === "peek" ? html`hidden` : ""}>${t("issues.customFields")}</div>` : ""}
      ${fields.map((f) => prop(f.name + (f.required ? " *" : ""), this.fieldControl(f, values[f.id], off("issue.edit"))))}`;
  }

  fieldControl(f, value, off) {
    const { t } = this.context.i18n;
    const attrs = html`data-field="${f.id}" data-field-type="${f.type}" ${off}`;
    switch (f.type) {
      case "select":
        return html`<lds-select ${attrs} value="${value ?? ""}"><option value="">–</option>${f.options.map((o) => html`<option value="${o.id}">${o.name}</option>`)}</lds-select>`;
      case "multi_select":
        return html`<lds-combobox ${attrs} multiple value="${(value ?? []).join(",")}" label="${f.name}">${f.options.map((o) => html`<lds-option value="${o.id}">${o.name}</lds-option>`)}</lds-combobox>`;
      case "user":
        return html`<lds-combobox ${attrs} value="${value ?? ""}" label="${f.name}"><lds-option value="">–</lds-option>
          ${this.data.peopleList.map((p) => html`<lds-option value="${p.id}">${p.name}</lds-option>`)}</lds-combobox>`;
      case "checkbox":
        return html`<lds-switch ${attrs} ${value ? html`checked` : ""} aria-label="${f.name}"></lds-switch>`;
      case "date":
        return html`<lds-date-picker ${attrs} value="${value ?? ""}" aria-label="${f.name}"></lds-date-picker>`;
      case "number":
        return html`<lds-input ${attrs} type="number" value="${value ?? ""}" aria-label="${f.name}" suffix="${f.unit ?? ""}"></lds-input>`;
      default:
        return html`<lds-input ${attrs} type="${f.type === "url" ? "url" : "text"}" value="${value ?? ""}" aria-label="${f.name}" placeholder="${f.type === "url" ? "https://" : t("issues.empty")}"></lds-input>`;
    }
  }

  descriptionBlock() {
    const { t } = this.context.i18n;
    const i = this.issue;
    const readOnly = i.archived_at || !this.can.includes("issue.edit");
    // Plain paragraphs edit as text; anything richer shows as it is.
    if (!readOnly && isPlain(i.description_doc)) {
      return html`<lds-textarea data-prop="description" rows="${this.mode === "peek" ? 4 : 6}" auto-grow value="${i.description}" placeholder="${t("issues.describe")}" aria-label="${t("issues.description")}"></lds-textarea>`;
    }
    return i.description
      ? html`<div class="description">${renderDoc(i.description_doc, this.docContext())}</div>`
      : html`<div class="description"><span class="muted">${t("issues.noDescription")}</span></div>`;
  }

  /** What rendering documents needs: links, images and names. */
  docContext() {
    return { ws: this.ws, issueKey: this.issue.key, people: this.data.people };
  }

  childrenBlock() {
    const { t } = this.context.i18n;
    const done = this.subIssues.filter((c) => ["completed", "abandoned"].includes(this.data.statuses.get(c.status_id)?.outcome)).length;
    const canAdd = this.can.includes("issue.create") && !this.issue.archived_at && this.data.types.get(this.issue.type_id)?.parent_rule !== "required";
    if (!this.subIssues.length && !canAdd) return "";
    return html`
      <h2>${t("issues.subIssues")}
        ${this.subIssues.length ? html`<span class="progress">${t("issues.doneOf", { done, total: this.subIssues.length })}
          <lds-progress value="${done}" max="${this.subIssues.length}" label="${t("issues.subIssuesDone")}"></lds-progress></span>` : ""}
        <span class="grow"></span>
        ${canAdd ? html`<lds-button variant="ghost" size="sm" data-action="add-child"><lds-icon slot="start" name="plus" size="sm"></lds-icon>${t("issues.add")}</lds-button>` : ""}
      </h2>
      <div class="list">${this.subIssues.map((c) => html`
        <div class="item">${statusGlyph(this.data, c.status_id)}<span class="key">${c.key}</span>
          <a href="${issueHref(this.ws, c.key)}">${c.title}</a>${avatar(this.data, c.assignee_id)}</div>`)}</div>`;
  }

  linksBlock() {
    const { t } = this.context.i18n;
    const canLink = this.can.includes("issue.link") && !this.issue.archived_at;
    if (!this.links.length && !canLink) return "";
    return html`
      <h2>${t("issues.links")}</h2>
      <div class="list">${this.links.map((l) => html`
        <div class="item"><span class="type">${t(`link.${l.type}`)}${l.lag_days ? ` ${l.lag_days > 0 ? "+" : ""}${l.lag_days}d` : ""}</span>
          <span class="key">${l.key}</span><a href="${issueHref(this.ws, l.key)}">${l.title}</a>
          ${canLink ? html`<lds-button variant="ghost" size="sm" data-action="unlink" data-id="${l.id}" label="${t("issues.unlink", { key: l.key })}"><lds-icon name="x" size="sm"></lds-icon></lds-button>` : html`<span></span>`}
        </div>`)}</div>
      ${canLink ? html`
        <div class="linker" data-linker>
          <lds-select size="sm" data-link-type value="relates_to" aria-label="${t("issues.linkType")}">${LINK_TYPES.map((x) => html`<option value="${x}">${t(`link.${x}`)}</option>`)}</lds-select>
          <lds-input size="sm" data-link-target placeholder="${this.data.project.key}-1" aria-label="${t("issues.linkTarget")}"></lds-input>
          <lds-button size="sm" data-action="link">${t("issues.link")}</lds-button>
        </div>` : ""}`;
  }

  attachmentsBlock() {
    const { i18n } = this.context;
    const { t } = i18n;
    const canAttach = this.can.includes("issue.attach") && !this.issue.archived_at;
    if (!this.attachments.length && !canAttach) return "";
    const me = this.context.session.user?.id;
    return html`
      <h2>${t("issues.attachments")}<span class="grow"></span>
        ${canAttach ? html`<lds-button variant="ghost" size="sm" data-action="attach"><lds-icon slot="start" name="paperclip" size="sm"></lds-icon>${t("issues.attach")}</lds-button>
          <input type="file" data-file multiple>` : ""}</h2>
      <div class="list">${this.attachments.map((a) => html`
        <div class="item"><lds-icon name="paperclip" size="sm"></lds-icon><span></span>
          <a href="/api/v1${this.base}/attachments/${a.id}/download" target="_blank" rel="noopener">${a.filename}</a>
          <span class="muted">${i18n.number(Math.max(1, Math.round(a.size / 1024)))} KB
            ${a.uploader_id === me || this.can.includes("issue.comment.moderate") ? html`<lds-button variant="ghost" size="sm" data-action="detach" data-id="${a.id}" label="${t("issues.removeAttachment", { name: a.filename })}"><lds-icon name="trash-2" size="sm"></lds-icon></lds-button>` : ""}</span>
        </div>`)}</div>`;
  }

  /** Comments and events, oldest first, then the composer. */
  activityBlock() {
    const { i18n } = this.context;
    const { t } = i18n;
    const me = this.context.session.user?.id;
    const entries = [
      ...this.activity.filter((e) => !e.type.startsWith("issue.comment")).map((e) => ({ at: e.occurred_at, event: e })),
      ...this.comments.map((c) => ({ at: c.created_at, comment: c })),
    ].sort((a, b) => (a.at < b.at ? -1 : 1));
    const canComment = this.can.includes("issue.comment") && !this.issue.archived_at;
    return html`
      <h2>${t("issues.activity")}</h2>
      <div class="feed">${entries.map((x) => (x.comment ? html`
        <div class="comment" data-comment="${x.comment.id}">
          <lds-avatar size="sm" name="${x.comment.author_name || "?"}"></lds-avatar>
          <div class="box">
            <div class="who"><strong>${x.comment.author_name || t("issue.someone")}</strong>
              <span class="muted">${i18n.relative(x.comment.created_at)}${x.comment.edited_at ? ` · ${t("issues.edited")}` : ""}</span>
              <span class="tools">
                ${x.comment.author_id === me && canComment && isPlain(x.comment.body_doc) ? html`<lds-button variant="ghost" size="sm" data-action="edit-comment" data-id="${x.comment.id}">${t("issues.edit")}</lds-button>` : ""}
                ${x.comment.author_id === me || this.can.includes("issue.comment.moderate") ? html`<lds-button variant="ghost" size="sm" data-action="delete-comment" data-id="${x.comment.id}">${t("issues.remove")}</lds-button>` : ""}
              </span></div>
            <div class="body">${renderDoc(x.comment.body_doc, this.docContext())}</div>
          </div>
        </div>` : html`
        <div class="event"><span><strong>${x.event.actor?.name ?? t("issue.someone")}</strong> ${this.describe(x.event)}</span>
          <span>· ${i18n.relative(x.event.occurred_at)}</span></div>`))}</div>
      ${canComment ? html`
        <div class="composer" data-composer>
          <lds-textarea rows="3" auto-grow value="${this.draft ?? ""}" placeholder="${t("issues.commentPlaceholder")}" aria-label="${t("issues.comment")}"></lds-textarea>
          <div class="actions"><lds-button size="sm" data-action="comment">${t("issues.comment")}</lds-button></div>
        </div>` : ""}`;
  }

  /** One line for an event of the feed. */
  describe(e) {
    const { t } = this.context.i18n;
    const name = (v) => (v && typeof v === "object" ? v.key ? `${v.key} ${v.name ?? ""}`.trim() : v.name ?? "" : v ?? "");
    const p = e.payload;
    let kind = e.type.replace(/^issue\./, "");
    // In or out of an iteration or a release.
    if (kind === "iteration_changed" || kind === "release_changed") kind = `${kind.split("_")[0]}_${p.to ? "set" : "cleared"}`;
    const key = `activity.${kind}`;
    return this.context.i18n.has(key)
      ? t(key, { from: name(p.from), to: name(p.to), issue: name(p.issue_id), type: p.type ? t(`link.${p.type}`) : "",
        fields: (p.fields ?? []).join(", "), fromKey: p.from_key ?? "", toKey: p.to_key ?? "" })
      : kind.replaceAll("_", " ");
  }

  dialogs() {
    const { t } = this.context.i18n;
    return html`
      <hzn-issue-create></hzn-issue-create>
      <hzn-move-issue></hzn-move-issue>
      <lds-modal size="sm" data-dialog="delete">
        <span slot="header">${t("issues.deleteTitle", { key: this.issue.key })}</span>
        <p>${t("issues.deleteBody")}</p>
        ${this.subIssues.length ? html`
          <lds-field label="${t("issues.deleteChildren", { count: this.subIssues.length })}">
            <lds-radio-group data-children value="reparent">
              <button type="button" value="reparent">${t("issues.reparent")}</button>
              <button type="button" value="cascade">${t("issues.cascade")}</button>
            </lds-radio-group>
          </lds-field>` : ""}
        <lds-alert intent="danger" role="alert"></lds-alert>
        <div slot="footer" class="actions-row">
          <lds-button variant="ghost" data-action="close-dialog">${t("tokens.cancel")}</lds-button>
          <lds-button variant="danger" data-action="confirm-delete">${t("issues.delete")}</lds-button>
        </div>
      </lds-modal>`;
  }

  // --- Commands --------------------------------------------------------------------

  registerCommands() {
    const { commands, i18n } = this.context;
    if (!commands || this.mode !== "page") return;
    const { t } = i18n;
    this.offCommands?.();
    const focus = (name) => () => this.shadowRoot.querySelector(`[data-prop="${name}"]`)?.focus();
    this.offCommands = commands.set("issue", {
      heading: t("command.issue", { key: this.issue.key }),
      items: [
        { label: t("command.changeStatus"), icon: "circle-dot", keys: "s", run: focus("status") },
        { label: t("command.assign"), icon: "user", keys: "a", run: focus("assignee") },
        { label: t("command.priority"), icon: priorityIcon("high"), keys: "p", run: focus("priority") },
        { label: t("command.labels"), icon: "tag", keys: "l", run: focus("labels") },
        ...(this.can.includes("issue.move") ? [{ label: t("issues.move"), icon: "folder", keys: "m", run: () => this.move() }] : []),
        { label: t("issues.copyLink"), icon: "copy", keys: "mod+shift+c", run: () => this.copyLink() },
      ],
    });
  }

  // --- Changes ---------------------------------------------------------------------

  /** Runs a write on the issue with its version; reloads on success. */
  async write(fn) {
    const { i18n } = this.context;
    try {
      const res = await fn(this.etag);
      if (res?.data?.version) {
        this.etag = res.etag ?? `"${res.data.version}"`;
      }
    } catch (err) {
      if (err.code === "version_conflict") {
        globalThis.LDS?.toast?.({ intent: "warning", title: i18n.t("issues.conflict") });
      } else if (err.code === "transition_requires_fields") {
        const names = (err.fields ?? []).map((f) => this.data.fields.get(f.field)?.name ?? i18n.t(`issues.field.${f.field}`));
        globalThis.LDS?.toast?.({ intent: "danger", title: i18n.t("issues.transitionNeeds", { fields: names.join(", ") }) });
      } else {
        const field = err.fields?.[0];
        globalThis.LDS?.toast?.({ intent: "danger", title: field && i18n.has(`error.${field.field}.${field.code}`) ? i18n.t(`error.${field.field}.${field.code}`) : errorMessage(err, i18n) });
      }
    }
    await this.load();
    document.dispatchEvent(new CustomEvent("hzn-issue-changed", { detail: { key: this.issue?.key ?? "" } }));
  }

  patch(body) {
    return this.write((ifMatch) => this.context.api.patch(this.base, { body, ifMatch }));
  }

  post(path, body) {
    return this.write((ifMatch) => this.context.api.post(`${this.base}/${path}`, { body, ifMatch }));
  }

  onChange(e) {
    const el = e.target;
    const prop = el.dataset?.prop;
    const value = e.detail?.value ?? "";
    if (el.dataset?.field) return this.saveField(el, e.detail);
    switch (prop) {
      case "status":
        if (value !== this.issue.status_id) this.post("transition", { status_id: value });
        break;
      case "priority":
        if (value !== this.issue.priority) this.post("prioritize", { priority: value });
        break;
      case "assignee":
        if (value !== (this.issue.assignee_id ?? "")) this.post("assign", { assignee_id: value || null });
        break;
      case "labels": {
        const next = e.detail.values ?? (value ? value.split(",") : []);
        if (next.join(",") !== this.issue.labels.join(",")) this.patch({ labels: next });
        break;
      }
      case "estimate":
        if (value !== (this.issue.estimate ?? "")) this.post("estimate", { estimate: value });
        break;
      case "iteration":
        if (value !== (this.issue.iteration_id ?? "")) this.setIteration(value);
        break;
      case "release":
        if (value !== (this.issue.release_id ?? "")) this.post("release", { release_id: value || null });
        break;
      case "start_date":
      case "due_date":
        if (value !== (this.issue[prop] ?? "")) this.patch({ [prop]: value });
        break;
      case "type":
        if (value !== this.issue.type_id) this.changeType(value);
        break;
      case "parent":
        if (value.trim().toUpperCase() !== (this.parent?.key ?? "")) this.patch({ parent: value.trim() });
        break;
      case "description":
        if (value !== this.issue.description) this.patch({ description: value });
        break;
    }
  }

  saveField(el, detail) {
    const id = el.dataset.field;
    let v;
    switch (el.dataset.fieldType) {
      case "checkbox":
        v = !!detail.checked;
        break;
      case "number":
        v = detail.value === "" ? null : Number(detail.value);
        break;
      case "multi_select":
        v = detail.values?.length ? detail.values : null;
        break;
      default:
        v = detail.value === "" ? null : detail.value;
    }
    this.patch({ fields: { [id]: v } });
  }

  /** Loads the issue into an iteration, or takes it out of its own. */
  async setIteration(id) {
    const { api, i18n } = this.context;
    const i = this.issue;
    const base = `/w/${encodeURIComponent(this.ws)}/projects/${encodeURIComponent(this.data.project.key)}/iterations`;
    try {
      if (id) await api.post(`${base}/${id}/issues`, { body: { add: [i.key] } });
      else await api.post(`${base}/${i.iteration_id}/issues`, { body: { remove: [i.key] } });
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
    await this.load();
  }

  /** A type with another workflow needs a status in it: same name, else same outcome, else its initial one. */
  changeType(typeId) {
    const d = this.data;
    const body = { type_id: typeId };
    const from = d.types.get(this.issue.type_id);
    const to = d.types.get(typeId);
    if (from?.workflow_id !== to?.workflow_id) {
      const cur = d.statuses.get(this.issue.status_id);
      const options = d.statusesOf(typeId);
      const pick = options.find((s) => s.name === cur?.name) ?? options.find((s) => s.outcome === cur?.outcome) ?? options.find((s) => s.initial) ?? options[0];
      if (pick) body.status_id = pick.id;
    }
    this.patch(body);
  }

  onKey(e) {
    const title = e.target.closest?.("[data-title-input]");
    if (title && e.key === "Enter") {
      e.preventDefault();
      this.saveTitle(title);
    } else if (title && e.key === "Escape") {
      e.preventDefault();
      this.render();
    } else if (e.target.closest?.("[data-title]") && e.key === "Enter") {
      this.editTitle();
    } else if (e.target.closest?.("[data-composer]") && e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      this.comment();
    } else if (e.target.closest?.("[data-link-target]") && e.key === "Enter") {
      e.preventDefault();
      this.link();
    }
  }

  editTitle() {
    if (this.issue.archived_at || !this.can.includes("issue.edit")) return;
    const h1 = this.shadowRoot.querySelector("[data-title]");
    if (!h1) return;
    const input = document.createElement("lds-input");
    input.setAttribute("value", this.issue.title);
    input.setAttribute("maxlength", "255");
    input.setAttribute("aria-label", this.context.i18n.t("issues.titleField"));
    input.dataset.titleInput = "";
    input.addEventListener("focusout", () => setTimeout(() => input.isConnected && this.saveTitle(input), 0));
    h1.replaceWith(input);
    requestAnimationFrame(() => input.focus());
  }

  saveTitle(input) {
    const title = (input.getAttribute("value") ?? "").trim();
    input.removeAttribute("data-title-input");
    if (!title || title === this.issue.title) return this.render();
    this.patch({ title });
  }

  async onClick(e) {
    const el = e.target.closest?.("[data-action]");
    if (e.target.closest?.("[data-title]") && !el) return this.editTitle();
    if (!el) return;
    const { api, i18n } = this.context;
    switch (el.dataset.action) {
      case "copy":
        return this.copyLink();
      case "move":
        return this.move();
      case "archive":
      case "unarchive":
        return this.post(el.dataset.action);
      case "delete":
        return this.dialog("delete").setAttribute("open", "");
      case "close-dialog":
        return el.closest("lds-modal").removeAttribute("open");
      case "confirm-delete":
        return this.remove(el);
      case "add-child":
        return this.shadowRoot.querySelector("hzn-issue-create").show({ ws: this.ws, project: this.data.project.key, parent: this.issue.key });
      case "link":
        return this.link();
      case "unlink":
        await api.delete(`${this.base}/links/${el.dataset.id}`).catch((err) => globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) }));
        return this.load();
      case "attach":
        return this.upload(el);
      case "detach":
        await api.delete(`${this.base}/attachments/${el.dataset.id}`).catch((err) => globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) }));
        return this.load();
      case "comment":
        return this.comment(el);
      case "edit-comment":
        return this.editComment(el.dataset.id);
      case "save-comment":
        return this.saveComment(el);
      case "delete-comment":
        if (!confirm(i18n.t("issues.deleteComment"))) return;
        await api.delete(`${this.base}/comments/${el.dataset.id}`).catch((err) => globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) }));
        return this.load();
    }
  }

  dialog(name) {
    return this.shadowRoot.querySelector(`[data-dialog="${name}"]`);
  }

  async copyLink() {
    await navigator.clipboard?.writeText(new URL(issueHref(this.ws, this.issue.key), location.href).href).catch(() => {});
    globalThis.LDS?.toast?.({ intent: "success", title: this.context.i18n.t("issues.linkCopied") });
  }

  async move() {
    if (!this.can.includes("issue.move") || this.issue.archived_at) return;
    const moved = await this.shadowRoot.querySelector("hzn-move-issue").show({ ws: this.ws, issue: this.issue, children: this.subIssues.length });
    if (!moved) return;
    document.dispatchEvent(new CustomEvent("hzn-issue-changed", { detail: { key: moved.key } }));
    if (this.mode === "page") this.context.router.navigate(issueHref(this.ws, moved.key), { replace: true });
    else this.setAttribute("issue", moved.key);
  }

  async remove(button) {
    const { api, i18n } = this.context;
    const modal = this.dialog("delete");
    const children = modal.querySelector("[data-children]")?.getAttribute("value");
    try {
      await busy(button, () => api.delete(this.base, { query: children ? { children } : {} }));
    } catch (err) {
      modal.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      return;
    }
    modal.removeAttribute("open");
    const key = this.issue.key;
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("issues.deleted", { key }) });
    document.dispatchEvent(new CustomEvent("hzn-issue-changed", { detail: { key } }));
    this.dispatchEvent(new CustomEvent("hzn-issue-gone", { detail: { key }, bubbles: true, composed: true }));
    if (this.mode === "page") this.context.router.navigate(`/w/${this.ws}/p/${this.data.project.key}/issues`, { replace: true });
  }

  async link() {
    const { api, i18n } = this.context;
    const box = this.shadowRoot.querySelector("[data-linker]");
    const target = box.querySelector("[data-link-target]");
    const value = (target.getAttribute("value") ?? "").trim();
    if (!value) return target.focus();
    try {
      await busy(box.querySelector('[data-action="link"]'), () =>
        api.post(`${this.base}/links`, { body: { type: box.querySelector("[data-link-type]").getAttribute("value"), target: value } }));
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      return;
    }
    this.load();
  }

  async comment() {
    const { api, i18n } = this.context;
    const box = this.shadowRoot.querySelector("[data-composer]");
    const area = box.querySelector("lds-textarea");
    const body = (area.getAttribute("value") ?? "").trim();
    if (!body) return area.focus();
    try {
      await busy(box.querySelector('[data-action="comment"]'), () => api.post(`${this.base}/comments`, { body: { body } }));
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      return;
    }
    this.draft = "";
    await this.load();
  }

  editComment(id) {
    const c = this.comments.find((x) => x.id === id);
    const box = this.shadowRoot.querySelector(`[data-comment="${id}"] .body`);
    if (!c || !box) return;
    const { t } = this.context.i18n;
    box.innerHTML = String(html`
      <lds-textarea rows="3" auto-grow value="${c.body}" aria-label="${t("issues.comment")}"></lds-textarea>
      <div class="actions-row"><lds-button size="sm" data-action="save-comment" data-id="${id}" data-version="${c.version}">${t("account.save")}</lds-button></div>`);
    box.querySelector("lds-textarea").focus();
  }

  async saveComment(button) {
    const { api, i18n } = this.context;
    const area = button.closest(".body").querySelector("lds-textarea");
    try {
      await busy(button, () => api.patch(`${this.base}/comments/${button.dataset.id}`, { body: { body: area.getAttribute("value") ?? "" }, ifMatch: `"${button.dataset.version}"` }));
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      return;
    }
    this.load();
  }

  /** Uploads files: announce, PUT to the presigned URL, complete. */
  upload(button) {
    const input = this.shadowRoot.querySelector("[data-file]");
    input.onchange = async () => {
      const { api, i18n } = this.context;
      for (const file of input.files) {
        try {
          await busy(button, async () => {
            const { data: up } = await api.post(`${this.base}/attachments`, { body: { filename: file.name, content_type: file.type, size: file.size } });
            const res = await fetch(up.upload_url, { method: up.method, body: file });
            if (!res.ok) throw new Error(`upload failed: ${res.status}`);
            await api.post(`${this.base}/attachments/${up.attachment.id}/complete`);
          });
        } catch (err) {
          globalThis.LDS?.toast?.({ intent: "danger", title: i18n.t("issues.uploadFailed", { name: file.name }) + " " + errorMessage(err, i18n) });
        }
      }
      input.value = "";
      this.load();
    };
    input.click();
  }

  /** Reloads when someone else changes the issue. */
  onPush(m) {
    if (m.t !== "event" || !this.issue || m.d.aggregate?.id !== this.issue.id) return;
    if (m.d.type === "issue.deleted") {
      this.dispatchEvent(new CustomEvent("hzn-issue-gone", { detail: { key: this.issue.key }, bubbles: true, composed: true }));
      return;
    }
    clearTimeout(this.timer);
    this.timer = setTimeout(() => {
      this.load();
    }, 150);
  }

  /** Whether a draft has focus: a comment, a link target, a title being edited. */
  editing() {
    const el = this.shadowRoot.activeElement;
    return ["lds-input", "lds-textarea"].includes(el?.localName) && !!el.closest("[data-composer], [data-linker], [data-comment], [data-title-input]");
  }
}

customElements.define("hzn-issue-detail", HznIssueDetail);

