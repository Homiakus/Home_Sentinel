# Home Sentinel — architecture contract

## 1. Слои

### 1.1 Media plane

Отвечает за RTSP/WebRTC, decode, buffering, recording и snapshots. Не импортирует orchestration.

### 1.2 Perception plane

Детекторы, tracker, face/person recognition, VLM. Результат — наблюдение/факт, а не команда физическому устройству.

### 1.3 Event plane

Нормализует события из camera/CV/HA/sensors/intercom. Обязательные свойства события: стабильный ID, source, occurred_at, kind, payload, artifact refs.

### 1.4 Domain

Содержит типы и правила предметной области без зависимости от Axiom/ADGO, network transports и UI.

### 1.5 Orchestration

- `orchestration/lifecycle`: Axiom-модели для bounded object lifecycle.
- `orchestration/incident`: ADGO workflows для долгих процессов.

### 1.6 Gateway

Единственная граница для внешних side effects: notification, Home Assistant, lock/door, siren, intercom, artifact archive.

## 2. Направление зависимостей

```text
cmd -> application/orchestration -> domain
                       |
                       +-> gateway interfaces

adapters -> gateway interfaces
media/perception -> domain events
```

Запрещено:

```text
domain -> adgo
media -> adgo
vision -> adgo
gateway implementation -> domain orchestration internals
```

## 3. Event contract

Все события должны быть дедуплицируемыми. `Event.ID` должен переживать redelivery. Timestamp источника и timestamp приёма не смешиваются.

## 4. Artifact contract

Workflow state хранит только `ArtifactRef`:
- URI;
- SHA-256 digest;
- size;
- media type;
- source metadata.

JPEG/video/audio bytes запрещены в durable control state.

## 5. External effect contract

Каждая activity, меняющая внешний мир:
1. имеет deadline/timeout;
2. имеет bounded retry;
3. получает stable idempotency key;
4. либо передаёт key downstream, либо read-before-write проверяет desired state;
5. умеет классифицировать ambiguous side effect;
6. для high-risk reversible action имеет compensation/reconciliation path.

## 6. Security authority

ML/LLM/VLM не являются authority. Они могут вычислять `confidence`, `identity`, `risk_features`, `summary`, но не имеют прямого доступа к relay/lock/siren gateway.

## 7. Storage

Начальная production topology: один Go process + ADGO Pebble backend + отдельный artifact store. Multi-host storage вводится только при измеренной необходимости.

## 8. Versioning

Каждый ADGO plan имеет immutable version. Миграция активных executions — только явно. Изменение семантики completed nodes без plan version bump запрещено.

## 9. Test pyramid

- domain unit tests;
- Axiom model compile + transition tests;
- ADGO plan compile tests;
- runtime workflow tests на MemoryStore;
- idempotency/reconciliation contract tests;
- crash/restart tests на durable backend;
- race tests;
- integration tests с fake gateways;
- hardware-in-loop отдельно от CI.
