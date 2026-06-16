# ADR-001: Transactional Outbox over Direct API Calls

**Status:** Accepted  
**Date:** 2026-06-17  
**Deciders:** VaniSync architecture baseline

---

## Context

Rural retail operators in Dumka often confirm orders while **offline or on flaky 2G**. If the SDK POSTs directly to a Beckn gateway inside the business handler:

- Failed requests leave ambiguous state (order taken locally but not recorded durably).
- Retries from the UI can duplicate orders.
- Blocking HTTP calls freeze the merchant UI for tens of seconds.

The Beckn/ONDC path requires signed, idempotent messages, but the edge device cannot assume synchronous gateway availability.

---

## Decision

Use the **transactional outbox pattern**:

1. Every domain write (`local_orders`) and outbox insert (`sync_queue`) occur in **one SQLite transaction**.
2. Handlers **never** perform synchronous network I/O.
3. A background **sync engine** relays outbox rows **FIFO** when connectivity is detected.
4. Each outbox row carries a **pre-computed Ed25519 signature** and **idempotency UUID** before insert.

---

## Consequences

**Positive**

- Orders are durable the moment the merchant sees confirmation.
- Retry semantics are centralized in the sync engine.
- Formal model (`VaniSyncOutbox.tla`) can prove safety (no orphans) and liveness (eventual consistency).

**Negative**

- Eventual consistency — gateway may lag minutes to hours behind local state.
- Requires outbox table maintenance and monitoring of `FAILED` rows.
- Slightly higher storage on device for queued payloads.

**Compliance**

- Enforced in `.cursor/rules/vanisync-beckn.mdc` and refinement tests in `test/refinement/`.

---

## Alternatives Considered

| Alternative | Why rejected |
|-------------|--------------|
| Direct HTTP in handler with local cache | Crash between HTTP and cache loses consistency |
| Message broker (Redis/RabbitMQ) on edge | Operational burden for CSC devices |
| Write-ahead log without outbox table | Harder to query/debug; duplicates Beckn payload storage |

---

## References

- [04-sync-behavioral-view.md](../04-sync-behavioral-view.md)
- [specs/VaniSyncOutbox.tla](../../../specs/VaniSyncOutbox.tla)
