# 0007. LiDiSoft Design System for all UI

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

LDS (`github.com/lidisoft/design-system`) provides tokens, themes and Web
Components, embeddable in a Go binary.

## Decision

Horizon imports LDS as a Go module and serves its bundle at `/lds/`. All UI
uses LDS components and semantic tokens. Generic components Horizon needs are
contributed to LDS; Horizon-specific components are `hzn-*` custom elements
that follow LDS authoring rules.

## Consequences

- Visual consistency across LiDiSoft products; Linear-like look is achieved
  through LDS tokens and density, not a separate stylesheet.
- Horizon's progress depends partly on LDS (popover, tooltip, drawer, combobox
  are prerequisites).
