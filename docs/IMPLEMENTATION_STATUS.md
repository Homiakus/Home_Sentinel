# Implementation status

`MASTER_PLAN.md` is the execution source of truth. This file is an observed snapshot.

## Verified foundation

T-001/T-002/T-003/T-011/T-012/T-018/T-019/T-020/T-021 are verified.

Latest durable-upgrade evidence on `8b13e5565590ac1d86f1e4ea7d9c8213542022fb`:
- module/build, vulnerability, format, vet, unit, race, engineering-loop reconciliation and benchmark smoke: PASS;
- security/SBOM/Trivy: PASS;
- mutation planner: `risk=CRITICAL`, `mutation_targets=["./internal/orchestration/incident"]`;
- Gremlins over the T-021 range: `Killed=69`, `Lived=0`, `Not covered=0`, `Timed out=0`, efficacy 100%, mutator coverage 100%;
- incident v1/v2 bundle identities, historical handler semantics, callback binding, Pebble reopen and unknown-digest fail-closed behavior are covered by executable tests.

This closes F-015 and T-021.

## T-004 durable evolution umbrella

Existing foundations:
- SQLite `schema_migrations` exists;
- events have `SchemaVersion`;
- ADGO executions persist PlanID/PlanVersion/PlanDigest;
- pinned Axiom rejects plan digest mismatch;
- Axiom Pebble implements `ExecutionCatalog`;
- Axiom `Host` supports multiple immutable plans over one Store and routes work by persisted digest;
- Axiom provides conservative explicit migration APIs;
- incident v1/v2 now retain complete execution bundles: plan + version-specific Registry/handlers + signal bindings.

T-021 proves the pattern for incident workflows. Remaining workflow/version surfaces are being reconciled independently rather than mechanically copying the incident implementation.

## F-016 / T-022 — siren configuration-derived plan drift

A Critical physical-safety gap was found while inventorying the next T-004 surface.

`internal/orchestration/action/siren/plan.go` derives durable plan identity from runtime configuration: `Version = "1-" + MaxActivationDuration.String()`, and the safety timer uses that duration. The enable node owns a physical effect and has `SirenEnsureDisabled` compensation.

Current `siren.Open` compiles only the currently configured duration and opens one single-plan ADGO Production runtime. Pinned Axiom single-engine coordinator semantics skip persisted executions whose PlanDigest differs from the current engine. Therefore this crash/config-change sequence is unsafe:
1. execution enables a siren and durably reaches the safety wait;
2. process crashes;
3. `MaxActivationDuration` changes before restart;
4. runtime reopens under a different PlanVersion/PlanDigest;
5. the old non-terminal execution can be skipped and its timer/disable/compensation can become unreachable through the current service.

T-022 is the next P0 slice. It will reconstruct historical siren plans strictly from canonical persisted duration versions, verify exact PlanID/PlanVersion/PlanDigest, register active + historical engines in Axiom Host, route Drive/Stop by persisted bundle identity, and fail closed on malformed or mismatched non-terminal state.

Required proof includes a Pebble crash/reopen test where duration A enables the device, runtime reopens with duration B, the old execution is canceled through its historical engine, and compensation demonstrably disables the still-enabled siren while new executions use duration B.

## Stage 17 residuals

T-013 callback HTTP READY; T-014 principal kinds TODO; T-015 authz audit/limiting BLOCKED; T-016 command preconditions TODO; T-017 authenticator boundary BLOCKED.

## Other P0 work

T-004 durable execution semantics continues via T-022 and later remaining workflow surfaces; T-005 crash/restore linearization, T-006 topology/fencing, T-009 release qualification. T-010 verified-main guard remains planned.
