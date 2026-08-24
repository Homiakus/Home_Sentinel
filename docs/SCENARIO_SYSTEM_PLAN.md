# Scenario System — подробный план создания сценариев Home Sentinel

Дата: 2026-08-24.
Статус: approved implementation track; код сценарного authoring layer ещё не реализован.
Связанный master plan: `docs/AXIOM_IMPLEMENTATION_PLAN.md`.

## 1. Цель

Сделать создание сценариев Home Sentinel доступным на трёх уровнях сложности без потери safety-инвариантов:

1. **Simple Builder** — `WHEN -> IF -> THEN` для большинства бытовых сценариев.
2. **Advanced Flow Editor** — граф для branching, wait, parallel, joins, human approval и recovery.
3. **Text / AI authoring** — естественный язык и expert DSL, но только через canonical Scenario AST и тот же validator/compiler.

Пользователь не должен знать про task lease, fencing, idempotency key, compensation stack, ADGO plan digest, worker attempts или reconciliation protocol. Эти свойства обязан выводить и проверять Scenario Compiler.

Главный архитектурный принцип:

```text
Simple UI / Graph UI / Natural language / Expert text
                       |
                       v
              canonical Scenario AST
                       |
          +------------+-------------+
          |                          |
          v                          v
   semantic validator          safety compiler
          |                          |
          +------------+-------------+
                       |
                  classifier
                 /          \
                v            v
             Axiom          ADGO
          reactive FSM   durable workflow
                \            /
                 +----------+
                      |
                invocation gateway
                      |
                physical world
```

**Scenario AST является source of truth.** UI, LLM и текстовые редакторы не создают исполняемый Axiom/ADGO plan напрямую.

---

## 2. Что есть сейчас и какой разрыв закрывает этот track

Сейчас Home Sentinel имеет production-oriented workflow primitives и конкретные кодовые workflow для Incident, Door, Siren и Camera Recovery, но новый сценарий создаётся через Go-код и `CompilePlan()`.

Это даёт хороший runtime, но плохой authoring UX:

- нет пользовательского scenario catalog;
- нет draft/publish lifecycle;
- нет capability browser;
- нет typed condition builder;
- нет visual editor;
- нет dry-run simulator;
- нет conflict/dependency analysis до publish;
- нет reusable templates/subflows;
- нет safe natural-language authoring;
- нет user-facing live trace сценария.

Цель этого плана — добавить **product layer поверх Axiom/ADGO**, не переписывая их.

---

## 3. Неподвижные инварианты сценарной системы

1. Scenario UI не вызывает physical gateway напрямую.
2. LLM/VLM не имеют authority на physical action.
3. Любой сценарий сначала проходит schema/type validation.
4. High-risk action автоматически получает mandatory safety requirements.
5. `unlock`, siren/high-risk action и другие опасные операции нельзя сделать безопаснее удалением узла в графе: safety compiler добавляет обязательные gates после пользовательского AST.
6. Published scenario immutable; редактирование создаёт новый draft/version.
7. Existing execution остаётся привязан к исходному compiled plan/digest.
8. Simulation никогда не вызывает real external effect.
9. Raw media и secrets не входят в Scenario AST или execution state.
10. Device deletion/renaming проверяет scenario dependencies.
11. Physical action использует desired-state semantics, а не toggle.
12. Ambiguous side effect всегда идёт в verify/reconcile path.
13. Scenario compiler должен уметь объяснить, почему выбран Axiom или ADGO.
14. Любой generated plan должен иметь deterministic digest для одинакового canonical AST + capability versions + compiler version.
15. Unknown capability/type/version fail closed.

---

# 4. Этап 28 — Canonical Scenario Model [P0 product foundation]

## 28.1 Package layout

Создать:

```text
internal/scenario/model/
  scenario.go
  id.go
  version.go
  trigger.go
  condition.go
  expression.go
  step.go
  flow.go
  value.go
  entity_ref.go
  metadata.go
  safety.go
  normalize.go
  validate.go
```

## 28.2 Scenario aggregate

Минимальный контракт:

```go
type Scenario struct {
    ID          ID
    Version     Version
    Name        string
    Description string
    Enabled     bool

    Triggers    []Trigger
    Condition   Expr
    Flow        Flow

    Parameters  []Parameter
    Metadata    Metadata
}
```

Canonical model не должен содержать `adgo.Node`, Axiom internals или provider-specific JSON.

## 28.3 Stable identity

- `ScenarioID` не меняется между версиями.
- `RevisionID`/version меняется при publish.
- draft имеет отдельный revision ID.
- node/step ID стабилен при UI edit там, где semantic step сохраняется.
- duplicate/clone создаёт новый ScenarioID.

## 28.4 Trigger model

Типы минимум:

- device event;
- entity state change;
- threshold crossing;
- schedule/time;
- presence;
- system lifecycle;
- webhook/normalized external event;
- manual trigger;
- compound/temporal trigger.

Trigger содержит capability reference + typed parameters, а не произвольную строку handler name.

## 28.5 Flow model

Минимальные semantic steps:

- Action;
- Wait;
- If/Else;
- Switch;
- Parallel;
- Join;
- HumanApproval;
- Subflow;
- Stop/Complete;
- Retry policy override только в expert-safe form;
- compensation hint только для system/compiler, не как свободный arbitrary action.

## 28.6 Canonical normalization

Перед hash/compile:

- trim stable strings;
- normalize durations/units;
- sort unordered sets;
- preserve user-visible ordered actions;
- normalize expression tree;
- remove UI-only layout metadata из semantic digest;
- reject duplicate IDs.

## 28.7 Tests

- JSON round-trip.
- deterministic normalization.
- deterministic semantic digest.
- clone produces new ID.
- UI layout changes не меняют semantic digest.
- semantic change меняет digest.
- invalid/unknown node kinds fail closed.
- fuzz decode/normalize.

Acceptance: один canonical AST может одинаково редактироваться Simple UI, Graph UI и AI layer.

---

# 5. Этап 29 — Capability Registry [P0 UX foundation]

## 29.1 Цель

Редактор не должен знать детали RTSP, Home Assistant, relay API или конкретного camera SDK. Каждый integration публикует capabilities.

Создать:

```text
internal/scenario/capability/
  registry.go
  descriptor.go
  trigger.go
  action.go
  state.go
  schema.go
  ui_hints.go
  version.go
  permission.go
```

## 29.2 Capability descriptor

Каждая capability содержит:

- stable ID;
- provider/integration ID;
- semantic version;
- category;
- human title/description;
- input schema;
- output/state schema;
- risk level;
- required permission;
- reversible flag;
- external-effect flag;
- idempotency semantics;
- readback/verification support;
- compensation strategy class;
- UI hints;
- availability/health.

## 29.3 Device example

Camera:

```text
Triggers:
  person.detected
  motion.started
  stream.offline
Actions:
  snapshot
  record_clip
  reconnect
States:
  online
  recording
```

Door:

```text
States:
  locked | unlocked | unknown
Actions:
  lock   risk=medium
  unlock risk=high mandatory-approval
```

## 29.4 Registry rules

- no duplicate capability ID+version;
- old published scenario сохраняет ссылку на required capability version/range;
- incompatible removal blocked while active scenarios depend on capability;
- provider temporary offline != capability deleted;
- hidden/internal capability не отображается обычному user role;
- arbitrary Home Assistant service calls не публикуются как generic unsafe capability.

## 29.5 Discovery API

Нужны queries:

- list devices;
- capabilities by device;
- triggers by category;
- actions by category;
- compatible values/types;
- permission/risk metadata;
- scenarios using capability/entity.

## 29.6 Tests

- duplicate registration.
- version compatibility.
- removal dependency protection.
- role filtering.
- unavailable provider shown but not silently removed.
- deterministic registry snapshot.

Acceptance: UI строит picker полностью из registry metadata без hardcoded knowledge об integration.

---

# 6. Этап 30 — Typed values, expressions и temporal semantics [P0]

## 30.1 Typed value system

Поддержать минимум:

- Bool;
- String;
- Int;
- Float;
- Duration;
- TimeOfDay;
- Timestamp;
- Percentage/Confidence;
- Temperature;
- Illuminance;
- Distance;
- Enum;
- EntityRef<T>;
- ArtifactRef;
- List<T>.

Для unit-aware значений хранить canonical unit; UI может показывать локализованную единицу.

## 30.2 Expression AST

Не хранить condition как строку.

```text
AND
├── eq(Security.mode, away)
├── between(Current.time, 22:00, 07:00)
└── OR
    ├── eq(Person.identity, unknown)
    └── lt(Person.confidence, 0.60)
```

Expression nodes:

- literal;
- reference;
- comparison;
- logical all/any/not;
- membership;
- range/between;
- exists/missing;
- changed;
- safe arithmetic where types permit.

## 30.3 Temporal operators

Отдельные first-class constructs:

- `for(duration)`;
- `within(duration)`;
- `after(event, duration/window)`;
- `before`;
- `debounce`;
- `throttle`;
- `cooldown`;
- `once-per`;
- `repeat N within T`;
- `until`;
- timeout;
- schedule window.

Не реализовывать temporal semantics как UI-only sugar: они входят в canonical AST и compiler tests.

## 30.4 Time correctness

- UTC at persistence boundaries;
- timezone explicit для wall-clock schedules;
- DST policy explicit;
- monotonic duration where possible для active process timers;
- restart uses durable deadline semantics;
- backward clock jump не продлевает actuator indefinitely;
- missed schedule/catch-up policy explicit.

## 30.5 Tests

- type mismatch matrix.
- unit conversion.
- DST forward/backward.
- midnight-crossing ranges.
- debounce/coalesce.
- repeat-within.
- clock jump.
- expression property tests.

Acceptance: невозможно опубликовать `temperature > "hello"` или сравнить CameraRef с числом.

---

# 7. Этап 31 — Scenario Compiler и automatic runtime classification [P0]

Создать:

```text
internal/scenario/compiler/
  compiler.go
  classify.go
  axiom.go
  adgo.go
  bindings.go
  digest.go
  diagnostics.go
```

## 31.1 Compiler pipeline

```text
Scenario AST
  -> normalize
  -> type check
  -> capability resolution
  -> permission check
  -> temporal lowering
  -> safety augmentation
  -> conflict/static analysis
  -> runtime classification
  -> Axiom/ADGO compile
  -> compiled manifest + digest + diagnostics
```

## 31.2 Classification to Axiom

Предпочитать ordinary Axiom если сценарий:

- реактивный;
- не содержит durable wait;
- не требует human approval;
- не требует long-running compensation/reconciliation;
- не содержит durable fork/join/subflow;
- не требует сохранения multi-step progress между рестартами.

Пример:

```text
motion detected AND dark -> light ON
```

## 31.3 Classification to ADGO

ADGO обязателен если есть:

- wait/timeout с durable continuation;
- human decision;
- high-risk external effect;
- ambiguous reconciliation;
- compensation;
- parallel/join with durable state;
- long-running subflow;
- bounded recovery loop;
- operator intervention.

## 31.4 Compiler output

Manifest хранит:

- scenario ID/version;
- semantic digest;
- compiler version;
- capability snapshot versions;
- selected runtime;
- Axiom module hash или ADGO plan digest;
- required permissions;
- physical resources;
- external effects;
- safety augmentations;
- warnings;
- migration compatibility metadata.

## 31.5 Diagnostics

Ошибки должны указывать на semantic path:

```text
flow.steps[4].action
HS-SCN-203: door.unlock requires a human-approval authority path
```

Не показывать пользователю raw compiler stack.

## 31.6 Tests

- golden Scenario -> Axiom.
- golden Scenario -> ADGO.
- deterministic digest.
- same AST produces same plan.
- UI-only metadata has no effect.
- unsupported feature fails before runtime.
- classification explanation golden tests.

Acceptance: пользователь никогда вручную не выбирает Axiom vs ADGO.

---

# 8. Этап 32 — Safety Compiler [P0, mandatory before physical publish]

## 32.1 Назначение

Пользовательский flow описывает intent. Safety Compiler добавляет mandatory runtime semantics, которые нельзя удалить из editor graph.

## 32.2 Risk classes

Минимум:

- low — notification/read-only/light-safe operation;
- medium — lock, camera reconnect, non-critical automation;
- high — unlock, siren, security mode weakening;
- critical — future emergency/irreversible operations.

Risk приходит от capability registry и может повышаться context policy, но не понижаться UI.

## 32.3 Automatic augmentation

Для external physical effect compiler должен выводить:

- permission gate;
- resource ownership/admission;
- desired state;
- read-before-write where applicable;
- stable idempotency;
- timeout;
- bounded retry;
- verify-after-write;
- ambiguous-effect reconciliation;
- compensation/fail-safe when reversible;
- audit metadata.

Для `door.unlock` дополнительно:

- security-admin authority;
- human approval unless explicit trusted pre-authorized policy is added in future and separately threat-modelled;
- stale state/version rejection.

Для siren:

- max duration hard limit;
- ensure-disabled compensation;
- manual emergency stop path.

## 32.4 UI representation

Generated safety nodes скрыты в Simple Builder, но показываются в Inspector:

```text
Unlock Front Door
Safety added automatically:
  ✓ Security-admin approval
  ✓ Resource reservation
  ✓ Current-state read
  ✓ Idempotent desired-state write
  ✓ Verify/reconcile
```

Advanced graph может визуально показывать generated nodes как locked/system nodes; пользователь не может удалить/переподключить их небезопасно.

## 32.5 Static conflict analysis

До publish искать:

- same resource opposite desired states;
- overlapping schedules;
- recursive subflow cycles;
- unbounded loops;
- action after impossible condition;
- duplicate irreversible effects;
- missing safe terminal path;
- scenario self-trigger loops;
- mutually-triggering scenario loops.

## 32.6 Tests

- attempted safety-node removal does not affect compiled plan.
- high-risk action cannot compile without authority path.
- conflicting door actions produce diagnostic.
- siren always has max bound + compensation.
- model-generated scenario cannot bypass safety metadata.

Acceptance: no physical action can be published through scenario system with weaker guarantees than handwritten Home Sentinel workflow.

---

# 9. Этап 33 — Scenario Catalog, Draft / Validate / Publish / Rollback [P0/P1]

Создать:

```text
internal/scenario/catalog/
  store.go
  draft.go
  revision.go
  publish.go
  activation.go
  dependencies.go
  migration.go
  audit.go
```

## 33.1 Lifecycle

```text
Draft
  -> Validate
  -> Simulate
  -> Review
  -> Publish immutable revision
  -> Active/Disabled/Archived
```

## 33.2 Version rules

- edit published scenario -> new draft;
- publish creates immutable revision;
- new executions use active revision;
- old executions continue pinned revision;
- rollback = activate prior compatible revision, not mutate history;
- deleting revision blocked while non-terminal execution references it;
- capability compatibility checked before activation.

## 33.3 Optimistic concurrency

- draft has version/ETag;
- stale browser save -> 409 conflict;
- explicit merge/reload UX;
- publish uses expected draft version;
- no last-write-wins silent overwrite.

## 33.4 Audit

Persist:

- created by;
- modified by;
- published by;
- timestamp;
- reason/change note;
- semantic diff;
- validation/simulation result IDs.

## 33.5 Dependency index

Queries:

- scenarios using entity;
- scenarios using capability;
- scenarios using subflow/template;
- affected scenarios before adapter/capability upgrade.

Acceptance: production execution никогда не меняет semantics из-за редактирования активного сценария.

---

# 10. Этап 34 — Headless Simulation, Dry Run и Replay [P0 UX quality]

## 34.1 Simulation modes

1. **Pure simulation** — synthetic events/states, no side effects.
2. **Replay** — исторические normalized events/artifacts references.
3. **What-if** — clone execution snapshot and change selected facts.
4. **Shadow mode** — active inputs, actions recorded as `would_execute`.

## 34.2 Simulator output

Для каждого шага:

- matched/skipped;
- condition result;
- resolved typed inputs;
- selected branch;
- generated safety nodes;
- hypothetical external effect;
- wait/deadline;
- warnings;
- Explain reason.

## 34.3 External effect isolation

Simulation registry использует only simulation handlers. Ни один production gateway instance не передаётся simulator.

Добавить compile-time/runtime guard:

- simulation context marker;
- gateway wrappers reject simulation calls even if accidentally wired.

## 34.4 Time control

Virtual clock:

- advance by duration;
- jump to timer;
- simulate timeout;
- DST/timezone test;
- replay ordering.

## 34.5 Tests

- zero real gateway calls.
- deterministic replay.
- same fixture -> same trace.
- high-risk actions display would-execute + safety path.
- timeout/human branch simulation.

Acceptance: пользователь может проверить сложный сценарий без риска физического действия.

---

# 11. Этап 35 — Scenario API [P0/P1]

API появляется только поверх Stage 17 auth/RBAC и Stage 33 catalog semantics.

Минимальные endpoints/operations:

```text
GET    /api/v1/scenarios
POST   /api/v1/scenarios
GET    /api/v1/scenarios/{id}
PUT    /api/v1/scenarios/{id}/draft
POST   /api/v1/scenarios/{id}/validate
POST   /api/v1/scenarios/{id}/simulate
POST   /api/v1/scenarios/{id}/publish
POST   /api/v1/scenarios/{id}/activate
POST   /api/v1/scenarios/{id}/disable
GET    /api/v1/scenarios/{id}/revisions
GET    /api/v1/scenarios/{id}/dependencies
GET    /api/v1/capabilities
GET    /api/v1/entities
```

Rules:

- strict bounded JSON;
- schema version header/body;
- ETag/expected version;
- idempotency key for publish/activation;
- actor/request ID audit;
- RBAC;
- pagination;
- no raw ADGO execution/data exposure;
- no secret/camera credential fields.

Role guidance:

- viewer: read scenarios/trace;
- operator: simulate, maybe edit safe drafts depending deployment policy;
- security-admin: publish high-risk scenarios;
- system: no authoring rights.

Acceptance: UI не имеет private backdoor API, все mutations проходят те же service contracts.

---

# 12. Этап 36 — Simple Builder UX [P1, primary authoring path]

## 36.1 Mental model

Основной интерфейс:

```text
WHEN
  [trigger]

AND IF
  [conditions]

THEN
  [actions]
```

При добавлении wait/branch UI естественно расширяется:

```text
THEN
  notify owners
  wait 30 sec for acknowledgement
  otherwise
    turn courtyard light on
    sound siren for 15 sec
```

## 36.2 Screens

- Scenario List;
- Create from blank/template;
- Basic metadata;
- Trigger Picker;
- Entity/Device Picker;
- Condition Builder;
- Action Picker;
- Wait/Branch editor;
- Safety Inspector;
- Test/Simulation panel;
- Publish review;
- Revision history.

## 36.3 Progressive disclosure

Default UI показывает только пользовательские decisions.
Advanced properties раскрывают:

- timeout;
- cooldown;
- retry summary;
- execution mode;
- permissions;
- safety generated behavior.

Не показывать raw ADGO fields.

## 36.4 Validation UX

Validation inline:

- field-level;
- step-level;
- scenario-level;
- blocking errors vs warnings;
- fix suggestions.

Каждая ошибка должна приводить пользователя к конкретному control.

## 36.5 Save behavior

- autosave draft debounce;
- explicit Saved/Saving/Error state;
- no silent loss;
- stale conflict dialog;
- offline draft editing только если conflict semantics реализованы; иначе read-only/offline warning.

## 36.6 Accessibility

- keyboard navigation;
- focus order;
- screen reader labels;
- no color-only status;
- WCAG-compatible contrast;
- minimum touch target;
- reduced motion.

Acceptance UX test: пользователь без знания Axiom создаёт `motion -> if dark -> light on` без документации.

---

# 13. Этап 37 — Advanced Flow Editor [P1]

## 37.1 Graph scope

Nodes:

- Trigger;
- Condition/Decision;
- Action;
- Wait;
- Human;
- Parallel;
- Join;
- Subflow;
- Stop.

System-generated safety nodes отображаются locked/non-editable.

## 37.2 Graph interactions

- drag/drop palette;
- connect compatible ports only;
- insert node into edge;
- multi-select;
- duplicate;
- undo/redo;
- keyboard commands;
- zoom/pan/fit;
- auto-layout;
- minimap only for sufficiently large graph;
- validation badges;
- node search.

## 37.3 Typed ports

Connection allowed only if output/input schema compatible.

UI должен предотвращать большинство invalid edges до compiler error.

## 37.4 Round-trip guarantee

- graph -> Scenario AST;
- Scenario AST -> graph;
- layout metadata отдельно от semantic AST;
- save/load не меняет semantics;
- simple scenario открывается в graph mode;
- advanced graph, который не может быть выражен simple form, открывается там как read-only summary/advanced badge.

## 37.5 Performance

- 100 nodes smooth desktop editing target после baseline measurement;
- virtualized inspectors/lists;
- layout не блокирует UI thread;
- large graph warnings rather than unlimited complexity.

Acceptance: graph является представлением Scenario AST, а не отдельным source of truth.

---

# 14. Этап 38 — Templates, Blueprints и reusable Subflows [P1]

## 38.1 Templates

Категории:

Security:
- Unknown person while away;
- Door left open;
- Perimeter intrusion;
- Camera tamper/offline;
- Night movement;
- Intercom escalation.

Comfort:
- Welcome home;
- Night lighting;
- Nobody home shutdown.

Reliability:
- Camera recovery;
- Connectivity alert;
- Low battery/device health.

## 38.2 Template parameters

Template содержит typed placeholders:

```text
Camera: EntityRef<Camera>
Timeout: Duration
Recipients: List<Person/Channel>
Minimum confidence: Percentage
```

Instantiation создаёт обычный Scenario AST; дальнейшее редактирование не требует template engine runtime.

## 38.3 Subflows

Reusable semantic components:

- CaptureEvidence;
- NotifyOwners;
- CheckPresence;
- VerifyDoorState;
- EscalateIncident.

Rules:

- versioned;
- immutable published revisions;
- explicit inputs/outputs;
- no hidden physical permission escalation;
- cycle detection;
- dependency graph before update/delete.

Acceptance: common automation создаётся параметризацией, а не копированием больших графов.

---

# 15. Этап 39 — Natural-language / LLM authoring [P1/P2]

## 39.1 Authority boundary

LLM output = proposed Scenario AST only.

Никогда:

```text
LLM -> gateway
LLM -> raw ADGO plan -> execute
```

Только:

```text
user text
 -> LLM structured proposal
 -> Scenario AST schema validation
 -> capability resolution
 -> safety compiler
 -> simulation
 -> visual diff/review
 -> explicit publish authority
```

## 39.2 Tool contract

LLM получает только:

- capability registry projection;
- entity metadata allowed for user;
- scenario schema;
- templates;
- non-secret context.

LLM возвращает structured AST + assumptions + unresolved slots.

## 39.3 Clarification behavior

Если named entity ambiguous:

- не угадывать;
- вернуть unresolved entity choice;
- UI предлагает picker.

Если request небезопасен:

- compiler добавляет mandatory gates;
- LLM не может маркировать action как lower-risk.

## 39.4 Diff UX

Перед publish показывать human-readable semantic diff:

```text
Added:
  WHEN unknown person on Entrance Camera
  IF security mode = Away
  THEN notify Owners
  WAIT 30 seconds
  OTHERWISE enable Siren for 15 seconds

System safety added:
  approval / max siren duration / ensure-off / resource reservation
```

## 39.5 Tests

- malformed LLM JSON rejected.
- unknown capability rejected.
- hallucinated entity unresolved.
- prompt injection cannot add secret/raw gateway operation.
- high-risk lowering attempt ignored by compiler.
- golden natural-language examples.

Acceptance: AI ускоряет authoring, но не расширяет authority пользователя или модели.

---

# 16. Этап 40 — Live Trace, Explain и Debugging [P1]

## 40.1 Runtime trace projection

User-facing trace:

```text
23:16:40 Person detected       matched
23:16:40 Security mode = Away  pass
23:16:41 Identity = Unknown    pass
23:16:41 Notification          sent
23:16:42 Await acknowledgement waiting (18s)
```

Не отдавать raw execution data.

## 40.2 Graph overlay

States:

- pending;
- running;
- waiting;
- completed;
- skipped;
- failed;
- awaiting human;
- reconciling.

Color никогда не единственный сигнал; использовать icon/text/status.

## 40.3 Explain

Кнопка `Why?` для node/branch:

- какие facts использованы;
- condition evaluation;
- почему branch selected;
- почему action skipped;
- какой safety gate blocked action;
- какой actor approved/rejected;
- retry/reconciliation reason.

Использовать ADGO/Axiom history/Explain как источник, но через sanitized scenario read model.

## 40.4 Replay from incident

Из incident timeline:

- открыть scenario revision;
- показать exact compiled revision;
- replay inputs;
- compare с current revision в dry-run.

Acceptance: operator может понять причину поведения без чтения Go logs.

---

# 17. Этап 41 — Mobile UX и adaptive authoring [P1]

На мобильном primary mode = vertical Simple Builder.

Не делать desktop canvas уменьшенным до ширины телефона.

## 41.1 Mobile layout

```text
Scenario title
Status / Save

WHEN
[Person detected]
[Entrance camera]

IF
[Mode = Away]
[Identity = Unknown]

THEN
[Save clip]
[Notify owners]

[+ Add step]

[Test] [Review & Publish]
```

## 41.2 Mobile requirements

- bottom sheets для pickers;
- sticky primary action;
- no horizontal scrolling for basic editor;
- touch targets >= platform guidance;
- keyboard does not cover active field;
- safe area support;
- dark/light theme;
- localizable labels;
- no hover-only controls.

## 41.3 Desktop/tablet

Desktop получает split view:

- left palette/tree;
- center builder/graph;
- right inspector;
- bottom/right simulation trace configurable.

Acceptance: создание простого сценария реально завершить с телефона без перехода в desktop graph.

---

# 18. Этап 42 — Scenario quality gates, testing и release criteria [P0/P1]

## 42.1 Unit/property/fuzz

- AST decode/normalize fuzz;
- expression evaluator property tests;
- temporal operator tests;
- capability schema tests;
- compiler determinism;
- safety augmentation invariants;
- conflict analyzer;
- template substitution;
- semantic diff.

## 42.2 Compiler golden suite

Golden scenarios минимум:

1. motion + darkness -> light;
2. unknown person away -> notify;
3. owner wait -> timeout;
4. safe door lock;
5. door unlock with approval;
6. siren bounded activation;
7. parallel snapshot + notification;
8. camera recovery subflow;
9. repeated event within window;
10. conflicting schedules.

Для каждого:

- canonical AST;
- diagnostics;
- runtime classification;
- compiled manifest;
- expected safety augmentation;
- deterministic digest.

## 42.3 End-to-end UI tests

Desktop + mobile:

- create;
- edit;
- autosave;
- stale conflict;
- validate;
- simulate;
- publish;
- disable;
- rollback;
- dependency warning;
- high-risk safety review.

## 42.4 Security tests

- role cannot publish above authority;
- system principal cannot author;
- CSRF/auth tests inherited Stage 17;
- no secret in scenario export;
- no arbitrary integration service escape;
- stale publish rejected;
- tampered compiled manifest rejected;
- generated safety gates cannot be removed.

## 42.5 Crash/restart tests

Published ADGO scenario:

- restart while wait;
- restart while human approval;
- restart around external effect;
- old revision continues after new revision published;
- rollback affects only new executions.

## 42.6 Performance tests

После baseline:

- catalog list 1k/10k scenarios;
- compile latency for typical/large graphs;
- validation latency during typing;
- simulation trace throughput;
- capability picker search;
- graph rendering 100+ nodes;
- memory allocation compile/simulate.

## 42.7 Final scenario release gate

- [ ] Canonical AST versioned.
- [ ] Capability Registry versioned.
- [ ] Compiler deterministic.
- [ ] Safety Compiler mandatory for physical actions.
- [ ] Draft/publish immutable revision semantics.
- [ ] Simulator cannot access production gateways.
- [ ] Auth/RBAC active.
- [ ] Dependency/conflict analysis active.
- [ ] Simple Builder passes usability tests.
- [ ] Advanced graph round-trip proven.
- [ ] Mobile simple authoring passes UX tests.
- [ ] Live trace/Explain available.
- [ ] Old workflow revision survives application/scenario upgrade.
- [ ] No physical effect bypasses gateway/safety path.

---

# 19. API/Package target structure

```text
internal/scenario/
  model/
  capability/
  expression/
  temporal/
  validate/
  compiler/
  safety/
  catalog/
  simulation/
  trace/
  templates/
  ai/

internal/api/
  scenarios/
  capabilities/
  simulation/

web/
  scenarios/
    ScenarioList
    SimpleBuilder
    FlowEditor
    TriggerPicker
    ConditionBuilder
    ActionPicker
    EntityPicker
    SafetyInspector
    SimulationPanel
    TracePanel
    VersionHistory
    PublishReview
```

Direction of dependencies:

```text
web/api
  -> scenario services
      -> scenario model/capability/compiler
          -> Axiom/ADGO adapters
              -> gateway interfaces
```

Запрещено:

```text
web -> gateway
scenario model -> concrete adapter
LLM -> gateway
UI -> raw ADGO engine mutation
```

---

# 20. Порядок реализации

Не строить визуальный editor первым.

Правильный порядок:

1. **28 Canonical Model**.
2. **29 Capability Registry**.
3. **30 Types/Expressions/Temporal**.
4. **31 Headless Compiler**.
5. **32 Safety Compiler**.
6. **33 Catalog + immutable revisions**.
7. **34 Simulation**.
8. **35 API**.
9. **36 Simple Builder**.
10. **38 Templates/Subflows**.
11. **40 Trace/Explain**.
12. **37 Advanced Graph**.
13. **41 Mobile refinement**.
14. **39 LLM authoring** только после стабильного AST/validator/compiler.
15. **42 full quality/release gates**.

Dependencies on master production plan:

- Stages 28-32 можно разрабатывать headless параллельно с core production hardening.
- Stage 33 должен учитывать Stage 15 plan/schema migrations.
- Stage 35/36+ publication mutations требуют Stage 17 auth/RBAC.
- Physical scenario publish требует Stage 20/24 ownership topology guarantees.
- Real capability actions требуют Stage 21 adapters.
- Runtime UI/trace должен использовать Stage 22 observability/read models.
- Release зависит от Stage 27 upgrade/rollback policy.

---

# 21. Атомарный commit plan

Каждый подпункт делить на небольшие проверяемые commits.

Рекомендуемая последовательность первых commits:

1. `feat(scenario): add canonical ids and versioned AST`
2. `test(scenario): add normalization and digest golden tests`
3. `feat(scenario): add typed capability registry`
4. `test(scenario): enforce capability version compatibility`
5. `feat(scenario): add typed expression tree`
6. `feat(scenario): add temporal operators`
7. `feat(scenario): add runtime classifier`
8. `feat(scenario): compile reactive scenarios to Axiom`
9. `feat(scenario): compile durable scenarios to ADGO`
10. `feat(scenario): inject mandatory physical safety semantics`
11. `test(scenario): prove unlock and siren safety invariants`
12. `feat(scenario): add immutable draft and publish catalog`
13. `feat(scenario): add headless deterministic simulator`
14. `feat(api): expose bounded scenario authoring endpoints`
15. `feat(web): add simple scenario builder`

Для каждого commit:

- contract;
- implementation;
- unit/failure tests;
- no unrelated refactor;
- docs/status update when stage materially changes.

---

# 22. Definition of Done для пользовательского сценария

Сценарий считается готовым к production publish только если:

1. AST валиден и schema version supported;
2. все entity/capability refs разрешены;
3. types/units валидны;
4. temporal semantics bounded;
5. compiler deterministic;
6. runtime выбран автоматически и объяснимо;
7. external effects имеют idempotency/timeouts;
8. physical effects имеют resource ownership + verify/reconcile;
9. high-risk permissions/HITL удовлетворены;
10. safety augmentation успешно применена;
11. conflict/dependency analysis выполнен;
12. simulation завершилась без blocking diagnostic;
13. immutable revision создана;
14. actor publish-authorized;
15. audit record создан;
16. compiled plan/digest сохранён;
17. old revisions остаются доступными для running executions/replay;
18. UI может показать Explain/trace без raw secrets/state leak.

Этот DoD применяется одинаково к сценарию, созданному через Simple Builder, Graph Editor, template, API, AXM expert import или LLM.