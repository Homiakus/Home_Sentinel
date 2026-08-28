# Home Sentinel — Living MASTER PLAN

> Единственный execution source of truth для progressive audit/implementation loop.
> Архитектурные intent-документы (`docs/AXIOM_IMPLEMENTATION_PLAN.md`, `docs/SCENARIO_SYSTEM_PLAN.md`) остаются нормативными спецификациями, но выбор, статус и порядок работ определяются здесь и сверяются с фактическим `main`.

**Plan revision:** 2026-08-28 / baseline `eb7ec252b96e92f981d58ea156bf4d4c7bf7c750`

---

# 1. Mission

Довести Home Sentinel до production-grade локальной security/automation платформы через маленькие доказуемые изменения, сохраняя fail-closed authority boundary, durable side-effect semantics, воспроизводимость, rollback и проверяемую эволюцию данных/API.

Рабочий цикл:

`AUDIT -> PLAN -> SELECT -> CHARACTERIZE -> IMPLEMENT -> TEST -> ADVERSARIAL REVIEW -> RECONCILE -> COMMIT -> PUSH main -> CI VERIFY -> COMPRESS -> NEXT`

Существенная новая информация всегда проходит `Finding -> Impact -> Plan reconciliation -> Decision` до изменения scope.

# 2. Current State

- Go control plane уже содержит domain/event model, Axiom lifecycle, ADGO durable workflows, auth/authz, persistence/migrations, gateways, recovery, scenario AST/compiler/safety/catalog/simulator.
- Media/data plane отделён от control plane; ML/VLM/LLM не являются authority source.
- Stage 17 заметно продвинулся относительно recorded status: в `main` присутствуют callback ingress/runtime и durable Telegram notifier с per-recipient receipts.
- Последний baseline commit перед созданием этого файла: `eb7ec252b96e92f981d58ea156bf4d4c7bf7c750`.
- Для указанного baseline GitHub Actions `ci`, `security` и `mutation` завершились успешно.
- Локальный clone/build в текущем execution environment недоступен из-за отсутствия outbound DNS к GitHub; поэтому начальный baseline опирается на observed repository + GitHub Actions evidence. Это environment limitation, не product pass/fail.

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
- SQLite: application state, migrations, auth/Telegram/receipts and related durable records
- Pebble/ADGO production storage: durable workflow execution/history
- content-addressed artifact references: media remains outside workflow state

Scenario product:
Scenario AST -> type/temporal checks -> Safety Compiler -> Axiom/ADGO lowering -> Catalog/Simulator
```

Primary boundaries: `internal/domain`, `internal/orchestration`, `internal/gateway`, `internal/auth*`, `internal/database`, `internal/scenario`, `internal/integrations`, `internal/httpserver`, `internal/engloop`.

# 4. Baseline

Observed at `eb7ec252...`:

| Gate | Baseline | Evidence/notes |
|---|---|---|
| build/module hygiene | PASS via CI | `go mod download/verify/list/tidy` + clean diff |
| formatting | PASS via CI | gofmt check |
| vet/static | PASS via CI | `go vet ./...` |
| unit/integration package tests | PASS via CI | `go test ./... -count=1` |
| race | PASS via CI | `go test -race ./... -count=1` |
| security | PASS via workflow | govulncheck/Trivy/supply-chain policy as configured |
| mutation | PASS latest workflow | active Work Packet critical-diff gate |
| benchmark smoke | PASS via CI | correlation/risk smoke targets |
| coverage | NOT BASELINED globally | create explicit coverage trend only when it gives actionable signal |
| flaky tests | no current evidence | must be recorded on first observed nondeterministic failure |
| full mutation score | NOT CLAIMED | only risk-scoped evidence is authoritative |
| target-hardware performance | OPEN | Stage 26/production qualification |

Historical note: the durable-notifier feature commit had a failing mutation workflow; the later Work Packet normalization commit restored the current mutation gate to green. Do not classify that historical failure as an active defect.

# 5. System Invariants

I-001. ML/VLM/LLM may produce evidence/facts, never human/physical authority.

I-002. Every external write passes through an invocation gateway with desired-state/idempotency/reconciliation semantics.

I-003. Ambiguous provider outcome is never blindly replayed.

I-004. Human-only authority cannot be obtained by `system` principal or unsigned/unbound callback input.

I-005. Physical resources require durable cross-execution admission; process-local mutex alone is insufficient.

I-006. Published scenario semantics are immutable/version-pinned; Safety Compiler cannot be bypassed.

I-007. Secrets are referenced, not persisted/logged as plaintext values.

I-008. Existing non-terminal durable executions remain continuable across compatible upgrade or fail closed with an explicit migration/restore path.

I-009. Runtime state, credentials, WAL/SHM, artifacts and local operator data are never committed to source control.

I-010. `main` must remain buildable and materially no worse after every iteration.

# 6. Findings Registry

## F-001 — Living execution plan was fragmented

**Status:** Resolved

**Category:** Architecture / Process

**Severity:** High

**Confidence:** Confirmed

**Evidence:** repository used `PLAN_INDEX`, Axiom/scenario plans, implementation snapshots and Work Packets but had no top-level `MASTER_PLAN.md` matching the requested single execution source of truth.

**Files / symbols:** `docs/PLAN_INDEX.md`, `docs/engineering/ENGINEERING_LOOP.md`, `docs/IMPLEMENTATION_STATUS.md`.

**Root cause:** intent, observed status and execution ordering lived in separate documents.

**Impact / blast radius:** stale status can select already-implemented work or miss newly discovered blockers.

**Affected tasks:** T-001.

**Recommended direction:** keep detailed domain plans as specifications; this file owns findings, atomic tasks, ordering and iteration log.

## F-002 — Runtime SQLite database and WAL are tracked by Git

**Status:** Planned

**Category:** Security / Data hygiene / Reproducibility

**Severity:** High

**Confidence:** Strong

**Evidence:** tracked `data/sentinel.db` (4096 B), `data/sentinel.db-shm` (32768 B), `data/sentinel.db-wal` (716912 B); default config writes to `data/sentinel.db`; `.gitignore` does not exclude runtime DB/WAL/SHM.

**Files / symbols:** `data/sentinel.db*`, `.gitignore`, `internal/config.Default`.

**Current behavior:** ordinary local execution can mutate files that are source-controlled.

**Expected behavior:** runtime state is generated locally and ignored; repository contains only schema/migration/test fixtures intentionally reviewed as source.

**Root cause:** missing repository-state boundary for default runtime path.

**Impact:** accidental disclosure of users/bindings/tokens metadata/audit state, non-deterministic diffs, stale WAL coupling, noisy or unsafe releases.

**Blast radius:** every developer/operator running the default configuration in a checkout.

**Reproduction:** inspect tracked `data/` and default DB path.

**Affected invariants:** I-007, I-009, I-010.

**Affected tasks:** T-002.

**Recommended direction:** delete tracked runtime DB/WAL/SHM, ignore runtime DB family and preserve only an empty directory marker if required; verify CI and repository scanners.

## F-003 — Recorded Stage 17 status is stale relative to `main`

**Status:** Planned

**Category:** Correctness / Planning

**Severity:** Medium

**Confidence:** Confirmed

**Evidence:** `docs/IMPLEMENTATION_STATUS.md` still describes Stage 17 as partial while recent commits add durable notifier/callback acceptance/runtime evidence and latest CI/security/mutation are green.

**Root cause:** status snapshot was not reconciled after latest Stage 17 slices.

**Impact:** wrong dependency decisions, especially blocking Stage 35 or duplicating already-closed work.

**Affected tasks:** T-003.

**Recommended direction:** clause-by-clause Stage 17 reconciliation against current code/tests; close only executable evidence, leave remaining clauses explicit.

## F-004 — `main` has no branch protection / required status checks

**Status:** Planned

**Category:** CI/CD / Reliability

**Severity:** High

**Confidence:** Confirmed

**Evidence:** GitHub reports `main` as `protected=false` with no required checks.

**Current behavior:** direct writes can land before GitHub Actions can reject them.

**Expected behavior:** verified-push discipline must have a technical guard or an equivalent pre-push verification path.

**Impact:** a credentialed automation or human can place broken state on `main` despite green post-push workflows.

**Affected invariants:** I-010.

**Affected tasks:** T-010.

**Compatibility constraint:** user explicitly requires successful iterations to push directly to `main`; protection design must preserve that workflow without force-push or hidden bypass.

# 7. Risk Register

| Risk | Severity | Control / next action |
|---|---|---|
| source-controlled runtime DB state | High | T-002 immediately |
| stale Stage 17 closure assumptions | High | T-003 before dependent API work |
| schema/plan upgrade breaks waiting workflows | Critical | T-004 |
| multi-process physical write race | Critical | T-006 |
| crash around external-effect linearization | Critical | T-005 |
| direct-main without required checks | High | T-010 |
| release rollback/provenance gaps | High | T-009 |
| target hardware overload/latency unknown | Medium/High | T-008 |

# 8. Pareto Improvements

Highest-leverage near-term sequence:

1. remove source-controlled runtime state;
2. reconcile Stage 17 to eliminate false blockers and identify exact auth/ingress delta;
3. close schema/plan versioning before more durable workflows/API surface;
4. prove crash/recovery + process topology before production physical scale;
5. only then expand Scenario API/UI and release surface.

# 9. Dependency DAG

```text
T-001 living plan
  |
  +--> T-002 repo runtime-state hygiene
  +--> T-003 Stage17 evidence reconciliation
          |
          +--> T-004 schema/plan catalog + migration safety
          |      |
          |      +--> T-005 crash/restore matrix
          |      +--> T-006 process topology/fencing gate
          |
          +--> T-007 authenticated Scenario API

T-004/T-005/T-006 --> T-009 release/upgrade/rollback qualification
T-008 target-hardware budgets --> T-009
T-010 verified-main guard can proceed independently after T-002
```

# 10. Implementation Phases

**Phase A — Repository truth & safety hygiene:** T-001..T-003, T-010.

**Phase B — Durable evolution & failure safety:** T-004..T-006.

**Phase C — Product surface:** T-007 plus dependency-safe scenario authoring work from the scenario plan.

**Phase D — Operational qualification:** T-008..T-009, observability/adapters/soak/release gates.

# 11. Atomic Tasks

## T-001 — Establish the living master plan

**Status:** DONE

**Priority:** P0

**Type:** IMPROVE

**Leverage:** HIGH

### Problem
Execution truth was fragmented across intent plans, recorded snapshots and Work Packets.

### Goal
Create this single reconciliation layer without deleting the detailed architecture/product plans.

### Scope
Audit topology/current plans/current CI; record baseline, findings, risks, DAG and atomic backlog.

### Non-goals
No production-code change; no reinterpretation of already-proven safety contracts.

### Tests
Documentation-only change; verify file is present, coherent with observed `main`, and repository workflows remain green after push.

### Acceptance criteria
All required MASTER PLAN sections exist; current baseline/findings/tasks are explicit.

### Dependencies
None.

### Risk
Low.

### Rollback
Revert this documentation commit.

## T-002 — Remove tracked runtime SQLite state

**Status:** READY

**Priority:** P0

**Type:** HARDEN

**Leverage:** HIGH

### Problem
Default runtime DB/WAL/SHM are source-controlled.

### Goal
Make runtime persistence impossible to commit accidentally under the default path.

### Scope
`.gitignore`, `data/sentinel.db`, `data/sentinel.db-shm`, `data/sentinel.db-wal`; optionally `data/.gitkeep` if directory presence is operationally required.

### Non-goals
No DB schema migration; no history rewrite; no secret rotation unless evidence proves a committed secret.

### Implementation
Add narrow SQLite runtime ignore rules, remove tracked runtime artifacts, preserve intentional fixtures elsewhere.

### Invariants
I-007, I-009, I-010.

### Edge cases
`-wal`, `-shm`, journal variants; test fixture DBs outside runtime directory; clean first boot when `data/` is absent.

### Tests
Repository tree contains no runtime DB; ignore rules cover generated siblings; CI/security/mutation remain green.

### Mutation tests
Not applicable to deletion-only repository hygiene.

### Acceptance criteria
Default app execution cannot dirty Git with SQLite runtime files.

### Verification commands
`git ls-files 'data/*.db*'`; `git check-ignore data/sentinel.db data/sentinel.db-wal data/sentinel.db-shm`; standard CI/security/mutation.

### Dependencies
T-001.

### Risk
Low/medium: must not delete intentional seed/fixture data.

### Rollback
Restore files only if proven source fixtures and move runtime DB elsewhere.

## T-003 — Reconcile Stage 17 against executable evidence

**Status:** READY

**Priority:** P0

**Type:** IMPROVE

**Leverage:** HIGH

### Problem
Recorded Stage 17 status predates recent implementation slices.

### Goal
Produce exact open/verified matrix for HTTP safety, authentication, callback binding/replay, principals, RBAC, audit, notifier and exactly-once resume.

### Scope
Stage 17 code/tests/work packets/status docs only; implement at most one missing clause per follow-up atomic task.

### Tests
Use existing CI, race, security, callback/notifier mutation/fault evidence; add characterization only for genuinely unproven clauses.

### Acceptance criteria
No Stage 17 clause is marked complete without executable evidence; dependent tasks know exact prerequisites.

### Dependencies
T-001.

## T-004 — Pin durable schemas and workflow plans across upgrade

**Status:** TODO

**Priority:** P0

**Type:** HARDEN

**Leverage:** HIGH

### Goal
Version external event schemas, persisted lifecycle data and ADGO plans so waiting executions survive compatible upgrade and incompatible evolution fails/migrates explicitly.

### Tests
Golden compatibility fixtures, waiting-execution upgrade/reopen, unknown/new schema rejection, migration rollback/restore.

### Mutation tests
Required for validators/version selection/migration guards.

### Dependencies
T-003.

## T-005 — Prove crash/restore linearization matrix

**Status:** TODO

**Priority:** P0

**Type:** HARDEN

**Leverage:** HIGH

### Goal
Exercise crash points around durable enqueue/claim/provider accept/local commit, disk failure/corruption and restore for critical workflows/gateways.

### Tests
Fault injection + replay + restart + restore; no blind resend after ambiguous effect.

### Mutation tests
Required for recovery state transitions.

### Dependencies
T-004.

## T-006 — Enforce supported process topology for physical writes

**Status:** TODO

**Priority:** P0

**Type:** HARDEN

**Leverage:** HIGH

### Goal
For v1 either enforce a single-writer startup lock or implement a real distributed admission/fencing protocol before multi-writer mode can start.

### Tests
Concurrent process/admission simulation, stale-owner/fencing, restart, split-brain negative tests.

### Dependencies
T-004.

## T-007 — Expose authenticated Scenario API without bypasses

**Status:** BLOCKED

**Priority:** P1

**Type:** IMPROVE

**Leverage:** HIGH

### Goal
Expose catalog/compiler/simulator through authenticated, authorized, bounded HTTP contracts.

### Dependencies
T-003 and remaining Stage 17 blockers; schema compatibility from T-004 where API persistence depends on it.

## T-008 — Establish target-hardware performance budgets

**Status:** TODO

**Priority:** P1

**Type:** IMPROVE

**Leverage:** MEDIUM

### Goal
Measure control-path latency, allocations, DB/Pebble load, queue pressure and degraded-mode thresholds on target hardware.

### Tests / benchmarks
Reproducible benchmark profiles + soak; fail on agreed significant regressions, not arbitrary micro-deltas.

## T-009 — Qualify release, upgrade and rollback

**Status:** BLOCKED

**Priority:** P0

**Type:** HARDEN

**Leverage:** HIGH

### Goal
Reproducible artifact, SBOM/provenance/checksums/signing policy, pre-upgrade backup, upgrade/rollback/restore drills and release checklist evidence.

### Dependencies
T-004, T-005, T-006, T-008 and remaining production adapters/observability prerequisites.

## T-010 — Add a technical verified-main guard compatible with direct-main workflow

**Status:** TODO

**Priority:** P1

**Type:** HARDEN

**Leverage:** HIGH

### Problem
`main` currently accepts direct writes with no required status checks.

### Goal
Make broken direct-main updates technically difficult while retaining the mandated commit/push-to-main iteration model.

### Scope
Evaluate repository ruleset/branch protection and/or a canonical local verified-push command; never use force push or weaken CI.

### Tests
Deliberate failing preflight must block the supported push path; green preflight remains usable.

# 12. Testing Strategy

Use risk-based test mesh from `docs/engineering/ENGINEERING_LOOP.md`:

- G0 static/build/module hygiene;
- G1 deterministic unit/golden;
- G2 property/fuzz/model;
- G3 race/concurrency;
- G4 contract/integration;
- G5 fault/crash/recovery;
- G6 mutation/test-of-tests;
- G7 E2E/HIL/UX;
- G8 performance/soak/release.

Critical edge space is modeled as:

`input x identity x time x ordering x concurrency x persistence x external failure x authorization x resource ownership x capacity x topology x version x cancellation x recovery`.

# 13. Mutation Testing Strategy

- Risk-scoped Gremlins diff gate remains mandatory for changed critical production packages.
- Any critical `LIVED`, `NOT COVERED` or `TIMED OUT` mutant blocks task closure unless documented as equivalent/non-actionable with reviewable rationale.
- Prefer contract-strengthening tests over coverage inflation.
- Full-module mutation score is not invented as a target until a reproducible baseline exists.

# 14. Performance Baselines

Current CI has benchmark smoke for risk/correlation but not a complete product budget. Preserve existing no-regression evidence and implement T-008 before claiming hardware capacity/SLO readiness.

# 15. Security Hardening

Priority controls:

1. repository/runtime-state boundary (T-002);
2. complete/reconcile Stage 17 auth/ingress/RBAC evidence (T-003);
3. schema/version fail-closed behavior (T-004);
4. physical-action crash/fencing guarantees (T-005/T-006);
5. release provenance/rollback (T-009).

Never lower authz, mutation, race or security gates to make an iteration green.

# 16. Migration Strategy

For any public/persistent transition:

`characterize old -> introduce versioned boundary -> dual compatibility -> migrate callers/state -> verify restart/rollback -> remove legacy only when no durable dependency remains`.

Destructive migration requires explicit backup/restore evidence and is never bundled as opportunistic cleanup.

# 17. Deferred Work

- UI polish and broad Scenario authoring UX beyond dependency-safe headless/API foundation.
- Multi-node topology if v1 explicitly enforces single writer.
- Full-module mutation campaigns when release/scheduled budget is available.
- Non-critical refactors without measurable risk/complexity/performance leverage.

# 18. Rejected Decisions

- **Rejected:** rewrite existing Axiom/ADGO/scenario plans into one huge duplicated document. Reason: creates drift; this file references them and owns execution state instead.
- **Rejected:** treat recorded Markdown checkboxes as evidence. Reason: code/tests/CI win over stale status.
- **Rejected:** fix every audit observation immediately. Reason: all material scope enters Findings + Atomic Tasks first.
- **Rejected:** force push to repair direct-main conflicts. Reason: forbidden and destroys traceability.

# 19. Completed Tasks

- T-001 — established living MASTER PLAN from observed `main` and current CI evidence.

# 20. Iteration Log

## Iteration 1

**Task:** T-001

**Findings addressed:** F-001

**Unexpected findings:** F-002, F-003, F-004

**Changes:** created `MASTER_PLAN.md`; recorded observed architecture/baseline, invariants, risks, DAG and first atomic backlog.

**Tests:** parent `main` baseline had successful `ci`, `security`, `mutation`; documentation diff self-reviewed; post-push workflows must remain green.

**Plan changes:** T-002 promoted to first implementation task because tracked runtime SQLite state violates repository/data boundary.

**Commit:** `docs(plan): establish living master plan`

**Push:** main

**Result:** PASS pending confirmation of post-push workflows; any failure reopens this iteration immediately.

# 21. Definition of Final Done

Convergence requires all of the following:

- no known Critical/High findings without explicit accepted deferral;
- all P0/P1 acceptance criteria verified;
- durable schema/plan upgrade and rollback proven;
- critical authority/resource invariants enforced by code/types/persistence/tests;
- security/race/static/fault/mutation gates green for affected semantics;
- target performance and soak evidence meet explicit budgets;
- no unexplained flaky tests;
- runtime state/secrets are outside source control;
- docs/status match observed implementation;
- final re-audit yields no new fundamental blockers;
- latest verified state is committed and pushed to `main` without force push.
