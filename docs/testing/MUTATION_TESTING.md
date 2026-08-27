# Mutation Testing / Test-of-Tests Policy

Mutation testing проверяет не покрытие строк, а способность тестов обнаруживать небольшую семантическую поломку production-кода.

## 1. Tool policy

Для Go baseline-инструмент — Gremlins либо другой специально утверждённый mutation engine.

Правила supply chain:

- версия инструмента pin-ится;
- обновление версии идёт отдельным reviewed change;
- изменение набора mutators фиксируется как изменение test semantics;
- mutation tool не получает права менять production source permanently;
- generated mutation artifacts не попадают в release binary.

До измерения baseline mutation percentage не превращается в произвольный числовой KPI.

## 2. Mutant states

Контур различает:

- `KILLED` — тест обнаружил mutation;
- `LIVED` — mutation прошла тесты;
- `NOT_COVERED` — mutated location не достигнута тестами;
- `TIMED_OUT` — требует triage, не считать автоматически качественным kill;
- `NOT_VIABLE` — mutation не компилируется/не запускается;
- `EQUIVALENT_WAIVED` — semantic equivalent или нерелевантна, есть documented justification.

Основная метрика после triage:

```text
mutation_score = killed / (killed + lived)
```

`NOT_VIABLE` и reviewed equivalent mutants не входят в denominator.

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

Для обычного кода после baseline действует no-regression по affected package и repository trend.

## 4. Cadence

### Per-change

На изменённых packages, если изменение затрагивает branch/validation/state semantics.

### Critical per-change

Обязательно для:

- `internal/security/...`;
- ingress/RBAC;
- door/siren/physical gateway;
- resource admission/fencing;
- migrations/schema compatibility;
- scenario compiler/safety compiler/catalog publication;
- durable recovery/reconciliation.

### Scheduled

Более широкий module mutation run, включая unchanged dependencies текущего subsystem.

### Release

Critical packages + все packages, затронутые release diff; survived critical mutants отсутствуют либо явно waived как equivalent с review evidence.

## 5. Mutation operators to prioritize

Особенно ценны для Home Sentinel:

- conditional boundary changes: `<` ↔ `<=`, `>` ↔ `>=`;
- condition negation;
- boolean `&&`/`||` inversion;
- equality/inequality mutation;
- arithmetic boundary changes;
- increment/decrement;
- negation removal;
- branch/body removal when supported;
- return/result changes when supported.

Tool-specific названия mutators не являются контрактом проекта и могут меняться при pin/update инструмента.

## 6. Survived mutant triage loop

```text
LIVED mutant
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
   | no -> add focused regression/property test
   | yes
   v
Is oracle too weak / path unreachable?
   |
   +-> strengthen oracle or construct reachable fixture
   |
   v
rerun focused mutation
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
3. указано, почему добавить meaningful test невозможно/бессмысленно;
4. waiver локален, а не wildcard на package;
5. изменение production semantics автоматически инвалидирует старый waiver.

## 9. Mutation + multidimensional edge space

Survived mutant повышает приоритет соответствующей области `EDGE_SPACE_MODEL.md`.

Примеры:

- survived TTL boundary -> расширить Time × Replay × KeyState;
- survived admission condition -> Concurrency × Ownership × Lease × CrashPoint;
- survived compiler branch -> GraphShape × RiskClass × Capability × Revision;
- survived size check -> PayloadSize × AuthState × DecoderState.

То есть mutation testing не является последним отчётом; он возвращает контур к генерации новых edge combinations.

## 10. Reports

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
critical lived mutants
baseline delta
```

Raw reports могут храниться как CI artifacts. В git фиксируются policy, reproducer tests и осмысленные waivers.
