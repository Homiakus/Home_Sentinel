# Degradation matrix

| Failure | Must continue | Becomes unavailable/degraded |
|---|---|---|
| Sentinel UI | Frigate recording/detection, HA automations | Unified UI |
| Sentinel Core | Frigate recording/detection, independent HA automations | Orchestration, unified notifications |
| Home Assistant | Frigate recording, Sentinel event processing | HA automations/entities |
| Ollama | Recording, detection, doorbell, notifications without enrichment | VLM enrichment |
| Telegram | All local functions | Remote Telegram notifications/actions |
| MQTT | Frigate recording; direct Frigate UI | Cross-service event automations; intercom commands if MQTT-only |
| Frigate | HA unrelated automation, Sentinel management | Recording/detection/review video path |
| Camera | Other cameras/system | That camera's live/recording/detection |
| Recording disk | Control plane should remain alive | Recording; system raises CRITICAL |

Door unlock must never become *less* restrictive because a dependency is unavailable.
