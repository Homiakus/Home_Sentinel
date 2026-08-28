# Home Sentinel — Living MASTER PLAN

> Единственный execution source of truth для progressive audit/implementation loop. Детальные intent-документы (`docs/AXIOM_IMPLEMENTATION_PLAN.md`, `docs/SCENARIO_SYSTEM_PLAN.md`) остаются нормативными спецификациями; findings, приоритет, dependency DAG и статус исполнения определяются здесь и сверяются с фактическим `main`.

**Plan revision:** 2026-08-28 / planning iteration 3 verified on `c30b39520b47903baf2c1136164b59a75782569f`; T-011 implementation prepared for verification.

---

# 1. Mission

Довести Home Sentinel до production-grade локальной security/automation платформы маленькими доказуемыми изменениями, сохраняя fail-closed authority boundary, durable side-effect semantics, воспроизводимость, rollback и проверяемую эволюцию данных/API.

Цикл:

`AUDIT -> PLAN -> SELECT -> CHARACTERIZE -> IMPLEMENT -> TEST -> ADVERSARIAL REVIEW -> RECONCILE -> COMMIT -> PUSH main -> VERIFY -> COMPRESS -> NEXT`

Любая существенная новая информация проходит `Finding -> Impact analysis -> Plan reconciliation -> Decision` до расширения scope.

# 2. Current State

- Go control plane содержит domain/event model, Axiom lifecycle, ADGO durable workflows, auth/authz, persistence/migrations, gateways, recovery и scenario AST/compiler/safety/catalog/simulator.
- Media/data plane отделён от control plane; ML/VLM/LLM не являются authority source.
- T-001 создал living plan; `ci/security/mutation` PASS на `905862f...`.
- T-002 удалил tracked runtime SQLite/WAL/SHM и добавил `/data/*.db*` в `.gitignore`; controlled rerun того же SHA подтвердил, что race failure был отдельным flake, а не regression T-002.
- Planning iteration 3 (`c30b395...`) записала F-005/T-011 и полностью прошла `ci`, `security`, `mutation`.
- T-011 меняет только siren tests: semantic compensation check переводится с background `Serve` + polling deadlines на deterministic `Drive`; отдельный deterministic test сохраняет `Serve` cancellation coverage.
- Default SQLite directory не обязан существовать: `internal/database.Open` создаёт parent через `os.MkdirAll`.
- Локальный full module build недоступен из текущего execution environment из-за отсутствия outbound DNS к GitHub; локально доступен standalone `gofmt`, а authoritative verification выполняют GitHub Actions.

# 3. Architecture Map

```text
Cameras / Sensors / HA / Intercom
             |
             v
Media + CV + normalization          [data plane]
             |
       typed domain events
             |
   +---------+----------+
   |                    |
   v                    v
Axiom lifecycle      ADGO workflows [control plane]
   |                    |
   +------ policy/RBAC--+
             |
      Invocation Gateway
             |
 external effects / physical IO

Persistence:
- SQLite: application/auth/Telegram/receipt state + migrations
- Pebble/ADGO: durable workflow execution/history
- content-addressed references: media stays outside workflow state

Scenario product:
Scenario AST -> type/temporal checks -> Safety Compiler -> Axiom/ADGO lowering -> Catalog/Simulator
```

Primary boundaries: `internal/domain`, `internal/orchestration`, `internal/gateway`, `internal/auth*`, `internal/database`, `internal/scenario`, `internal/integrations`, `internal/httpserver`, `internal/engloop`.

# 4. Baseline

| Gate | Baseline | Notes |
|---|---|---|
| module/build hygiene | PASS | CI download/verify/list/tidy + clean diff |
| formatting | PASS | gofmt |
| vet/static | PASS | `go vet ./...` |
| unit/integration | PASS | `go test ./... -count=1` |
| race | PASS with known historical flake F-005 | identical T-002 commit failed once, rerun passed |
| security | PASS | govulncheck/Trivy/supply-chain workflow as configured |
| mutation | PASS | active Work Packet critical-diff gate |
| benchmark smoke | PASS | current CI targets |
| global coverage | NOT BASELINED | add only with actionable threshold |
| flaky tests | F-005 active until T-011 verified | convergence blocker |
| full mutation score | NOT CLAIMED | risk-scoped evidence is authoritative |
| target-hardware performance | OPEN | T-008 |

Historical notifier mutation failure was corrected by Work Packet normalization and is not active.

# 5. System Invariants

- **I-001** ML/VLM/LLM may produce evidence/facts, never human/physical authority.
- **I-002** Every external write passes through an invocation gateway with desired-state/idempotency/reconciliation semantics.
- **I-003** Ambiguous provider outcome is never blindly replayed.
- **I-004** Human-only authority cannot be obtained by `system` principal or unsigned/unbound callback input.
- **I-005** Physical resources require durable cross-execution admission; process-local mutex alone is insufficient.
- **I-006** Published scenario semantics are immutable/version-pinned; Safety Compiler cannot be bypassed.
- **I-007** Secrets are referenced, not persisted/logged as plaintext values.
- **I-008** Existing non-terminal durable executions remain continuable across compatible upgrade or fail closed with explicit migration/restore.
- **I-009** Runtime state, credentials, WAL/SHM, artifacts and local operator data are never committed to source control.
- **I-010** `main` remains buildable and materially no worse after every iteration.
- **I-011** Critical safety tests synchronize on observable workflow contracts, not arbitrary scheduler wall-clock luck.

# 6. Findings Registry

## F-001 — Living execution plan was fragmented

**Status:** Resolved  
**Category:** Architecture / Process  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** execution truth was split across plan index, domain plans, status snapshots and Work Packets.  
**Root cause:** no single reconciliation layer.  
**Impact:** stale task selection and hidden blockers.  
**Affected tasks:** T-001.  
**Resolution:** this file owns findings/tasks/order/log; detailed plans remain specifications.

## F-002 — Runtime SQLite database and WAL were tracked by Git

**Status:** Resolved  
**Category:** Security / Data hygiene / Reproducibility  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** parent tree tracked `data/sentinel.db`, `data/sentinel.db-shm`, `data/sentinel.db-wal`; default path is `data/sentinel.db`.  
**Root cause:** missing repository/runtime-state boundary.  
**Impact:** accidental local-state disclosure and nondeterministic diffs.  
**Affected invariants:** I-007, I-009, I-010.  
**Affected tasks:** T-002.  
**Resolution:** runtime files removed; `/data/*.db*` ignored; no history rewrite/secret rotation without evidence of an actual secret.

## F-003 — Recorded Stage 17 status is stale relative to `main`

**Status:** Planned  
**Category:** Correctness / Planning  
**Severity:** Medium  
**Confidence:** Confirmed  
**Evidence:** `docs/IMPLEMENTATION_STATUS.md` predates recent callback/notifier/runtime slices.  
**Root cause:** status snapshot not reconciled after recent Stage 17 commits.  
**Impact:** wrong dependency decisions or duplicate implementation.  
**Affected tasks:** T-003.  
**Recommended direction:** clause-by-clause reconciliation against executable evidence.

## F-004 — `main` has no branch protection / required status checks

**Status:** Planned  
**Category:** CI/CD / Reliability  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** GitHub reports `main protected=false`, required checks empty.  
**Impact:** a direct write can land before post-push CI rejects it.  
**Affected invariants:** I-010.  
**Affected tasks:** T-010.  
**Constraint:** retain mandated direct-to-main workflow without force push or hidden bypass.

## F-005 — Siren compensation race test is scheduler-timing flaky

**Status:** Planned / VERIFYING via T-011  
**Category:** Testing / Reliability / Concurrency  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** T-002 commit `745f194...` passed unit tests but one `-race` attempt failed `TestManualStopUsesCompensation` after ~1 s with `siren was not enabled by worker`; controlled rerun of the exact same SHA fully passed; test blob was unchanged from prior green commit.  
**Files / symbols:** `internal/orchestration/action/siren/siren_test.go`, `Service.Serve`, `Service.Drive`; Axiom `RunLocal`.  
**Root cause:** correctness coupled to background coordinator/worker startup plus fixed polling deadlines.  
**Impact:** critical safety CI can fail spuriously and reruns can normalize bad habits.  
**Blast radius:** full race gate and confidence in manual-stop compensation.  
**Affected invariants:** I-010, I-011.  
**Affected tasks:** T-011; T-003 remains blocked until verification.  
**Resolution direction implemented by T-011:** `Start -> Drive -> assert waiting/enabled -> Stop -> Drive -> assert canceled/disabled`; no scheduler-start timeout. Preserve deterministic `Serve` cancellation contract separately.

# 7. Risk Register

| Risk | Severity | Control / next action |
|---|---|---|
| source-controlled runtime DB state | High | RESOLVED T-002 |
| flaky siren compensation race gate | High | T-011 VERIFYING |
| stale Stage 17 closure assumptions | High | T-003 after T-011 |
| schema/plan upgrade breaks waiting workflows | Critical | T-004 |
| multi-process physical write race | Critical | T-006 |
| crash around external-effect linearization | Critical | T-005 |
| direct-main without required checks | High | T-010 |
| release rollback/provenance gaps | High | T-009 |
| target hardware overload/latency unknown | Medium/High | T-008 |

# 8. Pareto Improvements

1. Repository/runtime-state boundary — DONE T-002.
2. Remove known flaky critical safety test — T-011 VERIFYING.
3. Reconcile Stage 17 evidence — T-003.
4. Pin schema/plan evolution before expanding durable/API surface — T-004.
5. Prove crash/recovery and process topology before physical scale — T-005/T-006.
6. Expand Scenario API/UI and release surface only after prerequisites.

# 9. Dependency DAG

```text
T-001 [DONE]
  +--> T-002 [DONE]
  |      +--> T-011 siren deterministic compensation test [VERIFYING]
  |              +--> T-003 Stage17 evidence reconciliation [BLOCKED]
  |                      +--> T-004 schema/plan migration safety
  |                      |      +--> T-005 crash/restore matrix
  |                      |      +--> T-006 process topology/fencing
  |                      +--> T-007 authenticated Scenario API
  +--> T-010 verified-main guard

T-004/T-005/T-006 --> T-009 release/upgrade/rollback qualification
T-008 target-hardware budgets --> T-009
```

# 10. Implementation Phases

- **Phase A — Repository truth & safety hygiene:** T-001, T-002, T-011, T-003, T-010.
- **Phase B — Durable evolution & failure safety:** T-004..T-006.
- **Phase C — Product surface:** T-007 plus dependency-safe scenario authoring work.
- **Phase D — Operational qualification:** T-008, T-009, remaining adapters/observability/soak/release gates.

# 11. Atomic Tasks

## T-001 — Establish the living master plan

**Status:** DONE  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH  
**Problem:** execution truth was fragmented.  
**Goal/Scope:** establish this plan as reconciliation layer without deleting detailed architecture plans.  
**Tests:** post-push `ci/security/mutation`.  
**Acceptance:** all required plan sections and evidence-aware backlog exist.  
**Dependencies:** none.  
**Rollback:** revert planning commit.  
**Result:** PASS on `905862f...`.

## T-002 — Remove tracked runtime SQLite state

**Status:** DONE  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Problem:** default DB/WAL/SHM were source-controlled.  
**Goal:** generated runtime persistence must not appear as source changes.  
**Scope:** `.gitignore`, removal of `data/sentinel.db*`, plan reconciliation.  
**Non-goals:** no schema/history rewrite or speculative secret rotation.  
**Invariants:** I-007, I-009, I-010.  
**Edge cases:** WAL/SHM siblings; clean first boot.  
**Tests:** tree/ignore check + standard CI/security/mutation.  
**Acceptance:** no tracked default runtime DB family; first boot preserved.  
**Dependencies:** T-001.  
**Risk:** low.  
**Rollback:** restore only if proven fixture and move runtime path first.  
**Result:** PASS on `745f194...`; F-005 separated as pre-existing flake.

## T-011 — Make siren compensation verification deterministic under `-race`

**Status:** VERIFYING  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

### Problem
`TestManualStopUsesCompensation` used background `Serve`, ignored controller read errors, polled state, and used fixed 1 s deadlines as correctness oracles.

### Evidence
F-005: same commit failed race attempt 1 and passed controlled attempt 2; unchanged test also passed previous commit.

### Goal
Prove `enabled -> cancel requested -> compensation -> canceled + disabled` deterministically.

### Scope
`internal/orchestration/action/siren/siren_test.go` and this plan only.

### Non-goals
No production siren semantics/API/persistence change; no timeout inflation; no weakening of compensation assertions.

### Files / symbols
`TestManualStopUsesCompensation`, new `TestServeReturnsCanceledContext`, `Service.Drive`, `Service.Serve`.

### Implementation
1. `Start` execution.
2. Synchronously `Drive` to durable safety-timer wait.
3. Assert `StatusWaiting` and physical fake is enabled, propagating read errors.
4. Request `Stop`.
5. Synchronously `Drive` cancellation/compensation.
6. Assert `StatusCanceled`, fake disabled and `Applied == 2` (enable + compensation transition).
7. Keep `Serve` lifecycle coverage with an already-canceled context, removing background startup timing.

### Invariants
I-003, I-010, I-011; siren fail-safe compensation remains observable.

### Compatibility constraints
No public or persisted-state change.

### Edge cases
Cancel after enable; compensation idempotence/state transition; terminal canceled state; controller read errors are not ignored; canceled service context returns cancellation.

### Tests
Local standalone `gofmt` PASS. Authoritative: package/full unit, `go test -race ./... -count=1`, repository CI/security/mutation. Targeted repeated race command remains recommended when local module environment is available.

### Mutation tests
No production mutation target changed; repository mutation workflow remains mandatory.

### Benchmarks
Not applicable.

### Acceptance criteria
No background scheduler-start polling/deadline remains in manual compensation test; enable-before-cancel and compensated disable are still asserted; `Serve` cancellation remains covered; first post-push `ci/security/mutation` all green without rerun.

### Verification commands
`gofmt -w internal/orchestration/action/siren/siren_test.go`; `go test ./internal/orchestration/action/siren -count=1`; `go test -race ./internal/orchestration/action/siren -count=20`; standard repository workflows.

### Dependencies
T-002.

### Blocks
T-003 until post-push verification succeeds.

### Risk
Low, test-only semantic characterization.

### Rollback
Restore old test only if replaced by a stronger explicit synchronization primitive.

### Estimated effort
Small, one reviewable commit.

## T-003 — Reconcile Stage 17 against executable evidence

**Status:** BLOCKED  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH  
**Problem:** recorded Stage 17 status is older than code.  
**Goal:** exact verified/open matrix for HTTP safety, authentication, callback binding/replay, principals, RBAC, audit, notifier and exactly-once resume.  
**Scope:** Stage 17 code/tests/work packets/status docs; implement at most one missing clause per follow-up task.  
**Non-goals:** no broad product expansion during reconciliation.  
**Tests:** existing executable evidence plus characterization for genuinely unproven clauses.  
**Acceptance:** no clause closed without evidence.  
**Dependencies:** T-011.

## T-004 — Pin durable schemas and workflow plans across upgrade

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Problem:** upgrade compatibility of durable waits/schemas must be explicit.  
**Goal:** version external schemas, persisted lifecycle state and ADGO plans; compatible upgrade continues, incompatible evolution fails/migrates explicitly.  
**Tests:** golden compatibility fixtures, waiting-execution reopen, unknown schema rejection, migration rollback/restore.  
**Mutation:** required for validators/version selection/migration guards.  
**Dependencies:** T-003.  
**Risk:** high; protect rollback.

## T-005 — Prove crash/restore linearization matrix

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Goal:** fault-inject around enqueue/claim/provider accept/local commit, disk failure/corruption and restore; no blind resend after ambiguous effect.  
**Tests:** crash/replay/restart/restore matrix.  
**Mutation:** recovery state transitions required.  
**Dependencies:** T-004.

## T-006 — Enforce supported process topology for physical writes

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Goal:** enforce single-writer startup or real distributed admission/fencing before multi-writer mode.  
**Tests:** concurrent process/admission, stale owner/fencing, restart and split-brain negative tests.  
**Dependencies:** T-004.

## T-007 — Expose authenticated Scenario API without bypasses

**Status:** BLOCKED  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH  
**Goal:** authenticated/authorized/bounded catalog/compiler/simulator HTTP contracts.  
**Tests:** authn/authz negatives, size/time budgets, unsafe scenario rejection.  
**Dependencies:** T-003 and T-004 where persistence/versioning applies.

## T-008 — Establish target-hardware performance budgets

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** MEDIUM  
**Goal:** measure control-path latency, allocations, SQLite/Pebble load, queue pressure and degraded-mode thresholds on target hardware.  
**Tests/benchmarks:** reproducible benchmark/soak profile with regression thresholds.

## T-009 — Qualify release, upgrade and rollback

**Status:** BLOCKED  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Goal:** reproducible artifact, SBOM/provenance/checksums/signing policy, backup, upgrade/rollback/restore drills and release evidence.  
**Dependencies:** T-004, T-005, T-006, T-008 and remaining production prerequisites.

## T-010 — Add a technical verified-main guard compatible with direct-main workflow

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH  
**Problem:** `main` accepts direct writes with no required checks.  
**Goal:** make broken direct-main updates technically difficult while retaining mandated commit/push-to-main iterations.  
**Scope:** ruleset/branch protection and/or canonical verified-push path.  
**Non-goals:** no force push, no CI weakening.  
**Acceptance:** direct workflow remains usable while failures cannot silently become accepted release state.

# 12. Testing Strategy

Risk mesh from `docs/engineering/ENGINEERING_LOOP.md`:

- G0 static/build/module hygiene;
- G1 deterministic unit/golden;
- G2 property/fuzz/model;
- G3 race/concurrency;
- G4 contract/integration;
- G5 fault/crash/recovery;
- G6 mutation/test-of-tests;
- G7 E2E/HIL/UX;
- G8 performance/soak/release.

Critical edge space:

`input x identity x time x ordering x concurrency x persistence x external failure x authorization x resource ownership x capacity x topology x version x cancellation x recovery`.

Safety tests prefer explicit state/contract synchronization over sleeps/deadlines used as correctness oracles. Sleeps may remain only where elapsed time itself is the contract (for example timer behavior), with bounded safety margin and deterministic state assertions.

# 13. Mutation Testing Strategy

- Risk-scoped Gremlins critical-diff gate remains mandatory for changed critical production packages.
- Critical `LIVED`, `NOT COVERED` or `TIMED OUT` mutants block closure unless proven equivalent/non-actionable with reviewable rationale.
- Strengthen observable contracts instead of inflating line coverage.
- Test-only changes still run the repository mutation workflow as a regression gate.
- Do not claim a full-module mutation score until a reproducible baseline exists.

# 14. Performance Baselines

CI benchmark smoke is green but is not a capacity/SLO claim. Preserve current evidence; T-008 defines target-hardware latency/allocation/load budgets before readiness claims.

# 15. Security Hardening

Priority:
1. repository/runtime-state boundary — DONE T-002;
2. deterministic critical safety gates — T-011 VERIFYING;
3. Stage 17 auth/ingress/RBAC reconciliation — T-003;
4. schema/version fail-closed behavior — T-004;
5. crash/fencing guarantees — T-005/T-006;
6. verified-main guard — T-010;
7. release provenance/rollback — T-009.

Never lower authz, race, mutation or security gates to obtain green CI.

# 16. Migration Strategy

`characterize old -> introduce versioned boundary -> dual compatibility -> migrate callers/state -> verify restart/rollback -> remove legacy when no durable dependency remains`.

Destructive migration requires explicit backup/restore evidence and is never opportunistic cleanup.

# 17. Deferred Work

- UI polish and broad Scenario authoring UX beyond dependency-safe API/headless foundation.
- Multi-node topology if v1 explicitly enforces single writer.
- Full-module mutation campaigns when release/scheduled budget exists.
- Non-critical refactors without measurable risk/complexity/performance leverage.

# 18. Rejected Decisions

- Duplicate every detailed plan into one mega-document — rejected: drift risk.
- Treat Markdown checkboxes as implementation evidence — rejected: code/tests/CI win.
- Fix every observation immediately — rejected: material scope enters Findings/Tasks first.
- Force push — rejected.
- Rewrite Git history/rotate credentials merely because runtime DB files existed — rejected without secret evidence.
- Accept a green rerun as resolution of F-005 — rejected: convergence forbids unexplained flakes.
- Increase siren test timeout as primary fix — rejected: masks nondeterminism.
- Remove `Serve` lifecycle coverage while fixing the flake — rejected: deterministic canceled-context coverage is cheap and preserves the wrapper contract.

# 19. Completed Tasks

- T-001 — living MASTER PLAN established and verified.
- T-002 — tracked default SQLite runtime state removed and verified.

T-011 is implemented but remains `VERIFYING` until first post-push `ci/security/mutation` succeed without rerun.

# 20. Iteration Log

## Iteration 1

**Task:** T-001  
**Findings addressed:** F-001  
**Unexpected findings:** F-002, F-003, F-004  
**Changes:** created living `MASTER_PLAN.md`.  
**Tests:** post-push `ci`, `security`, `mutation` PASS.  
**Commit:** `905862f738df655478022570cb57fcf3b97b279e` — `docs(plan): establish living master plan`  
**Push:** main  
**Result:** PASS

## Iteration 2

**Task:** T-002  
**Findings addressed:** F-002  
**Unexpected findings:** F-005  
**Changes:** `/data/*.db*` ignored; tracked SQLite DB/SHM/WAL removed.  
**Tests:** security PASS; mutation PASS; CI attempt 1 race hit F-005; controlled rerun of exact SHA fully PASS.  
**Plan changes:** F-005 separated from T-002; T-011 promoted ahead of T-003.  
**Commit:** `745f194ffc290b9a84a380899787932c938fad42` — `chore(repo): stop tracking runtime sqlite state`  
**Push:** main  
**Result:** PASS with F-005 carried explicitly

## Iteration 3

**Task:** planning reconciliation for F-005  
**Findings addressed:** F-005 recorded/impact-analyzed  
**Unexpected findings:** none  
**Changes:** added I-011 and T-011; T-003 blocked behind deterministic siren verification.  
**Tests:** post-push `ci`, `security`, `mutation` PASS.  
**Commit:** `c30b39520b47903baf2c1136164b59a75782569f` — `docs(plan): record siren race-test flake`  
**Push:** main  
**Result:** PASS

## Iteration 4

**Task:** T-011  
**Findings addressed:** F-005  
**Unexpected findings:** none before commit  
**Changes:** manual-stop test now uses synchronous `Drive` contract; controller read errors are asserted; compensation proves canceled/disabled plus two applied transitions; deterministic `Serve` cancellation test preserves lifecycle coverage.  
**Tests:** standalone `gofmt` PASS; authoritative post-push `ci/security/mutation` pending.  
**Plan changes:** T-011 -> VERIFYING; T-003 remains blocked until gates pass.  
**Commit:** pending creation  
**Push:** pending main  
**Result:** VERIFYING

# 21. Definition of Final Done

Convergence requires:
- no known Critical/High findings without explicit accepted deferral;
- all P0/P1 acceptance criteria verified;
- durable schema/plan upgrade and rollback proven;
- authority/resource invariants enforced by code/types/persistence/tests;
- security/race/static/fault/mutation gates green for affected semantics;
- target performance/soak evidence meets explicit budgets;
- no unexplained flaky tests;
- runtime state/secrets outside source control;
- docs/status match observed implementation;
- final re-audit yields no new fundamental blockers;
- latest verified state committed and pushed to `main` without force push.
