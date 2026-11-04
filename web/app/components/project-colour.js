// Colours of projects, issue types, labels, categories and select options:
// stored by name, painted with the LDS swatch tokens. Swatches pass contrast
// only as small fills beside a name: never as text colour or on their own.

export const COLOURS = ["blue", "green", "purple", "pink", "coral", "yellow", "grey"];

/** Paints every [data-colour] element under root with its swatch. */
export function paintSwatches(root) {
  for (const el of root.querySelectorAll("[data-colour]")) {
    // CSSOM writes are allowed by the CSP; style attributes are not.
    el.style.background = `var(--lds-color-swatch-${el.dataset.colour})`;
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
