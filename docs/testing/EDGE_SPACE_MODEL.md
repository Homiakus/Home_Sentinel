# Multidimensional Edge-Space Model

Этот документ задаёт обязательный способ проектирования граничных тестов Home Sentinel. Edge case рассматривается как комбинация факторов, а не как изолированный пример.

## 1. Factor model

Минимальный каталог измерений:

| Dimension | Typical factors |
|---|---|
| Input | empty/min/max/just-below/just-above/oversize/malformed/unknown field |
| Identity | valid/unknown/renamed/duplicate/stale ID/wrong tenant or scope |
| Version | current/older compatible/newer unknown/stale ETag/plan digest mismatch |
| Time | before boundary/exact boundary/after boundary/expired/future/skew/+jump/-jump |
| Ordering | in-order/out-of-order/duplicate/late/same timestamp/tie-break |
| Concurrency | single/two same resource/two different resources/cancel-vs-complete/retry-vs-reconcile |
| Persistence | fresh/reopen/partial commit/crash-before/crash-after/corrupt/read-only |
| External dependency | success/timeout/429/5xx/malformed/response lost/accepted-but-unknown |
| Authorization | anonymous/viewer/operator/security-admin/system/stale credential/replay |
| Ownership | free/held/same owner/other owner/expired lease/stale fence |
| Capacity | empty/normal/high watermark/full/unbounded-attempt/flood |
| Topology | single writer/second writer/restart/host change/network partition |
| Cancellation | active/cancelled/deadline exceeded/late completion/context ignored |
| Recovery | retry/reconcile/compensate/escalate/fail-closed/safe-disable |
| Privacy | public/telemetry/identity/secret/media/audit/redacted export |

Для конкретного Work Packet допускается удалять нерелевантные измерения, но запрещено молча игнорировать `Time`, `Concurrency`, `Persistence`, `Authorization` и `Recovery` в P0 physical/security/durable коде.

## 2. Boundary values

Для чисел и durations:

```text
min-invalid, min, min+epsilon, nominal, max-epsilon, max, max-invalid
0, -1, +1, overflow-adjacent
NaN, +Inf, -Inf where type/decoder permits
```

Для строк/bytes:

```text
empty, 1 byte, normal, max-1, max, max+1, invalid UTF-8 if reachable,
control chars, delimiter chars, path-like, shell-like, JSON nesting/size abuse
```

Для enum/version:

```text
known-low, known-current, deprecated-compatible, unknown-future, zero value
```

Для collections:

```text
nil, empty, one, duplicate, max accepted, max+1, adversarial ordering
```

## 3. Constraints

Не все декартовы комбинации валидны. Модель должна хранить constraints, например:

```text
system principal => cannot choose human approval action
expired admission => holder may be stale
response_lost => provider outcome can be applied or unknown
storage_read_only => new risky workflow denied
simulation_mode => production gateway impossible
```

Invalid-by-construction комбинации не включаются в coverage denominator. Однако вход, который внешний пользователь может прислать, не считается invalid-by-construction: он должен быть протестирован как reject path.

## 4. Interaction strength

Используется adaptive t-way strategy:

- обычная локальная логика: минимум 2-way;
- cross-component stateful logic: 3-way;
- security, authorization, persistence, concurrency, migration: 4-way на релевантных факторах;
- physical action safety: 4-way baseline + exhaustive critical projection;
- 5/6-way targeted combinations применяются scheduled/release для небольших критических factor sets, когда полный набор реалистичен.

Strength — не самоцель. Если известна опасная комбинация из 5 факторов, она добавляется напрямую даже при общем 3-way suite.

## 5. Critical projections

Для safety-кода выделяются подпространства, которые проверяются почти или полностью исчерпывающе.

### Door unlock

```text
Authorization × HumanApproval × ResourceOwnership × ProviderOutcome
× CrashPoint × VerificationResult × Replay/Staleness
```

Ключевой oracle: ни одна комбинация без валидной authority path не приводит к подтверждённому unlocked state.

### Siren

```text
DesiredState × Timer × Cancellation × CrashPoint × ProviderOutcome
× Compensation × Restart
```

Oracle: bounded activation и safe-disable остаются достижимыми при recovery.

### Scenario Safety Compiler

```text
RiskClass × CapabilityKind × UserGraphShape × GeneratedSafetyNodes
× CatalogRevision × TamperAttempt
```

Oracle: required safety node cannot disappear from published System Graph.

### Callback ingress

```text
KeyState × iat/exp × ReplayState × Principal × PayloadSize
× WorkflowBinding × RequestConcurrency
```

Oracle: malformed/stale/replayed/unauthorized input не мутирует workflow.

## 6. Sequence space

Многие Home Sentinel faults зависят не от набора значений, а от порядка событий.

Обязательные sequence families:

- duplicate before/after commit;
- late then newer event;
- newer then late event;
- cancel before provider call;
- cancel during call;
- cancel after provider accepted;
- restart while waiting;
- restart after external effect but before commit;
- new revision published while old execution waits;
- lease expires while stale worker resumes;
- reconcile races with new desired-state request.

Для state machines покрываются:

- all states;
- all legal transitions;
- all illegal transition rejection paths;
- transition pairs для critical FSM;
- selected 3-transition paths around irreversible/physical effects.

## 7. Crash-point matrix

Для каждого external effect:

```text
C0 before durable enqueue
C1 after enqueue / before claim
C2 after claim / before provider call
C3 provider accepted / response lost
C4 response received / before local completion commit
C5 local completion committed / caller response lost
```

Каждый crash point комбинируется минимум с:

- provider idempotent vs ambiguous semantics;
- restart;
- same-resource contender;
- cancellation/deadline;
- verify/reconcile outcome.

## 8. Coverage metrics

Code coverage не используется как единственный критерий. Evidence bundle может содержать:

- boundary-class coverage;
- t-way interaction coverage;
- state coverage;
- transition coverage;
- transition-pair coverage;
- crash-point coverage;
- fault-class coverage;
- mutation killed/lived coverage;
- fuzz corpus growth/reproducer count;
- security invariant oracle coverage.

## 9. Determinism and shrinking

Каждый generated/property/combinatorial failure обязан быть воспроизводимым:

- сохранять random seed;
- сохранять minimized input;
- сохранять factor assignment;
- сохранять event sequence;
- сохранять virtual clock state;
- сохранять crash point.

После падения контур должен shrink/minimize случай до наименьшего воспроизводимого counterexample и только затем чинить production code.

## 10. Prioritization

Если полный набор слишком дорогой:

1. safety/security invariants;
2. changed code interactions;
3. previously failed combinations;
4. mutation-surviving paths;
5. 4-way critical subsets;
6. 3-way changed subsystem;
7. 2-way broad regression;
8. long-tail scheduled fuzz/soak.

Нельзя экономить тестовый бюджет путём удаления crash/replay/authority измерений из physical action path.
