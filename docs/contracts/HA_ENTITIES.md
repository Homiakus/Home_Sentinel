# Home Assistant entity contract

Home Sentinel publishes its own Home Assistant entities through MQTT Device Discovery. Entity identity is derived from stable Sentinel IDs, never from an IP address or display name.

## Controller device

Device identifier: `home_sentinel`.

| Entity | Stable unique_id | Source topic | Meaning |
|---|---|---|---|
| `sensor.home_sentinel_system_health` | `home_sentinel_system_health` | `sentinel/system/state` | Unified Sentinel health state |
| `sensor.home_sentinel_recording_disk_free` | `home_sentinel_recording_disk_free` | `sentinel/system/state` | Recording storage free bytes |
| `sensor.home_sentinel_ai_status` | `home_sentinel_ai_status` | `sentinel/system/state` | Local AI runtime state |
| `sensor.home_sentinel_backup_status` | `home_sentinel_backup_status` | `sentinel/system/state` | Last backup state |

Availability: `sentinel/system/availability`, payloads `online` / `offline`.

## Managed camera device

For stable `camera_id = <id>`:

- device identifier: `home_sentinel_camera_<id>`;
- connectivity unique ID: `home_sentinel_camera_<id>_connectivity`;
- recording unique ID: `home_sentinel_camera_<id>_recording`;
- detection unique ID: `home_sentinel_camera_<id>_detection`.

Changing the camera display name or IP address MUST NOT change these IDs.

## Intercom device (contract reserved for P9)

For stable `intercom_id = <id>` the following IDs are reserved:

- `home_sentinel_intercom_<id>_doorbell`;
- `home_sentinel_intercom_<id>_door`;
- `home_sentinel_intercom_<id>_lock`;
- `home_sentinel_intercom_<id>_connectivity`.

The lock entity, when enabled, is only a proxy to the Sentinel authorization/command pipeline. Home Assistant never receives direct physical relay credentials.

## Lifecycle rules

1. Device discovery config is retained.
2. Removal uses an empty retained discovery payload.
3. Sentinel republishes discovery after Home Assistant MQTT birth (`homeassistant/status = online`).
4. Display-name changes update discovery metadata only.
5. `unique_id` values are immutable after provisioning.
