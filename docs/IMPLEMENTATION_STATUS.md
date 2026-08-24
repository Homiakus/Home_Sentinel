# Implementation status

## Completed

- Stage 0 — architecture boundary, ADR, detailed plan.
- Stage 1.1 — Go 1.26 module baseline pinned to Axiom revision `7682ba9170dd`.
- Stage 1.2 — transport-neutral event envelope and validation.
- Stage 1.3 — incident trigger/risk/status contracts and deterministic execution ID.
- Stage 1.4 — content-addressed artifact reference contract.
- Stage 1.5 — initial domain unit tests.
- Stage 2.1/2.2/2.3 — gateway contracts, desired-state physical APIs and explicit effect semantics.

## Next

1. Axiom Camera lifecycle.
2. ADGO Incident v1 plan and handlers.
3. Production ADGO wiring with Pebble.
4. CI/race gates and fake-gateway idempotency tests.
