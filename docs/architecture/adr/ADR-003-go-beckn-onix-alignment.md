# ADR-003: Go + beckn-onix Alignment for Retail BAP Path

**Status:** Accepted  
**Date:** 2026-06-17  
**Deciders:** VaniSync architecture baseline

---

## Context

VaniSync-Beckn must interoperate with the **Beckn/ONDC retail network**. Reference implementations exist:

- [beckn/beckn-onix](https://github.com/beckn/beckn-onix) — signing, validation, adapter patterns
- [beckn/protocol-specifications-v2](https://github.com/beckn/protocol-specifications-v2) — retail action JSON shapes
- [beckn/starter-kit](https://github.com/beckn/starter-kit) — Docker sandbox for integration tests

The team chose **Go** for edge deployment (single binary, good SQLite ecosystem, Android cross-compile via gomobile if needed).

---

## Decision

1. Implement the SDK in **Go 1.22+** as module `github.com/<org>/vanisync-beckn`.
2. **Do not fork** beckn-onix; align with its **signature header format** and validation semantics in `internal/beckn`.
3. First domain action: retail **`confirm`** (BAP path).
4. Integration tests (Phase 4) use `docker/compose.starter-kit.yml` wrapping beckn/starter-kit.
5. HTTP relay via pluggable `RelayClient` interface for mock injection in tests.

---

## Consequences

**Positive**

- Shared vocabulary with ONDC automation tooling and community examples.
- Starter-kit sandbox reduces custom mock gateway work.
- Go stdlib `log/slog` and `crypto/ed25519` minimize dependencies.

**Negative**

- beckn-onix is not imported directly initially — some duplication of schema builders until shared module extracted.
- Retail-only scope means other Beckn domains need new action builders later.

---

## Alternatives Considered

| Alternative | Why rejected |
|-------------|--------------|
| Node.js SDK | Heavier runtime on low-end edge devices |
| Embed beckn-onix via subprocess | Operational complexity |
| Custom protocol subset | Fails ONDC compliance review |

---

## References

- [01-system-overview.md](../01-system-overview.md)
- [docker/compose.starter-kit.yml](../../../docker/compose.starter-kit.yml)
