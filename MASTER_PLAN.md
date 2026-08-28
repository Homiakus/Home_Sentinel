# Home Sentinel — Living MASTER PLAN

> Single execution source of truth for progressive audit, atomic implementation and verified direct-to-main delivery. Detailed architecture/product plans remain intent specifications; this file owns findings, invariants, ordering and execution state.

**Plan revision:** 2026-08-28 / T-019 and T-020 verified on `36d4847bbb1bd51adc0a4e4bd76c5b480f1be918`; F-015 discovered during T-004 characterization; T-021 is the next atomic durable-upgrade slice.

---

# 1. Mission

Deliver a production-grade local security/automation platform via small reviewable changes while preserving fail-closed authority, durable external-effect semantics, reproducibility, rollback and evidence-driven evolution.

`AUDIT -> PLAN -> SELECT -> CHARACTERIZE -> IMPLEMENT -> TEST -> SELF-REVIEW -> RECONCILE -> COMMIT -> PUSH main -> VERIFY -> COMPRESS -> NEXT`

Unexpected material discoveries are recorded before implementation scope expands.

# 2. Current State

- Go control plane: typed domain/events, Axiom lifecycle, ADGO durable workflows, auth/authz, SQLite state, gateways/recovery and Scenario compiler/safety/catalog/simulator.
- Verified tasks: T-001, T-002, T-003, T-011, T-012, T-018, T-019, T-020.
- T-019 makes `internal/orchestration/` and `internal/engloop/` mutation-critical.
- T-020 makes zero-mutant critical evidence fail closed. Recovery campaign over T-019+T-020 produced 2 mutants, killed 2, lived/not-covered 0, efficacy and mutator coverage 100%; ci/race/security also PASS on the same SHA.
- T-004 characterization: ADGO already persists PlanID/PlanVersion/PlanDigest and pinned Axiom already supplies ExecutionCatalog, multi-plan Host and digest routing.
- New F-015: a pinned DAG is insufficient if historical executions use current handler semantics. Incident v1 and v2 differ in risk assessment/archive behavior and owner-response node binding.
- Local full module execution is unavailable in this agent environment due network dependency resolution; GitHub Actions is authoritative.

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
Bundle      : immutable plan + versioned handlers + signal bindings
Scenario    : AST -> validation -> Safety Compiler -> lowering -> catalog/simulator
```

Primary boundaries: `internal/domain`, `internal/orchestration`, `internal/gateway`, `internal/auth`, `internal/authz`, `internal/config`, `internal/database`, `internal/httpserver`, `internal/scenario`, `internal/integrations`, `internal/engloop`.

# 4. Baseline

| Gate | Baseline |
|---|---|
| module/build hygiene | PASS |
| formatting | PASS |
| vet/static | PASS |
| unit/integration | PASS |
| race | PASS; no active unexplained flake |
| security qualification | PASS |
| config critical mutation | PASS, 1 killed mutant |
| orchestration/engloop mutation | PASS, non-empty evidence; 2/2 killed on T-019+T-020 recovery range |
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
- **I-018** A CRITICAL mutation campaign with selected production targets is invalid evidence when Gremlins generates zero mutants.
- **I-019** Durable workflow identity pins an execution bundle: plan graph, version-specific handler semantics and external signal bindings; pinning only PlanDigest is insufficient.
- **I-020** Unknown persisted non-terminal plan/bundle identity fails startup or control-plane qualification closed; it is never silently skipped.

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
**Status:** Resolved | **Severity:** High | T-018; Gremlins killed 1/1.

## F-013 — Safety orchestration and gate-policy code were not mutation-critical
**Status:** Resolved | **Severity:** High | **Confidence:** Confirmed | T-019 + T-020.

T-019 routes `internal/orchestration/` and `internal/engloop/` as CRITICAL. Recovery range on `36d4847...` selected `./internal/engloop`, produced 2 mutants and killed both; ci/race/security PASS.

## F-014 — Critical mutation campaign accepted zero generated mutants
**Status:** Resolved | **Severity:** High | **Confidence:** Confirmed | T-020.

`MutationReport.HasMutationEvidence()` and CLI fail-closed semantics reject `MutantsTotal==0`. Recovery campaign produced non-empty clean evidence (2 killed, 0 lived/not-covered).

## F-015 — Pinned plan can execute with drifted handler semantics
**Status:** Planned / ACTIVE | **Severity:** Critical | **Confidence:** Confirmed

**Category:** Durability / Upgrade / Orchestration semantics.  
**Evidence:** historical incident v1 (`caa446b...`) and current v2 use the same Axiom revision `7682ba9170dd`, so historical definition can be recompiled compatibly; however v1 `assessRisk` uses the original local scoring formula and `archiveIncident` writes `archived=true`, while current handlers use `riskpolicy.DefaultPolicy()` and different archive facts. v1 also waits on `await-owner-response`; v2 uses `await-owner-ack`.  
**Root cause:** Home Sentinel service binds one current Registry and current signal-node constants to all executions. Durable ADGO identity pins the plan digest but the application has no immutable versioned execution-bundle catalog.  
**Impact:** after upgrade an old execution may be routed to the correct DAG but execute new business/safety semantics, or fail to consume the correct external event.  
**Blast radius:** all long-lived workflows whose handlers/bindings evolve independently of PlanDigest.  
**Invariants:** I-008, I-017, I-019, I-020.  
**Task:** T-021 first; T-004 remains the umbrella durable-evolution objective.

# 7. Risk Register

| Risk | Severity | Control |
|---|---|---|
| old execution uses new handler semantics | Critical | T-021 ACTIVE |
| unknown historical digest silently stranded | Critical | T-021/I-020 |
| in-flight workflow incompatible after upgrade | Critical | T-004 via T-021+follow-ups |
| callback transport absent | High | T-013 |
| machine/human principal ambiguity | High | T-014 |
| stale/duplicate HTTP command | High | T-016 |
| external-effect crash linearization | Critical | T-005 |
| multi-process physical write race | Critical | T-006 |
| direct main without required checks | High | T-010 |
| release provenance/rollback | High | T-009 |

# 8. Pareto Improvements

1. T-021 retain incident v1/v2 execution bundles and prove restart across upgrade.
2. Finish T-004 across remaining long-lived workflows/version surfaces.
3. T-013/T-014 callback transport/principal authority.
4. T-005/T-006 crash/fencing guarantees.
5. T-016 common command preconditions.
6. T-015/T-010 audit/abuse/main protection.

# 9. Dependency DAG

```text
T-001 DONE -> T-002 DONE -> T-011 DONE -> T-003 DONE
                                      |-> T-012 DONE -> T-018 DONE -> T-013 READY
                                      |-> T-019 DONE -> T-020 DONE -> T-021 -> T-004 close
                                      |                                      |-> T-005
                                      |                                      |-> T-006
                                      |-> T-014 -> T-015 / T-017
                                      |-> T-016
T-001 -> T-010

T-014 + T-016 + relevant T-013/T-004 -> T-007
T-004/T-005/T-006 + T-008 -> T-009
```

# 10. Implementation Phases

- **A Truth/gates:** T-001/T-002/T-003/T-011/T-012/T-018/T-019/T-020/T-010.
- **B Durable evolution + identity/transport:** T-021 -> T-004..T-006 and T-013..T-017.
- **C Product surface:** T-007.
- **D Qualification:** T-008/T-009.

# 11. Atomic Tasks

## T-001/T-002/T-003/T-011/T-012/T-018/T-019/T-020
**Status:** DONE. Verified commits recorded in Iteration Log.

## T-004 — Pin durable workflow execution semantics across upgrade
**Status:** ACTIVE via T-021 | **Priority:** P0

Axiom already supplies durable plan identity, ExecutionCatalog, Pebble catalog support, multi-plan Host, digest routing and conservative explicit migration APIs. Home Sentinel must retain complete historical execution bundles and prove restart/rollback behavior rather than recreate Axiom routing.

## T-021 — Retain incident v1/v2 execution bundles across upgrade
**Status:** READY | **Priority:** P0 | **Type:** DURABILITY | **Risk:** CRITICAL

### Problem
Incident service currently opens only current v2 plan/Registry and uses current node constants for signals. Historical v1 has different handler semantics and response node.

### Goal
New incidents start on active v2; persisted v1 executions continue after restart using exact v1 plan, v1 handlers and v1 signal bindings. Unknown non-terminal digests fail closed.

### Scope
`internal/orchestration/incident`: immutable v1/v2 bundle descriptors, historical v1 handlers, Host-backed service routing, version-specific owner-response binding and durable upgrade tests.

### Implementation direction
- retain an exact v1 plan definition from `caa446b...` and assert identity/version/digest compatibility;
- retain exact v1 handler semantics separately from current v2 handlers;
- define immutable bundle metadata including plan, Registry and event/node bindings;
- use Axiom `Host` over the existing Production Store/Router; do not reimplement host routing or backend creation;
- start new executions explicitly on active v2 bundle;
- for Drive/Get/Signal/Human operations, load persisted execution and select the exact bundle/engine by PlanDigest;
- validate non-terminal persisted executions against registered bundle digests at startup/qualification; unknown digest is an error;
- migrate `Serve` to Host coordinator/worker semantics without double-running the single-plan production engine;
- keep migration explicit; do not silently rewrite v1 durable state into v2.

### Acceptance
- v1 and v2 bundle identities are deterministic and distinct;
- v1 handler behavior has characterization tests proving historical risk/archive semantics;
- Memory test proves concurrent v1/v2 routing by persisted digest;
- Pebble test: create/drive v1 to waiting, close, reopen with v2 active + v1 retained, resume through v1 binding, no duplicate notification, completion uses v1 semantics;
- new execution after reopen uses v2;
- unknown persisted non-terminal digest fails closed;
- current v2 tests remain green;
- race/security/mutation-critical PASS with non-empty orchestration mutants and no survivors/not-covered blockers.

## T-005 — Crash/restore external-effect linearization
**Status:** TODO | **Priority:** P0 | Depends T-004.

## T-006 — Supported topology/fencing
**Status:** TODO | **Priority:** P0 | Depends T-004.

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

Critical edge space: `input x identity x authority x time x ordering x concurrency x persistence x external failure x ownership x topology x plan-version x handler-version x signal-binding x cancellation x recovery x gate-classification x evidence-emptiness`.

# 13. Mutation Testing Strategy

- Critical changed semantics require real Gremlins execution and non-empty generated evidence.
- `MutantsTotal==0` is failure when the workflow was invoked for selected critical targets.
- Config, orchestration, engloop, principal policy, command preconditions, migration guards and physical gateways are mutation-critical.
- Version-routing/unknown-digest guards are mutation-critical.
- Surviving/not-covered mutants require contract analysis; never weaken gates merely to obtain green.

# 14. Performance Baselines

Benchmark smoke is regression smoke only. T-008 owns target-hardware latency/throughput/allocation budgets.

# 15. Security Hardening

Source-state hygiene DONE; deterministic siren safety DONE; Stage17 truth DONE; remote plaintext guard DONE; config mutation boundary DONE; orchestration taxonomy DONE; zero-evidence guard DONE. Next: durable execution bundles -> callback/principal/preconditions -> crash/fencing -> main/release qualification.

# 16. Migration Strategy

`characterize old plan + handlers + bindings -> retain immutable old execution bundle -> introduce active new bundle -> route existing state by durable identity -> explicit migration only at safe/quiescent point -> verify restart/rollback -> retire old bundle only after no durable references remain`.

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
- reimplementing Axiom Host/migration logic;
- force push.

# 19. Completed Tasks

T-001, T-002, T-003, T-011, T-012, T-018, T-019, T-020.

# 20. Iteration Log

- **1 T-001** `905862f...`: ci/security/mutation PASS.
- **2 T-002** `745f194...`: DB hygiene; F-005 discovered; final PASS.
- **3 F-005 plan** `c30b395...`: PASS.
- **4 T-011** `5ccff8bf...`: deterministic siren test; first-attempt PASS.
- **5 T-003** `ef8ecaf...`: Stage17 reconciliation; PASS.
- **6 T-012** `d57f239...`: ci/security PASS; mutation setup exposed F-012.
- **7 T-018** `26b28f...`: config mutation boundary; Gremlins killed 1/1; all gates PASS.
- **8 F-013 plan/closure** `aaafae7...`: planning gates PASS.
- **9 T-019** `6b55fe38...`: ci/race/security PASS; mutation self-triggered but zero mutants exposed F-014.
- **10 F-014 planning** `a69bc45...`: finding/work packet recorded; ci/security PASS; false-success reproduced.
- **11 T-020** `36d4847...`: ci/race/security PASS; planner CRITICAL + `./internal/engloop`; Gremlins killed 2/2, lived/not-covered 0; closes F-013/F-014 and T-019/T-020.
- **12 F-015/T-021 planning:** record execution-bundle semantic pinning requirement before T-004 runtime changes.

# 21. Definition of Final Done

No unresolved Critical/High findings without accepted deferral; all P0/P1 acceptance verified; remote exposure safe; critical test-of-tests has selected targets and non-empty clean mutation evidence; explicit principal kinds; durable plan+handler+binding upgrade/restart/rollback; crash/fencing tests; stale/idempotency HTTP contracts; security/race/static/fault/mutation green; hardware budgets met; no unexplained flakes; docs match code; final re-audit finds no fundamental blocker; last verified state is main with no force push.
