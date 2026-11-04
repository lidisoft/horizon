# 0024. Git forge integration: webhooks in, one comment out

- **Status**: Accepted
- **Date**: 2026-11-04

## Context

Teams want branches, pull requests and commits shown on the issues they
name, issues finished when their pull request merges, and the issue status
visible from the pull request. Horizon supports GitHub, GitLab and Forgejo,
hosted or self-managed, with as little setup per instance as possible.

## Decision

- **Inbound webhooks per connection.** A workspace connects a forge with
  its address, an optional token and a secret Horizon generates; the forge's
  repository, organisation or group webhook posts to
  `/api/v1/hooks/vcs/{connection}`. No OAuth app or GitHub App: the same
  setup works on every forge, hosted or not. Deliveries are verified with
  each forge's scheme and processed in the request: links are written and
  events appended in one transaction.
- **Links by issue key** in branch names, commit messages and pull request
  titles, descriptions and source branches.
- **Merges move issues along the workflow**, as an automation with no
  actor: the fewest allowed transitions to the configured status
  (`git_merge_status`, off by default), each step's rules applied, one
  status event. The workflow still decides how an issue may move.
- **Write-back is one living comment per pull request**, listing its issues
  and their status, written by a worker job when its content changes, edited
  in place. The token needs only comment rights.
- **Tests** run a real Forgejo in a container (ADR-0022); GitHub and GitLab,
  which can't run locally (or only in a multi-gigabyte image), are tested
  with deliveries signed like theirs and a fake of the comment API.

## Consequences

- One webhook per repository (or organisation or group) to set up by hand;
  the settings page gives the URL, the secret and the events per forge.
- Issues move without anyone's permission when a pull request merges, which
  is why it is off by default and stays within the workflow's transitions.
- Commit statuses, check runs, reviews and repository listing are not
  supported; each would need more token rights and more API per forge.

## Alternatives considered

- **A GitHub App**: better setup on GitHub, but GitHub-only and needs an app
  registration per instance.
- **Moving issues directly to the target status**: simpler, but would skip
  the workflow's validators and post-actions.
- **A comment per event**: noisy on the pull request; one living comment
  stays readable.
