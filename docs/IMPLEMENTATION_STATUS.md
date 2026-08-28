# Implementation status

`MASTER_PLAN.md` is the execution source of truth. This file is an observed snapshot.

## Verified foundation

T-001/T-002/T-003/T-011/T-012/T-018/T-019/T-020 are verified.

Latest recovery evidence on `36d4847bbb1bd51adc0a4e4bd76c5b480f1be918`:
- module/build, format, vet, unit, race, engineering-loop reconciliation and benchmark smoke: PASS;
- security/SBOM/Trivy: PASS;
- mutation planner: `risk=CRITICAL`, `mutation_targets=["./internal/engloop"]`;
- Gremlins over the T-019+T-020 recovery range: `Killed=2`, `Lived=0`, `Not covered=0`, efficacy 100%, mutator coverage 100%;
- zero-mutant critical evidence now fails closed.

This closes F-013/F-014 and T-019/T-020.

## F-015 / T-021 durable execution-bundle finding

T-004 characterization found that pinning only ADGO PlanDigest is not enough for Home Sentinel application semantics.

Historical incident v1 from `caa446b...` and current code both use Axiom revision `7682ba9170dd`, so the v1 definition can be retained without crossing an Axiom revision boundary. However application semantics changed:
- v1 `assessRisk` uses the original local weighted scoring formula;
- current v2 `assessRisk` uses `riskpolicy.DefaultPolicy()` and emits a different fact set;
- v1 `archiveIncident` writes `archived=true`; current archive behavior differs;
- v1 owner response waits at `await-owner-response`; v2 waits at `await-owner-ack`.

Therefore an old execution routed to the correct v1 DAG could still be executed with incorrect v2 handlers or current signal bindings if the service reuses one current Registry.

T-021 will retain immutable incident v1/v2 execution bundles containing plan + version-specific Registry/handlers + signal bindings, route by persisted PlanDigest through Axiom Host, start new work on v2, and fail closed when a non-terminal persisted digest has no registered bundle.

Required proof includes a Pebble upgrade test: create and drive v1 to waiting, close, reopen with v2 active and v1 retained, resume through the v1 binding with no duplicate notification and historical v1 semantics, while new executions use v2.

## T-004 durable evolution umbrella

Existing foundations:
- SQLite `schema_migrations` exists;
- events have `SchemaVersion`;
- ADGO executions persist PlanID/PlanVersion/PlanDigest;
- pinned Axiom rejects plan digest mismatch;
- Axiom Pebble implements `ExecutionCatalog`;
- Axiom `Host` supports multiple immutable plans over one Store and routes work by persisted digest;
- Axiom provides conservative explicit migration APIs.

T-021 is the first implementation slice of T-004. Remaining workflow/version surfaces will be reconciled after incident upgrade behavior is proven.

## Stage 17 residuals

T-013 callback HTTP READY; T-014 principal kinds TODO; T-015 authz audit/limiting BLOCKED; T-016 command preconditions TODO; T-017 authenticator boundary BLOCKED.

## Other P0 work

T-004 durable execution semantics, T-005 crash/restore linearization, T-006 topology/fencing, T-009 release qualification. T-010 verified-main guard remains planned.
