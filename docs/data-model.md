# Data model

PostgreSQL schema outline. Authoritative definitions live in
`internal/migrations/`; this page explains intent. Conventions:

- Primary keys are **UUIDv7** (`uuid`), generated in Go.
- Every workspace-owned table has `workspace_id` as its first column; it leads
  every index.
- `created_at`, `updated_at` are `timestamptz`. Aggregates have a `version int`
  for optimistic concurrency.
- Foreign keys use `ON DELETE CASCADE` along ownership lines, so a hard delete
  of an aggregate removes what it owns.
- Names are snake_case and plural.

## Identity and access

| Table | Key columns |
|-------|-------------|
| `users` | email (unique, citext), name, password_hash (argon2id PHC string, nullable), locale, timezone (nullable = the browser's), is_instance_admin, deactivated_at |
| `sessions` | user_id, token_hash (SHA-256), last_seen_at, expires_at (sliding), user_agent, ip, auth_method (password, magic_link, oidc) |
| `password_resets` | user_id, token_hash, expires_at, used_at |
| `auth_throttle` | key (`email:…` or `ip:…`), failures, window_start — sign-in throttling shared by replicas |
| `magic_links` | user_id, token_hash, nonce_hash (the requesting browser's cookie), expires_at, used_at |
| `oidc_providers` | workspace_id (nullable = instance-wide), slug (in URLs), name, issuer, client_id, client_secret (sealed), email_domains (allowlist), provision_role_id (just-in-time accounts), enabled |
| `user_identities` | provider_id, subject, user_id — an account's identity at a provider |
| `user_identities` | user_id, provider_id, subject |
| `api_tokens` | user_id, name, token_hash (SHA-256), hint, scope (`read` < `write` < `admin`), last_used_at, expires_at (nullable = never) |
| `workspaces` | slug (unique; lower-case letters, digits, single hyphens), name, time_zone, settings JSONB |
| `workspace_members` | workspace_id, user_id, status (active, suspended), joined_at, suspended_at — people who joined |
| `invitations` | workspace_id, email (citext), token_hash, invited_by, team_ids, role_id (given on acceptance), sent_at, expires_at (7 days), accepted_at/by, revoked_at — people invited but not joined; at most one open per email |
| `teams` / `team_members` | workspace_id, name (citext, unique per workspace), description / team_id, user_id (leaving the workspace leaves its teams) |
| `roles` | workspace_id, name (citext, unique per workspace), description, builtin (admin, facilitator, contributor, viewer, guest; null = custom), permissions text[], position |
| `role_grants` | workspace_id, role_id, user_id or team_id (exactly one; foreign keys remove grants with the member or team), project_id (nullable = workspace scope), granted_by; one per subject and scope |

## Projects and configuration

| Table | Key columns |
|-------|-------------|
| (settings) | `workspaces.settings` and `projects.settings` JSONB, one key per setting: `estimation` = `{kind, values}`; a project key overrides the workspace's |
| `projects` | workspace_id, key (unique per workspace, 2–10 upper-case letters or digits), name, description, colour (named swatch), lead_id, visibility (workspace, private), settings JSONB (overrides), issue_seq bigint, archived_at |
| `project_key_aliases` | workspace_id, project_id, old_key — former keys; current keys and aliases share one namespace per workspace |
| `issue_types` | workspace_id, project_id (nullable = workspace), name, icon, colour, workflow_id, parent_rule (any, none, required), allowed_parent_type_ids (empty = any), position, copied_from |
| `workflows` | workspace_id, project_id (nullable), name, copied_from, version, draft JSONB (nullable: statuses, transitions and rules being edited), draft_version, draft_updated_by, draft_updated_at, published_at |
| `status_categories` | workspace_id, project_id (nullable = workspace), name, colour, position, outcome (`unstarted`, `started`, `completed`, `abandoned`) |
| `statuses` | workflow_id, name, category_id, position, is_initial — the published version |
| `transitions` | workflow_id, from_status_id (nullable = any), to_status_id, name, rules JSONB |
| `custom_fields` | workspace_id, project_id (nullable), name, type (fixed once created), description, options JSONB (select options with ids, names and colours; number unit and precision), position |
| `issue_type_fields` | workspace_id, issue_type_id, field_id, required, position |
| `config_disables` | project_id, definition_type, definition_id — inherited definitions a project switched off |
| `labels` | workspace_id, project_id (nullable), name (unique per scope, case-insensitive), colour, description |
| `non_working_dates` | workspace_id, project_id (nullable), date, name |

## Work items

| Table | Key columns |
|-------|-------------|
| `issues` | workspace_id, project_id, number, type_id, title, description (plain text of its document), status_id, priority, rank (text, collate "C"), estimate, assignee_id, reporter_id, parent_id, iteration_id, release_id, start_date, due_date, fields JSONB, started_at, resolved_at, archived_at, version |
| `issue_key_aliases` | workspace_id, project_id, number, issue_id — keys left behind by moves; a former project key resolves to the project first (`project_key_aliases`), then here |
| `issue_labels` | issue_id, label_id |
| `issue_links` | workspace_id, source_id, target_id, type (`relates_to`, `blocks`, `duplicates`, `precedes`), lag_days (`precedes` only) — inverse types (`blocked_by` …) are the same row read from its target |
| `comments` | workspace_id, issue_id, author_id, body_doc JSONB (a single-author block document), body (its plain text), edited_at, version |
| `attachments` | workspace_id, issue_id, uploader_id, filename, content_type (sniffed), size, object_key, state (pending, ready), completed_at |
| `iterations` | workspace_id, project_id, name, goal, state (`planned`, `scheduled`, `active`, `completed`, `archived`), start_date, end_date, started_at, completed_at, archived_at, completion JSONB (`{policy, rollover_to}`), version; `issues.iteration_id` references it (`ON DELETE SET NULL`) |
| `releases` | workspace_id, project_id, name (unique per project, ignoring case), description, state, start_date, release_date, released_at, archived_at, version; `issues.release_id` references it (`ON DELETE SET NULL`) |
| `milestones` | workspace_id, project_id (nullable), name, description, date, version |
| `milestone_issues` | milestone_id, issue_id |

Notable indexes:

- `issues (workspace_id, project_id, number)` unique — issue keys.
- `issues (workspace_id, project_id, status_id, rank)` — board columns and lists.
- `issues USING gin (fields jsonb_path_ops)` — custom field filters.
- `issues USING gin (setweight(to_tsvector('simple', title), 'A') || setweight(to_tsvector('simple', description), 'B'))`
  and `comments USING gin (to_tsvector('simple', body))` — full-text search
  (expression indexes: no `tsvector` column).
- `issues USING gin (title gin_trgm_ops)` — fuzzy and prefix matching for ⌘K
  (requires the `pg_trgm` extension).

The issue number comes from `UPDATE projects SET issue_seq = issue_seq + 1 …
RETURNING issue_seq` in the creating transaction, so numbers are gap-free per
project in practice and never reused.

**Rank** uses fractional indexing (base-62 strings): inserting between two
issues generates a key between theirs, so only the moved row changes.
Appending or prepending steps the first or last digit, so keys stay short
when issues pile up at either end; a rebalance job normalises keys that grow
past a length threshold in the middle.

An issue's description is a block document in `documents` (one per issue,
versioned for collaborative editing); `issues.description` keeps its plain
text for search and text clients. Issues and comments written before
documents existed have no document row (or `body_doc`): their text reads
as one paragraph per line, and the first write stores a document.
`started_at` is set the first time the issue
enters a started category; `resolved_at` while it sits in a completed or
abandoned one.

## Documents and collaboration

| Table | Key columns |
|-------|-------------|
| `documents` | workspace_id, issue_id (unique: the issue's description), version bigint (steps applied), content JSONB (snapshot at snapshot_version), described_version (covered by the last `IssueDescribed`) |
| `document_steps` | document_id, version (the batch applied to it), client_id, user_id, steps JSONB, created_at — primary key (document_id, version) |

Steps older than the latest snapshot minus a retention window are compacted
away. See [realtime.md](realtime.md).

## Events and integration

| Table | Key columns |
|-------|-------------|
| `events` | id (UUIDv7), workspace_id (null = instance-level), aggregate_type, aggregate_id, aggregate_version, type, actor_id (null = system), occurred_at, payload JSONB, published_at |
| `purges` | workspace_id, aggregate_type, aggregate_id, tombstone_id, objects text[], attempts — the `purge` job's queue after hard deletes |
| `subscriptions` | workspace_id, issue_id, user_id, reason (creator, assignee, mention, commenter, manual) |
| `inbox_items` | workspace_id, user_id, issue_id (one row each), reason (assigned, mentioned, subscribed), event_id (latest), first_event_id (oldest unread), unread_count, updated_at, read_at, snoozed_until, archived_at, emailed_event_id |
| `notification_settings` | user_id, email (unread, off), digest, digest_sent_at — no row: the defaults |
| `webhooks` | workspace_id, project_id (nullable = the workspace), name, url, secret (sealed), event_types text[] (empty = every event), active |
| `webhook_deliveries` | webhook_id, event_id, event_type, body JSONB, redelivery_of, state (pending, succeeded, failed), attempts, status_code, response_excerpt, error, duration_ms, next_attempt_at, delivered_at — one per endpoint and event, kept 30 days |
| `vcs_connections` | workspace_id, provider (github, gitlab, forgejo), name, base_url, webhook_secret (sealed), token (sealed, nullable), last_event_at |
| `vcs_links` | issue_id, connection_id, kind (branch, pull_request, commit), repository, external_id (branch, number, hash), url, title, state, author |
| `vcs_comments` | connection_id, repository, number, forge_id, body, dirty, attempts, next_attempt_at — the comment Horizon keeps on each pull request |
| `views` | workspace_id, project_id (nullable), owner_id (nullable = shared), name, layout, query JSONB, display JSONB |
| `horizon_releases` | version, commit, migrated_at — each release that ran `migrate up` ([operations.md](operations.md#upgrades)) |

Indexes on `events`: `(workspace_id, aggregate_type, aggregate_id, aggregate_version)`
for history; partial index on `(id) WHERE published_at IS NULL` for the relay.

## Reports

Projections maintained by the `reports` worker ([reports.md](reports.md)).
They hold ids, outcomes, weights and timestamps only.

| Table | Key columns |
|-------|-------------|
| `report_status_intervals` | workspace_id, project_id, issue_id, status_id, category_id, outcome, entered_at, left_at (null = current) |
| `report_issue_milestones` | workspace_id, issue_id, project_id, created_at, first_started_at, completed_at (start of its last run of completed stays; null when not completed), abandoned_at (likewise) — for lead and cycle time |
| `report_iteration_scope` | workspace_id, iteration_id, issue_id, added_at, removed_at (null: still in it) — one row per stay |
| `report_estimates` | workspace_id, issue_id, weight (null: no estimate), valid_from, valid_to |
| `events.reported_at` | The projector's queue: events applied (null: not yet) |

## Hard delete

Deleting an issue, in one transaction:

1. Handle children as requested (`reparent`, or `cascade`: repeat these steps
   for each descendant, deepest first); remove it from its iteration (emits
   `IterationLoaded`). Linked issues record `IssueUnlinked`.
2. `DELETE FROM issues` — cascades remove labels, links, key aliases,
   comments, documents, steps, attachment rows, subscriptions, inbox items,
   VCS links.
3. `DELETE FROM events WHERE aggregate_type='issue' AND aggregate_id=$1`, and
   delete `webhook_deliveries` that referenced those events
   (`eventlog.Tombstone`).
4. Insert the tombstone `IssueDeleted` event, and a `purges` row naming the
   aggregate, the tombstone and the object keys.

The `purge` job then waits until the relay has published the tombstone,
purges the JetStream subject `horizon.<ws>.issue.<id>.*` up to it (the
tombstone stays) and deletes the S3 objects.

Deleting a project runs the same per issue, without per-issue tombstones,
scrubs the history of its configuration overrides and grants, and leaves one
`ProjectDeleted` tombstone. Issues of other projects whose parent was there
become top-level. Events emitted by
**other** aggregates may mention the deleted issue by id (e.g. an iteration's
`IterationLoaded`); their payloads carry ids only, never issue content.
