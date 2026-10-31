// <hzn-settings-workflows>: /w/:ws/settings/workflows[?project=KEY&workflow=ID]
// — the workflow editor (wireframes: Workflow, states edit and publish).
//
// Edits change a shared draft, saved as they happen (draft_version guards
// against overwriting someone else's save); publishing applies it, asking
// where the issues of removed statuses go.

import { newIdempotencyKey as uuid } from "horizon/core/api.js";
import { requestContext } from "horizon/core/context.js";
import { html, ldsLightSheet, sheet } from "horizon/core/dom.js";
import { errorMessage } from "horizon/core/forms.js";
import { loadScope, originBadge, scopePicker, statusIcon, watchScope } from "horizon/views/config-scope.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

const FIELDS = ["assignee", "estimate", "priority", "due_date", "start_date", "iteration", "release", "description", "labels"];
const SAVE_DELAY = 500;

const styles = sheet(`
  .content { padding: 0 !important; }
  .split { display: grid; grid-template-columns: minmax(0, 1fr) 20rem; min-height: calc(100vh - 4.5rem); }
  .main { padding: var(--lds-space-5) var(--lds-space-6); min-width: 0; }
  .panel { border-inline-start: 1px solid var(--lds-color-border-subtle); padding: var(--lds-space-5); display: grid; gap: var(--lds-space-4); align-content: start; }
  .panel h3, .section h3 { margin: 0; font-size: var(--lds-font-size-md); }
  .panel .eyebrow { font-size: var(--lds-font-size-2xs); letter-spacing: 0.08em; text-transform: uppercase; color: var(--lds-color-fg-muted); }
  .top { display: flex; gap: var(--lds-space-4); align-items: end; flex-wrap: wrap; margin-bottom: var(--lds-space-6); }
  .top .chips { display: flex; gap: var(--lds-space-2); align-items: center; flex-wrap: wrap; }
  .section { display: flex; align-items: center; justify-content: space-between; margin: var(--lds-space-6) 0 var(--lds-space-3); }
  .status { display: inline-flex; gap: var(--lds-space-2); align-items: center; }
  tr.added td { background: var(--lds-color-success-subtle, var(--lds-color-bg-subtle)); }
  tr.removed td { background: var(--lds-color-danger-subtle, var(--lds-color-bg-subtle)); }
  tr.removed .status span { text-decoration: line-through; }
  td.end { text-align: end; white-space: nowrap; width: 1%; }
  .matrix { overflow-x: auto; }
  .matrix th, .matrix td { text-align: center; font-size: var(--lds-font-size-xs); }
  .matrix th[scope="row"] { text-align: start; white-space: nowrap; }
  .matrix td.self { background: var(--lds-color-bg-muted); }
  .cell { width: 100%; min-height: 1.75rem; border: 0; background: none; cursor: pointer; font: inherit; color: var(--lds-color-fg-link);
    display: inline-flex; gap: var(--lds-space-1); align-items: center; justify-content: center; border-radius: var(--lds-radius-sm); }
  .cell:hover { background: var(--lds-color-bg-muted); }
  .cell[aria-pressed="true"] { outline: 2px solid var(--lds-color-accent-default); }
  .cell.empty { color: var(--lds-color-fg-subtle); }
  .legend { display: flex; gap: var(--lds-space-4); font-size: var(--lds-font-size-xs); color: var(--lds-color-fg-muted); }
  .actions-row { display: flex; gap: var(--lds-space-2); align-items: center; }
  .post { display: grid; grid-template-columns: 1fr 1fr auto; gap: var(--lds-space-2); align-items: center; }
  lds-modal ul { margin: 0 0 var(--lds-space-4); padding-inline-start: var(--lds-space-5); }
  .map { display: grid; grid-template-columns: 1fr auto 1fr; gap: var(--lds-space-3); align-items: center; margin-top: var(--lds-space-3); }
`);

class HznSettingsWorkflows extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [ldsLightSheet(), settingsStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("lds-select", (e) => this.onMenu(e));
    this.unsubscribe = this.context.router.subscribe((m) => m.route?.name === "settings-workflows" && this.load());
    this.load();
  }

  disconnectedCallback() {
    this.unsubscribe?.();
    clearTimeout(this.saveTimer);
  }

  async load() {
    const { api, i18n, router } = this.context;
    try {
      this.scope = await loadScope(this.context, this.getAttribute("ws"));
      const [wfs, cats, types, roles] = await Promise.all([
        api.get(`${this.scope.base}/workflows`), api.get(`${this.scope.base}/categories`),
        api.get(`${this.scope.base}/issue-types`), api.get(`/w/${encodeURIComponent(this.scope.ws.slug)}/roles`),
      ]);
      this.workflows = wfs.data.items;
      this.canManage = wfs.data.can_manage;
      this.categories = cats.data.items.filter((c) => !c.disabled);
      this.types = types.data.items;
      this.roles = roles.data.items;
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    const wanted = router.current?.query.get("workflow");
    const visible = this.workflows.filter((w) => !w.disabled);
    this.wf = visible.find((w) => w.id === wanted) ?? visible[0];
    this.useWorkflow();
    this.render();
  }

  /** Sets the working copy: the draft, or the published version. */
  useWorkflow() {
    const w = this.wf;
    if (!w) return;
    this.draftVersion = w.draft_version;
    this.hasDraft = !!w.draft;
    const src = w.draft ?? { statuses: w.statuses, transitions: w.transitions };
    this.draft = structuredClone({ statuses: src.statuses, transitions: src.transitions });
    if (!this.draft.transitions.some((tr) => tr.id === this.selected)) this.selected = null;
  }

  get editable() {
    return this.canManage && this.wf && !this.wf.inherited;
  }

  catOf(id) {
    return this.categories.find((c) => c.id === id);
  }

  /** Changes between the draft and the published workflow. */
  changes() {
    const w = this.wf;
    const pub = new Map(w.statuses.map((s) => [s.id, s]));
    const dr = new Map(this.draft.statuses.map((s) => [s.id, s]));
    const added = this.draft.statuses.filter((s) => !pub.has(s.id));
    const removed = w.statuses.filter((s) => !dr.has(s.id));
    const changedStatuses = this.draft.statuses.filter((s, i) => {
      const p = pub.get(s.id);
      return p && (p.name !== s.name || p.category_id !== s.category_id || p.initial !== s.initial || w.statuses.indexOf(p) !== i);
    });
    const key = (tr) => JSON.stringify([tr.from, tr.to, tr.name, tr.rules]);
    const pubTr = new Set(w.transitions.map(key));
    const drTr = new Set(this.draft.transitions.map(key));
    const addedTr = this.draft.transitions.filter((tr) => !pubTr.has(key(tr)));
    const removedTr = w.transitions.filter((tr) => !drTr.has(key(tr)));
    return { added, removed, changedStatuses, addedTr, removedTr, count: added.length + removed.length + changedStatuses.length + addedTr.length + removedTr.length };
  }

  render() {
    const { t } = this.context.i18n;
    const scope = this.scope;
    if (!this.wf) {
      this.shadowRoot.innerHTML = String(settingsPage(t, scope.ws, "workflows", html`<p class="muted">${t("workflows.none")}</p>`, scopePicker(t, scope)));
      watchScope(this.shadowRoot, this.context, scope.ws.slug, "workflows");
      return;
    }
    const ch = this.changes();
    const actions = html`
      ${scopePicker(t, scope)}
      ${this.hasDraft || ch.count ? html`<lds-badge intent="warning">${t("workflows.draftBadge", { count: ch.count })}</lds-badge>` : ""}
      ${this.editable && (this.hasDraft || ch.count) ? html`
        <lds-button variant="ghost" data-action="discard">${t("workflows.discard")}</lds-button>
        <lds-button data-action="publish" ${ch.count ? "" : html`disabled`}>${t("workflows.publish")}</lds-button>` : ""}`;
    const body = html`
      <div class="split">
        <div class="main">${this.top()}${this.statusesTable(ch)}${this.matrix()}</div>
        <aside class="panel">${this.sidePanel()}</aside>
      </div>
      ${this.dialogs()}`;
    this.shadowRoot.innerHTML = String(settingsPage(t, scope.ws, "workflows", body, actions, this.wf.name));
    watchScope(this.shadowRoot, this.context, scope.ws.slug, "workflows");
    this.bind();
  }

  top() {
    const { t } = this.context.i18n;
    const w = this.wf;
    const usedBy = this.types.filter((it) => w.used_by.includes(it.id));
    const source = this.workflows.find((x) => x.id === w.copied_from);
    return html`
      <div class="top">
        <lds-field label="${t("workflows.workflow")}">
          <lds-select data-pick value="${w.id}">${this.workflows.map((x) => html`<option value="${x.id}">${x.name}${x.disabled ? ` (${t("config.switchedOff")})` : ""}</option>`)}</lds-select>
        </lds-field>
        <div><div class="muted">${t("workflows.usedBy")}</div>
          <div class="chips">${usedBy.length ? usedBy.map((it) => html`<lds-badge size="sm"><lds-icon slot="start" name="${it.icon}"></lds-icon>${it.name}</lds-badge>`) : html`<span class="muted">${t("workflows.unused")}</span>`}
          ${originBadge(t, w, this.scope)}</div></div>
        ${source ? html`<span class="muted">${t("workflows.copiedFrom", { name: source.name })}</span>` : ""}
        ${this.canManage ? html`
          <div class="actions-row">
            <lds-button variant="secondary" size="sm" data-action="copy"><lds-icon slot="start" name="copy"></lds-icon>${t("workflows.copy")}</lds-button>
            ${this.editable ? html`<lds-menu placement="bottom-end" data-wf-menu>
              <lds-button slot="trigger" size="sm" variant="ghost" label="${t("members.actionsFor", { name: w.name })}"><lds-icon name="more-horizontal"></lds-icon></lds-button>
              <lds-menu-item value="rename">${t("workflows.rename")}</lds-menu-item>
              <lds-menu-item value="delete">${t("workflows.delete")}</lds-menu-item>
            </lds-menu>` : ""}
          </div>` : ""}
      </div>`;
  }

  statusesTable(ch) {
    const { t } = this.context.i18n;
    const removed = new Set(ch.removed.map((s) => s.id));
    const added = new Set(ch.added.map((s) => s.id));
    const rows = [...this.draft.statuses.map((s) => ({ s, removed: false })), ...ch.removed.map((s) => ({ s, removed: true }))];
    return html`
      <div class="section"><h3>${t("workflows.statuses")}</h3>
        ${this.editable ? html`<lds-button variant="ghost" size="sm" data-action="add-status"><lds-icon slot="start" name="plus"></lds-icon>${t("workflows.addStatus")}</lds-button>` : ""}</div>
      <lds-table density="compact">
        <table>
          <thead><tr>
            <th scope="col">${t("workflows.name")}</th><th scope="col">${t("categories.category")}</th>
            <th scope="col">${t("categories.outcome")}</th><th scope="col"><span class="lds-visually-hidden">${t("account.actions")}</span></th>
          </tr></thead>
          <tbody>${rows.map(({ s, removed: gone }) => {
            const cat = this.catOf(s.category_id);
            const edit = this.editable && !gone;
            return html`
              <tr class="${gone ? "removed" : added.has(s.id) ? "added" : ""}">
                <td><span class="status"><lds-icon name="${statusIcon(cat?.outcome)}"></lds-icon>
                  ${edit ? html`<lds-input size="sm" data-status-name="${s.id}" value="${s.name}" maxlength="40" label="${t("workflows.name")}"></lds-input>` : html`<span>${s.name}</span>`}
                  ${s.initial ? html`<lds-badge size="sm" intent="info">${t("workflows.initial")}</lds-badge>` : ""}
                  ${added.has(s.id) ? html`<lds-badge size="sm" intent="success">${t("workflows.new")}</lds-badge>` : ""}
                  ${gone ? html`<lds-badge size="sm" intent="danger">${t("workflows.removed")}</lds-badge>` : ""}</span></td>
                <td>${edit ? html`<lds-select size="sm" data-status-cat="${s.id}" value="${s.category_id}">
                    ${this.categories.map((c) => html`<option value="${c.id}">${c.name}</option>`)}</lds-select>` : cat?.name ?? "—"}</td>
                <td class="muted">${cat ? t(`outcome.${cat.outcome}`) : ""}</td>
                <td class="end">${edit ? html`
                  <lds-menu placement="bottom-end" data-status-menu="${s.id}">
                    <lds-button slot="trigger" size="sm" variant="ghost" label="${t("members.actionsFor", { name: s.name })}"><lds-icon name="more-horizontal"></lds-icon></lds-button>
                    ${s.initial ? "" : html`<lds-menu-item value="initial">${t("workflows.makeInitial")}</lds-menu-item>`}
                    <lds-menu-item value="up">${t("categories.up")}</lds-menu-item>
                    <lds-menu-item value="down">${t("categories.down")}</lds-menu-item>
                    ${s.initial ? "" : html`<lds-menu-item value="remove">${t("workflows.removeStatus")}</lds-menu-item>`}
                  </lds-menu>` : gone && this.editable ? html`<lds-button size="sm" variant="ghost" data-restore="${s.id}">${t("workflows.restore")}</lds-button>` : ""}</td>
              </tr>`;
          })}</tbody>
        </table>
      </lds-table>
      <p class="muted">${t("workflows.categoriesNote")}</p>`;
  }

  matrix() {
    const { t } = this.context.i18n;
    const st = this.draft.statuses;
    const find = (from, to) => this.draft.transitions.find((tr) => (tr.from ?? null) === from && tr.to === to);
    const cell = (from, to) => {
      if (from === to) return html`<td class="self"></td>`;
      const tr = find(from, to);
      const ruled = tr && (tr.rules?.roles?.length || tr.rules?.validators?.length || tr.rules?.post_actions?.length || tr.rules?.children_final);
      if (!tr) {
        return html`<td>${this.editable ? html`<button class="cell empty" data-add-from="${from ?? ""}" data-add-to="${to}" aria-label="${t("workflows.addTransition")}">+</button>` : ""}</td>`;
      }
      return html`<td><button class="cell" data-tr="${tr.id}" aria-pressed="${tr.id === this.selected ? "true" : "false"}">
        <lds-icon name="${ruled ? "lock" : "check"}"></lds-icon>${tr.name}</button></td>`;
    };
    return html`
      <div class="section"><h3>${t("workflows.transitions")} <span class="muted">${t("workflows.rowToColumn")}</span></h3>
        <span class="legend"><span><lds-icon name="check"></lds-icon> ${t("workflows.allowed")}</span><span><lds-icon name="lock"></lds-icon> ${t("workflows.withRules")}</span></span></div>
      <lds-table density="compact" class="matrix">
        <table>
          <thead><tr><th scope="col">${t("workflows.fromTo")}</th>${st.map((s) => html`<th scope="col">${s.name}</th>`)}</tr></thead>
          <tbody>
            <tr><th scope="row"><em>${t("workflows.anyStatus")}</em></th>${st.map((to) => cell(null, to.id))}</tr>
            ${st.map((from) => html`<tr><th scope="row">${from.name}</th>${st.map((to) => cell(from.id, to.id))}</tr>`)}
          </tbody>
        </table>
      </lds-table>`;
  }

  sidePanel() {
    const { t } = this.context.i18n;
    const tr = this.draft.transitions.find((x) => x.id === this.selected);
    if (!tr) return html`<p class="muted">${t("workflows.pickTransition")}</p>`;
    const name = (id) => this.draft.statuses.find((s) => s.id === id)?.name ?? t("workflows.anyStatus");
    const rules = tr.rules ?? {};
    const off = this.editable ? "" : html`disabled`;
    return html`
      <div class="eyebrow">${t("workflows.transition")}</div>
      <lds-input data-tr-name value="${tr.name}" maxlength="40" ${this.editable ? "" : html`readonly`} label="${t("workflows.name")}"></lds-input>
      <span class="muted">${name(tr.from)} → ${name(tr.to)}</span>
      <h3>${t("workflows.conditions")}</h3>
      <lds-field label="${t("workflows.whoMay")}">
        <lds-combobox multiple data-rule="roles" value="${(rules.roles ?? []).join(",")}" placeholder="${t("workflows.anyoneWithTransition")}" ${off}>
          ${this.roles.map((r) => html`<lds-option value="${r.id}"><lds-icon slot="start" name="lock"></lds-icon>${r.name}</lds-option>`)}
        </lds-combobox>
      </lds-field>
      <lds-switch data-rule="children_final" ${rules.children_final ? html`checked` : ""} ${off}>${t("workflows.childrenFinal")}</lds-switch>
      <h3>${t("workflows.validators")}</h3>
      <lds-combobox multiple data-rule="validators" value="${(rules.validators ?? []).join(",")}" placeholder="${t("workflows.noValidators")}" ${off}>
        ${FIELDS.map((f) => html`<lds-option value="${f}">${t(`field.builtin.${f}`)}</lds-option>`)}
      </lds-combobox>
      <h3>${t("workflows.postActions")}</h3>
      ${(rules.post_actions ?? []).map((pa, i) => html`
        <div class="post">
          <lds-select size="sm" data-post-field="${i}" value="${pa.field}" ${off}>${FIELDS.map((f) => html`<option value="${f}">${t(`field.builtin.${f}`)}</option>`)}</lds-select>
          <lds-select size="sm" data-post-op="${i}" value="${pa.op}" ${off}>
            <option value="clear">${t("workflows.op.clear")}</option>
            ${pa.field === "assignee" ? html`<option value="set_actor">${t("workflows.op.set_actor")}</option>` : ""}
          </lds-select>
          ${this.editable ? html`<lds-button size="sm" variant="ghost" data-post-remove="${i}" label="${t("grants.remove")}"><lds-icon name="x"></lds-icon></lds-button>` : html`<span></span>`}
        </div>`)}
      ${this.editable ? html`
        <lds-button size="sm" variant="ghost" data-action="add-post"><lds-icon slot="start" name="plus"></lds-icon>${t("workflows.addPostAction")}</lds-button>
        <lds-button variant="ghost" data-action="remove-transition"><lds-icon slot="start" name="trash-2"></lds-icon>${t("workflows.removeTransition")}</lds-button>` : ""}`;
  }

  dialogs() {
    if (!this.canManage) return "";
    const { t } = this.context.i18n;
    return html`
      <lds-modal data-dialog="publish" size="md">
        <span slot="header">${t("workflows.publishTitle", { name: this.wf.name })}</span>
        <div data-publish-body></div>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("workflows.keepEditing")}</lds-button>
          <lds-button data-action="confirm-publish">${t("workflows.publish")}</lds-button>
        </div>
      </lds-modal>
      <lds-modal data-dialog="name" size="sm">
        <span slot="header" data-name-title></span>
        <lds-field label="${t("workflows.name")}" required><lds-input data-name-input maxlength="60"></lds-input></lds-field>
        <lds-alert intent="danger" role="alert"></lds-alert>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button data-action="confirm-name">${t("account.save")}</lds-button>
        </div>
      </lds-modal>`;
  }

  /** Wires the inputs rendered by render(). */
  bind() {
    const root = this.shadowRoot;
    root.querySelector("[data-pick]")?.addEventListener("lds-change", (e) => {
      const q = new URLSearchParams(this.context.router.current.query);
      q.set("workflow", e.detail.value);
      this.context.router.navigate(`/w/${this.scope.ws.slug}/settings/workflows?${q}`, { replace: true });
    });
    for (const input of root.querySelectorAll("[data-status-name]")) {
      input.addEventListener("lds-change", (e) => this.edit(() => (this.status(input.dataset.statusName).name = e.detail.value.trim())));
    }
    for (const select of root.querySelectorAll("[data-status-cat]")) {
      select.addEventListener("lds-change", (e) => this.edit(() => (this.status(select.dataset.statusCat).category_id = e.detail.value)));
    }
    const tr = this.draft.transitions.find((x) => x.id === this.selected);
    if (!tr || !this.editable) return;
    tr.rules ??= {};
    root.querySelector("[data-tr-name]")?.addEventListener("lds-change", (e) => this.edit(() => (tr.name = e.detail.value.trim())));
    for (const box of root.querySelectorAll("lds-combobox[data-rule]")) {
      box.addEventListener("lds-change", (e) => this.edit(() => (tr.rules[box.dataset.rule] = e.detail.values ?? [])));
    }
    root.querySelector('lds-switch[data-rule="children_final"]')?.addEventListener("lds-change", (e) =>
      this.edit(() => (tr.rules.children_final = !!(e.detail.checked ?? e.target.hasAttribute("checked")))));
    for (const s of root.querySelectorAll("[data-post-field]")) {
      s.addEventListener("lds-change", (e) => this.edit(() => {
        const pa = tr.rules.post_actions[s.dataset.postField];
        pa.field = e.detail.value;
        if (pa.field !== "assignee") pa.op = "clear";
      }));
    }
    for (const s of root.querySelectorAll("[data-post-op]")) {
      s.addEventListener("lds-change", (e) => this.edit(() => (tr.rules.post_actions[s.dataset.postOp].op = e.detail.value)));
    }
  }

  status(id) {
    return this.draft.statuses.find((s) => s.id === id);
  }

  /** Applies a change to the working copy, re-renders and saves soon. */
  edit(change) {
    change();
    this.render();
    clearTimeout(this.saveTimer);
    this.saveTimer = setTimeout(() => this.save(), SAVE_DELAY);
  }

  async save() {
    const { api, i18n } = this.context;
    try {
      const { data } = await api.put(`${this.scope.base}/workflows/${this.wf.id}/draft`, { body: { version: this.draftVersion, draft: this.draft } });
      this.draftVersion = data.draft_version;
      this.hasDraft = true;
      this.wf = data;
    } catch (err) {
      if (err.code === "version_conflict") {
        globalThis.LDS?.toast?.({ intent: "warning", title: i18n.t("workflows.conflict") });
        return this.load();
      }
      // Invalid drafts are kept locally; the error says what to fix.
      const field = err.fields?.[0];
      globalThis.LDS?.toast?.({ intent: "danger", title: field ? i18n.t("workflows.invalid", { field: field.field, code: field.code }) : errorMessage(err, i18n) });
    }
  }

  onClick(e) {
    const el = e.target.closest?.("[data-action], [data-close], [data-tr], [data-add-to], [data-restore], [data-post-remove]");
    if (!el) return;
    const { t } = this.context.i18n;
    if (el.hasAttribute("data-close")) return el.closest("lds-modal").removeAttribute("open");
    if (el.dataset.tr) {
      this.selected = el.dataset.tr;
      return this.render();
    }
    if (el.hasAttribute("data-add-to")) {
      const to = el.dataset.addTo;
      const tr = { id: uuid(), from: el.dataset.addFrom || null, to, name: t("workflows.toStatus", { name: this.status(to).name }), rules: {} };
      this.selected = tr.id;
      return this.edit(() => this.draft.transitions.push(tr));
    }
    if (el.dataset.restore) {
      const s = this.wf.statuses.find((x) => x.id === el.dataset.restore);
      return this.edit(() => this.draft.statuses.push({ ...s, initial: false }));
    }
    if (el.dataset.postRemove) {
      const tr = this.draft.transitions.find((x) => x.id === this.selected);
      return this.edit(() => tr.rules.post_actions.splice(Number(el.dataset.postRemove), 1));
    }
    switch (el.dataset.action) {
      case "add-status": {
        const cat = this.categories.find((c) => c.outcome === "started") ?? this.categories[0];
        return this.edit(() => this.draft.statuses.push({ id: uuid(), name: t("workflows.newStatus", { n: this.draft.statuses.length + 1 }), category_id: cat.id, initial: false }));
      }
      case "add-post": {
        const tr = this.draft.transitions.find((x) => x.id === this.selected);
        return this.edit(() => (tr.rules.post_actions ??= []).push({ field: "assignee", op: "clear" }));
      }
      case "remove-transition":
        this.edit(() => (this.draft.transitions = this.draft.transitions.filter((x) => x.id !== this.selected)));
        this.selected = null;
        return this.render();
      case "discard":
        return this.discard();
      case "publish":
        return this.openPublish();
      case "confirm-publish":
        return this.publish();
      case "copy":
        return this.openName("copy");
      case "confirm-name":
        return this.confirmName();
    }
  }

  onMenu(e) {
    const value = e.detail?.value ?? e.target.getAttribute?.("value");
    const statusMenu = e.target.closest?.("[data-status-menu]");
    if (statusMenu) {
      const id = statusMenu.dataset.statusMenu;
      const list = this.draft.statuses;
      const i = list.findIndex((s) => s.id === id);
      if (value === "initial") return this.edit(() => list.forEach((s) => (s.initial = s.id === id)));
      if (value === "up" && i > 0) return this.edit(() => ([list[i - 1], list[i]] = [list[i], list[i - 1]]));
      if (value === "down" && i < list.length - 1) return this.edit(() => ([list[i + 1], list[i]] = [list[i], list[i + 1]]));
      if (value === "remove") {
        return this.edit(() => {
          this.draft.statuses = list.filter((s) => s.id !== id);
          this.draft.transitions = this.draft.transitions.filter((tr) => tr.to !== id && tr.from !== id);
        });
      }
      return;
    }
    if (e.target.closest?.("[data-wf-menu]")) {
      if (value === "rename") this.openName("rename");
      if (value === "delete" && confirm(this.context.i18n.t("workflows.deleteConfirm", { name: this.wf.name }))) {
        this.context.api.delete(`${this.scope.base}/workflows/${this.wf.id}`)
          .then(() => this.context.router.navigate(`/w/${this.scope.ws.slug}/settings/workflows${this.scope.project ? `?project=${this.scope.project.key}` : ""}`, { replace: true }))
          .catch((err) => globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, this.context.i18n) }));
      }
    }
  }

  async discard() {
    clearTimeout(this.saveTimer);
    try {
      await this.context.api.delete(`${this.scope.base}/workflows/${this.wf.id}/draft`);
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, this.context.i18n) });
    }
    this.load();
  }

  openPublish(unmapped = null) {
    const { t } = this.context.i18n;
    const ch = this.changes();
    const types = this.types.filter((it) => this.wf.used_by.includes(it.id)).map((it) => it.name);
    const modal = this.shadowRoot.querySelector('[data-dialog="publish"]');
    modal.querySelector("[data-publish-body]").innerHTML = String(html`
      <p class="muted">${types.length ? t("workflows.publishLead", { types: types.join(", ") }) : t("workflows.publishLeadUnused")}</p>
      <ul>
        ${ch.added.map((s) => html`<li>${t("workflows.changeAdded", { name: s.name })}</li>`)}
        ${ch.removed.map((s) => html`<li>${t("workflows.changeRemoved", { name: s.name })}</li>`)}
        ${ch.changedStatuses.length ? html`<li>${t("workflows.changeStatuses", { count: ch.changedStatuses.length })}</li>` : ""}
        ${ch.addedTr.length ? html`<li>${t("workflows.changeTransitionsAdded", { names: ch.addedTr.map((x) => x.name).join(", ") })}</li>` : ""}
        ${ch.removedTr.length ? html`<li>${t("workflows.changeTransitionsRemoved", { names: ch.removedTr.map((x) => x.name).join(", ") })}</li>` : ""}
      </ul>
      ${unmapped ? html`
        <lds-alert intent="warning"><span slot="title">${t("workflows.unmappedTitle", { count: unmapped.reduce((n, u) => n + u.count, 0) })}</span>${t("workflows.unmappedBody")}</lds-alert>
        ${unmapped.map((u) => html`<div class="map" data-from="${u.id}"><span>${u.name} · ${t("types.issuesCount", { count: u.count })}</span>
          <lds-icon name="chevron-right"></lds-icon>
          <lds-select value="${this.suggest(u.id)}">${this.draft.statuses.map((s) => html`<option value="${s.id}">${s.name}</option>`)}</lds-select></div>`)}` : ""}`);
    modal.setAttribute("open", "");
  }

  /** Suggests a status of the same outcome for a removed one. */
  suggest(removedId) {
    const outcome = this.catOf(this.wf.statuses.find((s) => s.id === removedId)?.category_id)?.outcome;
    return (this.draft.statuses.find((s) => this.catOf(s.category_id)?.outcome === outcome) ?? this.draft.statuses[0]).id;
  }

  async publish() {
    const { api, i18n } = this.context;
    clearTimeout(this.saveTimer);
    await this.save();
    const modal = this.shadowRoot.querySelector('[data-dialog="publish"]');
    const mapping = {};
    for (const row of modal.querySelectorAll("[data-from]")) mapping[row.dataset.from] = row.querySelector("lds-select").getAttribute("value");
    try {
      await api.post(`${this.scope.base}/workflows/${this.wf.id}/publish`, { body: { version: this.draftVersion, status_mapping: mapping } });
    } catch (err) {
      if (err.code === "unmapped_statuses") {
        const unmapped = err.fields.map((f) => {
          const id = f.field.replace("status_mapping.", "");
          return { id, count: Number(f.code.split(":")[1] ?? 0), name: this.wf.statuses.find((s) => s.id === id)?.name ?? id };
        });
        return this.openPublish(unmapped);
      }
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      return;
    }
    modal.removeAttribute("open");
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("workflows.published", { name: this.wf.name }) });
    this.load();
  }

  openName(mode) {
    const { t } = this.context.i18n;
    this.nameMode = mode;
    const modal = this.shadowRoot.querySelector('[data-dialog="name"]');
    modal.querySelector("[data-name-title]").textContent = mode === "copy" ? t("workflows.copyTitle", { name: this.wf.name }) : t("workflows.rename");
    modal.querySelector("[data-name-input]").setAttribute("value", mode === "copy" ? t("workflows.copyName", { name: this.wf.name }) : this.wf.name);
    modal.querySelector("lds-alert").textContent = "";
    modal.setAttribute("open", "");
  }

  async confirmName() {
    const { api, i18n, router } = this.context;
    const modal = this.shadowRoot.querySelector('[data-dialog="name"]');
    const name = modal.querySelector("[data-name-input]").getAttribute("value") ?? "";
    let created;
    try {
      if (this.nameMode === "copy") ({ data: created } = await api.post(`${this.scope.base}/workflows`, { body: { name, copy_from: this.wf.id } }));
      else await api.patch(`${this.scope.base}/workflows/${this.wf.id}`, { body: { name } });
    } catch (err) {
      modal.querySelector("lds-alert").textContent = err.fields?.length ? i18n.t(`error.field.${err.fields[0].code}`) : errorMessage(err, i18n);
      return;
    }
    modal.removeAttribute("open");
    if (created) {
      const q = new URLSearchParams(router.current.query);
      q.set("workflow", created.id);
      router.navigate(`/w/${this.scope.ws.slug}/settings/workflows?${q}`, { replace: true });
    } else this.load();
  }
}

customElements.define("hzn-settings-workflows", HznSettingsWorkflows);
