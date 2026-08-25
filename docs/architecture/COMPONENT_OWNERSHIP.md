# Component ownership

| Capability | Primary owner | Sentinel responsibility |
|---|---|---|
| Recording / NVR | Frigate | Desired-state generation, health and links |
| Detection / tracking | Frigate | Policies, normalized events, diagnostics |
| Stream restream / WebRTC | go2rtc | Stream naming and desired configuration |
| Home automation | Home Assistant | Publish entities, invoke allowlisted actions |
| Event transport | Mosquitto | Topic contracts, ACL-aware client integration |
| Local VLM/LLM inference | Ollama | Queue, policy, validation and provider abstraction |
| Backup repository | restic | Scheduling, manifests, check/restore verification |
| Unified configuration / UX | Sentinel Core | Sole owner |
| Door safety policy | Sentinel Core + physical interlock | Authorization, expiring command, audit |

A feature must not gain a second primary owner merely for convenience.
