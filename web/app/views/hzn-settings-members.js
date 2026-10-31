// <hzn-settings-members>: /w/:ws/settings/members[?view=teams] — members
// and pending invitations in one table, and the workspace's teams
// (wireframes: Members, views members / teams / invite). Managing needs
// workspace.members.manage; everyone else reads.
//
// Each member's and team's workspace role is shown, and editable with
// workspace.roles.manage; project grants are listed on the Roles page.

import { requestContext } from "horizon/core/context.js";
import { html, ldsLightSheet, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, formValues, showFieldErrors } from "horizon/core/forms.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

const styles = sheet(`
  .toolbar { display: flex; gap: var(--lds-space-3); align-items: center; margin-bottom: var(--lds-space-4); }
  .toolbar lds-input { width: 18rem; }
  .person { display: flex; gap: var(--lds-space-3); align-items: center; }
  .person small { display: block; color: var(--lds-color-fg-muted); }
  td.end { text-align: end; white-space: nowrap; width: 1%; }
  .note { margin: var(--lds-space-4) 0 0; }
  .teams { display: grid; grid-template-columns: repeat(auto-fill, minmax(18rem, 1fr)); gap: var(--lds-space-5); }
  .team-head { display: flex; align-items: center; justify-content: space-between; }
  .team-head h3 { margin: 0; font-size: var(--lds-font-size-md); }
  .team p { margin: var(--lds-space-2) 0 var(--lds-space-4); color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  .grant { margin: 0 0 var(--lds-space-3); font-size: var(--lds-font-size-sm); color: var(--lds-color-fg-muted); }
  .grant strong { color: var(--lds-color-fg-default); font-weight: var(--lds-font-weight-medium); }
  .people { display: flex; gap: var(--lds-space-3); align-items: center; font-size: var(--lds-font-size-sm); color: var(--lds-color-fg-muted); }
  lds-modal form { display: grid; gap: var(--lds-space-4); }
  .empty { padding: var(--lds-space-8) 0; }
`);

const STATUS_INTENT = { active: "success", invited: "info", suspended: "neutral" };

class HznSettingsMembers extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [ldsLightSheet(), settingsStyles, styles];
    this.filter = { text: "", status: "all" };
    root.addEventListener("click", (e) => this.onClick(e));
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
    return this.context.router.current?.query.get("view") === "teams" ? "teams" : "members";
  }

  async load() {
    const { api, i18n } = this.context;
    const slug = encodeURIComponent(this.getAttribute("ws"));
    try {
      const [ws, members, teams, roles, grants, projects] = await Promise.all([
        api.get(`/w/${slug}`), api.get(`/w/${slug}/members`), api.get(`/w/${slug}/teams`), api.get(`/w/${slug}/roles`),
        api.get(`/w/${slug}/grants`), api.get(`/w/${slug}/projects`),
      ]);
      const projectName = new Map(projects.data.items.map((p) => [p.id, p.name]));
      // Project grants by user or team: "Horizon: Facilitator".
      this.projectGrants = new Map();
      for (const g of grants.data.items) {
        if (!g.project_id) continue;
        const subject = g.user_id || g.team_id;
        const list = this.projectGrants.get(subject) ?? [];
        list.push(`${projectName.get(g.project_id) ?? "?"}: ${g.role_name}`);
        this.projectGrants.set(subject, list);
      }
      this.ws = ws.data;
      this.members = members.data.items;
      this.teams = teams.data.items;
      this.roles = roles.data.items;
      this.canManage = this.ws.can.includes("workspace.members.manage");
      this.canGrant = this.ws.can.includes("workspace.roles.manage");
      this.invitations = this.canManage ? (await api.get(`/w/${slug}/invitations`)).data.items : [];
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    this.render();
  }

  /** Members then invitations, as rows of one table. */
  rows() {
    const self = this.context.session.user?.id;
    const people = this.members.map((m) => ({ kind: "member", id: m.user_id, name: m.name, email: m.email, role: m.role,
      status: m.status, teams: m.teams.map((t) => t.name), last: m.last_seen_at, deactivated: m.deactivated, self: m.user_id === self }));
    const teamName = new Map(this.teams.map((t) => [t.id, t.name]));
    const roleName = new Map(this.roles.map((r) => [r.id, r.name]));
    const invited = this.invitations.map((i) => ({ kind: "invitation", id: i.id, name: "", email: i.email, status: "invited",
      role: i.role_id ? { id: i.role_id, name: roleName.get(i.role_id) } : null,
      teams: i.team_ids.map((id) => teamName.get(id)).filter(Boolean), sent: i.sent_at, expired: i.expired }));
    return [...people, ...invited];
  }

  render() {
    if (!this.ws) return;
    const { t } = this.context.i18n;
    const view = this.view;
    const actions = html`
      <lds-segmented size="sm" value="${view}" data-view label="${t("members.view")}">
        <lds-segment value="members">${t("members.tabMembers", { count: this.members.length })}</lds-segment>
        <lds-segment value="teams">${t("members.tabTeams", { count: this.teams.length })}</lds-segment>
      </lds-segmented>
      ${this.canManage ? (view === "teams"
        ? html`<lds-button data-action="new-team"><lds-icon slot="start" name="plus"></lds-icon>${t("teams.new")}</lds-button>`
        : html`<lds-button data-action="invite"><lds-icon slot="start" name="plus"></lds-icon>${t("members.invite")}</lds-button>`) : ""}`;
    this.shadowRoot.innerHTML = String(settingsPage(t, this.ws, "members",
      html`${view === "teams" ? this.teamsView() : this.membersView()}${this.dialogs()}`, actions));

    this.shadowRoot.querySelector("[data-view]").addEventListener("lds-change", (e) => {
      const q = e.detail.value === "teams" ? "?view=teams" : "";
      this.context.router.navigate(`/w/${this.ws.slug}/settings/members${q}`, { replace: true });
    });
    const filter = this.shadowRoot.querySelector("[data-filter]");
    filter?.addEventListener("lds-input", (e) => {
      this.filter.text = e.detail.value;
      this.renderRows();
    });
    this.shadowRoot.querySelector("[data-status]")?.addEventListener("lds-change", (e) => {
      this.filter.status = e.detail.value;
      this.renderRows();
    });
    if (view === "members") this.renderRows();
  }

  membersView() {
    const { t } = this.context.i18n;
    const rows = this.rows();
    const count = (s) => rows.filter((r) => r.status === s).length;
    return html`
      <div class="toolbar">
        <lds-input data-filter type="search" size="sm" placeholder="${t("members.filter")}" value="${this.filter.text}">
          <lds-icon slot="prefix" name="search"></lds-icon>
        </lds-input>
        <lds-segmented size="sm" data-status value="${this.filter.status}" label="${t("members.status")}">
          <lds-segment value="all">${t("members.all")}</lds-segment>
          <lds-segment value="active">${t("members.active", { count: count("active") })}</lds-segment>
          ${this.canManage ? html`<lds-segment value="invited">${t("members.invited", { count: count("invited") })}</lds-segment>` : ""}
          <lds-segment value="suspended">${t("members.suspended", { count: count("suspended") })}</lds-segment>
        </lds-segmented>
      </div>
      <lds-table density="compact">
        <table>
          <thead><tr>
            <th scope="col">${t("members.member")}</th>
            <th scope="col">${t("members.statusColumn")}</th>
            <th scope="col">${t("members.role")}</th>
            <th scope="col">${t("members.projectGrants")}</th>
            <th scope="col">${t("members.teams")}</th>
            <th scope="col">${t("members.lastActive")}</th>
            <th scope="col"><span class="lds-visually-hidden">${t("account.actions")}</span></th>
          </tr></thead>
          <tbody data-rows></tbody>
        </table>
      </lds-table>
      <p class="muted note">${t("members.note")}</p>`;
  }

  renderRows() {
    const { i18n } = this.context;
    const { t } = i18n;
    const text = this.filter.text.trim().toLowerCase();
    const rows = this.rows().filter((r) =>
      (this.filter.status === "all" || r.status === this.filter.status) &&
      (!text || r.name.toLowerCase().includes(text) || r.email.toLowerCase().includes(text)));
    const when = (v) => html`<span title="${i18n.date(v, { dateStyle: "medium", timeStyle: "short" })}">${i18n.relative(v)}</span>`;
    const body = this.shadowRoot.querySelector("[data-rows]");
    body.innerHTML = String(html`${rows.length ? rows.map((r) => html`
      <tr>
        <td><div class="person">
          <lds-avatar size="sm" name="${r.name || r.email}"></lds-avatar>
          <div>${r.name || r.email}${r.self ? html` <span class="muted">${t("members.you")}</span>` : ""}${r.name ? html`<small>${r.email}</small>` : ""}</div>
        </div></td>
        <td>
          <lds-badge size="sm" intent="${r.expired ? "warning" : STATUS_INTENT[r.status]}">${t(r.expired ? "members.expired" : `members.status.${r.status}`)}</lds-badge>
          ${r.deactivated ? html` <lds-badge size="sm" intent="danger">${t("members.deactivated")}</lds-badge>` : ""}
        </td>
        <td>${this.roleCell(r)}</td>
        <td>${this.projectGrants.get(r.id)?.join(", ") ?? "—"}</td>
        <td>${r.teams.length ? r.teams.join(", ") : "—"}</td>
        <td>${r.kind === "invitation" ? html`${t("members.invitedOn")} ${when(r.sent)}` : r.last ? when(r.last) : t("tokens.never")}</td>
        <td class="end">${this.canManage && !r.self ? this.rowMenu(r) : ""}</td>
      </tr>`) : html`<tr><td colspan="7" class="muted">${t("members.noMatch")}</td></tr>`}`);
    for (const select of body.querySelectorAll("lds-select[data-grant-user]")) {
      select.addEventListener("lds-change", (e) => this.setRole({ user_id: select.dataset.grantUser }, select.dataset.grantId, e.detail.value));
    }
  }

  /** A member's workspace role: a select for those who may grant. */
  roleCell(r) {
    const { t } = this.context.i18n;
    if (r.kind !== "member" || !this.canGrant) return r.role?.name ?? html`<span class="muted">${t("members.noRole")}</span>`;
    return html`<lds-select size="sm" data-grant-user="${r.id}" data-grant-id="${r.role?.grant_id ?? ""}" value="${r.role?.id ?? ""}"
        label="${t("members.roleFor", { name: r.name })}">
      <option value="">${t("members.noRole")}</option>
      ${this.roles.map((role) => html`<option value="${role.id}">${role.name}</option>`)}
    </lds-select>`;
  }

  /** Sets (or, with no role, removes) a workspace-scope grant. */
  async setRole(subject, grantId, roleId) {
    const { api, i18n } = this.context;
    const base = `/w/${encodeURIComponent(this.ws.slug)}/grants`;
    try {
      if (roleId) await api.put(base, { body: { role_id: roleId, ...subject } });
      else if (grantId) await api.delete(`${base}/${grantId}`);
      globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("members.roleSaved") });
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
    this.load();
  }

  rowMenu(r) {
    const { t } = this.context.i18n;
    const items = r.kind === "invitation"
      ? [["resend", "members.resend"], ["revoke", "members.revoke"]]
      : [[r.status === "suspended" ? "reactivate" : "suspend", r.status === "suspended" ? "members.reactivate" : "members.suspendAction"], ["remove", "members.remove"]];
    return html`
      <lds-menu placement="bottom-end" data-kind="${r.kind}" data-id="${r.id}" data-label="${r.name || r.email}">
        <lds-button slot="trigger" size="sm" variant="ghost" label="${t("members.actionsFor", { name: r.name || r.email })}">
          <lds-icon name="more-horizontal"></lds-icon>
        </lds-button>
        ${items.map(([value, label]) => html`<lds-menu-item value="${value}">${t(label)}</lds-menu-item>`)}
      </lds-menu>`;
  }

  teamsView() {
    const { t } = this.context.i18n;
    if (!this.teams.length) {
      return html`<lds-empty-state class="empty"><lds-icon slot="icon" name="users"></lds-icon>
        <h2 slot="title">${t("teams.none")}</h2>${t("teams.noneBody")}</lds-empty-state>`;
    }
    return html`<div class="teams">${this.teams.map((team) => html`
      <lds-card class="team">
        <div class="team-head">
          <h3>${team.name}</h3>
          ${this.canManage ? html`
            <lds-menu placement="bottom-end" data-kind="team" data-id="${team.id}" data-label="${team.name}">
              <lds-button slot="trigger" size="sm" variant="ghost" label="${t("members.actionsFor", { name: team.name })}"><lds-icon name="more-horizontal"></lds-icon></lds-button>
              <lds-menu-item value="edit">${t("teams.edit")}</lds-menu-item>
              <lds-menu-item value="delete">${t("teams.delete")}</lds-menu-item>
            </lds-menu>` : ""}
        </div>
        <p>${team.description || t("teams.noDescription")}</p>
        <p class="grant">${t("teams.workspaceRole")}: <strong>${team.role?.name ?? t("members.noRole")}</strong></p>
        ${this.projectGrants.get(team.id) ? html`<p class="grant">${t("members.projectGrants")}: <strong>${this.projectGrants.get(team.id).join(", ")}</strong></p>` : ""}
        <div class="people">
          ${team.members.length ? html`<lds-avatar-group size="sm" max="5">${team.members.map((m) => html`<lds-avatar size="sm" name="${m.name}" title="${m.name}"></lds-avatar>`)}</lds-avatar-group>` : ""}
          ${t("teams.people", { count: team.members.length })}
        </div>
      </lds-card>`)}</div>`;
  }

  dialogs() {
    const { t } = this.context.i18n;
    const teamOptions = this.teams.map((team) => html`<lds-option value="${team.id}">${team.name}</lds-option>`);
    return html`
      <lds-modal data-dialog="invite" size="md">
        <span slot="header">${t("invite.title", { workspace: this.ws.name })}</span>
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <lds-field label="${t("invite.emails")}" required>
            <lds-combobox name="emails" multiple creatable placeholder="${t("invite.emailsPlaceholder")}"></lds-combobox>
            <span slot="help">${t("invite.help")}</span>
          </lds-field>
          ${this.canGrant ? html`<lds-field label="${t("invite.role")}">
            <lds-select name="role_id" value="${this.roles.find((r) => r.builtin === "contributor")?.id ?? ""}">
              ${this.roles.map((r) => html`<option value="${r.id}">${r.name}</option>`)}
            </lds-select>
          </lds-field>` : html`<p class="muted">${t("invite.defaultRole")}</p>`}
          ${this.teams.length ? html`<lds-field label="${t("invite.teams")}">
            <lds-combobox name="team_ids" multiple placeholder="${t("invite.teamsPlaceholder")}">${teamOptions}</lds-combobox>
          </lds-field>` : ""}
        </form>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button data-action="send-invites">${t("invite.send")}</lds-button>
        </div>
      </lds-modal>
      <lds-modal data-dialog="team" size="md">
        <span slot="header" data-team-title></span>
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <lds-field label="${t("teams.name")}" required><lds-input name="name" required maxlength="60"></lds-input></lds-field>
          <lds-field label="${t("teams.description")}"><lds-textarea name="description" maxlength="300" rows="2"></lds-textarea></lds-field>
          ${this.canGrant ? html`<lds-field label="${t("teams.workspaceRole")}">
            <lds-select name="role_id" value="">
              <option value="">${t("members.noRole")}</option>
              ${this.roles.map((r) => html`<option value="${r.id}">${r.name}</option>`)}
            </lds-select>
          </lds-field>` : ""}
          <lds-field label="${t("teams.members")}">
            <lds-combobox name="members" multiple placeholder="${t("teams.membersPlaceholder")}">
              ${this.members.map((m) => html`<lds-option value="${m.user_id}" keywords="${m.email}">${m.name}</lds-option>`)}
            </lds-combobox>
          </lds-field>
        </form>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button data-action="save-team">${t("account.save")}</lds-button>
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

  onClick(e) {
    const target = e.target.closest?.("[data-action], [data-close]");
    if (!target) return;
    if (target.hasAttribute("data-close")) {
      target.closest("lds-modal").removeAttribute("open");
      return;
    }
    const action = target.dataset.action;
    if (action === "invite") this.openInvite();
    else if (action === "send-invites") this.sendInvites();
    else if (action === "new-team") this.openTeam(null);
    else if (action === "save-team") this.saveTeam();
    else if (action === "confirm") this.pendingConfirm?.();
  }

  onMenu(e) {
    const menu = e.target.closest?.("lds-menu[data-kind]");
    const value = e.detail?.value ?? e.target.getAttribute?.("value");
    if (!menu || !value) return;
    const { kind, id, label } = menu.dataset;
    const { t } = this.context.i18n;
    const base = `/w/${encodeURIComponent(this.ws.slug)}`;
    const api = this.context.api;
    const run = (fn, done) => fn().then(() => {
      if (done) globalThis.LDS?.toast?.({ intent: "success", title: done });
      return this.load();
    }).catch((err) => globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, this.context.i18n) }));

    switch (`${kind}:${value}`) {
      case "member:suspend":
        return this.confirm(t("members.suspendTitle", { name: label }), t("members.suspendBody"), t("members.suspendAction"),
          () => run(() => api.patch(`${base}/members/${id}`, { body: { status: "suspended" } })));
      case "member:reactivate":
        return run(() => api.patch(`${base}/members/${id}`, { body: { status: "active" } }), t("members.reactivated", { name: label }));
      case "member:remove":
        return this.confirm(t("members.removeTitle", { name: label }), t("members.removeBody"), t("members.remove"),
          () => run(() => api.delete(`${base}/members/${id}`)));
      case "invitation:resend":
        return run(() => api.post(`${base}/invitations/${id}/resend`), t("members.resent", { email: label }));
      case "invitation:revoke":
        return this.confirm(t("members.revokeTitle", { email: label }), t("members.revokeBody"), t("members.revoke"),
          () => run(() => api.delete(`${base}/invitations/${id}`)));
      case "team:edit":
        return this.openTeam(this.teams.find((team) => team.id === id));
      case "team:delete":
        return this.confirm(t("teams.deleteTitle", { name: label }), t("teams.deleteBody"), t("teams.delete"),
          () => run(() => api.delete(`${base}/teams/${id}`)));
    }
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

  openInvite() {
    const modal = this.dialog("invite");
    const form = modal.querySelector("form");
    form.reset();
    clearFieldErrors(form);
    for (const c of form.querySelectorAll("lds-combobox")) c.setAttribute("value", "");
    form.querySelector("lds-alert").textContent = "";
    modal.setAttribute("open", "");
  }

  async sendInvites() {
    const { api, i18n } = this.context;
    const modal = this.dialog("invite");
    const form = modal.querySelector("form");
    const alert = form.querySelector("lds-alert");
    alert.textContent = "";
    clearFieldErrors(form);
    const emailsBox = form.querySelector('[name="emails"]');
    const split = (v) => (v || "").split(",").map((s) => s.trim()).filter(Boolean);
    // Text typed but not yet turned into a chip counts too.
    const pending = emailsBox.shadowRoot?.querySelector("input")?.value.trim();
    const emails = [...split(emailsBox.getAttribute("value")), ...(pending ? [pending] : [])];
    const teamIds = split(form.querySelector('[name="team_ids"]')?.getAttribute("value"));
    const roleId = form.querySelector('[name="role_id"]')?.getAttribute("value") || undefined;
    try {
      await busy(modal.querySelector('[data-action="send-invites"]'), () =>
        api.post(`/w/${encodeURIComponent(this.ws.slug)}/invitations`, { body: { emails, team_ids: teamIds, role_id: roleId } }));
    } catch (err) {
      const fields = (err.fields || []).map((f) => {
        const m = /^emails\[(\d+)\]$/.exec(f.field);
        return m ? { field: "emails", code: f.code, email: emails[Number(m[1])] } : f;
      });
      if (fields.length) {
        showFieldErrors(form, fields, i18n.t, i18n.has);
        const bad = fields.filter((f) => f.email).map((f) => f.email);
        if (bad.length) alert.textContent = i18n.t("invite.badEmails", { emails: bad.join(", ") });
      } else {
        alert.textContent = errorMessage(err, i18n);
      }
      return;
    }
    modal.removeAttribute("open");
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("invite.sent", { count: emails.length }) });
    this.load();
  }

  openTeam(team) {
    const { t } = this.context.i18n;
    this.editing = team;
    const modal = this.dialog("team");
    const form = modal.querySelector("form");
    clearFieldErrors(form);
    form.querySelector("lds-alert").textContent = "";
    modal.querySelector("[data-team-title]").textContent = team ? t("teams.editTitle", { name: team.name }) : t("teams.new");
    form.querySelector('[name="name"]').setAttribute("value", team?.name ?? "");
    form.querySelector('[name="description"]').setAttribute("value", team?.description ?? "");
    form.querySelector('[name="members"]').setAttribute("value", (team?.members ?? []).map((m) => m.user_id).join(","));
    form.querySelector('[name="role_id"]')?.setAttribute("value", team?.role?.id ?? "");
    modal.setAttribute("open", "");
  }

  async saveTeam() {
    const { api, i18n } = this.context;
    const modal = this.dialog("team");
    const form = modal.querySelector("form");
    const alert = form.querySelector("lds-alert");
    alert.textContent = "";
    clearFieldErrors(form);
    const { name = "", description = "" } = formValues(form);
    const wanted = new Set((form.querySelector('[name="members"]').getAttribute("value") || "").split(",").filter(Boolean));
    const base = `/w/${encodeURIComponent(this.ws.slug)}/teams`;
    try {
      await busy(modal.querySelector('[data-action="save-team"]'), async () => {
        let team = this.editing;
        if (team) {
          await api.patch(`${base}/${team.id}`, { body: { name, description }, ifMatch: `"${team.version}"` });
        } else {
          ({ data: team } = await api.post(base, { body: { name, description } }));
        }
        const current = new Set(team.members.map((m) => m.user_id));
        for (const id of wanted) if (!current.has(id)) await api.put(`${base}/${team.id}/members/${id}`);
        for (const id of current) if (!wanted.has(id)) await api.delete(`${base}/${team.id}/members/${id}`);
        const role = form.querySelector('[name="role_id"]');
        if (role && (role.getAttribute("value") || "") !== (team.role?.id ?? "")) {
          const grants = `/w/${encodeURIComponent(this.ws.slug)}/grants`;
          if (role.getAttribute("value")) await api.put(grants, { body: { role_id: role.getAttribute("value"), team_id: team.id } });
          else await api.delete(`${grants}/${team.role.grant_id}`);
        }
      });
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else alert.textContent = errorMessage(err, i18n);
      if (!this.editing && !err.fields?.length) this.load();
      return;
    }
    modal.removeAttribute("open");
    this.load();
  }
}

customElements.define("hzn-settings-members", HznSettingsMembers);
