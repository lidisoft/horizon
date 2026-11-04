// <hzn-settings-webhooks>: /w/:ws/settings/webhooks[/:id] — the
// workspace's webhooks (workspace.webhooks.manage): create and edit them,
// see the signing secret once, and, for one, its delivery log with a test
// ping and redelivery (docs/api.md#webhooks).

import { requestContext } from "horizon/core/context.js";
import { html, ldsLightSheet, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, showFieldErrors } from "horizon/core/forms.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

// Event filters offered; an empty choice means every event.
export const EVENT_TYPES = ["issue.*", "issue.created", "issue.updated", "issue.status_updated", "issue.assigned", "issue.commented",
  "issue.deleted", "iteration.*", "release.*", "milestone.*", "project.*", "workspace_member.*"];

const STATE_INTENTS = { succeeded: "success", failed: "danger", pending: "warning" };

const styles = sheet(`
  tbody tr[data-id] { cursor: pointer; }
  a.name { color: inherit; text-decoration: none; font-weight: var(--lds-font-weight-medium); }
  .url { font-family: var(--lds-font-family-mono); font-size: var(--lds-font-size-xs); overflow-wrap: anywhere; }
  lds-modal form { display: grid; gap: var(--lds-space-4); }
  .secret { display: grid; gap: var(--lds-space-3); }
  .secret code { font-family: var(--lds-font-family-mono); padding: var(--lds-space-3); background: var(--lds-color-bg-subtle);
    border-radius: var(--lds-radius-sm); overflow-wrap: anywhere; user-select: all; }
  .summary { display: grid; grid-template-columns: max-content 1fr; gap: var(--lds-space-2) var(--lds-space-5); margin: 0 0 var(--lds-space-6); }
  .summary dt { color: var(--lds-color-fg-muted); }
  .summary dd { margin: 0; }
  h3 { font-size: var(--lds-font-size-md); margin: 0 0 var(--lds-space-3); }
  details pre { white-space: pre-wrap; overflow-wrap: anywhere; font-size: var(--lds-font-size-xs); margin: var(--lds-space-2) 0 0;
    padding: var(--lds-space-3); background: var(--lds-color-bg-subtle); border-radius: var(--lds-radius-sm); }
  .error { color: var(--lds-color-danger-fg); font-size: var(--lds-font-size-sm); white-space: normal; }
  td.nowrap { white-space: nowrap; }
`);

class HznSettingsWebhooks extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [ldsLightSheet(), settingsStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    // The list and one webhook share this view: follow the route (the
    // shell keeps a stale id attribute when going back to the list).
    this.unroute = this.context.router.subscribe((m) => {
      if (m.route?.view === this.localName) this.load();
    });
    this.load();
  }

  disconnectedCallback() {
    this.unroute?.();
  }

  get base() {
    return `/w/${encodeURIComponent(this.getAttribute("ws"))}/webhooks`;
  }

  async load() {
    const { api, i18n } = this.context;
    const slug = encodeURIComponent(this.getAttribute("ws"));
    const id = this.context.router.current?.params.id;
    const seq = (this.seq = (this.seq ?? 0) + 1); // a later load wins
    try {
      const [ws, projects, hooks, deliveries] = await Promise.all([
        api.get(`/w/${slug}`), api.get(`/w/${slug}/projects`), api.get(this.base),
        id ? api.get(`${this.base}/${encodeURIComponent(id)}/deliveries`) : null,
      ]);
      if (seq !== this.seq) return;
      this.ws = ws.data;
      this.projects = projects.data.items.filter((p) => !p.archived_at);
      this.hooks = hooks.data.items;
      this.deliveries = deliveries?.data.items ?? [];
      this.next = deliveries?.data.next_cursor;
    } catch (err) {
      if (seq !== this.seq) return;
      const page = this.ws ? settingsPage(i18n.t, this.ws, "webhooks", html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`) : html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`;
      this.shadowRoot.innerHTML = String(page);
      return;
    }
    this.hook = id ? this.hooks.find((h) => h.id === id) : null;
    this.render();
  }

  render() {
    const { t } = this.context.i18n;
    const body = this.hook ? this.detail() : this.list();
    const actions = this.hook
      ? html`<lds-button variant="secondary" data-action="ping"><lds-icon slot="start" name="rocket"></lds-icon>${t("webhooks.ping")}</lds-button>
          <lds-button variant="secondary" data-action="edit">${t("webhooks.edit")}</lds-button>`
      : html`<lds-button data-action="new"><lds-icon slot="start" name="plus"></lds-icon>${t("webhooks.new")}</lds-button>`;
    this.shadowRoot.innerHTML = String(settingsPage(t, this.ws, "webhooks", html`${body}${this.dialog()}${this.secretDialog()}`, actions, this.hook?.name ?? ""));
    if (this.secret) {
      const dialog = this.shadowRoot.querySelector('[data-dialog="secret"]');
      dialog.querySelector("[data-secret]").textContent = this.secret;
      dialog.setAttribute("open", "");
      this.secret = "";
    }
  }

  scope(h) {
    const { t } = this.context.i18n;
    return h.project ? this.projects.find((p) => p.key === h.project)?.name ?? h.project : t("webhooks.allProjects");
  }

  events(h) {
    return h.event_types.length ? h.event_types.join(", ") : this.context.i18n.t("webhooks.allEvents");
  }

  list() {
    const { t } = this.context.i18n;
    if (!this.hooks.length) {
      return html`<lds-empty-state><lds-icon slot="icon" name="external-link"></lds-icon><h2 slot="title">${t("webhooks.none")}</h2>${t("webhooks.noneHint")}</lds-empty-state>`;
    }
    return html`
      <lds-table density="compact">
        <table>
          <thead><tr>
            <th scope="col">${t("webhooks.name")}</th><th scope="col">${t("webhooks.url")}</th>
            <th scope="col">${t("webhooks.scope")}</th><th scope="col">${t("webhooks.events")}</th><th scope="col">${t("webhooks.status")}</th>
          </tr></thead>
          <tbody>${this.hooks.map((h) => html`
            <tr data-id="${h.id}">
              <td><a class="name" href="/w/${this.ws.slug}/settings/webhooks/${h.id}">${h.name}</a></td>
              <td><span class="url">${h.url}</span></td>
              <td>${this.scope(h)}</td>
              <td>${this.events(h)}</td>
              <td>${h.active ? html`<lds-badge size="sm" intent="success">${t("webhooks.active")}</lds-badge>` : html`<lds-badge size="sm">${t("webhooks.inactive")}</lds-badge>`}</td>
            </tr>`)}</tbody>
        </table>
      </lds-table>`;
  }

  detail() {
    const { i18n } = this.context;
    const { t } = i18n;
    const h = this.hook;
    return html`
      <dl class="summary">
        <dt>${t("webhooks.url")}</dt><dd class="url">${h.url}</dd>
        <dt>${t("webhooks.scope")}</dt><dd>${this.scope(h)}</dd>
        <dt>${t("webhooks.events")}</dt><dd>${this.events(h)}</dd>
        <dt>${t("webhooks.status")}</dt><dd>${h.active ? t("webhooks.active") : t("webhooks.inactive")}</dd>
      </dl>
      <h3>${t("webhooks.deliveries")}</h3>
      ${this.deliveries.length ? html`
        <lds-table density="compact">
          <table>
            <thead><tr>
              <th scope="col">${t("webhooks.event")}</th><th scope="col">${t("webhooks.state")}</th><th scope="col">${t("webhooks.response")}</th>
              <th scope="col">${t("webhooks.when")}</th><th scope="col"></th>
            </tr></thead>
            <tbody>${this.deliveries.map((d) => html`
              <tr data-delivery="${d.id}">
                <td><details><summary>${d.event_type}${d.redelivery_of ? html` <span class="muted">${t("webhooks.redelivery")}</span>` : ""}</summary>
                  <pre>${JSON.stringify(d.body, null, 2)}</pre>${d.response_excerpt ? html`<pre>${d.response_excerpt}</pre>` : ""}</details></td>
                <td><lds-badge size="sm" intent="${STATE_INTENTS[d.state]}">${t(`webhooks.states.${d.state}`)}</lds-badge>
                  ${d.attempts > 1 ? html` <span class="muted">${t("webhooks.attempts", { count: d.attempts })}</span>` : ""}
                  ${d.state === "pending" && d.next_attempt_at ? html`<div class="muted">${t("webhooks.retry", { when: i18n.relative(d.next_attempt_at) })}</div>` : ""}</td>
                <td class="nowrap">${d.status_code ?? ""}${d.duration_ms != null ? html` <span class="muted">${t("webhooks.ms", { ms: d.duration_ms })}</span>` : ""}
                  ${d.error ? html`<div class="error">${d.error}</div>` : ""}</td>
                <td class="nowrap">${i18n.relative(d.created_at)}</td>
                <td><lds-button size="sm" variant="ghost" data-action="redeliver">${t("webhooks.redeliver")}</lds-button></td>
              </tr>`)}</tbody>
          </table>
        </lds-table>
        ${this.next ? html`<p><lds-button size="sm" variant="secondary" data-action="more">${t("inbox.more")}</lds-button></p>` : ""}`
        : html`<p class="muted">${t("webhooks.noDeliveries")}</p>`}`;
  }

  dialog() {
    const { t } = this.context.i18n;
    const h = this.hook;
    return html`
      <lds-modal data-dialog="edit" size="md">
        <span slot="header">${h ? t("webhooks.edit") : t("webhooks.new")}</span>
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <lds-field label="${t("webhooks.name")}" required><lds-input name="name" required maxlength="100" value="${h?.name ?? ""}"></lds-input></lds-field>
          <lds-field label="${t("webhooks.url")}" required hint="${t("webhooks.urlHint")}"><lds-input name="url" type="url" required value="${h?.url ?? ""}" placeholder="https://"></lds-input></lds-field>
          <lds-field label="${t("webhooks.scope")}">
            <lds-select name="project" value="${h?.project ?? ""}">
              <option value="">${t("webhooks.allProjects")}</option>
              ${this.projects.map((p) => html`<option value="${p.key}">${p.name} (${p.key})</option>`)}
            </lds-select>
          </lds-field>
          <lds-field label="${t("webhooks.events")}" hint="${t("webhooks.eventsHint")}">
            <lds-combobox name="event_types" multiple value="${(h?.event_types ?? []).join(",")}" placeholder="${t("webhooks.allEvents")}" label="${t("webhooks.events")}">
              ${EVENT_TYPES.map((e) => html`<lds-option value="${e}">${e}</lds-option>`)}
            </lds-combobox>
          </lds-field>
          <lds-switch name="active" ${h && !h.active ? "" : html`checked`}>${t("webhooks.active")}</lds-switch>
        </form>
        <div slot="footer" class="actions">
          ${h ? html`<lds-button variant="ghost" data-action="delete">${t("webhooks.delete")}</lds-button>
            <lds-button variant="ghost" data-action="rotate">${t("webhooks.rotate")}</lds-button>` : ""}
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button data-action="save">${h ? t("webhooks.save") : t("webhooks.create")}</lds-button>
        </div>
      </lds-modal>`;
  }

  secretDialog() {
    const { t } = this.context.i18n;
    return html`
      <lds-modal data-dialog="secret" size="md">
        <span slot="header">${t("webhooks.secretTitle")}</span>
        <div class="secret">
          <p>${t("webhooks.secretHint")}</p>
          <code data-secret></code>
          <p class="muted">${t("webhooks.signatureHint")}</p>
        </div>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-action="copy-secret"><lds-icon slot="start" name="copy"></lds-icon>${t("webhooks.copy")}</lds-button>
          <lds-button data-close>${t("webhooks.done")}</lds-button>
        </div>
      </lds-modal>`;
  }

  async onClick(e) {
    const target = e.target.closest?.("[data-action], [data-close], tr[data-id]");
    if (!target) return;
    if (target.hasAttribute("data-close")) {
      target.closest("lds-modal").removeAttribute("open");
      return;
    }
    if (target.dataset.id && !e.target.closest("a")) {
      this.context.router.navigate(`/w/${this.ws.slug}/settings/webhooks/${target.dataset.id}`);
      return;
    }
    const { api, i18n } = this.context;
    const url = this.hook ? `${this.base}/${this.hook.id}` : "";
    try {
      switch (target.dataset.action) {
        case "new":
        case "edit":
          this.shadowRoot.querySelector('[data-dialog="edit"]').setAttribute("open", "");
          return;
        case "save":
          return await this.save(target);
        case "rotate":
          return await this.save(target, { rotate_secret: true });
        case "delete":
          if (!confirm(i18n.t("webhooks.deleteConfirm", { name: this.hook.name }))) return;
          await api.delete(url);
          this.context.router.navigate(`/w/${this.ws.slug}/settings/webhooks`);
          return;
        case "ping":
          await busy(target, () => api.post(`${url}/ping`));
          globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("webhooks.pinged") });
          setTimeout(() => this.load(), 2500); // the worker sends it within seconds
          return this.load();
        case "redeliver":
          await busy(target, () => api.post(`${url}/deliveries/${target.closest("tr").dataset.delivery}/redeliver`));
          setTimeout(() => this.load(), 2500);
          return this.load();
        case "more": {
          const { data } = await api.get(`${url}/deliveries`, { query: { cursor: this.next } });
          this.deliveries.push(...data.items);
          this.next = data.next_cursor;
          return this.render();
        }
        case "copy-secret":
          await navigator.clipboard?.writeText(this.shadowRoot.querySelector("[data-secret]").textContent);
          return;
      }
    } catch (err) {
      globalThis.LDS?.toast?.({ intent: "danger", title: errorMessage(err, i18n) });
    }
  }

  /** Creates or updates from the dialog; a new secret is shown once. */
  async save(button, extra = {}) {
    const { api, i18n, router } = this.context;
    const modal = this.shadowRoot.querySelector('[data-dialog="edit"]');
    const form = modal.querySelector("form");
    clearFieldErrors(form);
    form.querySelector("lds-alert").textContent = "";
    const fd = new FormData(form);
    const body = {
      name: String(fd.get("name") ?? "").trim(),
      url: String(fd.get("url") ?? "").trim(),
      project: String(fd.get("project") ?? ""),
      event_types: fd.getAll("event_types").map(String).filter(Boolean),
      active: form.querySelector('[name="active"]').hasAttribute("checked"),
      ...extra,
    };
    let saved;
    try {
      ({ data: saved } = await busy(button, () => (this.hook
        ? api.patch(`${this.base}/${this.hook.id}`, { body, ifMatch: `"${this.hook.version}"` })
        : api.post(this.base, { body }))));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else form.querySelector("lds-alert").textContent = errorMessage(err, i18n);
      return;
    }
    modal.removeAttribute("open");
    this.secret = saved.secret ?? ""; // shown once, by the next render
    if (this.hook) await this.load();
    else router.navigate(`/w/${this.ws.slug}/settings/webhooks/${saved.id}`); // loads it
  }
}

customElements.define("hzn-settings-webhooks", HznSettingsWebhooks);
