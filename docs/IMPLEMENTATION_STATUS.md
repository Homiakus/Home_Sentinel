# Implementation status

## Completed

- Stage 0 — architecture boundary, ADR, detailed plan.
- Stage 1 — Go baseline, event/incident/artifact contracts and unit tests.
- Stage 2 — gateway contracts with desired-state and explicit effect semantics.
- Stage 3 — Camera lifecycle implemented with ordinary Axiom behind an adapter service.
- Stage 4.1 — typed incident trigger is the initial durable fact.
- Stage 4.2 — ADGO Incident v1 graph: normalize -> correlate -> assess -> notify -> wait -> archive.
- Stage 4.3 — activities registered; risk v1 is deterministic and media stays out of execution data.
- Stage 4.4 — durable owner-response wait and event deduplication wired.
- Stage 4.5 — plan compile, StartOrLoad dedup, wait/resume and duplicate signal tests added.
- Stage 5.1 — `OpenProduction` wiring added; default config uses Pebble, tests use memory backend.
- Stage 5.2 — local Drive and long-lived Serve entrypoints use ADGO Engine worker semantics.
- Stage 2.4 — idempotent fake notifier contract test added.

## Next

1. CI/race gates and repository verification.
2. Durable Pebble reopen/crash tests.
3. Risk policy v2 with explainable feature contributions.
4. ADGO human-in-the-loop for high-risk paths.
5. Physical lock/siren reconciliation workflows.
