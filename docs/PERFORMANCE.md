# Performance and allocation gates

Home Sentinel has two very different performance domains.

## Hot path

Media decode, frame inference and tracking are explicitly outside Axiom/ADGO. The correlation layer must remain bounded in time and memory.

Benchmarks:

```bash
go test ./internal/correlation -run '^$' -bench . -benchmem
go test ./internal/policy/risk -run '^$' -bench . -benchmem
```

`BenchmarkCorrelatorIngest` measures the bounded temporal aggregation path. `BenchmarkAssess` measures deterministic risk scoring.

No numerical regression threshold is claimed until a real baseline has been produced on the target Home Sentinel hardware. First release must record `ns/op`, `B/op` and `allocs/op` and then convert them into explicit CI budgets.

## Control path

Axiom/ADGO is not used per frame. Production performance validation must separately measure:

- incident StartOrLoad latency;
- task enqueue/claim/commit latency;
- Pebble reopen/recovery latency;
- concurrent incident throughput;
- history growth per incident;
- callback verification cost;
- door/siren reconciliation latency.

## Memory bounds already encoded

- correlation events per subject: configurable finite maximum;
- seen event IDs: configurable finite maximum;
- artifact references per correlated candidate: configurable finite maximum;
- ADGO global/activity concurrency: explicit plan/config limits.

## Release rule

Performance claims are evidence-based: do not add an optimization or a numeric SLO to this document without attaching a benchmark baseline and target hardware description.
