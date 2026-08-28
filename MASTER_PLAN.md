# Home Sentinel — Living MASTER PLAN

> Single execution source of truth for progressive audit, atomic implementation and verified direct-to-main delivery. Detailed architecture/product plans remain intent specifications; this file owns findings, invariants, ordering and execution state.

**Plan revision:** 2026-08-28 / T-012 and T-018 verified on `26b28f4505f7b4cd4059c14d310b326b97cdef50`; F-013/T-019 inserted before T-004.

---

# 1. Mission

Deliver a production-grade local security/automation platform through small reviewable changes while preserving fail-closed authority, durable external-effect semantics, reproducibility, rollback and evidence-driven evolution.

`AUDIT -> PLAN -> SELECT -> CHARACTERIZE -> IMPLEMENT -> TEST -> SELF-REVIEW -> RECONCILE -> COMMIT -> PUSH main -> VERIFY -> COMPRESS -> NEXT`

Material discoveries enter Findings and the dependency graph before implementation scope expands.

# 2. Current State

- Go control plane with typed domain/events, Axiom lifecycle, ADGO durable workflows, auth/authz, SQLite application state, gateways/recovery and Scenario model/compiler/safety/catalog/simulator.
- ML/VLM/LLM are evidence producers, never authority holders.
- Verified tasks: T-001, T-002, T-003, T-011, T-012, T-018.
- T-012 makes the current plaintext runtime loopback-only. T-018 repaired test-of-tests so `internal/config/` is CRITICAL and mutation-targetable.
- On `26b28f...`: `ci`, `security`, `mutation` all PASS; Gremlins planned `./internal/config`, killed 1/1 relevant mutant, lived 0, not-covered 0, efficacy/coverage 100%.
- T-004 characterization shows event/schema and ADGO execution plan identity already exist; the missing Home Sentinel guarantee is immutable historical plan availability/routing across restart and upgrade.
- New F-013 blocks T-004 implementation: `internal/orchestration/` and the gate-policy engine `internal/engloop/` are not currently mutation-critical.
- Local full module execution remains unavailable in this agent environment because network dependency resolution is blocked; GitHub Actions is authoritative for complete build/race/security/mutation evidence.

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

SQLite       : application/auth/audit/config state + schema_migrations
ADGO/Pebble  : execution PlanID/PlanVersion/PlanDigest + durable history/state
Scenario     : AST -> validation -> Safety Compiler -> lowering -> catalog/simulator
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
| critical-diff mutation | PASS; config boundary proven executable |
| benchmark smoke | PASS |
| global coverage | not yet a release metric |
| target-hardware performance | OPEN T-008 |

Verified reference SHA: `26b28f4505f7b4cd4059c14d310b326b97cdef50`.

# 5. System Invariants

- **I-001** ML/VLM/LLM never possess human/physical authority.
- **I-002** External writes cross desired-state/idempotency/reconciliation gateways.
- **I-003** Ambiguous provider outcomes are never blindly replayed.
- **I-004** Human authority cannot be obtained from unsigned, unbound or machine/system input.
- **I-005** Physical ownership requires durable cross-execution control; process-local locks are not multi-process fencing.
- **I-006** Published Scenario semantics are version-pinned and Safety Compiler is mandatory.
- **I-007** Secrets are referenced; plaintext does not enter durable state/log/audit responses.
- **I-008** Non-terminal durable work survives compatible upgrade or fails/migrates explicitly.
- **I-009** Runtime DB/WAL/SHM/artifact/operator state is outside source control.
- **I-010** Every pushed `main` state is buildable and materially no worse than its parent.
- **I-011** Critical safety tests use observable synchronization, not scheduler luck.
- **I-012** Until TLS is actually served by production runtime, HTTP binds are loopback-only.
- **I-013** Human/service/system principals must be technically distinguishable before machine transports receive auth context.
- **I-014** HTTP request idempotency/staleness and workflow semantic idempotency are separate contracts.
- **I-015** Security-bearing configuration validators are mutation-critical and must create executable mutation work.
- **I-016** Safety/control orchestration and the engineering gate-policy engine are mutation-critical; code deciding whether mutation runs cannot itself escape test-of-tests.
- **I-017** Existing ADGO executions run only against their pinned PlanID/PlanDigest; a new active plan never silently reinterprets old durable state.

# 6. Findings Registry

## F-001 — Living execution plan was fragmented
**Status:** Resolved | **Severity:** High | **Confidence:** Confirmed  
Resolved by T-001.

## F-002 — Runtime SQLite/WAL state was tracked
**Status:** Resolved | **Severity:** High | **Confidence:** Confirmed  
Resolved by T-002.

## F-003 — Stage 17 recorded status was stale
**Status:** Resolved | **Severity:** Medium | **Confidence:** Confirmed  
Resolved by T-003 and `docs/STAGE17_RECONCILIATION.md`.

## F-004 — `main` lacks required-check technical protection
**Status:** Planned | **Severity:** High | **Confidence:** Confirmed  
GitHub reports `protected=false`; T-010.

## F-005 — Siren compensation race test depended on scheduler timing
**Status:** Resolved | **Severity:** High | **Confidence:** Confirmed  
Resolved by T-011.

## F-006 — Hardened remote-TLS rule was disconnected from runtime
**Status:** Resolved | **Severity:** High | **Confidence:** Confirmed  
T-012 rejects all non-loopback plaintext runtime binds; T-018 supplied real mutation evidence. Verified on `26b28f...`.

## F-007 — Callback semantics not wired to external HTTP transport
**Status:** Planned | **Severity:** High | **Confidence:** Strong  
T-013.

## F-008 — No explicit human/service/system principal kinds
**Status:** Planned | **Severity:** High | **Confidence:** Confirmed  
T-014.

## F-009 — Authorization audit/rate limits are path-specific
**Status:** Planned | **Severity:** Medium/High | **Confidence:** Confirmed  
T-015.

## F-010 — Authentication boundary is not pluggable
**Status:** Planned | **Severity:** Medium | **Confidence:** Confirmed  
T-017.

## F-011 — Common HTTP stale/idempotency contract absent
**Status:** Planned | **Severity:** High | **Confidence:** Confirmed  
T-016.

## F-012 — Security configuration was not mutation-targetable
**Status:** Resolved | **Severity:** High | **Confidence:** Confirmed  
T-018 made `internal/config/` CRITICAL, repaired Work Packet enums and proved `mutation_targets=["./internal/config"]`. Recovery Gremlins: killed 1, lived 0, not covered 0, efficacy 100%.

## F-013 — Safety orchestration and gate-policy code are not mutation-critical
**Status:** Planned | **Severity:** High | **Confidence:** Confirmed

**Category:** Testing / Safety / Engineering loop.  
**Evidence:** `isCriticalSurface` currently covers security/auth/config/gateway/scenario safety/etc. but not `internal/orchestration/` or `internal/engloop/`; `ClassifyPaths` therefore falls through to generic MEDIUM for e.g. `internal/orchestration/incident/service.go`, and `MutationTargets` returns no orchestration target.  
**Affected code:** incident lifecycle, Door/Siren physical workflows, camera recovery, resourceguard, lifecycle adapters, and the gate-classification code itself.  
**Root cause:** critical-surface taxonomy grew by feature rather than authority/durability boundary.  
**Impact:** T-004 plan routing/migration or physical workflow changes could pass without Gremlins; mutations to the code deciding whether mutation runs could also escape mutation testing.  
**Blast radius:** all future orchestration safety/versioning work.  
**Invariants:** I-002, I-003, I-005, I-008, I-010, I-016, I-017.  
**Task:** T-019; blocks T-004 implementation.

# 7. Risk Register

| Risk | Severity | Control |
|---|---|---|
| orchestration/gate policy escapes mutation | High | T-019 READY |
| in-flight workflow incompatible after upgrade | Critical | T-004 BLOCKED by T-019 |
| callback transport absent | High | T-013 |
| machine/human principal ambiguity | High | T-014 |
| stale/duplicate HTTP command | High | T-016 |
| external-effect crash linearization | Critical | T-005 |
| multi-process physical write race | Critical | T-006 |
| direct main without required checks | High | T-010 |
| release provenance/rollback | High | T-009 |
| target hardware budgets unknown | Medium/High | T-008 |

# 8. Pareto Improvements

1. T-019 protect orchestration + engloop with real mutation gating.
2. T-004 immutable pinned-plan catalog/routing across upgrade.
3. T-013/T-014 callback transport and principal authority.
4. T-005/T-006 crash/fencing guarantees.
5. T-016 common command preconditions before Scenario API expansion.
6. T-015/T-010 audit/abuse and verified-main controls.

# 9. Dependency DAG

```text
T-001 [DONE]
 +-- T-002 [DONE]
 |    +-- T-011 [DONE]
 |          +-- T-003 [DONE]
 |                +-- T-012 [DONE]
 |                |     +-- T-018 [DONE]
 |                |     +-- T-013 callback HTTP [READY]
 |                +-- T-019 orchestration mutation boundary [READY]
 |                |     +-- T-004 pinned-plan evolution [BLOCKED]
 |                |           +-- T-005 crash/restore
 |                |           +-- T-006 topology/fencing
 |                +-- T-014 principals
 |                |     +-- T-015 authz audit/limiting
 |                |     +-- T-017 authenticator boundary
 |                +-- T-016 command preconditions
 +-- T-010 verified-main guard

T-014 + T-016 + relevant T-013/T-004 contracts -> T-007 Scenario API
T-004/T-005/T-006 + T-008 -> T-009 release qualification
```

# 10. Implementation Phases

- **A Repository/security truth:** T-001/T-002/T-003/T-011/T-012/T-018/T-019/T-010.
- **B Durable evolution + identity/transport:** T-004..T-006 and T-013..T-017.
- **C Product surface:** T-007 and dependency-safe Scenario authoring.
- **D Operational qualification:** T-008/T-009 plus adapters/observability/soak/release gates.

# 11. Atomic Tasks

## T-001 — Establish living master plan
**Status:** DONE | **Priority:** P0. Verified `905862f...`.

## T-002 — Remove tracked runtime SQLite state
**Status:** DONE | **Priority:** P0. Verified `745f194...`.

## T-003 — Reconcile Stage 17 executable evidence
**Status:** DONE | **Priority:** P0. Verified `ef8ecaf...`.

## T-004 — Pin durable workflow plans across upgrade
**Status:** BLOCKED on T-019 | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH

**Characterization:** Home Sentinel already has SQLite `schema_migrations`, event `SchemaVersion`, ADGO plan IDs/versions, and ADGO executions persist `PlanID/PlanVersion/PlanDigest`. Pinned Axiom rejects plan-digest mismatch and already provides conservative `ValidatePlanMigration/MigrateExecution`.  
**Actual gap:** each Home Sentinel service opens only the current plan via `adgo.OpenProduction`; historical immutable plans are not retained/routed.  
**Goal:** new executions start on active plan; existing executions resolve exactly their persisted digest; unknown retired digest fails closed; migration remains explicit and uses Axiom migration APIs.  
**Critical edge:** Siren plan version includes configured max-activation duration, so a safe timeout config change creates a new plan/digest and must not strand an older waiting execution.  
**Tests:** old waiting execution reopen under changed active plan, unknown digest rejection, active/new start, explicit migration, parameterized siren plan retention, durable backend reopen, race + mutation.  
**Non-goal:** do not reimplement Axiom migration mathematics.

## T-005 — Prove crash/restore external-effect linearization
**Status:** TODO | **Priority:** P0. Depends T-004.

## T-006 — Enforce supported process topology/fencing
**Status:** TODO | **Priority:** P0. Depends T-004.

## T-007 — Expose authenticated Scenario API without bypasses
**Status:** BLOCKED | **Priority:** P1. Depends relevant T-004/T-013/T-014/T-016 contracts.

## T-008 — Establish target-hardware performance budgets
**Status:** TODO | **Priority:** P1.

## T-009 — Qualify release, upgrade and rollback
**Status:** BLOCKED | **Priority:** P0. Depends T-004/T-005/T-006/T-008 and remaining release prerequisites.

## T-010 — Add technical verified-main guard
**Status:** TODO | **Priority:** P1.

## T-011 — Make siren compensation verification deterministic under race
**Status:** DONE | **Priority:** P0. Verified `5ccff8bf...` first attempt.

## T-012 — Reject remote plaintext runtime bind until TLS is served
**Status:** DONE | **Priority:** P0 | **Type:** HARDEN  
Implementation `d57f239...`; final combined evidence `26b28f...`: ci/security/mutation PASS, config mutation target selected, Gremlins killed all relevant mutants.

## T-013 — Wire callback ingress through narrow HTTP bearer adapter
**Status:** READY | **Priority:** P0 | **Type:** HARDEN. Depends T-012 (satisfied).

## T-014 — Introduce explicit principal kinds/human-authority boundary
**Status:** TODO | **Priority:** P0 | **Type:** HARDEN.

## T-015 — Standardize authorization audit/principal-aware limiting
**Status:** BLOCKED | **Priority:** P1. Depends T-014 + callback transport.

## T-016 — Add common HTTP command precondition/idempotency contract
**Status:** TODO | **Priority:** P0 | **Type:** HARDEN.

## T-017 — Extract pluggable authenticator boundary
**Status:** BLOCKED | **Priority:** P2. Depends T-014.

## T-018 — Make security configuration mutation-critical
**Status:** DONE | **Priority:** P0 | **Type:** HARDEN  
Commit `26b28f4505f7b4cd4059c14d310b326b97cdef50`. Planner selected `./internal/config`; Gremlins killed 1, lived 0, not-covered 0; ci/security/mutation PASS.

## T-019 — Make safety orchestration and gate policy mutation-critical
**Status:** READY | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH

### Problem
`internal/orchestration/` and `internal/engloop/` currently fall outside `isCriticalSurface`; T-004 could otherwise change plan routing without mandatory mutation execution.

### Goal
Classify both boundaries CRITICAL and produce concrete mutation targets for production Go changes.

### Scope
`internal/engloop/model.go`, `model_test.go`, T-019 Work Packet, active packet, plan/status reconciliation.

### Invariants
I-010, I-016, I-017 plus physical/durable I-002/I-003/I-005/I-008.

### Implementation
- add `internal/orchestration/` and `internal/engloop/` to `isCriticalSurface`;
- architecture tests pin CRITICAL classification and exact target directories;
- verify a T-019 change to `internal/engloop/model.go` self-triggers mutation using the new taxonomy;
- future orchestration changes must enter `Critical diff mutation testing`.

### Tests / mutation
Unit architecture tests + full CI/race/security. Mutation is mandatory; planner for implementation commit must include `./internal/engloop`, and Gremlins must report no lived/not-covered blockers for changed gate-policy semantics.

### Acceptance
No production orchestration or gate-policy Go change can remain below CRITICAL or silently skip mutation.

# 12. Testing Strategy

G0 build/static/module; G1 unit/golden; G2 property/fuzz/model; G3 race; G4 contract/integration; G5 fault/crash; G6 mutation; G7 E2E/HIL/UX; G8 performance/soak/release.

Critical edge space: `input x identity x authority x time x ordering x concurrency x persistence x external failure x resource ownership x capacity x topology x version x cancellation x recovery x test-gate-classification`.

# 13. Mutation Testing Strategy

- Critical changed semantics require real Gremlins execution, not only a Work Packet label.
- Security config, orchestration/control workflows, gate-policy engine, principal policy, command preconditions, schema/migration guards and physical gateways are mutation-critical.
- Taxonomy architecture tests prevent critical paths from disappearing silently.
- Surviving/not-covered mutants require observable-contract analysis; never weaken thresholds merely to obtain green.

# 14. Performance Baselines

Current benchmark smoke is regression smoke, not an SLO/capacity claim. T-008 owns target-hardware latency, throughput and allocation budgets.

# 15. Security Hardening

Source/runtime-state hygiene DONE; deterministic siren safety DONE; Stage17 truth DONE; remote plaintext guard DONE; config mutation boundary DONE. Next: orchestration mutation boundary -> pinned-plan evolution -> callback/principal/precondition work -> crash/fencing -> verified-main/release qualification.

# 16. Migration Strategy

`characterize old -> retain immutable old contract -> introduce active new contract -> route existing state by durable identity -> explicit migration only at safe/quiescent point -> verify restart/rollback -> retire legacy only when no durable references remain`.

For ADGO, use persisted PlanID/PlanDigest and Axiom migration APIs; never silently substitute current plan for old execution state.

# 17. Deferred Work

- broad Scenario UI/UX until authority/API prerequisites;
- full remote TLS listener/certificate lifecycle after current loopback invariant;
- OIDC/mTLS implementations after T-017;
- multi-node support if v1 deliberately enforces single writer;
- low-leverage cosmetic refactors.

# 18. Rejected Decisions

- Roadmap checkbox as proof — rejected.
- Green rerun as flake resolution — rejected.
- Increase safety-test timeout — rejected.
- Expose callback HTTP before remote plaintext is impossible — rejected.
- Treat ADGO semantic idempotency as HTTP idempotency — rejected.
- Model service/system as fake users — rejected.
- Add unused TLS fields to claim remote security — rejected.
- Treat requested mutation as proof when planner selects no critical production boundary — rejected.
- Auto-migrate old ADGO executions to current plan on startup — rejected; migration must be explicit.
- Reimplement Axiom plan-migration logic in Home Sentinel — rejected.
- Force push — rejected.

# 19. Completed Tasks

T-001, T-002, T-003, T-011, T-012, T-018.

# 20. Iteration Log

- **1 / T-001:** `905862f...`, ci/security/mutation PASS.
- **2 / T-002:** `745f194...`, F-005 discovered; final verification PASS.
- **3 / F-005 planning:** `c30b395...`, gates PASS.
- **4 / T-011:** `5ccff8bf...`, first-attempt race/security/mutation PASS.
- **5 / T-003:** `ef8ecaf...`, Stage17 reconciliation, all gates PASS.
- **6 / T-012:** `d57f239...`, product ci/security PASS; mutation setup failure exposed F-012.
- **7 / T-018 recovery:** `26b28f4505f7b4cd4059c14d310b326b97cdef50`; ci/security/mutation PASS. Planner: CRITICAL + `./internal/config`; Gremlins killed 1/lived 0/not-covered 0, efficacy 100%. T-012/T-018 and F-006/F-012 resolved.
- **8 / closure + F-013 planning:** pending commit; closes previous tasks and inserts T-019 before T-004.

# 21. Definition of Final Done

- no unresolved Critical/High findings except explicit accepted deferral;
- all P0/P1 acceptance criteria verified;
- remote HTTP exposure is TLS-protected or technically impossible;
- critical config/orchestration/gate-policy validators have real mutation execution;
- human/service/system authority distinctions are explicit;
- durable plan upgrade/restart/rollback is proven;
- crash/fencing invariants exercised around external effects;
- stale/idempotency HTTP contracts precede broad mutable APIs;
- security/race/static/fault/mutation gates green;
- target-hardware budgets met;
- no unexplained flakes;
- docs match observed code;
- final re-audit finds no fundamental blocker;
- last verified state is on `main`, no force push.
