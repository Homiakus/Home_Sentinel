# Home Sentinel

Home Sentinel — локальная security/automation платформа на Go. Архитектура разделяет высокочастотный media/data plane и проверяемый control plane.

## Быстрый запуск и управление

Управление теперь реализовано кроссплатформенным Go CLI и одинаково работает на Windows, Linux и macOS:

```bash
go run ./cmd/sentinelctl setup
go run ./cmd/sentinelctl doctor
go run ./cmd/sentinelctl build
go run ./cmd/sentinelctl start
go run ./cmd/sentinelctl status
```

На Linux/macOS остаётся короткий launcher `./scripts/sentinelctl`, на Windows — `./scripts/sentinelctl.ps1`. Оба только запускают Go CLI; бизнес-логики в shell больше нет.

Полный Docker Compose stack после заполнения `.env`:

```bash
go run ./cmd/sentinelctl stack-config
go run ./cmd/sentinelctl stack-up
go run ./cmd/sentinelctl stack-status
```

Control-plane HTTP в production Compose жёстко публикуется только на `127.0.0.1:8080`. Внутри контейнерного network namespace отдельный `sentinel-ingress` проксирует этот порт на loopback-only Sentinel (`127.0.0.1:18080`), поэтому основной сервер не ослабляет свою fail-closed политику remote plaintext bind.

Подробная инструкция: [`docs/MANAGEMENT.md`](docs/MANAGEMENT.md).

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
