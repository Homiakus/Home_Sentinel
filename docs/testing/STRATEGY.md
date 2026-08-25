# Testing strategy

- Unit: deterministic domain/config/state functions.
- Contract: each integration boundary using recorded fixtures/fake servers.
- Integration: SQLite/MQTT/HTTP/WebSocket/service adapters in containers.
- E2E: browser/user workflow through real service composition.
- HIL: multiple physical cameras and ESP32 intercom.
- Fault injection: service restarts, timeouts, malformed responses, disk pressure.
- Soak: minimum 72 hours before production release.

Tests must not silently depend on the developer's home LAN.
