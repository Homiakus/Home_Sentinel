# Testing strategy

Home Sentinel использует evidence-driven test mesh. Code coverage полезен как diagnostic signal, но не является доказательством качества тестов.

Обязательный execution protocol: [`../engineering/ENGINEERING_LOOP.md`](../engineering/ENGINEERING_LOOP.md).

Многомерные edge cases: [`EDGE_SPACE_MODEL.md`](EDGE_SPACE_MODEL.md).

Test-of-tests / mutation policy: [`MUTATION_TESTING.md`](MUTATION_TESTING.md).

## Test families

- **Unit/table/golden** — deterministic domain/config/state/compiler/policy functions.
- **Property/model** — invariants, state machines, normalization/determinism, shuffled event streams.
- **Fuzz** — decoders, parsers, bounded inputs, numeric extremes and security ingress.
- **Concurrency/race** — same-resource serialization, different-resource parallelism, cancellation/interleavings, `go test -race`.
- **Contract** — each integration boundary using recorded fixtures/fake servers.
- **Integration** — SQLite/Pebble/MQTT/HTTP/WebSocket/service adapters in isolated composition.
- **Fault/crash** — external-effect linearization points, process restart, lease expiry, disk pressure/corruption, network partition, clock jumps.
- **Mutation** — test-of-tests; critical survived mutants block safety-related plan items.
- **E2E** — browser/user workflow through real service composition.
- **HIL** — physical cameras/intercom/actuators, tagged separately.
- **Performance** — measured target-hardware budgets, allocations and latency distributions.
- **Soak/restore/upgrade** — long-running stability plus backup/restore and compatibility proof before production release.

## Core rules

1. Tests must not silently depend on the developer's home LAN.
2. Critical invariants require more than one oracle family where practical.
3. Edge cases are modeled as multidimensional factor interactions, not a flat checklist.
4. Pairwise is only a baseline; interaction strength increases for security, concurrency, persistence and physical actions.
5. Generated failures must preserve seed/minimized reproducer for deterministic replay.
6. Auto-fix cannot weaken assertions, remove tests, increase timeouts blindly or lower quality gates.
7. Status `[x]` is updated only after executable evidence exists.
8. Minimum soak target remains 72 hours for production release unless a later release policy explicitly supersedes it.
