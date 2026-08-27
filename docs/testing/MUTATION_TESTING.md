# Mutation Testing / Test-of-Tests Policy

Mutation testing проверяет не покрытие строк, а способность тестов обнаруживать небольшую семантическую поломку production-кода.

## 1. Tool policy

Для Go baseline-инструмент — **Gremlins v0.6.0** (`github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0`).

Почему версия фиксирована:

- Gremlins находится в ветке 0.x и не гарантирует backward-compatible CLI/config между minor releases;
- обновление версии меняет test semantics и проходит отдельным reviewed change;
- per-change режим использует `gremlins unleash --diff <base>`;
- JSON-результат обязательно проходит через `sentinel-engloop mutation`, поэтому raw exit code Gremlins не является единственным quality oracle.

Правила supply chain:

- версия инструмента pin-ится в Makefile/CI;
- изменение версии или набора mutators фиксируется как изменение test semantics;
- mutation tool не получает права оставлять production source изменённым;
- generated mutation artifacts не попадают в release binary;
- локальные бинарники и отчёты хранятся в `.tools/` и `.artifacts/`, исключённых из git.

До измерения baseline mutation percentage не превращается в произвольный числовой KPI.

## 2. Mutant states

Контур различает:

- `KILLED` — тест обнаружил mutation;
- `LIVED` — mutation прошла тесты;
- `NOT COVERED` — mutated location не достигнута тестами;
- `TIMED OUT` — результат не считается доказательством корректности;
- `NOT VIABLE` — mutation не компилируется/не запускается;
- `SKIPPED` — mutation не исполнялась;
- `EQUIVALENT_WAIVED` — semantic equivalent или нерелевантна, есть documented justification.

Основная метрика после triage:

```text
mutation_score = killed / (killed + lived)
```

`NOT VIABLE` и reviewed equivalent mutants не входят в denominator.

## 3. Blocking rules

Независимо от среднего score блокируют item:

- survived mutant меняет authz outcome;
- survived mutant меняет physical desired state;
- survived conditional boundary меняет timeout/TTL/lease/expiry semantics;
- survived mutant отключает verify/reconcile/compensation;
- survived mutant меняет resource ownership/fencing;
- survived mutant позволяет simulation вызвать production gateway;
- survived mutant меняет immutable revision/digest safety semantics;
- survived mutant делает parser/size-limit permissive в security ingress.

Для critical production surfaces `sentinel-engloop mutation` также считает блокирующими `NOT COVERED` и `TIMED OUT`: отсутствие доказательства на safety-sensitive mutation не считается успехом.

Для обычного кода после baseline действует no-regression по affected package и repository trend.

## 4. Cadence

### Per-change

На изменённом коде с branch/validation/state semantics через `--diff` относительно merge-base/предыдущего commit.

### Critical per-change

Обязательно для:

- `internal/security/...`;
- `internal/auth/...` и `internal/authz/...`;
- door/siren/physical gateway;
- resource admission/fencing;
- migrations/schema compatibility;
- scenario compiler/safety compiler/catalog publication;
- durable recovery/reconciliation.

### Scheduled

Более широкий module/subsystem mutation run, включая unchanged dependencies текущего subsystem.

### Release

Critical packages + все packages, затронутые release diff; survived critical mutants отсутствуют либо явно waived как equivalent с review evidence.

## 5. Mutation operators to prioritize

Особенно ценны для Home Sentinel:

- conditional boundary changes: `<` ↔ `<=`, `>` ↔ `>=`;
- condition negation;
- boolean/logical inversion;
- equality/inequality mutation;
- arithmetic boundary changes;
- increment/decrement;
- negation removal;
- branch/body removal when supported;
- return/result changes when supported.

Tool-specific названия mutators не являются контрактом проекта и могут меняться только вместе с reviewed pin update.

## 6. Survived mutant triage loop

```text
LIVED / NOT COVERED / TIMED OUT on critical surface
   |
   v
Is mutation semantically equivalent?
   | yes -> document + review waiver
   | no
   v
Which invariant should kill it?
   |
   v
Does a test exist?
   | no -> add focused regression/property/model test
   | yes
   v
Is oracle weak / path unreachable / timing nondeterministic?
   |
   +-> strengthen oracle or construct deterministic fixture
   |
   v
rerun focused mutation and affected edge-space
```

Контур не должен менять production code только ради «убийства» equivalent mutant.

## 7. Test changes induced by mutation

Разрешено:

- добавить более точный assertion;
- добавить boundary example;
- добавить property/invariant;
- улучшить fixture до reachable state;
- добавить missing negative test;
- добавить replay/crash scenario.

Запрещено:

- привязать тест к incidental implementation detail без contract reason;
- snapshot-ить случайные внутренние структуры только ради score;
- повышать sleep/timeouts;
- исключать package из mutation без reason;
- отключать mutator, потому что он находит реальные survived mutants.

## 8. Equivalent mutant waiver

Waiver допустим только если:

1. приведена mutation/location;
2. объяснено, почему observable behavior неизменен;
3. указано, почему meaningful test невозможен или бессмыслен;
4. waiver локален, а не wildcard на package;
5. изменение production semantics инвалидирует старый waiver.

Автоматический `sentinel-engloop mutation` не применяет waivers сам: waiver является отдельным reviewed evidence.

## 9. Mutation + multidimensional edge space

Survived или inconclusive critical mutant повышает приоритет соответствующей области `EDGE_SPACE_MODEL.md`.

Примеры:

- TTL boundary -> `Time × Replay × KeyState`;
- admission condition -> `Concurrency × Ownership × Lease × CrashPoint`;
- compiler branch -> `GraphShape × RiskClass × Capability × Revision`;
- size check -> `PayloadSize × AuthState × DecoderState`.

Mutation testing возвращает контур к генерации edge combinations, а не заканчивает проверку отчётом.

## 10. Commands

```text
make gremlins-install
make mutation-diff BASE=origin/main

go run ./cmd/sentinel-engloop mutation --file .artifacts/gremlins.json
```

Gremlins по умолчанию имеет нулевой efficacy threshold, поэтому Home Sentinel не полагается на произвольный процентный порог. Semantic blocking критических mutations выполняет собственный parser/gate.

## 11. Reports

Минимальный summary:

```text
scope
commit
mutation tool/version
mutants total
killed
lived
not covered
not viable
timeouts
waived equivalents
critical lived/inconclusive mutants
baseline delta
```

Raw JSON хранится как CI artifact; в git остаются policy, reproducer tests и reviewed waivers.
