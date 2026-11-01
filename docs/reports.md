# Reports

v1 ships four families of reports and a fixed project overview page.
Configurable dashboards, forecasts and file exports come after v1.

## Common rules

- **Scope**: a report takes the same filter as a view ([api.md](api.md#querying-issues)):
  one project, several, a label, an assignee, a saved view. It covers the
  issues that **currently** match the filter; their history comes from the
  report projections. Sprint reports are scoped to one iteration, release
  reports to one release or milestone.
- **Metric**: every report toggles between **issue count** and **estimate**.
  Estimates use each scale value's numeric weight
  ([configuration.md](configuration.md#estimation-scales)). If the issues in
  scope use different scales, or a project's scale is `none`, the report falls
  back to count and says so.
- **Outcomes, not statuses**: reports reason on category outcomes
  (`unstarted`, `started`, `completed`, `abandoned`), so they work across
  projects with different workflows. Only `completed` counts as delivered;
  `abandoned` is closed but not delivered.
- **Time**: daily buckets use the project's time zone (workspace default),
  see [configuration.md](configuration.md#time-zone-and-working-calendar).
  Cross-project reports use the workspace time zone.
- **Permissions**: a report only includes issues the viewer can see.
- **Links**: a report's URL encodes its filter and options, so it can be
  bookmarked and shared with members who can see the same issues.

## Definitions

| Term | Definition |
|------|------------|
| Started | First entry into a category with outcome `started` |
| Completed | **Last** entry into a category with outcome `completed` (a reopened issue completes again) |
| Lead time | Created → completed |
| Cycle time | Started → completed. Issues never started have no cycle time |
| Throughput | Issues (or estimate) completed per period |
| WIP | Issues currently in a `started` category |

## v1 reports

### Sprint reports (per iteration)

| Report | Shows |
|--------|-------|
| Burndown | Remaining scope per day versus an ideal line from start to end date |
| Burnup | Completed scope and total scope per day; scope changes are visible as steps |
| Sprint report | At completion: **committed** (in the iteration when it started), **added** after start, **removed**, and how each ended — completed, abandoned, rolled over |
| Velocity | Committed versus completed per iteration, last N (default 7), with average |

Scope changes come from `IterationLoaded`; estimate changes during the sprint
from `IssueEstimated`. Both are reflected on the day they happen.

### Flow reports

| Report | Shows |
|--------|-------|
| Cumulative flow | Issues per outcome (or per category, within one project) per day |
| Cycle and lead time | Scatter plot per completed issue, with p50 / p85 / p95 lines |
| Throughput | Completed per week, with the period average |

### Release and milestone progress

Per release or milestone: scope by outcome now, and a burnup over time
(completed versus total). No projected date in v1.

### Workload and distribution

| Report | Shows |
|--------|-------|
| Distribution | Open issues (or estimate) grouped by assignee, type, priority, label, project or any select field |
| Created vs resolved | Issues created versus completed or abandoned per period |

## Project overview page

One fixed page per project, not customisable in v1:

| Block | Content |
|-------|---------|
| Active sprint | Goal, days left, burndown sparkline, scope by outcome |
| Releases and milestones | Next upcoming, with progress bars |
| Flow health | Throughput over the last 4 weeks, cycle time p50/p85, WIP by status |
| Activity and my work | Recent project activity; the viewer's open issues in the project |

## How reports are computed

A `reports` worker maintains projection tables from the domain events
([ADR-0021](adr/0021-report-projections.md)):

| Table | Fed by | Serves |
|-------|--------|--------|
| `report_status_intervals` | `IssueCreated`, `IssueStatusUpdated` (transitions, type changes, workflow migrations), `IssueMoved`, `StatusCategoryUpdated` | Cumulative flow, burndown and burnup (what was done when), created vs resolved |
| `report_issue_milestones` | The same, derived per issue | Lead and cycle time, throughput |
| `report_iteration_scope` | `IssueIterationChanged` (loading, unloading, rollover, iteration deletion) | Burndown, burnup, sprint report, velocity |
| `report_estimates` | `IssueCreated`, `IssueEstimated` | Every report measured in estimate, as it was on each day |

Columns are described in [data-model.md](data-model.md#reports). Committed,
added and removed scope, and how each issue ended (completed, abandoned,
rolled over), follow from the iteration's `started_at` and `completed_at`.
Status events carry the category and outcome of the status entered, and
`IssueEstimated` the scale's weight, as they were: workflows and scales
change later. Reports compute their daily series from these tables per
request; a daily rollup can come when long ranges need it.

The worker applies the PostgreSQL `events` table, in id order, a batch per
transaction that also sets the events' `reported_at`, so each event counts
exactly once. Batches run one at a time across worker replicas (an advisory
lock). Each JetStream event wakes it up, and a job every minute catches up
with whatever it missed, so it never depends on the stream's retention.

- Projections store ids, outcomes, estimate weights and timestamps only, never
  issue content.
- Queries join projections with the current `issues` row to apply the filter
  and permissions.
- A tombstone (`IssueDeleted`) removes the issue's projection rows.
- `horizon admin rebuild-reports` empties the projections, clears
  `reported_at` and replays the PostgreSQL `events` table (for new reports, or
  after a bug fix).
- A category's outcome edited (`StatusCategoryUpdated`) changes the outcome of
  every stay in its statuses, and the milestones of the issues concerned.

## API

`GET /api/v1/w/{ws}/reports/{report}` with the filter and options as query
parameters (the same encoding as the shareable URL: `filter` as JSON, lists
comma-separated), or `POST` with a JSON body. Responses are series and
tables; the SPA uses the same endpoints. Every response says its `metric`,
`metric_fallback` when estimate was asked but scales are `none` or differ,
and the `time_zone` its days are in. Daily series have one value per day,
read at the day's end (now for today), `null` for days to come.

| `report` | Scope | Response |
|----------|-------|----------|
| `burndown`, `burnup` | `iteration` | Days from start to end date: `remaining` (neither completed nor abandoned), `scope` (all but abandoned), `completed`, `ideal` (the open scope committed at the start, down an equal share each working day), `working` |
| `sprint` | `iteration` | Each issue: committed, added, removed, and how it `ended` (completed, abandoned, rolled_over, removed, open); `totals` |
| `velocity` | `projects` (one), `last` | Committed at the start and completed at the end of the last completed iterations; `average` |
| `progress` | `release` or `milestone` | `by_outcome` now, and `scope`/`completed` per day from the release's start date (else the first issue's creation). Today's issues count from their creation: who was targeted when isn't projected |
| `cfd` | `filter`, `projects`; `by` (outcome, or category in one project) | `series` per outcome stacked completed, started, unstarted, abandoned, per day (default: the last 30 days) |
| `cycle-time` | `filter`, `projects` | The issues completed (default: last 90 days) with `cycle_days` and `lead_days` (to a tenth); `cycle` and `lead` p50/p85/p95 |
| `throughput` | `filter`, `projects`; `period` week or month | Completed per week (Monday first; default the last 12) or month (the last 6); `average` over past periods |
| `created-resolved` | As throughput | `created` and `resolved` (completed or abandoned) per period |
| `distribution` | `filter`, `projects`; `group` | Open issues (unstarted or started, not archived) by assignee (default), type, priority, label (an issue in each of its labels), project or `cf:<select field id>`, largest first; `key` "" for none |

`GET /api/v1/w/{ws}/projects/{key}/overview` returns all overview blocks in one
response.

## Charts

Charts are hand-written SVG components (`hzn-chart-line`, `hzn-chart-bar`,
`hzn-chart-scatter`, `hzn-chart-area`, `hzn-sparkline`) styled with LDS
tokens. No charting library ([ADR-0006](adr/0006-spa-native-js-no-dependencies.md)).
Every chart has a table fallback for accessibility.

Colours come from the LDS chart tokens (since `v0.0.4`), never from intents or core
steps:

| Use | Token |
|-----|-------|
| Outcomes, stacked always in this order | `--lds-color-chart-completed`, `-started`, `-unstarted`, `-abandoned` |
| Ideal or target line (dashed) | `--lds-color-chart-reference` |
| Today marker | `--lds-color-chart-marker` |
| Scope changes, dependency arrows, notes | `--lds-color-chart-annotation` |
| Bar end radius, gap between segments | `--lds-chart-bar-radius`, `--lds-chart-segment-gap` |

`unstarted` is below 3:1 against the surface by design, so any chart using it
labels its values or offers the table view. Chart text uses `fg` roles; two or
more series always get a legend.

## After v1

- Configurable dashboards of widgets (reports, saved views, counters),
  personal or shared.
- Forecasts for releases and milestones.
- CSV export of report data; PNG / SVG chart export.
