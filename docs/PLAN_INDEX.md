# Home Sentinel — индекс планов и engineering tracks

Все планы являются частями одного roadmap и не должны реализовываться как независимые архитектуры.

## 0. Обязательный execution protocol

[`engineering/ENGINEERING_LOOP.md`](engineering/ENGINEERING_LOOP.md)

Это не отдельный product track, а обязательный способ выполнения всех планов ниже. Перед каждым изменением контур reconciles записанный статус с фактическим `main`, формирует минимальный Work Packet, моделирует многомерное edge-space, пишет executable evidence, выполняет race/property/fuzz/fault/mutation gates и только после этого обновляет `[x]`/status.

Связанные test contracts:

- [`testing/EDGE_SPACE_MODEL.md`](testing/EDGE_SPACE_MODEL.md) — multidimensional boundary/interleaving/crash space;
- [`testing/MUTATION_TESTING.md`](testing/MUTATION_TESTING.md) — test-of-tests и mutation policy;
- [`testing/STRATEGY.md`](testing/STRATEGY.md) — общий test mesh.

Запрещено выполнять roadmap по одним Markdown-чекбоксам: recorded status является кэшем и обязан сверяться с кодом, contracts и verification evidence.

## 1. Production control-plane master plan

[`AXIOM_IMPLEMENTATION_PLAN.md`](AXIOM_IMPLEMENTATION_PLAN.md)

Содержит базовую архитектуру Home Sentinel, Axiom/ADGO integration, domain/gateway contracts, durable workflows, security, resource ownership, persistence, migrations, observability, performance и release/rollback gates.

## 2. Scenario Authoring / Automation product track

[`SCENARIO_SYSTEM_PLAN.md`](SCENARIO_SYSTEM_PLAN.md)

Содержит подробный план пользовательской системы сценариев: canonical Scenario AST, Capability Registry, typed expressions/temporal operators, Scenario Compiler, Safety Compiler, immutable draft/publish catalog, simulation, API, Simple Builder, Advanced Graph, templates/subflows, LLM authoring, live trace и mobile UX.

Этот track является продолжением master plan и использует его safety/runtime foundation. Он **не создаёт второй orchestration engine**.

## 3. Architecture contracts

- [`ARCHITECTURE.md`](ARCHITECTURE.md) — dependency boundaries.
- [`ADR-0001-axiom-adgo-boundary.md`](ADR-0001-axiom-adgo-boundary.md) — граница Axiom/ADGO и media/data plane.
- [`THREAT_MODEL.md`](THREAT_MODEL.md) — security assets, trust boundaries и threat controls.

## 4. Engineering gates

- [`PERFORMANCE.md`](PERFORMANCE.md) — performance/allocation baseline policy.
- [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) — записанный baseline; всегда reconciles с фактическим `main` перед выбором следующей работы.
- [`engineering/ENGINEERING_LOOP.md`](engineering/ENGINEERING_LOOP.md) — evidence-driven cyclic implementation/autofix state machine.

## Dependency rule

```text
MASTER PRODUCTION FOUNDATION
  domain / gateway / auth / storage / migrations / ownership
                    |
                    v
        SCENARIO HEADLESS FOUNDATION
 AST / capabilities / types / compiler / safety / catalog
                    |
                    v
           AUTHORING PRODUCT LAYER
 simple UI / graph / simulation / templates / AI / trace
```

Ни один UI/AI scenario path не может обойти master-plan RBAC, gateway, resource ownership, reconciliation или release/migration semantics.

Каждый переход по dependency graph выполняется через `ENGINEERING_LOOP.md`; mutation/edge-space evidence является частью Definition of Done для затронутых critical semantics.
