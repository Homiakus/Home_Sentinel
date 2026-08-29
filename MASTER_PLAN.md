# Home Sentinel — Living MASTER PLAN

> Single execution source of truth for progressive audit, atomic implementation and verified direct-to-main delivery. Detailed architecture/product plans remain intent specifications; this file owns findings, invariants, ordering and execution state.

**Plan revision:** 2026-08-29 / T-021 verified on `8b13e5565590ac1d86f1e4ea7d9c8213542022fb`; F-016 discovered during the next T-004 workflow inventory; T-022 is the active P0 slice.

---

# 1. Mission

Deliver a production-grade local security/automation platform via small reviewable changes while preserving fail-closed authority, durable external-effect semantics, reproducibility, rollback and evidence-driven evolution.

`AUDIT -> PLAN -> SELECT -> CHARACTERIZE -> IMPLEMENT -> TEST -> SELF-REVIEW -> RECONCILE -> COMMIT -> PUSH main -> VERIFY -> COMPRESS -> NEXT`

Unexpected material discoveries are recorded before implementation scope expands.

# 2. Current State

- Go control plane: typed domain/events, Axiom lifecycle, ADGO durable workflows, auth/authz, SQLite state, gateways/recovery and Scenario compiler/safety/catalog/simulator.
- Verified tasks: T-001, T-002, T-003, T-011, T-012, T-018, T-019, T-020, T-021.
- T-019 makes `internal/orchestration/` and `internal/engloop/` mutation-critical; T-020 makes zero-mutant critical evidence fail closed.
- T-021 retains complete incident v1/v2 execution bundles and routes durable state by persisted identity through Axiom Host.
- T-021 final evidence on `8b13e556...`: ci/race/security PASS; Gremlins 69 killed, 0 lived, 0 not-covered, 0 timeout; efficacy/coverage 100%.
- T-004 inventory has now exposed F-016: siren plan identity is derived from `MaxActivationDuration`, so a config change after crash can strand an already-enabled physical effect under the old digest.
- Local full module execution is unavailable in this agent environment due dependency-network resolution; GitHub Actions remains authoritative.

# 3. Architecture Map

```text
Media / sensors -> typed evidence/events
                       |
              +--------+--------+
              |                 |
         Axiom lifecycle   ADGO workflows
              |                 |
              +-- principal/policy/RBAC --+
                              |
                       invocation gateways
                              |
                       external/physical IO

SQLite      : app/auth/audit + schema_migrations
ADGO/Pebble : Execution PlanID/PlanVersion/PlanDigest + durable history
Axiom Host  : multi-plan engine registry/routing by persisted digest
Bundle      : immutable plan + versioned handlers + external bindings/config semantics
Scenario    : AST -> validation -> Safety Compiler -> lowering -> catalog/simulator
```

Primary boundaries: `internal/domain`, `internal/orchestration`, `internal/gateway`, `internal/auth`, `internal/authz`, `internal/config`, `internal/database`, `internal/httpserver`, `internal/scenario`, `internal/integrations`, `internal/engloop`.

# 4. Baseline

| Gate | Baseline |
|---|---|
| module/build hygiene | PASS |
| vulnerability/static | PASS |
| formatting/vet | PASS |
| unit/integration | PASS |
| race | PASS; no active unexplained flake |
| security/SBOM/Trivy | PASS |
| config critical mutation | PASS, non-empty killed evidence |
| orchestration/engloop mutation | PASS; latest incident campaign 69/69 killed |
| benchmark smoke | PASS |
| target-hardware performance | OPEN T-008 |

# 5. System Invariants

- **I-001** ML/VLM/LLM never possess human/physical authority.
- **I-002** External writes cross desired-state/idempotency/reconciliation gateways.
- **I-003** Ambiguous provider outcomes are never blindly replayed.
- **I-004** Human authority cannot come from unsigned/unbound/machine input.
- **I-005** Physical ownership requires durable cross-execution control; process-local locks are not multi-process fencing.
- **I-006** Published Scenario semantics are version-pinned and Safety Compiler is mandatory.
- **I-007** Secrets are references; plaintext does not enter durable state/log/audit responses.
- **I-008** Non-terminal durable work survives compatible upgrade or fails/migrates explicitly.
- **I-009** Runtime DB/WAL/SHM/artifacts/operator state stay outside source control.
- **I-010** Every pushed main state is buildable and materially no worse than its parent.
- **I-011** Critical safety tests use observable synchronization, not scheduler luck.
- **I-012** Until TLS is actually served, production HTTP binds are loopback-only.
- **I-013** Human/service/system principals must be technically distinguishable.
- **I-014** HTTP request idempotency/staleness and workflow semantic idempotency are separate contracts.
- **I-015** Security-bearing configuration validators are mutation-critical and create executable mutation work.
- **I-016** Safety/control orchestration and engineering gate-policy code are mutation-critical.
- **I-017** Existing ADGO executions run only against their pinned PlanID/PlanDigest; active plan changes never silently reinterpret durable state.
- **I-018** A CRITICAL mutation campaign with selected production targets is invalid when Gremlins generates zero mutants.
- **I-019** Durable workflow identity pins an execution bundle: plan graph, version-specific handler semantics and external signal bindings; pinning only PlanDigest plus current handlers is insufficient.
- **I-020** Unknown persisted non-terminal plan/bundle identity fails startup/control-plane qualification closed; it is never silently skipped.
- **I-021** Configuration-derived durable plans that own physical effects must be reconstructible and routable by persisted identity across restart; a config change must not orphan safety timers, cancellation or compensation.

# 6. Findings Registry

## F-001 — Fragmented execution plan
**Status:** Resolved | **Severity:** High | T-001.

## F-002 — Runtime SQLite/WAL tracked in Git
**Status:** Resolved | **Severity:** High | T-002.

## F-003 — Stage 17 recorded status stale
**Status:** Resolved | **Severity:** Medium | T-003.

## F-004 — `main` lacks required-check technical protection
**Status:** Planned | **Severity:** High | **Confidence:** Confirmed | T-010.

## F-005 — Siren compensation race test used scheduler timing
**Status:** Resolved | **Severity:** High | T-011.

## F-006 — Hardened remote-TLS rule disconnected from runtime
**Status:** Resolved | **Severity:** High | T-012 + T-018.

## F-007 — Callback semantics not wired to external HTTP transport
**Status:** Planned | **Severity:** High | T-013.

## F-008 — No explicit human/service/system principal kinds
**Status:** Planned | **Severity:** High | T-014.

## F-009 — Authorization audit/rate limits path-specific
**Status:** Planned | **Severity:** Medium/High | T-015.

## F-010 — Authentication boundary not pluggable
**Status:** Planned | **Severity:** Medium | T-017.

## F-011 — Common HTTP stale/idempotency contract absent
**Status:** Planned | **Severity:** High | T-016.

## F-012 — Security configuration was not mutation-targetable
**Status:** Resolved | **Severity:** High | T-018.

## F-013 — Safety orchestration and gate-policy code were not mutation-critical
**Status:** Resolved | **Severity:** High | T-019 + T-020.

## F-014 — Critical mutation campaign accepted zero generated mutants
**Status:** Resolved | **Severity:** High | T-020.

## F-015 — Pinned incident plan could execute with drifted handlers/bindings
**Status:** Resolved | **Severity:** Critical | **Confidence:** Confirmed | T-021.

Incident v1 and v2 now retain immutable plan + version-specific Registry/handlers + signal bindings, use Axiom Host routing by persisted digest, validate unknown non-terminal state fail-closed, and prove Pebble reopen plus historical callback semantics. Final `8b13e556...` mutation campaign: 69/69 killed, no blockers.

## F-016 — Config-derived siren plan can strand an enabled physical effect across restart
**Status:** ACTIVE | **Severity:** Critical | **Confidence:** Confirmed

**Category:** Physical safety / Durability / Configuration evolution.  
**Evidence:** `siren.CompilePlan(maxActivation)` sets `Version = "1-" + maxActivation.String()` and embeds the duration into the safety wait; the enable node owns the physical siren effect and records `SirenEnsureDisabled` compensation. Current `siren.Open` compiles only the current duration and uses one `adgo.OpenProduction` engine. Pinned Axiom single-engine resilient coordination skips non-terminal executions whose PlanDigest differs from the engine plan.  
**Failure sequence:** enable under duration A -> process crash -> config changes to duration B -> reopen only B -> old A execution is not advanced by the B engine; its timer/disable/compensation can become unreachable from current `Drive`/`Stop`.  
**Root cause:** runtime configuration is part of durable plan identity but the service has no reconstructible historical plan catalog/Host routing.  
**Impact:** a siren may remain physically enabled beyond its intended safety window after crash plus config change.  
**Blast radius:** siren timer, manual cancellation, compensation, restart recovery and any future config-derived physical workflow plan.  
**Invariants:** I-002, I-005, I-008, I-017, I-020, I-021.  
**Task:** T-022.

# 7. Risk Register

| Risk | Severity | Control |
|---|---|---|
| siren enabled under old duration becomes unreachable after restart/config change | Critical | T-022 ACTIVE |
| in-flight workflow incompatible after upgrade | Critical | T-004 continuation |
| callback transport absent | High | T-013 |
| machine/human principal ambiguity | High | T-014 |
| stale/duplicate HTTP command | High | T-016 |
| external-effect crash linearization | Critical | T-005 |
| multi-process physical write race | Critical | T-006 |
| direct main without required checks | High | T-010 |
| release provenance/rollback | High | T-009 |

# 8. Pareto Improvements

1. T-022 make siren config-derived physical safety restart-safe.
2. Continue T-004 inventory/version retention across remaining long-lived workflows (door/camera and persisted schema surfaces).
3. T-013/T-014 callback transport/principal authority.
4. T-005/T-006 crash/fencing guarantees.
5. T-016 common command preconditions.
6. T-015/T-010 audit/abuse/main protection.

# 9. Dependency DAG

```text
T-001 DONE -> T-002 DONE -> T-011 DONE -> T-003 DONE
                                      |-> T-012 DONE -> T-018 DONE -> T-013 READY
                                      |-> T-019 DONE -> T-020 DONE -> T-021 DONE -> T-022 -> T-004 continuation
                                      |                                                   |-> T-005
                                      |                                                   |-> T-006
                                      |-> T-014 -> T-015 / T-017
                                      |-> T-016
T-001 -> T-010

T-014 + T-016 + relevant T-013/T-004 -> T-007
T-004/T-005/T-006 + T-008 -> T-009
```

# 10. Implementation Phases

- **A Truth/gates:** T-001/T-002/T-003/T-011/T-012/T-018/T-019/T-020/T-010.
- **B Durable evolution + identity/transport:** T-021 DONE -> T-022 -> remaining T-004/T-005/T-006 and T-013..T-017.
- **C Product surface:** T-007.
- **D Qualification:** T-008/T-009.

# 11. Atomic Tasks

## T-001/T-002/T-003/T-011/T-012/T-018/T-019/T-020/T-021
**Status:** DONE. Verified commits/evidence recorded below.

## T-004 — Pin durable workflow execution semantics across upgrade
**Status:** ACTIVE via T-022 and remaining workflow inventory | **Priority:** P0

Axiom already supplies durable plan identity, ExecutionCatalog, Pebble catalog support, multi-plan Host, digest routing and conservative explicit migration APIs. Home Sentinel retains/reconstructs complete historical execution semantics rather than recreating Axiom routing.

## T-022 — Retain/reconstruct siren duration bundles across config changes
**Status:** READY | **Priority:** P0 | **Type:** PHYSICAL SAFETY / DURABILITY | **Risk:** CRITICAL

### Problem
Siren PlanVersion and safety timer are derived from `MaxActivationDuration`. A crash followed by duration change opens a new digest while an old execution may already own an enabled physical effect and compensation stack.

### Goal
New siren executions start on the configured duration; every supported non-terminal historical duration execution is reconstructed from persisted identity, digest-verified, registered in Axiom Host and remains controllable through Drive/Stop/Serve. Unsupported/malformed identity fails closed.

### Scope
`internal/orchestration/action/siren`: duration-version parser/reconstruction, bundle catalog, Host-backed service routing and Pebble crash/config-change tests.

### Implementation direction
- preserve current `CompilePlan(maxActivation)` semantics and make its `1-<duration>` identity canonical/reversible;
- parse only canonical positive duration versions; reject malformed/zero/negative/alias forms;
- scan non-terminal persisted siren executions on Open, reconstruct each unique historical plan, and verify PlanID + canonical PlanVersion + PlanDigest exactly before registration;
- register active and reconstructed historical plans with fresh Registries in Axiom Host over the existing Production Store/Router;
- Start only on active bundle; same execution id remains idempotent;
- Drive/Stop load persisted execution first and select its exact engine; historical cancellation must reach the original compensation stack;
- Serve performs synchronous fail-closed validation before Host coordinator/worker plus active schedule runner;
- do not rewrite old state to the active duration and do not fork Axiom durable logic.

### Acceptance
- parser round-trips canonical positive durations and rejects malformed/zero/negative/non-canonical versions;
- Pebble: duration A execution enables siren and waits; process closes; reopen same store with duration B; old execution remains A; Stop+Drive through historical engine reaches canceled and compensation disables siren;
- new execution after reopen uses duration B plan/version/digest;
- malformed historical version and valid-version/digest mismatch fail Open closed;
- existing automatic timer/manual-stop behavior remains green;
- race/security PASS;
- Gremlins produces non-empty `internal/orchestration/action/siren` evidence with zero lived/not-covered/timeout blockers.

## T-005 — Crash/restore external-effect linearization
**Status:** TODO | **Priority:** P0 | Depends T-004 relevant surfaces.

## T-006 — Supported topology/fencing
**Status:** TODO | **Priority:** P0 | Depends T-004 relevant surfaces.

## T-007 — Authenticated Scenario API
**Status:** BLOCKED | **Priority:** P1.

## T-008 — Target-hardware performance budgets
**Status:** TODO | **Priority:** P1.

## T-009 — Release/upgrade/rollback qualification
**Status:** BLOCKED | **Priority:** P0.

## T-010 — Technical verified-main guard
**Status:** TODO | **Priority:** P1.

## T-013 — Callback HTTP bearer adapter
**Status:** READY | **Priority:** P0.

## T-014 — Explicit principal kinds/human-authority boundary
**Status:** TODO | **Priority:** P0.

## T-015 — Authorization audit/principal-aware limiting
**Status:** BLOCKED | **Priority:** P1.

## T-016 — HTTP command expected-version/idempotency contract
**Status:** TODO | **Priority:** P0.

## T-017 — Pluggable authenticator boundary
**Status:** BLOCKED | **Priority:** P2.

# 12. Testing Strategy

G0 build/static/module; G1 unit/golden; G2 property/fuzz/model; G3 race; G4 contract/integration; G5 fault/crash; G6 mutation; G7 E2E/HIL/UX; G8 performance/soak/release.

Critical edge space: `input x identity x authority x time x ordering x concurrency x persistence x external failure x ownership x topology x plan-version x handler-version x config-derived-version x signal-binding x cancellation x compensation x recovery x gate-classification x evidence-emptiness`.

# 13. Mutation Testing Strategy

- Critical changed semantics require real Gremlins execution and non-empty generated evidence.
- `MutantsTotal==0` is failure when selected critical targets exist.
- Config, orchestration, engloop, principal policy, command preconditions, migration guards and physical gateways are mutation-critical.
- Version parsing/routing/digest guards and physical cancellation/compensation recovery are mutation-critical.
- Surviving/not-covered/timeout mutants require contract analysis; never weaken gates merely to obtain green.

# 14. Performance Baselines

Benchmark smoke is regression smoke only. T-008 owns target-hardware latency/throughput/allocation budgets.

# 15. Security Hardening

Source-state hygiene DONE; deterministic siren test DONE; Stage17 truth DONE; remote plaintext guard DONE; config mutation boundary DONE; orchestration taxonomy DONE; zero-evidence guard DONE; incident execution bundles DONE. Next: siren physical restart safety -> callback/principal/preconditions -> crash/fencing -> main/release qualification.

# 16. Migration Strategy

`characterize old semantics -> derive/retain immutable historical execution bundle -> introduce active new bundle -> route existing state by durable identity -> explicit migration only at safe/quiescent point -> verify restart/rollback -> retire old bundle only after no durable references remain`.

For config-derived plans, reconstruction itself is a compatibility boundary: parsed configuration, canonical version and recompiled digest must all agree before the bundle is accepted.

# 17. Deferred Work

Broad Scenario UI/UX; full TLS listener/cert lifecycle; OIDC/mTLS implementations; multi-node mode beyond explicit v1 topology; low-leverage refactors.

# 18. Rejected Decisions

- roadmap checkbox as proof;
- green rerun as flake resolution;
- safety timeout inflation;
- callback HTTP before remote plaintext is impossible;
- ADGO semantic idempotency as HTTP idempotency;
- service/system as fake users;
- unused TLS fields as security;
- mutation request without selected target as evidence;
- selected target with zero generated mutants as evidence;
- automatic plan substitution/migration on startup;
- pinning only the DAG while reusing drifted current handlers;
- silently ignoring unknown non-terminal plan digests;
- treating current runtime config as permission to reinterpret historical physical state;
- reimplementing Axiom Host/migration logic;
- force push.

# 19. Completed Tasks

T-001, T-002, T-003, T-011, T-012, T-018, T-019, T-020, T-021.

# 20. Iteration Log

- **1 T-001** `905862f...`: ci/security/mutation PASS.
- **2 T-002** `745f194...`: DB hygiene; F-005 discovered; final PASS.
- **3 F-005 plan** `c30b395...`: PASS.
- **4 T-011** `5ccff8bf...`: deterministic siren test; first-attempt PASS.
- **5 T-003** `ef8ecaf...`: Stage17 reconciliation; PASS.
- **6 T-012** `d57f239...`: ci/security PASS; mutation setup exposed F-012.
- **7 T-018** `26b28f...`: config mutation boundary; Gremlins killed 1/1; all gates PASS.
- **8 F-013 plan/closure** `aaafae7...`: planning gates PASS.
- **9 T-019** `6b55fe38...`: ci/race/security PASS; zero-mutant result exposed F-014.
- **10 F-014 planning** `a69bc45...`: finding/work packet recorded.
- **11 T-020** `36d4847...`: ci/race/security PASS; Gremlins 2/2 killed; closes F-013/F-014 and T-019/T-020.
- **12 F-015/T-021 planning** `36d9c31...`: execution-bundle invariant recorded; planning gates PASS.
- **13 T-021 scope reconciliation** `d72c415...`: callback/read-model version-aware scope recorded; planning gates PASS.
- **14 T-021 runtime** `e6e2bd4...`: ci/race/security PASS; mutation 62 killed but exposed characterization/testability blockers.
- **15 T-021 mutation recovery** `a68b497...`: ci/race/security PASS; mutation improved to 66 killed with remaining lived/not-covered/timeout blockers.
- **16 T-021 testability recovery** `a2adcd87...`: Gremlins 69/69 killed, 100%/100%; standard CI exposed only a gofmt defect in the new test file.
- **17 T-021 format closure** `8b13e556...`: full ci/race/reconciliation/benchmark + security/SBOM/Trivy + Gremlins 69/69 PASS; closes F-015/T-021.
- **18 F-016/T-022 planning:** record config-derived siren physical-safety restart invariant before implementation.

# 21. Definition of Final Done

No unresolved Critical/High findings without accepted deferral; all P0/P1 acceptance verified; remote exposure safe; critical test-of-tests has selected targets and non-empty clean mutation evidence; explicit principal kinds; durable plan+handler+binding/config evolution and restart/rollback; crash/fencing tests; stale/idempotency HTTP contracts; security/race/static/fault/mutation green; hardware budgets met; no unexplained flakes; docs match code; final re-audit finds no fundamental blocker; last verified state is main with no force push.
