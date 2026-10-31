// Message catalogs and Intl formatting (ADR-0017). Every user-visible string
// goes through t(). Catalog values are strings with {name} placeholders, or
// plural objects keyed by Intl.PluralRules categories:
//
//   "issues.count": { "one": "{count} issue", "other": "{count} issues" }

/**
 * @param {Record<string, string|Record<string,string>>} catalog
 * @param {object} [options]
 * @param {string} [options.locale] defaults to the browser's
 * @param {string} [options.timeZone] defaults to the browser's
 * @param {(key: string) => void} [options.onMissing]
 */
export function createI18n(catalog, { locale = navigator.language, timeZone, onMissing } = {}) {
  const plurals = new Intl.PluralRules(locale);
  const formats = new Map();
  const missing = new Set();

  function format(kind, options, make) {
    const key = kind + JSON.stringify(options);
    let f = formats.get(key);
    if (!f) {
      f = make();
      formats.set(key, f);
    }
    return f;
  }

  function interpolate(message, params) {
    return message.replace(/\{(\w+)\}/g, (whole, name) => {
      if (!(name in params)) return whole;
      const v = params[name];
      return typeof v === "number" ? i18n.number(v) : String(v);
    });
  }

  const i18n = {
    locale,

    /** Translates key; params fill {name} placeholders, count picks the plural. */
    t(key, params = {}) {
      let message = catalog[key];
      if (message === undefined) {
        if (!missing.has(key)) {
          missing.add(key);
          (onMissing || ((k) => console.warn(`i18n: missing message ${k}`)))(key);
        }
        return key;
      }
      if (typeof message === "object") {
        message = message[plurals.select(params.count ?? 0)] ?? message.other ?? key;
      }
      return interpolate(message, params);
    },

    /** Whether key exists, for optional messages such as error codes. */
    has(key) {
      return key in catalog;
    },

    number(value, options = {}) {
      return format("n", options, () => new Intl.NumberFormat(locale, options)).format(value);
    },

    /** Formats a Date, a timestamp or an RFC 3339 string in the user's zone. */
    date(value, options = { dateStyle: "medium" }) {
      const opts = { timeZone, ...options };
      return format("d", opts, () => new Intl.DateTimeFormat(locale, opts)).format(new Date(value));
    },

    /** "3 days ago", "in 2 hours", relative to now. */
    relative(value, now = Date.now()) {
      const seconds = (new Date(value).getTime() - now) / 1000;
      const units = [
        ["year", 31536000],
        ["month", 2592000],
        ["week", 604800],
        ["day", 86400],
        ["hour", 3600],
        ["minute", 60],
      ];
      const rtf = format("r", {}, () => new Intl.RelativeTimeFormat(locale, { numeric: "auto" }));
      for (const [unit, size] of units) {
        if (Math.abs(seconds) >= size) return rtf.format(Math.round(seconds / size), unit);
      }
      return rtf.format(Math.round(seconds), "second");
    },
  };
  return i18n;
}
