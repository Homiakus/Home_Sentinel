# ADR-0001: граница Axiom / ADGO в Home Sentinel

Status: Accepted

## Контекст

Home Sentinel должен одновременно обрабатывать высокочастотные данные камер/датчиков и надёжно выполнять редкие, но критичные security workflows. Попытка прогонять кадры, треки или сырые MQTT сообщения через workflow engine создаст лишние аллокации, latency и coupling.

## Решение

1. Media/CV/data plane не импортирует Axiom/ADGO.
2. Domain-пакеты не импортируют Axiom/ADGO.
3. `internal/orchestration/*` является единственным application-слоем, который знает о Axiom/ADGO.
4. Обычный Axiom применяется к lifecycle объектов.
5. ADGO применяется к durable graph execution.
6. Внешние эффекты идут через `internal/gateway`.
7. Большие artifacts хранятся вне execution state; в workflow передаются URI/digest/metadata.
8. LLM/VLM/ML workers возвращают facts; право на физическое действие остаётся у deterministic policy/control plane.
9. External effects считаются at-least-once и обязаны поддерживать idempotency или reconciliation.
10. Все high-risk действия требуют явного policy gate; необратимые действия не могут быть скрыты внутри inference handler.

## Следствия

Плюсы:
- Axiom можно обновлять/заменять без переписывания media/CV ядра;
- отказ worker/coordinator не уничтожает incident state;
- решения объяснимы через history/Explain;
- проще тестировать security policy без камер и Home Assistant.

Минусы:
- появляется явный adapter/gateway слой;
- нужно проектировать idempotency keys;
- workflow contracts становятся частью публичной внутренней архитектуры и требуют versioning.
