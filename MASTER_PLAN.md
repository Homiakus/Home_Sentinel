# Home Sentinel — Living MASTER PLAN

> Single execution source of truth for progressive audit/implementation. Detailed architecture/product plans remain intent specifications; this file owns findings, atomic tasks, ordering and iteration state reconciled against observed `main`.

**Plan revision:** 2026-08-28 / T-012 product code on `d57f239f64477f46c0b6ca7d324f0423596eaf8d` has ci/security PASS but mutation setup failed; T-018 recovery prepared.

---

# 1. Mission

Deliver a production-grade local security/automation platform via small reviewable changes while preserving fail-closed authority, durable external-effect semantics, reproducibility, rollback and evidence-driven evolution.

`AUDIT -> PLAN -> SELECT -> CHARACTERIZE -> IMPLEMENT -> TEST -> SELF-REVIEW -> RECONCILE -> COMMIT -> PUSH main -> VERIFY -> COMPRESS -> NEXT`

Material discoveries enter Findings and the dependency graph before scope expands.

# 2. Current State

- Go control plane: typed domain/event model, Axiom lifecycle, ADGO durable workflows, auth/authz, SQLite migrations/application state, gateways/recovery and Scenario model/compiler/safety/catalog/simulator.
- Media/data plane is not an authority plane; ML/VLM/LLM produce evidence only.
- Verified tasks: T-001 living plan, T-002 runtime DB hygiene, T-003 Stage 17 evidence reconciliation, T-011 deterministic siren compensation verification.
- T-012 implements loopback-only runtime binding until production actually serves TLS. On commit `d57f239...`, standard CI and security PASS; mutation FAILED before Gremlins because its Work Packet was invalid.
- That failure exposed F-012: security-bearing `internal/config/` was not a mutation target at all. T-018 is now the blocker that repairs the engineering loop before T-012 may be called verified.
- Complete local module verification is unavailable in this agent environment because dependency resolution is blocked; GitHub Actions is authoritative for complete build/test/security/mutation evidence.

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

SQLite: application/auth/audit/config-related state
ADGO/Pebble: durable workflow state/history
Scenario: AST -> validation -> Safety Compiler -> lowering -> catalog/simulator
```

Primary boundaries: `internal/domain`, `internal/orchestration`, `internal/gateway`, `internal/auth`, `internal/authz`, `internal/config`, `internal/database`, `internal/httpserver`, `internal/scenario`, `internal/integrations`, `internal/engloop`.

# 4. Baseline

| Gate | Baseline |
|---|---|
| module/build hygiene | PASS |
| formatting | PASS |
| vet/static | PASS |
| unit/integration | PASS |
| race | PASS; no active known flake |
| security qualification | PASS |
| critical-diff mutation | baseline PASS, T-012 iteration BLOCKED by F-012 before mutants ran |
| benchmark smoke | PASS |
| global coverage | not baselined as release metric |
| target-hardware performance | OPEN T-008 |

Pre-existing vs introduced discipline: `d57f239...` product CI/security are green; its mutation failure is classified as engineering-loop packet/targeting failure, not as a config validator regression.

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
- **I-014** HTTP request idempotency/staleness controls and workflow semantic idempotency are separate required contracts.
- **I-015** Security-bearing configuration validators are mutation-critical and must produce an executable mutation target; a CRITICAL Work Packet is not evidence if the mutation planner silently excludes its production code.

# 6. Findings Registry

## F-001 — Living execution plan was fragmented
**Status:** Resolved  
**Category:** Architecture / Process  
**Severity:** High  
**Confidence:** Confirmed  
**Resolution:** T-001.

## F-002 — Runtime SQLite/WAL state was tracked
**Status:** Resolved  
**Category:** Security / Data hygiene  
**Severity:** High  
**Confidence:** Confirmed  
**Resolution:** T-002 removed tracked state and ignored `/data/*.db*` while preserving first boot.

## F-003 — Stage 17 recorded status was stale
**Status:** Resolved  
**Category:** Planning / Correctness  
**Severity:** Medium  
**Confidence:** Confirmed  
**Resolution:** T-003 created clause-level evidence and residual F-006..F-011.

## F-004 — `main` lacks required-check technical protection
**Status:** Planned  
**Category:** CI/CD / Reliability  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** GitHub reports `protected=false`.  
**Task:** T-010.

## F-005 — Siren compensation race test depended on scheduler timing
**Status:** Resolved  
**Category:** Testing / Reliability  
**Severity:** High  
**Confidence:** Confirmed  
**Resolution:** T-011 deterministic workflow-state verification; first post-push race/security/mutation green.

## F-006 — Hardened remote-TLS rule disconnected from runtime
**Status:** Planned / VERIFYING via T-012  
**Category:** Security / Configuration / HTTP  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** hardened config rejects non-loopback without TLS while runtime formerly accepted arbitrary bind and production uses plaintext `ListenAndServe`.  
**Current behavior after T-012 code:** loopback-only validator is present and ci/security PASS on `d57f239...`; required mutation evidence is blocked by F-012.  
**Affected invariants:** I-004, I-010, I-012, I-015.  
**Tasks:** T-012, T-018 blocker.

## F-007 — Callback semantics not wired to external HTTP transport
**Status:** Planned  
**Category:** Security / Integration  
**Severity:** High  
**Confidence:** Strong  
**Task:** T-013.

## F-008 — No explicit human/service/system principal kinds
**Status:** Planned  
**Category:** Authorization / Architecture  
**Severity:** High  
**Confidence:** Confirmed  
**Task:** T-014.

## F-009 — Authorization audit/rate limits are path-specific
**Status:** Planned  
**Category:** Security / Audit / Abuse resistance  
**Severity:** Medium/High  
**Confidence:** Confirmed  
**Task:** T-015.

## F-010 — Authentication boundary is not pluggable
**Status:** Planned  
**Category:** Authentication / Architecture  
**Severity:** Medium  
**Confidence:** Confirmed  
**Task:** T-017.

## F-011 — Common HTTP stale/idempotency contract absent
**Status:** Planned  
**Category:** Correctness / API  
**Severity:** High  
**Confidence:** Confirmed  
**Task:** T-016.

## F-012 — Security configuration was not mutation-targetable

**Status:** Planned / IN_PROGRESS via T-018  
**Category:** Testing / Security / Engineering loop  
**Severity:** High  
**Confidence:** Confirmed

**Evidence:** T-012 mutation job failed during `active-packet` validation because `status_before:"READY"` is not a WorkPacket `PlanState`. Inspection then showed a deeper defect: `ClassifyPaths` treated `internal/config/` only as HIGH and `MutationTargets` only emitted paths accepted by `isCriticalSurface`, which excluded `internal/config/`.

**Files / symbols:** `docs/engineering/work-packets/t012-runtime-bind-security.json`, `internal/engloop/model.go::{validPlanState,validGate,ClassifyPaths,MutationTargets,isCriticalSurface}`, `internal/engloop/model_test.go`, `.github/workflows/mutation.yml`.

**Current behavior:** a CRITICAL packet can request mutation while a production security config change yields no `./internal/config` mutation target; workflow then may skip Gremlins entirely. The specific T-012 packet additionally used invalid enums (`READY`, `mutation`).

**Expected behavior:** security config changes classify CRITICAL, produce `./internal/config`, require `mutation-critical`, and packet validation catches schema errors before a change is relied upon.

**Root cause:** engineering-loop critical-surface taxonomy omitted configuration even though it carries secret-reference, remote-exposure and fail-closed validation policy; execution task states were accidentally reused as WorkPacket states.

**Impact:** false confidence in test-of-tests for security validators; T-012 cannot satisfy its acceptance criteria.

**Blast radius:** all current/future `internal/config` security validation, not only remote bind.

**Reproduction:** compare `internal/config/model.go` against `isCriticalSurface`; run active packet from T-012 commit; mutation planning fails before Gremlins.

**Affected invariants:** I-010, I-012, I-015.

**Affected tasks:** T-012 blocked; T-018 foundational blocker.

**Recommended direction:** make `internal/config/` a critical surface, lock this through engloop architecture tests, repair packets, keep mutation base before T-012 so Gremlins actually attacks the validator.

# 7. Risk Register

| Risk | Severity | Control |
|---|---|---|
| security config can escape mutation test-of-tests | High | T-018 IN_PROGRESS |
| remote plaintext control surface | High | T-012 code implemented; verification blocked by T-018 |
| callback transport absent | High | T-013 |
| machine/human principal ambiguity | High | T-014 |
| stale/duplicate HTTP command | High | T-016 |
| in-flight workflow break on upgrade | Critical | T-004 |
| external-effect crash linearization | Critical | T-005 |
| multi-process physical write race | Critical | T-006 |
| direct main without required checks | High | T-010 |
| release provenance/rollback | High | T-009 |
| target hardware budgets unknown | Medium/High | T-008 |

# 8. Pareto Improvements

1. T-018 repair mutation targeting, then close T-012 with real Gremlins evidence.
2. T-004 durable schema/plan versioning.
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
 |                +-- T-012 implementation [VERIFYING]
 |                |     +-- T-018 mutation-boundary blocker [IN_PROGRESS]
 |                |            +-- T-012 final verification
 |                |                   +-- T-013 callback HTTP
 |                +-- T-014 principals
 |                |     +-- T-015 authz audit/limiting
 |                |     +-- T-017 authenticator boundary
 |                +-- T-016 command preconditions
 |                +-- T-004 schema/plan versioning
 |                       +-- T-005 crash/restore
 |                       +-- T-006 topology/fencing
 +-- T-010 verified-main guard

T-014 + T-016 + relevant T-013/T-004 contracts -> T-007 Scenario API
T-004/T-005/T-006 + T-008 -> T-009 release qualification
```

# 10. Implementation Phases

- **A Repository/security truth:** T-001/T-002/T-011/T-003/T-012/T-018/T-010.
- **B Identity/transport + durable evolution:** T-013..T-017 and T-004..T-006.
- **C Product surface:** T-007 plus dependency-safe Scenario authoring.
- **D Operational qualification:** T-008/T-009 and remaining adapters/observability/soak/release gates.

# 11. Atomic Tasks

## T-001 — Establish living master plan
**Status:** DONE | **Priority:** P0 | **Type:** IMPROVE | **Leverage:** HIGH  
Verified on `905862f...`.

## T-002 — Remove tracked runtime SQLite state
**Status:** DONE | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH  
Verified on `745f194...`.

## T-003 — Reconcile Stage 17 executable evidence
**Status:** DONE | **Priority:** P0 | **Type:** IMPROVE | **Leverage:** HIGH  
Verified on `ef8ecaf44d114c5e6341dab5fa6b869952407421`.

## T-004 — Pin durable schemas and workflow plans across upgrade
**Status:** TODO | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH  
**Goal:** version external event/lifecycle/ADGO plan+registry state so compatible in-flight work continues and incompatible evolution migrates/fails explicitly.  
**Tests:** golden old/new fixtures, waiting-execution reopen, unknown version rejection, backup/restore/rollback; mutation for version guards.  
**Dependencies:** T-003; next foundational Critical track after T-012 closes.

## T-005 — Prove crash/restore external-effect linearization
**Status:** TODO | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH  
Depends T-004.

## T-006 — Enforce supported process topology/fencing
**Status:** TODO | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH  
Depends T-004.

## T-007 — Expose authenticated Scenario API without bypasses
**Status:** BLOCKED | **Priority:** P1 | **Type:** IMPROVE | **Leverage:** HIGH  
Depends relevant T-004/T-013/T-014/T-016 contracts.

## T-008 — Establish target-hardware performance budgets
**Status:** TODO | **Priority:** P1 | **Type:** IMPROVE | **Leverage:** MEDIUM.

## T-009 — Qualify release, upgrade and rollback
**Status:** BLOCKED | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH  
Depends T-004/T-005/T-006/T-008 and remaining P0 production prerequisites.

## T-010 — Add technical verified-main guard
**Status:** TODO | **Priority:** P1 | **Type:** HARDEN | **Leverage:** HIGH.

## T-011 — Make siren compensation verification deterministic under race
**Status:** DONE | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH  
Verified on `5ccff8bf...` first attempt.

## T-012 — Reject remote plaintext runtime bind until TLS is served
**Status:** BLOCKED / VERIFYING | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH

### Problem
Production runtime uses plaintext `ListenAndServe`; prior runtime config accepted remote binds.

### Implementation
`Config.Validate` now parses host:port and accepts only package-defined loopback hosts; remote/unspecified/wildcard hosts return `ErrInsecureRemoteBind`; malformed address is classified separately. Default is unchanged.

### Verification status
Commit `d57f239...`: standard ci PASS; security PASS; mutation planning FAILED before Gremlins due to F-012. Product code is not reverted, but task cannot be DONE until a real mutation campaign covers it.

### Dependencies
T-003; verification blocked by T-018.

### Acceptance
Remote plaintext impossible through runtime config AND mutants weakening that security validator do not survive.

## T-013 — Wire callback ingress through narrow HTTP bearer adapter
**Status:** BLOCKED | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH  
Depends final T-012 verification.

## T-014 — Introduce explicit principal kinds/human-authority boundary
**Status:** TODO | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH.

## T-015 — Standardize authorization audit/principal-aware limiting
**Status:** BLOCKED | **Priority:** P1 | **Type:** HARDEN | **Leverage:** MEDIUM/HIGH  
Depends T-014 and relevant T-013 transport.

## T-016 — Add common HTTP command precondition/idempotency contract
**Status:** TODO | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH.

## T-017 — Extract pluggable authenticator boundary
**Status:** BLOCKED | **Priority:** P2 | **Type:** IMPROVE | **Leverage:** MEDIUM  
Depends T-014.

## T-018 — Make security configuration mutation-critical

**Status:** IN_PROGRESS  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

### Problem
T-012 revealed that the mutation workflow could not honestly test `internal/config`: its packet was schema-invalid and config production code was outside `MutationTargets`.

### Evidence
Mutation job on `d57f239...` failed during `active-packet` validation with invalid `status_before "READY"`; `engloop/model.go` allows only `OPEN/PARTIAL/IMPLEMENTED_UNVERIFIED/VERIFIED/BLOCKED/STALE/SUPERSEDED`, and the mutation gate literal is `mutation-critical`. `isCriticalSurface` omitted `internal/config/`.

### Goal
Make security-sensitive configuration a first-class test-of-tests boundary and recover T-012 without changing its product semantics.

### Scope
`internal/engloop/model.go`, `model_test.go`, T-012/T-018 Work Packets, active packet, plan/status docs.

### Non-goals
No config product-policy change beyond already-pushed T-012; no Gremlins weakening/skip; no workflow rerun before root cause is fixed.

### Implementation
1. add `internal/config/` to `isCriticalSurface` and remove it from the weaker HIGH-only bucket;
2. architecture tests assert CRITICAL classification, `./internal/config` mutation target and mutation/security gates;
3. repair T-012 packet to `status_before: OPEN` and `mutation-critical`;
4. make T-018 the active valid CRITICAL packet;
5. keep mutation base `ef8ecaf...` so the next Gremlins diff includes T-012 validator lines.

### Invariants
I-010, I-012, I-015.

### Compatibility constraints
Only engineering-loop classification/gates become stricter; production runtime behavior remains T-012 behavior.

### Edge cases
`_test.go` does not itself become a mutation target; config production file always does; packet enum mismatch fails early; active base must be ancestor and include at least one commit.

### Tests
Engloop unit tests; active-packet validation; full CI/race/security; mutation workflow must progress into `Critical diff mutation testing` and upload evidence.

### Mutation tests
Mandatory. Recovery campaign rooted at `ef8ecaf...` must cover the T-012 config diff and T-018 engloop diff.

### Acceptance criteria
No config security change can be classified below CRITICAL; `MutationTargets` returns `./internal/config`; repaired packet validates; Gremlins executes and mutation evidence passes; standard ci/security also pass.

### Verification commands
`go test ./internal/engloop ./internal/config -count=1`; `go run ./cmd/sentinel-engloop active-packet --root . --json`; full CI/race/security; `make mutation-diff BASE=ef8ecaf44d114c5e6341dab5fa6b869952407421`.

### Dependencies
T-012 failure evidence; blocks T-012 closure.

### Risk
Medium: stricter test classification may increase future mutation cost, intentionally for security config.

### Rollback
Do not rollback config criticality unless security-bearing config is moved to another explicit critical boundary with equivalent mutation targeting.

# 12. Testing Strategy

G0 build/static/module; G1 unit/golden; G2 property/fuzz/model; G3 race; G4 contract/integration; G5 fault/crash; G6 mutation; G7 E2E/HIL/UX; G8 performance/soak/release.

Critical edge space:
`input x identity x authority x time x ordering x concurrency x persistence x external failure x resource ownership x capacity x topology x version x cancellation x recovery x test-infrastructure-classification`.

# 13. Mutation Testing Strategy

- Risk-scoped changed critical semantics require Gremlins evidence.
- A CRITICAL Work Packet is insufficient unless `MutationTargets` actually selects the production code.
- Security configuration (`internal/config/`) is now a mandatory critical mutation boundary via T-018.
- Principal policy (T-014), HTTP command preconditions (T-016), schema/migration guards (T-004) are mandatory mutation targets.
- Surviving mutants require observable-contract analysis, not coverage inflation.

# 14. Performance Baselines

Current benchmark smoke is regression smoke only, not an SLO/capacity claim. T-008 owns target-hardware budgets.

# 15. Security Hardening

Sequence: source/runtime state DONE -> deterministic safety tests DONE -> Stage17 truth DONE -> remote plaintext guard implemented -> mutation-boundary recovery T-018 -> close T-012 -> callback transport/principal model -> schema/crash/fencing -> command preconditions/audit -> main/release qualification. Never weaken security/race/mutation gates to get green.

# 16. Migration Strategy

`characterize old -> introduce safe/versioned boundary -> dual compatibility only where safe -> migrate callers/state -> restart/rollback verify -> delete legacy after durable dependency is gone`.

For active security exposure, restrictive fail-closed behavior is preferred over half-wired TLS.

# 17. Deferred Work

- broad Scenario UI/UX until authority/API prerequisites;
- full TLS listener/certificate lifecycle after loopback guard is proven;
- OIDC/mTLS implementations after T-017;
- multi-node support if v1 enforces single writer;
- low-leverage refactors.

# 18. Rejected Decisions

- Roadmap checkbox as proof — rejected.
- Green rerun as flake resolution — rejected.
- Increase safety-test timeout — rejected.
- Expose callback HTTP before remote plaintext is impossible — rejected.
- Treat ADGO semantic idempotency as HTTP command idempotency — rejected.
- Model service/system as fake users — rejected.
- Add unused TLS fields to claim remote security — rejected.
- Treat a CRITICAL Work Packet requesting mutation as proof when `MutationTargets` is empty — rejected after F-012.
- Blindly rerun the failed T-012 mutation job — rejected; fix root cause first.
- Force push — rejected.

# 19. Completed Tasks

T-001, T-002, T-003, T-011. T-012 product implementation exists but is not final-DONE. T-018 is active blocker.

# 20. Iteration Log

## Iteration 1
T-001 — `905862f...`; ci/security/mutation PASS.

## Iteration 2
T-002 — `745f194...`; F-005 discovered; final classification/verification PASS.

## Iteration 3
F-005 planning — `c30b395...`; gates PASS.

## Iteration 4
T-011 — `5ccff8bf...`; first-attempt ci/race/security/mutation PASS; F-005 resolved.

## Iteration 5
T-003 — `ef8ecaf44d114c5e6341dab5fa6b869952407421`; Stage17 reconciliation; ci/security/mutation PASS.

## Iteration 6
**Task:** T-012  
**Finding:** F-006  
**Commit:** `d57f239f64477f46c0b6ca7d324f0423596eaf8d` — `fix(config): reject remote plaintext runtime bind`  
**Tests:** standard ci PASS; security PASS; mutation FAILED before Gremlins because Work Packet used invalid `READY`; investigation exposed F-012 empty config mutation boundary.  
**Plan changes:** T-012 remains VERIFYING/BLOCKED; T-018 inserted as foundational blocker.  
**Push:** main  
**Result:** BLOCKED BY TEST INFRASTRUCTURE, product checks PASS

## Iteration 7
**Task:** T-018  
**Findings addressed:** F-012  
**Changes:** config becomes mutation-critical; architecture tests; repaired T-012 packet; dedicated T-018 packet with mutation base before T-012.  
**Tests:** pending post-push.  
**Commit:** pending creation  
**Push:** pending main  
**Result:** VERIFYING

# 21. Definition of Final Done

- no unresolved Critical/High findings except explicit accepted deferral;
- all P0/P1 acceptance criteria verified;
- remote HTTP exposure is TLS-protected or technically impossible;
- security config and other critical validators have real mutation targets, not nominal gates;
- human/service/system authority distinctions are explicit;
- durable schema/plan upgrade+rollback proven;
- crash/fencing invariants exercised around external effects;
- stale/idempotency HTTP contracts precede broad mutable APIs;
- security/race/static/fault/mutation gates green;
- target-hardware budgets met;
- no unexplained flakes;
- docs match observed code;
- final re-audit finds no fundamental blocker;
- last verified state is on `main`, no force push.
