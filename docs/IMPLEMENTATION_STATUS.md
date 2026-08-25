# Implementation status

## Current main baseline

- Stage 0 — architecture boundary, ADR and dependency direction.
- Stage 1 — Go/domain/event/incident/artifact baseline.
- Stage 2 — gateway desired-state/idempotency contracts and fakes; external `Operation` requires execution + idempotency identity.
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
- Stage 16a — typed fail-closed configuration and secret-reference loading baseline exists.
- Stage 20a — single-writer durable resource reservation: Door, Siren and Camera Recovery reject a second non-terminal execution for the same physical resource; different resources remain parallel; terminal executions release ownership; concurrent Start race is covered.
- Stage 28 — Canonical Scenario Model: strict headless AST, stable scenario/revision/step identity, typed capability references, semantic flow nodes, strict decode, deterministic normalization and semantic digest, clone-to-draft, nested duplicate detection and fuzz baseline.
- Stage 29 — Capability Registry: versioned capability descriptors, entity/device binding, risk/permission/visibility metadata, typed schemas/UI hints, health without deletion, compatible resolution, dependency-protected removal, deterministic snapshot/digest and discovery filters.
- Stage 30a — Typed/temporal semantic baseline: canonical typed values and units, TypeRef capability schemas, static expression checking, first-class temporal AST, explicit timezone/DST/catch-up policy and deterministic wall-clock resolution.
- Stage 30b — Typed expression bindings: ActionStep and SubflowStep arguments upgraded from raw map[string]Value to map[string]Expr with normalization, reference validation, type inference, capability schema compatibility validation, default handling and unknown argument rejection.

## Scenario authoring audit

Canonical Scenario AST, Capability Registry and the typed/temporal semantic layer are now present as the headless product foundation. Runtime classification/compiler, Safety Compiler, immutable catalog, simulator and UI remain open.

Official product track: [`SCENARIO_SYSTEM_PLAN.md`](SCENARIO_SYSTEM_PLAN.md).

Stages 28-42:

- 28 — Canonical Scenario Model — **implemented baseline**;
- 29 — Capability Registry — **implemented baseline**;
- 30 — typed expressions + temporal semantics + argument bindings — **implemented baseline; execution lowering pending Stage 31/34**;
- 31 — automatic Scenario -> Axiom/ADGO compiler;
- 32 — mandatory Safety Compiler;
- 33 — immutable Draft/Validate/Simulate/Publish catalog;
- 34 — headless simulation/replay;
- 35 — authenticated Scenario API;
- 36 — Simple Builder;
- 37 — Advanced Flow Editor;
- 38 — Templates/Subflows;
- 39 — LLM authoring through structured AST only;
- 40 — Live Trace/Explain;
- 41 — mobile/adaptive authoring;
- 42 — scenario quality/security/release gates.

Scenario AST, UI and AI layer do not replace Axiom/ADGO and cannot bypass gateway/RBAC/resource ownership.

## Important audit findings

1. Stage 20a closes same-process/same-store cross-execution reservation, including human/reconciliation waits and restart reconstruction from durable executions. **Multi-process fencing is not claimed complete**: Stage 24 still requires a single-writer startup lock for v1 or a true distributed admission/fencing protocol before multi-writer topology is supported.
2. P0: repository still has no committed `go.sum`; CI cache is intentionally disabled until Stage 14 creates the reproducible module lock.
3. P0: callback crypto exists, but authenticated ingress + RBAC + binding to actual waiting workflow node are not yet wired in guaranteed `main` baseline.
4. P0: plan/schema migration policy, backup/restore and release rollback remain required production stages.
5. P1: observability currently exposes read-model diagnostics but not full metrics/SLO/exporters/runbooks.
6. Capability Registry deliberately treats provider offline as health/availability state, not deletion; scenario publication still requires compiler/catalog integration.
7. Capability removal fails closed without a dependency resolver and is blocked while entities or scenarios reference it.
8. Stage 30 currently defines and validates temporal semantics, including DST policy, but durable debounce/throttle/repeat/schedule execution is intentionally deferred to compiler/runtime lowering and simulator tests; it is not claimed as executed behavior yet.

## Next implementation order

Core production safety:

1. Stage 14 — `go.sum`, module hygiene and supply-chain gates from a clean Go environment; do not fabricate a lock file.
2. Stage 17 — authenticated ingress and RBAC; complete integration with Stage 16 config/secrets.
3. Stage 15 — plan/schema catalog and migration compatibility.
4. Stage 18/23/24 — backup/restore, exhaustive crash matrix, backpressure/degraded mode and explicit process topology gate.

Scenario product foundation can proceed in parallel where it does not weaken production gates:

5. Stage 31/32 — Scenario Compiler + Safety Compiler.
6. Stage 33/34 — immutable catalog + simulator.
7. Stage 35+ — authenticated API, then Simple Builder/Graph/Templates/Trace/AI.

Integration/release:

8. Stage 21/22/26 — real adapters, observability and target-hardware budgets.
9. Stage 27 + Scenario Stage 42 — release/upgrade/rollback and scenario release gates.

Index: `docs/PLAN_INDEX.md`.
Master production plan: `docs/AXIOM_IMPLEMENTATION_PLAN.md`.
Scenario plan: `docs/SCENARIO_SYSTEM_PLAN.md`.
