# Implementation status

## Completed

- Stage 0 — architecture boundary, ADR, detailed plan.
- Stage 1 — Go baseline, event/incident/artifact contracts and unit tests.
- Stage 2 — gateway contracts with desired-state and explicit effect semantics.
- Stage 3 — Camera lifecycle implemented with ordinary Axiom behind an adapter service.
- Stage 4 — ADGO Incident graph, typed activities, StartOrLoad dedup, durable waits and duplicate-signal tests.
- Stage 5 — production wiring plus Pebble close/reopen continuation test.
- Stage 6 — explainable deterministic `risk-v2` and explicit low/medium/high ADGO routing.
- Stage 7 — durable high-risk owner decisions with persisted actor/reason/payload.
- Stage 8 — door and siren physical effects use desired state, idempotency/reconciliation, human gates and fail-safe compensation.
- Stage 9 — bounded temporal correlation with dedup, lateness rules, cross-camera context and concurrency coverage.
- Stage 10 — bounded camera recovery graph with verification and operator escalation.
- Stage 11 — operator read model, durable timeline, Explain and Diagnostics.
- Stage 12.1 — callback token authentication uses HMAC-SHA256, constant-time verification and expiry.
- Stage 12.2 — bounded replay guard is implemented as edge defense; ADGO SeenEvents remains durable semantic deduplication.
- Stage 12.3 — explicit threat model documents trust boundaries, authority and remaining deployment risks.
- Stage 13.1 — benchmark harness exists for correlation and risk hot paths.
- Stage 13.2 — CI runs module verification, format, vet, unit, race and benchmark smoke checks.

## Remaining production work

- Real camera/RTSP, Home Assistant, notification and hardware gateway adapters.
- Authenticated HTTP/TLS ingress, user authorization/RBAC and key rotation.
- Target-hardware benchmark baseline and numeric regression thresholds.
- Cross-platform/system integration tests with actual devices.
- Observe/fix the push CI run once GitHub connector exposes it; the available workflow-run wrapper filters to pull-request runs and currently returns no push run.
