# Home Sentinel — Living MASTER PLAN

> Single execution source of truth for progressive audit/implementation. Detailed architecture/product plans remain intent specifications; this file owns findings, atomic tasks, ordering and iteration status reconciled against observed `main`.

**Plan revision:** 2026-08-28 / T-003 verified on `ef8ecaf44d114c5e6341dab5fa6b869952407421`; T-012 implementation prepared for verification.

---

# 1. Mission

Deliver a production-grade local security/automation platform via small reviewable changes while preserving fail-closed authority, durable external-effect semantics, reproducibility, rollback and evidence-driven evolution.

`AUDIT -> PLAN -> SELECT -> CHARACTERIZE -> IMPLEMENT -> TEST -> SELF-REVIEW -> RECONCILE -> COMMIT -> PUSH main -> VERIFY -> COMPRESS -> NEXT`

Material discoveries always enter Findings and the dependency graph before scope expands.

# 2. Current State

- Go control plane: typed domain/event model, Axiom lifecycle, ADGO durable workflows, auth/authz, SQLite migrations/application state, gateways/recovery and Scenario model/compiler/safety/catalog/simulator.
- Media/data plane is not an authority plane; ML/VLM/LLM produce evidence only.
- T-001 living-plan, T-002 runtime-DB hygiene and T-011 deterministic siren compensation are verified.
- T-003 Stage 17 reconciliation is verified on `ef8ecaf...`; exact evidence lives in `docs/STAGE17_RECONCILIATION.md`.
- T-012 implements the next P0: current plaintext runtime becomes loopback-only until a real TLS listener exists. No fake/unused TLS configuration is added.
- Complete local module verification is unavailable in this agent environment because dependency resolution is blocked; GitHub Actions is authoritative for full build/test/security/mutation.

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
| critical-diff mutation | PASS |
| benchmark smoke | PASS |
| global coverage | not baselined as release metric |
| target-hardware performance | OPEN T-008 |

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

# 6. Findings Registry

## F-001 — Living execution plan was fragmented
**Status:** Resolved | **Severity:** High | **Confidence:** Confirmed  
Resolved by T-001.

## F-002 — Runtime SQLite/WAL state was tracked
**Status:** Resolved | **Severity:** High | **Confidence:** Confirmed  
Resolved by T-002 (`/data/*.db*` ignored; runtime files removed; first boot preserved).

## F-003 — Stage 17 recorded status was stale
**Status:** Resolved | **Severity:** Medium | **Confidence:** Confirmed  
T-003 replaced the stale sentence with a clause matrix and split residual work into F-006..F-011.

## F-004 — `main` lacks required-check technical protection
**Status:** Planned | **Severity:** High | **Confidence:** Confirmed  
GitHub reports `protected=false`; T-010 must preserve mandated direct-to-main flow without force push.

## F-005 — Siren compensation race test depended on scheduler timing
**Status:** Resolved | **Severity:** High | **Confidence:** Confirmed  
T-011 replaced background polling/deadlines with synchronous workflow state assertions; first post-push CI/race/security/mutation passed.

## F-006 — Hardened remote-TLS rule disconnected from runtime
**Status:** VERIFYING via T-012 | **Severity:** High | **Confidence:** Confirmed  
**Evidence:** hardened config rejects non-loopback without TLS; runtime config previously accepted any non-empty bind and server uses plaintext `ListenAndServe`.  
**Root cause:** hardened/runtime configuration migration incomplete.  
**Impact:** credentials/control routes could be intentionally exposed over remote plaintext.  
**Resolution being verified:** current runtime now parses host:port and accepts only loopback; full TLS remains a separate future capability.  
**Invariants:** I-004, I-010, I-012.

## F-007 — Callback semantics not wired to external HTTP transport
**Status:** Planned | **Severity:** High | **Confidence:** Strong  
`CallbackSecurity` and tested orchestration `CallbackIngress` exist, but production HTTP/app wiring has no callback transport. T-013.

## F-008 — No explicit human/service/system principal kinds
**Status:** Planned | **Severity:** High | **Confidence:** Confirmed  
Current session principal is a user and roles are viewer/operator/admin; callbacks correctly require `usr_*`, but future machine transports lack a first-class authority type. T-014.

## F-009 — Authorization audit/rate limits are path-specific
**Status:** Planned | **Severity:** Medium/High | **Confidence:** Confirmed  
Callback allow/deny is durable/fail-closed; generic capability middleware is not audited. Rate limiter is scope+IP only. T-015.

## F-010 — Authentication boundary is not pluggable
**Status:** Planned | **Severity:** Medium | **Confidence:** Confirmed  
Local password/session auth is concrete; no local/OIDC/mTLS authenticator boundary. T-017 after principal model.

## F-011 — Common HTTP stale/idempotency contract absent
**Status:** Planned | **Severity:** High | **Confidence:** Confirmed  
No common ETag/If-Match/expected-version or Idempotency-Key command contract. T-016.

# 7. Risk Register

| Risk | Severity | Control |
|---|---|---|
| remote plaintext control surface | High | T-012 VERIFYING |
| callback transport not externally wired | High | T-013 |
| machine/human principal ambiguity | High | T-014 |
| stale/duplicate HTTP command | High | T-016 |
| in-flight workflow break on upgrade | Critical | T-004 |
| external-effect crash linearization | Critical | T-005 |
| multi-process physical write race | Critical | T-006 |
| direct main without required checks | High | T-010 |
| release provenance/rollback | High | T-009 |
| target hardware budgets unknown | Medium/High | T-008 |

# 8. Pareto Improvements

1. Finish T-012 remote plaintext fail-closed guard.
2. T-004 durable schema/plan versioning.
3. T-013/T-014 callback transport and principal authority.
4. T-005/T-006 crash/fencing guarantees.
5. T-016 common command preconditions before Scenario API expansion.
6. T-015/T-010 audit/abuse and verified-main guard.

# 9. Dependency DAG

```text
T-001 [DONE]
 +-- T-002 [DONE]
 |    +-- T-011 [DONE]
 |          +-- T-003 [DONE]
 |                +-- T-012 [VERIFYING]
 |                |     +-- T-013 callback HTTP
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

- **A Repository/security truth:** T-001/T-002/T-011/T-003/T-012/T-010.
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

## T-003 — Reconcile Stage 17 against executable evidence
**Status:** DONE | **Priority:** P0 | **Type:** IMPROVE | **Leverage:** HIGH  
**Result:** exact matrix in `docs/STAGE17_RECONCILIATION.md`; stale status corrected; F-006..F-011/T-012..T-017 created.  
**Verification:** `ci/security/mutation` PASS on `ef8ecaf44d114c5e6341dab5fa6b869952407421`.

## T-004 — Pin durable schemas and workflow plans across upgrade
**Status:** TODO | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH  
**Goal:** version external event/lifecycle/ADGO plan+registry state so compatible in-flight work continues and incompatible evolution migrates/fails explicitly.  
**Tests:** golden old/new fixtures, waiting-execution reopen, unknown version rejection, backup/restore/rollback; mutation on version guards.  
**Dependencies:** T-003; can proceed independently of later HTTP slices.

## T-005 — Prove crash/restore external-effect linearization
**Status:** TODO | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH  
Fault-inject enqueue/claim/provider-accept/local-commit/disk/restore; no blind replay after ambiguity. Depends T-004.

## T-006 — Enforce supported process topology/fencing
**Status:** TODO | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH  
Enforce single-writer or genuine distributed fencing before multi-writer physical control. Depends T-004.

## T-007 — Expose authenticated Scenario API without bypasses
**Status:** BLOCKED | **Priority:** P1 | **Type:** IMPROVE | **Leverage:** HIGH  
Depends on principal/precondition/version/transport contracts from T-014/T-016 and relevant T-004/T-013 work.

## T-008 — Establish target-hardware performance budgets
**Status:** TODO | **Priority:** P1 | **Type:** IMPROVE | **Leverage:** MEDIUM.

## T-009 — Qualify release, upgrade and rollback
**Status:** BLOCKED | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH  
Depends T-004/T-005/T-006/T-008 and remaining P0 production prerequisites.

## T-010 — Add technical verified-main guard
**Status:** TODO | **Priority:** P1 | **Type:** HARDEN | **Leverage:** HIGH  
Preserve direct-to-main iteration while preventing failing state from being treated as accepted release state.

## T-011 — Make siren compensation verification deterministic under race
**Status:** DONE | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH  
Verified on `5ccff8bf...` first attempt, including race/security/mutation.

## T-012 — Reject remote plaintext runtime bind until TLS is served
**Status:** VERIFYING | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH

### Problem
Production runtime used plaintext `ListenAndServe` while runtime `Config.Validate` allowed arbitrary non-empty listen addresses.

### Evidence
F-006 and Stage 17 reconciliation.

### Goal
Technically enforce I-012 now instead of leaving a knowingly insecure interim state.

### Scope
`internal/config/model.go`, `model_test.go`, dedicated engineering Work Packet/active packet, status/master-plan reconciliation.

### Non-goals
No TLS certificate fields, TLS listener, reverse-proxy assumptions or callback endpoint in this slice.

### Implementation
`Config.Validate` parses `Server.ListenAddress` with `net.SplitHostPort`; malformed values fail validation; non-loopback hosts return `ErrInsecureRemoteBind`; reuse the already-tested package `loopbackHost` semantics.

### Invariants
I-004, I-010, I-012.

### Compatibility
Default loopback unchanged. Existing plaintext remote binds intentionally stop at startup.

### Edge cases
`localhost`, IPv4 127/8, `::1`; reject IPv4/IPv6 unspecified, LAN, arbitrary hostname and empty-host wildcard; malformed host:port classified separately.

### Tests
Table tests plus full CI/race/security; dedicated mutation range rooted at T-003 commit.

### Mutation tests
Required. Active Work Packet points mutation base at `ef8ecaf...` so Gremlins targets this validator change.

### Acceptance
Remote plaintext cannot be configured through production `Config`; default remains valid; mutation campaign does not leave a weakened guard alive.

### Rollback
Only after a verified production TLS listener replaces I-012 with an equally strong remote-TLS invariant.

## T-013 — Wire callback ingress through narrow HTTP bearer adapter
**Status:** BLOCKED | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH  
Depends T-012. Preserve exact binding/event ID, strict body bounds, callback bearer authority, rate limits and audited orchestration ingress.

## T-014 — Introduce explicit principal kinds/human-authority boundary
**Status:** TODO | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH  
Distinguish user/service/system; machine principals cannot acquire human-only capabilities. Mutation required.

## T-015 — Standardize authorization audit/principal-aware limiting
**Status:** BLOCKED | **Priority:** P1 | **Type:** HARDEN | **Leverage:** MEDIUM/HIGH  
Depends T-014 and callback transport for callback limiting.

## T-016 — Add common HTTP command precondition/idempotency contract
**Status:** TODO | **Priority:** P0 | **Type:** HARDEN | **Leverage:** HIGH  
Expected-version/ETag stale rejection + request idempotency semantics; mutation required.

## T-017 — Extract pluggable authenticator boundary
**Status:** BLOCKED | **Priority:** P2 | **Type:** IMPROVE | **Leverage:** MEDIUM  
After T-014; no OIDC/mTLS authority claim before then.

# 12. Testing Strategy

G0 build/static/module; G1 unit/golden; G2 property/fuzz/model; G3 race; G4 contract/integration; G5 fault/crash; G6 mutation; G7 E2E/HIL/UX; G8 performance/soak/release.

Critical edge space:
`input x identity x authority x time x ordering x concurrency x persistence x external failure x resource ownership x capacity x topology x version x cancellation x recovery`.

# 13. Mutation Testing Strategy

Risk-scoped changed critical semantics require Gremlins evidence. Security validators (T-012), principal policy (T-014), command preconditions (T-016), version/migration guards (T-004) are mandatory mutation targets. Surviving mutants require observable-contract analysis, not coverage inflation.

# 14. Performance Baselines

Current benchmark smoke is regression smoke only, not SLO/capacity evidence. T-008 owns target-hardware budgets.

# 15. Security Hardening

Sequence: source/runtime state DONE -> deterministic safety tests DONE -> Stage17 truth DONE -> remote plaintext guard T-012 -> callback transport/principal model -> schema/crash/fencing -> command preconditions/audit -> main/release qualification. Never weaken security/race/mutation gates to get green.

# 16. Migration Strategy

`characterize old -> introduce safe/versioned boundary -> dual compatibility only where safe -> migrate callers/state -> restart/rollback verify -> delete legacy after durable dependency is gone`.

For active security exposure, a restrictive compatible subset (loopback-only) is preferred over half-wired TLS.

# 17. Deferred Work

- broad Scenario UI/UX until authority/API prerequisites;
- full TLS listener/certificate lifecycle as a separate reviewed task after T-012;
- OIDC/mTLS implementations after T-017;
- multi-node support if v1 explicitly enforces single writer;
- low-leverage refactors.

# 18. Rejected Decisions

- Roadmap checkbox as proof — rejected.
- Green rerun as flake resolution — rejected; T-011 fixed root cause.
- Increase safety-test timeout — rejected.
- Expose callback HTTP before remote plaintext is impossible — rejected.
- Treat ADGO semantic idempotency as HTTP command idempotency/stale protection — rejected.
- Model service/system as fake users — rejected pending T-014.
- Add unused certificate/TLS fields to claim security — rejected.
- Force push — rejected.

# 19. Completed Tasks

T-001, T-002, T-003, T-011. T-012 is implemented and waiting on post-push verification.

# 20. Iteration Log

## Iteration 1
T-001 — commit `905862f...`; ci/security/mutation PASS; main PASS.

## Iteration 2
T-002 — commit `745f194...`; F-005 discovered; same-SHA rerun classified flake; final gates PASS.

## Iteration 3
F-005 planning — commit `c30b395...`; gates PASS.

## Iteration 4
T-011 — commit `5ccff8bf...`; first-attempt ci/race/security/mutation PASS; F-005 resolved.

## Iteration 5
T-003 — commit `ef8ecaf44d114c5e6341dab5fa6b869952407421`; Stage17 matrix/status decomposition; F-006..F-011 created; ci/security/mutation PASS.

## Iteration 6
**Task:** T-012  
**Finding:** F-006  
**Changes:** production runtime listen validation becomes loopback-only until TLS is actually served; negative table tests; dedicated mutation Work Packet/range.  
**Tests:** post-push ci/security/mutation pending.  
**Commit:** pending creation  
**Push:** pending main  
**Result:** VERIFYING

# 21. Definition of Final Done

- no unresolved Critical/High findings except explicitly accepted deferral;
- all P0/P1 acceptance criteria verified;
- remote HTTP exposure is TLS-protected or technically impossible;
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
