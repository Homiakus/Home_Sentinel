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
- Stage 11.1 — incident operator read model projects committed ADGO state instead of maintaining a second FSM.
- Stage 11.2 — timeline is sourced from durable ordered history and credential-like fields are redacted.
- Stage 11.3 — `Explain` and execution diagnostics are exposed through Home Sentinel DTOs without exposing raw execution data.
- CI — format, vet, unit and race commands configured for every push to main/PR.

## Next

1. Observe/fix CI result when the connector exposes the push workflow run.
2. Add callback authentication/replay hardening and explicit threat model.
3. Add benchmark/performance budgets and release gates.
