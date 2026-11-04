# 0023. Secrets at rest sealed with SECRET_KEY

- **Status**: Accepted
- **Date**: 2026-11-04

## Context

Webhooks sign their deliveries with a secret Horizon must read back; Git
forge connections keep API tokens; OIDC providers keep client secrets. A
database dump, a backup or a read-only SQL session must not hand them out.
Instances rotate keys, and operators must be able to replace a leaked key.

## Decision

Secrets are sealed with AES-256-GCM under `SECRET_KEY` (32 random bytes)
in `internal/platform/secrets`. A sealed value reads
`v1.<key id>.<base64url(nonce ‖ ciphertext)>`; the key id is the first
8 hex digits of the key's SHA-256, so a value names the key that sealed it
without revealing it. Each value is bound to its place (associated data such
as `webhooks:<id>`): copied to another row, it no longer opens.

Rotation: the new key goes to `SECRET_KEY`, the old one to
`SECRET_KEY_PREVIOUS`. Old values still open; new and changed ones are
sealed with the new key; `horizon admin rekey` reseals the rest, after which
the old key can go.

Secrets that only need checking (session and API tokens, invitation and
reset links) stay hashed, as before; sealing is for secrets Horizon must use.

## Consequences

- Losing `SECRET_KEY` (and its previous keys) means re-entering every
  sealed secret; the operations guide says so.
- Every module that stores such a secret seals on write, opens on use, and
  takes part in `admin rekey`.
- Domain events and API answers never carry sealed or plain secrets; a
  webhook's secret is shown once, when created or rotated.

## Alternatives considered

- **pgcrypto in SQL**: the key would travel in queries and appear in
  database logs and `pg_stat_statements`.
- **An external KMS or Vault**: a dependency every instance would need;
  instances can still keep `SECRET_KEY` in their own secret store.
- **One key, no ids**: rotation would need a stop-the-world re-encryption.
