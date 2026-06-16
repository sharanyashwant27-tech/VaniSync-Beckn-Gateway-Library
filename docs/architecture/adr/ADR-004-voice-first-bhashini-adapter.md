# ADR-004: Voice-First UX via Bhashini Adapter (Interface Now, Wire Later)

**Status:** Accepted  
**Date:** 2026-06-17  
**Deciders:** VaniSync architecture baseline

---

## Context

Many merchants in Dumka prefer **Santali** (Ol Chiki script) over Hindi or English for order entry. Typing on low-end phones is slow and error-prone. National stack options include:

- **Bhashini** ASR/TTS APIs
- **AI4Bharat IndicConformerASR** for low-resource languages

Voice input must not block offline order capture: ASR may require network, but **local order commit must succeed** even if ASR fails (fallback to structured form).

---

## Decision

1. Define `internal/voice.ASRProvider` interface with `Transcribe(ctx, audio) (text, error)`.
2. Ship **`StubASRProvider`** returning a fixed Santali dev transcript for local testing.
3. Map transcript → Beckn intent in `pkg/vanisync` via `ConfirmOrderFromVoice` (NL mapping stub in v1).
4. **No HTTP calls to Bhashini** inside core outbox/sync packages — voice is an optional upstream adapter.
5. Real Bhashini wiring deferred to Phase 4 after retail confirm path is stable.

---

## Consequences

**Positive**

- Voice UX can be demoed in Dumka pilot without waiting for ASR production keys.
- Core sync invariants remain testable without audio fixtures.
- Swappable ASR provider for Ol Chiki model updates.

**Negative**

- v1 voice path is not production-ready for Santali recognition accuracy.
- Intent mapping quality depends on future NL→Beckn layer (MCP-Beckn integration deferred).

---

## Alternatives Considered

| Alternative | Why rejected |
|-------------|--------------|
| Hard-code Bhashini HTTP in Client | Couples offline core to ASR availability |
| Text-only v1 | Excludes primary user persona in Santhal Pargana |
| On-device ASR model in v1 | Model size and training pipeline not ready |

---

## References

- [02-stakeholders-concerns.md](../02-stakeholders-concerns.md)
- [01-system-overview.md](../01-system-overview.md) — voice layer in context diagram
