# Implementation status

## Status semantics

This is an observed snapshot, not an independent source of truth. `MASTER_PLAN.md` owns execution order/status. Detailed intent remains in `docs/AXIOM_IMPLEMENTATION_PLAN.md` and `docs/SCENARIO_SYSTEM_PLAN.md`; completion requires executable evidence.

Before closing implementation work, run the engineering-loop reconciliation and the applicable CI/security/mutation gates.

## Current main baseline

Implemented/proven foundations include:

- Stages 0–13 core architecture/domain/gateway/Axiom/ADGO/risk/HITL/physical-safety/correlation/recovery/read-model/security primitives and current CI baseline, with residual clauses tracked in the master plan rather than implied complete.
- Stage 14b supply-chain baseline: committed module lock, module hygiene, reviewed immutable Action allowlist, pinned scanners, `govulncheck`, Trivy, CycloneDX module SBOM evidence and Dependabot. Release provenance/signing/reproducibility remains release qualification work.
- Stage 16a typed fail-closed callback/security configuration and secret-reference loading baseline.
- Stage 20a same-store cross-execution physical resource reservation for Door/Siren/Camera Recovery; multi-process fencing remains open.
- Stages 28–34 headless Scenario foundation: canonical model, capability registry, typed/temporal semantics, compiler, Safety Compiler, static conflicts, immutable catalog/dependency index and simulator/replay.
- Engineering loop: roadmap reconciliation, Work Packet validation, risk/gate planning, multidimensional edge-suite generation, mutation evidence validation and executable supply-chain self-check.

## Stage 17 — reconciled status

Stage 17 remains **PARTIAL**, but the previous summary was stale. See [`STAGE17_RECONCILIATION.md`](STAGE17_RECONCILIATION.md) for the clause-by-clause evidence matrix.

### Verified now

- HTTP server read-header/read/write/idle timeouts.
- bounded + strict JSON command decoding through the shared decoder.
- request IDs.
- local session authentication baseline.
- SameSite/HttpOnly cookie behavior, CSRF middleware and CSP/security headers.
- capability RBAC baseline for viewer/operator/admin; unlock requires the dedicated capability and recent authentication.
- callback keyring/secret-reference runtime and replay guard.
- exact callback execution/node/event/action binding.
- callback human-subject validation against the current persisted user and capability grant.
- callback authorization allow/deny audit; allowed mutation fails closed if audit persistence fails.
- durable medium-risk callback redelivery/restart dedupe and high-risk stale-resolution safety.
- callback exactly-once semantic resume at the orchestration boundary.
- durable Telegram notifier semantics: frozen recipients, per-recipient receipts, safe retry, ambiguous crash-window handling and no blind resend.

### Still open/partial

1. Production runtime currently does not enforce the hardened non-loopback/TLS rule; plaintext remote bind is possible through `Config`. **F-006 / T-012**.
2. Secure callback acceptance/orchestration exists but is not wired to an external HTTP callback transport. **F-007 / T-013**.
3. Principal types do not explicitly distinguish human user/service/system authority; current persisted roles are viewer/operator/admin. **F-008 / T-014**.
4. Authorization decision auditing is complete on callback path, not a generic HTTP middleware contract; rate limiting is IP-scoped rather than per-principal. **F-009 / T-015**.
5. Common stale-command ETag/expected-version and HTTP idempotency-key contracts are absent. **F-011 / T-016**.
6. Browser authentication is a concrete local password/session implementation, not a pluggable local/OIDC/mTLS boundary. **F-010 / T-017**.

Stage 35 authenticated Scenario API remains blocked by the relevant Stage 17 authority/concurrency prerequisites.

## Scenario authoring audit

Stages 28–34 are the implemented headless foundation. Remaining product track:

- 35 — authenticated Scenario API;
- 36 — Simple Builder;
- 37 — Advanced Flow Editor;
- 38 — Templates/Subflows;
- 39 — LLM authoring through structured AST only;
- 40 — Live Trace/Explain;
- 41 — mobile/adaptive authoring;
- 42 — scenario quality/security/release gates.

Scenario UI/AI cannot bypass catalog/compiler/Safety Compiler/RBAC/gateway/resource-ownership boundaries.

## Important open production findings

- Multi-process physical-write fencing/topology is not proven; same-store reservation is not a distributed mutex.
- Durable plan/schema evolution and in-flight execution migration/restore remain P0.
- Crash/restore linearization around provider effects remains P0.
- Runtime remote plaintext bind must fail closed before an external callback endpoint is exposed.
- Branch `main` currently lacks a technical required-check guard.
- Target-hardware latency/allocation/load budgets remain unqualified.
- Release provenance, reproducible artifact, signing/checksums and rollback drill remain release work.

## Next dependency-aware order

1. T-012 — reject unsafe remote runtime bind until TLS runtime support exists.
2. T-004 — durable schema/plan catalog and compatible upgrade path; may proceed independently of later HTTP work.
3. T-013/T-014/T-015/T-016 — close the Stage 17 external transport/principal/audit/concurrency P0 residuals in dependency order.
4. T-005/T-006 — crash/restore matrix and explicit process topology/fencing.
5. T-007 / Scenario Stage 35 — authenticated Scenario API after authority/concurrency prerequisites.
6. T-008/T-009/T-010 — hardware budgets, release qualification and verified-main guard.
7. T-017 — pluggable authenticator boundary before claiming non-local authentication modes.

Index: `docs/PLAN_INDEX.md`  
Engineering protocol: `docs/engineering/ENGINEERING_LOOP.md`  
Production intent: `docs/AXIOM_IMPLEMENTATION_PLAN.md`  
Scenario intent: `docs/SCENARIO_SYSTEM_PLAN.md`  
Execution truth: `../MASTER_PLAN.md`
