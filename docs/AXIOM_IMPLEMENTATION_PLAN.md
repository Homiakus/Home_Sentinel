# MASTER PLAN v2 — внедрение Axiom / ADGO в Home Sentinel

Дата повторного аудита: 2026-08-24.

## 1. Цель и архитектурная граница

Home Sentinel строится как локальная Go-платформа безопасности с двумя разными контурами:

```text
MEDIA / DATA PLANE                       CONTROL PLANE
RTSP -> decode -> CV -> tracking         typed events -> correlation
        |                                           |
        +------ compact facts/artifacts ---------->+
                                                    |
                                    Axiom lifecycle / ADGO workflows
                                                    |
                                      policy + human authority
                                                    |
                                          invocation gateways
                                                    |
                                              physical world
```

Неподвижные правила архитектуры:

- Axiom/ADGO **не используются** на уровне frame/decoder/inference/tracker hot path.
- ML/VLM/LLM создают только evidence/facts и не обладают правом физического действия.
- Axiom используется для компактных lifecycle state machines.
- ADGO используется только там, где нужны durable state, wait/retry/recovery/HITL/reconciliation.
- Любой внешний write проходит через gateway с desired-state + idempotency + verify/reconcile semantics.
- Raw media не хранится в Axiom/ADGO state; только content-addressed artifact refs.
- UI/API не создают вторую FSM; read model строится из committed state/history.

Статусы:
- `[x]` реализовано в `main`;
- `[~]` частично реализовано, production gate ещё не закрыт;
- `[ ]` не реализовано.

Приоритеты:
- **P0** — блокирует безопасный production release;
- **P1** — блокирует эксплуатационную зрелость;
- **P2** — масштабирование/расширение после первой production-версии.

---

# 2. Повторный аудит полноты плана

Старый план корректно покрывал Axiom/ADGO, HITL, physical reconciliation и correlation, но не полностью покрывал жизненный цикл production-системы. В v2 добавлены отдельные обязательные блоки:

1. reproducible builds / `go.sum` / supply-chain;
2. event/schema evolution и migration in-flight executions;
3. typed application configuration и secrets lifecycle;
4. authenticated ingress + RBAC + authorization matrix;
5. key rotation и callback integration, а не только криптографический helper;
6. **global per-device admission/serialization между разными executions**;
7. clock/time semantics и тесты скачков времени;
8. durable storage backup/restore/retention/corruption handling;
9. real gateway adapters и contract/HIL tests;
10. backpressure/load shedding/disk-pressure/degraded mode;
11. metrics/SLO/runbooks/alerting;
12. chaos/crash/fault-injection matrix;
13. release/upgrade/rollback protocol;
14. privacy/media retention и audit retention;
15. API concurrency semantics: stale command rejection, ETag/version binding;
16. explicit single-node vs multi-node ownership model.

Особенно важный найденный P0-риск: `adgo.Node.ResourceKeys` сериализуют кандидатов в рамках конкретного execution scheduler snapshot, но сами по себе не являются глобальным mutex для двух независимых door/siren executions. Физические ресурсы требуют отдельного durable admission/fencing слоя.

---

# 3. Реализованный фундамент

## Этап 0 — Architecture boundary [x]

- [x] ADR `ADR-0001-axiom-adgo-boundary.md`.
- [x] dependency direction и запрет ADGO в media hot path.
- [x] artifact-reference boundary.
- [x] deterministic authority boundary.

## Этап 1 — Go/domain baseline [x]

- [x] Go 1.26 module.
- [x] Axiom pinned на конкретную revision/pseudo-version.
- [x] Makefile: fmt/vet/test/race.
- [x] event envelope: ID/kind/source/occurredAt/receivedAt/correlation/artifacts.
- [x] clock-skew validation.
- [x] incident trigger/risk/status/decision contracts.
- [x] deterministic execution ID.
- [x] content-addressed artifact ref.
- [x] базовые table/unit tests.
- [ ] **P0:** commit `go.sum` и enforce module hygiene — перенесено в Stage 14.
- [ ] **P0:** payload byte-size/schema limits — Stage 15/17.

## Этап 2 — Gateway contracts [~]

- [x] Notifier.
- [x] DoorController desired state.
- [x] SirenController desired state.
- [x] ArtifactStore.
- [x] CameraRecoveryController.
- [x] Applied/AlreadyApplied/Ambiguous semantics.
- [x] provider operation ID.
- [x] fake idempotency tests.
- [ ] **P0:** `Operation.ExecutionID` validation + common operation metadata contract.
- [ ] **P0:** global resource admission wrapper for physical writes — Stage 20.
- [ ] Presence/Evidence provider interfaces добавлять только вместе с реальным consumer, без speculative interfaces.

## Этап 3 — Camera lifecycle на Axiom [~]

- [x] connecting/online/degraded/offline/disabled.
- [x] Connected/StreamDegraded/StreamFailed/Recovered/Disable/Enable.
- [x] disabled + late recovery invariant.
- [x] Axiom скрыт за camera service adapter.
- [x] compile/happy/late-recovery regression tests.
- [ ] **P1:** replay-equivalence test из persisted history/state.
- [ ] **P1:** model/schema version policy — Stage 15.

## Этап 4 — Incident ADGO workflow [x]

- [x] NormalizeTrigger -> CorrelateEvidence -> AssessRisk.
- [x] NotifyOwner external effect.
- [x] durable owner wait.
- [x] ArchiveIncident.
- [x] StartOrLoad dedup.
- [x] duplicate signal safety.
- [x] artifacts remain references.

## Этап 5 — Production ADGO/Pebble [~]

- [x] `adgo.OpenProduction`.
- [x] Pebble default, memory test backend.
- [x] bounded worker concurrency.
- [x] durable reopen test.
- [x] resilient coordinator/worker semantics inherited from ADGO.
- [ ] **P0:** explicit stable worker identity policy across restart/host identity.
- [ ] **P0:** crash matrix at every persist/effect boundary — Stage 23.
- [ ] **P1:** graceful drain integration in application process.
- [ ] **P1:** lease-expiration/fencing tests at Home Sentinel level.

## Этап 6 — Risk policy v2 [~]

- [x] pure deterministic scorer.
- [x] versioned policy identifier.
- [x] explainable contributions.
- [x] alarm/identity/entry/dwell/cross-camera/confidence/evidence features.
- [x] low/medium/high/critical routing.
- [x] threshold and deterministic tests.
- [ ] **P1:** fuzz NaN/Inf/extreme numeric inputs.
- [ ] **P1:** policy-config signature/version migration and golden vectors.

## Этап 7 — Human in the loop [~]

- [x] durable approve/edit/reject/retry/confirm/abort semantics.
- [x] actor/reason/payload persisted by ADGO.
- [x] restart-safe workflow state.
- [x] stale completed-node decisions rejected by ADGO runtime semantics.
- [ ] **P0:** authenticated callback ingress wired to workflow services — Stage 17.
- [ ] **P0:** authorization matrix per decision/action — Stage 17.
- [ ] **P1:** explicit human-decision deadline/escalation policy.

## Этап 8 — Physical action safety [~]

### Door
- [x] desired lock state, no toggle.
- [x] unlock human gate.
- [x] read-before-write.
- [x] verify-after-write.
- [x] stable idempotency key.
- [x] ambiguous effect -> durable reconciliation.

### Siren
- [x] desired enabled/disabled state.
- [x] bounded activation timer.
- [x] manual stop through cancellation.
- [x] ensure-disabled compensation.

Remaining:
- [ ] **P0:** cross-execution resource serialization — Stage 20.
- [ ] **P0:** crash after provider accept / before ADGO commit fault injection.
- [ ] **P0:** gateway context/deadline compliance contract.
- [ ] **P1:** real-device reconciliation/HIL tests.

## Этап 9 — Multi-sensor correlation [~]

- [x] bounded temporal window.
- [x] bounded seen-ID cache.
- [x] duplicate suppression.
- [x] out-of-order acceptance.
- [x] late-event rejection.
- [x] cross-camera aggregation.
- [x] concurrency tests.
- [ ] **P1:** explicit source clock-quality/skew fact.
- [ ] **P1:** incident merge policy for two independently-created candidates.
- [ ] **P1:** property/fuzz tests over shuffled event streams.

## Этап 10 — Camera recovery [~]

- [x] stateful recovery only; heartbeat remains cheap scheduler code.
- [x] network probe -> stream probe -> reconnect -> verify.
- [x] bounded retry.
- [x] durable operator escalation.
- [x] one explicit final retry instead of unbounded graph cycle.
- [ ] **P0:** global per-camera recovery admission.
- [ ] **P1:** process-crash/lease-expiry tests during reconnect.

## Этап 11 — Operator read model / diagnostics [~]

- [x] timeline from durable history.
- [x] risk assessment DTO.
- [x] ADGO Explain projection.
- [x] Diagnostics projection.
- [x] credential-like key redaction.
- [ ] **P0:** pagination/size bounds for large histories.
- [ ] **P0:** HTTP API auth/authorization — Stage 17.
- [ ] **P1:** structured logger adapter.
- [ ] **P1:** Prometheus/OpenTelemetry exporters — Stage 22.

## Этап 12 — Security primitives [~]

- [x] threat model.
- [x] HMAC-SHA256 callback tokens.
- [x] constant-time MAC compare.
- [x] key ID bound into signed message.
- [x] issuedAt/expiresAt/max TTL/clock skew.
- [x] maximum token length.
- [x] multi-key verification + active signing key for overlap rotation.
- [x] bounded edge replay guard.
- [x] ADGO SeenEvents remains durable semantic dedup boundary.
- [ ] **P0:** wire keyring/replay/authz into ingress.
- [ ] **P0:** secret source + key rotation operational workflow — Stage 16.
- [ ] **P0:** RBAC and physical-action authorization — Stage 17.
- [ ] **P1:** callback parser fuzzing.
- [ ] **P1:** audit tamper-evidence/retention policy.

## Этап 13 — Performance / current CI [~]

- [x] correlation benchmark.
- [x] risk benchmark.
- [x] callback verification benchmark.
- [x] CI: module download/verify, format, vet, unit, race, benchmark smoke.
- [x] CI cache temporarily disabled while repository has no `go.sum`.
- [ ] **P0:** target-hardware baseline and numeric budgets.
- [ ] **P0:** control-path/Pebble benchmarks.
- [ ] **P0:** supply-chain gates — Stage 14.

---

# 4. Новые обязательные production stages

## Этап 14 — Reproducible builds и supply chain [P0]

### Module lock
- [ ] Сгенерировать и commit `go.sum` из clean Go 1.26 environment.
- [ ] `go mod tidy` должен оставлять working tree чистым.
- [ ] вернуть `setup-go cache: true` только после появления lock.
- [ ] CI: `go mod verify` + `git diff --exit-code go.mod go.sum` после tidy.

### Dependency security
- [ ] `govulncheck ./...`.
- [ ] SBOM (CycloneDX или SPDX) для release artifact.
- [ ] dependency/license inventory.
- [ ] policy для обновления pinned Axiom revision.
- [ ] compatibility test против выбранного Axiom commit.
- [ ] Dependabot/Renovate policy без автоматического major upgrade control plane.

### GitHub Actions supply chain
- [ ] pin critical actions на commit SHA или formal update policy.
- [ ] минимальные workflow permissions.
- [ ] build provenance/release checksum.

Acceptance: clean clone воспроизводимо собирается без изменения module metadata.

---

## Этап 15 — Schema evolution, plan catalog и migrations [P0]

### Event schemas
- [ ] versioned kind convention (`*.v1`, `*.v2`).
- [ ] registry/decoder по event kind.
- [ ] maximum payload bytes до JSON decode.
- [ ] unknown-field policy для внешнего ingress.
- [ ] backward compatibility golden fixtures.

### Axiom lifecycle schemas
- [ ] явно версионировать Camera definition/state.
- [ ] migration rule для persisted state при изменении fields/defaults.

### ADGO plans
- [ ] plan catalog: digest -> immutable plan/registry version.
- [ ] новые executions стартуют на active plan.
- [ ] старые executions продолжаются на pinned plan.
- [ ] запрет удаления plan implementation, пока существуют non-terminal executions.
- [ ] explicit migration только через tested `PlanDelta`/migration path, не silent replacement.

### Persistent store
- [ ] application storage schema version marker.
- [ ] pre-migration backup.
- [ ] forward migration + rollback/restore instructions.

Acceptance: upgrade во время waiting human incident не ломает его продолжение.

---

## Этап 16 — Typed configuration, secrets и key lifecycle [P0]

- [ ] единый `internal/config` с typed structs.
- [ ] defaults -> config file -> env/secret refs; документированный precedence.
- [ ] startup validation до открытия physical gateways.
- [ ] fail-closed для malformed security config.
- [ ] запрет secret values в logs/errors/Execution.Data.
- [ ] secret references вместо secret strings в durable config/state.
- [ ] callback keyring загружается из secret source.
- [ ] active key + overlap verify keys + retirement timestamp.
- [ ] rotation runbook и тест old/new overlap.
- [ ] TLS/camera/HA credential rotation hooks.
- [ ] sanitized config dump для diagnostics.

---

## Этап 17 — Authenticated ingress, API contracts и RBAC [P0]

### HTTP server safety
- [ ] `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`.
- [ ] max request/body size до decode.
- [ ] JSON strict decode для command endpoints.
- [ ] request ID + actor identity.
- [ ] per-IP/per-principal rate limits для callbacks/login/action commands.
- [ ] TLS termination contract; production запрещает plaintext remote bind.

### Authentication
- [ ] pluggable local/OIDC/mTLS authenticator interface.
- [ ] callback token keyring verification.
- [ ] replay guard перед workflow mutation.

### Authorization
Роли минимум:
- viewer — read-only timeline/health;
- operator — acknowledge/reject, safe recovery actions;
- security-admin — unlock/siren/high-risk policy actions;
- system — machine-generated normalized events без human authority.

- [ ] permission matrix `resource + action`.
- [ ] unlock никогда не разрешается inference/system principal.
- [ ] authorization decision пишется в audit trail.
- [ ] callback claims bind execution/node/event; server сверяет actual waiting node до mutation.

### Concurrency API
- [ ] command принимает expected execution version/ETag для stale UI protection.
- [ ] `409 Conflict` на stale operator decision.
- [ ] idempotency key на POST command endpoints.

Browser UI:
- [ ] SameSite cookies/CSRF protection если используется cookie auth.
- [ ] CSP/security headers.

---

## Этап 18 — Persistence, backup, retention и corruption handling [P0/P1]

- [ ] отдельные durable roots для ADGO state, artifacts и audit/config metadata.
- [ ] filesystem permission check при старте.
- [ ] disk-space low/high watermarks.
- [ ] ENOSPC behavior: fail closed для новых physical workflows, сохранить ability to disable siren/lock safe state.
- [ ] periodic Pebble checkpoint/backup strategy.
- [ ] atomic backup manifest с application/Axiom/plan versions.
- [ ] restore tool/runbook.
- [ ] automated restore drill test.
- [ ] retention policy для terminal executions/history.
- [ ] artifact retention + reference-aware GC.
- [ ] audit retention отдельно от media retention.
- [ ] corruption detection и quarantine; никаких silent resets.

---

## Этап 19 — Time, ordering и deterministic clock semantics [P0/P1]

- [x] OccurredAt и ReceivedAt разделены.
- [x] базовый max clock skew guard.
- [x] callback issuedAt/expiresAt.
- [ ] injectable `Clock` для policy/correlation/security tests.
- [ ] UTC everywhere at persistence/API boundaries.
- [ ] wall-clock jump tests: +1h/-1h.
- [ ] NTP/backward jump не должен продлевать physical action бесконечно.
- [ ] durable timer semantics documented relative to ADGO runtime clock.
- [ ] source clock-quality/skew metric.
- [ ] deterministic ordering tie-break: timestamp -> source/event ID.

---

## Этап 20 — Global physical resource admission и fencing [P0]

Цель: два независимых executions не должны одновременно управлять одним actuator/device.

- [ ] dynamic resource key: `door:<id>`, `siren:<id>`, `camera-recovery:<id>`.
- [ ] durable `AdmissionController` MaxConcurrent=1 на physical resource.
- [ ] admission lease TTL > handler timeout, heartbeat для long-running operations.
- [ ] release в normal/error paths.
- [ ] expired admission lease recovery.
- [ ] gateway обязан уважать context cancellation/deadline.
- [ ] где provider поддерживает fencing/version — передавать fence token.
- [ ] где fencing невозможно — desired-state + post-verify + reconciliation остаются обязательными.

Tests:
- [ ] two executions / same door -> никогда не выполняют write одновременно.
- [ ] two executions / different doors -> могут выполняться параллельно.
- [ ] lease holder crash -> новый holder допускается только после expiry.
- [ ] stale holder не должен безопасно изменить итог без subsequent verify/reconcile.

---

## Этап 21 — Real gateway adapters и integration contracts [P0/P1]

Не добавлять абстракции без consumer. Реализовывать вертикальными slices.

### Camera/RTSP
- [ ] adapter connection/probe/reconnect.
- [ ] credential isolation.
- [ ] bounded connect/read deadlines.

### Home Assistant / automation
- [ ] explicit service allowlist.
- [ ] entity allowlist.
- [ ] no arbitrary service/template execution from model-generated data.
- [ ] desired-state read/verify.

### Notification
- [ ] provider idempotency mapping.
- [ ] timeout/rate-limit classification.
- [ ] callback deep-link tokens generated security layer.

### Physical IO
- [ ] relay/lock/siren adapters expose state readback where hardware permits.
- [ ] safe defaults on disconnect/reboot.

### Contract tests
- [ ] fake + recorded integration fixtures.
- [ ] provider timeout/429/5xx/malformed response.
- [ ] hardware-in-loop suite tagged separately from unit CI.

---

## Этап 22 — Observability, SLO и runbooks [P1]

### Structured logs
- [ ] execution_id / node_id / task_id / worker_id / resource_id.
- [ ] actor/request_id для operator actions.
- [ ] centralized redaction hook.
- [ ] no high-cardinality raw payloads.

### Metrics
- [ ] event ingest/reject/late/duplicate counts.
- [ ] correlation window/group pressure.
- [ ] incident start/complete/fail/waiting-human.
- [ ] queue age/activity latency/retries/lease recoveries.
- [ ] ambiguous physical effects/reconciliation count.
- [ ] admission denied/lease expiry.
- [ ] gateway latency/error classification.
- [ ] Pebble size/disk free/backup age.

### Traces
- [ ] correlation ID -> incident execution -> gateway operation linkage.
- [ ] sampling policy, не trace every frame.

### SLO/alerts
- [ ] notification latency SLO.
- [ ] physical action verification SLO.
- [ ] maximum waiting-unreconciled incident age.
- [ ] camera offline recovery SLO.
- [ ] disk/backup freshness alerts.
- [ ] runbooks для каждого page-worthy alert.

---

## Этап 23 — Fault injection, chaos и exhaustive test matrix [P0/P1]

### Deterministic/property/fuzz
- [ ] fuzz event/callback/config decoders.
- [ ] risk NaN/Inf/extreme fuzz.
- [ ] shuffled/duplicated correlation property tests.
- [ ] state-machine property tests для camera/door/siren.

### Crash points
Для каждого external effect проверить crash:
1. до durable enqueue;
2. после enqueue, до claim;
3. после claim, до provider call;
4. provider accepted, response lost;
5. response received, до local completion commit;
6. completion committed, response caller lost.

- [ ] Door matrix.
- [ ] Siren matrix.
- [ ] Notification matrix.
- [ ] Camera reconnect matrix.

### Infrastructure failures
- [ ] worker lease expires.
- [ ] process SIGKILL/restart.
- [ ] disk full.
- [ ] corrupted state file/db.
- [ ] network partition.
- [ ] Home Assistant unavailable.
- [ ] clock jumps.
- [ ] event flood/backpressure.
- [ ] 24h/72h soak test.

---

## Этап 24 — Backpressure, degraded mode и process topology [P0/P1]

### Explicit topology
- [ ] v1 объявляет supported topology: single control-plane process или multi-process.
- [ ] если single-process — startup lock запрещает второй writer на том же durable root.
- [ ] если multi-process — только через durable ownership/admission primitives; никаких process-local mutex assumptions.

### Backpressure
- [ ] bounded ingest queues.
- [ ] drop/coalesce policy для low-value repeated observations.
- [ ] critical sensor events не silently drop.
- [ ] admission limits для incident creation.
- [ ] overload state виден operator/metrics.

### Degraded safety
- [ ] при storage read-only/full запрещать новые risky actions.
- [ ] emergency safe-disable сирены должен оставаться возможным.
- [ ] camera/CV outage не приводит к auto-unlock.
- [ ] notifier outage сохраняет durable incident и retry/escalation state.

---

## Этап 25 — Privacy / media / audit lifecycle [P1]

- [ ] классификация данных: secret / identity / media / telemetry / audit.
- [ ] configurable media retention.
- [ ] минимизация identity facts в durable state.
- [ ] artifact access authorization.
- [ ] encryption-at-rest deployment requirement + key ownership.
- [ ] redacted export/support bundle.
- [ ] audit actor/action/reason без secret payload.
- [ ] delete/retention semantics документированы отдельно для media и immutable security audit.

---

## Этап 26 — Performance budgets на target hardware [P0 release gate]

Измерять отдельно hot/control planes.

Hot/control benchmarks:
- [x] risk scorer.
- [x] correlator ingest.
- [x] callback verification.
- [ ] Incident StartOrLoad p50/p95/p99.
- [ ] ADGO advance/claim/commit.
- [ ] Pebble reopen and 10k/100k execution catalog.
- [ ] history growth bytes/incident.
- [ ] concurrent incident throughput.
- [ ] resource admission latency.
- [ ] reconciliation latency.

Для target hardware записать:
- CPU/RAM/storage/OS/Go version;
- ns/op, B/op, allocs/op;
- p50/p95/p99;
- steady-state RSS;
- disk growth/day.

CI numeric budget добавлять только после измеренного baseline; никаких выдуманных порогов.

---

## Этап 27 — Release, upgrade и rollback [P0]

- [ ] semantic version Home Sentinel.
- [ ] embedded build metadata: git SHA, build time, Axiom revision, schema/plan versions.
- [ ] release notes перечисляют storage/schema/plan changes.
- [ ] pre-upgrade backup verification.
- [ ] migration dry-run.
- [ ] startup compatibility check before opening gateways.
- [ ] graceful drain before planned shutdown.
- [ ] rollback procedure: binary + config + state compatibility/restore.
- [ ] checksum/SBOM release artifacts.
- [ ] canary/acceptance checklist на target host.

Final release gate:
- [ ] clean clone builds reproducibly.
- [ ] CI fully green.
- [ ] race/fuzz/crash matrix green.
- [ ] backup restore drill green.
- [ ] no raw media/secrets in durable workflow state.
- [ ] no physical write outside gateway packages.
- [ ] global same-device serialization verified.
- [ ] high-risk RBAC/HITL verified.
- [ ] stale/replayed callback rejected.
- [ ] old pinned workflow survives application upgrade.
- [ ] target hardware performance budgets green.
- [ ] operator runbooks exist for degraded/storage/reconciliation states.

---

# 5. Реальный порядок дальнейшего внедрения

Не идти просто по номеру. Приоритет после текущего `main`:

1. **Stage 20 — global physical resource admission**: закрыть найденный P0 race между independent executions.
2. **Stage 14 — module lock/supply chain**: получить `go.sum` из clean environment, затем module hygiene/govuln.
3. **Stage 16 + 17 — config/secrets + authenticated ingress/RBAC**.
4. **Stage 15 — plan/schema catalog/migrations**, пока API ещё не стал публичным.
5. **Stage 18 — backup/restore/retention** до накопления production state.
6. **Stage 23/24 — crash matrix, backpressure, degraded mode**.
7. **Stage 21 — реальные adapters вертикальными slices**.
8. **Stage 22/26 — metrics/SLO + measured performance budgets**.
9. **Stage 27 — release/upgrade/rollback gate**.

Каждый пункт реализуется атомарно: contract -> implementation -> failure tests -> docs/status -> commit в `main`.

---

# 6. Definition of Done для любого нового workflow

Workflow не считается production-ready, пока одновременно не выполнено:

1. deterministic inputs и explicit schema/version;
2. immutable compiled plan/version/digest;
3. bounded retry/time/budget/concurrency;
4. idempotency для external effects;
5. desired-state semantics для physical effects;
6. ambiguous-effect reconciliation;
7. compensation либо документированный irreversible policy;
8. resource ownership/admission;
9. restart/replay/crash tests;
10. human authorization для high-risk;
11. read model/Explain/Diagnostics;
12. no secrets/raw media in state/history;
13. metrics + operator-visible failure state;
14. migration compatibility policy;
15. documented timeout/cancellation semantics.

Этот DoD применяется к Incident, Door, Siren, Recovery и всем будущим workflow одинаково.
