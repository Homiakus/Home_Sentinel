# MQTT topic contract v1

## Ownership

| Namespace | Primary writer | Readers | Purpose |
|---|---|---|---|
| `sentinel/system/...` | Sentinel | HA, Sentinel | normalized system state |
| `sentinel/camera/<camera_id>/...` | Sentinel | HA | normalized camera state |
| `sentinel/intercom/<device_id>/state/...` | intercom | Sentinel, HA | physical observed state |
| `sentinel/intercom/<device_id>/event/...` | intercom | Sentinel | button/device events |
| `sentinel/intercom/<device_id>/command/...` | Sentinel | one intercom | expiring commands |
| `frigate/...` | Frigate | Sentinel, HA | native Frigate events/state |
| `homeassistant/...` | Sentinel/HA according to Discovery contract | HA | MQTT Discovery/configuration |

## Rules

1. Sentinel-owned JSON payloads contain `schema_version`.
2. Command payloads contain `request_id`, `issued_at`, `expires_at` and are idempotent by request ID.
3. Wildcards (`+`, `#`) are subscription syntax only and are forbidden in publish topics.
4. Stable IDs, never display names or IP addresses, appear in topic paths.
5. Native Frigate payloads are normalized at the integration boundary before application logic.
6. Device clients never receive broad `sentinel/#` publish rights.
7. Retained topics are reserved for observed state/configuration, not transient button events.

## Frigate ingestion

The initial native feeds used by Sentinel are:

- `frigate/reviews` — review change feed and event IDs.
- `frigate/tracked_object_update` — metadata enrichments such as descriptions/sub-label updates when enabled.

Application code must not depend directly on these payloads; adapters convert them to `events.Envelope` schema v1.
