// Project colours: stored by name, painted with LDS swatch tokens. Until LDS
// ships --lds-color-swatch-<name>, every swatch falls back to the accent.

export const COLOURS = ["blue", "green", "teal", "purple", "pink", "coral", "orange", "yellow", "grey"];

/** Paints every [data-colour] element under root with its swatch. */
export function paintSwatches(root) {
  for (const el of root.querySelectorAll("[data-colour]")) {
    // CSSOM writes are allowed by the CSP; style attributes are not.
    el.style.background = `var(--lds-color-swatch-${el.dataset.colour}, var(--lds-color-accent-default))`;
  }
}

/** A key suggestion from a project name: "Design system" → "DS", "Horizon" → "HOR". */
export function suggestKey(name) {
  const words = name
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .toUpperCase()
    .split(/[^A-Z0-9]+/)
    .filter(Boolean);
  let key = words.length > 1 ? words.map((w) => w[0]).join("") : (words[0] || "").slice(0, 3);
  key = key.replace(/^[0-9]+/, "").slice(0, 10);
  return key.length >= 2 ? key : "";
}
