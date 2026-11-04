// <hzn-settings-git>: /w/:ws/settings/git — Git forge connections
// (workspace.integrations.manage) and what a merged pull request does to
// its issues (docs/integrations.md). A new connection shows the webhook
// URL and secret to set on the forge, once.

import { requestContext } from "horizon/core/context.js";
import { html, ldsLightSheet, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, showFieldErrors } from "horizon/core/forms.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

const PROVIDERS = ["github", "gitlab", "forgejo"];
const BASES = { github: "https://github.com", gitlab: "https://gitlab.com", forgejo: "" };

const styles = sheet(`
  section + section { margin-block-start: var(--lds-space-8); }
  h3 { font-size: var(--lds-font-size-md); margin: 0 0 var(--lds-space-2); }
  .lead { margin: 0 0 var(--lds-space-4); color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); max-inline-size: 44rem; }
  .merge { max-inline-size: 24rem; }
  .url { font-family: var(--lds-font-family-mono); font-size: var(--lds-font-size-xs); overflow-wrap: anywhere; }
  lds-modal form { display: grid; gap: var(--lds-space-4); }
  .setup { display: grid; gap: var(--lds-space-3); }
  .setup code { display: block; font-family: var(--lds-font-family-mono); padding: var(--lds-space-2) var(--lds-space-3);
    background: var(--lds-color-bg-subtle); border-radius: var(--lds-radius-sm); overflow-wrap: anywhere; user-select: all; }
  .setup ol { margin: 0; padding-inline-start: var(--lds-space-5); display: grid; gap: var(--lds-space-1); }
  td.actions { text-align: end; white-space: nowrap; }
`);

class HznSettingsGit extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [ldsLightSheet(), settingsStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("lds-change", (e) => {
      if (e.target.matches?.("[data-merge]")) this.saveMerge(e.detail.value);
      if (e.target.matches?.('[name="provider"]')) {
        const base = this.shadowRoot.querySelector('[data-dialog="edit"] [name="base_url"]');
        if (!base.getAttribute("value") || Object.values(BASES).includes(base.getAttribute("value"))) base.setAttribute("value", BASES[e.detail.value] ?? "");
      }
    });
    this.load();
  }

  attributeChangedCallback(_, before, after) {
    if (this.shadowRoot && before && before !== after) this.load();
  }

  get slug() {
    return encodeURIComponent(this.getAttribute("ws"));
  }

  async load() {
    const { api, i18n } = this.context;
    try {
      const [ws, conns, settings, workflows] = await Promise.all([
        api.get(`/w/${this.slug}`), api.get(`/w/${this.slug}/integrations/vcs`),
        api.get(`/w/${this.slug}/config/settings/git`), api.get(`/w/${this.slug}/config/workflows`),
      ]);
      this.ws = ws.data;
      this.connections = conns.data.items;
      this.settings = settings.data;
      this.statuses = [...new Set(workflows.data.items.flatMap((w) => w.statuses.map((s) => s.name)))];
    } catch (err) {
      const msg = html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`;
      this.shadowRoot.innerHTML = String(this.ws ? settingsPage(i18n.t, this.ws, "git", msg) : msg);
      return;
    }
    this.render();
  }

  render() {
    const { i18n } = this.context;
    const { t } = i18n;
    const merge = this.settings.merge_status.workspace;
    const body = html`
      <section>
        <h3>${t("git.merge")}</h3>
        <p class="lead">${t("git.mergeHint")}</p>
        <lds-select class="merge" data-merge value="${merge}" label="${t("git.merge")}" ${this.settings.can_manage ? "" : html`disabled`}>
          <option value="@completed">${t("git.mergeCompleted")}</option>
          ${this.statuses.map((s) => html`<option value="${s}">${t("git.mergeTo", { status: s })}</option>`)}
          <option value="@off">${t("git.mergeOff")}</option>
        </lds-select>
      </section>
      <section>
        <h3>${t("git.connections")}</h3>
        <p class="lead">${t("git.connectionsHint")}</p>
        ${this.connections.length ? html`
          <lds-table density="compact">
            <table>
              <thead><tr><th scope="col">${t("git.name")}</th><th scope="col">${t("git.provider")}</th><th scope="col">${t("git.baseURL")}</th>
                <th scope="col">${t("git.comments")}</th><th scope="col">${t("git.lastEvent")}</th><th scope="col"></th></tr></thead>
              <tbody>${this.connections.map((c) => html`
                <tr data-id="${c.id}">
                  <td>${c.name}</td><td>${t(`git.providers.${c.provider}`)}</td><td><span class="url">${c.base_url}</span></td>
                  <td>${c.has_token ? t("git.commentsOn") : t("git.commentsOff")}</td>
                  <td>${c.last_event_at ? i18n.relative(c.last_event_at) : t("git.never")}</td>
                  <td class="actions"><lds-button size="sm" variant="ghost" data-action="edit" label="${t("git.editNamed", { name: c.name })}">${t("issues.edit")}</lds-button></td>
                </tr>`)}</tbody>
            </table>
          </lds-table>`
          : html`<lds-empty-state><lds-icon slot="icon" name="git-pull-request"></lds-icon><h2 slot="title">${t("git.none")}</h2>${t("git.noneHint")}</lds-empty-state>`}
      </section>
      ${this.dialog()}${this.setupDialog()}`;
    const actions = html`<lds-button data-action="new"><lds-icon slot="start" name="plus"></lds-icon>${t("git.new")}</lds-button>`;
    this.shadowRoot.innerHTML = String(settingsPage(t, this.ws, "git", body, actions));
    if (this.setup) {
      this.showSetup(this.setup);
      this.setup = null;
    }
  }

  dialog() {
    const { t } = this.context.i18n;
    const c = this.editing;
    return html`
      <lds-modal data-dialog="edit" size="md">
        <span slot="header">${c ? t("git.edit") : t("git.new")}</span>
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <lds-field label="${t("git.provider")}">
            <lds-select name="provider" value="${c?.provider ?? "github"}" ${c ? html`disabled` : ""}>
              ${PROVIDERS.map((p) => html`<option value="${p}">${t(`git.providers.${p}`)}</option>`)}
            </lds-select>
          </lds-field>
          <lds-field label="${t("git.name")}" required><lds-input name="name" required maxlength="100" value="${c?.name ?? ""}"></lds-input></lds-field>
          <lds-field label="${t("git.baseURL")}" required hint="${t("git.baseURLHint")}"><lds-input name="base_url" type="url" required value="${c?.base_url ?? BASES.github}"></lds-input></lds-field>
          <lds-field label="${t("git.token")}" hint="${t(c?.has_token ? "git.tokenReplaceHint" : "git.tokenHint")}">
            <lds-input name="token" type="password" placeholder="${c?.has_token ? "••••••••" : ""}"></lds-input></lds-field>
          ${c?.has_token ? html`<lds-checkbox name="remove_token">${t("git.removeToken")}</lds-checkbox>` : ""}
        </form>
        <div slot="footer" class="actions">
          ${c ? html`<lds-button variant="ghost" data-action="delete">${t("git.delete")}</lds-button>
            <lds-button variant="ghost" data-action="rotate">${t("git.rotate")}</lds-button>` : ""}
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button data-action="save">${c ? t("git.save") : t("git.create")}</lds-button>
        </div>
      </lds-modal>`;
  }

  setupDialog() {
    const { t } = this.context.i18n;
    return html`
      <lds-modal data-dialog="setup" size="md">
        <span slot="header">${t("git.setupTitle")}</span>
        <div class="setup">
          <p>${t("git.setupLead")}</p>
          <div><strong>${t("git.webhookURL")}</strong><code data-url></code></div>
          <div><strong>${t("git.webhookSecret")}</strong><code data-secret></code></div>
          <ol data-steps></ol>
          <p class="muted">${t("git.secretOnce")}</p>
        </div>
        <div slot="footer" class="actions"><lds-button data-close>${t("webhooks.done")}</lds-button></div>
      </lds-modal>`;
  }

  /** Fills and opens the setup dialog for a connection with its secret. */
  showSetup(c) {
    const { t } = this.context.i18n;
    const d = this.shadowRoot.querySelector('[data-dialog="setup"]');
    d.querySelector("[data-url]").textContent = c.webhook_url;
    d.querySelector("[data-secret]").textContent = c.webhook_secret;
    d.querySelector("[data-steps]").innerHTML = String(html`${[1, 2, 3].map((n) => html`<li>${t(`git.steps.${c.provider}.${n}`)}</li>`)}`);
    d.setAttribute("open", "");
  }

  async saveMerge(value) {
    const { api, i18n } = this.context;
    try {
      const { data } = await api.put(`/w/${this.slug}/config/settings/git`, { body: { merge_status: value } });
      this.settings = { ...this.settings, ...data };
      globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("git.saved") });
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
  }

  async onClick(e) {
    const target = e.target.closest?.("[data-action], [data-close]");
    if (!target) return;
    if (target.hasAttribute("data-close")) {
      target.closest("lds-modal").removeAttribute("open");
      return;
    }
    const { api, i18n } = this.context;
    switch (target.dataset.action) {
      case "new":
      case "edit":
        this.editing = target.dataset.action === "edit" ? this.connections.find((c) => c.id === target.closest("tr").dataset.id) : null;
        this.render();
        this.shadowRoot.querySelector('[data-dialog="edit"]').setAttribute("open", "");
        return;
      case "save":
        return this.save(target);
      case "rotate":
        return this.save(target, { rotate_secret: true });
      case "delete":
        if (!confirm(i18n.t("git.deleteConfirm", { name: this.editing.name }))) return;
        try {
          await api.delete(`/w/${this.slug}/integrations/vcs/${this.editing.id}`);
        } catch (err) {
          globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
          return;
        }
        this.editing = null;
        return this.load();
    }
  }

  async save(button, extra = {}) {
    const { api, i18n } = this.context;
    const modal = this.shadowRoot.querySelector('[data-dialog="edit"]');
    const form = modal.querySelector("form");
    clearFieldErrors(form);
    form.querySelector("lds-alert").textContent = "";
    const fd = new FormData(form);
    const body = { name: String(fd.get("name") ?? "").trim(), base_url: String(fd.get("base_url") ?? "").trim(), ...extra };
    const token = String(fd.get("token") ?? "").trim();
    if (token) body.token = token;
    else if (form.querySelector('[name="remove_token"]')?.hasAttribute("checked")) body.token = "";
    let saved;
    try {
      ({ data: saved } = await busy(button, () => (this.editing
        ? api.patch(`/w/${this.slug}/integrations/vcs/${this.editing.id}`, { body, ifMatch: `"${this.editing.version}"` })
        : api.post(`/w/${this.slug}/integrations/vcs`, { body: { ...body, provider: String(fd.get("provider") ?? "github") } }))));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else form.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      return;
    }
    modal.removeAttribute("open");
    this.editing = null;
    if (saved.webhook_secret) this.setup = saved; // shown once, after the reload
    await this.load();
  }
}

customElements.define("hzn-settings-git", HznSettingsGit);
