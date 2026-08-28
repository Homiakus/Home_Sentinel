# Implementation status

## Status semantics

Observed snapshot only. `MASTER_PLAN.md` owns execution ordering/status; detailed intent remains in `docs/AXIOM_IMPLEMENTATION_PLAN.md` and `docs/SCENARIO_SYSTEM_PLAN.md`. Completion requires executable evidence.

## Current baseline

- Core Go control plane, Axiom/ADGO durable workflows, auth/authz, SQLite migrations/state, gateways/recovery and Scenario headless model/compiler/safety/catalog/simulator are present.
- Standard CI covers module hygiene, formatting, vet, unit, race, engineering reconciliation and benchmark smoke.
- Security qualification and critical-diff mutation are separate mandatory gates for applicable work.
- T-011 removed the known siren race-test flake; no active unexplained flake is currently recorded.
- Stage 17 evidence is reconciled in [`STAGE17_RECONCILIATION.md`](STAGE17_RECONCILIATION.md).

## T-012 runtime remote-bind hardening

Commit `d57f239f64477f46c0b6ca7d324f0423596eaf8d` implements the fail-closed runtime rule: until the production HTTP server actually terminates TLS, runtime binds are loopback-only.

Observed on that commit:

- standard `ci`: PASS;
- `security`: PASS;
- mutation: FAILED before Gremlins execution because the new Work Packet used an invalid `status_before` enum.

The runtime validator itself is therefore not classified as broken, but T-012 is **not verified** because its required test-of-tests did not execute.

## F-012 / T-018 — mutation boundary recovery

The mutation failure exposed a deeper engineering-loop defect:

1. T-012 Work Packet used execution-plan vocabulary (`READY`) instead of WorkPacket `PlanState` vocabulary and used `mutation` instead of the real `mutation-critical` gate name.
2. More importantly, `internal/config/` was only classified HIGH and was absent from `MutationTargets`. Even a CRITICAL packet could therefore request mutation while producing no config mutation target.

T-018 repairs this boundary:

- `internal/config/` becomes a CRITICAL surface;
- production Go changes under it map to `./internal/config` mutation target;
- architecture tests pin classification, target generation and mutation/security gates;
- T-012 Work Packet is corrected to valid schema values;
- active mutation base stays at `ef8ecaf44d114c5e6341dab5fa6b869952407421`, so the recovery campaign includes the original remote-bind validator change rather than testing only the engloop fix.

T-012 may become DONE only after T-018 lets the mutation workflow reach Gremlins and the resulting mutants are killed/accepted by the mutation evidence validator.

## Stage 17 remaining work

- T-013: narrow authenticated callback HTTP bearer adapter.
- T-014: explicit human/service/system principal kinds and authority rules.
- T-015: generic authorization audit + principal-aware abuse limiting.
- T-016: common stale-command expected-version/ETag + request idempotency contract.
- T-017: pluggable authenticator boundary after principal semantics.

## Other P0 production work

- T-004 durable event/lifecycle/ADGO plan versioning and in-flight upgrade/rollback.
- T-005 crash/restore external-effect linearization.
- T-006 explicit process topology / distributed fencing policy.
- T-009 release provenance/reproducibility/rollback qualification.

## Next order

1. Verify T-018 recovery and then close T-012 if Gremlins proves the remote-bind guard mutation-resistant.
2. Decompose/execute T-004 durable versioning as the next foundational Critical track.
3. In parallel dependency order, close T-013/T-014/T-016 Stage 17 P0 residuals before authenticated Scenario API.
4. T-005/T-006, then product API expansion and operational qualification.

Execution truth: `../MASTER_PLAN.md`
