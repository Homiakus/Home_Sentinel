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
- Stage 8.1 — door desired-state workflow with stable operation IDs and read/verify boundaries.
- Stage 8.2 — unlock requires high-risk human approval; ambiguous door I/O uses durable reconciliation.
- Stage 8.3 — siren activation has a mandatory bounded timer and an idempotent ensure-disabled compensation.
- Stage 8.4 — cancellation/manual override invokes compensation, so the safe terminal physical state is siren-off.
- Stage 9 — bounded temporal correlation with dedup, lateness rules, cross-camera context and concurrency coverage.
- CI — format, vet, unit and race commands configured for every push to main/PR.

## Next

1. Observe/fix CI result when the connector exposes the push workflow run.
2. Add health/recovery workflows.
3. Add observability/API and operator read model.
4. Add security hardening and performance gates.
