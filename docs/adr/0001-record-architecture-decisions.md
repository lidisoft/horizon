# 0001. Record architecture decisions

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

Horizon makes many structural choices up front (persistence, messaging,
frontend without dependencies). Future contributors, and AI assistants, need
to know why, so that changes are informed reversals rather than accidents.
The LiDiSoft Design System keeps a single `decisions.md`; with many more
decisions here, one file per decision is easier to reference and review.

## Decision

Record every architectural decision as an ADR in `docs/adr/`, numbered,
immutable once accepted, superseded by newer ADRs. Other documentation follows
the LDS layout (`AGENTS.md`, `docs/*.md`) and links to ADRs for the "why".

## Consequences

A PR that changes architecture includes an ADR. The index in
`docs/adr/README.md` must be kept current.
