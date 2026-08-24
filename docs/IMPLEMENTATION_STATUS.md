# Implementation status

## Completed

- Stage 0 — architecture boundary, ADR, detailed plan.
- Stage 1 — Go baseline, event/incident/artifact contracts and unit tests.
- Stage 2 — gateway contracts with desired-state and explicit effect semantics.
- Stage 3 — Camera lifecycle implemented with ordinary Axiom behind an adapter service.
- Stage 4 — ADGO Incident graph, typed activities, StartOrLoad dedup, durable waits and duplicate-signal tests.
- Stage 5.1 — `OpenProduction` wiring; Pebble default and memory test backend.
- Stage 5.2 — local Drive and long-lived Serve use ADGO Engine worker semantics.
- Stage 5.3 — Pebble close/reopen regression test verifies incident continuation without duplicate notification.
- Stage 6.1 — risk inputs include detector confidence, identity, alarm mode, entry state, dwell and cross-camera continuity.
- Stage 6.2 — pure deterministic `risk-v2` scorer with versioned thresholds and contribution breakdown.
- Stage 6.3 — low/medium/high routing is an explicit ADGO decision node.
- Stage 6.4 — deterministic/boundary/invalid-input risk tests.
- Stage 7 — high/critical incidents enter an explicit durable `NodeHuman`; approve/reject/abort paths are graph outcomes and actor/reason/payload are persisted by ADGO.
- CI — format, vet, unit and race commands configured for every push to main/PR.

## Next

1. Observe CI result through GitHub UI/API when available to the connector and fix any integration failures found there.
2. Add physical lock/siren desired-state activities and ambiguous-side-effect reconciliation tests.
3. Add multi-sensor temporal correlation window.
4. Add health/recovery workflows.
5. Add observability/API, security hardening and performance gates.
