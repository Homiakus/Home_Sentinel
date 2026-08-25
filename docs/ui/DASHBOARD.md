# Sentinel Dashboard

The Sentinel web UI is an embedded, same-origin management surface served by the Go control plane. It does not require direct browser access to Frigate, go2rtc, MQTT, Home Assistant or Ollama.

## Pages

- **Overview** — overall health, camera availability, recent incidents, entrance state, disk/backup state.
- **Cameras** — snapshot wall. Tiles intentionally use authenticated latest-frame requests rather than opening one WebRTC session per camera.
- **Camera detail** — live stream, stream metadata and on-demand diagnostics. Two-way-talk capable cameras expose a separate talk action.
- **Entrance** — intercom camera, observed door/lock state, two-way-talk action and step-up protected unlock.
- **Events** — normalized Sentinel event feed with local filtering.
- **Incidents** — correlated incident list and ordered event timeline.
- **Search** — local metadata search over cameras, intercoms, incidents and normalized event payloads. Frigate semantic search remains a separate optional capability until a stable public API contract is selected.
- **System** — dependency-aware health and hardware/storage diagnostics.
- **Settings** — Home Assistant setup, Frigate plan/apply, Telegram pairing, AI model inventory, backup operations and user/RBAC administration.

## Live-stream security boundary

The browser never receives the internal go2rtc HTTP API address. Sentinel exposes only the following authenticated paths:

- `/stream.html`
- `/webrtc.html`
- `/video-stream.js`
- `/video-rtc.js`
- `/api/ws`

`src` must match a currently managed Sentinel camera stream. Sentinel session cookies, Authorization headers and CSRF tokens are stripped before forwarding the request to go2rtc. `Set-Cookie` from go2rtc is stripped on the response.

Frigate/go2rtc HTTP port 1984 remains private inside the Docker control network. WebRTC media port 8555 is a separate data-plane port and must be bound only to an explicitly selected trusted LAN/VPN host address. `SENTINEL_FRIGATE_WEBRTC_CANDIDATES` controls the candidates generated into the Sentinel-owned Frigate `go2rtc.webrtc` section.

If WebRTC is not reachable, the universal viewer can fall back to browser-compatible streaming modes. Two-way talk is only presented when the camera declares that capability and a go2rtc live proxy is configured; production use also requires a browser secure context (HTTPS).

## Realtime UI contract

`GET /api/v1/events/stream` returns standard SSE `message` events. The JSON payload retains the normalized event `type`, so clients do not need to register a separate EventSource handler for every future event type.

## Settings safety

- Secrets are write-only from the dashboard and are never echoed back.
- Frigate apply is plan/validate/apply/verify/rollback and requires fresh password reauthentication.
- User access changes require the `users:manage` capability and fresh password reauthentication.
- The last enabled administrator cannot be disabled or demoted.
- Door unlock uses the existing RBAC + CSRF + step-up + expiring command pipeline.
- Backup restore-test operates on a selected snapshot and does not replace production state.
