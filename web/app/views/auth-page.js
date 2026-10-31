// Shared layout of the signed-out pages (sign-in, forgot and reset
// password): a centred card without the app frame's sidebar.

import { html, sheet } from "horizon/core/dom.js";

export const authStyles = sheet(`
  :host { display: grid; place-items: center; min-height: 100vh; padding: var(--lds-space-6); box-sizing: border-box; }
  .page { width: min(100%, 24rem); display: grid; gap: var(--lds-space-6); }
  .brand { margin: 0; text-align: center; font-family: var(--lds-font-family-display); font-size: var(--lds-font-size-xl); }
  h1 { margin: 0 0 var(--lds-space-2); font-size: var(--lds-font-size-lg); }
  .lead { margin: 0 0 var(--lds-space-5); color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); }
  form { display: grid; gap: var(--lds-space-4); }
  .links { margin: var(--lds-space-4) 0 0; text-align: center; font-size: var(--lds-font-size-sm); }
  a { color: var(--lds-color-fg-link); }
  lds-alert:empty { display: none; }
`);

/** The page markup: brand, then a card titled title holding body. */
export function authPage(t, title, body) {
  return html`
    <div class="page">
      <p class="brand">${t("app.name")}</p>
      <lds-card>
        <h1>${title}</h1>
        ${body}
      </lds-card>
    </div>
  `;
}
