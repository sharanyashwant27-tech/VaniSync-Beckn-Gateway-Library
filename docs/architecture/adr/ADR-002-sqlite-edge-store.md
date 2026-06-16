# ADR-002: SQLite as Primary Store on Edge

**Status:** Accepted  
**Date:** 2026-06-17  
**Deciders:** VaniSync architecture baseline

---

## Context

VaniSync-Beckn targets **low-cost Android phones and CSC terminals** in Santhal Pargana. Infrastructure assumptions:

- No co-located PostgreSQL or Redis on the device.
- Storage is limited but sufficient for thousands of orders (JSON payloads ~2–10 KB each).
- Operators need **offline reads** of pending/synced orders without cloud dependency.

The library must support atomic transactions for the outbox pattern (ADR-001).

---

## Decision

Use **embedded SQLite** as the sole primary store on the edge:

- Driver: `modernc.org/sqlite` (pure Go, no CGO) preferred for cross-compilation to ARM Android.
- Schema managed via `migrations/*.sql` applied at startup.
- Database file default: `./data/vanisync.db` (configurable).
- WAL journal mode enabled for crash safety and concurrent read during sync.

---

## Consequences

**Positive**

- Single-file deployment; easy backup/copy for support.
- ACID transactions natively support atomic outbox writes.
- MCP SQLite server can inspect live state during development.

**Negative**

- Not suitable for multi-device shared writes (single-writer model assumed in v1).
- Large backlogs require occasional vacuum/archival policy (future ops concern).
- Server-side OCC triggers live on gateway Postgres, not on edge SQLite.

---

## Alternatives Considered

| Alternative | Why rejected |
|-------------|--------------|
| PostgreSQL on LAN at CSC | Not always available; adds ops complexity |
| BoltDB / bbolt | Weaker ad-hoc querying for support staff |
| Room (Android-only) | Locks SDK to Kotlin; project chose Go |

---

## References

- [03-structural-view.md](../03-structural-view.md)
- [migrations/001_initial.sql](../../../migrations/001_initial.sql) (planned)
