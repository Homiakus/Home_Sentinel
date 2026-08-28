# Implementation status

## Status semantics

This file is a recorded snapshot, not an independent source of truth. Before selecting or closing work, run:

```text
go run ./cmd/sentinel-engloop reconcile --root . --strict
```

The authoritative execution protocol is `docs/engineering/ENGINEERING_LOOP.md`; intent comes from `docs/PLAN_INDEX.md` and the linked production/scenario plans, while completion requires executable evidence from the observed checkout.

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
- Stage 14b — dependency/supply-chain baseline is CI-enforced: committed `go.sum`, clean `go mod tidy`/`go mod verify`, reviewed immutable GitHub Action allowlist, pinned security tools, `govulncheck`, Trivy HIGH/CRITICAL qualification, CycloneDX module SBOM evidence and Dependabot. Release provenance/reproducibility/signing remains coupled to Stage 27 release qualification.
- Stage 16a — typed fail-closed configuration and secret-reference loading baseline exists.
- Stage 20a — single-writer durable resource reservation: Door, Siren and Camera Recovery reject a second non-terminal execution for the same physical resource; different resources remain parallel; terminal executions release ownership; concurrent Start race is covered.
- Stage 28 — Canonical Scenario Model: strict headless AST, stable scenario/revision/step identity, typed capability references, semantic flow nodes, strict decode, deterministic normalization and semantic digest, clone-to-draft, nested duplicate detection and fuzz baseline.
- Stage 29 — Capability Registry: versioned capability descriptors, entity/device binding, risk/permission/visibility metadata, typed schemas/UI hints, health without deletion, compatible resolution, dependency-protected removal, deterministic snapshot/digest and discovery filters.
- Stage 30a — Typed/temporal semantic baseline: canonical typed values and units, TypeRef capability schemas, static expression checking, first-class temporal AST, explicit timezone/DST/catch-up policy and deterministic wall-clock resolution.
- Stage 30b — Typed expression bindings: ActionStep and SubflowStep arguments use `map[string]Expr` with normalization, reference validation, type inference, capability-schema compatibility validation, default handling and unknown-argument rejection.
- Stage 31 — Scenario Compiler: pure multi-pass compiler pipeline (Normalize -> Validate -> BuildEnv -> Resolve -> TypeCheck -> Temporal -> SafetyAugment -> StaticConflict -> Classify -> Lower -> Manifest -> Digest), structured diagnostics with HS-SCN-xxx codes and explainable runtime classification.
- Stage 31a — Axiom Lowering: exact FSM lowering for simple, stateless reactive trigger-action flows.
- Stage 31b — ADGO Lowering: durable multi-step workflow generation (Activity, Wait, HumanApproval, Fork, Join, Subflow, Gate/Decision, Compensation, ResourceLock).
- Stage 32 — Safety Compiler: mandatory security boundary generating human approval gates, single-writer resource locks, read-before-write, verify-after-write, maximum-duration clamps and ensure-disabled compensation; separate User Intent Graph and System Graph representations.
- Stage 32b — Static Conflict Analysis: pre-publish detection of self-recursion, mutual recursion, conflicting desired states on the same resource, unreachable steps and potential trigger-action feedback loops.
- Stage 33 — Scenario Catalog: immutable draft/validate/simulate/publish lifecycle, optimistic concurrency ETag control, rollback without history mutation, audit logs and active-execution version pinning.
- Stage 33b — Dependency Index: bidirectional dependency graph (scenario <-> capabilities, entities, subflows, templates), fail-closed protection against active dependency removal.
- Stage 34 — Headless Simulator: pure simulation, replay and what-if analysis with virtual clock manipulation, hypothetical WOULD_EXECUTE side effects with zero real gateway calls and safety-node trace projections.
- Engineering loop baseline — `cmd/sentinel-engloop` + `internal/engloop` provide roadmap reconciliation, Work Packet validation, risk/gate planning, multidimensional constrained t-way edge-suite generation, critical mutation evidence validation and executable supply-chain self-verification.

## Scenario authoring audit

Canonical Scenario AST, Capability Registry, typed/temporal semantics, Scenario Compiler, Safety Compiler, Scenario Catalog and Headless Simulator are present as the headless product foundation. Scenario API and UI layer remain open.

Official product track: [`SCENARIO_SYSTEM_PLAN.md`](SCENARIO_SYSTEM_PLAN.md).

Stages 28-42:

- 28 — Canonical Scenario Model — **implemented baseline**;
- 29 — Capability Registry — **implemented baseline**;
- 30 — typed expressions + temporal semantics + argument bindings — **implemented baseline**;
- 31 — automatic Scenario -> Axiom/ADGO compiler — **implemented baseline**;
- 32 — mandatory Safety Compiler + static conflict analysis — **implemented baseline**;
- 33 — immutable Draft/Validate/Simulate/Publish catalog + dependency index — **implemented baseline**;
- 34 — headless simulation/replay — **implemented baseline**;
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
2. Stage 14 dependency hygiene and repository qualification are now executable gates: module lock/hygiene, reviewed Action SHA allowlist, pinned scanners, `govulncheck`, Trivy and module SBOM generation are proven on `main`. Release artifact provenance, reproducible-build evidence, checksums/signing and rollback qualification remain Stage 27 concerns rather than reasons to reopen dependency hygiene.
3. Stage 17 is **PARTIAL**: session authentication, CSRF, security headers, capability RBAC and callback HMAC/replay primitives exist, but authenticated callback ingress bound to the actual waiting workflow node, authorization-decision audit, explicit user/service/system principal semantics and end-to-end exactly-once resume are not yet proven.
4. Plan/schema migration policy, backup/restore and release rollback remain required production stages.
5. Observability exposes read-model diagnostics but full metrics/SLO/exporters/runbooks remain production work.
6. Capability Registry treats provider offline as health/availability state, not deletion; removal fails closed while entities or scenarios depend on a capability.
7. Temporal semantics define DST/timezone policy, but durable debounce/throttle/repeat/schedule execution still requires runtime/lowering and simulator evidence where applicable.
8. Mutation quality is a separate test-of-tests gate. Critical `LIVED`, `NOT COVERED` or `TIMED OUT` mutations are not acceptable evidence for closing safety-sensitive work.

## Next implementation order

Core production safety:

1. Stage 17 — complete authenticated ingress/RBAC principal contract, callback binding/replay/audit and exactly-once workflow resume; integrate with Stage 16 config/secrets.
2. Stage 15 — plan/schema catalog and migration compatibility.
3. Stage 18/23/24 — backup/restore, exhaustive crash matrix, backpressure/degraded mode and explicit process-topology gate.

Scenario product work can proceed only where production prerequisites allow it:

4. Stage 35 — authenticated Scenario API only after Stage 17 is proven.
5. Stage 36/38/40/37/41 — authoring UX, templates/trace/graph/mobile without bypassing catalog/compiler/safety contracts.
6. Stage 39 — LLM authoring only through the stable structured AST/validator/compiler boundary.

Integration/release:

7. Stage 21/22/26 — real adapters, observability and measured target-hardware budgets.
8. Stage 27 + Scenario Stage 42 — release/upgrade/rollback, provenance/reproducibility and scenario release gates, including mutation/edge-space evidence.

Index: `docs/PLAN_INDEX.md`.
Engineering protocol: `docs/engineering/ENGINEERING_LOOP.md`.
Master production plan: `docs/AXIOM_IMPLEMENTATION_PLAN.md`.
Scenario plan: `docs/SCENARIO_SYSTEM_PLAN.md`.
