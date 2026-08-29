# Home Sentinel — Living MASTER PLAN

> Single execution source of truth for progressive audit, atomic implementation and verified direct-to-main delivery. Detailed architecture/product plans remain intent specifications; this file owns findings, invariants, ordering and execution state.

**Plan revision:** 2026-08-29 / T-024 verified on `6a2be3eddd54aff6a69ee1693b2034fbe92d910d`; F-017 and durable-upgrade umbrella T-004 are resolved; T-005 crash/restore external-effect linearization is the active P0 slice.

---

# 1. Mission

Deliver a production-grade local security/automation platform via small reviewable changes while preserving fail-closed authority, durable external-effect semantics, reproducibility, rollback and evidence-driven evolution.

`AUDIT -> PLAN -> SELECT -> CHARACTERIZE -> IMPLEMENT -> TEST -> SELF-REVIEW -> RECONCILE -> COMMIT -> PUSH main -> VERIFY -> COMPRESS -> NEXT`

Unexpected material discoveries are recorded before implementation scope expands.

# 2. Current State

- Go control plane: typed domain/events, Axiom lifecycle, ADGO durable workflows, auth/authz, SQLite state, gateways/recovery and Scenario compiler/safety/catalog/simulator.
- Verified tasks: T-001, T-002, T-003, T-004, T-011, T-012, T-018, T-019, T-020, T-021, T-022, T-023, T-024.
- Complete ADGO inventory is incident, siren action, door action and camera recovery; every family now has an explicit durable execution-bundle evolution boundary.
- T-021 retains incident v1/v2 plan + handler + callback/human binding semantics.
- T-022 reconstructs configuration-derived siren historical bundles and preserves timer/cancel/compensation behavior across restart plus configuration change.
- T-023 freezes door v1 graph/handlers/human+reconciliation bindings and routes persisted work by exact bundle identity.
- T-024 freezes camera v1 graph/handlers/operator binding, routes persisted work through Axiom Host, validates Open/Serve fail-closed, and proves Pebble restart plus retained-v1/future-active coexistence.
- T-024 final evidence on `6a2be3ed...`: module/vulnerability/format/vet/unit/race/reconciliation/benchmark PASS; security/SBOM/Trivy PASS; Gremlins 52 killed, 0 lived, 0 not-covered, 0 timeout, efficacy and mutator coverage 100%.
- Pinned ADGO explicitly provides at-least-once external work, persists task + idempotency key before provider invocation, and permits redelivery when a provider accepted an effect but the process died before completion commit. T-005 now owns Home Sentinel’s deterministic proof of that crash window for physical effects.
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
ADGO/Pebble : Execution PlanID/PlanVersion/PlanDigest + durable tasks/history
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
| door durable-upgrade mutation | PASS, 46/46 killed |
| camera durable-upgrade mutation | PASS, 52/52 killed |
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
- **I-019** Durable workflow identity pins an execution bundle: plan graph, version-specific handler semantics and external signal/human bindings.
- **I-020** Unknown persisted non-terminal plan/bundle identity fails startup/control-plane qualification closed.
- **I-021** Configuration-derived durable plans that own physical effects must be reconstructible and routable by persisted identity across restart.
- **I-022** A fixed-version durable workflow has an immutable-bundle release boundary before semantic v2: graph, handlers and bindings are retained together.
- **I-023** ADGO external effects are at-least-once: after `provider accepted -> completion commit lost`, recovery may redeliver only with the same durable idempotency identity, and Home Sentinel must prove the physical effect is not applied twice or else enter explicit reconciliation.
- **I-024** Crash/fault tests must control commit/lease boundaries deterministically; process sleeps, scheduler luck and timing inflation are not crash-linearization evidence.

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

Incident v1/v2 retain immutable plan + version-specific Registry/handlers + signal/human bindings and exact persisted-digest routing. Final `8b13e556...`: Gremlins 69/69 killed and full gates PASS.

## F-016 — Config-derived siren plan could strand an enabled physical effect across restart
**Status:** Resolved | **Severity:** Critical | **Confidence:** Confirmed | T-022.

Canonical duration reconstruction, exact identity validation, Host routing and historical Stop/compensation recovery are verified. Final `dfced327...`: Gremlins 68/68 killed and full gates PASS.

## F-017 — Door/camera fixed-version services lacked a first-v2 retained-bundle boundary
**Status:** Resolved | **Severity:** Critical | **Confidence:** Confirmed | T-023 + T-024.

Door v1 is frozen at `sha256:ca5201ec70e540f7323176f4a2ca156c9c6a373bd35a664d33c0178b51cd6880`; camera v1 is frozen at `sha256:a0781f96957948af4ea5535c04f1a00ffffa40653c55cd0ed8ea0cfbcd20706f`. Both route non-terminal operations by persisted bundle identity, fail Open/preflight closed on unknown/mismatched state, preserve terminal history, and prove retained-v1 coexistence with a distinct test future-active bundle without shipping a fake production v2. Door final evidence: 46/46 mutants killed. Camera final evidence on `6a2be3ed...`: 52/52 killed; full CI/race/security PASS.

# 7. Risk Register

| Risk | Severity | Control |
|---|---|---|
| provider accepted physical effect but ADGO completion commit is lost | Critical | T-005 ACTIVE |
| multi-process physical write race | Critical | T-006 |
| callback transport absent | High | T-013 |
| machine/human principal ambiguity | High | T-014 |
| stale/duplicate HTTP command | High | T-016 |
| direct main without required checks | High | T-010 |
| release provenance/rollback | High | T-009 |

# 8. Pareto Improvements

1. T-005/T-006 prove external-effect crash linearization and supported single-writer/fencing topology.
2. T-013/T-014 establish callback transport and machine-vs-human authority.
3. T-016 adds common command preconditions/idempotency.
4. T-015/T-010 tighten audit/abuse/main protection.
5. T-008/T-009 complete hardware and release qualification.

# 9. Dependency DAG

```text
T-001 DONE -> T-002 DONE -> T-011 DONE -> T-003 DONE
                                      |-> T-012 DONE -> T-018 DONE -> T-013 READY
                                      |-> T-019 DONE -> T-020 DONE -> T-021 DONE -> T-022 DONE -> T-023 DONE -> T-024 DONE -> T-004 DONE
                                      |                                                                                             |-> T-005 ACTIVE
                                      |                                                                                             |-> T-006
                                      |-> T-014 -> T-015 / T-017
                                      |-> T-016
T-001 -> T-010

T-014 + T-016 + relevant T-013/T-004 -> T-007
T-004 + T-005 + T-006 + T-008 -> T-009
```

# 10. Implementation Phases

- **A Truth/gates:** T-001/T-002/T-003/T-011/T-012/T-018/T-019/T-020 DONE; T-010 remains.
- **B Durable control:** T-004/T-021/T-022/T-023/T-024 DONE -> T-005 ACTIVE -> T-006; T-013..T-017 in parallel dependency order.
- **C Product surface:** T-007.
- **D Qualification:** T-008/T-009.

# 11. Atomic Tasks

## T-001/T-002/T-003/T-004/T-011/T-012/T-018/T-019/T-020/T-021/T-022/T-023/T-024
**Status:** DONE. Verified commits/evidence recorded below.

## T-004 — Pin durable workflow execution semantics across upgrade
**Status:** DONE | **Priority:** P0

All ADGO families are inventoried and have a verified evolution boundary: incident v1/v2 retained bundles, reconstructible siren configuration-derived bundles, immutable door v1 and immutable camera v1. Unknown/mismatched non-terminal identities fail closed, terminal historical records remain readable, and future-active coexistence is proven without fake production versions.

## T-005 — Prove crash/restore external-effect linearization
**Status:** ACTIVE | **Priority:** P0 | **Type:** PHYSICAL EFFECT / DURABILITY | **Risk:** CRITICAL | Depends T-004.

### Characterization
- pinned ADGO contract is explicitly at-least-once, not exactly-once;
- coordinator durably enqueues task + `IdempotencyKey` before worker invocation;
- activity completion atomically commits result/node/facts/history; if that commit is lost, redelivery is legal;
- ADGO explicitly names the critical window `provider accepted effect -> process died before ADGO commit` and requires provider idempotency or explicit reconciliation;
- door, siren and camera physical gateways already receive `gateway.Operation{ExecutionID, IdempotencyKey}`; their fakes deduplicate confirmed effects by idempotency key;
- incident Telegram notifier already has a separate Stage17 durable receipt state machine proving `sending` crash/reopen becomes ambiguous without blind resend; T-005 therefore focuses on door/siren/camera physical workflow integration.

### Goal
Build deterministic fault tests that lose the ADGO completion commit only after a physical provider has accepted/applied the effect, restart durable orchestration over the same state, force lease recovery without wall-clock sleeps, and prove redelivery preserves the original idempotency identity so physical application occurs at most once. Where the provider outcome cannot be proven, recovery must remain in explicit ambiguous/reconciliation state rather than blind replay.

### Preferred implementation
Use the public pinned ADGO `Store`/`PebbleStore`/`NewEngine` contracts and Home Sentinel’s compiled plans/registries. A test-only Store decorator may fail the selected completion `Commit` after observing provider application; the underlying Pebble state remains at the durable running task. Reopen Pebble/new engine, deterministically expire the persisted lease via a controlled state mutation, then recover/redeliver. Do not add production refactors merely to generate mutation targets.

### Acceptance
- deterministic fault injector distinguishes enqueue/claim/provider-call/completion-commit and fails exactly the post-provider completion commit;
- test proves the provider was applied once before the lost commit and the durable task still carries the same non-empty idempotency key;
- after Pebble close/reopen and deterministic lease expiry, a new engine/coordinator redelivers with the identical idempotency key;
- door recovery reaches the correct final lock state with physical `Applied==1`; no second physical application occurs;
- the crash-recovered siren enable is physically applied exactly once; a later safety-disable is a distinct expected physical effect that must also occur at most once, leaving the siren disabled without weakening cancellation/compensation safety;
- camera reconnect recovery reaches a verified stream state with one physical reconnect application;
- ambiguous outcome paths do not automatically replay an effect whose application is unknown; they remain explicit reconciliation/human-safe according to each workflow contract;
- no `time.Sleep`, inflated timeout or scheduler-probability assertion is used as crash evidence;
- full static/unit/fault/race/replay/security gates PASS;
- if production critical `.go` semantics change, Gremlins must produce non-empty clean evidence; if the slice is genuinely `_test.go`-only, mutation planner must select no production target rather than manufacturing a refactor solely for mutation numbers.

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

Critical edge space: `input x identity x authority x time x ordering x concurrency x persistence x external failure x ownership x topology x plan-version x handler-version x config-derived-version x signal/human-binding x task-state x lease-state x provider-acceptance x completion-commit x cancellation x compensation x recovery x gate-classification x evidence-emptiness`.

# 13. Mutation Testing Strategy

- Critical changed production semantics require real Gremlins execution and non-empty generated evidence.
- `MutantsTotal==0` is failure when selected critical production targets exist.
- `_test.go` files are intentionally excluded from mutation targets; a test-only critical fault-verification slice may legitimately have no mutation target.
- Never add a semantic-neutral production refactor solely to make a test-only slice produce mutants.
- Config, orchestration, engloop, principal policy, command preconditions, migration guards and physical gateways remain mutation-critical whenever their production code changes.
- Equivalent representation branches should be removed rather than suppressed; lived/not-covered/timeout mutants require contract analysis.

# 14. Performance Baselines

Benchmark smoke is regression smoke only. T-008 owns target-hardware latency/throughput/allocation budgets.

# 15. Security Hardening

Source-state hygiene DONE; deterministic siren test DONE; Stage17 truth DONE; remote plaintext guard DONE; config mutation boundary DONE; orchestration taxonomy DONE; zero-evidence guard DONE; incident bundles DONE; siren config-drift recovery DONE; door v1 bundle boundary DONE; camera v1 bundle boundary DONE. Next: physical crash linearization -> fencing/topology -> callback/principal/preconditions -> main/release qualification.

# 16. Migration Strategy

`characterize old semantics -> pin immutable historical execution bundle -> introduce active new bundle only for a real semantic change -> route existing state by durable identity -> explicit migration only at safe/quiescent point -> verify restart/rollback -> retire old bundle only after no durable references remain`.

For config-derived plans, parsed configuration, canonical version and recompiled digest must agree. For fixed-version plans, a semantic graph/handler change requires a version bump and retained old bundle in the same release; golden digest drift under the same version is a test failure.

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
- selected production target with zero generated mutants as evidence;
- production refactor added only to force mutation evidence for a test-only task;
- automatic plan substitution/migration on startup;
- pinning only the DAG while reusing drifted current handlers;
- silently ignoring unknown non-terminal plan digests;
- treating current runtime config as permission to reinterpret historical physical state;
- creating a fake v2 only to exercise upgrade plumbing;
- changing a fixed-version plan/handler bundle without version bump + retained old bundle;
- suppressing an equivalent mutation when the unobservable representation branch can be removed;
- time-based crash tests when commit/lease state can be controlled directly;
- claiming exactly-once from ADGO task state;
- reimplementing Axiom Host/migration logic;
- force push.

# 19. Completed Tasks

T-001, T-002, T-003, T-004, T-011, T-012, T-018, T-019, T-020, T-021, T-022, T-023, T-024.

# 20. Iteration Log

- **1 T-001** `905862f...`: ci/security/mutation PASS.
- **2 T-002** `745f194...`: DB hygiene; F-005 discovered; final PASS.
- **3 F-005 plan** `c30b395...`: PASS.
- **4 T-011** `5ccff8bf...`: deterministic siren test; first-attempt PASS.
- **5 T-003** `ef8ecaf...`: Stage17 reconciliation; PASS.
- **6 T-012** `d57f239...`: ci/security PASS; mutation setup exposed F-012.
- **7 T-018** `26b28f...`: config mutation boundary; Gremlins 1/1 killed; all gates PASS.
- **8 F-013 plan/closure** `aaafae7...`: planning gates PASS.
- **9 T-019** `6b55fe38...`: ci/race/security PASS; zero-mutant result exposed F-014.
- **10 F-014 planning** `a69bc45...`: finding/work packet recorded.
- **11 T-020** `36d4847...`: Gremlins 2/2 killed; closes F-013/F-014 and T-019/T-020.
- **12-17 T-021** planning/runtime/recovery through `8b13e556...`: incident v1/v2 execution bundles; final full gates PASS; Gremlins 69/69; closes F-015.
- **18-20 T-022** planning/runtime/recovery through `dfced327...`: siren config-derived historical bundles; final full gates PASS; Gremlins 68/68; closes F-016.
- **21 F-017/T-023 planning** `74341a52...`: complete ADGO inventory and preventive fixed-version boundary recorded.
- **22 T-023 runtime** `e996c4cd...`: door frozen v1 handlers/digest + Host routing + Pebble tests; mutation exposed one equivalent nil-vs-empty representation survivor.
- **23 T-023 recovery** `a130177e...`: full CI/race/security PASS; Gremlins 46/46 killed; T-023 done.
- **24 T-023 closure / T-024 planning** `fcb1bbe8...`: T-023 reconciled DONE, camera golden identity/work packet activated; planning CI/race/security PASS.
- **25 T-024 runtime** `7ee654fc...`: frozen camera v1 bundle + Host routing + Open/Serve/Pebble tests; security/format/vet/unit PASS; Gremlins 50 killed with one NOT COVERED Start guard and one TIMED OUT Serve-preflight mutant.
- **26 T-024 testability recovery** `dd14a4de...`: targeted Start/Serve mutation observability added; introduced variadic/non-variadic `ServeResilient` assignment compile defect was caught by govulncheck package loading and classified as introduced, not retried blindly.
- **27 T-024 final recovery** `6a2be3ed...`: exact Serve adapter fix; module/vulnerability/format/vet/unit/race/reconciliation/benchmark PASS; security/SBOM/Trivy PASS; Gremlins 52/52 killed, no blockers, 100% efficacy/coverage. Closes T-024, F-017 and T-004.
- **28 T-005 characterization:** pinned ADGO contract confirms durable task/idempotency before invoke, at-least-once redelivery after lost completion commit, fencing on stale attempts and explicit ambiguous-side-effect reconciliation; activate deterministic physical crash-window proof.

# 21. Definition of Final Done

No unresolved Critical/High findings without accepted deferral; all P0/P1 acceptance verified; remote exposure safe; critical production test-of-tests has selected targets and non-empty clean mutation evidence; explicit principal kinds; durable plan+handler+binding/config evolution and restart/rollback; deterministic external-effect crash/fencing tests; stale/idempotency HTTP contracts; security/race/static/fault/mutation green; hardware budgets met; no unexplained flakes; docs match code; final re-audit finds no fundamental blocker; last verified state is main with no force push.
