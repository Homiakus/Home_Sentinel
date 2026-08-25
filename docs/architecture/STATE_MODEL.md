# Desired / Applied / Observed state

- **Desired**: user/policy intent stored by Sentinel.
- **Applied**: last configuration revision Sentinel successfully committed to an integration.
- **Observed**: current facts probed from the real system.

`desired == applied` does not imply healthy observed state. `applied != observed` can represent drift, external edits or failed runtime state.
