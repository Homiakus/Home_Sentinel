# IMPLEMENTATION_PLAN.md

## Home Sentinel — пошаговый план реализации

**Статус:** baseline v1 — 146/146 задач реализовано [x]  
**Основной язык:** Go  
**Целевая платформа:** Debian/Ubuntu Linux + Docker Compose  
**Принцип:** local-first, modular, event-driven, fail-safe  
**Количество атомарных задач:** 146

> Этот документ является исполняемым планом разработки. Каждый пункт должен завершаться работающим, проверяемым инкрементом. Нельзя переходить к масштабному UI/AI до стабилизации конфигурационной модели, камер, Frigate, MQTT и recovery path.

---

## 1. Зафиксированные технологические решения

- Go 1.26.x.
- HTTP: `net/http` + `github.com/go-chi/chi/v5`.
- UI: `templ` + HTMX + минимальный JavaScript.
- Realtime UI: SSE по умолчанию; WebSocket только для двусторонних сценариев.
- Persistence: SQLite в WAL через `database/sql` и `modernc.org/sqlite`.
- MQTT: Mosquitto + `github.com/eclipse/paho.golang/autopaho`.
- WebSocket client: `github.com/coder/websocket`.
- Cameras: собственные adapters; ONVIF-библиотека допускается только за внутренним adapter contract.
- Media probing: `ffprobe`/FFmpeg через безопасный `os/exec` adapter, без shell interpolation.
- NVR: Frigate; go2rtc используется как stream gateway.
- HA: MQTT Discovery для Sentinel entities + официальный REST/WebSocket API для state/actions.
- AI: Ollama за `AIProvider`.
- Backup: restic за `BackupProvider`/command adapter.
- Telegram: тонкий собственный Bot API client поверх HTTP.
- Secrets: opaque `SecretRef`, никакого plaintext в domain/API/logs.
- Production containers: только pinned versions/digests; `latest` запрещён.

## 2. Инварианты проекта

1. **Frigate продолжает запись**, если Sentinel UI, HA, Ollama или Telegram недоступны.
2. **AI никогда напрямую не открывает дверь.**
3. **Observed state не подменяется desired state.**
4. **IP-адрес не является identity устройства.**
5. **Конфигурация применяется транзакционно:** Plan → Validate → Backup → Apply → Verify → Commit/Rollback.
6. **Никакого редактирования внутренних `.storage` Home Assistant.**
7. **Никаких raw RTSP credentials в UI, Telegram, логах и audit.**
8. **Никаких пользовательских строк через shell.**
9. **Каждое опасное действие имеет authz + correlation + audit.**
10. **Backup считается рабочим только после restore-test.**
11. **Один внешний сбой не должен порождать каскад ложных тревог.**
12. **Обычный пользователь работает с понятиями “камера”, “вход”, “запись”, “уведомления”, а не с YAML/FFmpeg/MQTT.**

## 3. Целевой dependency order

```text
P0 Architecture
 ↓
P1 Repository
 ↓
P2 Config/DB
 ↓
P3 Core/API/UI
 ↓
P4 Hardware
 ↓
P5 Cameras
 ↓
P6 Frigate/go2rtc
 ↓
P7 MQTT/Event Bus
 ├───────────────┐
 ↓               ↓
P8 HA           P9 Intercom
 └──────┬────────┘
        ↓
      P10 AI
        ↓
      P11 Telegram
        ↓
      P12 Dashboard
        ↓
      P13 Backup
        ↓
      P14 Security
        ↓
      P15 Reliability
        ↓
      P16 Release
```

---

## 4. Правило выполнения каждого пункта

Перед закрытием задачи обязательно:

- код собран;
- добавлены необходимые unit/contract/integration tests;
- ошибки имеют typed/reason-coded представление;
- секреты проходят redaction;
- обновлена соответствующая документация/contract;
- `go test -race ./...` не получает новой ошибки;
- выполнена указанная проверка;
- получен конкретный Definition of Done.

---


# P0 — Архитектурные контракты и границы системы

### [x] HS-001 — Зафиксировать роли компонентов

**Где:** `docs/architecture/COMPONENT_OWNERSHIP.md`

**Сделать:** Определить единственного владельца каждой ответственности: Sentinel Core — control plane, Frigate — NVR/detection, go2rtc — stream gateway, Home Assistant — automation, Mosquitto — event transport, Ollama — inference, restic — backup.

**Проверить:** Архитектурный review: для каждой функции существует ровно один primary owner.

**Definition of Done:** Нет дублирования записи видео, automation engine, AI runtime или backup engine в Go-коде.

### [x] HS-002 — Сформировать threat model и trust zones

**Где:** `docs/security/THREAT_MODEL.md`, `docs/network/TRUST_ZONES.md`

**Сделать:** Описать LAN, Camera VLAN, management plane, VPN clients, контейнерную сеть, Telegram как внешний канал; перечислить активы, атакующих, точки входа и допустимые направления трафика.

**Проверить:** Проверить сценарии compromised camera, stolen Telegram account, SSRF, leaked token, malicious RTSP endpoint.

**Definition of Done:** Для каждой угрозы указан preventive/detective/recovery control.

### [x] HS-003 — Зафиксировать систему идентификаторов

**Где:** `internal/domain/id.go`, `docs/contracts/IDENTIFIERS.md`

**Сделать:** Определить стабильные `camera_id`, `device_id`, `incident_id`, `event_id`, `request_id`, `correlation_id`; не использовать IP или display name как identity.

**Проверить:** Property-тесты генерации/парсинга, тест rename/IP change.

**Definition of Done:** Переименование камеры и DHCP-смена IP не меняют её logical identity.

### [x] HS-004 — Определить Desired/Observed State

**Где:** `internal/domain/state.go`, `docs/architecture/STATE_MODEL.md`

**Сделать:** Разделить желаемую конфигурацию пользователя, сгенерированную конфигурацию интеграций и фактически наблюдаемое состояние сервисов.

**Проверить:** Смоделировать drift: Frigate изменён вручную, камера offline, HA недоступен.

**Definition of Done:** UI может отдельно показать desired, applied и observed state.

### [x] HS-005 — Зафиксировать Event Envelope v1

**Где:** `internal/events/envelope.go`, `docs/contracts/EVENTS_V1.md`

**Сделать:** Определить обязательные поля события, версию схемы, source, occurred_at, received_at, correlation_id, causation_id, payload и severity.

**Проверить:** JSON round-trip, backward-compatible decoding, malformed payload tests.

**Definition of Done:** Все интеграции преобразуют внешние события в единый envelope.

### [x] HS-006 — Составить матрицу деградации

**Где:** `docs/operations/DEGRADATION_MATRIX.md`

**Сделать:** Для отказов Frigate, HA, MQTT, Ollama, Telegram, диска, камеры и Sentinel определить сохраняемые функции, запрещённые действия и UI-state.

**Проверить:** Tabletop failure review и проверка отсутствия циклических зависимостей.

**Definition of Done:** AI/Telegram/HA не являются необходимыми для базовой записи Frigate.

### [x] HS-007 — Зафиксировать ownership конфигураций

**Где:** `docs/contracts/CONFIG_OWNERSHIP.md`

**Сделать:** Для каждого файла/настройки определить owner, generated/manual status, допустимый способ изменения и rollback strategy.

**Проверить:** Проверить Frigate YAML, Mosquitto, HA generated files, Docker Compose, secrets, restic profiles.

**Definition of Done:** Нет файла, который одновременно считается вручную и автоматически управляемым.

### [x] HS-008 — Сформировать acceptance matrix

**Где:** `docs/ACCEPTANCE.md`

**Сделать:** Перевести ТЗ в проверяемые E2E критерии: camera onboarding, recording, detection, HA entity, doorbell, unlock, AI enrichment, Telegram, backup/restore, reboot recovery.

**Проверить:** Каждый критерий связан хотя бы с одним будущим автоматизированным тестом.

**Definition of Done:** Есть единый чек-лист, по которому можно принять релиз.

---

# P1 — Репозиторий, сборка и инженерная база

### [x] HS-009 — Инициализировать Go workspace

**Где:** `go.mod`, `go.sum`, `.go-version`

**Сделать:** Зафиксировать Go 1.26.x, module path, минимальный набор зависимостей; включить reproducible module download.

**Проверить:** `go version`, `go mod verify`, чистая сборка в контейнере.

**Definition of Done:** Проект собирается на чистой машине одной командой.

### [x] HS-010 — Создать структуру репозитория

**Где:** `cmd/`, `internal/`, `web/`, `configs/`, `deploy/`, `docs/`, `tests/`

**Сделать:** Создать только реально используемые верхнеуровневые пакеты; запретить package-by-layer explosion.

**Проверить:** `go list ./...` и архитектурный lint импортов.

**Definition of Done:** Структура соответствует bounded contexts из P0.

### [x] HS-011 — Создать единый developer entrypoint

**Где:** `Makefile` или `Taskfile.yml`

**Сделать:** Команды: bootstrap, generate, build, test, lint, integration-test, compose-up, compose-down, doctor, package.

**Проверить:** Запустить полный happy path на чистом checkout.

**Definition of Done:** Разработчику не нужно помнить набор разрозненных shell-команд.

### [x] HS-012 — Настроить форматирование и статический анализ

**Где:** `.golangci.yml` при необходимости, `tools.go`/Go tool directives

**Сделать:** Обязательные `gofmt`, `go vet`, race tests; дополнительные линтеры включать только с понятной ценностью.

**Проверить:** CI намеренно ломается на тестовом нарушении.

**Definition of Done:** Локальная и CI-проверка используют одинаковые правила.

### [x] HS-013 — Создать test taxonomy

**Где:** `docs/testing/STRATEGY.md`, `tests/`

**Сделать:** Разделить unit, contract, integration, E2E, hardware-in-loop, fault-injection, soak.

**Проверить:** Каждая будущая интеграция имеет определённый тип тестового double/fake.

**Definition of Done:** Нет интеграционных тестов, случайно зависящих от домашней сети.

### [x] HS-014 — Создать dev Docker Compose

**Где:** `deploy/compose/compose.dev.yml`

**Сделать:** Поднять Sentinel, Mosquitto, fake RTSP/ONVIF fixtures и тестовые сервисы; production Compose пока не смешивать с dev.

**Проверить:** `docker compose up` + smoke test.

**Definition of Done:** Локальная разработка возможна без реальных камер.

### [x] HS-015 — Встроить миграции и web assets

**Где:** `internal/database/migrations/`, `web/`

**Сделать:** Использовать `go:embed` для SQL migrations и стабильных статических ресурсов; generated templ код проверять в CI.

**Проверить:** Запуск бинарника из пустого working directory.

**Definition of Done:** Бинарник не зависит от исходного дерева проекта.

### [x] HS-016 — Настроить CI pipeline

**Где:** `.github/workflows/ci.yml`

**Сделать:** Build, unit, race, integration, generated-file check, dependency audit, container build.

**Проверить:** PR с намеренно сломанным тестом блокируется.

**Definition of Done:** Merge в main невозможен при красном базовом pipeline.

---

# P2 — Конфигурация, SQLite и транзакционная модель

### [x] HS-017 — Создать типизированную конфигурацию Sentinel

**Где:** `internal/config/model.go`

**Сделать:** Описать server, network, MQTT, Frigate, HA, AI, Telegram, storage, backup, security sections с явными defaults.

**Проверить:** Table-driven parse/validation tests.

**Definition of Done:** Невалидная конфигурация отвергается до запуска интеграций.

### [x] HS-018 — Реализовать загрузку конфигурации

**Где:** `internal/config/load.go`

**Сделать:** Приоритеты: defaults → config file → env overrides → secret references; печатать effective config только с redaction.

**Проверить:** Тест precedence и unknown keys.

**Definition of Done:** Нет тихого игнорирования опечаток в критичных полях.

### [x] HS-019 — Создать SecretRef

**Где:** `internal/secrets/`

**Сделать:** Секреты хранить отдельно от domain model; поддержать env/file secret providers и opaque references.

**Проверить:** Тест логирования и JSON serialization.

**Definition of Done:** Пароль камеры никогда не появляется в API response или structured log.

### [x] HS-020 — Поднять SQLite/WAL

**Где:** `internal/database/open.go`

**Сделать:** Использовать `database/sql` и CGo-free SQLite driver; задать busy timeout, foreign keys, WAL, connection limits и graceful close.

**Проверить:** Concurrent read/write test + restart.

**Definition of Done:** БД переживает аварийное завершение без логической порчи.

### [x] HS-021 — Создать migration runner

**Где:** `internal/database/migrate.go`, `internal/database/migrations/*.sql`

**Сделать:** Нумерованные forward migrations, schema_version, транзакции, запрет автоматического destructive downgrade.

**Проверить:** Миграция empty→latest и N-1→latest.

**Definition of Done:** Повторный запуск migration idempotent.

### [x] HS-022 — Создать domain repositories

**Где:** `internal/repository/`

**Сделать:** Репозитории для devices, cameras, events metadata, incidents, users, policies, revisions, backup jobs, audit.

**Проверить:** Repository contract tests против временной SQLite.

**Definition of Done:** SQL не протекает в HTTP handlers и integration adapters.

### [x] HS-023 — Реализовать config revisions

**Где:** `internal/config/revisions.go`

**Сделать:** Каждое изменение desired state создаёт immutable revision с actor, reason, diff, checksum.

**Проверить:** Create/list/diff/restore tests.

**Definition of Done:** Любое applied изменение можно связать с revision.

### [x] HS-024 — Реализовать Plan/Apply/Verify/Commit

**Где:** `internal/apply/`

**Сделать:** Построить общий transaction coordinator для внешних конфигураций с preflight, backup, atomic write, verify и rollback.

**Проверить:** Fake integration с ошибками на каждой стадии.

**Definition of Done:** Ошибка после apply возвращает last-known-good либо явно помечает manual intervention.

### [x] HS-025 — Добавить resource locks

**Где:** `internal/locks/`

**Сделать:** Сериализовать конфликтующие операции: camera onboarding, Frigate apply, restore, upgrade; чтения не блокировать без необходимости.

**Проверить:** Concurrency/race tests.

**Definition of Done:** Два apply не могут одновременно менять один managed resource.

### [x] HS-026 — Реализовать export/import Sentinel state

**Где:** `internal/backup/statebundle.go`, `cmd/sentinel/`

**Сделать:** Экспортировать schema-versioned bundle без plaintext secrets либо с отдельным encrypted secret payload.

**Проверить:** Export→fresh DB→import→compare.

**Definition of Done:** Конфигурацию можно переносить независимо от видеозаписей.

---

# P3 — Жизненный цикл приложения, API, доступ и UI-каркас

### [x] HS-027 — Собрать application lifecycle

**Где:** `internal/app/app.go`, `cmd/sentinel/main.go`

**Сделать:** Явно запускать DB, event bus, integrations, schedulers и HTTP; graceful shutdown через context.

**Проверить:** SIGTERM во время запросов и фоновой задачи.

**Definition of Done:** Процесс завершается без orphan goroutines и corrupt writes.

### [x] HS-028 — Создать HTTP router

**Где:** `internal/httpserver/router.go`

**Сделать:** Использовать `net/http` + chi; versioned `/api/v1`, UI routes отдельно, middleware ordering документировать.

**Проверить:** httptest route matrix.

**Definition of Done:** Unknown route/method возвращают предсказуемый ответ.

### [x] HS-029 — Стандартизировать API errors

**Где:** `internal/httpserver/problem.go`

**Сделать:** Единый error envelope: code, message, details, request_id; внутренние stack/error details не отдавать клиенту.

**Проверить:** Golden tests.

**Definition of Done:** UI и CLI могут машинно различать классы ошибок.

### [x] HS-030 — Реализовать локальных пользователей и sessions

**Где:** `internal/auth/`

**Сделать:** Хэширование паролей, secure session cookies, session rotation, logout-all, bootstrap admin.

**Проверить:** Auth integration tests и cookie flags.

**Definition of Done:** Без аутентификации административные API недоступны.

### [x] HS-031 — Реализовать RBAC

**Где:** `internal/authz/`

**Сделать:** Роли viewer/operator/admin и capability-based checks для live, config, backup, unlock.

**Проверить:** Matrix tests для каждого sensitive endpoint.

**Definition of Done:** Проверка прав находится в server boundary, а не только в UI.

### [x] HS-032 — Добавить CSRF и security headers

**Где:** `internal/httpserver/middleware/`

**Сделать:** CSRF для cookie-auth mutations, CSP, frame policy, content type policy, secure headers.

**Проверить:** Browser/httptest checks.

**Definition of Done:** POST из стороннего origin не выполняет действие.

### [x] HS-033 — Создать UI shell на templ+HTMX

**Где:** `web/layouts/`, `web/components/`, `web/pages/`

**Сделать:** Sidebar, top health strip, notifications, responsive layout, shared components, error/empty/loading states.

**Проверить:** Desktop/mobile snapshot tests или browser smoke.

**Definition of Done:** Все основные экраны доступны без SPA framework.

### [x] HS-034 — Добавить realtime transport

**Где:** `internal/realtime/`

**Сделать:** SSE как базовый push для status/events; WebSocket использовать только там, где нужен двусторонний канал.

**Проверить:** Reconnect/Last-Event-ID/backpressure tests.

**Definition of Done:** Потеря браузером связи не приводит к утечке subscribers.

---

# P4 — Инвентаризация сервера и HardwareProfile

### [x] HS-035 — Реализовать OS/runtime probe

**Где:** `internal/hardware/os.go`

**Сделать:** Определять distro, kernel, arch, container runtime, cgroup limits, hostname.

**Проверить:** Fixture tests `/etc/os-release` + реальная smoke.

**Definition of Done:** Doctor показывает фактическое окружение.

### [x] HS-036 — Реализовать CPU/RAM probe

**Где:** `internal/hardware/cpu.go`

**Сделать:** Собирать model, cores, instruction features, RAM total/available без привязки к одному vendor.

**Проверить:** Tests на fixtures и host.

**Definition of Done:** AI/NVR recommender получает нормализованный профиль.

### [x] HS-037 — Реализовать Intel/VAAPI probe

**Где:** `internal/hardware/video_linux.go`

**Сделать:** Проверять `/dev/dri`, доступ контейнера, `vainfo`/эквивалентный capability probe через адаптер команд.

**Проверить:** Mock command output + hardware smoke при наличии.

**Definition of Done:** Система отличает наличие GPU от реально доступного decoder path.

### [x] HS-038 — Реализовать NVIDIA probe

**Где:** `internal/hardware/nvidia.go`

**Сделать:** Определять GPU/VRAM и доступность container runtime path; не считать установленный драйвер достаточным.

**Проверить:** Mock `nvidia-smi` + negative tests.

**Definition of Done:** Рекомендация не включает CUDA, если контейнер её не видит.

### [x] HS-039 — Реализовать storage inventory

**Где:** `internal/hardware/storage.go`

**Сделать:** Mount points, filesystem, total/free, rotational, device mapping; отличать system и recordings disks.

**Проверить:** Fixture + temporary filesystem tests.

**Definition of Done:** Wizard может безопасно предложить место для архива.

### [x] HS-040 — Реализовать SMART adapter

**Где:** `internal/hardware/smart.go`

**Сделать:** Запускать `smartctl` через ограниченный command adapter, нормализовать health/temperature/errors.

**Проверить:** Recorded JSON fixtures.

**Definition of Done:** Неизвестный SMART формат даёт UNKNOWN, а не false healthy.

### [x] HS-041 — Реализовать network inventory

**Где:** `internal/hardware/network.go`

**Сделать:** Интерфейсы, addresses, routes, local CIDRs; исключать loopback/container networks из camera scan по умолчанию.

**Проверить:** Network fixture tests.

**Definition of Done:** Discovery имеет явный allowlist сетей.

### [x] HS-042 — Собрать HardwareProfile recommender

**Где:** `internal/hardware/recommend.go`

**Сделать:** Выдавать рекомендации decoder/detector/AI concurrency/storage с reason codes, а не магические изменения.

**Проверить:** Golden profiles: CPU-only, Intel iGPU, NVIDIA.

**Definition of Done:** Каждая рекомендация объяснима пользователю.

---

# P5 — Камеры: discovery, probing и onboarding

### [x] HS-043 — Создать Camera domain model

**Где:** `internal/cameras/model.go`

**Сделать:** Разделить identity, endpoint, credentials ref, streams, capabilities, policies, observed health.

**Проверить:** Serialization/validation tests.

**Definition of Done:** Domain model не содержит Frigate-specific YAML fields.

### [x] HS-044 — Создать camera credential service

**Где:** `internal/cameras/credentials.go`

**Сделать:** Связать Camera с SecretRef, поддержать rotation без смены camera_id.

**Проверить:** Password rotation test.

**Definition of Done:** Новый пароль применяется без delete/re-add камеры.

### [x] HS-045 — Реализовать WS-Discovery scanner

**Где:** `internal/cameras/discovery/wsdiscovery.go`

**Сделать:** Сканировать только разрешённые CIDR, ограничивать duration/concurrency, дедуплицировать ответы.

**Проверить:** Recorded discovery packets + timeout test.

**Definition of Done:** Одна камера не появляется десятками кандидатов.

### [x] HS-046 — Реализовать ONVIF adapter

**Где:** `internal/cameras/onvif/`

**Сделать:** Поддержать device info, capabilities, services и authentication; внешнюю библиотеку изолировать собственным интерфейсом.

**Проверить:** Contract tests на fixtures/эмуляторе.

**Definition of Done:** Смена ONVIF library не затрагивает domain/application слой.

### [x] HS-047 — Получить ONVIF media profiles

**Где:** `internal/cameras/onvif/media.go`

**Сделать:** Извлекать stream URI, codec/resolution/fps где доступны; различать main/sub streams.

**Проверить:** Fixtures разных vendors.

**Definition of Done:** Wizard показывает реальные профили, а не угадывает URL.

### [x] HS-048 — Добавить PTZ capability probe

**Где:** `internal/cameras/onvif/ptz.go`

**Сделать:** Определить поддержку PTZ и допустимые операции; пока не реализовывать автотрекинг.

**Проверить:** PTZ/no-PTZ fixtures.

**Definition of Done:** UI не показывает PTZ controls неподдерживаемой камере.

### [x] HS-049 — Реализовать Generic RTSP adapter

**Где:** `internal/cameras/rtsp/`

**Сделать:** Принимать явный URL/шаблон, безопасно маскировать credentials, проверять TCP/auth/readability.

**Проверить:** Valid, bad password, timeout, malformed URL.

**Definition of Done:** Ручной RTSP onboarding не зависит от ONVIF.

### [x] HS-050 — Реализовать ffprobe adapter

**Где:** `internal/media/ffprobe.go`

**Сделать:** Запуск с timeout и без shell, парсить JSON: streams, codec, width/height, fps, audio, bitrate.

**Проверить:** Recorded ffprobe outputs и hanging process.

**Definition of Done:** Ни один пользовательский URL не попадает в shell command string.

### [x] HS-051 — Реализовать snapshot/stream test

**Где:** `internal/cameras/probe.go`

**Сделать:** Получить кадр через controlled FFmpeg/go2rtc path и вернуть preview + latency + diagnostics.

**Проверить:** Corrupt stream, delayed stream, no keyframe.

**Definition of Done:** Камера provisioned только после успешного media test либо явного override.

### [x] HS-052 — Добавить USB/UVC adapter

**Где:** `internal/cameras/uvc/`

**Сделать:** Инвентаризировать `/dev/video*`, capability modes и преобразовать выбранный input в gateway stream.

**Проверить:** v4l2 fixtures/optional HIL.

**Definition of Done:** USB камера проходит тот же onboarding contract.

### [x] HS-053 — Создать vendor profile registry

**Где:** `configs/camera-profiles/`, `internal/cameras/profiles/`

**Сделать:** Хранить только fallback hints для известных vendors; ONVIF и явные данные всегда приоритетнее.

**Проверить:** Profile schema/golden tests.

**Definition of Done:** Профили можно обновлять без изменения business logic.

---

# P6 — Frigate и go2rtc как NVR/stream backend

### [x] HS-054 — Создать Frigate HTTP client

**Где:** `internal/integrations/frigate/client.go`

**Сделать:** Типизированный клиент только для используемых API: version/health/config/events/media; auth, timeout, retries только для safe operations.

**Проверить:** httptest fake Frigate.

**Definition of Done:** Network error и HTTP error различаются.

### [x] HS-055 — Создать Frigate capability/version gate

**Где:** `internal/integrations/frigate/capabilities.go`

**Сделать:** На старте определять версию/доступные возможности и блокировать неподдерживаемые generated features.

**Проверить:** Tests на несколько capability fixtures.

**Definition of Done:** Несовместимая версия даёт actionable diagnostic.

### [x] HS-056 — Создать Frigate config model

**Где:** `internal/integrations/frigate/config/`

**Сделать:** Собственная минимальная typed model для генерируемой части YAML; неизвестные/ручные секции не перетирать без ownership.

**Проверить:** Golden YAML tests.

**Definition of Done:** Одинаковый desired state генерирует стабильный deterministic YAML.

### [x] HS-057 — Реализовать stream mapping

**Где:** `internal/integrations/frigate/streams.go`

**Сделать:** Преобразовать Camera main/detect/audio capabilities в Frigate roles и выбрать hardware decode profile.

**Проверить:** Golden cases H264/H265/main+sub.

**Definition of Done:** Detect и recording roles не перепутаны.

### [x] HS-058 — Реализовать go2rtc stream generator

**Где:** `internal/integrations/go2rtc/config.go`

**Сделать:** Создавать стабильные stream names и единую точку подключения потребителей; credentials не дублировать в UI.

**Проверить:** Golden config + special-character credentials.

**Definition of Done:** Frigate и dashboard используют один canonical stream ID.

### [x] HS-059 — Реализовать recording/detection policies

**Где:** `internal/integrations/frigate/policies.go`

**Сделать:** Маппинг retention, zones, object tracking, snapshots, audio settings из Sentinel policy.

**Проверить:** Policy matrix tests.

**Definition of Done:** UI-настройка имеет однозначное изменение generated config.

### [x] HS-060 — Реализовать preflight validation

**Где:** `internal/integrations/frigate/validate.go`

**Сделать:** Перед заменой production config выполнять schema/API validation доступным способом и media readiness checks.

**Проверить:** Inject invalid YAML/invalid camera.

**Definition of Done:** Невалидный config никогда не становится current.

### [x] HS-061 — Реализовать atomic apply + rollback

**Где:** `internal/integrations/frigate/apply.go`

**Сделать:** Snapshot current, write temp, fsync/rename, reload/restart через adapter, wait ready, verify cameras, rollback при failure.

**Проверить:** Fault injection на write/restart/ready.

**Definition of Done:** Last-known-good автоматически возвращается после неудачного deploy.

### [x] HS-062 — Интегрировать Frigate events/media

**Где:** `internal/integrations/frigate/events.go`

**Сделать:** Получать metadata/snapshots/clips по ID, не копируя весь Frigate archive в Sentinel DB.

**Проверить:** Fake event lifecycle.

**Definition of Done:** Incident хранит ссылки/IDs и enrichment, а не дублирует видео.

### [x] HS-063 — Сверять desired и actual Frigate state

**Где:** `internal/integrations/frigate/reconcile.go`

**Сделать:** Периодически обнаруживать drift и классифицировать managed/manual differences.

**Проверить:** Ручное изменение fixture config.

**Definition of Done:** Drift виден пользователю и не затирается молча без policy.

---

# P7 — Mosquitto, MQTT и внутренняя событийная система

### [x] HS-064 — Создать production Mosquitto config

**Где:** `deploy/mosquitto/`

**Сделать:** Отдельные users/ACL для Sentinel, Frigate, HA, intercom; persistence; запрет anonymous.

**Проверить:** mosquitto_pub/sub positive/negative ACL tests.

**Definition of Done:** Каждый client видит только нужные topics.

### [x] HS-065 — Реализовать MQTT client adapter

**Где:** `internal/integrations/mqtt/`

**Сделать:** Использовать Eclipse Paho MQTT v5/autopaho за собственным интерфейсом; reconnect, session, TLS option, metrics.

**Проверить:** Broker restart during publish/subscribe.

**Definition of Done:** Reconnect не создаёт дублирующихся subscriptions.

### [x] HS-066 — Зафиксировать topic namespace

**Где:** `docs/contracts/MQTT_TOPICS.md`

**Сделать:** Разделить `sentinel/...`, `frigate/...`, `homeassistant/...`; version payloads собственных команд.

**Проверить:** Topic lint tests.

**Definition of Done:** Нет wildcard publish и конфликтующих ownership.

### [x] HS-067 — Реализовать external→internal event adapters

**Где:** `internal/events/adapters/`

**Сделать:** Frigate/doorbell/system события преобразовывать в Event Envelope v1.

**Проверить:** Fixture-driven decoder tests.

**Definition of Done:** Application layer не знает исходный MQTT payload format.

### [x] HS-068 — Создать internal EventBus

**Где:** `internal/events/bus.go`

**Сделать:** In-process fan-out с bounded queues, cancellation, metrics и явной политикой overflow.

**Проверить:** Slow subscriber stress test.

**Definition of Done:** Один медленный consumer не блокирует ingestion.

### [x] HS-069 — Реализовать transactional outbox

**Где:** `internal/events/outbox.go`

**Сделать:** Критичные side effects/notifications записывать в SQLite вместе с state change и доставлять worker-ом.

**Проверить:** Crash between commit and publish.

**Definition of Done:** После restart действие не теряется и не выполняется бесконтрольно дважды.

### [x] HS-070 — Создать Correlation Engine v1

**Где:** `internal/incidents/correlation.go`

**Сделать:** Объединять события по location/camera/time/object/request correlation; правила сделать детерминированными.

**Проверить:** Recorded timelines including overlapping visitors.

**Definition of Done:** Один звонок не создаёт 5 независимых incidents.

### [x] HS-071 — Создать event replay harness

**Где:** `cmd/sentinel-replay/`, `tests/fixtures/events/`

**Сделать:** Проигрывать записанные event sequences через тот же application pipeline без внешних side effects.

**Проверить:** Golden incidents.

**Definition of Done:** Регрессии correlation/policy воспроизводятся офлайн.

---

# P8 — Home Assistant: полная управляемая интеграция

### [x] HS-072 — Создать HA REST client

**Где:** `internal/integrations/homeassistant/rest.go`

**Сделать:** Проверка `/api/`, states, service/action calls только через allowlist; bearer auth и timeouts.

**Проверить:** httptest HA fixture.

**Definition of Done:** Никаких запросов к undocumented `.storage` endpoints.

### [x] HS-073 — Создать HA WebSocket client

**Где:** `internal/integrations/homeassistant/ws.go`

**Сделать:** Auth handshake, subscribe_events/state changes, request IDs, reconnect/resubscribe.

**Проверить:** Fake WS disconnect/reconnect.

**Definition of Done:** После reconnect подписки восстанавливаются ровно один раз.

### [x] HS-074 — Реализовать HA onboarding wizard

**Где:** `internal/setup/homeassistant.go`, `web/pages/setup/ha.templ`

**Сделать:** Ввод URL/token, TLS validation, capability check, сохранение SecretRef, диагностика connection.

**Проверить:** Wrong token/cert/offline tests.

**Definition of Done:** Wizard объясняет конкретную причину отказа.

### [x] HS-075 — Проверять MQTT integration HA

**Где:** `internal/integrations/homeassistant/mqttcheck.go`

**Сделать:** Проверить, что HA реально видит broker/discovery; при невозможности автоматической настройки дать точную UI-инструкцию и повторную проверку.

**Проверить:** HA without MQTT → configure → retry.

**Definition of Done:** Sentinel не считает HA настроенным до подтверждённого discovery path.

### [x] HS-076 — Реализовать MQTT Device Discovery publisher

**Где:** `internal/integrations/homeassistant/discovery.go`

**Сделать:** Публиковать один Sentinel device и связанные camera/intercom/system components с stable unique IDs, availability и retained configs.

**Проверить:** Real HA integration test.

**Definition of Done:** Перезапуск Sentinel не создаёт дубликаты entities.

### [x] HS-077 — Определить HA entity contract

**Где:** `docs/contracts/HA_ENTITIES.md`

**Сделать:** Зафиксировать camera status, doorbell, door, lock command proxy, AI summary, backup, disk, system health entities.

**Проверить:** Unique ID/entity lifecycle tests.

**Definition of Done:** Rename display name не ломает automations.

### [x] HS-078 — Реализовать HA action bridge

**Где:** `internal/integrations/homeassistant/actions.go`

**Сделать:** Разрешить только явно заданные domain/service действия; в обе стороны сохранять actor/correlation.

**Проверить:** Unauthorized service tests.

**Definition of Done:** Sentinel не превращается в generic arbitrary HA service executor.

### [x] HS-079 — Генерировать HA dashboard/package без `.storage`

**Где:** `deploy/homeassistant/`, `internal/integrations/homeassistant/render.go`

**Сделать:** Генерировать документированный YAML/package/dashboard вариант там, где пользователь выбрал file-managed HA; иначе предоставить импортируемый snippet/instructions.

**Проверить:** Fresh HA fixture load.

**Definition of Done:** Generated assets валидны и не повреждают существующую HA конфигурацию.

### [x] HS-080 — Добавить Frigate-in-HA verification

**Где:** `internal/integrations/homeassistant/frigatecheck.go`

**Сделать:** Проверять наличие/работу Frigate integration там, где она нужна пользователю; установку HACS/config flow не эмулировать скрытыми API.

**Проверить:** HA instance with/without integration.

**Definition of Done:** Dashboard показывает точный статус и шаг ручного подтверждения, если он неизбежен.

---

# P9 — Самодельный домофон и безопасное управление дверью

### [x] HS-081 — Зафиксировать Intercom domain/protocol

**Где:** `internal/intercom/model.go`, `docs/contracts/INTERCOM_V1.md`

**Сделать:** Camera, button, door sensor, lock actuator, audio capability; команды с request_id, issued_at, expires_at, desired action.

**Проверить:** Schema tests.

**Definition of Done:** Протокол не зависит от конкретной ESP32 платы.

### [x] HS-082 — Подготовить reference ESP32 MQTT contract

**Где:** `docs/intercom/ESP32_REFERENCE.md`, `tests/fixtures/intercom/`

**Сделать:** Topics, retained state, LWT, debounce, ack/result, monotonic command handling, reboot behavior.

**Проверить:** Simulator publishes button/state and receives commands.

**Definition of Done:** Физический контроллер можно заменить симулятором без изменения Sentinel.

### [x] HS-083 — Реализовать button ingestion

**Где:** `internal/intercom/button.go`

**Сделать:** Debounce/deduplication, timestamp normalization, event generation.

**Проверить:** Bounce burst and duplicate packet tests.

**Definition of Done:** Одно физическое нажатие даёт одно logical event.

### [x] HS-084 — Реализовать door/lock observed state

**Где:** `internal/intercom/state.go`

**Сделать:** Разделить relay command, lock state и door contact state; stale state обозначать UNKNOWN.

**Проверить:** Disconnect during open/close.

**Definition of Done:** UI никогда не выдаёт команду за подтверждённое физическое состояние.

### [x] HS-085 — Реализовать secure unlock flow

**Где:** `internal/intercom/unlock.go`

**Сделать:** Authz → optional re-auth/confirmation → expiring command → MQTT → ack → observed door transition → audit.

**Проверить:** Replay, expired request, duplicate ACK, controller offline.

**Definition of Done:** Просроченная/повторная команда не открывает дверь.

### [x] HS-086 — Интегрировать video/audio capabilities

**Где:** `internal/intercom/media.go`

**Сделать:** Связать домофон с Camera/go2rtc stream; two-way audio включать только после capability test.

**Проверить:** No-audio/one-way/two-way fixtures.

**Definition of Done:** UI скрывает неподдерживаемые controls.

### [x] HS-087 — Коррелировать doorbell incident

**Где:** `internal/incidents/entrance.go`

**Сделать:** Связать recent person review, doorbell, AI result, unlock and door state в один incident.

**Проверить:** Replay нескольких посетителей.

**Definition of Done:** Correlation остаётся детерминированной при поздних событиях.

### [x] HS-088 — Создать Entrance UI

**Где:** `web/pages/entrance/`

**Сделать:** Live stream, button event, door state, lock state, recent visitor, safe unlock interaction.

**Проверить:** Browser E2E с simulator.

**Definition of Done:** Входом можно управлять без переходов в Frigate/HA UI.

---

# P10 — Локальная AI/VLM система на Ollama

### [x] HS-089 — Создать AIProvider interface

**Где:** `internal/ai/provider.go`

**Сделать:** Методы health, models, analyze event; transport DTO не протекают в domain.

**Проверить:** Fake provider tests.

**Definition of Done:** DisabledProvider полностью отключает AI без if-ов по всему коду.

### [x] HS-090 — Создать Ollama client

**Где:** `internal/ai/ollama/client.go`

**Сделать:** Поддержать local native API и при необходимости OpenAI-compatible path через config; health/timeouts/cancel.

**Проверить:** httptest slow/malformed server.

**Definition of Done:** AI request отменяется по context и не блокирует shutdown.

### [x] HS-091 — Реализовать model inventory

**Где:** `internal/ai/ollama/models.go`

**Сделать:** Получать установленные модели, размеры/capabilities где доступны; хранить observed, не desired.

**Проверить:** Fixture list/update.

**Definition of Done:** UI различает установленную и выбранную модель.

### [x] HS-092 — Создать AI hardware recommender

**Где:** `internal/ai/recommend.go`

**Сделать:** На базе HardwareProfile назначать OFF/LIGHT/BALANCED/HIGH, max_parallel и frame budget; только рекомендации, не скрытая загрузка модели.

**Проверить:** Golden hardware profiles.

**Definition of Done:** Каждое решение имеет reason code.

### [x] HS-093 — Создать representative frame selector

**Где:** `internal/ai/frames.go`

**Сделать:** Брать ограниченное число кадров из Frigate event вокруг ключевых моментов, дедуплицировать почти одинаковые.

**Проверить:** Recorded event frames.

**Definition of Done:** VLM не получает каждый кадр видеопотока.

### [x] HS-094 — Зафиксировать prompt/schema v1

**Где:** `internal/ai/prompts/`, `docs/contracts/AI_OUTPUT_V1.schema.json`

**Сделать:** Короткий scene summary + activity/risk/object hints; machine fields только по JSON schema.

**Проверить:** Golden model outputs including extra prose.

**Definition of Done:** Невалидный structured output не управляет автоматикой.

### [x] HS-095 — Реализовать validation/confidence policy

**Где:** `internal/ai/validate.go`

**Сделать:** Schema validation, enum normalization, confidence thresholds, raw text separately.

**Проверить:** Malformed/hallucinated field tests.

**Definition of Done:** Опасные решения никогда не принимаются только из AI text.

### [x] HS-096 — Создать AI job queue

**Где:** `internal/ai/queue.go`

**Сделать:** Priority, bounded capacity, dedupe by event, concurrency, timeout, retry only safe failures.

**Проверить:** Burst 100 events + slow model.

**Definition of Done:** Запись/Frigate ingestion не блокируются VLM очередью.

### [x] HS-097 — Реализовать per-camera privacy policy

**Где:** `internal/ai/policy.go`

**Сделать:** AI off/on, allowed enrichments, remote provider denied by default, retention of frames/results.

**Проверить:** Policy tests.

**Definition of Done:** Кадры запрещённой камеры не попадают в AI provider.

### [x] HS-098 — Создать AI evaluation harness

**Где:** `cmd/sentinel-ai-eval/`, `tests/datasets/ai/`

**Сделать:** Dataset manifest, expected tags, latency/JSON-validity/resource metrics; сравнение моделей.

**Проверить:** Прогон на fake + реальной локальной модели.

**Definition of Done:** Смена модели принимается по измерениям, а не по названию.

---

# P11 — Telegram Bot как защищённый дополнительный интерфейс

### [x] HS-099 — Создать Telegram Bot API client

**Где:** `internal/integrations/telegram/client.go`

**Сделать:** Минимальный типизированный HTTP client: getMe, updates/webhook mode config, send/edit message/media, callback answers.

**Проверить:** httptest fixtures.

**Definition of Done:** Bot logic не зависит от стороннего Telegram framework.

### [x] HS-100 — Реализовать pairing

**Где:** `internal/telegram/pairing.go`

**Сделать:** Короткоживущий одноразовый pairing code из Sentinel UI, binding по numeric Telegram user ID.

**Проверить:** Expired/reused/wrong code tests.

**Definition of Done:** Username не используется как identity.

### [x] HS-101 — Связать Telegram ACL с RBAC

**Где:** `internal/telegram/authz.go`

**Сделать:** Viewer/operator/admin mappings и revoke sessions/bindings.

**Проверить:** Permission matrix.

**Definition of Done:** Viewer не получает unlock callback даже вручную сформированным запросом.

### [x] HS-102 — Реализовать event notifications

**Где:** `internal/telegram/notify.go`

**Сделать:** Snapshot, concise event summary, AI enrichment when ready, severity, dedupe/update existing message.

**Проверить:** Burst/retry/message update tests.

**Definition of Done:** Один incident не превращается в спам из множества сообщений.

### [x] HS-103 — Сделать safe deep links/live links

**Где:** `internal/telegram/links.go`

**Сделать:** Ссылки ведут на Sentinel UI через configured external/VPN base URL; не публиковать raw RTSP/go2rtc URLs.

**Проверить:** URL allowlist tests.

**Definition of Done:** Telegram не раскрывает внутренние credentials/endpoints.

### [x] HS-104 — Реализовать critical action confirmation

**Где:** `internal/telegram/actions.go`

**Сделать:** Nonce + user binding + command ID + expiry + single-use storage; unlock требует отдельного confirm step.

**Проверить:** Replay/expired/cross-user callback.

**Definition of Done:** Callback нельзя переиспользовать.

### [x] HS-105 — Реализовать команды/status UI

**Где:** `internal/telegram/commands.go`

**Сделать:** `/status`, `/cameras`, `/events`, `/door`, `/backup`, `/help`; основной UX через inline buttons.

**Проверить:** Command tests with roles.

**Definition of Done:** Каждая команда либо read-only, либо проходит общий application command path.

### [x] HS-106 — Добавить resiliency/rate limits

**Где:** `internal/telegram/worker.go`

**Сделать:** Outbox delivery, exponential backoff, Telegram retry hints, per-chat throttling и circuit breaker.

**Проверить:** Simulated 429/5xx/network outage.

**Definition of Done:** Telegram outage не раздувает память и не тормозит event ingestion.

---

# P12 — Единый Dashboard, события и инциденты

### [x] HS-107 — Реализовать Overview

**Где:** `web/pages/overview/`

**Сделать:** Security state, service health, cameras, entrance, recent incidents, disk, AI, backup.

**Проверить:** Browser smoke against fixtures.

**Definition of Done:** За 1 экран видно, есть ли проблема и где.

### [x] HS-108 — Реализовать Camera Wall

**Где:** `web/pages/cameras/wall.templ`

**Сделать:** Adaptive grid, low-latency live preview, health/record/detect indicators, lazy connection.

**Проверить:** 2/4/9 camera browser load.

**Definition of Done:** Открытие wall не создаёт лишние direct camera sessions.

### [x] HS-109 — Реализовать Camera Detail

**Где:** `web/pages/cameras/detail/`

**Сделать:** Live, stream info, events, policies, zones, AI, diagnostics, advanced raw details.

**Проверить:** E2E edit→plan→apply.

**Definition of Done:** Обычные настройки не требуют YAML.

### [x] HS-110 — Реализовать Event Feed

**Где:** `web/pages/events/`

**Сделать:** Фильтры camera/type/time/severity, thumbnails, paging/cursor, live append.

**Проверить:** Large fixture dataset.

**Definition of Done:** Feed не загружает весь архив в RAM.

### [x] HS-111 — Завершить Incident domain

**Где:** `internal/incidents/model.go`, repositories

**Сделать:** Lifecycle open→enriched→acknowledged→closed, related event IDs/media refs/actions.

**Проверить:** State transition tests.

**Definition of Done:** Недопустимый переход отвергается.

### [x] HS-112 — Реализовать Incident Timeline

**Где:** `web/pages/incidents/detail/`

**Сделать:** Временная шкала Frigate, doorbell, AI, Telegram, unlock, door transitions с correlation IDs.

**Проверить:** Golden incident.

**Definition of Done:** Пользователь может восстановить последовательность событий.

### [x] HS-113 — Реализовать unified search

**Где:** `internal/search/`, `web/pages/search/`

**Сделать:** Сначала metadata search; semantic Frigate search подключить adapter-ом при capability availability.

**Проверить:** Fallback when semantic search disabled.

**Definition of Done:** Поиск остаётся полезным без AI.

### [x] HS-114 — Реализовать System Status

**Где:** `web/pages/system/`

**Сделать:** Dependency graph, observed status, versions, resources, queues, last errors.

**Проверить:** Fault injection presentation.

**Definition of Done:** Root cause отображается выше вторичных симптомов.

### [x] HS-115 — Реализовать Settings

**Где:** `web/pages/settings/`

**Сделать:** HA, MQTT, Frigate, AI, Telegram, retention, backup, users, security; basic/advanced disclosure.

**Проверить:** RBAC/browser tests.

**Definition of Done:** Viewer не видит secret-edit controls.

### [x] HS-116 — Реализовать Diagnostics UX

**Где:** `web/pages/diagnostics/`

**Сделать:** Пошаговый camera/system doctor с evidence, cause, recommendation и safe fix button.

**Проверить:** Known failure fixtures.

**Definition of Done:** Сообщение отвечает на «что сломано, почему и что делать».

---

# P13 — Хранилище, retention, backup и восстановление

### [x] HS-117 — Создать StoragePolicy domain

**Где:** `internal/storage/policy.go`

**Сделать:** System/config/recordings/backup/cache classes, thresholds, retention expectations.

**Проверить:** Policy validation tests.

**Definition of Done:** Нельзя случайно назначить recordings на маленький system partition без предупреждения.

### [x] HS-118 — Создать retention planner

**Где:** `internal/storage/retention.go`

**Сделать:** Оценивать суточный объём по фактическим/заданным bitrate и прогнозировать дни хранения.

**Проверить:** Synthetic bitrate profiles.

**Definition of Done:** UI показывает прогноз до применения policy.

### [x] HS-119 — Реализовать Disk Guard

**Где:** `internal/storage/guard.go`

**Сделать:** NORMAL/WARNING/CRITICAL, hysteresis, forecast, emergency actions только из явного policy.

**Проверить:** Disk pressure simulation.

**Definition of Done:** System disk не заполняется незаметно из-за Sentinel metadata/cache.

### [x] HS-120 — Создать restic adapter

**Где:** `internal/backup/restic/`

**Сделать:** Запуск binary без shell, repository/password через SecretRef/env/file, parse JSON output где доступно.

**Проверить:** Fake binary + local temp repository.

**Definition of Done:** Секрет restic отсутствует в argv/logs.

### [x] HS-121 — Определить backup sets

**Где:** `internal/backup/sets.go`

**Сделать:** Critical config/DB, important snapshots/marked incidents, disposable recordings; явные include/exclude.

**Проверить:** Manifest golden tests.

**Definition of Done:** Непрерывный видеоархив не уходит в backup случайно.

### [x] HS-122 — Реализовать backup scheduler

**Где:** `internal/backup/scheduler.go`

**Сделать:** Jobs, windows, concurrency lock, missed-run behavior, manual run через тот же pipeline.

**Проверить:** Time-controlled tests.

**Definition of Done:** Два prune/backup не конфликтуют.

### [x] HS-123 — Реализовать retention + maintenance

**Где:** `internal/backup/retention.go`

**Сделать:** Политика forget/prune, отдельные maintenance credentials при append-only remote design.

**Проверить:** Disposable repository tests.

**Definition of Done:** Удаление snapshots не запускается без preview/политики.

### [x] HS-124 — Реализовать repository check

**Где:** `internal/backup/check.go`

**Сделать:** Периодический restic check, хранение результата/длительности, alert при corruption.

**Проверить:** Corrupted test repo where practical.

**Definition of Done:** Последний успешный backup и последний успешный check отображаются отдельно.

### [x] HS-125 — Реализовать sandbox restore test

**Где:** `internal/backup/restoretest.go`

**Сделать:** Restore во временный каталог, проверка manifest/checksums, открытие SQLite read-only, validation generated configs.

**Проверить:** Backup→restore test в CI/local.

**Definition of Done:** Backup не помечается «проверенным», пока restore test не прошёл.

### [x] HS-126 — Создать disaster-recovery bundle

**Где:** `cmd/sentinel restore`, `docs/operations/DISASTER_RECOVERY.md`

**Сделать:** Bootstrap metadata, compose templates, encrypted state bundle references, порядок восстановления чистого сервера.

**Проверить:** Восстановление в отдельную VM/container host.

**Definition of Done:** Документированный RTO-путь не зависит от памяти автора.

---

# P14 — Безопасность и hardening

### [x] HS-127 — Описать и сгенерировать network policy

**Где:** `docs/network/`, `deploy/firewall/`

**Сделать:** Camera VLAN→server allowlist, deny Internet by default for cameras, management/UI through trusted LAN/VPN.

**Проверить:** nmap/connection matrix in test lab.

**Definition of Done:** RTSP/MQTT/Ollama/go2rtc internal ports не доступны из WAN.

### [x] HS-128 — Реализовать SSRF guard

**Где:** `internal/security/netpolicy/`

**Сделать:** Parse/resolve host, validate every resolved IP against allowed CIDRs, block loopback/link-local/metadata/sensitive service ranges unless explicit advanced policy.

**Проверить:** DNS rebinding-style and IPv6 tests.

**Definition of Done:** Camera probe не становится arbitrary internal HTTP/RTSP proxy.

### [x] HS-129 — Реализовать centralized redaction

**Где:** `internal/security/redact/`

**Сделать:** URL credentials, Authorization headers, tokens, passwords, secret fields; использовать в logs/errors/audit diffs.

**Проверить:** Snapshot/golden tests с секретами.

**Definition of Done:** Secret scanner не находит fixture-secret в логах.

### [x] HS-130 — Завершить immutable audit trail

**Где:** `internal/audit/`

**Сделать:** Actor/source/action/target/before-after redacted/result/request/correlation; append-only semantics на уровне app.

**Проверить:** Mutation attempts/tests.

**Definition of Done:** Каждая dangerous/config operation имеет audit запись.

### [x] HS-131 — Добавить API rate limits и body limits

**Где:** `internal/httpserver/middleware/limits.go`

**Сделать:** Login, probe, AI, Telegram pairing, unlock endpoints; max request size/time.

**Проверить:** Abuse tests.

**Definition of Done:** Один клиент не истощает goroutines/RAM простыми запросами.

### [x] HS-132 — Добавить step-up protection опасных действий

**Где:** `internal/authz/sensitive.go`

**Сделать:** Unlock, restore, secret reveal/change, user admin, destructive retention требуют свежей сессии/подтверждения по policy.

**Проверить:** Stale session tests.

**Definition of Done:** Украденная старая web-session имеет ограниченный ущерб.

### [x] HS-133 — Зафиксировать supply-chain policy

**Где:** `docs/security/SUPPLY_CHAIN.md`, CI

**Сделать:** Pin Go modules, tool versions, container tags/digests, generate SBOM, dependency/vulnerability scan.

**Проверить:** CI artifact inspection.

**Definition of Done:** Production deploy не использует floating `latest`.

### [x] HS-134 — Создать security regression suite

**Где:** `tests/security/`

**Сделать:** Auth bypass, IDOR, CSRF, SSRF, command injection, secret leakage, MQTT ACL, Telegram replay, malformed media endpoints.

**Проверить:** Запуск отдельной CI job.

**Definition of Done:** Известные классы атак покрыты автоматическими regression tests.

---

# P15 — Observability, watchdog и отказоустойчивость

### [x] HS-135 — Создать Unified Health Model

**Где:** `internal/health/`

**Сделать:** UNKNOWN/STARTING/HEALTHY/DEGRADED/FAILED, freshness, cause chain, since timestamp.

**Проверить:** State-machine tests.

**Definition of Done:** UI не сводит всё к misleading online/offline.

### [x] HS-136 — Экспортировать Prometheus metrics

**Где:** `internal/telemetry/metrics.go`

**Сделать:** HTTP, camera, MQTT, Frigate, HA, AI queue/latency, Telegram, disk, backup, unlock metrics без high-cardinality IDs.

**Проверить:** Metrics scrape + cardinality check.

**Definition of Done:** Метрики пригодны для alerting без взрыва series.

### [x] HS-137 — Настроить structured logging

**Где:** `internal/telemetry/logging.go`

**Сделать:** Go `slog`, request_id/correlation_id/component, levels, redaction, rotation delegated runtime.

**Проверить:** Golden log tests.

**Definition of Done:** Один incident трассируется через несколько интеграций.

### [x] HS-138 — Реализовать bounded watchdog

**Где:** `internal/watchdog/`

**Сделать:** Health probes, retry/backoff/jitter, circuit breaker, restart adapter только для разрешённых сервисов.

**Проверить:** Flapping service simulation.

**Definition of Done:** Нет бесконечного restart storm.

### [x] HS-139 — Построить dependency/root-cause graph

**Где:** `internal/health/graph.go`

**Сделать:** Camera→go2rtc→Frigate, HA→MQTT, AI→Ollama и т.д.; suppress derivative alerts при root failure.

**Проверить:** Broker-down scenario.

**Definition of Done:** Пользователь получает одну корневую проблему вместо каскада ложных тревог.

### [x] HS-140 — Создать soak/fault-injection suite

**Где:** `tests/fault/`, `docs/testing/SOAK.md`

**Сделать:** Camera disconnect, broker restart, Frigate restart, Ollama hang, Telegram 429, disk pressure, server restart.

**Проверить:** Минимум 72h pre-release soak на реальном стенде.

**Definition of Done:** Нет uncontrolled memory/goroutine growth и система восстанавливается согласно matrix.

---

# P16 — Установка, обновления, релиз и окончательная приёмка

### [x] HS-141 — Создать Update Manager

**Где:** `internal/update/`

**Сделать:** Inventory component versions/digests, compatibility rules, pre-update backup, staged update, health verification.

**Проверить:** Fake old→new component set.

**Definition of Done:** Обновление нельзя начать без совместимости и rollback data.

### [x] HS-142 — Реализовать migration/rollback release path

**Где:** `internal/update/migrate.go`, `docs/operations/ROLLBACK.md`

**Сделать:** DB/config migration ordering, backward compatibility window, rollback limitations явно фиксировать.

**Проверить:** Upgrade N-1→N→rollback where supported.

**Definition of Done:** Необратимая migration требует явного предупреждения/backup checkpoint.

### [x] HS-143 — Создать Installation Wizard

**Где:** `internal/setup/`, `web/pages/setup/`

**Сделать:** Server→storage→network→MQTT→HA→Frigate→camera→intercom→AI→Telegram→backup→E2E test.

**Проверить:** Fresh VM/browser E2E.

**Definition of Done:** После wizard система имеет проверенный working baseline.

### [x] HS-144 — Собрать production packaging

**Где:** `deploy/compose/compose.prod.yml`, release artifacts

**Сделать:** Pinned images, volumes, healthchecks, least privileges, documented device passthrough GPU/video, upgrade commands.

**Проверить:** Fresh Debian deployment.

**Definition of Done:** Установка воспроизводится без ручного редактирования контейнеров.

### [x] HS-145 — Создать release qualification pipeline

**Где:** `docs/RELEASE_CHECKLIST.md`, CI/manual HIL

**Сделать:** Unit/race/integration/security/E2E/restore/fault tests + real camera matrix + reboot/internet-off test.

**Проверить:** Release candidate проходит полный checklist.

**Definition of Done:** Тег релиза ставится только после сохранённого qualification report.

### [x] HS-146 — Выполнить финальную acceptance matrix

**Где:** `docs/ACCEPTANCE.md`, `reports/acceptance-<version>.md`

**Сделать:** Пройти все пользовательские сценарии ТЗ и записать evidence: screenshots/log refs/test IDs/backup snapshot ID.

**Проверить:** Independent re-run на production-like host.

**Definition of Done:** Все mandatory критерии PASS; exceptions имеют owner и явное решение.

---

# 5. Milestones

## [x] M0 — Architecture locked
Закрыты HS-001…HS-008. Можно писать код без размытых границ ответственности.

## [x] M1 — Sentinel Core boots
Закрыты HS-009…HS-034. Есть запускаемый Go control plane, БД, auth, API и UI shell.

## [x] M2 — Hardware-aware camera manager
Закрыты HS-035…HS-053. Sentinel обнаруживает сервер и добавляет реальные камеры разных типов.

## [x] M3 — Working NVR path
Закрыты HS-054…HS-071. Камера → go2rtc/Frigate → recording/detection → normalized event работает и восстанавливается после restart.

## [x] M4 — Smart-home and intercom
Закрыты HS-072…HS-089. HA видит Sentinel entities; домофон и безопасный unlock работают через единый control plane.

## [x] M5 — Local intelligence
Закрыты HS-090…HS-107. Локальная VLM и Telegram добавляют enrichment/notifications, не входя в критический recording path.

## [x] M6 — Unified product UX
Закрыты HS-108…HS-117. Повседневная работа выполняется из Sentinel Dashboard.

## [x] M7 — Recoverable system
Закрыты HS-118…HS-127. Настроены retention, backup, check и доказанное восстановление.

## [x] M8 — Hardened production
Закрыты HS-128…HS-141. Security, observability, fault isolation и soak tests подтверждены.

## [x] M9 — Release 1.0
Закрыты HS-142…HS-146. Есть воспроизводимая установка, обновление, rollback и полный acceptance report.

---

# 6. Обязательная E2E последовательность перед v1.0

```text
Fresh Debian host
  ↓
Install Sentinel
  ↓
Hardware detection
  ↓
Configure storage
  ↓
Start Mosquitto + Frigate + HA + Ollama
  ↓
Add ONVIF camera
  ↓
Add generic RTSP camera
  ↓
Add/interconnect DIY doorbell
  ↓
Verify live + recording + detection
  ↓
Verify HA MQTT Discovery entities
  ↓
Press doorbell
  ↓
Correlate Frigate person + button
  ↓
Run local VLM enrichment
  ↓
Receive Telegram notification
  ↓
Confirm safe unlock
  ↓
Observe door transition
  ↓
Create backup
  ↓
Restart server
  ↓
Verify automatic recovery
  ↓
Restore backup into clean sandbox
  ↓
Run acceptance suite
```

# 7. Запрещённые сокращения пути

- Не писать собственный NVR вместо Frigate.
- Не писать собственный video transcoder вместо FFmpeg/go2rtc.
- Не хранить копию всего видеoархива в Sentinel DB.
- Не делать Ollama обязательным для записи.
- Не создавать скрытую зависимость door unlock от HA/Telegram.
- Не использовать Home Assistant `.storage` как API.
- Не разрешать generic arbitrary shell commands из Dashboard.
- Не разрешать generic arbitrary HA service calls.
- Не считать HTTP 200 достаточной проверкой камеры — нужен media decode/snapshot.
- Не делать auto-fix конфигурации без revision/plan/verify/rollback.
- Не считать snapshot restic доказательством backup без restore test.
- Не выпускать релиз с floating container tags.

# 8. Приоритет первой рабочей версии

Если нужно быстрее получить полезный рабочий результат, реализовывать строго такой vertical slice:

1. HS-001…HS-034 — foundation.
2. HS-035…HS-053 — camera onboarding.
3. HS-054…HS-071 — Frigate + MQTT.
4. HS-072…HS-080 — Home Assistant.
5. HS-082…HS-089 — домофон.
6. HS-108…HS-110 и HS-117 — минимальный удобный Dashboard.
7. HS-118…HS-127 — backup/recovery.
8. Только после этого HS-090…HS-107 — VLM и Telegram enrichment.

Такой порядок специально не позволяет AI/UI-функциям оттянуть разработку от критической задачи: надёжно принять поток камеры, записать его, обнаружить событие, пережить перезапуск и восстановиться из backup.
