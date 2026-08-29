# Implementation status

`MASTER_PLAN.md` is the execution source of truth. This file is an observed snapshot.

## Verified foundation

T-001/T-002/T-003/T-004/T-005/T-011/T-012/T-018/T-019/T-020/T-021/T-022/T-023/T-024 are verified.

Latest physical crash-linearization evidence on `2e8b9606212f47bf04d66bf5f1833a52195b7018`:
- module hygiene, supply-chain, vulnerability, format, vet, unit, race, engineering-loop reconciliation and benchmark smoke: PASS;
- security/SBOM/Trivy: PASS;
- engineering loop classifies the active T-005 range `CRITICAL` but selects `mutation_targets=[]` because the implementation is genuinely `_test.go`-only; no production refactor was introduced merely to manufacture mutation evidence;
- deterministic fault harness fails exactly the post-provider ADGO completion commit after a physical effect, preserves the durable running task and original non-empty idempotency key, reopens Pebble, forces lease expiry by persisted state mutation and proves safe redelivery;
- door reaches the intended locked state with one physical write; recovered delivery is suppressed by observed desired state;
- camera reaches verified stream state while redelivery presents the exact same idempotency key and the provider deduplicates the second delivery;
- siren enable is physically applied once; recovery remains waiting on the durable safety timer; the timer is made due by persisted `NotBefore` mutation and exactly one separate safety-disable completes the workflow;
- unresolved ambiguous door outcome remains in human/reconciliation state and a redrive does not blindly invoke the provider;
- no crash correctness assertion uses `time.Sleep`, scheduler probability or timeout inflation.

This closes T-005.

## Durable workflow evolution inventory — complete

- incident — retained v1/v2 graph + handler + callback/human binding semantics verified;
- siren action — reconstructible configuration-derived historical bundles verified;
- door action — immutable v1 retained-bundle boundary verified;
- camera recovery — immutable v1 retained-bundle boundary verified.

## T-006 — active P0 supported-topology slice

F-018 is confirmed: physical-resource admission is durable across executions but only process-locally atomic. `resourceguard.Check` explicitly requires the caller to serialize `Check + StartOrLoad` within one process, and door/siren/camera use a process-local `sync.Mutex` for that sequence. Two control-plane processes can therefore both observe a free resource before either creates its durable reservation.

Pinned ADGO already exposes a `Production.Admission` controller, but an expiring TTL permit is not sufficient whole-execution fencing for workflows that may remain in human/reconciliation waits unless a dedicated heartbeat/ownership lifecycle is added. v1 will therefore not claim distributed multi-writer support.

Pinned ADGO `PebbleStore` explicitly owns a process-level database lock. T-006 will use that property to make the supported topology mechanically true:
- canonicalize one runtime root per installation;
- acquire a dedicated process-lifetime writer lock under that root before application DB migration/service initialization;
- fail a concurrent second writer closed before services become available;
- keep the lock for the entire App lifetime and release it on normal or partial-startup cleanup;
- prove cross-process exclusion and reacquisition using deterministic subprocess handshake, not sleeps;
- document multiple independent roots controlling the same devices as unsupported;
- keep distributed multi-writer mode deferred until it has real fencing semantics.

T-006 production code will live under `internal/orchestration/topology` so the existing orchestration mutation-critical policy applies. Gremlins must generate non-empty clean evidence for the production slice.

## Stage 17 residuals

T-013 callback HTTP READY; T-014 principal kinds TODO; T-015 authz audit/limiting BLOCKED; T-016 command preconditions TODO; T-017 authenticator boundary BLOCKED.

## Other work

T-006 topology/fencing and T-009 release qualification are P0. T-010 verified-main guard is planned; T-008 owns target-hardware performance budgets.
