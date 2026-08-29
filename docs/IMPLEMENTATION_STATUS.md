# Implementation status

`MASTER_PLAN.md` is the execution source of truth. This file is an observed snapshot.

## Verified foundation

T-001/T-002/T-003/T-004/T-011/T-012/T-018/T-019/T-020/T-021/T-022/T-023/T-024 are verified.

Latest durable-upgrade evidence on `6a2be3eddd54aff6a69ee1693b2034fbe92d910d`:
- module/build, vulnerability, format, vet, unit, race, engineering-loop reconciliation and benchmark smoke: PASS;
- security/SBOM/Trivy: PASS;
- mutation planner: `risk=CRITICAL`, `mutation_targets=["./internal/orchestration/recovery/camera"]`;
- Gremlins over the T-024 range: `Killed=52`, `Lived=0`, `Not covered=0`, `Timed out=0`, efficacy 100%, mutator coverage 100%; artifact `9713262585`, SHA-256 `c7442a70aaa5cd1523892d2c73a463e5ec0dbcf7e6d04c98634b73ed4897c344`;
- camera v1 identity is frozen at `sha256:a0781f96957948af4ea5535c04f1a00ffffa40653c55cd0ed8ea0cfbcd20706f` with explicitly v1-pinned handler semantics/operator binding;
- Pebble tests prove restart at operator wait, exact persisted-bundle routing, unknown/mismatched non-terminal fail-closed behavior, terminal-history compatibility, and retained-v1 + distinct future-active coexistence;
- the first T-024 mutation campaign exposed one uncovered existing-execution guard and one Serve-preflight timeout mutant; deterministic tests/seam closed both without weakening gates;
- an intermediate recovery commit introduced a `ServeResilient` function-signature compile mismatch; govulncheck package loading caught it immediately, it was classified as introduced, and the exact adapter fix was reverified through the full suite.

This closes T-024, F-017 and umbrella T-004.

## Durable workflow evolution inventory — complete

- incident — retained v1/v2 graph + handler + callback/human binding semantics verified;
- siren action — reconstructible configuration-derived historical bundles verified;
- door action — immutable v1 retained-bundle boundary verified;
- camera recovery — immutable v1 retained-bundle boundary verified.

No ADGO workflow family remains without an explicit upgrade/restart identity boundary.

## T-005 — active P0 physical crash-linearization slice

Pinned ADGO is explicitly at-least-once. It commits a durable task plus idempotency key before invoking an external worker. If a provider accepts an effect and the process dies before the activity completion commit, task redelivery is legal. Exactly-once cannot be inferred from ADGO state; the handler/provider boundary must deduplicate or reconcile.

Home Sentinel already passes `gateway.Operation{ExecutionID, IdempotencyKey}` to door, siren and camera physical gateways, and their fakes deduplicate confirmed provider effects. What is still missing is a deterministic end-to-end fault proof of the exact crash window.

T-005 will use public pinned ADGO Store/Pebble/Engine contracts plus Home Sentinel plans/registries to:
- fail exactly the activity completion commit after the fake provider has physically applied the effect;
- preserve the underlying Pebble running task and original idempotency key;
- close/reopen durable state and deterministically expire the lease without sleeps;
- redeliver from a new engine/coordinator and prove identical idempotency identity;
- prove one physical application for door, siren and camera reconnect;
- prove unknown provider outcomes remain explicit ambiguous/reconciliation paths and are not blindly replayed.

The Stage17 durable Telegram notifier already has its own persistent `prepared -> sending -> applied/ambiguous` crash protocol and is not duplicated by this slice.

If T-005 remains genuinely `_test.go`-only, engineering-loop mutation targeting is expected to select no production target. This is intentional: I-018 forbids zero-mutant evidence only when a critical production target was actually selected; no semantic-neutral production refactor will be added just to manufacture mutation statistics.

## Stage 17 residuals

T-013 callback HTTP READY; T-014 principal kinds TODO; T-015 authz audit/limiting BLOCKED; T-016 command preconditions TODO; T-017 authenticator boundary BLOCKED.

## Other P0 work

T-006 topology/fencing and T-009 release qualification remain P0. T-010 verified-main guard remains planned; T-008 owns target-hardware performance budgets.
