# Wireframes

Static copy of the Horizon wireframes, viewable without an account or a
server: open [`index.html`](index.html) in a browser, straight from disk.

The gallery groups the boards as the canvas does. Open a board to switch its
theme and the states it supports (for example the workflow editor's publish
dialog, or the members page's teams and invite views). Links inside the boards
move between them, as on the canvas.

## Source of truth

The boards are authored on a Claude Design canvas (private; ask for access).
This folder is an export of it and is not edited by hand:

| File | Content |
|------|---------|
| `boards.js` | Generated. Each board's template, logic and size, copied from the canvas, plus the grouping |
| `lds/` | Generated. The LDS build the canvas uses (`v0.0.5`): tokens, CSS, components with the icon sprite, fonts |
| `viewer.js` | Hand-written renderer for the template features the boards use (`{{ }}`, `sc-for`, `sc-if`, `dc-import`, `helmet`) and the gallery |
| `index.html` | Entry point |

To refresh it, change the canvas, then export it again over `boards.js` and
`lds/`. Keep `viewer.js` in step if a board starts using a template feature it
doesn't handle; set `window.HZ_DEBUG = true` in the console to log template
expressions that fail.

These are wireframes: sample data is Horizon's own roadmap, and the boards use
some core colour tokens (project swatches) that application code must not use
([AGENTS.md](../../AGENTS.md), rule 6).
