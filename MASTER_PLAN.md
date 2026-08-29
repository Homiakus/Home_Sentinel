# Home Sentinel — Living MASTER PLAN

> Single execution source of truth for progressive audit, atomic implementation and verified direct-to-main delivery. Detailed architecture/product plans remain intent specifications; this file owns findings, invariants, ordering and execution state.

**Plan revision:** 2026-08-29 / T-023 verified on `a130177ed3b2b57aded26b32ec003e6da0da2c46`; F-017 remains active only for camera recovery; T-024 is the active P0 slice.

---

# 1. Mission

Deliver a production-grade local security/automation platform via small reviewable changes while preserving fail-closed authority, durable external-effect semantics, reproducibility, rollback and evidence-driven evolution.

`AUDIT -> PLAN -> SELECT -> CHARACTERIZE -> IMPLEMENT -> TEST -> SELF-REVIEW -> RECONCILE -> COMMIT -> PUSH main -> VERIFY -> COMPRESS -> NEXT`

Unexpected material discoveries are recorded before implementation scope expands.

# 2. Current State

- Go control plane: typed domain/events, Axiom lifecycle, ADGO durable workflows, auth/authz, SQLite state, gateways/recovery and Scenario compiler/safety/catalog/simulator.
- Verified tasks: T-001, T-002, T-003, T-011, T-012, T-018, T-019, T-020, T-021, T-022, T-023.
- T-021 retains complete incident v1/v2 execution bundles and routes durable state by persisted identity through Axiom Host.
- T-022 reconstructs canonical historical siren duration bundles and preserves timer/cancel/compensation semantics across restart plus config change.
- T-023 freezes door v1 graph and version-specific handlers, routes Drive/approval/reconciliation by persisted bundle identity and proves retained-v1 coexistence with a distinct future active identity without shipping a fake v2.
- T-023 final evidence on `a130177...`: module/vulnerability/format/vet/unit/race/reconciliation/benchmark PASS; security/SBOM/Trivy PASS; Gremlins 46 killed, 0 lived, 0 not-covered, 0 timeout, efficacy and mutator coverage 100%.
- Complete ADGO inventory contains only incident, siren, door action and camera recovery. Camera v1 is the only remaining family without an immutable retained-bundle release boundary.
- Camera history shows initial creation plus formatting only; there is no current historical semantic drift. Its current canonical v1 digest on the pinned Axiom compiler is `sha256:a0781f96957948af4ea5535c04f1a00ffffa40653c55cd0ed8ea0cfbcd20706f`.
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
| door durable-upgrade mutation | PASS, 46/46 killed |
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
- **I-019** Durable workflow identity pins an execution bundle: plan graph, version-specific handler semantics and external signal/human bindings; pinning only PlanDigest plus current handlers is insufficient.
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

T-022 adds canonical duration reconstruction, exact identity validation, active + historical Host engines and persisted-digest Drive/Stop routing. Final `dfced327...`: all CI/race/security gates PASS; Gremlins 68/68 killed.

## F-017 — Door/camera fixed-version services lacked a first-v2 retained-bundle boundary
**Status:** ACTIVE only for camera | **Severity:** Critical | **Confidence:** Confirmed

**Category:** Durability / Physical-control upgrade readiness.  
**Evidence:** complete ADGO inventory contains incident, siren, door and camera recovery. Door and camera were both fixed `PlanVersion="1"` single-plan services. Their histories show no semantic predecessor beyond current v1, so no existing durable state is known to have been reinterpreted. Door is now protected by T-023; camera remains single-plan.  
**Root cause:** camera still lacks immutable bundle catalog, golden v1 identity, version-pinned handler registry, persisted-digest operation routing and fail-closed non-terminal startup/Serve qualification.  
**Impact:** the first future camera semantic v2 could strand a v1 operator wait or reinterpret external reconnect/recovery work.  
**Blast radius:** camera operator decisions, reconnect effects, restart recovery and Serve coordination.  
**Invariants:** I-008, I-017, I-019, I-020, I-022.  
**Task:** T-024. F-017 and T-004 close after T-024 is verified.

# 7. Risk Register

| Risk | Severity | Control |
|---|---|---|
| future camera v2 strands/reinterprets v1 recovery work | Critical | T-024 ACTIVE |
| external-effect crash linearization | Critical | T-005 |
| multi-process physical write race | Critical | T-006 |
| callback transport absent | High | T-013 |
| machine/human principal ambiguity | High | T-014 |
| stale/duplicate HTTP command | High | T-016 |
| direct main without required checks | High | T-010 |
| release provenance/rollback | High | T-009 |

# 8. Pareto Improvements

1. T-024 finish the last fixed-version ADGO retained-bundle boundary and close T-004/F-017.
2. T-005/T-006 prove crash linearization and supported single-writer/fencing topology.
3. T-013/T-014 callback transport/principal authority.
4. T-016 common command preconditions.
5. T-015/T-010 audit/abuse/main protection.
6. T-008/T-009 hardware/release qualification.

# 9. Dependency DAG

```text
T-001 DONE -> T-002 DONE -> T-011 DONE -> T-003 DONE
                                      |-> T-012 DONE -> T-018 DONE -> T-013 READY
                                      |-> T-019 DONE -> T-020 DONE -> T-021 DONE -> T-022 DONE -> T-023 DONE -> T-024 -> T-004 close
                                      |                                                                               |-> T-005
                                      |                                                                               |-> T-006
                                      |-> T-014 -> T-015 / T-017
                                      |-> T-016
T-001 -> T-010

T-014 + T-016 + relevant T-013/T-004 -> T-007
T-004/T-005/T-006 + T-008 -> T-009
```

# 10. Implementation Phases

- **A Truth/gates:** T-001/T-002/T-003/T-011/T-012/T-018/T-019/T-020/T-010.
- **B Durable evolution + identity/transport:** T-021/T-022/T-023 DONE -> T-024 -> T-004 close -> T-005/T-006 and T-013..T-017.
- **C Product surface:** T-007.
- **D Qualification:** T-008/T-009.

# 11. Atomic Tasks

## T-001/T-002/T-003/T-011/T-012/T-018/T-019/T-020/T-021/T-022/T-023
**Status:** DONE. Verified commits/evidence recorded below.

## T-004 — Pin durable workflow execution semantics across upgrade
**Status:** ACTIVE via T-024 only | **Priority:** P0

Axiom supplies durable plan identity, ExecutionCatalog, Pebble catalog support, multi-plan Host, digest routing and conservative explicit migration APIs. Incident, siren and door now have verified complete execution-bundle handling. Camera is the final ADGO family.

## T-023 — Establish immutable door v1 execution-bundle boundary
**Status:** DONE | **Priority:** P0 | **Risk:** CRITICAL

Door v1 is frozen at `sha256:ca5201ec70e540f7323176f4a2ca156c9c6a373bd35a664d33c0178b51cd6880`; handlers are explicitly v1-pinned; Open validates non-terminal persisted identities; Drive/unlock/reconciliation route through persisted bundle; Pebble restart tests cover human wait and ambiguous-side-effect reconciliation; test-only future-active coexistence proves retained v1 routing without production fake v2. Final `a130177...`: full CI/race/security PASS; Gremlins 46/46 killed, 100%/100%.

## T-024 — Establish immutable camera-recovery v1 execution-bundle boundary
**Status:** READY | **Priority:** P0 | **Type:** RECOVERY / DURABILITY | **Risk:** CRITICAL

### Characterization
- current camera plan/handlers were introduced in `e8f72edf...`; later history contains no handler semantic changes and only formatting normalization for the plan;
- current v1 canonical digest on pinned Axiom `7682ba9170dd` is `sha256:a0781f96957948af4ea5535c04f1a00ffffa40653c55cd0ed8ea0cfbcd20706f`;
- `NodeOperator` is a durable human wait and `ActivityReconnect` is an external idempotent effect with ambiguous-side-effect verification;
- current Service routes Start/Drive/ResolveOperator/Get/Serve through one current `adgo.Production` engine.

### Goal
Freeze camera v1 as an immutable plan+handler+operator-binding bundle, route non-terminal persisted operations by exact identity through Axiom Host, fail closed on unknown/mismatched state, and prove retained-v1 plus future-active coexistence before any real v2 is shipped.

### Scope
`internal/orchestration/recovery/camera`: golden v1 identity, explicitly v1-pinned handlers, bundle catalog, Host-backed Service, version-aware operator binding/Get/Serve preflight and Pebble upgrade/fault tests.

### Acceptance
- golden PlanID/PlanVersion/PlanDigest test fails on v1 graph drift;
- v1 Registry is composed only from explicitly version-pinned handler functions; a future version must introduce a distinct registry/handler set;
- Start uses only active bundle; Drive and ResolveOperator load persisted execution and use its exact engine/binding;
- Get validates unknown/mismatched non-terminal identity but remains able to read terminal historical records;
- Open synchronously validates every non-terminal persisted camera execution; Serve repeats preflight and uses Host resilient coordination rather than one active engine;
- Pebble restart from `NodeOperator` preserves v1 identity and completes an explicit operator retry through v1 without duplicate reconnect;
- unknown digest and ID/version mismatch fail Open closed; terminal retired history does not block startup;
- internal test seam proves retained v1 plus a distinct test future-active bundle coexist, old work completes under v1 and new work gets the future identity without shipping a fake production v2;
- existing healthy/reconnect/operator tests remain green;
- full CI/race/security PASS and Gremlins produces non-empty `internal/orchestration/recovery/camera` evidence with zero lived/not-covered/timeout blockers.

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

Critical edge space: `input x identity x authority x time x ordering x concurrency x persistence x external failure x ownership x topology x plan-version x handler-version x config-derived-version x signal/human-binding x cancellation x compensation x recovery x gate-classification x evidence-emptiness`.

# 13. Mutation Testing Strategy

- Critical changed semantics require real Gremlins execution and non-empty generated evidence.
- `MutantsTotal==0` is failure when selected critical targets exist.
- Config, orchestration, engloop, principal policy, command preconditions, migration guards and physical gateways are mutation-critical.
- Version/digest routing, human/reconciliation bindings, physical cancellation and compensation recovery are mutation-critical.
- Equivalent representation branches should be removed when possible rather than suppressing Gremlins; surviving/not-covered/timeout mutants require contract analysis and never justify weakening gates.

# 14. Performance Baselines

Benchmark smoke is regression smoke only. T-008 owns target-hardware latency/throughput/allocation budgets.

# 15. Security Hardening

Source-state hygiene DONE; deterministic siren test DONE; Stage17 truth DONE; remote plaintext guard DONE; config mutation boundary DONE; orchestration taxonomy DONE; zero-evidence guard DONE; incident bundles DONE; siren config-drift recovery DONE; door v1 bundle boundary DONE. Next: camera v1 boundary -> callback/principal/preconditions -> crash/fencing -> main/release qualification.

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
- selected target with zero generated mutants as evidence;
- automatic plan substitution/migration on startup;
- pinning only the DAG while reusing drifted current handlers;
- silently ignoring unknown non-terminal plan digests;
- treating current runtime config as permission to reinterpret historical physical state;
- creating a fake v2 only to exercise upgrade plumbing;
- changing a fixed-version plan/handler bundle without version bump + retained old bundle;
- suppressing an equivalent mutation when the unobservable representation branch can be removed;
- reimplementing Axiom Host/migration logic;
- force push.

# 19. Completed Tasks

T-001, T-002, T-003, T-011, T-012, T-018, T-019, T-020, T-021, T-022, T-023.

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
- **11 T-020** `36d4847...`: ci/race/security PASS; Gremlins 2/2 killed; closes F-013/F-014 and T-019/T-020.
- **12 F-015/T-021 planning** `36d9c31...`: execution-bundle invariant recorded.
- **13 T-021 scope reconciliation** `d72c415...`: callback/read-model version-aware scope recorded.
- **14 T-021 runtime** `e6e2bd4...`: mutation exposed characterization/testability blockers.
- **15 T-021 mutation recovery** `a68b497...`: improved mutation evidence with remaining blockers.
- **16 T-021 testability recovery** `a2adcd87...`: Gremlins 69/69 killed; gofmt issue remained.
- **17 T-021 format closure** `8b13e556...`: full CI/race/security + Gremlins 69/69 PASS; closes F-015/T-021.
- **18 F-016/T-022 planning** `296795e8...`: siren config-derived physical-safety invariant recorded.
- **19 T-022 runtime** `26658f7...`: unit/security PASS; Gremlins 67/1 exposed missing Stop return assertion.
- **20 T-022 mutation recovery** `dfced327...`: full CI/race/security + Gremlins 68/68 PASS; closes F-016/T-022.
- **21 F-017/T-023 planning** `74341a52...`: complete ADGO inventory; preventive door/camera retained-bundle boundary recorded; full planning gates PASS.
- **22 T-023 runtime** `e996c4cd...`: door frozen v1 handlers/digest + Host routing + Pebble tests; standard unit/security PASS; Gremlins 47 killed/1 equivalent representation survivor.
- **23 T-023 mutation recovery** `a130177e...`: remove nil-vs-empty clone branch; full CI/race/reconciliation/benchmark + security/SBOM/Trivy PASS; Gremlins 46/46 killed, 100%/100%; closes door half of F-017 and T-023.

# 21. Definition of Final Done

No unresolved Critical/High findings without accepted deferral; all P0/P1 acceptance verified; remote exposure safe; critical test-of-tests has selected targets and non-empty clean mutation evidence; explicit principal kinds; durable plan+handler+binding/config evolution and restart/rollback; crash/fencing tests; stale/idempotency HTTP contracts; security/race/static/fault/mutation green; hardware budgets met; no unexplained flakes; docs match code; final re-audit finds no fundamental blocker; last verified state is main with no force push.
