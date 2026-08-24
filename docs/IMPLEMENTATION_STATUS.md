# Implementation status

## Current main baseline

- Stage 0 — architecture boundary, ADR and dependency direction.
- Stage 1 — Go/domain/event/incident/artifact baseline.
- Stage 2 — gateway desired-state/idempotency contracts and fakes.
- Stage 3 — Camera lifecycle on ordinary Axiom behind adapter service.
- Stage 4 — durable Incident ADGO graph with StartOrLoad and waits.
- Stage 5 — production ADGO wiring and Pebble reopen continuation test.
- Stage 6 — deterministic explainable risk-v2 and explicit routing.
- Stage 7 — durable owner human decisions with audit actor/reason/payload.
- Stage 8 — door/siren desired-state workflows, reconciliation and compensation.
- Stage 9 — bounded temporal correlation, duplicate/late/out-of-order handling.
- Stage 10 — bounded camera recovery graph and operator escalation.
- Stage 11 — operator read model, durable timeline, Explain and Diagnostics.
- Stage 12 — threat model; HMAC callbacks; key ID, iat/exp/maxTTL/skew; keyring rotation overlap; bounded replay guard.
- Stage 13 — correlation/risk/callback benchmarks; CI module verify, fmt, vet, unit, race and benchmark smoke.

## Important audit findings

1. P0: ADGO node ResourceKeys do not by themselves prove global same-device serialization across independent executions. Stage 20 adds durable dynamic resource admission.
2. P0: repository still has no committed `go.sum`; CI cache is intentionally disabled until Stage 14 creates the reproducible module lock.
3. P0: callback crypto exists, but authenticated ingress + RBAC + binding to actual waiting workflow node are not yet wired.
4. P0: plan/schema migration policy, backup/restore and release rollback were absent from the original plan and are now explicit stages.
5. P1: observability currently exposes read-model diagnostics but not metrics/SLO/exporters/runbooks.

## Next implementation order

1. Stage 20 — global physical resource admission/fencing.
2. Stage 14 — `go.sum`, module hygiene and supply-chain gates from a clean Go environment.
3. Stage 16/17 — typed config, secret/key loading, authenticated ingress and RBAC.
4. Stage 15 — plan/schema catalog and migration compatibility.
5. Stage 18/23/24 — backup/restore, exhaustive crash matrix, backpressure/degraded mode.
6. Stage 21/22/26 — real adapters, observability and target-hardware budgets.
7. Stage 27 — release/upgrade/rollback gates.

Full audited plan: `docs/AXIOM_IMPLEMENTATION_PLAN.md`.
