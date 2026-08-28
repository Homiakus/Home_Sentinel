# Implementation status

## Status semantics

Observed snapshot only. `MASTER_PLAN.md` owns execution ordering/status; `docs/AXIOM_IMPLEMENTATION_PLAN.md` and `docs/SCENARIO_SYSTEM_PLAN.md` own detailed intent. Completion requires executable evidence.

## Current baseline

- Go control plane with domain/event model, Axiom lifecycle, ADGO durable workflows, persistence/migrations, auth/authz, gateways/recovery and Scenario model/compiler/safety/catalog/simulator.
- CI baseline: module hygiene, format, vet, unit, race, security qualification, critical-diff mutation and benchmark smoke.
- Supply-chain baseline includes module lock, scanner/action pinning, vulnerability checks and SBOM evidence; release provenance/signing/rollback remain release qualification.
- Runtime callback key material is loaded from secret references; callback security exposes a narrow acceptance/signing boundary.
- Same-store physical resource reservations exist for Door/Siren/Camera Recovery; multi-process fencing remains open.
- Headless Scenario stages 28–34 are implemented; authenticated API/UI authoring remains blocked by authority/API prerequisites.

## Stage 17 — reconciled status

Stage 17 remains **PARTIAL**. Detailed evidence matrix: [`STAGE17_RECONCILIATION.md`](STAGE17_RECONCILIATION.md).

### Verified

- bounded HTTP read-header/read/write/idle timeouts;
- bounded + strict JSON command decoder;
- request IDs;
- local session authentication, HttpOnly/SameSite cookies, CSRF and CSP/security headers;
- capability RBAC baseline for viewer/operator/admin; unlock requires `door:unlock` plus recent authentication;
- callback keyring/secret-reference runtime and replay admission;
- exact callback execution/node/event/action binding and human `usr_*` subject validation;
- callback allow/deny authorization audit with fail-closed audit-before-allowed-mutation behavior;
- durable medium-risk callback retry/restart dedupe and high-risk stale-resolution safety;
- callback exactly-once semantic resume at orchestration boundary;
- durable Telegram notifier frozen-recipient/per-recipient receipt/ambiguity semantics.

### Runtime remote-bind hardening — T-012 VERIFYING

The production runtime previously accepted any non-empty listen address while the server used plaintext `ListenAndServe`, even though `HardenedServerConfig` already encoded the correct remote-TLS requirement.

T-012 now makes the current runtime **loopback-only until TLS is actually served**:

- `localhost`, IPv4 loopback and `::1` remain valid;
- wildcard/unspecified, LAN and arbitrary remote hostnames fail with `ErrInsecureRemoteBind`;
- malformed listen values fail as malformed `host:port`, not as a remote-bind policy error;
- the default `127.0.0.1:8080` behavior is unchanged;
- no certificate fields or false TLS capability are introduced.

Post-push CI/security/mutation evidence is still required before this item becomes DONE.

### Remaining Stage 17 gaps

1. Secure callback semantics are not wired to an external HTTP callback transport. **F-007 / T-013**.
2. Principal types do not explicitly distinguish human user/service/system authority. **F-008 / T-014**.
3. Authorization decision auditing is callback-specific and rate limiting is IP-scoped rather than principal-aware. **F-009 / T-015**.
4. Common stale-command ETag/expected-version and HTTP `Idempotency-Key` contracts are absent. **F-011 / T-016**.
5. Browser authentication is concrete local password/session auth, not a pluggable local/OIDC/mTLS boundary. **F-010 / T-017**.

## Important production gaps outside Stage 17

- durable event/lifecycle/ADGO plan versioning and in-flight upgrade/rollback;
- crash/restore linearization around provider effects;
- explicit single-writer versus multi-process physical fencing topology;
- target-hardware latency/allocation/load budgets;
- technical required-check protection for direct `main` updates;
- reproducible release artifact/provenance/signing/checksums and restore drill.

## Dependency-aware order

1. Finish verification of T-012 remote-bind fail-closed guard.
2. T-004 durable schema/plan evolution may proceed in parallel with later HTTP work.
3. T-013/T-014/T-015/T-016 close callback transport/principal/audit/concurrency P0 residuals.
4. T-005/T-006 crash/restore and process topology/fencing.
5. T-007 authenticated Scenario API.
6. T-008/T-009/T-010 operational/release/main-guard qualification.
7. T-017 authenticator abstraction before claiming OIDC/mTLS modes.

Execution truth: `../MASTER_PLAN.md`
