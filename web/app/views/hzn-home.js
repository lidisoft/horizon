// <hzn-home>: "/". Sends the user to the workspace they opened last (or
// their first one). Without a workspace it says so, lets instance admins
// create one, and reports the instance's readiness checks.

import { rememberedWorkspace } from "horizon/components/hzn-app.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { busy, errorMessage, formValues, showFieldErrors, submitOnEnter } from "horizon/core/forms.js";

const styles = sheet(`
  :host { display: block; padding: var(--lds-space-12) var(--lds-space-6); }
  .checks { display: grid; grid-template-columns: max-content max-content; gap: var(--lds-space-2) var(--lds-space-6); margin: 0; justify-content: center; }
  .checks dt { color: var(--lds-color-fg-muted); }
  .checks dd { margin: 0; }
  h3 { margin: var(--lds-space-8) 0 var(--lds-space-3); text-align: center; font-size: var(--lds-font-size-sm); }
  p.status { text-align: center; color: var(--lds-color-fg-muted); }
  form { display: grid; gap: var(--lds-space-4); max-width: 24rem; margin: var(--lds-space-6) auto 0; }
  lds-alert:empty { display: none; }
`);

const CHECKS = ["postgres", "nats", "s3", "schema"];

/** A slug suggestion from a name: "Acme Corp" → "acme-corp". */
export function slugify(name) {
  return name
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^[^a-z]+|-+$/g, "")
    .slice(0, 40)
    .replace(/-+$/g, "");
}

class HznHome extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const { router, session, workspaces } = this.context;
    const list = workspaces.list();
    if (list.length) {
      const slug = list.some((w) => w.slug === rememberedWorkspace()) ? rememberedWorkspace() : list[0].slug;
      queueMicrotask(() => router.navigate(`/w/${slug}`, { replace: true }));
      return;
    }

    const { t } = this.context.i18n;
    const admin = session.user?.is_instance_admin;
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [styles];
    root.innerHTML = String(html`
      <lds-empty-state>
        <lds-icon slot="icon" name="rocket"></lds-icon>
        <h2 slot="title">${t("home.noWorkspace")}</h2>
        ${t(admin ? "home.noWorkspaceAdmin" : "home.noWorkspaceMember")}
      </lds-empty-state>
      ${admin ? html`
        <form novalidate>
          <lds-alert intent="danger" role="alert"></lds-alert>
          <lds-field label="${t("field.workspaceName")}" required>
            <lds-input name="name" required maxlength="100"></lds-input>
          </lds-field>
          <lds-field label="${t("field.workspaceSlug")}" required>
            <lds-input name="slug" required maxlength="40"><span slot="prefix">/w/</span></lds-input>
            <span slot="help">${t("home.slugHelp")}</span>
          </lds-field>
          <lds-button type="submit">${t("home.createWorkspace")}</lds-button>
        </form>` : ""}
      <section aria-busy="true" aria-labelledby="checks-title">
        <h3 id="checks-title">${t("home.checks")}</h3>
        <p class="status">${t("home.checking")}</p>
      </section>
    `);
    const form = root.querySelector("form");
    if (form) {
      const name = form.querySelector('[name="name"]');
      const slug = form.querySelector('[name="slug"]');
      let slugEdited = false;
      slug.addEventListener("lds-input", () => (slugEdited = true));
      name.addEventListener("lds-input", (e) => {
        if (!slugEdited) slug.setAttribute("value", slugify(e.detail.value));
      });
      submitOnEnter(form);
      form.addEventListener("submit", (e) => {
        e.preventDefault();
        this.create(form);
      });
    }
    this.loadChecks();
  }

  async create(form) {
    const { api, i18n, router, workspaces } = this.context;
    const alert = form.querySelector("lds-alert");
    alert.textContent = "";
    const { name = "", slug = "" } = formValues(form);
    const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    let created;
    try {
      ({ data: created } = await busy(form, () => api.post("/w", { body: { name, slug, time_zone: timeZone } })));
    } catch (err) {
      if (err.fields?.length) showFieldErrors(form, err.fields, i18n.t, i18n.has);
      else alert.textContent = errorMessage(err, i18n);
      return;
    }
    await workspaces.refresh();
    router.navigate(`/w/${created.slug}`);
  }

  async loadChecks() {
    const { t } = this.context.i18n;
    const section = this.shadowRoot.querySelector("section");
    let checks = null;
    try {
      // /readyz is a probe outside /api/v1; it answers 503 with the same body.
      const res = await fetch("/readyz", { headers: { Accept: "application/json" } });
      checks = (await res.json()).checks;
    } catch {
      // network failure: reported below
    }
    section.removeAttribute("aria-busy");
    if (!checks) {
      section.querySelector(".status").textContent = t("home.check.unreachable");
      return;
    }
    section.querySelector(".status").replaceWith(document.createRange().createContextualFragment(String(html`
      <dl class="checks">
        ${CHECKS.filter((name) => name in checks).map((name) => {
          const ok = checks[name] === "ok";
          return html`<dt>${t(`check.${name}`)}</dt>
            <dd><lds-badge size="sm" intent="${ok ? "success" : "danger"}" title="${ok ? "" : checks[name]}">${t(ok ? "home.check.ok" : "home.check.failing")}</lds-badge></dd>`;
        })}
      </dl>
    `)));
  }
}

customElements.define("hzn-home", HznHome);
