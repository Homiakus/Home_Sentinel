# Implementation status

`MASTER_PLAN.md` is the execution source of truth. This file is an observed snapshot.

## Verified foundation

T-001/T-002/T-003/T-011/T-012/T-018/T-019/T-020/T-021/T-022/T-023 are verified.

Latest fixed-version durable-upgrade evidence on `a130177ed3b2b57aded26b32ec003e6da0da2c46`:
- module/build, vulnerability, format, vet, unit, race, engineering-loop reconciliation and benchmark smoke: PASS;
- security/SBOM/Trivy: PASS;
- mutation planner: `risk=CRITICAL`, `mutation_targets=["./internal/orchestration/action/door"]`;
- Gremlins over the T-023 range: `Killed=46`, `Lived=0`, `Not covered=0`, `Timed out=0`, efficacy 100%, mutator coverage 100%;
- door v1 graph identity is frozen at `sha256:ca5201ec70e540f7323176f4a2ca156c9c6a373bd35a664d33c0178b51cd6880` and its Registry uses explicitly v1-pinned handler functions;
- Pebble tests prove restart at unlock human wait and ambiguous-side-effect reconciliation, exact persisted-bundle routing, unknown/mismatched non-terminal fail-closed behavior and terminal-history compatibility;
- an internal test-only future-active identity proves retained v1 coexistence through Axiom Host without shipping a fake semantic v2.

The first T-023 mutation campaign killed 47 mutants and exposed one lived `len(map)>0` boundary mutant in binding cloning. That mutant was representation-equivalent (`nil` versus empty map), so the unobservable branch was removed rather than suppressed; final campaign is 46/46 clean.

This closes T-023 and the door half of F-017.

## T-004 durable evolution umbrella

Complete ADGO plan inventory:
- incident — retained v1/v2 bundle handling verified;
- siren action — reconstructible configuration-derived historical bundles verified;
- door action — immutable v1 retained-bundle boundary verified;
- camera recovery — final remaining family.

Camera plan/handlers were introduced in `e8f72edf...`; later history has no handler semantic change and only formatting normalization of the plan. There is therefore no known already-misinterpreted camera durable state. The remaining risk is preventive before the first real semantic v2.

## F-017 / T-024 — camera first-v2 retained-bundle boundary

Current camera recovery still opens one current `adgo.Production` engine at `PlanVersion="1"` and routes Drive, operator resolution, Get and Serve through that engine/current node constants.

Canonical camera v1 identity on pinned Axiom `7682ba9170dd`:
`sha256:a0781f96957948af4ea5535c04f1a00ffffa40653c55cd0ed8ea0cfbcd20706f`.

T-024 is the active P0 slice. It will:
- freeze the camera v1 graph plus explicitly version-pinned handler semantics;
- register immutable bundles in Axiom Host over the existing durable Store/Router;
- route Drive and ResolveOperator by persisted PlanDigest and version-specific operator binding;
- make Get validate non-terminal identity while allowing terminal historical reads;
- validate all non-terminal persisted camera executions synchronously at Open and again before Serve;
- replace single-active-engine Serve coordination with Host resilient coordination;
- prove Pebble restart from operator wait and retained-v1 + future-active coexistence without a fake production v2;
- fail closed on unknown/mismatched non-terminal identities;
- require clean non-empty Gremlins evidence on `internal/orchestration/recovery/camera` plus full CI/race/security.

After T-024 is verified, F-017 and the T-004 durable-workflow upgrade umbrella can be reconciled closed.

## Stage 17 residuals

T-013 callback HTTP READY; T-014 principal kinds TODO; T-015 authz audit/limiting BLOCKED; T-016 command preconditions TODO; T-017 authenticator boundary BLOCKED.

## Other P0 work

T-005 crash/restore external-effect linearization, T-006 topology/fencing and T-009 release qualification remain P0. T-010 verified-main guard remains planned.
