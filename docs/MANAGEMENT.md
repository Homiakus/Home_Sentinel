# Сборка, настройка и управление Home Sentinel

## Единый кроссплатформенный CLI

Основная точка управления написана на Go:

```bash
go run ./cmd/sentinelctl
```

Она одинаково компилируется на Windows, Linux и macOS. Shell больше не содержит логики управления: `scripts/sentinelctl` и `scripts/sentinelctl.ps1` — только тонкие launchers.

После сборки CLI можно использовать как обычный исполняемый файл:

```bash
go build -o bin/sentinelctl ./cmd/sentinelctl
```

На Windows получится `bin\sentinelctl.exe`.

## Быстрый host-mode запуск

```bash
go run ./cmd/sentinelctl setup
go run ./cmd/sentinelctl doctor
go run ./cmd/sentinelctl build
go run ./cmd/sentinelctl start
go run ./cmd/sentinelctl status
```

Host-mode Sentinel слушает `127.0.0.1:8080`. Для удалённого доступа используйте VPN или TLS reverse proxy.

## Команды

| Команда | Назначение |
|---|---|
| `menu` | интерактивное меню |
| `setup` | runtime-каталоги, базовый config и `.env`-шаблон |
| `configure` | открыть локальный JSON config |
| `doctor` | Go/Git/config/port/Docker/Compose диагностика |
| `build` | собрать `cmd/sentinel` с build metadata |
| `image` | собрать локальный Docker image |
| `check` | кроссплатформенные gofmt/vet/test/supply-chain/engloop проверки |
| `run` | foreground host-mode |
| `start` / `stop` | background host-mode |
| `restart` | перезапуск host-mode |
| `status` | PID, версия, пути и порт |
| `logs [N]` | tail/follow журнала без зависимости от Unix `tail` |
| `update` | `git pull --ff-only`, rebuild, restart |
| `stack-config` | проверить production Compose и обязательные секреты |
| `stack-up` | поднять полный production stack |
| `stack-down` | остановить stack без удаления volumes |
| `stack-restart` | перезапустить stack |
| `stack-status` | `docker compose ps` |
| `stack-logs` | follow логов Compose |
| `stack-pull` | pull images + reconcile `up -d` |

Systemd-команды `service-install/service-enable/service-disable/service-status/service-remove` сохранены для Linux. На Windows/macOS portable host-mode `start/stop/restart` работает без systemd.

## Что создаёт setup

```text
bin/
var/config.json
var/data/
var/log/
var/run/
var/frigate-secrets/
.env
```

`var/`, `bin/` и `.env` уже исключены из Git. Повторный `setup` не перезаписывает существующий config или `.env`.

## Как устранён Docker bind conflict

Основной HTTP runtime Home Sentinel намеренно запрещает plaintext bind на non-loopback. Это ограничение не отключается и не обходится флагом.

Production Compose использует два процесса из одного и того же Sentinel image:

```text
host 127.0.0.1:8080
        |
        v
Docker published port :8080
        |
        v
sentinel-ingress 0.0.0.0:8080
(shared network namespace)
        |
        v
Sentinel 127.0.0.1:18080
```

`sentinel-ingress` запускает встроенную команду:

```bash
sentinel proxy --listen 0.0.0.0:8080 --upstream http://127.0.0.1:18080
```

Ingress разрешает только loopback upstream и очищает входящие forwarding headers перед проксированием. Published control-plane порт в Compose жёстко задан как `127.0.0.1:8080:8080` и больше не управляется через `.env`, поэтому случайно опубликовать admin/control HTTP на `0.0.0.0` через штатный Compose нельзя.

Для удалённого доступа ставьте VPN или TLS reverse proxy на host и направляйте его на `127.0.0.1:8080`.

## Production Compose

Сначала:

```bash
go run ./cmd/sentinelctl setup
```

Затем отредактируйте `.env`:

- замените все `registry.example.invalid` / `REPLACE_ME` на проверенные immutable image refs;
- укажите адрес WebRTC, доступный только доверенной LAN/VPN;
- создайте три MQTT password files и пропишите их host paths.

Проверка без запуска:

```bash
go run ./cmd/sentinelctl stack-config
```

`stack-config` fail-closed проверяет:

1. наличие Docker и Compose v2;
2. заполненные image refs;
3. наличие MQTT secret files;
4. обязательные Frigate WebRTC параметры;
5. сохранность loopback-only control-plane Compose contract;
6. `docker compose config --quiet`.

После успешной проверки:

```bash
go run ./cmd/sentinelctl stack-up
go run ./cmd/sentinelctl stack-status
```

Обновление image stack:

```bash
go run ./cmd/sentinelctl stack-pull
```

## Windows

Из PowerShell:

```powershell
.\scripts\sentinelctl.ps1 setup
.\scripts\sentinelctl.ps1 doctor
.\scripts\sentinelctl.ps1 build
.\scripts\sentinelctl.ps1 start
```

Или напрямую:

```powershell
go run ./cmd/sentinelctl status
```

Фоновый процесс запускается detached; `stop` завершает его через Windows process API. Docker-команды работают через Docker Desktop и Compose v2. Пути secret files в `.env` должны быть путями host, доступными Docker Desktop.

## Linux/macOS

```bash
./scripts/sentinelctl setup
./scripts/sentinelctl doctor
./scripts/sentinelctl build
./scripts/sentinelctl start
```

Launcher использует POSIX `sh`, но вся логика находится в Go CLI.

## Проверка кроссплатформенности

CI отдельно компилирует и тестирует `cmd/sentinelctl` на:

- Ubuntu;
- Windows;
- macOS.

Таким образом изменения platform-specific process management не считаются готовыми, пока не проходят все три runner'а.
