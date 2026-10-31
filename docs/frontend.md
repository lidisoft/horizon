# Frontend

A single-page app in **plain JavaScript**: ES modules served as-is, no build
step, no npm, **no third-party code** ([ADR-0006](adr/0006-spa-native-js-no-dependencies.md)).
Styling and base components come from **LDS**, served by the Go binary at
`/lds/`. The app is **standalone** but built to be mounted in the LiDiSoft
shell later ([ADR-0018](adr/0018-standalone-spa-shell-ready.md)).

## Look and feel

v1 targets **desktop browsers only** (minimum viewport about 1024 px wide).
Layouts need not adapt to phones yet, but avoid choices that would make it
hard later (fixed pixel widths for content, hover-only actions).


Linear is the reference: a slim left sidebar (workspace switcher, inbox, my
issues, projects, views), a dense content area, an issue detail panel or full
page, ⌘K for everything, keyboard shortcuts for every frequent action, subtle
motion, dark and light themes (LDS `data-lds-theme`).

## Layout of `web/`

```
web/
  embed.go              go:embed of index.html and app/
  index.html            shell page template: LDS assets, import map, <hzn-app>
  app/
    main.js             entry: defines the views, then <hzn-app>
    core/               framework-free foundations, usable with or without the frame
      context.js        service lookup for views (provideContext / requestContext)
      dom.js            constructable style sheets, escaped html`` templates
      api.js            fetch wrapper: auth, idempotency keys, problem details, ETags
      socket.js         WebSocket client: reconnect, subscriptions, replay
      store.js          normalised cache (entities by id) + query cache + subscriptions
      mutations.js      optimistic updates with rollback
      router.js         History API router, issue-key deep links
      shortcuts.js      keyboard layer: scopes, sequences ("g i"), help overlay
      i18n.js           message catalog lookup, plurals, Intl formatting
      config.js         client-side workspace → project resolver
      rank.js           fractional index keys (same algorithm as Go)
    editor/             block editor and collab client
    components/         hzn-* custom elements
    views/              route-level elements: list, board, backlog, table, timeline, calendar, …
    i18n/en.json        message catalog
  vendor/               intentionally empty — see ADR-0006
```

Modules are loaded natively (`<script type="module">`). Go embeds `web/` and,
at startup, serves every file of `app/` at `/static/<content hash>/<path>`
(cached as immutable) and renders `index.html` with an **import map** from
bare specifiers to those URLs (`horizon/core/store.js` →
`/static/f738455ceb32/core/store.js`). `index.html` itself is `no-cache`, so
a deploy changes every changed module's URL and nothing else.

Rules this implies, checked by `internal/server/web_test.go`:

- Modules import each other **only through `horizon/…` specifiers**, never
  relative paths (those would bypass the hashes and load a module twice).
- The message catalog is a JSON module
  (`import catalog from "horizon/i18n/en.json" with { type: "json" }`), so it
  is hashed too; every literal `t("…")` key must exist in it.
- No inline scripts or styles. The Content-Security-Policy is
  `script-src 'self'` plus the import map's own hash, and `style-src 'self'`:
  components style their shadow roots with constructable style sheets
  (`sheet()` in `core/dom.js`), which CSP allows.

LDS is served from its Go module at `/lds/<module version>/` (immutable too),
so upgrading `github.com/lidisoft/design-system` in `go.mod` changes its URLs.

## State and data flow

```
user action ─▶ mutation (optimistic apply to store) ─▶ REST
                       ▲                                  │
                       └── rollback on error ◀────────────┤
socket event ─▶ store.apply(event) ─▶ subscribed components re-render
```

- **Store**: entities normalised by id; queries (a view's filter) cached by
  hash and kept live by re-evaluating incoming events client-side when
  possible, refetching otherwise.
- **Rendering**: components subscribe to store slices and re-render with
  small, explicit DOM updates (a keyed-list helper in `core/`). No virtual DOM.
- **Large lists**: windowed rendering for list and table views above ~200 rows.

## Components

Generic UI comes from **LDS** (pinned at `v0.0.5`). The generic pieces Horizon
asked LDS for have all shipped:

| Element | Horizon use |
|---------|-------------|
| `lds-combobox` + `lds-option` | Every property picker (status, priority, assignee, labels, iteration, release, parent). Glyph or avatar in the option's `start` slot, key or dates in `end`; labels use `multiple` and `creatable`; server-side search with `filter="none"` + `lds-search` |
| `lds-segmented` + `lds-segment` | View switcher (list, board, backlog, table, timeline, calendar), report measure (points / issues), inbox unread / all |
| `lds-date-range` | Start → due on issues, iteration dates; value is the ISO interval `start/end` |
| `lds-kbd` | One element per key in the command palette, menus, tooltips and the shortcut help (`G` then `I`) |
| `lds-empty-state` | Empty views and iterations, searches with no results (name the query, offer to clear filters) |
| `lds-skeleton` | Loading rows, cards and charts, with `aria-busy` on the region |
| `lds-nav-group href` | A project in the sidebar: the header links to the project overview (swatch in `start`, key in `end`), the chevron shows its pages |
| `lds-popover` | Display options, filter builder, picker anchors. Non-modal; `placement`, `trigger` slot |
| `lds-tooltip` | Every icon-only button (still give the button a `label`); `placement="right"` on the collapsed sidebar |
| `lds-drawer non-modal` | Issue peek panel: no backdrop, list stays interactive; set `--lds-drawer-inset-top` to the header height |
| Chart tokens | `--lds-color-chart-completed` / `started` / `unstarted` / `abandoned` (fixed stacking order), `-reference`, `-marker`, `-annotation`, `--lds-chart-bar-radius`, `--lds-chart-segment-gap` — see [reports.md](reports.md#charts) |
| `lds-icon` | 68 Lucide icons, including status (`circle-dashed`, `circle`, `circle-dot`, `circle-check`, `circle-x`), priority (`signal-*`, `octagon-alert`), layouts and git |

The combobox exposes its highlighted option through ARIA element reflection;
browsers without it still filter and select but do not announce the highlight.

**Horizon-specific — `hzn-*` in this repo**, following LDS rules (shadow DOM,
semantic tokens only, `:host` display):

| Element | Purpose |
|---------|---------|
| `hzn-app` | Frame: sidebar, routing outlet, ⌘K, toasts |
| `hzn-issue-row` | Dense list row with inline pickers |
| `hzn-issue-detail` | Issue page and peek panel |
| `hzn-board` | Kanban with drag and drop between columns and rank |
| `hzn-backlog` | Backlog plus iteration buckets, drag to plan |
| `hzn-grid` | Table view with field columns and inline editing |
| `hzn-timeline` | Gantt: bars, dependencies, drag to reschedule, milestones |
| `hzn-calendar` | Month and week calendar |
| `hzn-filter-bar` | Filter chips and builder |
| `hzn-editor` | Block editor |
| `hzn-*-picker` | Status, priority, assignee, labels, estimate, iteration, release pickers: `lds-combobox` wrappers that load options from the store and apply mutations |
| `hzn-inbox` | Notifications |
| `hzn-chart-*`, `hzn-sparkline` | Hand-written SVG charts for reports ([reports.md](reports.md#charts)) |
| `hzn-project-overview` | Fixed project overview page |

## Shell readiness

Every route view is a custom element that takes its context from attributes
(`<hzn-board workspace="acme" project="HZN">`) and gets its services (`api`,
`store`, `router`, `i18n`, `shortcuts`, later `socket`) from a context object,
not from globals. A view calls `requestContext(this)` when connected: it
dispatches a composed `hzn-context` event that the nearest provider answers.
`hzn-app` provides the context in standalone mode (`provideContext`); a future
shell adapter would provide the same object.

## Block editor

A hand-written `contenteditable` editor working on the document model in
[realtime.md](realtime.md#document-model):

- The model is the source of truth. DOM input events (`beforeinput`) are
  converted to steps and applied to the model, then the DOM is reconciled. The
  DOM is never read back as the document, except to recover from IME
  composition.
- Slash menu (`/`) for block types, Markdown-style input rules (`## `, `- `,
  `[] `, `` ``` ``), mentions (`@`), issue references (`HZN-` autocomplete),
  paste from HTML and Markdown, and image paste uploaded to S3.
- Undo/redo with grouping, local to the user (it inverts their own steps,
  mapped through remote ones).
- Remote cursors and selections rendered as overlays.

This is the riskiest frontend component. It gets its own milestone and test
corpus ([roadmap.md](roadmap.md)).

## Accessibility and keyboard

- Every action is reachable by keyboard; the shortcut help (`?`) is generated
  from the shortcut registry.
- Board and backlog drag and drop have keyboard equivalents (move with
  `alt+arrow`, or via the command palette).
- LDS conventions apply (focus rings, ARIA wiring, contrast checked by eye in
  both themes).

## i18n

Every string goes through `t("key", params)` with the catalog in
`app/i18n/en.json`. Dates and numbers use `Intl` with the user's locale and
time zone. Only English ships in v1.

## Testing

No npm means no Jest or Playwright from Node:

- Pure modules are tested with a small in-browser runner: `web/test/runner.js`
  (`describe`, `test`, `assert`) and one `*.test.js` per module under
  `web/test/`, importing `horizon/…` modules and `horizon-test/runner.js`.
  `web/test/` is read from disk by the tests and never embedded in the binary.
- `web/browser_test.go` serves the runner page with the real app assets under
  the production CSP, runs every test file in headless Chrome through
  `chromedp`, and reports each JS test as a Go subtest. `task test:web` runs
  them; `task test:web:serve` serves the page on http://localhost:8089/test/
  to debug in a local browser (`?filter=store` runs a subset).
- The editor's step algorithms will share their test vectors with Go.
- End-to-end flows use `chromedp` from Go tests against the real server
  engine and page (`TestAppShell`), with real keyboard input.
