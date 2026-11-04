// <hzn-iteration-dialog>: plans, edits and completes iterations.
//   plan({ ws, project, next })            a new iteration, prefilled with the suggestion
//   edit({ ws, project, iteration })       name, goal and dates
//   complete({ ws, project, iteration, list })  closes it under the completion policy
// On success it fires hzn-iteration-changed on the document.

import { isOpen, points } from "horizon/components/planning-ui.js";
import { requestContext } from "horizon/core/context.js";
import { html, sheet } from "horizon/core/dom.js";
import { busy, clearFieldErrors, errorMessage, showFieldErrors } from "horizon/core/forms.js";

const styles = sheet(`
  form { display: grid; gap: var(--lds-space-4); }
  lds-alert:empty { display: none; }
  .actions { display: flex; justify-content: flex-end; gap: var(--lds-space-3); }
  .summary { display: grid; grid-template-columns: auto 1fr; gap: var(--lds-space-1) var(--lds-space-4); margin: 0; }
  .summary dt { color: var(--lds-color-fg-muted); }
  .summary dd { margin: 0; font-variant-numeric: tabular-nums; }
  p { margin: 0; color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
`);

class HznIterationDialog extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [styles];
    const { t } = this.context.i18n;
    root.innerHTML = String(html`
      <lds-modal size="md">
        <span slot="header" data-heading></span>
        <form novalidate></form>
        <div slot="footer" class="actions">
          <lds-button variant="ghost" data-close>${t("tokens.cancel")}</lds-button>
          <lds-button variant="primary" data-submit></lds-button>
        </div>
      </lds-modal>`);
    this.modal = root.querySelector("lds-modal");
    this.form = root.querySelector("form");
    this.form.addEventListener("submit", (e) => {
      e.preventDefault();
      this.submit();
    });
    this.form.addEventListener("lds-change", (e) => {
      if (e.target.matches("lds-date-range")) this.dates = { start: e.detail.start, end: e.detail.end };
      if (e.target.matches("[data-target]")) this.target = e.detail.value;
    });
    root.querySelector("[data-close]").addEventListener("click", () => this.modal.removeAttribute("open"));
    root.querySelector("[data-submit]").addEventListener("click", () => this.form.requestSubmit());
  }

  base() {
    return `/w/${encodeURIComponent(this.ws)}/projects/${encodeURIComponent(this.project)}/iterations`;
  }

  /** Opens the dialog to plan an iteration. */
  plan({ ws, project, next }) {
    this.open("plan", { ws, project }, { name: next?.name ?? "", goal: "", start_date: next?.start_date ?? "", end_date: next?.end_date ?? "" });
  }

  /** Opens the dialog to edit an iteration. */
  edit({ ws, project, iteration }) {
    this.open("edit", { ws, project, iteration }, iteration);
  }

  open(mode, { ws, project, iteration = null }, values) {
    const { t } = this.context.i18n;
    Object.assign(this, { mode, ws, project, iteration });
    this.dates = { start: values.start_date ?? "", end: values.end_date ?? "" };
    const active = iteration?.state === "active";
    this.shadowRoot.querySelector("[data-heading]").textContent = t(mode === "plan" ? "iterations.plan" : "iterations.edit");
    this.shadowRoot.querySelector("[data-submit]").textContent = t(mode === "plan" ? "iterations.planSubmit" : "account.save");
    this.form.innerHTML = String(html`
      <lds-alert intent="danger" role="alert"></lds-alert>
      <lds-field label="${t("iterations.name")}" required><lds-input name="name" required maxlength="100" value="${values.name}"></lds-input></lds-field>
      <lds-field label="${t("iterations.goal")}"><lds-textarea name="goal" rows="2" maxlength="1000" value="${values.goal ?? ""}"></lds-textarea></lds-field>
      <lds-field label="${t("iterations.dates")}">
        <lds-date-range name="dates" start="${this.dates.start}" end="${this.dates.end}" ${active ? html`min="${values.start_date}"` : ""}
          label="${t("iterations.dates")}" start-label="${t("iterations.startDate")}" end-label="${t("iterations.endDate")}"></lds-date-range>
        <span slot="help">${t(active ? "iterations.datesActive" : "iterations.datesHint")}</span>
      </lds-field>`);
    this.modal.setAttribute("open", "");
    requestAnimationFrame(() => this.form.querySelector('[name="name"]')?.focus());
  }

  /**
   * Opens the dialog to complete an iteration: what the policy will do, and
   * where unfinished issues go when it rolls them over.
   */
  complete({ ws, project, iteration, list }) {
    const { i18n } = this.context;
    const { t } = i18n;
    Object.assign(this, { mode: "complete", ws, project, iteration });
    const policy = list.settings.completion_policy.value;
    const s = iteration.stats;
    const finished = ["completed", "abandoned"].reduce((n, o) => n + (s.by_outcome[o]?.issues ?? 0), 0);
    const unfinished = s.issues - finished;
    const targets = list.items.filter((it) => isOpen(it) && it.id !== iteration.id);
    this.target = targets.find((it) => it.state !== "active")?.id ?? "";
    this.shadowRoot.querySelector("[data-heading]").textContent = t("iterations.completeTitle", { name: iteration.name });
    this.shadowRoot.querySelector("[data-submit]").textContent = t("iterations.complete");
    this.form.innerHTML = String(html`
      <lds-alert intent="danger" role="alert"></lds-alert>
      <dl class="summary">
        <dt>${t("iterations.done")}</dt><dd>${t("iterations.issuesPoints", { count: s.by_outcome.completed?.issues ?? 0, points: points(s.by_outcome.completed?.points ?? 0, i18n) })}</dd>
        <dt>${t("iterations.abandoned")}</dt><dd>${t("iterations.issuesPoints", { count: s.by_outcome.abandoned?.issues ?? 0, points: points(s.by_outcome.abandoned?.points ?? 0, i18n) })}</dd>
        <dt>${t("iterations.unfinished")}</dt><dd>${t("iterations.issuesPoints", { count: unfinished, points: points(s.points - (s.by_outcome.completed?.points ?? 0) - (s.by_outcome.abandoned?.points ?? 0), i18n) })}</dd>
      </dl>
      <p>${t(`iterations.policyNote.${policy}`)}</p>
      ${policy === "archive_done_rollover_rest" && unfinished > 0 ? html`
        <lds-field label="${t("iterations.rolloverTo")}">
          <lds-select data-target value="${this.target}">
            ${targets.map((it) => html`<option value="${it.id}">${it.name}</option>`)}
            <option value="">${t("planning.backlog")}</option>
          </lds-select>
        </lds-field>` : ""}`);
    this.modal.setAttribute("open", "");
  }

  async submit() {
    const { api, i18n } = this.context;
    const form = this.form;
    const alert = form.querySelector("lds-alert");
    alert.textContent = "";
    clearFieldErrors(form);
    const button = this.shadowRoot.querySelector("[data-submit]");
    let request;
    if (this.mode === "complete") {
      const body = this.target ? { rollover_to: this.target } : {};
      // No If-Match: issues loaded meanwhile bump the version, and the policy applies to what is there.
      request = () => api.post(`${this.base()}/${this.iteration.id}/complete`, { body });
    } else {
      const fd = new FormData(form);
      const body = {
        name: String(fd.get("name") ?? "").trim(),
        goal: String(fd.get("goal") ?? "").trim(),
        start_date: this.dates.start ?? "",
        end_date: this.dates.end ?? "",
      };
      request = this.mode === "plan"
        ? () => api.post(this.base(), { body })
        : () => api.patch(`${this.base()}/${this.iteration.id}`, { body, ifMatch: `"${this.iteration.version}"` });
    }
    let saved;
    try {
      ({ data: saved } = await busy(button, request));
    } catch (err) {
      const rest = err.fields?.length ? showFieldErrors(form, err.fields, i18n.t, i18n.has) : [];
      if (!err.fields?.length || rest.length) alert.textContent = errorMessage(err, i18n);
      return;
    }
    this.modal.removeAttribute("open");
    const title = { plan: "iterations.planned", edit: "account.saved", complete: "iterations.completed" }[this.mode];
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t(title, { name: saved.name }) });
    document.dispatchEvent(new CustomEvent("hzn-iteration-changed", { detail: { project: this.project, id: saved.id } }));
  }
}

customElements.define("hzn-iteration-dialog", HznIterationDialog);
