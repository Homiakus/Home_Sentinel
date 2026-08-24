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

## Статус

Репозиторий стартует с архитектурного каркаса. Подробный поэтапный план внедрения: [`docs/AXIOM_IMPLEMENTATION_PLAN.md`](docs/AXIOM_IMPLEMENTATION_PLAN.md).

Архитектурный контракт: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).
