# 0008. WebSockets for realtime

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

Clients must see other users' changes immediately, show presence, and
exchange collaborative-editing steps with low latency in both directions.

## Decision

One **WebSocket** per tab carries domain-event pushes, presence and document
steps. Domain commands stay on the REST API so they have one implementation,
one permission path and are usable by integrations.

## Consequences

- Bidirectional channel available for editing and presence.
- Load balancers must support WebSocket upgrades and long-lived connections.
- Reconnect, replay and resync logic is needed on the client
  ([realtime.md](../realtime.md)).

## Alternatives considered

- **Server-Sent Events**: simpler for pushes, but collaborative editing needs
  a low-latency upstream channel too.
