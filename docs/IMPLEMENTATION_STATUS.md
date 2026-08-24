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
- Stage 8.1 — door control is a separate ADGO workflow using desired state, stable operation IDs, read-before-write and verify-after-write.
- Stage 8.1 — unlock requires an explicit high-risk `NodeHuman`; locking does not.
- Stage 8.3 — ambiguous door I/O maps to ADGO `FailureAmbiguousSideEffect` and durable `Reconcile:<node>` rather than blind retry.
- Stage 8.4 — tests cover automatic verification of an ambiguous-but-applied command and operator-authorized retry of an unresolved command.
- CI — format, vet, unit and race commands configured for every push to main/PR.

## Next

1. Observe/fix CI result when the connector exposes the push workflow run.
2. Add siren safety workflow with maximum activation duration and manual override.
3. Add multi-sensor temporal correlation window and incident merging.
4. Add health/recovery workflows.
5. Add observability/API, security hardening and performance gates.
