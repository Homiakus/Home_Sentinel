# Implementation status

## Completed

- Stage 0 — architecture boundary, ADR, detailed plan.
- Stage 1.1 — Go 1.26 module baseline pinned to Axiom revision `7682ba9170dd`.
- Stage 1.2 — transport-neutral event envelope and validation.
- Stage 1.3 — incident trigger/risk/status contracts and deterministic execution ID.
- Stage 1.4 — content-addressed artifact reference contract.
- Stage 1.5 — initial domain unit tests.
- Stage 2.1/2.2/2.3 — gateway contracts, desired-state physical APIs and explicit effect semantics.
- Stage 3.1/3.2/3.3 — Camera lifecycle modeled in ordinary Axiom.
- Stage 3.4 — Axiom hidden behind `camera.Service` adapter.
- Stage 3.5 — compile/transition/late-recovery regression tests added.

## Next

1. ADGO Incident v1 plan and handlers.
2. Production ADGO wiring with Pebble.
3. CI/race gates and fake-gateway idempotency tests.
4. Risk policy v2 and human-in-the-loop.
