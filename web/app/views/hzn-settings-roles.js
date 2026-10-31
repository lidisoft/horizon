// <hzn-settings-roles>: /w/:ws/settings/roles[?view=grants] — the
// permission matrix (wireframes: Roles) and the list of grants. Editing needs
// workspace.roles.manage; changes to the matrix are kept pending, then saved
// together.

import { requestContext } from "horizon/core/context.js";
import { html, ldsLightSheet, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, formValues, showFieldErrors } from "horizon/core/forms.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

const styles = sheet(`
  .lead { margin: 0 0 var(--lds-space-4); max-width: 48rem; }
  .matrix { overflow-x: auto; }
  .matrix table { min-width: 100%; }
  .matrix th.role { text-align: center; min-width: 7.5rem; vertical-align: bottom; }
  .matrix th.role small { display: block; font-weight: normal; color: var(--lds-color-fg-muted); }
  .matrix .head { display: inline-flex; align-items: center; gap: var(--lds-space-1); }
  .matrix td.cell { text-align: center; }
  .matrix td.cell.changed { background: var(--lds-color-warning-subtle, var(--lds-color-bg-muted)); }
  .matrix tr.area th, .matrix tr.area td { background: var(--lds-color-bg-subtle); font-size: var(--lds-font-size-2xs);
    letter-spacing: 0.08em; text-transform: uppercase; color: var(--lds-color-fg-muted); }
  .matrix tr.area td { text-align: center; letter-spacing: normal; text-transform: none; }
  .key { font-family: var(--lds-font-family-mono); font-size: var(--lds-font-size-xs); white-space: nowrap; }
  .desc { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  input[type="checkbox"] { width: 1rem; height: 1rem; accent-color: var(--lds-color-accent-default); cursor: pointer; }
  input[type="checkbox"]:disabled { cursor: default; }
  td.end { text-align: end; white-space: nowrap; width: 1%; }
  .toolbar { display: flex; justify-content: flex-end; margin-bottom: var(--lds-space-4); }
  lds-modal form { display: grid; gap: var(--lds-space-4); }
`);

class HznSettingsRoles extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [ldsLightSheet(), settingsStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("change", (e) => this.onToggle(e));
    root.addEventListener("lds-select", (e) => this.onMenu(e));
    this.unsubscribe = this.context.router.subscribe(() => this.render());
    this.load();
  }

  disconnectedCallback() {
    this.unsubscribe?.();
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.load();
  }

  get view() {
    return this.context.router.current?.query.get("view") === "grants" ? "grants" : "permissions";
  }

  get base() {
    return `/w/${encodeURIComponent(this.ws.slug)}`;
  }

  async load() {
    const { api, i18n } = this.context;
    const slug = encodeURIComponent(this.getAttribute("ws"));
    try {
      const [ws, roles, grants] = await Promise.all([api.get(`/w/${slug}`), api.get(`/w/${slug}/roles`), api.get(`/w/${slug}/grants`)]);
      this.ws = ws.data;
      this.roles = roles.data.items;
      this.catalogue = roles.data.catalogue;
      this.grants = grants.data.items;
      this.canManage = this.ws.can.includes("workspace.roles.manage");
      if (this.canManage) {
        const [members, teams] = await Promise.all([api.get(`/w/${slug}/members`), api.get(`/w/${slug}/teams`)]);
        this.members = members.data.items;
        this.teams = teams.data.items;
      }
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    // Pending edits: role id → Set of permissions.
    this.pending = new Map();
    this.render();
  }

  /** The permissions of a role, with pending edits applied. */
  perms(role) {
    return this.pending.get(role.id) ?? new Set(role.permissions);
  }

  changeCount() {
    let n = 0;
    for (const role of this.roles) {
      const want = this.pending.get(role.id);
      if (!want) continue;
      const have = new Set(role.permissions);
      for (const p of want) if (!have.has(p)) n++;
      for (const p of have) if (!want.has(p)) n++;
    }
    return n;
  }

  render() {
    if (!this.ws) return;
    const { t } = this.context.i18n;
    const view = this.view;
    const changes = this.changeCount();
    const actions = html`
      <lds-segmented size="sm" value="${view}" data-view label="${t("roles.view")}">
        <lds-segment value="permissions">${t("roles.tabPermissions")}</lds-segment>
        <lds-segment value="grants">${t("roles.tabGrants")}</lds-segment>
      </lds-segmented>
      ${this.canManage && view === "permissions" ? html`
        <lds-button variant="secondary" data-action="new-role"><lds-icon slot="start" name="plus"></lds-icon>${t("roles.new")}</lds-button>
        ${changes ? html`<lds-button variant="ghost" data-action="discard">${t("roles.discard")}</lds-button>` : ""}
        <lds-button data-action="save" ${changes ? "" : html`disabled`}>${changes ? t("roles.save", { count: changes }) : t("account.save")}</lds-button>` : ""}
      ${this.canManage && view === "grants" ? html`<lds-button data-action="new-grant"><lds-icon slot="start" name="plus"></lds-icon>${t("grants.add")}</lds-button>` : ""}`;
    this.shadowRoot.innerHTML = String(settingsPage(t, this.ws, "roles",
      html`<p class="muted lead">${t("roles.lead")}</p>${view === "grants" ? this.grantsView() : this.matrix()}${this.dialogs()}`, actions));
    this.shadowRoot.querySelector("[data-view]").addEventListener("lds-change", (e) => {
      if (this.changeCount() && !confirm(t("roles.leaveUnsaved"))) {
        e.target.setAttribute("value", this.view);
        return;
      }
      this.pending = new Map();
      const q = e.detail.value === "grants" ? "?view=grants" : "";
      this.context.router.navigate(`/w/${this.ws.slug}/settings/roles${q}`, { replace: true });
    });
  }

  matrix() {
    const { t, has } = this.context.i18n;
    const people = (r) => [r.people ? t("roles.people", { count: r.people }) : "", r.teams ? t("roles.teams", { count: r.teams }) : ""].filter(Boolean).join(" · ") || t("roles.nobody");
    return html`
      <lds-table density="compact" class="matrix">
        <table>
          <thead><tr>
            <th scope="col">${t("roles.permission")}</th>
            ${this.roles.map((r) => html`<th scope="col" class="role">
              <span class="head">${r.name}${r.builtin ? "" : html` <lds-badge size="sm" intent="info">${t("roles.custom")}</lds-badge>`}
                ${this.canManage && r.builtin !== "admin" ? html`
                  <lds-menu placement="bottom-end" data-role="${r.id}">
                    <lds-button slot="trigger" size="sm" variant="ghost" label="${t("members.actionsFor", { name: r.name })}"><lds-icon name="more-horizontal"></lds-icon></lds-button>
                    <lds-menu-item value="rename">${t("roles.rename")}</lds-menu-item>
                    <lds-menu-item value="delete">${t("roles.delete")}</lds-menu-item>
                  </lds-menu>` : ""}
              </span>
              <small>${people(r)}</small></th>`)}
          </tr></thead>
          <tbody>
            ${this.catalogue.map((area) => html`
              <tr class="area"><th scope="rowgroup">${t(`perm.area.${area.key}`)}</th>
                ${this.roles.map((r) => {
                  const p = this.perms(r);
                  return html`<td>${area.permissions.filter((k) => p.has(k)).length} / ${area.permissions.length}</td>`;
                })}</tr>
              ${area.permissions.map((key) => html`
                <tr>
                  <th scope="row"><span class="key">${key}</span><br><span class="desc">${has(`perm.${key}`) ? t(`perm.${key}`) : ""}</span></th>
                  ${this.roles.map((r) => {
                    const on = this.perms(r).has(key);
                    const changed = on !== r.permissions.includes(key);
                    // The Admin role always keeps workspace.roles.manage.
                    const locked = !this.canManage || (r.builtin === "admin" && key === "workspace.roles.manage");
                    return html`<td class="cell${changed ? " changed" : ""}"><input type="checkbox" data-role="${r.id}" data-perm="${key}"
                      aria-label="${t("roles.cellLabel", { role: r.name, permission: key })}" ${on ? html`checked` : ""} ${locked ? html`disabled` : ""}></td>`;
                  })}
                </tr>`)}`)}
          </tbody>
        </table>
      </lds-table>`;
  }

  grantsView() {
    const { t } = this.context.i18n;
    if (!this.grants.length) return html`<p class="muted">${t("grants.none")}</p>`;
    return html`
      <lds-table density="compact">
        <table>
          <thead><tr>
            <th scope="col">${t("grants.subject")}</th>
            <th scope="col">${t("grants.role")}</th>
            <th scope="col">${t("grants.scope")}</th>
            <th scope="col"><span class="lds-visually-hidden">${t("account.actions")}</span></th>
          </tr></thead>
          <tbody>${this.grants.map((g) => html`
            <tr>
              <td>${g.team_id ? html`<lds-icon name="users"></lds-icon> ` : ""}${g.subject_name}${g.subject_email ? html` <span class="muted">${g.subject_email}</span>` : ""}</td>
              <td>${g.role_name}</td>
              <td>${g.project_id ? t("grants.project") : t("grants.workspace")}</td>
              <td class="end">${this.canManage ? html`<lds-button size="sm" variant="ghost" data-remove-grant="${g.id}" data-name="${g.subject_name}">${t("grants.remove")}</lds-button>` : ""}</td>
            </tr>`)}</tbody>
        </table>
      </lds-table>`;
  }

  dialogs() {
    if (!this.canManage) return "";
    const { t } = this.context.i18n;
    return html`
      <lds-modal data-dialog="role" size="sm">
        <span slot="header" data-role-title></span>
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <lds-field label="${t("roles.name")}" required><lds-input name="name" required maxlength="40"></lds-input></lds-field>
          <lds-field label="${t("roles.copyFrom")}" data-copy>
            <lds-select name="copy" value="">
              <option value="">${t("roles.copyNone")}</option>
              ${this.roles.map((r) => html`<option value="${r.id}">${r.name}</option>`)}
            </lds-select>
          </lds-field>
        </form>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button data-action="save-role">${t("account.save")}</lds-button>
        </div>
      </lds-modal>
      <lds-modal data-dialog="grant" size="sm">
        <span slot="header">${t("grants.add")}</span>
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <lds-field label="${t("grants.subject")}" required>
            <lds-combobox name="subject" placeholder="${t("grants.subjectPlaceholder")}">
              ${this.members.map((m) => html`<lds-option value="user:${m.user_id}" keywords="${m.email}"><lds-avatar slot="start" size="sm" name="${m.name}"></lds-avatar>${m.name}</lds-option>`)}
              ${this.teams.map((team) => html`<lds-option value="team:${team.id}"><lds-icon slot="start" name="users"></lds-icon>${team.name}</lds-option>`)}
            </lds-combobox>
          </lds-field>
          <lds-field label="${t("grants.role")}" required>
            <lds-select name="role_id" value="${this.roles.find((r) => r.builtin === "contributor")?.id ?? ""}">
              ${this.roles.map((r) => html`<option value="${r.id}">${r.name}</option>`)}
            </lds-select>
          </lds-field>
          <p class="muted">${t("grants.workspaceOnly")}</p>
        </form>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button data-action="save-grant">${t("grants.give")}</lds-button>
        </div>
      </lds-modal>
      <lds-modal data-dialog="confirm" size="sm">
        <span slot="header" data-confirm-title></span>
        <p class="muted" data-confirm-body></p>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button variant="danger" data-action="confirm"></lds-button>
        </div>
      </lds-modal>`;
  }

  dialog(name) {
    return this.shadowRoot.querySelector(`[data-dialog="${name}"]`);
  }

  onToggle(e) {
    const box = e.target;
    if (box?.type !== "checkbox" || !box.dataset.perm) return;
    const role = this.roles.find((r) => r.id === box.dataset.role);
    const next = new Set(this.perms(role));
    if (box.checked) next.add(box.dataset.perm);
    else next.delete(box.dataset.perm);
    this.pending.set(role.id, next);
    const scroll = this.shadowRoot.querySelector(".content")?.scrollTop;
    this.render();
    this.shadowRoot.querySelector(`input[data-role="${role.id}"][data-perm="${box.dataset.perm}"]`)?.focus();
    if (scroll) this.shadowRoot.querySelector(".content").scrollTop = scroll;
  }

  onClick(e) {
    const target = e.target.closest?.("[data-action], [data-close], [data-remove-grant]");
    if (!target) return;
    if (target.hasAttribute("data-close")) {
      target.closest("lds-modal").removeAttribute("open");
      return;
    }
    if (target.dataset.removeGrant) {
      const { t } = this.context.i18n;
      this.confirm(t("grants.removeTitle", { name: target.dataset.name }), t("grants.removeBody"), t("grants.remove"),
        () => this.run(() => this.context.api.delete(`${this.base}/grants/${target.dataset.removeGrant}`)));
      return;
    }
    const action = target.dataset.action;
    if (action === "discard") {
      this.pending = new Map();
      this.render();
    } else if (action === "save") this.save();
    else if (action === "new-role") this.openRole(null);
    else if (action === "save-role") this.saveRole();
    else if (action === "new-grant") this.openDialog("grant");
    else if (action === "save-grant") this.saveGrant();
    else if (action === "confirm") this.pendingConfirm?.();
  }

  onMenu(e) {
    const menu = e.target.closest?.("lds-menu[data-role]");
    const value = e.detail?.value ?? e.target.getAttribute?.("value");
    if (!menu || !value) return;
    const role = this.roles.find((r) => r.id === menu.dataset.role);
    const { t } = this.context.i18n;
    if (value === "rename") this.openRole(role);
    else if (value === "delete") {
      this.confirm(t("roles.deleteTitle", { name: role.name }), t("roles.deleteBody"), t("roles.delete"),
        () => this.run(() => this.context.api.delete(`${this.base}/roles/${role.id}`)));
    }
  }

  /** Runs a change, reporting errors in a toast, then reloads. */
  async run(fn) {
    try {
      await fn();
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, this.context.i18n) });
    }
    await this.load();
  }

  confirm(title, body, action, fn) {
    const modal = this.dialog("confirm");
    modal.querySelector("[data-confirm-title]").textContent = title;
    modal.querySelector("[data-confirm-body]").textContent = body;
    modal.querySelector('[data-action="confirm"]').textContent = action;
    this.pendingConfirm = () => {
      modal.removeAttribute("open");
      fn();
    };
    modal.setAttribute("open", "");
  }

  openDialog(name) {
    const modal = this.dialog(name);
    const form = modal.querySelector("form");
    clearFieldErrors(form);
    form.querySelector("lds-alert").textContent = "";
    modal.setAttribute("open", "");
    return form;
  }

  async save() {
    const { api, i18n } = this.context;
    const button = this.shadowRoot.querySelector('[data-action="save"]');
    try {
      await busy(button, async () => {
        for (const role of this.roles) {
          const want = this.pending.get(role.id);
          if (!want) continue;
          await api.patch(`${this.base}/roles/${role.id}`, { body: { permissions: [...want] }, ifMatch: `"${role.version}"` });
        }
      });
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
      return;
    }
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("roles.saved") });
    this.load();
  }

  openRole(role) {
    const { t } = this.context.i18n;
    this.editing = role;
    const form = this.openDialog("role");
    this.dialog("role").querySelector("[data-role-title]").textContent = role ? t("roles.renameTitle", { name: role.name }) : t("roles.new");
    form.querySelector('[name="name"]').setAttribute("value", role?.name ?? "");
    form.querySelector("[data-copy]").hidden = !!role;
    form.querySelector('[name="copy"]').setAttribute("value", "");
  }

  async saveRole() {
    const { api, i18n } = this.context;
    const modal = this.dialog("role");
    const form = modal.querySelector("form");
    clearFieldErrors(form);
    const { name = "", copy = "" } = formValues(form);
    const source = this.roles.find((r) => r.id === copy);
    try {
      await busy(modal.querySelector('[data-action="save-role"]'), () => this.editing
        ? api.patch(`${this.base}/roles/${this.editing.id}`, { body: { name }, ifMatch: `"${this.editing.version}"` })
        : api.post(`${this.base}/roles`, { body: { name, permissions: source ? source.permissions : [] } }));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else form.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      return;
    }
    modal.removeAttribute("open");
    this.load();
  }

  async saveGrant() {
    const { api, i18n } = this.context;
    const modal = this.dialog("grant");
    const form = modal.querySelector("form");
    clearFieldErrors(form);
    const subject = form.querySelector('[name="subject"]').getAttribute("value") || "";
    const [kind, id] = subject.split(":");
    if (!id) {
      showFieldErrors(form, [{ field: "subject", code: "required" }], i18n.t, i18n.has);
      return;
    }
    const body = { role_id: formValues(form).role_id, [kind === "team" ? "team_id" : "user_id"]: id };
    try {
      await busy(modal.querySelector('[data-action="save-grant"]'), () => api.put(`${this.base}/grants`, { body }));
    } catch (err) {
      form.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      return;
    }
    modal.removeAttribute("open");
    this.load();
  }
}

customElements.define("hzn-settings-roles", HznSettingsRoles);
