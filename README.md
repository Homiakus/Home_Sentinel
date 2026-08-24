# Home Sentinel

Home Sentinel — локальная security/automation платформа на Go. Архитектура разделяет высокочастотный media/data plane и проверяемый control plane.

## Архитектурный принцип

```text
Cameras / Sensors / Home Assistant / Intercom
                    |
                    v
      media + CV + event normalization
                    |
             typed domain events
                    |
        +-----------+-----------+
        |                       |
        v                       v
   Axiom lifecycle         ADGO workflows
   devices/zones           incidents/HITL
        |                       |
        +-----------+-----------+
                    v
             Invocation Gateway
                    |
       notifications / HA / physical IO
```

- **Axiom** — lifecycle доменных объектов: камера, зона, устройство, incident state.
- **ADGO** — durable workflow: корреляция evidence, risk evaluation, уведомление, ожидание решения пользователя, recovery/reconciliation.
- **Media/CV plane не зависит от Axiom/ADGO** и передаёт в control plane только нормализованные события и ссылки на artifacts.
- Внешние эффекты выполняются только через gateway-слой с idempotency/reconciliation.
- Пользовательские автоматизации будут строиться через canonical **Scenario AST + Scenario/Safety Compiler**, а не напрямую через raw Axiom/ADGO graphs.

## Планы и статус

Единый индекс engineering plans: [`docs/PLAN_INDEX.md`](docs/PLAN_INDEX.md).

Основной production roadmap: [`docs/AXIOM_IMPLEMENTATION_PLAN.md`](docs/AXIOM_IMPLEMENTATION_PLAN.md).

Подробный roadmap удобного создания сценариев: [`docs/SCENARIO_SYSTEM_PLAN.md`](docs/SCENARIO_SYSTEM_PLAN.md).

Текущий реализованный baseline: [`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md).

Архитектурный контракт: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).
