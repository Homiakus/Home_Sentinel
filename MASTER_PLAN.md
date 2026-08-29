# Home Sentinel — Living MASTER PLAN

> Single execution source of truth for progressive audit, atomic implementation and verified direct-to-main delivery. Detailed architecture/product plans remain intent specifications; this file owns findings, invariants, ordering and execution state.

**Plan revision:** 2026-08-29 / T-022 verified on `dfced3277a61211ae312b8eef18e761b980a9238`; F-017 discovered while finishing the T-004 durable-workflow inventory; T-023 is the next atomic P0 slice.

---

# 1. Mission

Deliver a production-grade local security/automation platform via small reviewable changes while preserving fail-closed authority, durable external-effect semantics, reproducibility, rollback and evidence-driven evolution.

`AUDIT -> PLAN -> SELECT -> CHARACTERIZE -> IMPLEMENT -> TEST -> SELF-REVIEW -> RECONCILE -> COMMIT -> PUSH main -> VERIFY -> COMPRESS -> NEXT`

Unexpected material discoveries are recorded before implementation scope expands.

# 2. Current State

- Go control plane: typed domain/events, Axiom lifecycle, ADGO durable workflows, auth/authz, SQLite state, gateways/recovery and Scenario compiler/safety/catalog/simulator.
- Verified tasks: T-001, T-002, T-003, T-011, T-012, T-018, T-019, T-020, T-021, T-022.
- T-021 retains complete incident v1/v2 execution bundles and routes durable state by persisted identity through Axiom Host.
- T-022 reconstructs canonical historical siren duration bundles, verifies PlanID/PlanVersion/PlanDigest, routes old Drive/Stop through their exact engine, and preserves physical compensation across crash + configuration change.
- T-022 final evidence on `dfced327...`: module/vulnerability/format/vet/unit/race/reconciliation/benchmark PASS; security/SBOM/Trivy PASS; Gremlins 68 killed, 0 lived, 0 not-covered, 0 timeout, efficacy and mutator coverage 100%.
- ADGO inventory is complete: incident, siren, door action and camera recovery are the only compiled workflow families. Door/camera history shows initial creation plus formatting only; there is no current historical semantic drift. F-017 is preventive: their single-plan services still lack an explicit retained-bundle upgrade boundary for the first future v2.
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
| incident durable-upgrade mutation | PASS, 69/69 killed |
| siren durable-upgrade mutation | PASS, 68/68 killed |
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
- **I-022** A fixed-version durable workflow must have an explicit immutable-bundle release boundary before its first semantic v2: plan digest and handlers are pinned together, old non-terminal state remains routable, and an unregistered identity fails closed.

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

Incident v1/v2 now retain immutable plan + version-specific Registry/handlers + signal bindings, use Axiom Host routing by persisted digest, validate unknown non-terminal state fail-closed, and prove Pebble reopen plus historical callback semantics. Final `8b13e556...` mutation campaign: 69/69 killed, no blockers.

## F-016 — Config-derived siren plan could strand an enabled physical effect across restart
**Status:** Resolved | **Severity:** Critical | **Confidence:** Confirmed | T-022.

T-022 adds canonical `1-<duration>` reconstruction, exact identity/digest validation, active + historical Axiom Host engines, persisted-digest Drive/Stop routing and fail-closed non-terminal startup validation. Pebble tests prove duration-A enable -> process close -> duration-B reopen -> historical cancellation/compensation disables the siren without replaying the original enable; new work uses duration B. Final `dfced327...`: all CI/race/security gates PASS; Gremlins 68/68 killed, no blockers.

## F-017 — Door/camera fixed-version services lack a first-v2 retained-bundle boundary
**Status:** Planned / ACTIVE | **Severity:** Critical | **Confidence:** Confirmed

**Category:** Durability / Physical-control upgrade readiness.  
**Evidence:** the complete ADGO inventory contains only incident, siren, door and camera recovery. Door and camera both use `PlanVersion = "1"`, open a single `adgo.Production` engine, and route human/reconciliation operations through current node constants. Path history shows their plans/handlers have not semantically changed since initial introduction, so no historical execution is currently known to be misinterpreted.  
**Root cause:** unlike incident/siren, neither fixed-version service has an immutable execution-bundle catalog, golden v1 identity, persisted-digest operation routing or fail-closed startup validation.  
**Impact:** the first future semantic v2 can strand or reinterpret existing v1 non-terminal work unless v1 is retained at the same time. Door is physical access control; camera recovery performs external reconnect effects and human recovery decisions.  
**Blast radius:** future door/camera plan or handler evolution, human waits, reconciliation and external effects.  
**Invariants:** I-008, I-017, I-019, I-020, I-022.  
**Tasks:** T-023 door first; T-024 camera second. T-004 closes after both are verified and the workflow inventory is reconciled.

# 7. Risk Register

| Risk | Severity | Control |
|---|---|---|
| future door v2 strands/reinterprets v1 access-control work | Critical | T-023 ACTIVE |
| future camera v2 strands/reinterprets v1 recovery work | Critical | T-024 planned |
| external-effect crash linearization | Critical | T-005 |
| multi-process physical write race | Critical | T-006 |
| callback transport absent | High | T-013 |
| machine/human principal ambiguity | High | T-014 |
| stale/duplicate HTTP command | High | T-016 |
| direct main without required checks | High | T-010 |
| release provenance/rollback | High | T-009 |

# 8. Pareto Improvements

1. T-023/T-024 finish durable upgrade boundaries for the last fixed-version ADGO families and close T-004.
2. T-005/T-006 prove crash linearization and supported single-writer/fencing topology.
3. T-013/T-014 callback transport/principal authority.
4. T-016 common command preconditions.
5. T-015/T-010 audit/abuse/main protection.
6. T-008/T-009 hardware/release qualification.

# 9. Dependency DAG

```text
T-001 DONE -> T-002 DONE -> T-011 DONE -> T-003 DONE
                                      |-> T-012 DONE -> T-018 DONE -> T-013 READY
                                      |-> T-019 DONE -> T-020 DONE -> T-021 DONE -> T-022 DONE -> T-023 -> T-024 -> T-004 close
                                      |                                                                  |-> T-005
                                      |                                                                  |-> T-006
                                      |-> T-014 -> T-015 / T-017
                                      |-> T-016
T-001 -> T-010

T-014 + T-016 + relevant T-013/T-004 -> T-007
T-004/T-005/T-006 + T-008 -> T-009
```

# 10. Implementation Phases

- **A Truth/gates:** T-001/T-002/T-003/T-011/T-012/T-018/T-019/T-020/T-010.
- **B Durable evolution + identity/transport:** T-021/T-022 DONE -> T-023 -> T-024 -> T-004 close -> T-005/T-006 and T-013..T-017.
- **C Product surface:** T-007.
- **D Qualification:** T-008/T-009.

# 11. Atomic Tasks

## T-001/T-002/T-003/T-011/T-012/T-018/T-019/T-020/T-021/T-022
**Status:** DONE. Verified commits/evidence recorded below.

## T-004 — Pin durable workflow execution semantics across upgrade
**Status:** ACTIVE via T-023/T-024 | **Priority:** P0

Axiom supplies durable plan identity, ExecutionCatalog, Pebble catalog support, multi-plan Host, digest routing and conservative explicit migration APIs. Home Sentinel has verified complete execution-bundle handling for incident and config-derived siren plans. Door/camera are the final ADGO families; their histories show no existing semantic drift, so the remaining work is preventive retained-v1 release boundaries before any first v2.

## T-023 — Establish immutable door v1 execution-bundle boundary
**Status:** READY | **Priority:** P0 | **Type:** PHYSICAL ACCESS / DURABILITY | **Risk:** CRITICAL

### Problem
Door v1 has not historically drifted, but the service is single-plan and binds all Drive/human/reconciliation operations to the current engine/constants. A future v2 could strand a non-terminal approval/reconciliation or reinterpret physical access-control work unless v1 is retained explicitly.

### Goal
Freeze current door v1 as an immutable execution bundle with a deterministic golden digest, route persisted operations by exact bundle identity, fail closed on unknown non-terminal identity, and make the production path capable of retaining v1 when a future active version is introduced—without inventing a fake semantic v2 now.

### Scope
`internal/orchestration/action/door`: immutable v1 descriptor/identity, bundle catalog/Host routing, version-aware approval/reconciliation bindings, startup validation and Pebble tests.

### Acceptance
- repository history characterization documents that current v1 plan/handlers have no semantic predecessors beyond formatting;
- current v1 PlanID/PlanVersion/PlanDigest are golden-tested and drift fails tests;
- new executions use the active v1 bundle today;
- Drive, unlock approval and reconciliation select the bundle/engine from persisted identity rather than unconditional current engine constants;
- unknown or mismatched non-terminal persisted identity fails Open/preflight closed; terminal history does not block startup;
- Pebble restart preserves waiting human/reconciliation state and external-effect idempotency;
- an internal multi-bundle test seam proves retained v1 + distinct active plan can coexist through Axiom Host without changing production semantics to a fake v2;
- ci/race/security PASS and Gremlins produces non-empty clean door orchestration evidence.

## T-024 — Establish immutable camera-recovery v1 execution-bundle boundary
**Status:** TODO | **Priority:** P0 | **Type:** RECOVERY / DURABILITY | **Risk:** CRITICAL | Depends T-023 pattern.

Freeze current camera-recovery v1 identity/handlers/human binding, route persisted operations by exact bundle, fail closed on unknown non-terminal identity, and prove retained-v1 plus future-active coexistence without introducing a fake semantic v2.

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

Critical edge space: `input x identity x authority x time x ordering x concurrency x persistence x external failure x ownership x topology x plan-version x handler-version x config-derived-version x signal/human-binding x cancellation x compensation x recovery x gate-classification x evidence-emptiness`.

# 13. Mutation Testing Strategy

- Critical changed semantics require real Gremlins execution and non-empty generated evidence.
- `MutantsTotal==0` is failure when selected critical targets exist.
- Config, orchestration, engloop, principal policy, command preconditions, migration guards and physical gateways are mutation-critical.
- Version parsing/routing/digest guards, human/reconciliation binding, physical cancellation and compensation recovery are mutation-critical.
- Surviving/not-covered/timeout mutants require contract analysis; never weaken gates merely to obtain green.

# 14. Performance Baselines

Benchmark smoke is regression smoke only. T-008 owns target-hardware latency/throughput/allocation budgets.

# 15. Security Hardening

Source-state hygiene DONE; deterministic siren test DONE; Stage17 truth DONE; remote plaintext guard DONE; config mutation boundary DONE; orchestration taxonomy DONE; zero-evidence guard DONE; incident execution bundles DONE; siren config-drift recovery DONE. Next: door/camera immutable bundle boundaries -> callback/principal/preconditions -> crash/fencing -> main/release qualification.

# 16. Migration Strategy

`characterize old semantics -> pin immutable historical execution bundle -> introduce active new bundle only for a real semantic change -> route existing state by durable identity -> explicit migration only at safe/quiescent point -> verify restart/rollback -> retire old bundle only after no durable references remain`.

For config-derived plans, reconstruction is a compatibility boundary: parsed configuration, canonical version and recompiled digest must all agree before the bundle is accepted. For fixed-version plans, a semantic graph/handler change requires a version bump and retained old bundle in the same release; golden digest drift under the same version is a test failure.

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
- creating a fake v2 only to exercise upgrade plumbing;
- changing a fixed-version plan/handler bundle without version bump + retained old bundle;
- reimplementing Axiom Host/migration logic;
- force push.

# 19. Completed Tasks

T-001, T-002, T-003, T-011, T-012, T-018, T-019, T-020, T-021, T-022.

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
- **15 T-021 mutation recovery** `a68b497...`: ci/race/security PASS; mutation improved to 66 killed with remaining blockers.
- **16 T-021 testability recovery** `a2adcd87...`: Gremlins 69/69 killed; standard CI exposed only gofmt defect.
- **17 T-021 format closure** `8b13e556...`: full ci/race/security + Gremlins 69/69 PASS; closes F-015/T-021.
- **18 F-016/T-022 planning** `296795e8...`: siren config-derived physical-safety invariant recorded; planning gates PASS.
- **19 T-022 runtime** `26658f7...`: module/vuln/format/vet/unit/security PASS; Gremlins 67 killed/1 lived exposed a missing Stop return-contract assertion; race was later canceled only by the recovery push.
- **20 T-022 mutation recovery** `dfced327...`: full module/vuln/format/vet/unit/race/reconciliation/benchmark + security/SBOM/Trivy PASS; Gremlins 68/68 killed, 0 blockers, 100% efficacy/coverage; closes F-016/T-022.
- **21 F-017/T-023 planning:** full ADGO inventory confirms no current door/camera historical semantic drift; record preventive retained-bundle release boundary before future v2 changes.

# 21. Definition of Final Done

No unresolved Critical/High findings without accepted deferral; all P0/P1 acceptance verified; remote exposure safe; critical test-of-tests has selected targets and non-empty clean mutation evidence; explicit principal kinds; durable plan+handler+binding/config evolution and restart/rollback; crash/fencing tests; stale/idempotency HTTP contracts; security/race/static/fault/mutation green; hardware budgets met; no unexplained flakes; docs match code; final re-audit finds no fundamental blocker; last verified state is main with no force push.
