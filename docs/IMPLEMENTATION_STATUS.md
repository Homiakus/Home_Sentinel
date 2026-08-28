# Implementation status

`MASTER_PLAN.md` is the execution source of truth. This file is an observed snapshot.

## Verified foundation

T-001/T-002/T-003/T-011/T-012/T-018 are verified. On `26b28f...`, runtime remote plaintext is loopback-only and config security changes are mutation-critical with a real killed mutant.

## T-019 current state

Commit `6b55fe38b7948a18f14c3eb89051887a482a97ee` changes the engineering-loop taxonomy so `internal/orchestration/` and `internal/engloop/` are CRITICAL. Architecture tests cover incident, siren, camera recovery and gate-policy paths.

Evidence:
- planner: PASS, `risk=CRITICAL`, `mutation_targets=["./internal/engloop"]`;
- critical-diff Gremlins: executed;
- standard CI/race: PASS;
- security/SBOM/Trivy: PASS;
- Gremlins result: `Killed=0`, `Lived=0`, `Not covered=0`, `MutantsTotal=0`, efficacy 0%.

The workflow currently reports this as success, but the project does not accept zero generated mutants as proof for a selected CRITICAL target. Therefore T-019 is VERIFYING/BLOCKED rather than DONE.

## F-014 / T-020

`internal/engloop/mutation.go` records `MutantsTotal`, but `cmd/sentinel-engloop runMutation` only fails on `CriticalBlockers`; zero evidence exits success. T-020 will make zero-mutant critical campaigns fail closed and rerun a range beginning before T-019 so the taxonomy and the new evidence guard are qualified together.

## T-004 characterization after T-019/T-020

Existing foundations:
- SQLite `schema_migrations` already exists;
- events have `SchemaVersion`;
- ADGO executions persist PlanID/PlanVersion/PlanDigest;
- pinned Axiom rejects plan digest mismatch;
- Axiom Pebble implements `ExecutionCatalog`;
- Axiom `Host` supports multiple registered immutable plans over one store and routes existing execution work by persisted digest;
- Axiom already supplies conservative explicit migration APIs.

Actual Home Sentinel gap: orchestration services still open one current plan. Historical plans and plan-specific signal binding metadata are not retained. Real incident v1 from commit `caa446b...` waits on node `await-owner-response`, while v2 uses `await-owner-ack`; sending only the current target node would not resume v1 safely.

Next after T-020/T-019 closure: integrate Axiom Host for incident v1/v2 with exact persisted-digest routing and version-specific owner-response binding, then prove durable reopen/upgrade.

## Stage 17 residuals

T-013 callback HTTP READY; T-014 principal kinds TODO; T-015 authz audit/limiting BLOCKED; T-016 command preconditions TODO; T-017 authenticator boundary BLOCKED.

## Other P0 work

T-004 durable plan evolution, T-005 crash/restore linearization, T-006 topology/fencing, T-009 release qualification. T-010 verified-main guard remains planned.
