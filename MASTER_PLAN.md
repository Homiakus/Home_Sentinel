# Home Sentinel — Living MASTER PLAN

> Single execution source of truth for the progressive audit/implementation loop. Detailed intent documents remain normative specifications; findings, atomic tasks, ordering and iteration state are reconciled here against observed `main`.

**Plan revision:** 2026-08-28 / T-011 verified on `5ccff8bf89508da86f82e77703f11586eb7bc8a0`; T-003 Stage 17 evidence reconciliation prepared.

---

# 1. Mission

Evolve Home Sentinel into a production-grade local security/automation platform through small reviewable changes while preserving fail-closed authority, durable external-effect semantics, reproducibility, rollback and evidence-driven evolution.

Loop:

`AUDIT -> PLAN -> SELECT -> CHARACTERIZE -> IMPLEMENT -> TEST -> SELF-REVIEW -> RECONCILE -> COMMIT -> PUSH main -> VERIFY -> COMPRESS -> NEXT`

Material discoveries always follow `Finding -> impact analysis -> plan reconciliation -> decision` before scope expands.

# 2. Current State

- Go control plane: domain/event model, Axiom lifecycle, ADGO durable workflows, auth/authz, SQLite application state/migrations, gateways/recovery, Scenario model/compiler/safety/catalog/simulator.
- Media/data plane is separated from control authority; ML/VLM/LLM are evidence producers, never human/physical authority.
- T-001 established this living plan.
- T-002 removed tracked runtime SQLite/WAL/SHM state.
- T-011 replaced a scheduler-timing siren compensation test with a deterministic workflow contract and passed `ci`, `security` and `mutation` on the first post-push attempt.
- Stage 17 has been re-audited clause-by-clause. Callback binding/audit/exact resume are substantially proven; several runtime transport/principal/API contracts remain open. Detailed matrix: `docs/STAGE17_RECONCILIATION.md`.
- The production runtime still accepts non-loopback plaintext listen addresses even though `HardenedServerConfig` already encodes a remote-TLS requirement. This is the next fail-closed P0 slice.
- Local full-module execution is unavailable in this agent environment because outbound dependency resolution is blocked; GitHub Actions remains the authoritative complete verification environment. Standalone local formatting/file checks are still used where possible.

# 3. Architecture Map

```text
Cameras / Sensors / HA / Intercom
             |
             v
Media + CV + normalization               [data plane]
             |
       typed domain events
             |
   +---------+----------+
   |                    |
   v                    v
Axiom lifecycle      ADGO workflows      [control plane]
   |                    |
   +----- principal/policy/RBAC ---------+
             |
      Invocation Gateways
             |
     external / physical effects

Application state: SQLite + migrations
Workflow state: ADGO/Pebble durable execution/history
Media: content-addressed artifact references, not workflow blobs
Scenario: AST -> validate/type/time -> Safety Compiler -> Axiom/ADGO -> catalog/simulator
```

Primary boundaries: `internal/domain`, `internal/orchestration`, `internal/gateway`, `internal/auth`, `internal/authz`, `internal/database`, `internal/config`, `internal/httpserver`, `internal/scenario`, `internal/integrations`, `internal/engloop`.

# 4. Baseline

| Gate | Current baseline | Notes |
|---|---|---|
| module/build hygiene | PASS | CI download/verify/tidy/clean checks |
| formatting | PASS | gofmt gate |
| vet/static | PASS | `go vet ./...` |
| unit/integration | PASS | `go test ./... -count=1` |
| race | PASS | T-011 removed known F-005 flake; first post-push race run green |
| security qualification | PASS | supply-chain, SBOM, Trivy/govulncheck path as configured |
| mutation critical-diff | PASS | risk-scoped Work Packet gate |
| benchmark smoke | PASS | current CI targets |
| global coverage | NOT BASELINED | introduce only with actionable policy |
| unexplained flaky tests | none active | F-005 resolved by T-011 |
| full mutation score | NOT CLAIMED | only scoped evidence is authoritative |
| target hardware | OPEN | T-008 |

# 5. System Invariants

- **I-001** ML/VLM/LLM produce evidence, never human/physical authority.
- **I-002** Every external write crosses an invocation gateway with desired-state/idempotency/reconciliation semantics.
- **I-003** Ambiguous provider outcomes are never blindly replayed.
- **I-004** Human authority cannot be obtained by unsigned, unbound or machine/system input.
- **I-005** Physical resources require durable cross-execution ownership; process-local mutexes are insufficient for multi-process claims.
- **I-006** Published scenario semantics are version-pinned and Safety Compiler cannot be bypassed.
- **I-007** Secrets are referenced; plaintext values do not enter durable state/log/audit responses.
- **I-008** Non-terminal durable executions survive compatible upgrade or fail closed with explicit migration/restore.
- **I-009** Runtime DB/WAL/SHM/artifact/operator state is not source-controlled.
- **I-010** Every pushed `main` state is buildable and materially no worse than its parent.
- **I-011** Critical safety tests synchronize on observable contracts, not scheduler wall-clock luck.
- **I-012** Until TLS is actually served by the production runtime, the HTTP server may bind only to loopback addresses.
- **I-013** Human, service and system identities must be technically distinguishable before machine transports can acquire authorization context.
- **I-014** Transport idempotency/staleness controls do not replace workflow semantic idempotency; both layers must have explicit contracts.

# 6. Findings Registry

## F-001 — Living execution plan was fragmented

**Status:** Resolved  
**Category:** Architecture / Process  
**Severity:** High  
**Confidence:** Confirmed  
**Root cause:** intent, status and execution ordering lived in separate documents.  
**Resolution:** T-001 created this reconciliation layer.

## F-002 — Runtime SQLite database and WAL were tracked by Git

**Status:** Resolved  
**Category:** Security / Data hygiene  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** prior tree tracked `data/sentinel.db*`; default runtime path uses that directory.  
**Resolution:** T-002 removed artifacts and ignores `/data/*.db*`; first boot still creates its parent directory.

## F-003 — Recorded Stage 17 status was stale

**Status:** Resolved by reconciliation; residual findings split out  
**Category:** Planning / Correctness  
**Severity:** Medium  
**Confidence:** Confirmed  
**Evidence:** old status claimed callback binding, authorization audit and exactly-once resume were unproven although code/tests/work packets now prove them.  
**Resolution:** T-003 creates `docs/STAGE17_RECONCILIATION.md`, updates the implementation snapshot and decomposes remaining gaps into F-006..F-011 / T-012..T-017.

## F-004 — `main` has no required-check technical guard

**Status:** Planned  
**Category:** CI/CD / Reliability  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** GitHub reports `main protected=false`, no required status checks.  
**Impact:** direct writes land before post-push CI can reject them.  
**Task:** T-010.  
**Constraint:** preserve the explicitly required direct-to-main iteration model without force push.

## F-005 — Siren compensation race test was scheduler-timing flaky

**Status:** Resolved  
**Category:** Testing / Reliability / Concurrency  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** one race attempt failed on unchanged test; same SHA rerun and parent passed.  
**Root cause:** background `Serve` startup + fixed 1s polling deadlines used as correctness oracle.  
**Resolution:** T-011 uses synchronous `Drive` state transitions and separately tests canceled `Serve`; first post-push `ci/security/mutation` all passed without rerun.

## F-006 — Hardened remote-TLS rule is disconnected from production runtime

**Status:** Planned  
**Category:** Security / Configuration / HTTP  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** `HardenedServerConfig.Validate` rejects non-loopback without TLS; runtime `Config.Validate` accepts any non-empty listen address; production server calls plaintext `ListenAndServe`.  
**Root cause:** hardened and runtime config contracts remain structurally separate while migration is incomplete.  
**Impact:** operator can expose authenticated/control HTTP surface over plaintext remote bind contrary to stated security contract.  
**Blast radius:** all HTTP routes, session cookies, operator commands.  
**Affected invariants:** I-004, I-010, I-012.  
**Task:** T-012.  
**Recommended direction:** fail closed by enforcing loopback-only on current runtime; add full TLS as a later compatible capability rather than leaving an unsafe interim path.

## F-007 — Secure callback semantics are not wired to an external HTTP transport

**Status:** Planned  
**Category:** Security / Architecture / Integration  
**Severity:** High  
**Confidence:** Strong  
**Evidence:** `CallbackSecurity` exists in `App`; orchestration `CallbackIngress` and extensive exact-once tests exist, but production references to `CallbackIngress` are absent from HTTP/app wiring and route table exposes no callback endpoint.  
**Impact:** Stage 17 callback boundary is secure in isolation but not a usable authenticated external ingress.  
**Affected invariants:** I-003, I-004.  
**Task:** T-013.

## F-008 — Principal model lacks explicit human/service/system kinds

**Status:** Planned  
**Category:** Authorization / Architecture  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** `auth.Principal` is a session user; persisted roles are viewer/operator/admin; callback subjects are intentionally restricted to `usr_*`; no service/system principal type exists.  
**Impact:** current boundaries are safe for known transports but future machine transports could accidentally reuse a human-shaped authority context.  
**Affected invariants:** I-004, I-013.  
**Task:** T-014.

## F-009 — Authorization audit and rate limiting are path-specific

**Status:** Planned  
**Category:** Security / Audit / Abuse resistance  
**Severity:** Medium/High  
**Confidence:** Confirmed  
**Evidence:** callback allow/deny is durably audited and allowed mutation fails closed when audit fails; generic `requireCapability` only returns 403. Rate limiter keys by scope+clientIP, not principal.  
**Impact:** inconsistent forensic trace and abuse controls between transports.  
**Task:** T-015.

## F-010 — Authentication boundary is not pluggable

**Status:** Planned  
**Category:** Architecture / Authentication  
**Severity:** Medium  
**Confidence:** Confirmed  
**Evidence:** browser auth is concrete local password/session stores; no local/OIDC/mTLS authenticator interface exists outside intent docs.  
**Task:** T-017 after T-014.

## F-011 — HTTP command stale/idempotency contract is absent

**Status:** Planned  
**Category:** Correctness / API  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** no common `If-Match`/ETag/expected-version or `Idempotency-Key` command contract found. Internal ADGO idempotency is not stale-UI protection.  
**Affected invariants:** I-014.  
**Task:** T-016.

# 7. Risk Register

| Risk | Severity | Control / next action |
|---|---|---|
| remote plaintext control surface | High | T-012 next |
| callback boundary not externally wired | High | T-013 after T-012 |
| machine/human principal ambiguity for future transports | High | T-014 |
| stale HTTP command / duplicate command request | High | T-016 |
| durable schema/plan upgrade breaks waiting workflows | Critical | T-004 |
| crash around external-effect linearization | Critical | T-005 |
| multi-process physical write race | Critical | T-006 |
| direct main without required checks | High | T-010 |
| release rollback/provenance gap | High | T-009 |
| target hardware budgets unknown | Medium/High | T-008 |

# 8. Pareto Improvements

Highest leverage now:

1. T-012 close an immediately exposable security configuration gap.
2. T-004 pin durable schemas/plans before further workflow/API expansion.
3. T-013/T-014 close callback transport + identity authority boundaries.
4. T-005/T-006 prove crash/fencing guarantees around physical effects.
5. T-016 establish common command preconditions before Scenario API expands surface.
6. T-015/T-010 standardize audit/abuse and verified-main controls.

# 9. Dependency DAG

```text
T-001 [DONE]
 +-- T-002 [DONE]
 |    +-- T-011 [DONE]
 |          +-- T-003 Stage17 reconciliation [VERIFYING]
 |                +-- T-012 remote-bind fail-closed
 |                |     +-- T-013 callback HTTP transport
 |                +-- T-014 principal kinds
 |                |     +-- T-015 authz audit/principal limiting
 |                |     +-- T-017 pluggable authenticator
 |                +-- T-016 command concurrency/idempotency
 |                +-- T-004 schema/plan evolution
 |                       +-- T-005 crash/restore matrix
 |                       +-- T-006 topology/fencing
 +-- T-010 verified-main guard

T-014 + T-016 + relevant T-013 auth contracts --> T-007 Scenario API
T-004/T-005/T-006 + T-008 --> T-009 release qualification
```

# 10. Implementation Phases

- **Phase A — Repository truth/security hygiene:** T-001, T-002, T-011, T-003, T-012, T-010.
- **Phase B — Identity/transport and durable evolution:** T-013..T-017, T-004..T-006.
- **Phase C — Product surface:** T-007 and dependency-safe Scenario authoring layers.
- **Phase D — Operational qualification:** T-008, T-009, remaining adapters/observability/soak/release gates.

# 11. Atomic Tasks

## T-001 — Establish the living master plan

**Status:** DONE  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH  
**Acceptance:** required plan sections, evidence-aware findings/DAG/backlog.  
**Verification:** `ci/security/mutation` PASS on `905862f...`.

## T-002 — Remove tracked runtime SQLite state

**Status:** DONE  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Acceptance:** default DB/WAL/SHM absent from tree and ignored; first boot preserved.  
**Verification:** gates PASS on `745f194...`; F-005 separated as pre-existing flake.

## T-003 — Reconcile Stage 17 against executable evidence

**Status:** VERIFYING  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

### Problem
Recorded status collapsed a mixed Stage 17 into one stale PARTIAL sentence.

### Evidence
Current server/auth/callback/notifier code plus Stage 17 Work Packets and green repository gates.

### Goal
Produce an exact `VERIFIED/PARTIAL/OPEN` clause matrix and atomic residual tasks without falsely reopening proven callback semantics.

### Scope
Documentation/status/master-plan reconciliation only.

### Non-goals
No production behavior change in this iteration.

### Files
`docs/STAGE17_RECONCILIATION.md`, `docs/IMPLEMENTATION_STATUS.md`, `MASTER_PLAN.md`.

### Tests
Repository CI/security/mutation after push; reconciliation must not claim unsupported implementation.

### Acceptance criteria
Every original Stage 17 clause has evidence/status; stale callback statements are corrected; each material residual maps to F-006..F-011/T-012..T-017.

### Dependencies
T-011.

### Risk / rollback
Low documentation risk; revert atomic docs commit if evidence is contradicted.

## T-004 — Pin durable schemas and workflow plans across upgrade

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

### Goal
Version external event schemas, lifecycle state and ADGO plan/registry implementations so compatible in-flight workflows continue and incompatible evolution migrates/fails explicitly.

### Tests
Golden old/new fixtures, waiting-execution reopen, unknown schema rejection, migration backup/restore/rollback; mutation required for version guards.

### Dependencies
T-003. May proceed independently of later Stage 17 HTTP transport tasks.

## T-005 — Prove crash/restore external-effect linearization matrix

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Goal:** fault-inject enqueue/claim/provider-accept/local-commit/disk/restore boundaries; no blind replay of ambiguous effects.  
**Dependencies:** T-004.

## T-006 — Enforce supported process topology for physical writes

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Goal:** enforce single-writer startup or genuine distributed admission/fencing before multi-writer mode.  
**Dependencies:** T-004.

## T-007 — Expose authenticated Scenario API without bypasses

**Status:** BLOCKED  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH  
**Goal:** bounded authenticated/authorized catalog/compiler/simulator contracts with no Safety Compiler or version-precondition bypass.  
**Dependencies:** T-014, T-016 and relevant T-004/T-013 contracts.

## T-008 — Establish target-hardware performance budgets

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** MEDIUM  
**Goal:** measure control latency, allocations, SQLite/Pebble load, queue pressure and degraded thresholds on target hardware.

## T-009 — Qualify release, upgrade and rollback

**Status:** BLOCKED  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Goal:** reproducible artifact, SBOM/provenance/checksums/signing policy, backup/upgrade/rollback/restore drills.  
**Dependencies:** T-004/T-005/T-006/T-008 plus remaining P0 production prerequisites.

## T-010 — Add a technical verified-main guard compatible with direct-main workflow

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH  
**Goal:** make failing direct-main updates technically difficult while retaining mandated direct push and no force-push behavior.

## T-011 — Make siren compensation verification deterministic under `-race`

**Status:** DONE  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Resolution:** `Start -> Drive(wait/enabled) -> Stop -> Drive(canceled/compensated)`; separate canceled-context `Serve` test.  
**Verification:** first post-push `ci`, `security`, `mutation` PASS on `5ccff8bf...`, no rerun.

## T-012 — Reject remote plaintext runtime bind until TLS is served

**Status:** READY  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

### Problem
Runtime `Config.Validate` accepts remote binds while production HTTP uses plaintext `ListenAndServe`.

### Goal
Enforce I-012 immediately: current runtime is loopback-only until TLS serving is implemented.

### Scope
Runtime server-config validation + targeted tests + plan reconciliation.

### Non-goals
Do not add half-wired certificate config or TLS listener in the same slice.

### Files / symbols
`internal/config/model.go`, config tests, `MASTER_PLAN.md`.

### Implementation
Parse `Server.ListenAddress`; reject invalid addresses and any non-loopback host with `ErrInsecureRemoteBind`. Reuse package loopback semantics already used by HardenedConfig.

### Compatibility
Default `127.0.0.1:8080` unchanged. Existing plaintext remote configurations intentionally fail startup rather than silently remain insecure.

### Edge cases
IPv4/IPv6 loopback, `localhost`, wildcard/unspecified IP, malformed host:port.

### Tests
Table tests for loopback allowed and `0.0.0.0`, `::`, LAN address rejected; full CI/security/mutation.

### Mutation tests
Required because this is a security validator.

### Rollback
Only after a verified production TLS listener is available.

## T-013 — Wire callback ingress through a narrow HTTP bearer adapter

**Status:** BLOCKED  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Goal:** expose bounded callback endpoints that preserve exact binding/event identity, use callback bearer auth, strict body limits, rate limiting and audited `CallbackIngress` only.  
**Dependencies:** T-012; runtime orchestration service wiring must be explicit.

## T-014 — Introduce explicit principal kinds and human-authority boundary

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Goal:** distinguish user/service/system principals in types and make human-only capabilities impossible for machine principals by construction/policy.  
**Non-goal:** do not add OIDC/mTLS yet.  
**Tests/mutation:** authorization matrix, system negative tests, callback human-subject compatibility; mutation required.

## T-015 — Standardize authorization audit and principal-aware abuse limiting

**Status:** BLOCKED  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** MEDIUM/HIGH  
**Goal:** one safe authorization decision record and principal-aware limiter without leaking credentials.  
**Dependencies:** T-014; callback transport T-013 for callback-specific limiting.

## T-016 — Add common HTTP command precondition and idempotency contract

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Goal:** define expected-version/ETag stale rejection and request idempotency key semantics before broad command APIs.  
**Tests:** stale/new/retry/conflict matrix, concurrent duplicates, mutation of comparison/idempotency guards.

## T-017 — Extract pluggable authenticator boundary

**Status:** BLOCKED  
**Priority:** P2  
**Type:** IMPROVE  
**Leverage:** MEDIUM  
**Goal:** make local authentication an implementation of a principal-producing interface suitable for future OIDC/mTLS without granting them authority implicitly.  
**Dependencies:** T-014.

# 12. Testing Strategy

Risk mesh:

- G0 build/module/static;
- G1 unit/golden;
- G2 property/fuzz/model;
- G3 race/concurrency;
- G4 contract/integration;
- G5 fault/crash/recovery;
- G6 mutation/test-of-tests;
- G7 E2E/HIL/UX;
- G8 performance/soak/release.

Critical edge space:

`input x identity x authority x time x ordering x concurrency x persistence x external failure x resource ownership x capacity x topology x version x cancellation x recovery`.

Tests use explicit observable synchronization. Wall-clock sleeps are valid only where elapsed time itself is the contract.

# 13. Mutation Testing Strategy

- Risk-scoped critical-diff mutation remains mandatory for changed critical production semantics.
- `LIVED`, `NOT COVERED` and unexplained `TIMED OUT` mutants block safety-sensitive closure.
- Strengthen observable contracts rather than chasing line coverage.
- Security validators (T-012), authorization/principal policy (T-014) and command preconditions (T-016) require mutation evidence.
- Full-module mutation score is not claimed until a reproducible campaign exists.

# 14. Performance Baselines

Current CI benchmark smoke is green but is not an SLO/capacity claim. T-008 owns hardware budgets and regression thresholds.

# 15. Security Hardening

Current sequence:

1. runtime/source-state boundary — DONE T-002;
2. deterministic safety gates — DONE T-011;
3. Stage 17 truth reconciliation — T-003 VERIFYING;
4. remote plaintext fail-closed — T-012;
5. explicit callback transport + principal authority — T-013/T-014;
6. durable schema/crash/fencing — T-004/T-005/T-006;
7. command preconditions/audit/abuse guard — T-016/T-015;
8. verified-main + release qualification — T-010/T-009.

Never weaken security/race/mutation gates to obtain green CI.

# 16. Migration Strategy

`characterize old -> introduce versioned/new boundary -> dual compatibility where safe -> migrate callers/state -> restart/rollback verification -> delete legacy only after no durable dependency remains`.

For security gaps, safe temporary restriction is preferred over a half-complete compatibility layer (for example loopback-only before TLS is truly served).

# 17. Deferred Work

- broad UI polish and authoring UX until authority/API prerequisites are stable;
- multi-node support if v1 explicitly enforces single writer;
- full TLS certificate lifecycle as a separate task after T-012 establishes the fail-closed interim rule;
- pluggable OIDC/mTLS authentication implementation after T-017 boundary;
- non-critical refactors without measurable leverage.

# 18. Rejected Decisions

- Treat old roadmap checkboxes as implementation evidence — rejected.
- Accept green rerun as resolution of a flaky critical test — rejected; T-011 fixed synchronization.
- Increase test timeout instead of removing scheduler luck — rejected.
- Expose callback HTTP endpoint before remote plaintext is fail-closed — rejected.
- Treat internal ADGO idempotency as HTTP stale-command/idempotency contract — rejected.
- Model service/system identities as synthetic users just to reuse RBAC — rejected pending T-014.
- Add certificate fields without actually serving TLS — rejected as false security.
- Force push — rejected.

# 19. Completed Tasks

- T-001 — living master plan.
- T-002 — runtime DB source-control hygiene.
- T-011 — deterministic siren compensation verification.

T-003 is ready for post-push verification; after it passes, T-012 becomes the next active implementation slice.

# 20. Iteration Log

## Iteration 1

**Task:** T-001  
**Findings:** F-001 addressed; F-002/F-003/F-004 discovered.  
**Commit:** `905862f738df655478022570cb57fcf3b97b279e` — `docs(plan): establish living master plan`  
**Tests:** ci/security/mutation PASS  
**Push:** main  
**Result:** PASS

## Iteration 2

**Task:** T-002  
**Findings:** F-002 addressed; F-005 discovered.  
**Commit:** `745f194ffc290b9a84a380899787932c938fad42` — `chore(repo): stop tracking runtime sqlite state`  
**Tests:** security/mutation PASS; first race attempt exposed F-005; controlled same-SHA rerun PASS.  
**Push:** main  
**Result:** PASS, F-005 explicitly carried

## Iteration 3

**Task:** F-005 planning reconciliation  
**Commit:** `c30b39520b47903baf2c1136164b59a75782569f` — `docs(plan): record siren race-test flake`  
**Tests:** ci/security/mutation PASS  
**Push:** main  
**Result:** PASS

## Iteration 4

**Task:** T-011  
**Findings addressed:** F-005  
**Changes:** deterministic synchronous compensation test + deterministic `Serve` cancellation test.  
**Commit:** `5ccff8bf89508da86f82e77703f11586eb7bc8a0` — `test(siren): make compensation verification deterministic`  
**Tests:** first post-push ci (including race), security and mutation all PASS; no rerun.  
**Push:** main  
**Result:** PASS

## Iteration 5

**Task:** T-003  
**Findings addressed:** F-003; F-006..F-011 created from evidence.  
**Changes:** exact Stage 17 clause matrix; corrected status snapshot; dependency-aware residual tasks.  
**Tests:** documentation consistency; post-push ci/security/mutation pending.  
**Commit:** pending creation  
**Push:** pending main  
**Result:** VERIFYING

# 21. Definition of Final Done

Convergence requires:

- no unresolved Critical/High findings except explicit accepted deferral;
- all P0/P1 acceptance criteria verified;
- remote HTTP exposure is TLS-protected or technically impossible;
- principal authority boundaries are explicit for human/service/system identities;
- durable schema/plan upgrade and rollback are proven;
- physical external-effect crash/fencing invariants are exercised;
- command stale/idempotency contracts exist before broad mutable APIs;
- security/race/static/fault/mutation gates are green for affected semantics;
- target-hardware performance meets explicit budgets;
- no unexplained flaky tests;
- runtime state/secrets remain outside source control;
- implementation docs match observed code;
- final re-audit finds no new fundamental blocker;
- latest verified state is committed and pushed to `main` without force push.
