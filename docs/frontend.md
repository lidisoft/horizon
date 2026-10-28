# Frontend

A single-page app in **plain JavaScript**: ES modules served as-is, no build
step, no npm, **no third-party code** ([ADR-0006](adr/0006-spa-native-js-no-dependencies.md)).
Styling and base components come from **LDS**, served by the Go binary at
`/lds/`. The app is **standalone** but built to be mounted in the LiDiSoft
shell later ([ADR-0018](adr/0018-standalone-spa-shell-ready.md)).

## Look and feel

Linear is the reference: a slim left sidebar (workspace switcher, inbox, my
issues, projects, views), a dense content area, an issue detail panel or full
page, ⌘K for everything, keyboard shortcuts for every frequent action, subtle
motion, dark and light themes (LDS `data-lds-theme`).

## Layout of `web/`

```
web/
  index.html            shell page: LDS assets, import map, <hzn-app>
  app/
    core/               framework-free foundations, usable with or without the frame
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

Modules are loaded natively (`<script type="module">`), with an import map for
stable bare specifiers (`horizon/core/store.js`). Go serves `web/` with
`go:embed` and content-hashed URLs for caching: the server rewrites the import
map at startup with hashes of each file.

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

Generic UI comes from **LDS**. What Horizon needs that LDS lacks splits into
two groups:

**Generic — build upstream in LDS** (tracked in the LDS roadmap):
`lds-popover`, `lds-tooltip`, `lds-drawer`, `lds-kbd`, `lds-empty-state`,
`lds-skeleton`, a combobox / multi-select with search, a date-range picker.

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
| `hzn-*-picker` | Status, priority, assignee, labels, estimate, iteration, release pickers |
| `hzn-inbox` | Notifications |

## Shell readiness

Every route view is a custom element that takes its context from attributes
(`<hzn-board workspace="acme" project="HZN">`) and gets its services (`api`,
`store`, `socket`) from a context object, not from globals. `hzn-app` creates
that context in standalone mode; a future shell adapter would create it too.

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

- Pure modules (`store`, `rank`, `editor` steps and mapping, filter evaluation)
  are tested with a tiny in-browser runner (`web/test/index.html`), run
  headless in CI through Go's `chromedp`.
- The editor's step algorithms share their test vectors with Go.
- End-to-end flows use `chromedp` from Go tests against a real server.
