# Implementation status

`MASTER_PLAN.md` is the execution source of truth. This file is an observed snapshot.

## Verified foundation

T-001/T-002/T-003/T-011/T-012/T-018/T-019/T-020/T-021/T-022 are verified.

Latest physical-safety durable-upgrade evidence on `dfced3277a61211ae312b8eef18e761b980a9238`:
- module/build, vulnerability, format, vet, unit, race, engineering-loop reconciliation and benchmark smoke: PASS;
- security/SBOM/Trivy: PASS;
- mutation planner: `risk=CRITICAL`, `mutation_targets=["./internal/orchestration/action/siren"]`;
- Gremlins over the T-022 range: `Killed=68`, `Lived=0`, `Not covered=0`, `Timed out=0`, efficacy 100%, mutator coverage 100%;
- the first T-022 runtime commit exposed exactly one lived `Stop` return-contract mutant; the targeted test-only recovery killed it without changing production semantics;
- Pebble tests prove duration-A enable -> close -> duration-B reopen -> exact historical identity -> historical cancellation/compensation disables the siren, while new executions use duration B.

This closes F-016 and T-022.

## T-004 durable evolution umbrella

Complete ADGO plan inventory:
- incident;
- siren action;
- door action;
- camera recovery.

Incident and siren now have verified multi-plan/reconstructible durable execution-bundle behavior.

Door and camera characterization found no current historical semantic drift:
- door plan/handlers were introduced in `dc1e32de...`; later plan/handler history is formatting-only;
- camera plan/handlers were introduced in `e8f72edf...`; no later handler semantic change is present and the plan only received formatting normalization.

However both services still open one current `adgo.Production` engine with `PlanVersion="1"` and route human/reconciliation operations through current constants. That is safe for the current unchanged v1 history, but it is not a safe release boundary for their first future semantic v2.

## F-017 / T-023 / T-024 — fixed-version future upgrade boundary

F-017 is preventive rather than evidence of already-corrupted durable state. The risk is that a future door/camera v2 could strand existing v1 non-terminal executions or bind them to new handlers/nodes if the release changes the current plan without retaining v1.

T-023 is the next atomic P0 slice for door access control. It will:
- freeze current v1 PlanID/PlanVersion/PlanDigest as a golden execution-bundle identity;
- establish Host-backed bundle routing and fail-closed non-terminal startup validation;
- route Drive, unlock approval and reconciliation by persisted bundle identity;
- provide an internal multi-bundle test seam proving retained v1 can coexist with a distinct future active plan without shipping a fake semantic v2;
- prove Pebble restart/human/reconciliation behavior and mutation cleanliness.

T-024 will apply the verified pattern independently to camera recovery, after which T-004 can be reconciled closed.

## Stage 17 residuals

T-013 callback HTTP READY; T-014 principal kinds TODO; T-015 authz audit/limiting BLOCKED; T-016 command preconditions TODO; T-017 authenticator boundary BLOCKED.

## Other P0 work

T-005 crash/restore linearization, T-006 topology/fencing and T-009 release qualification remain P0. T-010 verified-main guard remains planned.
