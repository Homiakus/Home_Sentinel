# Поэтапный план внедрения Axiom / ADGO в Home Sentinel

## Цель

Построить Home Sentinel как локальную Go-платформу безопасности, где perception работает с высокой частотой и минимальной задержкой, а критичные решения выполняются через deterministic/durable control plane.

Обозначения статуса:
- `[x]` выполнено;
- `[~]` начато;
- `[ ]` не начато.

---

## Этап 0. Архитектурная фиксация

### 0.1 Граница ответственности
- [x] Зафиксировать запрет на использование Axiom/ADGO в frame/RTSP/decoder/tracker hot path.
- [x] Зафиксировать Axiom для lifecycle и ADGO для durable graph workflows.
- [x] Зафиксировать gateway boundary для внешних side effects.
- [x] Зафиксировать artifact references вместо binary payload в execution state.
- [x] Зафиксировать deterministic authority: ML/LLM/VLM возвращают facts, но не выполняют физические команды.

### 0.2 Acceptance
- [x] ADR существует в `docs/ADR-0001-axiom-adgo-boundary.md`.
- [x] Направление зависимостей описано в `docs/ARCHITECTURE.md`.

---

## Этап 1. Go baseline и domain contracts

### 1.1 Module baseline
- [ ] Создать `go.mod` на Go 1.26.
- [ ] Зафиксировать конкретную revision Axiom вместо плавающей ветки.
- [ ] Добавить Makefile с `fmt`, `vet`, `test`, `test-race`.

### 1.2 Domain event envelope
Создать `internal/domain/event`:
- [ ] `ID` — стабильный idempotency/dedup key.
- [ ] `Kind` — versionable event kind.
- [ ] `SourceID`/`SourceType`.
- [ ] `OccurredAt` и `ReceivedAt` раздельно.
- [ ] `CorrelationID`.
- [ ] typed metadata/artifact refs.
- [ ] validation без сетевых зависимостей.

### 1.3 Incident domain
Создать `internal/domain/incident`:
- [ ] Trigger.
- [ ] Severity/Risk.
- [ ] Evidence.
- [ ] Decision.
- [ ] Status.
- [ ] deterministic incident/execution ID builder.

### 1.4 Artifact reference
- [ ] Content-addressed reference: URI + digest + size + MIME.
- [ ] Validation digest/URI.
- [ ] Запрет raw bytes в domain workflow contracts.

### 1.5 Tests
- [ ] table tests event validation.
- [ ] deterministic ID property.
- [ ] timestamp invariants.
- [ ] invalid artifact rejection.

---

## Этап 2. Gateway contracts

### 2.1 Interfaces
В `internal/gateway` определить маленькие интерфейсы:
- [ ] `Notifier`.
- [ ] `DoorController`.
- [ ] `SirenController`.
- [ ] `ArtifactStore`.
- [ ] `EvidenceProvider`.
- [ ] `PresenceProvider`.

### 2.2 Idempotency contract
Каждая write-operation получает `Operation`:
- [ ] `IdempotencyKey`.
- [ ] `ExecutionID`.
- [ ] `Deadline` через context.
- [ ] desired state вместо toggle semantics.

Например `SetLockState(locked=true)` допустим, `ToggleLock()` запрещён.

### 2.3 Result semantics
- [ ] `Applied`.
- [ ] `AlreadyApplied`.
- [ ] `Unknown/Ambiguous`.
- [ ] provider operation ID для reconciliation.

### 2.4 Tests
- [ ] fake gateway deduplicates same key.
- [ ] retry не дублирует physical semantic effect.
- [ ] ambiguous result переводится в reconciliation path.

---

## Этап 3. Axiom lifecycle — Camera

### 3.1 State
- [ ] `connecting`.
- [ ] `online`.
- [ ] `degraded`.
- [ ] `offline`.
- [ ] `disabled`.

### 3.2 Events
- [ ] Connected.
- [ ] StreamDegraded.
- [ ] StreamFailed.
- [ ] Recovered.
- [ ] DisableRequested.
- [ ] EnableRequested.

### 3.3 Invariants
- [ ] disabled camera не может считаться online.
- [ ] failure/recovery events не должны произвольно включать disabled camera.
- [ ] state transitions должны быть воспроизводимы через history/replay.

### 3.4 Adapter
- [ ] Domain остаётся Axiom-free.
- [ ] `orchestration/lifecycle/camera` строит `model.Definition`.
- [ ] API сервиса скрывает `axiom.Engine` от вызывающего кода.

### 3.5 Tests
- [ ] compile definition.
- [ ] happy transition sequence.
- [ ] disable + late recovery не включает camera.
- [ ] replay yields same state.

---

## Этап 4. ADGO Incident v1

### 4.1 Initial facts
- [ ] event_id.
- [ ] source_id.
- [ ] trigger_kind.
- [ ] occurred_at.
- [ ] confidence.
- [ ] artifact refs.

### 4.2 Graph v1

```text
NormalizeTrigger
      -> CorrelateEvidence
      -> AssessRisk
      -> NotifyOwner
      -> AwaitOwnerResponse
      -> ArchiveIncident
```

### 4.3 Activities
- [ ] `NormalizeTrigger`: pure validation/normalization.
- [ ] `CorrelateEvidence`: собирает facts/artifact refs, но не bytes.
- [ ] `AssessRisk`: deterministic scoring v1.
- [ ] `NotifyOwner`: external effect, bounded retry, timeout, idempotency.
- [ ] `ArchiveIncident`: сохраняет итоговую summary/reference.

### 4.4 Durable wait
- [ ] `AwaitOwnerResponse` ждёт event `incident.owner.response`.
- [ ] event должен быть dedup-safe.
- [ ] stale/duplicate response не вызывает повторный side effect.

### 4.5 Tests
- [ ] plan compile.
- [ ] workflow доходит до waiting.
- [ ] signal продолжает workflow.
- [ ] duplicate workflow start через `StartOrLoad` не создаёт новый execution.
- [ ] duplicate signal безопасен.

---

## Этап 5. Production ADGO runtime

### 5.1 Wiring
- [ ] `adgo.OpenProduction`.
- [ ] Pebble по умолчанию.
- [ ] memory backend только test/dev.
- [ ] конфиг lease/poll/coordinator intervals.

### 5.2 Workers
- [ ] стабильные worker IDs.
- [ ] bounded concurrency.
- [ ] graceful drain.
- [ ] cancellation через root context.

### 5.3 Recovery
- [ ] restart между TaskPending/TaskRunning.
- [ ] expired lease recovery.
- [ ] stale worker fencing.
- [ ] recovery quarantine.

### 5.4 Tests
- [ ] reopen durable store.
- [ ] crash simulation before/after effect.
- [ ] worker lease expiration.

---

## Этап 6. Risk policy v2

### 6.1 Features
- [ ] alarm mode.
- [ ] known/unknown identity.
- [ ] presence.
- [ ] door/window state.
- [ ] dwell time.
- [ ] cross-camera continuity.
- [ ] time profile.
- [ ] detector confidence/quality.

### 6.2 Deterministic scoring
- [ ] отдельный pure package.
- [ ] versioned coefficients.
- [ ] no network calls.
- [ ] explainable contribution breakdown.

### 6.3 Gate
- [ ] low -> archive/log.
- [ ] medium -> notify.
- [ ] high -> human approval + escalation.
- [ ] critical -> policy-defined local action, но только через safety gate.

### 6.4 Tests
- [ ] boundary tests на thresholds.
- [ ] fuzz numeric inputs.
- [ ] deterministic repeatability.

---

## Этап 7. Human-in-the-loop

- [ ] durable approve/edit/reject/retry/abort.
- [ ] actor/reason/audit payload.
- [ ] timeout/escalation.
- [ ] stale callback rejection.
- [ ] operator UI читает ADGO status/history, а не держит собственную теневую FSM.

Tests:
- [ ] approval after restart.
- [ ] duplicate callback.
- [ ] stale callback.
- [ ] timeout path.

---

## Этап 8. Physical actions и reconciliation

### 8.1 Door/lock
- [ ] desired-state commands.
- [ ] read-before-write.
- [ ] verify-after-write.
- [ ] stable idempotency key.
- [ ] ambiguous effect classification.

### 8.2 Siren
- [ ] set enabled/disabled, никаких toggle.
- [ ] maximum activation duration safety timer.
- [ ] manual override.

### 8.3 Compensation
- [ ] high-risk reversible actions имеют compensation либо явный irreversible marker/policy.

### 8.4 Tests
- [ ] process crash after provider accepted effect, before local commit.
- [ ] reconciliation recognizes already-applied desired state.

---

## Этап 9. Multi-sensor correlation

- [ ] временные окна вне workflow hot path.
- [ ] track/person candidates агрегируются в compact facts.
- [ ] correlation IDs стабильны.
- [ ] incident merger не создаёт duplicate executions.
- [ ] bounded evidence window.

Tests:
- [ ] out-of-order events.
- [ ] duplicate events.
- [ ] late events.
- [ ] camera clock skew.

---

## Этап 10. Health/recovery workflows

Workflow camera health:
```text
Offline -> ProbeNetwork -> ProbeRTSP -> Reconnect -> Verify -> Escalate
```

- [ ] простой heartbeat остаётся обычным scheduler/event code.
- [ ] ADGO включается только для stateful recovery sequence.
- [ ] retry budget ограничен.
- [ ] device-specific resource key исключает конфликтующие recovery actions.

---

## Этап 11. Observability

- [ ] structured logs с execution_id/node/task/worker.
- [ ] ADGO history API наружу через read-only service.
- [ ] Explain endpoint.
- [ ] incident timeline.
- [ ] metrics: queue age, activity latency, retries, recoveries, ambiguous effects, waiting human.
- [ ] health diagnostics.

Запрещено логировать secrets и raw sensitive media payload.

---

## Этап 12. Security hardening

- [ ] threat model.
- [ ] least-privilege gateway credentials.
- [ ] secrets только env/secret manager, не Execution.Data.
- [ ] signed/authenticated callbacks.
- [ ] replay protection.
- [ ] authorization matrix для physical actions.
- [ ] audit retention.
- [ ] fuzz parsers/event envelope.

---

## Этап 13. Performance and production gates

### Performance
- [ ] benchmark event normalization.
- [ ] benchmark risk scorer.
- [ ] benchmark workflow start/advance.
- [ ] benchmark Pebble state growth.
- [ ] allocation budgets для hot packages.

### CI gates
- [ ] `go test ./...`.
- [ ] `go test -race ./...`.
- [ ] `go vet ./...`.
- [ ] format check.
- [ ] vulnerability scan после появления lockfile/dependency policy.

### Release gate
- [ ] restart/crash matrix зелёная.
- [ ] idempotency contract tests зелёные.
- [ ] high-risk action policy reviewed.
- [ ] no raw media in durable state.
- [ ] plan versioning documented.

---

# Порядок реализации

Строгое правило: новый этап не должен протаскивать Axiom внутрь нижележащих слоёв. Текущий порядок:

1. domain + gateway contracts;
2. Camera Axiom lifecycle;
3. Incident ADGO v1;
4. production runtime/Pebble;
5. risk policy;
6. HITL;
7. physical effect reconciliation;
8. correlation/recovery;
9. observability/security/performance.

После каждого этапа статус в этом документе обновляется отдельным commit вместе с соответствующими тестами.
