# Home Sentinel — Evidence-Driven Engineering Loop

Статус: обязательный протокол выполнения `docs/PLAN_INDEX.md`.

Цель контура — циклически превращать roadmap Home Sentinel в маленькие доказуемые изменения кода, не подменяя качество количеством тестов и не доверяя статусам плана без проверки фактического репозитория.

## 1. Источники истины

Контур использует четыре разных класса источников и не смешивает их:

1. **Intent** — `docs/PLAN_INDEX.md`, `docs/AXIOM_IMPLEMENTATION_PLAN.md`, `docs/SCENARIO_SYSTEM_PLAN.md`, legacy `IMPLEMENTATION_PLAN.md`.
2. **Observed implementation** — текущий `main`: код, миграции, конфигурация, generated artifacts, `go.mod/go.sum`, workflows.
3. **Verification evidence** — тесты, race/fuzz/property/mutation/fault-injection результаты, benchmark/performance evidence, restore/replay evidence.
4. **Recorded status** — `docs/IMPLEMENTATION_STATUS.md` и `docs/PROGRESS.md`.

Recorded status является кэшем, а не источником истины. Если Markdown говорит `[ ]`, а acceptance уже доказан кодом и тестом, задача переводится в `VERIFIED` и статус обновляется. Если Markdown говорит `[x]`, но доказательство отсутствует или устарело, задача переводится в `PARTIAL`/`STALE`.

### 1.1 Состояния plan item

Каждый item перед выполнением нормализуется в одно из состояний:

- `OPEN` — implementation evidence отсутствует;
- `PARTIAL` — часть acceptance существует;
- `IMPLEMENTED_UNVERIFIED` — код есть, необходимого evidence нет;
- `VERIFIED` — acceptance доказан;
- `BLOCKED` — отсутствует prerequisite или внешний ресурс;
- `STALE` — записанный статус противоречит `main`;
- `SUPERSEDED` — item заменён более новым контрактом и явно связан с ним.

Ни один item не закрывается только по наличию файла или компилирующегося кода.

## 2. Главная state machine

```text
RECONCILE_PLAN
      |
      v
SELECT_ITEM -> SLICE_WORK -> DEFINE_INVARIANTS
      |                         |
      |                         v
      |                  MODEL_EDGE_SPACE
      |                         |
      |                         v
      |                    WRITE_RED_TESTS
      |                         |
      |                         v
      +-----------------> IMPLEMENT_MINIMUM
                                |
                                v
                         RUN_LOCAL_GATES
                          /           \
                       fail           pass
                        |               |
                        v               v
                   CLASSIFY_FAULT   EXPAND_TESTS
                        |               |
                        v               v
                 CONTROLLED_REPAIR  MUTATION_TEST
                        |               |
                        +------<--------+
                                |
                                v
                         ADVERSARIAL_GATES
                                |
                                v
                          DIFF_INVARIANT_AUDIT
                                |
                                v
                         UPDATE_EVIDENCE_STATUS
                                |
                                v
                         COMMIT / CI VERIFY
                                |
                         +------+------+
                         |             |
                       DONE         NEXT ITEM
```

Любая новая информация может вернуть цикл назад. Например, survived mutant возвращает в `WRITE_RED_TESTS`, race failure — в `MODEL_EDGE_SPACE`, обнаруженный stale plan — в `RECONCILE_PLAN`.

## 3. Выбор следующего пункта master plan

Выбор детерминированный и dependency-aware.

Порядок:

1. нарушение safety invariant или красный `main`;
2. `P0` item, ошибочно считавшийся закрытым;
3. незакрытый prerequisite для уже начатой функции;
4. следующий production item из фактического порядка `AXIOM_IMPLEMENTATION_PLAN.md`;
5. scenario item, prerequisites которого реально закрыты;
6. `P1`, затем `P2`;
7. refactor/cleanup только при измеримой связи с риском, стоимостью изменения или performance.

Контур не обязан идти по номеру Stage. Он обязан соблюдать dependency graph и текущие acceptance gates.

### 3.1 Reconciliation до начала кода

Для candidate item контур обязан:

- прочитать его текст и связанные contracts/ADR;
- найти фактические реализации и тесты;
- проверить, не был ли item уже закрыт более поздним commit;
- проверить prerequisites;
- выписать недостающие acceptance clauses;
- определить минимальный вертикальный slice.

Результат — `Work Packet`.

## 4. Work Packet

Перед изменением production-кода создаётся логическая карточка:

```yaml
plan_item: Stage-17.authenticated-ingress
intent: "Authenticated ingress + RBAC for command endpoints"
status_before: PARTIAL
risk_class: CRITICAL
invariants:
  - "system principal cannot obtain human authority"
  - "unlock requires explicit authorized human path"
contracts:
  - docs/THREAT_MODEL.md
  - docs/AXIOM_IMPLEMENTATION_PLAN.md
code_surfaces:
  - internal/api/...
  - internal/security/...
edge_model: docs/testing/EDGE_SPACE_MODEL.md
required_gates:
  - unit
  - property
  - fuzz
  - race
  - contract
  - mutation-critical
  - replay
  - security
acceptance_evidence:
  - "unauthorized request rejected before workflow mutation"
  - "stale/replayed callback rejected"
```

Это не обязательно отдельный committed YAML для каждого мелкого изменения; структура обязательна как рабочий контракт контура и должна отражаться в PR/commit evidence для крупных P0 slices.

## 5. Сначала инварианты, затем код

До реализации формулируются:

- state invariants;
- authorization invariants;
- identity/version invariants;
- idempotency/linearization invariants;
- persistence/restart invariants;
- cancellation/deadline invariants;
- resource ownership/fencing invariants;
- secret/redaction invariants;
- compatibility/migration invariants;
- degraded-mode invariants.

Каждый критический invariant должен иметь минимум один executable oracle: unit/property/model/fault/mutation-sensitive test.

## 6. Многомерное пространство граничных случаев

Перед кодом строится факторная модель по `docs/testing/EDGE_SPACE_MODEL.md`.

Контур рассматривает edge case не как список отдельных примеров, а как точку в пространстве факторов:

```text
Input × Identity × Time × Ordering × Concurrency × Persistence
× External Failure × Authorization × Resource Ownership
× Capacity × Topology × Version × Cancellation × Recovery
```

Для факторов задаются значения, constraints и risk interactions. Затем выбираются:

- обязательные single-factor boundaries;
- t-way covering combinations;
- targeted high-order combinations для опасных связок;
- sequence/interleaving tests;
- crash-point matrix;
- adversarial values.

Нельзя ограничиваться pairwise для physical/security/durability логики. Pairwise — baseline, а strength повышается по риску.

## 7. Test pyramid заменяется на test mesh

Для каждого slice применимы только необходимые слои, но критический код проходит несколько независимых oracle families:

### G0 — Static/build

- `gofmt`;
- module hygiene;
- compile;
- `go vet`;
- architecture/import restrictions;
- generated-code consistency.

### G1 — Deterministic unit/table/golden

Чистые функции, parsers, policy, normalization, state transitions, semantic digests.

### G2 — Property/fuzz/model

- Go native fuzzing для decoders/parsers/boundaries;
- property tests для invariants;
- model/state-machine tests;
- shuffled/duplicated/reordered streams;
- deterministic seeds и replay corpus.

### G3 — Concurrency/race

- `go test -race`;
- synchronized start barriers для contested resources;
- cancellation/timeout races;
- lease/admission interleavings;
- different-resource parallelism.

### G4 — Contract/integration

- fake/recorded servers;
- malformed/slow/429/5xx/partial responses;
- SQLite/Pebble/reopen semantics;
- network boundary behavior;
- no dependency on home LAN.

### G5 — Fault/crash/recovery

Для external effect проверяются linearization points:

1. before durable enqueue;
2. after enqueue / before claim;
3. after claim / before provider call;
4. provider accepted / response lost;
5. response received / before local commit;
6. local commit / caller response lost.

Дополнительно: SIGKILL, lease expiry, disk full/read-only, corruption, network partition, clock jump, overload.

### G6 — Test-of-tests / mutation

`docs/testing/MUTATION_TESTING.md`.

Цель — проверить, что тесты действительно отличают корректную семантику от близкой ошибочной реализации.

### G7 — E2E/HIL/UX

Пользовательский путь, real composition и отдельно tagged HIL. Для physical action HIL не заменяет simulator/fake tests, а дополняет их.

### G8 — Performance/soak/release

Measured budgets, regression trends, 24h/72h soak по release policy, restore drill и upgrade/rollback proof.

## 8. Test-of-tests через mutation testing

Для Go используется Gremlins либо другой выбранный и **зафиксированный** mutation engine. До pin версии mutation tool не становится supply-chain dependency release workflow.

Mutation testing применяется дифференцированно:

- на каждый рискованный PR — изменённые production packages;
- для `internal/security`, physical gateways, admission/fencing, compiler/safety compiler, migrations — обязательный critical mutation pass;
- полный module mutation run — scheduled/release, потому что он дорогой;
- survived mutant в коде safety invariant блокирует закрытие item независимо от среднего mutation score;
- equivalent/non-actionable mutant разрешается исключить только с документированным reason и reviewable suppression.

Первый baseline измеряется, после чего действует no-regression policy. Процентный target не выдумывается до baseline.

## 9. Controlled Auto-Fix

Автоисправление — отдельная policy boundary, а не режим «менять до зелёного».

### 9.1 Классы изменений

**A0 — механические, разрешены автоматически**

- formatting;
- import cleanup;
- deterministic generated artifacts;
- очевидные typo в internal non-contract code.

**A1 — локальные доказуемые исправления**

Разрешены при наличии воспроизводимого failing test и неизменных acceptance/invariants:

- off-by-one;
- missing validation;
- incorrect error propagation;
- deterministic branch/state bug;
- bounded resource leak.

После fix обязательно повторяются relevant edge + mutation gates.

**A2 — guarded semantic repair**

Concurrency, persistence, retry, API behavior, compiler lowering, resource admission. Допускается только если:

- есть минимальный reproducer;
- diff ограничен текущим Work Packet;
- контракт не меняется скрыто;
- проходят expanded gates;
- добавлено regression evidence.

**A3 — human/review gate, не чинить автономно**

- ослабление authz/safety policy;
- изменение смысла physical action;
- destructive migration/data deletion;
- secret/credential policy;
- public schema compatibility break;
- lowering CI/mutation/performance gates;
- удаление/skip/quarantine failing security test;
- изменение golden expected output только ради зелёного CI.

### 9.2 Запрещённые repair tactics

Контур никогда автоматически не:

- удаляет failing test;
- заменяет strong assertion на weak assertion;
- увеличивает timeout без root-cause evidence;
- добавляет sleep для лечения race;
- помечает тест flaky и исключает из release gate;
- меняет expected result под текущий buggy output;
- игнорирует error;
- отключает race/fuzz/mutation/security check;
- снижает threshold/budget для прохождения CI;
- обходит gateway/RBAC/Safety Compiler.

### 9.3 Repair budget

Один и тот же root failure допускает максимум три последовательные автономные semantic repair итерации. После третьей неудачи item становится `BLOCKED_NEEDS_REDESIGN` с сохранёнными reproducer, logs, seed, mutant/fault point и последним diff.

Новый независимый failure начинает отдельный budget.

## 10. Failure classifier

Каждый красный gate сначала классифицируется:

- `PRODUCT_DEFECT`;
- `TEST_DEFECT`;
- `SPEC_CONFLICT`;
- `NONDETERMINISM`;
- `ENVIRONMENT`;
- `DEPENDENCY_REGRESSION`;
- `PERFORMANCE_REGRESSION`;
- `SECURITY_INVARIANT_BREACH`.

Repair запрещён до классификации. `TEST_DEFECT` не означает автоматическое изменение теста: сначала доказывается противоречие теста контракту.

## 11. Diff invariant audit

Перед commit контур анализирует не только тесты, но и смысл diff:

- нет нового physical write вне gateway;
- нет нового authority path из AI/system principal;
- нет plaintext secret в durable/log/API structures;
- нет process-local mutex как единственной гарантии cross-execution ownership;
- нет unbounded queue/retry/history growth;
- нет wall-clock dependency там, где нужен injectable clock;
- нет silent compatibility break;
- нет mutable published scenario semantics;
- нет production gateway в simulator.

Для критичных файлов change автоматически повышает risk class и test strength.

## 12. Evidence bundle

Для P0/critical slice сохраняются или описываются в commit/CI:

- plan item + plan revision/commit;
- affected contracts/invariants;
- commands/gates;
- deterministic fuzz/property seed/corpus additions;
- covering-array model/strength либо targeted combinations;
- mutation summary: killed/lived/not-covered/waived;
- fault/crash points exercised;
- benchmark delta, если затронут hot/control path;
- status transition до/после.

Не нужно commit-ить гигабайты логов; нужны воспроизводимые inputs и ссылки/артефакты CI.

## 13. Commit rule

Item делится на минимальные semantic commits. Предпочтительно:

```text
test(scope): reproduce invariant violation
feat/fix(scope): implement minimal behavior
test(scope): expand edge/fault/mutation coverage
docs(scope): reconcile plan/status evidence
```

Малый slice может быть одним commit, если red/green evidence всё равно восстанавливается из diff/CI.

Нельзя смешивать несвязанный refactor с P0 repair.

## 14. Как контур выполняет текущий roadmap

Контур обязан начать не с написания нового UI, а с reconciliation текущего `main` и открытых gates.

Production track:

1. перепроверить Stage 14 фактическим `go.sum`, clean `go mod tidy`, `go mod verify`, supply-chain gates;
2. Stage 17 authenticated ingress/RBAC;
3. Stage 15 schema/plan migration compatibility;
4. Stage 18 backup/restore/corruption/retention;
5. Stage 23/24 crash matrix, backpressure, degraded mode, topology;
6. Stage 21 real adapters + contracts/HIL;
7. Stage 22/26 observability + measured budgets;
8. Stage 27 release/upgrade/rollback.

Scenario track может двигаться только при выполненных prerequisites:

- Stage 35 API после Stage 17;
- publish/physical scenarios после ownership/topology gates;
- Stage 36+ UI не обходит catalog/compiler/safety;
- Stage 39 LLM только через structured AST;
- Stage 42 использует этот engineering loop как общий quality gate.

## 15. Definition of Done plan item

Plan item = `VERIFIED`, только если одновременно:

1. acceptance clauses сопоставлены executable evidence;
2. edge-space review завершён;
3. required deterministic tests green;
4. race/concurrency gates green, если применимо;
5. fault/restart gates green, если применимо;
6. relevant mutation pass не оставил необъяснённого safety-critical survived mutant;
7. no-regression относительно mutation/performance baseline;
8. diff invariant audit green;
9. документация/status reconciled с текущим `main`;
10. CI на commit зелёный.

Чекбокс в плане обновляется **после** evidence, а не вместо evidence.

## 16. Научная и инженерная основа

Для многомерных комбинаций используется combinatorial interaction testing / covering arrays. Pairwise является только частным случаем; для high-risk взаимодействий strength повышается. Для test-of-tests используется mutation testing: тест обязан падать при семантически опасном небольшом изменении production-кода.

Практическая политика Home Sentinel описана в:

- `docs/testing/EDGE_SPACE_MODEL.md`;
- `docs/testing/MUTATION_TESTING.md`;
- `docs/testing/STRATEGY.md`.
