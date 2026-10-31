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
