# Home Sentinel — Living MASTER PLAN

> Единственный execution source of truth для progressive audit/implementation loop. Детальные intent-документы (`docs/AXIOM_IMPLEMENTATION_PLAN.md`, `docs/SCENARIO_SYSTEM_PLAN.md`) остаются нормативными спецификациями; findings, приоритет, dependency DAG и статус исполнения определяются здесь и сверяются с фактическим `main`.

**Plan revision:** 2026-08-28 / verified through T-002 `745f194ffc290b9a84a380899787932c938fad42`

---

# 1. Mission

Довести Home Sentinel до production-grade локальной security/automation платформы маленькими доказуемыми изменениями, сохраняя fail-closed authority boundary, durable side-effect semantics, воспроизводимость, rollback и проверяемую эволюцию данных/API.

Рабочий цикл:

`AUDIT -> PLAN -> SELECT -> CHARACTERIZE -> IMPLEMENT -> TEST -> ADVERSARIAL REVIEW -> RECONCILE -> COMMIT -> PUSH main -> CI VERIFY -> COMPRESS -> NEXT`

Существенная новая информация всегда проходит `Finding -> Impact -> Plan reconciliation -> Decision` до изменения scope.

# 2. Current State

- Go control plane содержит domain/event model, Axiom lifecycle, ADGO durable workflows, auth/authz, persistence/migrations, gateways, recovery и scenario AST/compiler/safety/catalog/simulator.
- Media/data plane отделён от control plane; ML/VLM/LLM не являются authority source.
- Stage 17 продвинулся дальше записанного `docs/IMPLEMENTATION_STATUS.md`: в `main` присутствуют callback ingress/runtime и durable Telegram notifier с per-recipient receipts.
- T-001 verified на `905862f738df655478022570cb57fcf3b97b279e`: `ci`, `security`, `mutation` PASS.
- T-002 verified на `745f194ffc290b9a84a380899787932c938fad42`: runtime SQLite/WAL/SHM удалены из source tree, `/data/*.db*` игнорируется; `security` и `mutation` PASS; первый CI attempt упал на timing-flake `TestManualStopUsesCompensation`, controlled rerun того же commit полностью PASS.
- Default SQLite directory не обязан существовать: `internal/database.Open` создаёт parent через `os.MkdirAll`.
- Локальный clone/build в текущем execution environment недоступен из-за отсутствия outbound DNS к GitHub; verification опирается на observed repository + GitHub Actions evidence. Это limitation окружения, не product pass/fail.

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

| Gate | Baseline | Evidence / notes |
|---|---|---|
| module/build hygiene | PASS | CI download/verify/list/tidy + clean diff |
| formatting | PASS | gofmt |
| vet/static | PASS | `go vet ./...` |
| unit/integration | PASS | `go test ./... -count=1` |
| race | PASS but flaky evidence exists | T-002 attempt 1 timing-failed siren test; identical attempt 2 PASS => F-005 |
| security | PASS | govulncheck/Trivy/supply-chain workflow as configured |
| mutation | PASS | active Work Packet critical-diff gate |
| benchmark smoke | PASS | current CI smoke targets |
| global coverage | NOT BASELINED | add only with actionable threshold |
| flaky tests | **KNOWN: F-005** | must be removed before convergence |
| full mutation score | NOT CLAIMED | risk-scoped evidence is authoritative |
| target-hardware performance | OPEN | T-008 |

Historical mutation failure on the durable-notifier feature commit was corrected by later Work Packet normalization and is not an active defect.

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
- **I-011** Critical safety tests must synchronize on observable workflow contracts, not arbitrary scheduler wall-clock luck.

# 6. Findings Registry

## F-001 — Living execution plan was fragmented

**Status:** Resolved  
**Category:** Architecture / Process  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** execution intent/status/order were split across plan index, domain plans, status snapshots and Work Packets.  
**Root cause:** no single reconciliation layer.  
**Impact:** stale task selection and hidden blockers.  
**Affected tasks:** T-001.  
**Resolution:** this file now owns findings/tasks/order/iteration log while detailed plans remain specifications.

## F-002 — Runtime SQLite database and WAL were tracked by Git

**Status:** Resolved  
**Category:** Security / Data hygiene / Reproducibility  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** parent tree tracked `data/sentinel.db`, `data/sentinel.db-shm`, `data/sentinel.db-wal`; default path is `data/sentinel.db`; old `.gitignore` did not exclude them.  
**Files / symbols:** `.gitignore`, `data/sentinel.db*`, `internal/config.Default`, `internal/database.Open`.  
**Root cause:** missing repository/runtime-state boundary.  
**Impact:** accidental local state disclosure and nondeterministic diffs.  
**Affected invariants:** I-007, I-009, I-010.  
**Affected tasks:** T-002.  
**Resolution:** current tree removes the three files and ignores `/data/*.db*`; first boot remains valid because the opener creates the parent. No history rewrite/secret rotation without evidence of an actual committed secret.

## F-003 — Recorded Stage 17 status is stale relative to `main`

**Status:** Planned  
**Category:** Correctness / Planning  
**Severity:** Medium  
**Confidence:** Confirmed  
**Evidence:** `docs/IMPLEMENTATION_STATUS.md` predates recent callback/notifier/runtime slices.  
**Root cause:** status snapshot not reconciled after latest Stage 17 commits.  
**Impact:** wrong dependency decisions or duplicate implementation.  
**Affected tasks:** T-003.  
**Recommended direction:** clause-by-clause reconciliation against code/tests; close only executable evidence.

## F-004 — `main` has no branch protection / required status checks

**Status:** Planned  
**Category:** CI/CD / Reliability  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** GitHub reports `main protected=false` and no required checks.  
**Impact:** a direct write can land before post-push CI rejects it.  
**Affected invariants:** I-010.  
**Affected tasks:** T-010.  
**Constraint:** retain the user-mandated direct-to-main iteration workflow without force push or hidden bypass.

## F-005 — Siren compensation race test is scheduler-timing flaky

**Status:** Planned  
**Category:** Testing / Reliability / Concurrency  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** on T-002 commit `745f194...`, CI attempt 1 passed unit tests but `go test -race ./... -count=1` failed `internal/orchestration/action/siren.TestManualStopUsesCompensation` after ~1.01 s with `siren was not enabled by worker`; controlled attempt 2 of the exact same commit fully passed. The test blob is identical to previous green commit `905862f...` (`6448bdbe...`).  
**Files / symbols:** `internal/orchestration/action/siren/siren_test.go`, `Service.Serve`, `Service.Drive`; Axiom `adgo.Production.Serve` / `Engine.RunLocal`.  
**Current behavior:** the test launches background `Serve`, then polls physical fake state with a hard 1 s deadline.  
**Expected behavior:** compensation semantics are proved deterministically independent of runner scheduling.  
**Root cause:** test correctness is coupled to unsynchronized background coordinator/worker startup and polling progress plus arbitrary wall-clock deadlines; race instrumentation can consume that timing budget.  
**Impact:** critical safety CI can fail spuriously; reruns can normalize ignoring a real regression; current convergence criterion forbids unexplained flakes.  
**Blast radius:** full `-race` gate and confidence in siren manual-stop compensation.  
**Reproduction:** compare CI attempt 1 vs controlled attempt 2 on the same SHA.  
**Affected invariants:** I-010, I-011.  
**Affected tasks:** T-011; T-003 is temporarily preempted until T-011 closes.  
**Recommended direction:** use the explicit synchronous workflow contract `Start -> Drive -> observe enabled -> Stop -> Drive -> observe StatusCanceled + disabled`. Axiom `RunLocal` uses the production Engine protocol and removes scheduler-start timing from this semantic test. Preserve Serve lifecycle coverage separately only if no deterministic coverage exists.

# 7. Risk Register

| Risk | Severity | Control / next action |
|---|---|---|
| source-controlled runtime DB state | High | RESOLVED T-002 |
| flaky siren compensation race gate | High | **T-011 next** |
| stale Stage 17 closure assumptions | High | T-003 after T-011 |
| schema/plan upgrade breaks waiting workflows | Critical | T-004 |
| multi-process physical write race | Critical | T-006 |
| crash around external-effect linearization | Critical | T-005 |
| direct-main without required checks | High | T-010 |
| release rollback/provenance gaps | High | T-009 |
| target hardware overload/latency unknown | Medium/High | T-008 |

# 8. Pareto Improvements

1. Repository/runtime-state boundary — DONE T-002.
2. Remove known flaky critical safety test — T-011.
3. Reconcile Stage 17 evidence — T-003.
4. Pin schema/plan evolution before expanding durable/API surface — T-004.
5. Prove crash/recovery and process topology before physical scale — T-005/T-006.
6. Expand Scenario API/UI and release surface only after prerequisites.

# 9. Dependency DAG

```text
T-001 [DONE]
  +--> T-002 [DONE]
  |      +--> T-011 siren deterministic compensation test [READY]
  |              +--> T-003 Stage17 evidence reconciliation
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
**Acceptance:** all required plan sections exist; baseline/findings/tasks explicit.  
**Verification:** post-push `ci`, `security`, `mutation` PASS on `905862f...`.

## T-002 — Remove tracked runtime SQLite state

**Status:** DONE  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Problem:** default DB/WAL/SHM were source-controlled.  
**Scope:** `.gitignore`, removal of `data/sentinel.db*`, plan reconciliation; no schema/history rewrite.  
**Implementation:** ignore `/data/*.db*`; remove three runtime artifacts; no `.gitkeep` because DB opener creates parent.  
**Invariants:** I-007, I-009, I-010.  
**Acceptance:** current source tree has no default runtime DB artifacts and generated SQLite state is ignored.  
**Verification:** security PASS, mutation PASS, CI attempt 2 PASS after F-005 classification; attempt 1 failure is recorded as a pre-existing flake, not erased.

## T-011 — Make siren compensation verification deterministic under `-race`

**Status:** READY  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

### Problem
`TestManualStopUsesCompensation` relies on unsynchronized background `Serve` progress and fixed 1 s polling deadlines; identical commits can pass or fail under race instrumentation.

### Evidence
F-005; same SHA failed attempt 1 and passed controlled attempt 2; unchanged test blob also passed prior commit.

### Goal
Prove the actual observable contract `enabled -> cancel requested -> compensation -> canceled + disabled` deterministically.

### Scope
`internal/orchestration/action/siren/siren_test.go`; production code only if a missing observable contract makes a test-only solution impossible.

### Non-goals
No siren plan semantic change, no weakened compensation assertion, no “fix” by merely inflating timeouts.

### Implementation
Prefer `Start -> Drive -> assert enabled -> Stop -> Drive -> assert StatusCanceled -> assert disabled`, because `Drive` maps to Axiom `RunLocal` and keeps production Engine scheduling/commit semantics without background worker startup timing.

### Invariants
I-003, I-010, I-011 and siren fail-safe compensation.

### Compatibility constraints
No public API or persisted-state change.

### Edge cases
cancel after enable; compensation idempotence; terminal canceled state; fake controller read failure must not be silently ignored in the revised test.

### Tests
Package/full unit + `go test -race ./... -count=1`; preserve/introduce deterministic Serve cancellation lifecycle coverage only if otherwise absent.

### Mutation tests
No production mutation target expected for a test-only repair; repository mutation workflow remains mandatory.

### Benchmarks
Not applicable.

### Acceptance criteria
The test has no scheduler-start polling deadline; it still proves enable occurs before cancellation and compensation disables the siren; full CI/security/mutation are green without relying on rerun for acceptance.

### Verification commands
`go test ./internal/orchestration/action/siren -count=1`; `go test -race ./internal/orchestration/action/siren -count=20`; standard repository CI/security/mutation.

### Dependencies
T-002.

### Blocks
T-003 until the known flake is removed.

### Risk
Low: test-only semantic characterization, provided assertions are preserved.

### Rollback
Restore old test only if a stronger deterministic synchronization primitive replaces it.

### Estimated effort
Small, one reviewable commit.

## T-003 — Reconcile Stage 17 against executable evidence

**Status:** BLOCKED  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH  
**Goal:** exact open/verified matrix for HTTP safety, authentication, callback binding/replay, principals, RBAC, audit, notifier and exactly-once resume.  
**Scope:** Stage 17 code/tests/work packets/status docs; implement at most one missing clause per follow-up task.  
**Acceptance:** no clause closed without executable evidence.  
**Dependencies:** T-011.

## T-004 — Pin durable schemas and workflow plans across upgrade

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Goal:** version external schemas, persisted lifecycle state and ADGO plans so waiting executions survive compatible upgrade and incompatible evolution fails/migrates explicitly.  
**Tests:** golden compatibility fixtures, waiting-execution reopen, unknown schema rejection, migration rollback/restore.  
**Mutation:** required for validators/version selection/migration guards.  
**Dependencies:** T-003.

## T-005 — Prove crash/restore linearization matrix

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Goal:** fault-inject around enqueue/claim/provider accept/local commit, disk failure/corruption and restore; no blind resend after ambiguous effect.  
**Mutation:** required for recovery state transitions.  
**Dependencies:** T-004.

## T-006 — Enforce supported process topology for physical writes

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH  
**Goal:** enforce single-writer startup or real distributed admission/fencing before multi-writer mode.  
**Tests:** concurrent process/admission, stale owner/fencing, restart, split-brain negative tests.  
**Dependencies:** T-004.

## T-007 — Expose authenticated Scenario API without bypasses

**Status:** BLOCKED  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH  
**Goal:** authenticated/authorized/bounded catalog/compiler/simulator HTTP contracts.  
**Dependencies:** T-003 and T-004 where persistence/versioning applies.

## T-008 — Establish target-hardware performance budgets

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** MEDIUM  
**Goal:** measure control-path latency, allocations, DB/Pebble load, queue pressure and degraded-mode thresholds on target hardware.

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
**Scope:** ruleset/branch protection and/or canonical verified-push path; never force push or weaken CI.

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

Tests for critical semantics must prefer explicit state/contract synchronization over sleeps/deadlines used as a correctness oracle.

# 13. Mutation Testing Strategy

- Risk-scoped Gremlins critical-diff gate remains mandatory for changed critical production packages.
- Critical `LIVED`, `NOT COVERED` or `TIMED OUT` mutants block closure unless proven equivalent/non-actionable with reviewable rationale.
- Strengthen observable contracts instead of inflating coverage.
- Do not invent a full-module mutation target before a reproducible baseline exists.

# 14. Performance Baselines

CI benchmark smoke is green but is not a full capacity/SLO claim. Preserve existing evidence and implement T-008 before target-hardware readiness claims.

# 15. Security Hardening

Priority:
1. repository/runtime-state boundary — DONE T-002;
2. deterministic critical safety gates — T-011;
3. Stage 17 auth/ingress/RBAC reconciliation — T-003;
4. schema/version fail-closed behavior — T-004;
5. crash/fencing guarantees — T-005/T-006;
6. release provenance/rollback — T-009.

Never lower authz, race, mutation or security gates to obtain green CI.

# 16. Migration Strategy

`characterize old -> introduce versioned boundary -> dual compatibility -> migrate callers/state -> verify restart/rollback -> remove legacy when no durable dependency remains`.

Destructive migration requires explicit backup/restore evidence and is never opportunistic cleanup.

# 17. Deferred Work

- UI polish and broad Scenario authoring UX beyond dependency-safe headless/API foundation.
- Multi-node topology if v1 explicitly enforces single writer.
- Full-module mutation campaigns when release/scheduled budget exists.
- Non-critical refactors without measurable risk/complexity/performance leverage.

# 18. Rejected Decisions

- Rewrite all detailed plans into one duplicated mega-document — rejected: drift risk.
- Treat Markdown checkboxes as implementation evidence — rejected: code/tests/CI win.
- Fix every audit observation immediately — rejected: material scope enters Findings/Tasks first.
- Force push — rejected: breaks traceability and explicit user constraint.
- Rewrite Git history/rotate credentials only because runtime DB files existed — rejected without evidence of actual committed secret.
- Accept a green rerun as sufficient resolution of F-005 — rejected: convergence explicitly forbids unexplained flaky tests.
- Increase siren test timeout as the primary fix — rejected: masks scheduling nondeterminism instead of testing the contract.

# 19. Completed Tasks

- T-001 — living MASTER PLAN established and verified.
- T-002 — tracked default SQLite runtime state removed and verified; incidental race-test flake recorded separately as F-005.

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
**Unexpected findings:** F-005 during post-push verification  
**Changes:** `/data/*.db*` ignored; tracked `sentinel.db`, `-shm`, `-wal` removed; first-boot semantics preserved.  
**Tests:** security PASS; mutation PASS; CI attempt 1 unit/vet/etc. PASS but race step hit F-005; controlled rerun of exact SHA fully PASS including race/engloop/benchmark.  
**Plan changes:** F-005 separated from T-002 because identical siren code/test passed before and on rerun; T-011 promoted ahead of T-003.  
**Commit:** `745f194ffc290b9a84a380899787932c938fad42` — `chore(repo): stop tracking runtime sqlite state`  
**Push:** main  
**Result:** PASS with F-005 carried as explicit blocker, not ignored

## Iteration 3

**Task:** planning reconciliation for F-005  
**Findings addressed:** none yet; F-005 recorded and impact-analyzed  
**Unexpected findings:** none  
**Changes:** added I-011 and T-011; dependency DAG now blocks T-003 until deterministic siren safety verification is restored.  
**Tests:** documentation-only planning iteration; standard post-push `ci`, `security`, `mutation` required.  
**Plan changes:** T-011 becomes next P0.  
**Commit:** `docs(plan): record siren race-test flake`  
**Push:** main  
**Result:** PASS pending post-push confirmation; failure reopens this planning iteration.

# 21. Definition of Final Done

Convergence requires:
- no known Critical/High findings without explicit accepted deferral;
- all P0/P1 acceptance criteria verified;
- durable schema/plan upgrade and rollback proven;
- authority/resource invariants enforced by code/types/persistence/tests;
- security/race/static/fault/mutation gates green for affected semantics;
- target performance/soak evidence meets explicit budgets;
- **no unexplained flaky tests**;
- runtime state/secrets outside source control;
- docs/status match observed implementation;
- final re-audit yields no new fundamental blockers;
- latest verified state committed and pushed to `main` without force push.
