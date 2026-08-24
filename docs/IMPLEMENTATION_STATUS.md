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
- Stage 10.1 — camera recovery is invoked as a stateful ADGO workflow, not for simple heartbeat polling.
- Stage 10.2 — network/stream probes are facts; deterministic decisions select healthy, reconnect or operator branches.
- Stage 10.3 — reconnect has bounded retry/idempotency and verification; unresolved recovery escalates to a durable operator decision.
- Stage 10.4 — operator retry is a single explicit second-attempt subgraph, avoiding an unbounded recovery cycle.
- CI — format, vet, unit and race commands configured for every push to main/PR.

## Next

1. Observe/fix CI result when the connector exposes the push workflow run.
2. Add observability/API and operator read model.
3. Add security hardening and performance gates.
