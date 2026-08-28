# Implementation status

## Status semantics

Observed snapshot only. `MASTER_PLAN.md` owns execution ordering/status; detailed intent remains in `docs/AXIOM_IMPLEMENTATION_PLAN.md` and `docs/SCENARIO_SYSTEM_PLAN.md`. Completion requires executable evidence.

## Verified baseline

- Core Go control plane, Axiom/ADGO durable workflows, auth/authz, SQLite migrations/state, gateways/recovery and Scenario headless model/compiler/safety/catalog/simulator are present.
- Standard CI covers module hygiene, format, vet, unit, race, engineering reconciliation and benchmark smoke.
- Security qualification includes supply-chain check, CycloneDX SBOM and Trivy repository qualification.
- T-011 removed the siren scheduler-timing flake.
- Stage 17 evidence is reconciled in [`STAGE17_RECONCILIATION.md`](STAGE17_RECONCILIATION.md).

## T-012/T-018 — verified runtime exposure + test-of-tests recovery

The current production HTTP runtime is plaintext, therefore production `Config.Validate()` now permits only loopback listen addresses. Wildcard, unspecified, LAN and arbitrary remote hostnames fail closed; malformed host:port is rejected separately; default `127.0.0.1:8080` remains valid.

The first T-012 mutation attempt exposed an engineering-loop defect: config security changes were not mutation-targetable. T-018 repaired that boundary.

Final evidence on `26b28f4505f7b4cd4059c14d310b326b97cdef50`:

- `ci`: PASS including race, reconciliation and benchmark smoke;
- `security`: PASS including SBOM and Trivy;
- `mutation`: PASS;
- planner: `risk=CRITICAL`, `mutation_targets=["./internal/config"]`;
- Gremlins: killed 1, lived 0, not covered 0, efficacy 100%, mutator coverage 100%.

Therefore F-006/F-012 and T-012/T-018 are resolved/verified.

## F-013 — next engineering-loop blocker

Read-only T-004 characterization found that safety-critical `internal/orchestration/` and the gate-policy engine `internal/engloop/` are outside `isCriticalSurface`. A change to incident/door/siren/recovery plan routing could therefore be classified only MEDIUM and skip mutation entirely; the code deciding whether mutation runs is also not self-protected.

T-019 is now the next P0 prerequisite: make both boundaries CRITICAL and prove the taxonomy change self-triggers mutation.

## T-004 durable plan characterization

Existing foundations are stronger than the old roadmap implied:

- SQLite already tracks `schema_migrations`;
- event envelopes already persist and validate `SchemaVersion`;
- Home Sentinel ADGO plans already have IDs/versions;
- ADGO executions persist `PlanID`, `PlanVersion`, `PlanDigest`;
- pinned Axiom rejects plan-digest mismatch;
- pinned Axiom already provides conservative `ValidatePlanMigration` / `MigrateExecution`.

The actual Home Sentinel gap is runtime plan availability/routing: each orchestration service currently opens one current plan. Old non-terminal executions need an immutable historical plan catalog and dispatch by persisted digest; explicit migration should reuse Axiom rather than silently replace the plan.

Important edge case: Siren plan version depends on `MaxActivationDuration`, so changing the safety timeout creates a new plan/digest. An old waiting siren execution must still be resolvable after restart.

T-004 remains BLOCKED until T-019 gives this work real mutation test-of-tests.

## Stage 17 remaining work

- T-013: narrow authenticated callback HTTP bearer adapter — now READY after T-012.
- T-014: explicit human/service/system principal kinds and authority rules.
- T-015: generic authorization audit + principal-aware abuse limiting.
- T-016: common stale-command expected-version/ETag + request idempotency contract.
- T-017: pluggable authenticator boundary after principal semantics.

## Other P0 production work

- T-005 crash/restore external-effect linearization.
- T-006 explicit process topology/distributed fencing policy.
- T-009 release provenance/reproducibility/rollback qualification.
- T-010 technical verified-main guard.

## Dependency-aware order

1. T-019 orchestration/gate-policy mutation boundary.
2. T-004 immutable pinned-plan catalog/routing.
3. T-013/T-014/T-016 Stage 17 P0 residuals in dependency-safe order.
4. T-005/T-006 crash/restore and topology/fencing.
5. T-007 authenticated Scenario API.
6. T-008/T-009/T-010 operational/release/main-guard qualification.

Execution truth: `../MASTER_PLAN.md`
