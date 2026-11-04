# Integrations

Horizon talks to other systems through outbound webhooks
([api.md](api.md#webhooks)) and Git forge connections, described here.

## Git forges

GitHub (github.com and Enterprise Server), GitLab (gitlab.com and
self-managed) and Forgejo (and Gitea, which sends the same payloads) are
supported. A workspace's integration managers (`workspace.integrations.manage`)
connect a forge in **Settings → Integrations → Git**; decisions are in
[ADR-0024](adr/0024-git-forge-integration.md).

### Connections

A connection holds the forge's web address, an optional access token and a
webhook secret Horizon generates. Creating it answers the webhook URL
(`{BASE_URL}/api/v1/hooks/vcs/{connection id}`) and the secret, shown once;
both go into a webhook on a repository, a GitHub organisation or a GitLab
group:

| Forge | Webhook settings | Events |
|-------|------------------|--------|
| GitHub | Content type `application/json`, secret | Pushes, Pull requests, Branch or tag creation and deletion |
| GitLab | Secret token | Push events, Merge request events |
| Forgejo | Type Forgejo, content type `application/json`, secret | Push, Create, Delete, Pull request |

Deliveries are checked against the secret (`X-Hub-Signature-256`,
`X-Forgejo-Signature`, `X-Gitlab-Token`); a delivery that fails the check is
refused with `401`. The secret and the token are sealed at rest
([ADR-0023](adr/0023-secrets-at-rest.md)) and rotated from the connection.

### Links

Issue keys are found in any case (`HZN-42`, `hzn-42-relay`), by a project's
current or former keys and an issue's former keys after a move:

| Forge event | Linked when the key is in | Link |
|-------------|---------------------------|------|
| Branch created, pushed to, deleted | The branch name | `branch`, state `open` or `deleted` |
| Commits pushed | The commit message (first 100 commits of a push) | `commit`, titled by the message's first line |
| Pull (merge) request opened, edited, closed, merged | Its title, description or source branch | `pull_request`, state `open`, `closed` or `merged` |

A pull request keeps the issues it named before, even when its title
changes. Each new link is an `IssueGitLinked` event on the issue, each
change of state an `IssueGitUpdated`, both without an actor (Horizon is
the actor shown). They appear in the issue's activity and the inbox (pull
requests only), and reach webhooks. `GET /w/{ws}/issues/{key}/git` lists an
issue's links; the issue page shows them under **Development**.

### Merged pull requests

When a pull request merges, the issues it names may move. The setting
`git_merge_status` ([configuration.md](configuration.md#catalogue)), at the
workspace with a project override (`/config/settings/git`), says where:

| Value | Does |
|-------|------|
| `@off` (default) | Nothing |
| `@completed` | Moves each issue to the nearest completed status of its workflow |
| A status name | Moves each issue to the status of that name, where its workflow has it |

The move follows the workflow: the fewest allowed transitions lead there (at
most six), each step's validators and children rule apply and its
post-actions run, and one `IssueStatusUpdated` records it with `by:
pull_request_merged`, the pull request's URL and the transitions taken. It
needs no permission or role. An issue no transition leads from, one a rule
holds back, and archived issues stay where they are.

### Pull request comment

With a token, Horizon keeps one comment on each pull request that names
issues: a table of the issues, linked to Horizon, with their status. It is
written when the pull request's links change and when one of its issues
changes status, title or key, edited in place afterwards, and posted again if
someone deletes it. The `vcs-comments` job writes due comments every few
seconds and retries failures (1 min to 2 h, eight tries).

| Forge | Token |
|-------|-------|
| GitHub | Fine-grained token with Pull requests: read and write (or a classic token with `repo`) |
| GitLab | Personal, group or project access token with the `api` scope |
| Forgejo | Access token with `write:issue` |

API calls go to `https://api.github.com` (or `{base}/api/v3` for
Enterprise Server), `{base}/api/v4` (GitLab) and `{base}/api/v1` (Forgejo).
They can't reach loopback, private or link-local addresses unless
`FORGE_ALLOW_PRIVATE=true` ([operations.md](operations.md#configuration)),
which a self-hosted forge on the instance's network needs.
