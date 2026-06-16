# Stakeholders and Concerns — VaniSync-Beckn

**Document:** 02-stakeholders-concerns  
**Standard:** ISO/IEC/IEEE 42010  
**Status:** Draft (Phase 1 baseline)

---

## 1. Stakeholders

| Stakeholder | Role | Primary interest |
|-------------|------|------------------|
| **VLE / rural merchant** | Runs shop, confirms orders on phone or CSC terminal | Fast local confirmation, works without signal |
| **CSC operator** | Manages shared infrastructure in block HQ | Low support burden, clear sync status |
| **End customer (buyer)** | Places order via buyer app on Beckn network | Order honoured when merchant comes online |
| **Sync / platform engineer** | Integrates SDK, monitors relay health | Predictable retries, observable logs, testable invariants |
| **ONDC / Beckn compliance reviewer** | Validates protocol adherence | Correct retail action JSON, signature headers |
| **AI4Bharat / Bhashini integrator** | Provides Santali ASR | Stable adapter interface, no coupling to sync core |
| **VaniSync product owner** | Roadmap for Santhal Pargana rollout | Dumka pilot readiness, offline-first credibility |

---

## 2. Concern → Architecture Response Map

| Concern | Description | Architectural response |
|---------|-------------|------------------------|
| **Offline resilience** | Dumka blocks lose connectivity for hours | Local SQLite primary store; no sync in request path |
| **Data loss prevention** | Power cut mid-order must not lose sale | Atomic `local_orders` + `sync_queue` in one SQL txn |
| **Duplicate delivery** | At-least-once relay must not double-charge | Signed idempotency UUID on every outbox row |
| **Ordering** | Confirm before status update | FIFO dequeue (`ORDER BY created_at ASC`) |
| **Multilingual access** | Santali-first merchants | Voice ASR adapter; structured API fallback |
| **Beckn compliance** | Gateway rejects malformed payloads | Pre-sign payloads; beckn-onix header alignment |
| **Security on edge** | Keys on shared CSC devices | Ed25519 key manager (Vault deferred) |
| **Conflict on sync** | Same order edited offline and on server | OCC via `updated_at` metadata (v1 single-writer) |
| **Debuggability** | Field staff lack dev tools | `slog` structured logs; MCP SQLite for agents |
| **Provable correctness** | Regulators / funders ask “will it sync?” | TLA+ safety/liveness + Go refinement tests |

---

## 3. Architecture Drivers (Prioritized)

1. **Offline-first** — local commit is the source of truth until ACK from gateway.
2. **Atomic outbox** — non-negotiable; see [ADR-001](./adr/ADR-001-transactional-outbox.md).
3. **No blocking I/O in handlers** — business operations return after DB commit only.
4. **FIFO relay** — preserves causal order of merchant actions.
5. **Voice-ready, not voice-coupled** — ASR is an adapter; core library has no HTTP to Bhashini in v1.

---

## 4. Concern Views (42010 Viewpoint Mapping)

| Viewpoint | Document | Key concerns addressed |
|-----------|----------|------------------------|
| Context / overview | [01-system-overview.md](./01-system-overview.md) | EoI, scope, quality attributes |
| Structural | [03-structural-view.md](./03-structural-view.md) | Module boundaries, package deps |
| Behavioral (sync) | [04-sync-behavioral-view.md](./04-sync-behavioral-view.md) | Relay, retry, OCC, failure modes |
| Decision | [adr/](./adr/) | Rationale for major choices |

---

## 5. Assumptions and Constraints

**Assumptions**

- One primary writer per device (VLE phone or CSC terminal) in v1.
- Gateway supports idempotent POST via outbox UUID / signature.
- Device storage ≥ 64 MB free for SQLite and outbox backlog.

**Constraints**

- Go 1.22+ implementation target.
- Embedded SQLite (no server-grade Postgres on edge).
- Retail Beckn domain only for first release.

---

## 6. Open Questions (Tracked Outside This Doc)

| ID | Question | Owner |
|----|----------|-------|
| OQ-1 | Bhashini Santali model availability for Ol Chiki | Voice integrator |
| OQ-2 | ONDC registry credentials per CSC vs per merchant | Compliance |
| OQ-3 | Maximum outbox backlog before operator warning | Product |
