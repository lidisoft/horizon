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
      session.js        signed-in user and CSRF token; sign-in, sign-out
      forms.js          form values, field errors from problem details, lds-input stopgaps
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

## Sessions and routes

Routes: `/` goes to the workspace opened last (remembered in local storage)
or the first one; `/w/:ws` is the workspace, `/w/:ws/p/:key` a project (a
former key redirects), `/w/:ws/settings/…` its settings (with their own
sidebar), `/invitation` accepts an invitation, and
`/account` holds the user's profile, password, sessions and tokens. The
sidebar's switcher lists the user's workspaces; views call
`context.workspaces.refresh()` after creating, renaming or joining one.

`hzn-app` asks `GET /api/v1/me` who is signed in before it starts routing.
Routes marked `public` (sign-in, forgot and reset password) render without
the sidebar; any other route sends a signed-out user to
`/sign-in?next=<path>`, and signing in returns them there (same-origin paths
only). The frame's guard owns these redirects; views don't navigate after
signing in or out.

The API client reports a `401 unauthenticated` to the session, which signs
the page out; a `403 csrf_invalid` (the session changed in another tab)
reloads `/me` for a fresh token and retries the mutation once with the same
idempotency key.

Forms use LDS controls, which are form-associated: `FormData` reads them.
Validation errors from the API (`422`, one `{ field, code }` each) are shown
in the matching `lds-field`'s error slot as `error.<field>.<code>`, falling
back to `error.field.<code>`.

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

`lds-popover` in v0.0.5 decides a press is outside its panel from the
event target at the document, which shadow roots retarget to their host, so
inside a view every press closed it: `keepOpenOnInsidePress` (core/dom.js)
cancels those closes from the press's composed path.

The combobox exposes its highlighted option through ARIA element reflection;
browsers without it still filter and select but do not announce the highlight.

**Horizon-specific — `hzn-*` in this repo**, following LDS rules (shadow DOM,
semantic tokens only, `:host` display):

| Element | Purpose |
|---------|---------|
| `hzn-app` | Frame: sidebar, routing outlet, ⌘K, toasts |
| `hzn-issue-list` (view) | A project's issues grouped by status in workflow order, rank order within; peek panel (non-modal `lds-drawer`) beside it |
| `hzn-issue-detail` | Issue page and peek panel (`mode="page" \| "peek"`): properties save as they change (If-Match), description, sub-issues, links, attachments, activity and comments |
| `hzn-issue-create` | New issue dialog (button, `C`, ⌘K) |
| `hzn-move-issue` | Move to another project, with the server's preview of matches and gaps |
| `hzn-search` (view) | Search page (`/w/{ws}/search?q=`): results with highlighted snippets; ⌘K shows the first matches as you type and "Search for …" leads here |
| `hzn-board` (view) | `/w/{ws}/p/{key}/board`: kanban. Columns are statuses, one per name across the project's workflows, in category order; a card dropped on a column moves to its own workflow's status of that name when a transition allows it (columns it can't reach dim while dragging), and ranks within a column. Shows the active iteration (a picker when several are; `?iteration=all` for every issue; all issues when none is active). Swimlanes by assignee, parent or priority, remembered per project in the browser; moving a card across lanes changes that field. `h`/`l`, `j`/`k` move between cards, `Alt+←/→` and `Alt+↑/↓` move the card. A saved board view keeps its swimlanes and WIP limits per column: a column over its limit shows `count / limit` highlighted, and whoever can edit the view sets limits from the column headers |
| `hzn-backlog` (view) | `/w/{ws}/p/{key}/backlog`: the open iterations (active first) and the backlog (unfinished issues in no open iteration), with each bucket's issues, points and load against velocity. Drag issues between buckets and up and down; `Alt+↑/↓` reorders, `Alt+Shift+↑/↓` moves to the bucket above or below, ⌘K lists where the current issue can go. Plan, edit, start, complete and delete iterations |
| `hzn-releases` (view) | `/w/{ws}/p/{key}/releases`: the project's releases with the progress of their issues (each can list them), then the project's and the workspace's milestones. Create, edit, release, archive and delete releases; issues are targeted from the issue's Release property |
| `hzn-project-home` (view) | `/w/{ws}/p/{key}`: the project overview ([reports.md](reports.md#project-overview-page)): active sprint with a burndown sparkline and its issues by outcome, next release and milestone, flow health, the viewer's open issues and recent activity; pushed events refresh it |
| `hzn-reports` (view) | `/w/{ws}/p/{key}/reports`: the project's reports in four tabs: Sprint (an iteration's burndown, burnup and sprint report, and velocity), Flow (cumulative flow, cycle time, throughput), Releases (a release's or milestone's progress) and Workload (distribution, created vs resolved). Count or estimate; flow and workload take the view bar's filter. The tab and its choices are in the address, so the URL shares the report |
| `hzn-chart-line`, `hzn-chart-area`, `hzn-chart-bar`, `hzn-chart-scatter`, `hzn-sparkline` | SVG charts (`components/charts.js`): series by tone on the LDS chart tokens, a legend from two series, a tooltip and crosshair on hover, a table view for every chart, a dot for a lone value ([reports.md](reports.md#charts)) |
| `hzn-roadmap` (view) | `/w/{ws}/roadmap`: the workspace roadmap ([Roadmap](#roadmap)) |
| `hzn-milestones` (view) | `/w/{ws}/milestones`: every milestone the user sees, by date |
| `hzn-milestone-list` | Milestones with progress and linked issues; create, edit, link issues (by key) and delete them |
| `hzn-release-dialog`, `hzn-milestone-dialog` | Create and edit releases; create, edit and link milestones |
| `hzn-iteration-dialog` | Plan or edit an iteration (name, goal, dates, prefilled with the suggested next one); complete one, saying what the policy will do and where unfinished issues go |
| `hzn-table` (view) | `/w/{ws}/p/{key}/table`: the issues as a grid. Columns from the built-in fields and the project's custom fields (a picker in an `lds-popover`); headers sort through the query (`?sort=key:desc`). A saved table view keeps its columns (`display.columns`) and sort; without one, columns are remembered per project in the browser. Cells edit in place (Enter or double click; Enter or a choice saves with `If-Match`, Escape cancels; title, status, priority, assignee, estimate, dates and text, number, date, select and checkbox fields); arrow keys move between cells, Space selects the row. Selected rows take a status (by name, in each issue's workflow), a priority, an assignee or archiving, one request per issue, with a summary of what was skipped or failed |
| `hzn-timeline` (view) | `/w/{ws}/p/{key}/timeline`: Gantt of the project's issues ([Timeline](#timeline)) |
| `hzn-calendar` (view) | `/w/{ws}/p/{key}/calendar?mode=month\|week&date=`: issues on their due dates (a month cell shows three, then "+N more" opens that week), milestones and release dates, the iterations as strips across their days, non-working days shaded and holidays named; the week view adds the issues under way each day. Arrow keys move between days, Page Up/Down by month or week; dragging an issue to another day or `Alt+arrows` (a day, a week) moves its dates through `hzn-reschedule` |
| `hzn-reschedule` | The timeline's propose-then-confirm dialog, shared by the timeline and the calendar |
| `hzn-view-bar` | On the list, the board and the table: the saved views picker (save as new, save changes, delete; picking a view of another layout goes there), the filter as chips (an AND of conditions built field → condition → value in an `lds-popover`) and text search. The address carries `?view=` and, while it differs from the view's, `?filter=` (JSON); a filter switches the layout to `POST /issues/query` |
| `hzn-editor` | Block editor: descriptions, comments ([Block editor](#block-editor)) |
| `hzn-*-picker` | Status, priority, assignee, labels, estimate, iteration, release pickers: `lds-combobox` wrappers that load options from the store and apply mutations |
| `hzn-inbox` (view) | `/w/{ws}/inbox?state=unread\|all\|snoozed`: one row per followed issue with its latest event, reason (assigned, mentioned) and the count of unread updates; opening a row reads it; mark read or unread, snooze (an hour, until tomorrow, until next week 08:00) and archive. The sidebar's Inbox item shows the unread count, refreshed by `inbox` socket messages; `G` then `I` goes there. The bell on an issue subscribes or unsubscribes; email settings are on the account page |
| `hzn-settings-webhooks` (view) | `/w/{ws}/settings/webhooks[/{id}]` (Integrations): the webhooks; a dialog creates and edits them (name, URL, project, event types, active, new secret); the secret shows once; a webhook's page lists its deliveries (event, result, status, time, the body and response on demand) with a test ping and redelivery |
| `hzn-settings-git` (view) | `/w/{ws}/settings/git` (Integrations): what a merged pull request does (workspace setting), the forge connections (forge, name, address, token; a new one shows the webhook URL, the secret and the forge's setup steps once). The issue page lists its branches, pull requests and commits under Development |

## Shell readiness

Every route view is a custom element that takes its context from attributes
(`<hzn-board workspace="acme" project="HZN">`) and gets its services (`api`,
`store`, `router`, `i18n`, `shortcuts`, later `socket`) from a context object,
not from globals. A view calls `requestContext(this)` when connected: it
dispatches a composed `hzn-context` event that the nearest provider answers.
`hzn-app` provides the context in standalone mode (`provideContext`); a future
shell adapter would provide the same object.

## Timeline

`hzn-timeline` draws each issue from its start to its due date (a due date
alone is a one-day bar, dashed), in rows sorted by start, with issues
without dates last: a click in their row gives them that working day.
Arrows are `precedes` links (`GET /projects/{key}/dependencies`), red when
the successor starts too early for the lag. The project's non-working days
are shaded (holidays in the warning tone), its iterations form a band
under the dates, milestones and release dates are dashed markers, and
today is a solid one. Zoom is weeks, months or quarters (remembered in the
browser and in a saved timeline view).

Dragging a bar moves it, dragging an end moves that date; on a focused bar
`Alt+←/→` moves it by a working day and `Alt+Shift+←/→` its due date. Every
change first asks the server for a plan (`POST /issues/{key}/schedule`):
the issues the bar precedes that would start too early move to the
working day after it ends (plus the lag), keeping their length in working
days of their own project, never earlier. With nothing to move the change
is saved; otherwise a dialog lists each issue and its shift, with those
that can't move (no `issue.edit`, archived), and offers to move them all,
the issue alone, or nothing. Confirming applies the plan, each successor
at the version it was planned from.

## Roadmap

`hzn-roadmap` (sidebar: Roadmap) shows the top-level issues that have
children (epics and the like) of every project the user sees, grouped by
project (`POST /w/{ws}/roadmap`). A bar is the issue's own dates, or,
dashed, its children's span; its fill is the share of its children
completed (abandoned ones left out), and the row says how many of how
many. Milestones (the workspace's, and the projects' with their key) and
release dates are markers, today a solid line. Projects, the state (open,
all, closed) and words narrow it, kept in the address (`?projects=`,
`?state=`, `?q=`); zoom is months, quarters or years. A bar opens its issue.

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

The pieces, under `web/app/editor/`:

| Module | Role |
|--------|------|
| `schema.js` | Generated from `internal/document/schema.json` (`task gen:schema`) |
| `model.js`, `transform.js` | Nodes, positions, steps, mapping and rebase; the same as `internal/document`, checked against the shared corpus |
| `state.js` | Editor state (document, selection, marks for typed text) and transactions, which build steps |
| `commands.js` | Typing, deleting (characters, words, lines; joins at block edges), Enter (new block or list item, leaving an empty item), marks over exact ranges, block types, lists (wrap, retype, Tab/Shift-Tab following ProseMirror's sink and lift), quotes |
| `history.js` | Undo and redo of the user's own steps: typing groups, everything else is a step of its own; entries map through others' steps |
| `view.js` | Renders the document into the `contenteditable` element (reusing the DOM of unchanged nodes), maps DOM points to positions and back (through shadow roots with `getComposedRanges`), replays `beforeinput` events as commands, reads IME composition back once it ends |
| `render.js` | Read-only HTML of a document (comments, archived issues) |
| `rules.js` | Markdown shortcuts while typing: `#`–`###`, `-`, `1.`, `[]`, `>`, ```` ``` ````, `---` at a paragraph's start; `` `code` ``, `**bold**`, `~~strike~~`, `:emoji:` inline. Undo puts the typed text back |
| `clipboard.js` | Paste: HTML (other apps, the editor itself) and Markdown to the schema's nodes; unknown markup is dropped, images aren't fetched |
| `tables.js` | Insert tables; add and remove rows and columns; Tab and Shift-Tab between cells, a new row past the last |

`<hzn-editor>` puts them together with a small formatting toolbar, shown
while it has focus (with table tools inside a table). Typing `/` opens the
block menu (text, headings, lists, tasks, quote, callout, code, divider,
table, image), `@` suggests the project's people, an issue key (`HZN-`)
suggests issues from search, `:name` emoji; ⌘K edits the link of the
selection. Pasted or dropped images, and images picked from the menu,
upload as attachments of the issue and show inline. Descriptions save a moment after typing stops and
when the editor loses focus (until the step protocol); comments are
written and edited in it and posted whole.

This is the riskiest frontend component. It gets its own milestone and test
corpus ([roadmap.md](roadmap.md)).

## Accessibility and keyboard

- Every action is reachable by keyboard; the shortcut help (`?`) is generated
  from the shortcut registry. Issue list: `J`/`K` move, `Enter` peeks, `O`
  opens the page; anywhere in a workspace `C` creates an issue. Issue page:
  `S` status, `A` assignee, `P` priority, `L` labels, `E` title, `R` comment,
  `M` move, `⌘⇧C` copy link.
- ⌘K lists the commands of what is on screen: views register a group with
  `context.commands.set(id, { heading, items })` while they're shown (the
  issue page adds "Change status…", "Assign to…" and others). Typing an issue
  key offers to open it.
- The peek panel keeps focus on the list: `J`/`K` move the peek along.
- Board and backlog drag and drop have keyboard equivalents (move with
  `alt+arrow`, or via the command palette). In the backlog, `Alt+↑/↓`
  moves the current issue within its bucket and `Alt+Shift+↑/↓` to the
  bucket above or below.
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
