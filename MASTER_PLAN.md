# Home Sentinel — Living MASTER PLAN

> Single execution source of truth for progressive audit, atomic implementation and verified direct-to-main delivery. Detailed product/architecture documents remain intent specifications; this file owns findings, invariants, ordering and execution state.

**Plan revision:** 2026-08-29 / T-005 verified on `2e8b9606212f47bf04d66bf5f1833a52195b7018`; crash/restore external-effect linearization is resolved. F-018 / T-006 single-writer topology is the active P0 slice.

---

# 1. Mission

Deliver a production-grade local security/automation platform through small reviewable changes while preserving fail-closed authority, durable external-effect semantics, reproducibility, rollback and evidence-driven evolution.

`AUDIT -> PLAN -> SELECT -> CHARACTERIZE -> IMPLEMENT -> TEST -> SELF-REVIEW -> RECONCILE -> COMMIT -> PUSH main -> VERIFY -> COMPRESS -> NEXT`

Unexpected material discoveries are recorded before implementation scope expands.

# 2. Current State

- Go control plane: typed domain/events, Axiom lifecycle, ADGO durable workflows, auth/authz, SQLite state, gateways/recovery, Scenario compiler/safety/catalog/simulator.
- Verified tasks: T-001, T-002, T-003, T-004, T-005, T-011, T-012, T-018, T-019, T-020, T-021, T-022, T-023, T-024.
- Every ADGO workflow family has an explicit execution-bundle evolution boundary: incident, configuration-derived siren, fixed-version door, fixed-version camera.
- T-005 now proves the exact ADGO at-least-once crash window `provider accepted effect -> completion commit lost` for door/siren/camera with deterministic Pebble restart/lease recovery and preserved idempotency identity.
- T-005 final evidence on `2e8b9606...`: module/supply-chain/vulnerability/format/vet/unit/race/reconciliation/benchmark PASS; security/SBOM/Trivy PASS; engineering-loop classifies the slice CRITICAL while correctly selecting no mutation target because the final implementation is `_test.go`-only.
- T-006 owns the remaining physical-control topology boundary: current `resourceguard.Check + StartOrLoad` is serialized only inside one process. v1 will technically support exactly one control-plane writer per canonical runtime root and fail a second writer before application services become available.
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
ADGO/Pebble : pinned execution identity + durable tasks/history
Axiom Host  : multi-plan routing by persisted digest
Bundle      : immutable graph + handlers + external bindings/config semantics
Topology    : one v1 control-plane writer per canonical runtime root
Scenario    : AST -> validation -> Safety Compiler -> lowering -> catalog/simulator
```

Primary boundaries: `internal/domain`, `internal/orchestration`, `internal/gateway`, `internal/auth`, `internal/authz`, `internal/config`, `internal/database`, `internal/httpserver`, `internal/scenario`, `internal/integrations`, `internal/engloop`.

# 4. Baseline

| Gate | Baseline |
|---|---|
| module/build hygiene | PASS |
| vulnerability/static | PASS |
| formatting/vet | PASS |
| unit/integration/fault | PASS |
| race | PASS; no active unexplained flake |
| security/SBOM/Trivy | PASS |
| config critical mutation | PASS, non-empty killed evidence |
| incident durable-upgrade mutation | PASS, 69/69 killed |
| siren durable-upgrade mutation | PASS, 68/68 killed |
| door durable-upgrade mutation | PASS, 46/46 killed |
| camera durable-upgrade mutation | PASS, 52/52 killed |
| external-effect crash proof | PASS on `2e8b9606...` |
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
- **I-019** Durable workflow identity pins an execution bundle: plan graph, version-specific handlers and external signal/human bindings.
- **I-020** Unknown persisted non-terminal plan/bundle identity fails startup/control-plane qualification closed.
- **I-021** Configuration-derived durable plans owning physical effects are reconstructible/routable by persisted identity across restart.
- **I-022** A fixed-version durable workflow has an immutable-bundle release boundary before semantic v2.
- **I-023** ADGO external effects are at-least-once: after `provider accepted -> completion commit lost`, recovery may redeliver only with the same durable idempotency identity; otherwise explicit reconciliation is required.
- **I-024** Crash/fault tests control commit/lease/timer boundaries deterministically; process sleeps and scheduler luck are not correctness evidence.
- **I-025** Supported v1 physical topology has exactly one Home Sentinel control-plane writer per canonical runtime root. A second writer must fail before application/runtime services are exposed. Multi-writer operation requires explicit distributed fencing and is not inferred from process-local mutexes or expiring admission leases.

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

## F-016 — Config-derived siren plan could strand an enabled physical effect across restart
**Status:** Resolved | **Severity:** Critical | **Confidence:** Confirmed | T-022.

## F-017 — Door/camera fixed-version services lacked a first-v2 retained-bundle boundary
**Status:** Resolved | **Severity:** Critical | **Confidence:** Confirmed | T-023 + T-024.

## F-018 — Physical-resource admission is only process-local
**Status:** Active | **Severity:** Critical | **Confidence:** Confirmed | **Task:** T-006.

**Evidence:** `internal/orchestration/resourceguard.Check` explicitly requires callers to serialize `Check + StartOrLoad` within one process and states that multi-process control planes need distributed admission/fencing. Door/siren/camera services use process-local `sync.Mutex` around this admission sequence. Pinned ADGO exposes durable `AdmissionController`, but Home Sentinel physical services do not use it. Pinned ADGO Pebble explicitly owns a process-level database lock.

**Root cause:** durable execution ownership is checked through persisted state, but admission of a *new* conflicting execution is not atomic across two Home Sentinel processes. Two writers can both observe “free” before either creates its durable reservation.

**Blast radius:** physical door commands, siren activation, camera recovery and any future actuator using the same `resourceguard` pattern. An unsupported multi-process deployment could create conflicting durable physical owners.

**Decision:** v1 supports one control-plane writer per canonical runtime root, enforced by a process-lifetime Pebble lock acquired before application services initialize. Expiring admission TTL is not treated as whole-execution fencing because human/reconciliation waits can outlive it. Distributed multi-writer fencing is explicitly deferred until it has a non-expiring/heartbeated ownership protocol with device-level topology semantics.

# 7. Risk Register

| Risk | Severity | Control |
|---|---|---|
| provider accepted physical effect but completion commit lost | Critical | T-005 DONE |
| two control-plane writers admit conflicting physical owners | Critical | T-006 ACTIVE |
| callback transport absent | High | T-013 |
| machine/human principal ambiguity | High | T-014 |
| stale/duplicate HTTP command | High | T-016 |
| direct main without required checks | High | T-010 |
| release provenance/rollback | High | T-009 |

# 8. Pareto Improvements

1. T-006 technically enforces the supported physical-control topology.
2. T-013/T-014 establish callback transport and machine-vs-human authority.
3. T-016 adds common command preconditions/idempotency.
4. T-015/T-010 tighten audit/abuse/main protection.
5. T-008/T-009 complete hardware and release qualification.

# 9. Dependency DAG

```text
T-001 DONE -> T-002 DONE -> T-011 DONE -> T-003 DONE
                                      |-> T-012 DONE -> T-018 DONE -> T-013 READY
                                      |-> T-019 DONE -> T-020 DONE -> T-021 DONE -> T-022 DONE -> T-023 DONE -> T-024 DONE -> T-004 DONE
                                      |                                                                                             |-> T-005 DONE -> T-006 ACTIVE
                                      |-> T-014 -> T-015 / T-017
                                      |-> T-016
T-001 -> T-010

T-014 + T-016 + relevant T-013/T-004 -> T-007
T-004 + T-005 + T-006 + T-008 -> T-009
```

# 10. Implementation Phases

- **A Truth/gates:** T-001/T-002/T-003/T-011/T-012/T-018/T-019/T-020 DONE; T-010 remains.
- **B Durable control:** T-004/T-005/T-021/T-022/T-023/T-024 DONE -> T-006 ACTIVE; T-013..T-017 in dependency order.
- **C Product surface:** T-007.
- **D Qualification:** T-008/T-009.

# 11. Atomic Tasks

## T-001/T-002/T-003/T-004/T-005/T-011/T-012/T-018/T-019/T-020/T-021/T-022/T-023/T-024
**Status:** DONE. Verified commits/evidence recorded below.

## T-005 — Prove crash/restore external-effect linearization
**Status:** DONE | **Priority:** P0 | **Final verified SHA:** `2e8b9606212f47bf04d66bf5f1833a52195b7018`.

Deterministic fault tests now fail exactly the post-provider completion commit, preserve the running durable task/idempotency key, restart Pebble, force lease recovery without sleeps and verify safe redelivery. Door suppresses the duplicate by observed desired state; camera redelivers the same key and provider deduplicates; siren suppresses duplicate enable then independently performs one controlled safety-disable; unresolved ambiguous door outcome remains human/reconciliation without blind replay. Final full CI/race/security PASS. Production semantics were unchanged, so mutation targeting correctly selected no production target.

## T-006 — Enforce supported single-writer physical topology
**Status:** ACTIVE | **Priority:** P0 | **Type:** PHYSICAL OWNERSHIP / TOPOLOGY | **Risk:** CRITICAL | Depends T-004/T-005.

### Characterization
- `resourceguard.Check` reserves a physical resource through non-terminal durable execution state but its `Check + StartOrLoad` admission is only process-locally serialized;
- door/siren/camera each rely on process-local `startMu` around that sequence;
- two processes can race before either durable execution exists;
- ADGO `Production.Admission` is available but an expiring permit is not sufficient whole-execution fencing for human/reconciliation waits without a dedicated heartbeat/ownership lifecycle;
- pinned ADGO `PebbleStore` documents a process-level database lock held for the DB lifetime.

### Goal
Make supported v1 deployment topology mechanically true: only one Home Sentinel control-plane writer may own a canonical runtime root at a time. The guard must be acquired before application services are initialized and held until application shutdown. Multi-writer/distributed mode remains explicitly unsupported until a real fencing protocol exists.

### Preferred implementation
Add a small production package under `internal/orchestration/topology` that canonicalizes the runtime root and opens a dedicated Pebble lock database such as `<runtime-root>/control-plane-writer`. Integrate acquisition at the beginning of `app.Open`, before database migration/service startup; store the guard on `App`; release it last from `App.Close`, including partial-startup cleanup. Do not use stale PID files or expiring TTL leases as fencing.

### Acceptance
- canonical runtime root is stable for relative/absolute aliases and invalid/empty roots fail closed;
- first process/app acquires the writer guard; a concurrent second process using the same canonical root fails before DB/service availability;
- lock remains held through the whole App lifetime, including human/reconciliation waits in downstream workflows;
- `Close` releases ownership and a subsequent process can acquire the same root;
- partial `app.Open` failure after lock acquisition releases the guard;
- subprocess test proves cross-process exclusion deterministically using handshake/IPC, not sleeps;
- docs explicitly state one canonical runtime root per installation and do not claim different roots controlling the same devices are safe;
- distributed multi-writer mode remains unsupported; ADGO TTL admission is not misrepresented as device fencing;
- production topology code is mutation-critical and Gremlins must produce non-empty clean evidence;
- full module/static/unit/fault/race/replay/security/benchmark gates PASS.

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

G0 build/static/module; G1 unit/golden; G2 property/fuzz/model; G3 race; G4 contract/integration; G5 fault/crash/topology; G6 mutation; G7 E2E/HIL/UX; G8 performance/soak/release.

Critical edge space: `input x identity x authority x time x ordering x concurrency x process-count x runtime-root-alias x persistence x external failure x physical ownership x topology x plan-version x task-state x lease-state x provider-acceptance x completion-commit x cancellation x compensation x recovery x gate-classification x evidence-emptiness`.

# 13. Mutation Testing Strategy

- Critical changed production semantics require real Gremlins execution and non-empty generated evidence.
- `MutantsTotal==0` is failure when selected critical production targets exist.
- `_test.go` files are excluded from mutation targets; test-only critical fault slices may legitimately have no mutation target.
- Never add semantic-neutral production code only to manufacture mutants.
- Config, orchestration, topology, engloop, principal policy, command preconditions, migration guards and physical gateways are mutation-critical whenever production code changes.
- Lived/not-covered/timeout mutants require contract analysis; equivalent branches should be simplified rather than suppressed.

# 14. Performance Baselines

Benchmark smoke is regression smoke only. T-008 owns target-hardware latency/throughput/allocation budgets. T-006 writer-lock acquisition is startup-only and must not enter per-request/per-effect hot paths.

# 15. Security Hardening

Source-state hygiene DONE; deterministic safety tests DONE; Stage17 truth DONE; remote plaintext guard DONE; config mutation boundary DONE; orchestration taxonomy DONE; zero-evidence guard DONE; durable execution bundles DONE; deterministic physical crash linearization DONE. Next: single-writer topology -> callback/principal/preconditions -> main/release qualification.

# 16. Migration Strategy

`characterize old semantics -> pin immutable historical execution bundle -> introduce active new bundle only for a real semantic change -> route existing state by durable identity -> explicit migration only at safe/quiescent point -> verify restart/rollback -> retire old bundle only after no durable references remain`.

Topology migration rule: a v1 installation has one canonical runtime root and one control-plane writer. Multi-writer deployment is a future explicit migration requiring a real distributed fencing contract, not a configuration toggle around current process-local admission.

# 17. Deferred Work

Broad Scenario UI/UX; full TLS listener/cert lifecycle; OIDC/mTLS implementations; distributed multi-writer physical control; low-leverage refactors.

# 18. Rejected Decisions

- roadmap checkbox as proof;
- green rerun as flake resolution;
- timeout inflation as safety evidence;
- callback HTTP before remote plaintext is impossible;
- ADGO workflow idempotency as HTTP idempotency;
- service/system principals as fake users;
- selected production mutation target with zero generated mutants as evidence;
- production refactor added only to force mutation evidence for test-only work;
- automatic plan substitution/migration on startup;
- pinning only DAG while reusing drifted handlers;
- silently ignoring unknown non-terminal plan digests;
- treating current config as permission to reinterpret historical physical state;
- changing fixed-version semantics without version bump + retained bundle;
- sleep-based crash tests when durable state can be controlled;
- claiming exactly-once from ADGO task state;
- assuming process-local `sync.Mutex` is distributed fencing;
- using expiring admission TTL as whole-execution ownership without heartbeat/lifecycle proof;
- stale PID files as safety fencing;
- claiming multiple runtime roots controlling the same physical devices are safe;
- force push.

# 19. Completed Tasks

T-001, T-002, T-003, T-004, T-005, T-011, T-012, T-018, T-019, T-020, T-021, T-022, T-023, T-024.

# 20. Iteration Log

- **1 T-001** `905862f...`: master plan; ci/security/mutation PASS.
- **2 T-002** `745f194...`: runtime DB hygiene; F-005 discovered.
- **3 F-005** `c30b395...`: finding reconciliation.
- **4 T-011** `5ccff8bf...`: deterministic siren compensation test; PASS.
- **5 T-003** `ef8ecaf...`: Stage17 evidence reconciliation; PASS.
- **6-11 T-012/T-018/T-019/T-020** through `36d4847...`: runtime plaintext guard, mutation taxonomy and zero-evidence fail-closed; PASS.
- **12-17 T-021** through `8b13e556...`: incident bundles; Gremlins 69/69; PASS.
- **18-20 T-022** through `dfced327...`: siren config-derived bundles; Gremlins 68/68; PASS.
- **21-23 T-023** through `a130177e...`: door retained v1 bundle; Gremlins 46/46; PASS.
- **24-27 T-024** through `6a2be3ed...`: camera retained v1 bundle; one intermediate compile defect caught by package loading; final Gremlins 52/52 and full gates PASS.
- **28 T-005 plan** `1d1fa6a...`: T-004 closure + crash-proof packet activated; planning gates PASS.
- **29 T-005 scope** `5f4ea581...`: siren acceptance corrected to distinguish enable from later safety-disable; planning gates PASS.
- **30 T-005 fault proof** `bf04ca4a...`: door/camera/ambiguity evidence passed; unit exposed a siren test-model assumption that `RunLocal` would complete a future timer. Classified as test-model defect, not rerun.
- **31 T-005 recovery** `2e8b9606...`: persisted timer is deterministically made due; module/supply-chain/vulnerability/format/vet/unit/race/reconciliation/benchmark and security/SBOM/Trivy PASS; test-only mutation classification is correct. T-005 DONE.
- **32 F-018/T-006 characterization:** process-local `resourceguard.Check + StartOrLoad` cannot support two control-plane writers; v1 single-writer process-lifetime topology selected.

# 21. Definition of Final Done

No unresolved Critical/High findings without accepted deferral; all P0/P1 acceptance verified; remote exposure safe; critical production test-of-tests has selected targets and non-empty clean mutation evidence; explicit principal kinds; durable plan+handler+binding/config evolution and restart/rollback; deterministic external-effect crash/fencing tests; supported topology technically enforced; stale/idempotency HTTP contracts; security/race/static/fault/mutation green; hardware budgets met; no unexplained flakes; docs match code; final re-audit finds no fundamental blocker; last verified state is main with no force push.
