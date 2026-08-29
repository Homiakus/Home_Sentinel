# Сборка, настройка и управление Home Sentinel

Для обычной установки на Linux в репозитории есть единая точка управления:

```bash
./scripts/sentinelctl
```

Если запустить её без аргументов в интерактивном терминале, откроется простое меню. Все действия также доступны отдельными командами, поэтому скрипт удобно использовать по SSH и в автоматизации.

## Быстрый старт

```bash
git clone https://github.com/Homiakus/Home_Sentinel.git
cd Home_Sentinel

./scripts/sentinelctl setup
./scripts/sentinelctl doctor
./scripts/sentinelctl build
./scripts/sentinelctl start
./scripts/sentinelctl status
```

После запуска web-интерфейс по текущему security-контракту слушает только loopback:

```text
http://127.0.0.1:8080
```

Для удалённого доступа используйте доверенный VPN или reverse proxy/TLS. Скрипт специально не переводит HTTP-сервер на `0.0.0.0` автоматически.

## Что делает `setup`

Команда идемпотентна: её можно запускать повторно.

Она создаёт локальные runtime-каталоги, которые уже исключены из Git:

```text
bin/                    собранный бинарник
var/config.json         локальная конфигурация
var/data/               SQLite и runtime-данные
var/log/                журналы
var/run/                PID-файл host-mode процесса
var/frigate-secrets/    локальные credentials Frigate
```

`var/config.json` создаётся с безопасным минимальным профилем: loopback HTTP, стандартные таймауты и выключенные experimental features. Если `.env` отсутствует, рядом создаётся копия `.env.example` для будущей Docker/production настройки. Production image placeholders скрипт не подменяет непроверенными версиями.

## Основные команды

| Команда | Назначение |
|---|---|
| `menu` | интерактивное меню |
| `setup` | подготовить каталоги и базовую конфигурацию |
| `configure` | открыть `var/config.json` в `$EDITOR` |
| `doctor` | проверить Go, Git, Make, JSON-конфиг, порт 8080, Docker и runtime state |
| `build` | собрать `./cmd/sentinel` в `bin/sentinel` с build metadata |
| `image` | собрать локальный Docker image `home-sentinel:local` |
| `check` | запустить полный существующий `make check` |
| `run` | запустить Sentinel в foreground |
| `start` | запустить Sentinel в background |
| `stop` | корректно остановить background-процесс |
| `restart` | перезапустить |
| `status` | показать PID, версию, пути и состояние TCP/8080 |
| `logs [N]` | показать последние N строк и следить за логом |
| `update` | `git pull --ff-only`, rebuild и restart при необходимости |

`update` отказывается работать при незакоммиченных локальных изменениях — это защищает локальные правки от случайной перезаписи.

## Проверка окружения

```bash
./scripts/sentinelctl doctor
```

`doctor` возвращает ненулевой exit code только при настоящих блокирующих ошибках. Предупреждения, например незаполненный `.env`, не мешают host-mode запуску.

Версия Go берётся из `.go-version`. Если установленная версия ниже требуемой, сборка останавливается до начала компиляции.

## Настройка

```bash
export EDITOR=nano
./scripts/sentinelctl configure
```

Runtime precedence самого приложения сохраняется без изменений:

```text
defaults < SENTINEL_CONFIG JSON < SENTINEL_* environment
```

Управляющий скрипт задаёт только необходимые host-mode overrides:

```text
SENTINEL_CONFIG=<repo>/var/config.json
SENTINEL_DB_PATH=<repo>/var/data/sentinel.db
SENTINEL_LISTEN=127.0.0.1:8080
SENTINEL_FRIGATE_CREDENTIALS_DIR=<repo>/var/frigate-secrets
```

Дополнительные `SENTINEL_*` переменные можно задавать перед запуском скрипта.

## Постоянная служба systemd

На Linux Home Sentinel можно перевести из PID-managed режима в нормальную системную службу:

```bash
./scripts/sentinelctl service-install
./scripts/sentinelctl service-enable
./scripts/sentinelctl service-status
```

Отключение:

```bash
./scripts/sentinelctl service-disable
```

Удаление unit-файла:

```bash
./scripts/sentinelctl service-remove
```

Unit запускается от текущего пользователя, использует тот же `var/config.json`, автоматически перезапускается при ошибке и получает `UMask=0077`, `NoNewPrivileges=true`, `PrivateTmp=true`.

Не используйте одновременно `start` и активный systemd unit: выберите один способ управления процессом.

## Docker

Сборка локального Sentinel image:

```bash
./scripts/sentinelctl image
```

Production Compose сейчас **не запускается автоматически** через `sentinelctl`. Причина — существующий compose-файл задаёт приложению `SENTINEL_LISTEN=0.0.0.0:8080`, а текущий runtime намеренно запрещает non-loopback plaintext bind. Автоматически обходить эту проверку небезопасно.

`doctor` явно показывает это как предупреждение. После того как в архитектуре будет формализован доверенный reverse-proxy/container bind (или TLS listener), управление полным Compose-стеком можно безопасно включить в тот же скрипт.

## Makefile shortcuts

Те же операции доступны через Makefile:

```bash
make setup
make doctor
make build
make start
make status
make stop
make image
make manage
```

`make manage` открывает интерактивное меню.

## Восстановление после ошибки запуска

Если `start` завершился сразу, скрипт автоматически показывает последние строки журнала. Полный журнал:

```bash
./scripts/sentinelctl logs 200
```

Проверка:

```bash
./scripts/sentinelctl doctor
```

После изменения конфигурации:

```bash
./scripts/sentinelctl restart
```
