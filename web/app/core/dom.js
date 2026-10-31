// Small DOM helpers for hzn-* components: constructable style sheets (CSP
// allows them, unlike <style> elements) and escaped HTML templates.

const sheets = new Map();

/** Returns a cached CSSStyleSheet for cssText, for adoptedStyleSheets. */
export function sheet(cssText) {
  let s = sheets.get(cssText);
  if (!s) {
    s = new CSSStyleSheet();
    s.replaceSync(cssText);
    sheets.set(cssText, s);
  }
  return s;
}

// LDS styles some light-DOM markup from the document's stylesheet: the cells
// inside <lds-table> and the .lds-* utility classes. Those rules don't reach
// into shadow roots, so views adopt a copy.
const LIGHT_DOM_RULE = /lds-table|\.lds-[a-z]/;
let ldsLight = null;

/**
 * Returns a sheet holding LDS's light-DOM rules (lds-table cells, .lds-*
 * utilities), copied from lds.min.css, for adoptedStyleSheets. It fills in
 * when the stylesheet finishes loading if it hasn't yet.
 */
export function ldsLightSheet() {
  if (ldsLight) return ldsLight;
  ldsLight = new CSSStyleSheet();
  const link = document.querySelector('link[rel="stylesheet"][href$="lds.min.css"]');
  const fill = () => ldsLight.replaceSync(link?.sheet ? lightRules(link.sheet.cssRules) : "");
  if (link && !link.sheet) link.addEventListener("load", fill, { once: true });
  else fill();
  return ldsLight;
}

function lightRules(rules) {
  let css = "";
  for (const rule of rules) {
    if (rule instanceof CSSStyleRule) {
      if (LIGHT_DOM_RULE.test(rule.selectorText)) css += rule.cssText + "\n";
    } else if (rule instanceof CSSMediaRule) {
      const inner = lightRules(rule.cssRules);
      if (inner) css += `@media ${rule.conditionText} {\n${inner}}\n`;
    }
  }
  return css;
}

const ESCAPES = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };

/** Escapes a value for HTML text or a quoted attribute. */
export function escapeHtml(value) {
  return String(value ?? "").replace(/[&<>"']/g, (c) => ESCAPES[c]);
}

class Raw {
  constructor(value) {
    this.value = value;
  }
  toString() {
    return this.value;
  }
}

/** Marks trusted markup so html`` inserts it unescaped. */
export function raw(value) {
  return new Raw(String(value));
}

/**
 * Tagged template that escapes every interpolated value, except raw() ones
 * and arrays of them (lists built with nested html``).
 */
export function html(strings, ...values) {
  let out = strings[0];
  values.forEach((value, i) => {
    out += render(value) + strings[i + 1];
  });
  return raw(out);
}

function render(value) {
  if (value instanceof Raw) return value.value;
  if (Array.isArray(value)) return value.map(render).join("");
  if (value === false || value == null) return "";
  return escapeHtml(value);
}
