# 0010. Projects own issues; teams are permission groups

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

Linear has teams own issues, with cross-team projects. Jira and OpenProject
have projects own issues, workflows and sprints.

## Decision

Follow Jira/OpenProject: a **project** owns its issues, iterations, releases,
configuration overrides and key prefix (`HZN-42`). **Teams** are groups of
users used to grant roles. Sub-projects are out of scope for v1.

## Consequences

- Issue keys, sprints and workflows are per project, which is familiar to
  Jira users.
- Cross-project work is handled through views, milestones and the workspace
  roadmap rather than a team-owned backlog.
