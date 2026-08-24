# Home Sentinel threat model

## Assets

1. Physical authority: door locks, sirens, relays and Home Assistant actions.
2. Sensitive media: snapshots, clips, face crops, audio.
3. Presence/identity facts and incident history.
4. Credentials for cameras, messaging providers and automation gateways.
5. Durable Axiom/ADGO state and operator decisions.

## Trust boundaries

```text
untrusted camera/network input
        -> parsers/perception
        -> normalized observations
        -> correlation
        -> deterministic policy / ADGO
        -> gateway
        -> physical world

external callback
        -> authentication + expiry + replay guard
        -> authorization
        -> ADGO durable human/signal boundary
```

## Required invariants

- ML/LLM/VLM output is evidence, never authority to invoke a physical gateway directly.
- Domain/media/perception packages have no credentials for physical actuators.
- Unlock is represented as desired state and requires a durable high-risk human gate.
- Siren enable has a bounded deadline and ensure-disabled compensation.
- Every external write has an idempotency key and verify/reconcile semantics.
- Ambiguous side effects are not blindly retried.
- Callback tokens bind execution + node + event + nonce + expiration and are HMAC authenticated.
- Callback authentication keys never enter ADGO Execution.Data/history.
- ADGO `SeenEvents` remains the durable semantic dedup boundary; the in-memory replay guard is defense in depth only.
- Raw media bytes never enter workflow state.
- Operator history/read models redact credential-like keys.

## Primary threats and controls

### Forged owner callback
Control: HMAC-SHA256 token, constant-time verification, TTL, nonce replay guard, then workflow/node binding.

### Callback replay
Control: edge replay guard plus stable ADGO event ID deduplication. A process restart may clear the edge guard, therefore durable ADGO dedup is authoritative.

### Duplicate physical command after worker crash
Control: persist-before-execute, stable idempotency key, desired-state command, read-before-write and verify-after-write.

### Provider accepted command but response was lost
Control: classify as ambiguous, verify physical state, otherwise durable reconciliation requiring operator/provider evidence.

### Compromised inference model
Control: inference cannot import/use actuator gateways; deterministic policy and human gates own authority.

### Durable state exfiltration
Control: secrets are prohibited in execution data. Store only references/IDs and sanitized facts. Filesystem permissions and encryption-at-rest remain deployment responsibilities.

### Event flood / memory exhaustion
Control: bounded correlator group size, bounded seen-event cache, bounded artifact references and ADGO admission/concurrency limits.

### Camera outage recovery loop
Control: no unbounded graph cycle. One automatic reconnect plus at most one explicit operator-authorized final attempt.

## Deployment requirements not yet provided by this repository

- OS/filesystem hardening and encrypted disk/key store;
- authenticated HTTP/TLS ingress;
- real Home Assistant/camera/notification adapters;
- per-user authorization/RBAC;
- key rotation mechanism;
- secure backup/restore policy.

These are intentionally called out as remaining deployment work rather than hidden behind application abstractions.
