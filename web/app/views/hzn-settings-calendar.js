// <hzn-settings-calendar>: /w/:ws/settings/calendar[?project=KEY] — the
// working calendar (docs/configuration.md#time-zone-and-working-calendar):
// the project's time zone (the workspace's is on the General page),
// working weekdays, and non-working dates. A project overrides the time
// zone and the weekdays, and adds its own dates to the workspace's or
// replaces them. The timeline schedules in these working days.

import { requestContext } from "horizon/core/context.js";
import { html, ldsLightSheet, sheet } from "horizon/core/dom.js";
import { busy, errorMessage } from "horizon/core/forms.js";
import { loadScope, scopePicker, watchScope } from "horizon/views/config-scope.js";
import { settingsPage, settingsStyles } from "horizon/views/settings-page.js";

const WEEKDAYS = [1, 2, 3, 4, 5, 6, 7];
const MODES = ["inherit", "add", "replace"];

const styles = sheet(`
  .page-body { display: grid; gap: var(--lds-space-5); max-width: 48rem; }
  section { display: grid; gap: var(--lds-space-3); }
  .section-head { display: flex; align-items: center; justify-content: space-between; gap: var(--lds-space-4); }
  h2 { margin: 0; font-size: var(--lds-font-size-md); }
  p { margin: 0; }
  .days { display: flex; gap: var(--lds-space-4); flex-wrap: wrap; }
  .dates { display: grid; gap: var(--lds-space-1); }
  .date { display: grid; grid-template-columns: 9rem minmax(0, 1fr) auto; align-items: center; gap: var(--lds-space-3);
    min-block-size: 2.25rem; border-block-end: 1px solid var(--lds-color-border-subtle); }
  .date.inherited { color: var(--lds-color-fg-muted); }
  .add { display: grid; grid-template-columns: 11rem minmax(0, 1fr) auto; gap: var(--lds-space-2); align-items: end; }
  lds-combobox { max-width: 22rem; }
`);

/** Weekday names in the reader's language, Monday first. */
function weekdayName(locale, iso) {
  return new Intl.DateTimeFormat(locale, { weekday: "long", timeZone: "UTC" }).format(new Date(Date.UTC(2024, 0, iso))); // 2024-01-01 is a Monday
}

function longDate(locale, date) {
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeZone: "UTC" }).format(new Date(`${date}T00:00:00Z`));
}

class HznSettingsCalendar extends HTMLElement {
  static observedAttributes = ["ws"];

  connectedCallback() {
    if (this.shadowRoot) return;
    this.context = requestContext(this);
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [ldsLightSheet(), settingsStyles, styles];
    root.addEventListener("click", (e) => this.onClick(e));
    root.addEventListener("lds-change", (e) => this.onChange(e));
    this.unsubscribe = this.context.router.subscribe((m) => m.route?.name === "settings-calendar" && this.load());
    this.load();
  }

  disconnectedCallback() {
    this.unsubscribe?.();
  }

  async load() {
    const { api, i18n } = this.context;
    try {
      this.scope = await loadScope(this.context, this.getAttribute("ws"));
      const { data } = await api.get(`${this.scope.base}/settings/calendar`);
      this.saved = data;
    } catch (err) {
      this.shadowRoot.innerHTML = String(html`<lds-alert intent="danger">${errorMessage(err, i18n)}</lds-alert>`);
      return;
    }
    const s = this.saved;
    const project = !!this.scope.project;
    // The edited state; "set" is whether the scope sets the value itself.
    this.edit = {
      zone: { set: s.time_zone.overridden, value: s.time_zone.value },
      days: { set: !project || s.working_days.overridden, value: [...s.working_days.value] },
      mode: project ? s.holidays.mode : "replace",
      dates: [...(project ? s.holidays.own : s.holidays.workspace)],
    };
    this.render();
  }

  get dirty() {
    const s = this.saved;
    const e = this.edit;
    const project = !!this.scope.project;
    const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
    return (project && (e.zone.set !== s.time_zone.overridden || (e.zone.set && e.zone.value !== s.time_zone.value)))
      || (project && e.days.set !== s.working_days.overridden) || (e.days.set && !same(e.days.value, s.working_days.value))
      || (project && e.mode !== s.holidays.mode)
      || !same(e.dates, project ? s.holidays.own : s.holidays.workspace);
  }

  render() {
    const { i18n } = this.context;
    const { t } = i18n;
    const scope = this.scope;
    const can = this.saved.can_manage;
    const e = this.edit;
    const project = scope.project;
    const zones = Intl.supportedValuesOf?.("timeZone") ?? [e.zone.value];
    const override = (key, on) => (project
      ? html`<lds-switch data-override="${key}" ${on ? html`checked` : ""} ${can ? "" : html`disabled`}>${t("iterationSettings.override", { name: project.name })}</lds-switch>`
      : "");
    const days = e.days.set ? e.days.value : this.saved.working_days.workspace;
    const own = project && e.mode !== "inherit";
    const editsDates = can && (!project || own);
    const inherited = project && e.mode === "add" ? this.saved.holidays.workspace.filter((h) => !e.dates.some((d) => d.date === h.date)) : [];
    const list = project && e.mode === "inherit" ? this.saved.holidays.workspace : e.dates;
    const rows = [...list.map((h) => ({ ...h, own: !project || own })), ...inherited.map((h) => ({ ...h, own: false }))].sort((a, b) => a.date.localeCompare(b.date));
    const body = html`
      <div class="page-body">
        <section aria-labelledby="cal-zone">
          <div class="section-head"><h2 id="cal-zone">${t("calendar.zone")}</h2>${override("zone", e.zone.set)}</div>
          ${project
            ? html`<lds-combobox data-zone value="${e.zone.set ? e.zone.value : this.saved.time_zone.workspace}" label="${t("calendar.zone")}" ${can && e.zone.set ? "" : html`disabled`}>
                ${zones.map((z) => html`<lds-option value="${z}">${z}</lds-option>`)}</lds-combobox>
              <p class="muted">${e.zone.set ? "" : `${t("iterationSettings.inherited", { name: scope.ws.name })} `}${t("calendar.zoneNote")}</p>`
            : html`<p><strong>${this.saved.time_zone.workspace}</strong> · <a href="/w/${encodeURIComponent(scope.ws.slug)}/settings/general">${t("calendar.changeZone")}</a></p>
              <p class="muted">${t("calendar.zoneNote")}</p>`}
        </section>
        <lds-divider></lds-divider>
        <section aria-labelledby="cal-days">
          <div class="section-head"><h2 id="cal-days">${t("calendar.days")}</h2>${override("days", e.days.set)}</div>
          <div class="days" role="group" aria-labelledby="cal-days">
            ${WEEKDAYS.map((d) => html`<lds-checkbox data-day="${d}" ${days.includes(d) ? html`checked` : ""} ${can && e.days.set ? "" : html`disabled`}>${weekdayName(i18n.locale, d)}</lds-checkbox>`)}
          </div>
          <p class="muted">${project && !e.days.set ? `${t("iterationSettings.inherited", { name: scope.ws.name })} ` : ""}${t("calendar.daysNote")}</p>
        </section>
        <lds-divider></lds-divider>
        <section aria-labelledby="cal-dates">
          <div class="section-head"><h2 id="cal-dates">${t("calendar.dates")}</h2></div>
          ${project ? html`
            <lds-radio-group data-mode value="${e.mode}" label="${t("calendar.mode")}" ${can ? "" : html`disabled`}>
              ${MODES.map((m) => html`<button type="button" value="${m}">${t(`calendar.modes.${m}`, { name: scope.ws.name })}</button>`)}
            </lds-radio-group>` : ""}
          <div class="dates" role="list" aria-label="${t("calendar.dates")}">
            ${rows.length ? rows.map((h) => html`
              <div class="date${h.own ? "" : " inherited"}" role="listitem">
                <span>${longDate(i18n.locale, h.date)}</span><span>${h.name}</span>
                ${h.own && editsDates ? html`<lds-button variant="ghost" size="sm" data-action="remove-date" data-date="${h.date}" label="${t("calendar.removeDate", { name: h.name })}"><lds-icon name="trash-2" size="sm"></lds-icon></lds-button>` : html`<span></span>`}
              </div>`) : html`<p class="muted">${t("calendar.noDates")}</p>`}
          </div>
          ${editsDates ? html`
            <div class="add">
              <lds-field label="${t("calendar.date")}"><lds-date-picker data-new-date aria-label="${t("calendar.date")}"></lds-date-picker></lds-field>
              <lds-field label="${t("calendar.dateName")}"><lds-input data-new-name maxlength="100"></lds-input></lds-field>
              <lds-button variant="secondary" data-action="add-date"><lds-icon slot="start" name="plus" size="sm"></lds-icon>${t("calendar.addDate")}</lds-button>
            </div>` : ""}
          <p class="muted">${t("calendar.datesNote")}</p>
        </section>
      </div>`;
    const actions = html`${scopePicker(t, scope)}
      ${can ? html`<lds-button variant="ghost" data-action="discard" ${this.dirty ? "" : html`disabled`}>${t("roles.discard")}</lds-button>
        <lds-button data-action="save" ${this.dirty ? "" : html`disabled`}>${t("account.save")}</lds-button>` : ""}`;
    this.shadowRoot.innerHTML = String(settingsPage(t, scope.ws, "calendar", body, actions));
    watchScope(this.shadowRoot, this.context, scope.ws.slug, "calendar");
  }

  onChange(ev) {
    const el = ev.target;
    const e = this.edit;
    if (el.matches?.("[data-override]")) {
      const part = e[el.dataset.override];
      part.set = !!ev.detail.checked;
      if (el.dataset.override === "zone" && part.set && !this.saved.time_zone.overridden) part.value = this.saved.time_zone.workspace;
      if (el.dataset.override === "days" && part.set && !this.saved.working_days.overridden) part.value = [...this.saved.working_days.workspace];
      return this.render();
    }
    if (el.matches?.("[data-zone]") && ev.detail.value) e.zone.value = ev.detail.value;
    else if (el.matches?.("[data-day]")) {
      const d = Number(el.dataset.day);
      const next = new Set(e.days.value);
      if (ev.detail.checked) next.add(d);
      else next.delete(d);
      e.days.value = WEEKDAYS.filter((x) => next.has(x));
    } else if (el.matches?.("[data-mode]")) {
      e.mode = ev.detail.value;
      if (e.mode !== "inherit" && this.saved.holidays.mode === "inherit" && !e.dates.length) e.dates = [];
      return this.render();
    } else return;
    this.syncButtons();
  }

  syncButtons() {
    for (const b of this.shadowRoot.querySelectorAll('[data-action="save"], [data-action="discard"]')) b.toggleAttribute("disabled", !this.dirty);
  }

  onClick(ev) {
    const el = ev.target.closest?.("[data-action]");
    if (!el || el.hasAttribute("disabled")) return;
    switch (el.dataset.action) {
      case "discard":
        return this.load();
      case "save":
        return this.save(el);
      case "remove-date":
        this.edit.dates = this.edit.dates.filter((h) => h.date !== el.dataset.date);
        return this.render();
      case "add-date": {
        const root = this.shadowRoot;
        const date = root.querySelector("[data-new-date]").getAttribute("value") ?? "";
        const name = (root.querySelector("[data-new-name]").getAttribute("value") ?? "").trim();
        if (!/^\d{4}-\d{2}-\d{2}$/.test(date) || !name) {
          globalThis.LDS?.toast?.({ intent: "warning", title: this.context.i18n.t("calendar.dateNeeded") });
          return;
        }
        this.edit.dates = [...this.edit.dates.filter((h) => h.date !== date), { date, name }].sort((a, b) => a.date.localeCompare(b.date));
        return this.render();
      }
    }
  }

  async save(button) {
    const { api, i18n } = this.context;
    const s = this.saved;
    const e = this.edit;
    const project = !!this.scope.project;
    const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
    const body = {};
    if (project) {
      if (!e.zone.set && s.time_zone.overridden) body.time_zone = null;
      else if (e.zone.set && (!s.time_zone.overridden || e.zone.value !== s.time_zone.value)) body.time_zone = e.zone.value;
    }
    if (project && !e.days.set && s.working_days.overridden) body.working_days = null;
    else if (e.days.set && (!same(e.days.value, s.working_days.value) || (project && !s.working_days.overridden))) body.working_days = e.days.value;
    if (project) {
      if (e.mode === "inherit") {
        if (s.holidays.mode !== "inherit") body.holidays = null;
      } else {
        if (s.holidays.mode !== e.mode) body.holidays_mode = e.mode;
        if (s.holidays.mode === "inherit" || !same(e.dates, s.holidays.own)) body.holidays = e.dates;
      }
    } else if (!same(e.dates, s.holidays.workspace)) {
      body.holidays = e.dates;
    }
    try {
      await busy(button, () => api.put(`${this.scope.base}/settings/calendar`, { body }));
    } catch (err) {
      const field = err.fields?.[0];
      globalThis.LDS?.toast?.({ intent: "danger", title: field && i18n.has(`calendar.error.${field.field}`) ? i18n.t(`calendar.error.${field.field}`) : errorMessage(err, i18n) });
      return;
    }
    globalThis.LDS?.toast?.({ intent: "success", title: i18n.t("account.saved") });
    this.load();
  }
}

customElements.define("hzn-settings-calendar", HznSettingsCalendar);
