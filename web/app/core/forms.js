// Form helpers for views built from LDS controls: reading values, showing
// API validation errors in lds-field error slots, and two stopgaps for
// lds-input (LDS v0.0.5) that the design system is asked to absorb:
//
// - its native <input> lives in a shadow root, so Enter doesn't submit the
//   surrounding form (implicit submission): submitOnEnter restores it;
// - it doesn't forward autocomplete, which password managers rely on:
//   setAutocomplete sets it on the inner input.

import { html } from "horizon/core/dom.js";

/** The form's values as a plain object (lds-* controls are form-associated). */
export function formValues(form) {
  return Object.fromEntries(new FormData(form));
}

/** Submits form when Enter is pressed in one of its lds-input controls. */
export function submitOnEnter(form) {
  form.addEventListener("keydown", (event) => {
    if (event.key !== "Enter" || event.isComposing || event.defaultPrevented) return;
    if (event.target?.localName !== "lds-input") return;
    event.preventDefault();
    form.requestSubmit();
  });
}

/** Sets autocomplete on an lds-input's native input once it is upgraded. */
export function setAutocomplete(input, value) {
  input.setAttribute("autocomplete", value);
  customElements.whenDefined(input.localName).then(() => {
    requestAnimationFrame(() => {
      const inner = input.shadowRoot?.querySelector("input");
      if (inner) inner.autocomplete = value;
    });
  });
}

/** Removes the error messages added by showFieldErrors. */
export function clearFieldErrors(form) {
  for (const el of form.querySelectorAll('[slot="error"]')) el.remove();
}

/**
 * Shows each field error in the lds-field holding the control named after
 * the field, translated as error.<field>.<code> or error.field.<code>.
 * Returns the errors that matched no control.
 */
export function showFieldErrors(form, fields, t, has) {
  clearFieldErrors(form);
  const unmatched = [];
  for (const { field, code } of fields || []) {
    const field_ = form.querySelector(`lds-field:has([name="${CSS.escape(field)}"])`);
    if (!field_) {
      unmatched.push({ field, code });
      continue;
    }
    const key = has(`error.${field}.${code}`) ? `error.${field}.${code}` : `error.field.${code}`;
    field_.insertAdjacentHTML("beforeend", String(html`<span slot="error">${t(key)}</span>`));
  }
  const first = form.querySelector('lds-field[invalid] [name], lds-field:has([slot="error"]) [name]');
  first?.focus();
  return unmatched;
}

/**
 * The translated message for an API error: error.<code>, else a generic one.
 * Errors that are not API answers are bugs: they are also logged, so a
 * generic message never hides them.
 */
export function errorMessage(err, i18n) {
  if (err?.name === "NetworkError") return i18n.t("error.network");
  if (err?.name !== "ApiError") console.error(err);
  const code = err?.code;
  return code && i18n.has(`error.${code}`) ? i18n.t(`error.${code}`) : i18n.t("error.unexpected");
}

/** Toggles the loading state of a form's submit button while fn runs. */
export async function busy(form, fn) {
  const button = form.querySelector('lds-button[type="submit"]');
  button?.setAttribute("loading", "");
  try {
    return await fn();
  } finally {
    button?.removeAttribute("loading");
  }
}
